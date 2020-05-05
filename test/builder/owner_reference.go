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

package builder

import (
	v1beta1 "github.com/tektoncd/pipeline/internal/builder/v1beta1"
)

// OwnerReferenceOp is an operation which modifies an OwnerReference struct.
// Deprecated: moved to internal/builder/v1alpha1
type OwnerReferenceOp = v1beta1.OwnerReferenceOp

var (
	// OwnerReferenceAPIVersion sets the APIVersion to the OwnerReference.
	// Deprecated: moved to internal/builder/v1alpha1
	OwnerReferenceAPIVersion = v1beta1.OwnerReferenceAPIVersion

	// Controller sets the Controller to the OwnerReference.
	// Deprecated: moved to internal/builder/v1alpha1
	Controller = v1beta1.Controller

	// BlockOwnerDeletion sets the BlockOwnerDeletion to the OwnerReference.
	// Deprecated: moved to internal/builder/v1alpha1
	BlockOwnerDeletion = v1beta1.BlockOwnerDeletion
)
