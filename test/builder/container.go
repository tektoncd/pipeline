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

// ContainerOp is an operation which modifies a Container struct.
type ContainerOp = builder.ContainerOp

// VolumeMountOp is an operation which modifies a VolumeMount struct.
type VolumeMountOp = builder.VolumeMountOp

// ResourceRequirementsOp is an operation which modifies a ResourceRequirements struct.
type ResourceRequirementsOp = builder.ResourceRequirementsOp

// ResourceListOp is an operation which modifies a ResourceList struct.
type ResourceListOp = builder.ResourceListOp

var (

	// Command sets the command to the Container (step in this case).
	Command = builder.Command

	// Args sets the command arguments to the Container (step in this case).
	Args = builder.Args

	// EnvVar add an environment variable, with specified name and value, to the Container (step).
	EnvVar = builder.EnvVar

	// WorkingDir sets the WorkingDir on the Container.
	WorkingDir = builder.WorkingDir

	// VolumeMount add a VolumeMount to the Container (step).
	VolumeMount = builder.VolumeMount

	// Resources adds ResourceRequirements to the Container (step).
	Resources = builder.Resources

	// Limits adds Limits to the ResourceRequirements.
	Limits = builder.Limits

	// Requests adds Requests to the ResourceRequirements.
	Requests = builder.Requests

	// CPU sets the CPU resource on the ResourceList.
	CPU = builder.CPU

	// Memory sets the memory resource on the ResourceList.
	Memory = builder.Memory

	// EphemeralStorage sets the ephemeral storage resource on the ResourceList.
	EphemeralStorage = builder.EphemeralStorage

	// TerminationMessagePolicy sets the policy of the termination message.
	TerminationMessagePolicy = builder.TerminationMessagePolicy
)
