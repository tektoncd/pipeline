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

// StepOp is an operation which modifies a Container struct.
type StepOp = builder.StepOp

var (
	// StepCommand sets the command to the Container (step in this case).
	StepCommand = builder.StepCommand

	// StepSecurityContext sets the SecurityContext to the Step.
	StepSecurityContext = builder.StepSecurityContext

	// StepArgs sets the command arguments to the Container (step in this case).
	StepArgs = builder.StepArgs

	// StepEnvVar add an environment variable, with specified name and value, to the Container (step).
	StepEnvVar = builder.StepEnvVar

	// StepWorkingDir sets the WorkingDir on the Container.
	StepWorkingDir = builder.StepWorkingDir

	// StepVolumeMount add a VolumeMount to the Container (step).
	StepVolumeMount = builder.StepVolumeMount

	// StepResources adds ResourceRequirements to the Container (step).
	StepResources = builder.StepResources

	// StepLimits adds Limits to the ResourceRequirements.
	StepLimits = builder.StepLimits

	// StepRequests adds Requests to the ResourceRequirements.
	StepRequests = builder.StepRequests

	// StepCPU sets the CPU resource on the ResourceList.
	StepCPU = builder.StepCPU

	// StepMemory sets the memory resource on the ResourceList.
	StepMemory = builder.StepMemory

	// StepEphemeralStorage sets the ephemeral storage resource on the ResourceList.
	StepEphemeralStorage = builder.StepEphemeralStorage

	// StepTerminationMessagePath sets the source of the termination message.
	StepTerminationMessagePath = builder.StepTerminationMessagePath

	// StepTerminationMessagePolicy sets the policy of the termination message.
	StepTerminationMessagePolicy = builder.StepTerminationMessagePolicy
)
