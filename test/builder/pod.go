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

// PodOp is an operation which modifies a Pod struct.
// Deprecated: moved to internal/builder/v1alpha1
type PodOp = v1beta1.PodOp

// PodSpecOp is an operation which modifies a PodSpec struct.
// Deprecated: moved to internal/builder/v1alpha1
type PodSpecOp = v1beta1.PodSpecOp

// PodStatusOp is an operation which modifies a PodStatus struct.
// Deprecated: moved to internal/builder/v1alpha1
type PodStatusOp = v1beta1.PodStatusOp

var (
	// Pod creates a Pod with default values.
	// Any number of Pod modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	Pod = v1beta1.Pod

	// PodNamespace sets the namespace on the Pod.
	// Deprecated: moved to internal/builder/v1alpha1
	PodNamespace = v1beta1.PodNamespace

	// PodAnnotation adds an annotation to the Pod.
	// Deprecated: moved to internal/builder/v1alpha1
	PodAnnotation = v1beta1.PodAnnotation

	// PodLabel adds a label to the Pod.
	// Deprecated: moved to internal/builder/v1alpha1
	PodLabel = v1beta1.PodLabel

	// PodOwnerReference adds an OwnerReference, with specified kind and name, to the Pod.
	// Deprecated: moved to internal/builder/v1alpha1
	PodOwnerReference = v1beta1.PodOwnerReference

	// PodSpec creates a PodSpec with default values.
	// Any number of PodSpec modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PodSpec = v1beta1.PodSpec

	// PodRestartPolicy sets the restart policy on the PodSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PodRestartPolicy = v1beta1.PodRestartPolicy

	// PodServiceAccountName sets the service account on the PodSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PodServiceAccountName = v1beta1.PodServiceAccountName

	// PodContainer adds a Container, with the specified name and image, to the PodSpec.
	// Any number of Container modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PodContainer = v1beta1.PodContainer

	// PodInitContainer adds an InitContainer, with the specified name and image, to the PodSpec.
	// Any number of Container modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PodInitContainer = v1beta1.PodInitContainer

	// PodVolumes sets the Volumes on the PodSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PodVolumes = v1beta1.PodVolumes

	// PodCreationTimestamp sets the creation time of the pod
	// Deprecated: moved to internal/builder/v1alpha1
	PodCreationTimestamp = v1beta1.PodCreationTimestamp

	// PodStatus creates a PodStatus with default values.
	// Any number of PodStatus modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PodStatus = v1beta1.PodStatus

	// PodStatusConditions adds a Conditions (set) to the Pod status.
	// Deprecated: moved to internal/builder/v1alpha1
	PodStatusConditions = v1beta1.PodStatusConditions
)
