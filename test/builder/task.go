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

// TaskOp is an operation which modify a Task struct.
type TaskOp = builder.TaskOp

// ClusterTaskOp is an operation which modify a ClusterTask struct.
type ClusterTaskOp = builder.ClusterTaskOp

// TaskSpeOp is an operation which modify a TaskSpec struct.
type TaskSpecOp = builder.TaskSpecOp

// InputsOp is an operation which modify an Inputs struct.
type InputsOp = builder.InputsOp

// OutputsOp is an operation which modify an Outputs struct.
type OutputsOp = builder.OutputsOp

// TaskRunOp is an operation which modify a TaskRun struct.
type TaskRunOp = builder.TaskRunOp

// TaskRunSpecOp is an operation which modify a TaskRunSpec struct.
type TaskRunSpecOp = builder.TaskRunSpecOp

// TaskResourceOp is an operation which modify a TaskResource struct.
type TaskResourceOp = builder.TaskResourceOp

// TaskResourceBindingOp is an operation which modify a TaskResourceBinding struct.
type TaskResourceBindingOp = builder.TaskResourceBindingOp

// TaskRunStatusOp is an operation which modify a TaskRunStatus struct.
type TaskRunStatusOp = builder.TaskRunStatusOp

// TaskRefOp is an operation which modify a TaskRef struct.
type TaskRefOp = builder.TaskRefOp

// TaskRunInputsOp is an operation which modify a TaskRunInputs struct.
type TaskRunInputsOp = builder.TaskRunInputsOp

// TaskRunOutputsOp is an operation which modify a TaskRunOutputs struct.
type TaskRunOutputsOp = builder.TaskRunOutputsOp

// ResolvedTaskResourcesOp is an operation which modify a ResolvedTaskResources struct.
type ResolvedTaskResourcesOp = builder.ResolvedTaskResourcesOp

// StepStateOp is an operation which modify a StepStep struct.
type StepStateOp = builder.StepStateOp

// VolumeOp is an operation which modify a Volume struct.
type VolumeOp = builder.VolumeOp

var (
	// Task creates a Task with default values.
	// Any number of Task modifier can be passed to transform it.
	Task = builder.Task

	// ClusterTask creates a ClusterTask with default values.
	// Any number of ClusterTask modifier can be passed to transform it.
	ClusterTask = builder.ClusterTask

	// ClusterTaskSpec sets the specified spec of the cluster task.
	// Any number of TaskSpec modifier can be passed to create it.
	ClusterTaskSpec = builder.ClusterTaskSpec

	// TaskSpec sets the specified spec of the task.
	// Any number of TaskSpec modifier can be passed to create/modify it.
	TaskSpec = builder.TaskSpec

	// Step adds a step with the specified name and image to the TaskSpec.
	// Any number of Container modifier can be passed to transform it.
	Step = builder.Step

	Sidecar = builder.Sidecar

	// TaskStepTemplate adds a base container for all steps in the task.
	TaskStepTemplate = builder.TaskStepTemplate

	// TaskVolume adds a volume with specified name to the TaskSpec.
	// Any number of Volume modifier can be passed to transform it.
	TaskVolume = builder.TaskVolume

	// VolumeSource sets the VolumeSource to the Volume.
	VolumeSource = builder.VolumeSource

	// TaskInputs sets inputs to the TaskSpec.
	// Any number of Inputs modifier can be passed to transform it.
	TaskInputs = builder.TaskInputs

	// TaskOutputs sets inputs to the TaskSpec.
	// Any number of Outputs modifier can be passed to transform it.
	TaskOutputs = builder.TaskOutputs

	// InputsResource adds a resource, with specified name and type, to the Inputs.
	// Any number of TaskResource modifier can be passed to transform it.
	InputsResource = builder.InputsResource

	ResourceTargetPath = builder.ResourceTargetPath

	// OutputsResource adds a resource, with specified name and type, to the Outputs.
	OutputsResource = builder.OutputsResource

	// InputsParamSpec adds a ParamSpec, with specified name and type, to the Inputs.
	// Any number of TaskParamSpec modifier can be passed to transform it.
	InputsParamSpec = builder.InputsParamSpec

	// TaskRun creates a TaskRun with default values.
	// Any number of TaskRun modifier can be passed to transform it.
	TaskRun = builder.TaskRun

	// TaskRunStatus sets the TaskRunStatus to tshe TaskRun
	TaskRunStatus = builder.TaskRunStatus

	// PodName sets the Pod name to the TaskRunStatus.
	PodName = builder.PodName

	// StatusCondition adds a StatusCondition to the TaskRunStatus.
	StatusCondition = builder.StatusCondition

	Retry = builder.Retry

	// StepState adds a StepState to the TaskRunStatus.
	StepState = builder.StepState

	// TaskRunStartTime sets the start time to the TaskRunStatus.
	TaskRunStartTime = builder.TaskRunStartTime

	// TaskRunCompletionTime sets the start time to the TaskRunStatus.
	TaskRunCompletionTime = builder.TaskRunCompletionTime

	// TaskRunCloudEvent adds an event to the TaskRunStatus.
	TaskRunCloudEvent = builder.TaskRunCloudEvent

	// TaskRunTimeout sets the timeout duration to the TaskRunSpec.
	TaskRunTimeout = builder.TaskRunTimeout

	// TaskRunNilTimeout sets the timeout duration to nil on the TaskRunSpec.
	TaskRunNilTimeout = builder.TaskRunNilTimeout

	// TaskRunNodeSelector sets the NodeSelector to the TaskRunSpec.
	TaskRunNodeSelector = builder.TaskRunNodeSelector

	// TaskRunTolerations sets the Tolerations to the TaskRunSpec.
	TaskRunTolerations = builder.TaskRunTolerations

	// TaskRunAffinity sets the Affinity to the TaskRunSpec.
	TaskRunAffinity = builder.TaskRunAffinity

	// TaskRunPodSecurityContext sets the SecurityContext to the TaskRunSpec (through PodTemplate).
	TaskRunPodSecurityContext = builder.TaskRunPodSecurityContext

	// StateTerminated sets Terminated to the StepState.
	StateTerminated = builder.StateTerminated

	// SetStepStateTerminated sets Terminated state of a step.
	SetStepStateTerminated = builder.SetStepStateTerminated

	// SetStepStateRunning sets Running state of a step.
	SetStepStateRunning = builder.SetStepStateRunning

	// SetStepStateWaiting sets Waiting state of a step.
	SetStepStateWaiting = builder.SetStepStateWaiting

	// TaskRunOwnerReference sets the OwnerReference, with specified kind and name, to the TaskRun.
	TaskRunOwnerReference = builder.TaskRunOwnerReference

	Controller = builder.Controller

	BlockOwnerDeletion = builder.BlockOwnerDeletion

	TaskRunLabel = builder.TaskRunLabel

	TaskRunAnnotation = builder.TaskRunAnnotation

	// TaskRunSelfLink adds a SelfLink
	TaskRunSelfLink = builder.TaskRunSelfLink

	// TaskRunSpec sets the specified spec of the TaskRun.
	// Any number of TaskRunSpec modifier can be passed to transform it.
	TaskRunSpec = builder.TaskRunSpec

	// TaskRunCancelled sets the status to cancel to the TaskRunSpec.
	TaskRunCancelled = builder.TaskRunCancelled

	// TaskRunTaskRef sets the specified Task reference to the TaskRunSpec.
	// Any number of TaskRef modifier can be passed to transform it.
	TaskRunTaskRef = builder.TaskRunTaskRef

	// TaskRunSpecStatus sets the Status in the Spec, used for operations
	// such as cancelling executing TaskRuns.
	TaskRunSpecStatus = builder.TaskRunSpecStatus

	// TaskRefKind set the specified kind to the TaskRef.
	TaskRefKind = builder.TaskRefKind

	// TaskRefAPIVersion sets the specified api version to the TaskRef.
	TaskRefAPIVersion = builder.TaskRefAPIVersion

	// TaskRunTaskSpec sets the specified TaskRunSpec reference to the TaskRunSpec.
	// Any number of TaskRunSpec modifier can be passed to transform it.
	TaskRunTaskSpec = builder.TaskRunTaskSpec

	// TaskRunServiceAccount sets the serviceAccount to the TaskRunSpec.
	TaskRunServiceAccountName = builder.TaskRunServiceAccountName

	// TaskRunInputs sets inputs to the TaskRunSpec.
	// Any number of TaskRunInputs modifier can be passed to transform it.
	TaskRunInputs = builder.TaskRunInputs

	// TaskRunInputsParam add a param, with specified name and value, to the TaskRunInputs.
	TaskRunInputsParam = builder.TaskRunInputsParam

	// TaskRunInputsResource adds a resource, with specified name, to the TaskRunInputs.
	// Any number of TaskResourceBinding modifier can be passed to transform it.
	TaskRunInputsResource = builder.TaskRunInputsResource

	// TaskResourceBindingRef set the PipelineResourceRef name to the TaskResourceBinding.
	TaskResourceBindingRef = builder.TaskResourceBindingRef

	// TaskResourceBindingResourceSpec set the PipelineResourceResourceSpec to the TaskResourceBinding.
	TaskResourceBindingResourceSpec = builder.TaskResourceBindingResourceSpec

	// TaskResourceBindingRefAPIVersion set the PipelineResourceRef APIVersion to the TaskResourceBinding.
	TaskResourceBindingRefAPIVersion = builder.TaskResourceBindingRefAPIVersion

	// TaskResourceBindingPaths add any number of path to the TaskResourceBinding.
	TaskResourceBindingPaths = builder.TaskResourceBindingPaths

	// TaskRunOutputs sets inputs to the TaskRunSpec.
	// Any number of TaskRunOutputs modifier can be passed to transform it.
	TaskRunOutputs = builder.TaskRunOutputs

	// TaskRunOutputsResource adds a TaskResourceBinding, with specified name, to the TaskRunOutputs.
	// Any number of TaskResourceBinding modifier can be passed to modifiy it.
	TaskRunOutputsResource = builder.TaskRunOutputsResource

	// ResolvedTaskResources creates a ResolvedTaskResources with default values.
	// Any number of ResolvedTaskResources modifier can be passed to transform it.
	ResolvedTaskResources = builder.ResolvedTaskResources

	// ResolvedTaskResourcesTaskSpec sets a TaskSpec to the ResolvedTaskResources.
	// Any number of TaskSpec modifier can be passed to transform it.
	ResolvedTaskResourcesTaskSpec = builder.ResolvedTaskResourcesTaskSpec

	// ResolvedTaskResourcesInputs adds an input PipelineResource, with specified name, to the ResolvedTaskResources.
	ResolvedTaskResourcesInputs = builder.ResolvedTaskResourcesInputs

	// ResolvedTaskResourcesOutputs adds an output PipelineResource, with specified name, to the ResolvedTaskResources.
	ResolvedTaskResourcesOutputs = builder.ResolvedTaskResourcesOutputs
)
