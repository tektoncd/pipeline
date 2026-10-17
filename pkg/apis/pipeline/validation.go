/*
Copyright 2026 The Tekton Authors

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

package pipeline

import (
	"context"
	"fmt"
	"slices"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"
)

// ControllerManagedLabels are the labels stamped and trusted by the reconciler.
// For example, pipeline-in-pipeline cycle detection reads PipelineLabelKey, so a
// user must not be able to change these labels once the controller has set them.
var ControllerManagedLabels = []string{
	PipelineLabelKey,
	PipelineTaskLabelKey,
	PipelineRunLabelKey,
}

// IsControllerServiceAccount reports whether the admission request in ctx comes
// from a ServiceAccount in the Tekton system namespace (the controller, webhook
// and events controller). Those are Tekton's own trusted components and are
// exempt from the controller-managed label immutability check so the reconciler
// can stamp and normalise these labels. The caller's UserInfo is populated by
// the apiserver and cannot be spoofed; it is absent (returns false) for calls
// made outside the admission webhook path.
func IsControllerServiceAccount(ctx context.Context) bool {
	ui := apis.GetUserInfo(ctx)
	if ui == nil {
		return false
	}
	// e.g. system:serviceaccounts:tekton-pipelines
	return slices.Contains(ui.Groups, "system:serviceaccounts:"+system.Namespace())
}

// ValidateImmutableLabels rejects any change to a controller-managed label once
// it has been set. Presence of the key is treated as "set" (matching how the
// reconciler reads these labels), so a managed label cannot be removed or have
// its value changed once present, even if that value is empty. A key that is
// absent from oldLabels is treated as "not set yet" so the controller can still
// stamp it on first reconcile.
func ValidateImmutableLabels(oldLabels, newLabels map[string]string, keys []string) (errs *apis.FieldError) {
	for _, key := range keys {
		oldVal, hadOld := oldLabels[key]
		if !hadOld {
			continue
		}
		if newVal, hasNew := newLabels[key]; !hasNew || newVal != oldVal {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("label %q is immutable once set", key),
				"",
			).ViaFieldKey("labels", key).ViaField("metadata"))
		}
	}
	return errs
}
