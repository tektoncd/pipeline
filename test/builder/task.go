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

// TaskOp is an operation which modify a Task struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskOp = v1alpha1.TaskOp

// ClusterTaskOp is an operation which modify a ClusterTask struct.
// Deprecated: moved to internal/builder/v1alpha1
type ClusterTaskOp = v1alpha1.ClusterTaskOp

// TaskSpecOp is an operation which modify a TaskSpec struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskSpecOp = v1alpha1.TaskSpecOp

// TaskResourcesOp is an operation which modify a TaskResources struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskResourcesOp = v1alpha1.TaskResourcesOp

// InputsOp is an operation which modify an Inputs struct.
// Deprecated: moved to internal/builder/v1alpha1
type InputsOp = v1alpha1.InputsOp

// OutputsOp is an operation which modify an Outputs struct.
// Deprecated: moved to internal/builder/v1alpha1
type OutputsOp = v1alpha1.OutputsOp

// TaskRunOp is an operation which modify a TaskRun struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskRunOp = v1alpha1.TaskRunOp

// TaskRunSpecOp is an operation which modify a TaskRunSpec struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskRunSpecOp = v1alpha1.TaskRunSpecOp

// TaskRunResourcesOp is an operation which modify a TaskRunResources struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskRunResourcesOp = v1alpha1.TaskRunResourcesOp

// TaskResourceOp is an operation which modify a TaskResource struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskResourceOp = v1alpha1.TaskResourceOp

// TaskResourceBindingOp is an operation which modify a TaskResourceBinding struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskResourceBindingOp = v1alpha1.TaskResourceBindingOp

// TaskRunStatusOp is an operation which modify a TaskRunStatus struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskRunStatusOp = v1alpha1.TaskRunStatusOp

// TaskRefOp is an operation which modify a TaskRef struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskRefOp = v1alpha1.TaskRefOp

// TaskResultOp is an operation which modifies there
// Deprecated: moved to internal/builder/v1alpha1
type TaskResultOp = v1alpha1.TaskResultOp

// TaskRunInputsOp is an operation which modify a TaskRunInputs struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskRunInputsOp = v1alpha1.TaskRunInputsOp

// TaskRunOutputsOp is an operation which modify a TaskRunOutputs struct.
// Deprecated: moved to internal/builder/v1alpha1
type TaskRunOutputsOp = v1alpha1.TaskRunOutputsOp

// StepStateOp is an operation which modifies a StepState struct.
// Deprecated: moved to internal/builder/v1alpha1
type StepStateOp = v1alpha1.StepStateOp

// SidecarStateOp is an operation which modifies a SidecarState struct.
// Deprecated: moved to internal/builder/v1alpha1
type SidecarStateOp = v1alpha1.SidecarStateOp

// VolumeOp is an operation which modify a Volume struct.
// Deprecated: moved to internal/builder/v1alpha1
type VolumeOp = v1alpha1.VolumeOp

var (

	// Task creates a Task with default values.
	// Any number of Task modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	Task = v1alpha1.Task

	// TaskType sets the TypeMeta on the Task which is useful for making it serializable/deserializable.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskType = v1alpha1.TaskType

	// ClusterTask creates a ClusterTask with default values.
	// Any number of ClusterTask modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	ClusterTask = v1alpha1.ClusterTask

	// TaskNamespace sets the namespace on the task.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskNamespace = v1alpha1.TaskNamespace

	// ClusterTaskType sets the TypeMeta on the ClusterTask which is useful for making it serializable/deserializable.
	// Deprecated: moved to internal/builder/v1alpha1
	ClusterTaskType = v1alpha1.ClusterTaskType

	// ClusterTaskSpec sets the specified spec of the cluster task.
	// Any number of TaskSpec modifier can be passed to create it.
	// Deprecated: moved to internal/builder/v1alpha1
	ClusterTaskSpec = v1alpha1.ClusterTaskSpec

	// TaskSpec sets the specified spec of the task.
	// Any number of TaskSpec modifier can be passed to create/modify it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskSpec = v1alpha1.TaskSpec

	// TaskDescription sets the description of the task
	// Deprecated: moved to internal/builder/v1alpha1
	TaskDescription = v1alpha1.TaskDescription

	// Step adds a step with the specified name and image to the TaskSpec.
	// Any number of Container modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	Step = v1alpha1.Step

	// Sidecar adds a sidecar container with the specified name and image to the TaskSpec.
	// Any number of Container modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	Sidecar = v1alpha1.Sidecar

	// TaskWorkspace adds a workspace declaration.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskWorkspace = v1alpha1.TaskWorkspace

	// TaskStepTemplate adds a base container for all steps in the task.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskStepTemplate = v1alpha1.TaskStepTemplate

	// TaskVolume adds a volume with specified name to the TaskSpec.
	// Any number of Volume modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskVolume = v1alpha1.TaskVolume

	// VolumeSource sets the VolumeSource to the Volume.
	// Deprecated: moved to internal/builder/v1alpha1
	VolumeSource = v1alpha1.VolumeSource

	// TaskParam sets the Params to the TaskSpec
	// Deprecated: moved to internal/builder/v1alpha1
	TaskParam = v1alpha1.TaskParam

	// TaskResources sets the Resources to the TaskSpec
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResources = v1alpha1.TaskResources

	// TaskResults sets the Results to the TaskSpec
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResults = v1alpha1.TaskResults

	// TaskResourcesInput adds a TaskResource as Inputs to the TaskResources
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResourcesInput = v1alpha1.TaskResourcesInput

	// TaskResourcesOutput adds a TaskResource as Outputs to the TaskResources
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResourcesOutput = v1alpha1.TaskResourcesOutput

	// TaskResultsOutput adds a TaskResult as Outputs to the TaskResources
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResultsOutput = v1alpha1.TaskResultsOutput

	// TaskInputs sets inputs to the TaskSpec.
	// Any number of Inputs modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskInputs = v1alpha1.TaskInputs

	// TaskOutputs sets inputs to the TaskSpec.
	// Any number of Outputs modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskOutputs = v1alpha1.TaskOutputs

	// InputsResource adds a resource, with specified name and type, to the Inputs.
	// Any number of TaskResource modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	InputsResource = v1alpha1.InputsResource

	// ResourceOptional marks a TaskResource as optional.
	// Deprecated: moved to internal/builder/v1alpha1
	ResourceOptional = v1alpha1.ResourceOptional

	// ResourceTargetPath sets the target path to a TaskResource.
	// Deprecated: moved to internal/builder/v1alpha1
	ResourceTargetPath = v1alpha1.ResourceTargetPath

	// OutputsResource adds a resource, with specified name and type, to the Outputs.
	// Deprecated: moved to internal/builder/v1alpha1
	OutputsResource = v1alpha1.OutputsResource

	// InputsParamSpec adds a ParamSpec, with specified name and type, to the Inputs.
	// Any number of TaskParamSpec modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	InputsParamSpec = v1alpha1.InputsParamSpec

	// TaskRun creates a TaskRun with default values.
	// Any number of TaskRun modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRun = v1alpha1.TaskRun

	// TaskRunNamespace sets the namespace for the TaskRun.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunNamespace = v1alpha1.TaskRunNamespace

	// TaskRunStatus sets the TaskRunStatus to tshe TaskRun
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunStatus = v1alpha1.TaskRunStatus

	// PodName sets the Pod name to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	PodName = v1alpha1.PodName

	// StatusCondition adds a StatusCondition to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	StatusCondition = v1alpha1.StatusCondition

	// TaskRunResult adds a result with the specified name and value to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunResult = v1alpha1.TaskRunResult

	// Retry adds a RetriesStatus (TaskRunStatus) to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	Retry = v1alpha1.Retry

	// StepState adds a StepState to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	StepState = v1alpha1.StepState

	// SidecarState adds a SidecarState to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	SidecarState = v1alpha1.SidecarState

	// TaskRunStartTime sets the start time to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunStartTime = v1alpha1.TaskRunStartTime

	// TaskRunCompletionTime sets the start time to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunCompletionTime = v1alpha1.TaskRunCompletionTime

	// TaskRunCloudEvent adds an event to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunCloudEvent = v1alpha1.TaskRunCloudEvent

	// TaskRunTimeout sets the timeout duration to the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunTimeout = v1alpha1.TaskRunTimeout

	// TaskRunNilTimeout sets the timeout duration to nil on the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunNilTimeout = v1alpha1.TaskRunNilTimeout

	// TaskRunNodeSelector sets the NodeSelector to the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunNodeSelector = v1alpha1.TaskRunNodeSelector

	// TaskRunTolerations sets the Tolerations to the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunTolerations = v1alpha1.TaskRunTolerations

	// TaskRunAffinity sets the Affinity to the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunAffinity = v1alpha1.TaskRunAffinity

	// TaskRunPodSecurityContext sets the SecurityContext to the TaskRunSpec (through PodTemplate).
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunPodSecurityContext = v1alpha1.TaskRunPodSecurityContext

	// StateTerminated sets Terminated to the StepState.
	// Deprecated: moved to internal/builder/v1alpha1
	StateTerminated = v1alpha1.StateTerminated

	// SetStepStateTerminated sets Terminated state of a step.
	// Deprecated: moved to internal/builder/v1alpha1
	SetStepStateTerminated = v1alpha1.SetStepStateTerminated

	// SetStepStateRunning sets Running state of a step.
	// Deprecated: moved to internal/builder/v1alpha1
	SetStepStateRunning = v1alpha1.SetStepStateRunning

	// SetStepStateWaiting sets Waiting state of a step.
	// Deprecated: moved to internal/builder/v1alpha1
	SetStepStateWaiting = v1alpha1.SetStepStateWaiting

	// TaskRunOwnerReference sets the OwnerReference, with specified kind and name, to the TaskRun.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunOwnerReference = v1alpha1.TaskRunOwnerReference

	// TaskRunLabels add the specified labels to the TaskRun.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunLabels = v1alpha1.TaskRunLabels

	// TaskRunLabel adds a label with the specified key and value to the TaskRun.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunLabel = v1alpha1.TaskRunLabel

	// TaskRunAnnotation adds an annotation with the specified key and value to the TaskRun.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunAnnotation = v1alpha1.TaskRunAnnotation

	// TaskRunSelfLink adds a SelfLink
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunSelfLink = v1alpha1.TaskRunSelfLink

	// TaskRunSpec sets the specified spec of the TaskRun.
	// Any number of TaskRunSpec modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunSpec = v1alpha1.TaskRunSpec

	// TaskRunCancelled sets the status to cancel to the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunCancelled = v1alpha1.TaskRunCancelled

	// TaskRunTaskRef sets the specified Task reference to the TaskRunSpec.
	// Any number of TaskRef modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunTaskRef = v1alpha1.TaskRunTaskRef

	// TaskRunSpecStatus sets the Status in the Spec, used for operations
	// such as cancelling executing TaskRuns.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunSpecStatus = v1alpha1.TaskRunSpecStatus

	// TaskRefKind set the specified kind to the TaskRef.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRefKind = v1alpha1.TaskRefKind

	// TaskRefAPIVersion sets the specified api version to the TaskRef.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRefAPIVersion = v1alpha1.TaskRefAPIVersion

	// TaskRunTaskSpec sets the specified TaskRunSpec reference to the TaskRunSpec.
	// Any number of TaskRunSpec modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunTaskSpec = v1alpha1.TaskRunTaskSpec

	// TaskRunServiceAccountName sets the serviceAccount to the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunServiceAccountName = v1alpha1.TaskRunServiceAccountName

	// TaskRunParam sets the Params to the TaskSpec
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunParam = v1alpha1.TaskRunParam

	// TaskRunResources sets the TaskRunResources to the TaskRunSpec
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunResources = v1alpha1.TaskRunResources

	// TaskRunResourcesInput adds a TaskRunResource as Inputs to the TaskRunResources
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunResourcesInput = v1alpha1.TaskRunResourcesInput

	// TaskRunResourcesOutput adds a TaskRunResource as Outputs to the TaskRunResources
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunResourcesOutput = v1alpha1.TaskRunResourcesOutput

	// TaskRunInputs sets inputs to the TaskRunSpec.
	// Any number of TaskRunInputs modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunInputs = v1alpha1.TaskRunInputs

	// TaskRunInputsParam add a param, with specified name and value, to the TaskRunInputs.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunInputsParam = v1alpha1.TaskRunInputsParam

	// TaskRunInputsResource adds a resource, with specified name, to the TaskRunInputs.
	// Any number of TaskResourceBinding modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunInputsResource = v1alpha1.TaskRunInputsResource

	// TaskResourceBindingRef set the PipelineResourceRef name to the TaskResourceBinding.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResourceBindingRef = v1alpha1.TaskResourceBindingRef

	// TaskResourceBindingResourceSpec set the PipelineResourceResourceSpec to the TaskResourceBinding.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResourceBindingResourceSpec = v1alpha1.TaskResourceBindingResourceSpec

	// TaskResourceBindingRefAPIVersion set the PipelineResourceRef APIVersion to the TaskResourceBinding.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResourceBindingRefAPIVersion = v1alpha1.TaskResourceBindingRefAPIVersion

	// TaskResourceBindingPaths add any number of path to the TaskResourceBinding.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskResourceBindingPaths = v1alpha1.TaskResourceBindingPaths

	// TaskRunOutputs sets inputs to the TaskRunSpec.
	// Any number of TaskRunOutputs modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunOutputs = v1alpha1.TaskRunOutputs

	// TaskRunOutputsResource adds a TaskResourceBinding, with specified name, to the TaskRunOutputs.
	// Any number of TaskResourceBinding modifier can be passed to modifiy it.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunOutputsResource = v1alpha1.TaskRunOutputsResource

	// TaskRunWorkspaceEmptyDir adds a workspace binding to an empty dir volume source.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunWorkspaceEmptyDir = v1alpha1.TaskRunWorkspaceEmptyDir

	// TaskRunWorkspacePVC adds a workspace binding to a PVC volume source.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunWorkspacePVC = v1alpha1.TaskRunWorkspacePVC

	// TaskRunWorkspaceVolumeClaimTemplate adds a workspace binding with a VolumeClaimTemplate volume source.
	// Deprecated: moved to internal/builder/v1alpha1
	TaskRunWorkspaceVolumeClaimTemplate = v1alpha1.TaskRunWorkspaceVolumeClaimTemplate
)
