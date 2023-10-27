/*
Copyright 2023 The Tekton Authors
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

package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*StepAction)(nil)

// SetDefaults implements apis.Defaultable
func (s *StepAction) SetDefaults(ctx context.Context) {
	s.Spec.SetDefaults(ctx)
}

// SetDefaults set any defaults for the StepAction spec
func (ss *StepActionSpec) SetDefaults(ctx context.Context) {
	for i := range ss.Params {
		ss.Params[i].SetDefaults(ctx)
	}
	for i := range ss.Results {
		ss.Results[i].SetDefaults(ctx)
	}
}
