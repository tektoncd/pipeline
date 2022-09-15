/*
Copyright 2022 The Tekton Authors

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

package v1beta1

import "context"

// ManagedByLabelKey is the label key used to mark what is managing this resource
const ManagedByLabelKey = "app.kubernetes.io/managed-by"

// SetDefaults walks a ResolutionRequest object and sets any default
// values that are required to be set before a reconciler sees it.
func (rr *ResolutionRequest) SetDefaults(ctx context.Context) {
	if rr.TypeMeta.Kind == "" {
		rr.TypeMeta.Kind = "ResolutionRequest"
	}
	if rr.TypeMeta.APIVersion == "" {
		rr.TypeMeta.APIVersion = "resolution.tekton.dev/v1alpha1"
	}
}
