/*
Copyright 2019 The Tekton Authors

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

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*ClusterTask)(nil)

// Validate performs validation of the metadata and spec of this ClusterTask.
func (t *ClusterTask) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(t.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	if apis.IsInDelete(ctx) {
		return nil
	}
	return t.Spec.Validate(ctx)
}
