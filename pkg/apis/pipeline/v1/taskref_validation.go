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

package v1

import (
	"context"

	"knative.dev/pkg/apis"
)

// Validate ensures that a supplied TaskRef field is populated
// correctly. No errors are returned for a nil TaskRef.
func (ref *TaskRef) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return errs
	}
	// A non-default Kind is only meaningful for a Custom Task reference, which
	// requires APIVersion to be set as well (see TaskRef.IsCustomTask). Without
	// APIVersion the kind is silently ignored at resolution: the ref resolves
	// as an ordinary namespaced Task when one exists, or fails with a
	// confusing not-found error.
	if ref.Kind != "" && ref.Kind != NamespacedTaskKind && ref.APIVersion == "" {
		errs = errs.Also(apis.ErrInvalidValue("custom task ref must specify apiVersion", "apiVersion"))
	}
	return errs.Also(validateRef(ctx, ref.Name, ref.Resolver, ref.Params))
}
