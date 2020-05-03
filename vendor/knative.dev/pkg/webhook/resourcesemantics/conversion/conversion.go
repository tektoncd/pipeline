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
	apixv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

// Convert implements webhook.ConversionController
func (r *reconciler) Convert(
	ctx context.Context,
	req *apixv1beta1.ConversionRequest,
) *apixv1beta1.ConversionResponse {

	if r.withContext != nil {
		ctx = r.withContext(ctx)
	}

	res := &apixv1beta1.ConversionResponse{
		UID: req.UID,
		Result: metav1.Status{
			Status: metav1.StatusSuccess,
		},
	}

	result := make([]runtime.RawExtension, 0, len(req.Objects))

	for _, obj := range req.Objects {
		converted, err := r.convert(ctx, obj, req.DesiredAPIVersion)
		if err != nil {
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
) (outRaw runtime.RawExtension, err error) {

	logger := logging.FromContext(ctx)

	defer func() {
		if err != nil {
			logger.Errorf("Conversion failed: %s", err)
		}
	}()

	inGVK, err := parseGVK(inRaw)
	if err != nil {
		return
	}

	logger.Infof("Converting %s to version %s", formatGVK(inGVK), targetVersion)

	inGK := inGVK.GroupKind()
	conv, ok := r.kinds[inGK]
	if !ok {
		err = fmt.Errorf("no conversion support for type %s", formatGK(inGVK.GroupKind()))
		return
	}

	outGVK, err := parseAPIVersion(targetVersion, inGK.Kind)
	if err != nil {
		return
	}

	inZygote, ok := conv.Zygotes[inGVK.Version]
	if !ok {
		err = fmt.Errorf("conversion not supported for type %s", formatGVK(inGVK))
		return
	}
	outZygote, ok := conv.Zygotes[outGVK.Version]
	if !ok {
		err = fmt.Errorf("conversion not supported for type %s", formatGVK(outGVK))
		return
	}
	hubZygote, ok := conv.Zygotes[conv.HubVersion]
	if !ok {
		inGK := inGVK.GroupKind()
		err = fmt.Errorf("conversion not supported for type %s", formatGK(inGK))
		return
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
		err = fmt.Errorf("unable to unmarshal input: %s", err)
		return
	}

	if inGVK.Version == conv.HubVersion {
		hub = in
	} else if err = hub.ConvertFrom(ctx, in); err != nil {
		err = fmt.Errorf("conversion failed to version %s for type %s -  %s", outGVK.Version, formatGVK(inGVK), err)
		return
	}

	if outGVK.Version == conv.HubVersion {
		out = hub
	} else if err = hub.ConvertTo(ctx, out); err != nil {
		err = fmt.Errorf("conversion failed to version %s for type %s -  %s", outGVK.Version, formatGVK(inGVK), err)
		return
	}

	out.GetObjectKind().SetGroupVersionKind(outGVK)

	if defaultable, ok := out.(apis.Defaultable); ok {
		defaultable.SetDefaults(ctx)
	}

	if outRaw.Raw, err = json.Marshal(out); err != nil {
		err = fmt.Errorf("unable to marshal output: %s", err)
		return
	}

	return
}

func parseGVK(in runtime.RawExtension) (gvk schema.GroupVersionKind, err error) {
	var typeMeta metav1.TypeMeta

	if err = json.Unmarshal(in.Raw, &typeMeta); err != nil {
		err = fmt.Errorf("error parsing type meta %q - %s", string(in.Raw), err)
		return
	}

	gv, err := schema.ParseGroupVersion(typeMeta.APIVersion)
	gvk = gv.WithKind(typeMeta.Kind)

	if gvk.Group == "" || gvk.Version == "" || gvk.Kind == "" {
		err = fmt.Errorf("invalid GroupVersionKind %v", gvk)
		return
	}

	return
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
