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

package pod

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

// ValidateResources checks that the pod-level resource requests do not exceed
// the corresponding limits, so that such values are rejected when the TaskRun
// or PipelineRun is created rather than when its Pod is.
// Which resource names and fields are supported at pod level is Kubernetes
// policy that varies across versions, so it is deliberately left to Kubernetes,
// as are checks that depend on the containers of the Pod.
func (tpl *Template) ValidateResources() (errs *apis.FieldError) {
	if tpl == nil || tpl.Resources == nil {
		return nil
	}
	r := tpl.Resources
	for _, name := range sortedResourceNames(r.Requests) {
		limit, ok := r.Limits[name]
		if !ok {
			continue
		}
		if request := r.Requests[name]; request.Cmp(limit) > 0 {
			errs = errs.Also(apis.ErrInvalidValue(request.String(), "",
				fmt.Sprintf("must be less than or equal to %s limit of %s", name, limit.String())).
				ViaFieldKey("resources.requests", string(name)))
		}
	}
	return errs
}

func sortedResourceNames(list corev1.ResourceList) []corev1.ResourceName {
	names := make([]corev1.ResourceName, 0, len(list))
	for name := range list {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
