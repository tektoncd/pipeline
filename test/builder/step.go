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
	v1alpha1 "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
)

// StepOp is an operation which modifies a Container struct.
// Deprecated: moved to internal/builder/v1alpha1
type StepOp = v1alpha1.StepOp

var (
	// StepName sets the name of the step.
	// Deprecated: moved to internal/builder/v1alpha1
	StepName = v1alpha1.StepName

	// StepCommand sets the command to the Container (step in this case).
	// Deprecated: moved to internal/builder/v1alpha1
	StepCommand = v1alpha1.StepCommand

	// StepSecurityContext sets the SecurityContext to the Step.
	// Deprecated: moved to internal/builder/v1alpha1
	StepSecurityContext = v1alpha1.StepSecurityContext

	// StepArgs sets the command arguments to the Container (step in this case).
	// Deprecated: moved to internal/builder/v1alpha1
	StepArgs = v1alpha1.StepArgs

	// StepEnvVar add an environment variable, with specified name and value, to the Container (step).
	// Deprecated: moved to internal/builder/v1alpha1
	StepEnvVar = v1alpha1.StepEnvVar

	// StepWorkingDir sets the WorkingDir on the Container.
	// Deprecated: moved to internal/builder/v1alpha1
	StepWorkingDir = v1alpha1.StepWorkingDir

	// StepVolumeMount add a VolumeMount to the Container (step).
	// Deprecated: moved to internal/builder/v1alpha1
	StepVolumeMount = v1alpha1.StepVolumeMount

	// StepScript sets the script to the Step.
	// Deprecated: moved to internal/builder/v1alpha1
	StepScript = v1alpha1.StepScript

	// StepResources adds ResourceRequirements to the Container (step).
	// Deprecated: moved to internal/builder/v1alpha1
	StepResources = v1alpha1.StepResources

	// StepLimits adds Limits to the ResourceRequirements.
	// Deprecated: moved to internal/builder/v1alpha1
	StepLimits = v1alpha1.StepLimits

	// StepRequests adds Requests to the ResourceRequirements.
	// Deprecated: moved to internal/builder/v1alpha1
	StepRequests = v1alpha1.StepRequests

	// StepCPU sets the CPU resource on the ResourceList.
	// Deprecated: moved to internal/builder/v1alpha1
	StepCPU = v1alpha1.StepCPU

	// StepMemory sets the memory resource on the ResourceList.
	// Deprecated: moved to internal/builder/v1alpha1
	StepMemory = v1alpha1.StepMemory

	// StepEphemeralStorage sets the ephemeral storage resource on the ResourceList.
	// Deprecated: moved to internal/builder/v1alpha1
	StepEphemeralStorage = v1alpha1.StepEphemeralStorage

	// StepTerminationMessagePath sets the source of the termination message.
	// Deprecated: moved to internal/builder/v1alpha1
	StepTerminationMessagePath = v1alpha1.StepTerminationMessagePath
)
