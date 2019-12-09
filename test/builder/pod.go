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
	builder "github.com/tektoncd/pipeline/test/builder/v1alpha1"
)

// PodOp is an operation which modifies a Pod struct.
type PodOp = builder.PodOp

// PodSpecOp is an operation which modifies a PodSpec struct.
type PodSpecOp = builder.PodSpecOp

// PodStatusOp is an operation which modifies a PodStatus struct.
type PodStatusOp = builder.PodStatusOp

var (
	// Pod creates a Pod with default values.
	// Any number of Pod modifiers can be passed to transform it.
	Pod = builder.Pod

	// PodAnnotation adds an annotation to the Pod.
	PodAnnotation = builder.PodAnnotation

	// PodLabel adds a label to the Pod.
	PodLabel = builder.PodLabel

	// PodOwnerReference adds an OwnerReference, with specified kind and name, to the Pod.
	PodOwnerReference = builder.PodOwnerReference

	// PodSpec creates a PodSpec with default values.
	// Any number of PodSpec modifiers can be passed to transform it.
	PodSpec = builder.PodSpec

	// PodRestartPolicy sets the restart policy on the PodSpec.
	PodRestartPolicy = builder.PodRestartPolicy

	// PodServiceAccountName sets the service account on the PodSpec.
	PodServiceAccountName = builder.PodServiceAccountName

	// PodContainer adds a Container, with the specified name and image, to the PodSpec.
	// Any number of Container modifiers can be passed to transform it.
	PodContainer = builder.PodContainer

	// PodInitContainer adds an InitContainer, with the specified name and image, to the PodSpec.
	// Any number of Container modifiers can be passed to transform it.
	PodInitContainer = builder.PodInitContainer

	// PodVolume sets the Volumes on the PodSpec.
	PodVolumes = builder.PodVolumes

	// PodCreationTimestamp sets the creation time of the pod
	PodCreationTimestamp = builder.PodCreationTimestamp

	// PodStatus creates a PodStatus with default values.
	// Any number of PodStatus modifiers can be passed to transform it.
	PodStatus = builder.PodStatus

	PodStatusConditions = builder.PodStatusConditions
)
