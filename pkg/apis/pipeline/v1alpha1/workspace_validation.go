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

	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

// Validate looks at the Volume provided in wb and makes sure that it is valid.
// This means that only one VolumeSource can be specified, and also that the
// supported VolumeSource is itself valid.
func (b *WorkspaceBinding) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(b, &WorkspaceBinding{}) || b == nil {
		return apis.ErrMissingField(apis.CurrentField)
	}

	// Users should only provide one supported VolumeSource.
	if b.PersistentVolumeClaim != nil && b.EmptyDir != nil {
		return apis.ErrMultipleOneOf("workspace.persistentvolumeclaim", "workspace.emptydir")
	}

	// Users must provide at least one supported VolumeSource.
	if b.PersistentVolumeClaim == nil && b.EmptyDir == nil {
		return apis.ErrMissingOneOf("workspace.persistentvolumeclaim", "workspace.emptydir")
	}

	// For a PersistentVolumeClaim to work, you must at least provide the name of the PVC to use.
	if b.PersistentVolumeClaim != nil && b.PersistentVolumeClaim.ClaimName == "" {
		return apis.ErrMissingField("workspace.persistentvolumeclaim.claimname")
	}
	return nil
}
