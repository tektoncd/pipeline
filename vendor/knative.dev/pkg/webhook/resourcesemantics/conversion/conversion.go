/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package conversion

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
)

// Convert implements webhook.ConversionController
func (r *reconciler) Convert(
	ctx context.Context,
	req *apixv1.ConversionRequest,
) *apixv1.ConversionResponse {

	if r.withContext != nil {
		ctx = r.withContext(ctx)
	}

	res := &apixv1.ConversionResponse{
		UID: req.UID,
		Result: metav1.Status{
			Status: metav1.StatusSuccess,
		},
	}

	result := make([]runtime.RawExtension, 0, len(req.Objects))

	for _, obj := range req.Objects {
		converted, err := r.convert(ctx, obj, req.DesiredAPIVersion)
		if err != nil {
			logging.FromContext(ctx).Errorf("Conversion failed: %v", err)
			res.Result.Status = metav1.StatusFailure
			res.Result.Message = err.Error()
			break
		}

		result = append(result, converted)
	}

	res.ConvertedObjects = result
	return res
}

func (r *reconciler) convert(
	ctx context.Context,
	inRaw runtime.RawExtension,
	targetVersion string,
) (runtime.RawExtension, error) {
	logger := logging.FromContext(ctx)
	var ret runtime.RawExtension

	inGVK, err := parseGVK(inRaw)
	if err != nil {
		return ret, err
	}

	inGK := inGVK.GroupKind()
	conv, ok := r.kinds[inGK]
	if !ok {
		return ret, fmt.Errorf("no conversion support for type %s", formatGK(inGVK.GroupKind()))
	}

	outGVK, err := parseAPIVersion(targetVersion, inGK.Kind)
	if err != nil {
		return ret, err
	}

	inZygote, ok := conv.Zygotes[inGVK.Version]
	if !ok {
		return ret, fmt.Errorf("conversion not supported for type %s", formatGVK(inGVK))
	}
	outZygote, ok := conv.Zygotes[outGVK.Version]
	if !ok {
		return ret, fmt.Errorf("conversion not supported for type %s", formatGVK(outGVK))
	}
	hubZygote, ok := conv.Zygotes[conv.HubVersion]
	if !ok {
		return ret, fmt.Errorf("conversion not supported for type %s", formatGK(inGVK.GroupKind()))
	}

	in := inZygote.DeepCopyObject().(ConvertibleObject)
	hub := hubZygote.DeepCopyObject().(ConvertibleObject)
	out := outZygote.DeepCopyObject().(ConvertibleObject)

	hubGVK := inGVK.GroupKind().WithVersion(conv.HubVersion)

	logger = logger.With(
		zap.String("inputType", formatGVK(inGVK)),
		zap.String("outputType", formatGVK(outGVK)),
		zap.String("hubType", formatGVK(hubGVK)),
	)

	// TODO(dprotaso) - potentially error on unknown fields
	if err = json.Unmarshal(inRaw.Raw, &in); err != nil {
		return ret, fmt.Errorf("unable to unmarshal input: %w", err)
	}

	if acc, err := kmeta.DeletionHandlingAccessor(in); err == nil {
		// TODO: right now we don't convert any non-namespaced objects. If we ever do that
		// this needs to updated to deal with it.
		logger = logger.With(zap.String(logkey.Key, acc.GetNamespace()+"/"+acc.GetName()))
	} else {
		logger.Infof("Could not get Accessor for %s: %v", formatGK(inGVK.GroupKind()), err)
	}
	ctx = logging.WithLogger(ctx, logger)

	if inGVK.Version == conv.HubVersion {
		hub = in
	} else if err = hub.ConvertFrom(ctx, in); err != nil {
		return ret, fmt.Errorf("conversion failed to version %s for type %s -  %w", outGVK.Version, formatGVK(inGVK), err)
	}

	if outGVK.Version == conv.HubVersion {
		out = hub
	} else if err = hub.ConvertTo(ctx, out); err != nil {
		return ret, fmt.Errorf("conversion failed to version %s for type %s -  %w", outGVK.Version, formatGVK(inGVK), err)
	}

	out.GetObjectKind().SetGroupVersionKind(outGVK)

	if defaultable, ok := out.(apis.Defaultable); ok {
		defaultable.SetDefaults(ctx)
	}

	if ret.Raw, err = json.Marshal(out); err != nil {
		return ret, fmt.Errorf("unable to marshal output: %w", err)
	}
	return ret, nil
}

func parseGVK(in runtime.RawExtension) (schema.GroupVersionKind, error) {
	var (
		typeMeta metav1.TypeMeta
		gvk      schema.GroupVersionKind
	)

	if err := json.Unmarshal(in.Raw, &typeMeta); err != nil {
		return gvk, fmt.Errorf("error parsing type meta %q - %w", string(in.Raw), err)
	}

	gv, err := schema.ParseGroupVersion(typeMeta.APIVersion)
	if err != nil {
		return gvk, fmt.Errorf("error parsing GV %q: %w", typeMeta.APIVersion, err)
	}
	gvk = gv.WithKind(typeMeta.Kind)

	if gvk.Group == "" || gvk.Version == "" || gvk.Kind == "" {
		return gvk, fmt.Errorf("invalid GroupVersionKind %v", gvk)
	}

	return gvk, nil
}

func parseAPIVersion(apiVersion string, kind string) (schema.GroupVersionKind, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		err = fmt.Errorf("desired API version %q is not valid", apiVersion)
		return schema.GroupVersionKind{}, err
	}

	if !isValidGV(gv) {
		err = fmt.Errorf("desired API version %q is not valid", apiVersion)
		return schema.GroupVersionKind{}, err
	}

	return gv.WithKind(kind), nil
}

func formatGVK(gvk schema.GroupVersionKind) string {
	return fmt.Sprintf("[kind=%s group=%s version=%s]", gvk.Kind, gvk.Group, gvk.Version)
}

func formatGK(gk schema.GroupKind) string {
	return fmt.Sprintf("[kind=%s group=%s]", gk.Kind, gk.Group)
}

func isValidGV(gk schema.GroupVersion) bool {
	return gk.Group != "" && gk.Version != ""
}
