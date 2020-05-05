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

// PipelineOp is an operation which modify a Pipeline struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineOp = v1alpha1.PipelineOp

// PipelineSpecOp is an operation which modify a PipelineSpec struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineSpecOp = v1alpha1.PipelineSpecOp

// PipelineTaskOp is an operation which modify a PipelineTask struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineTaskOp = v1alpha1.PipelineTaskOp

// PipelineRunOp is an operation which modify a PipelineRun struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineRunOp = v1alpha1.PipelineRunOp

// PipelineRunSpecOp is an operation which modify a PipelineRunSpec struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineRunSpecOp = v1alpha1.PipelineRunSpecOp

// PipelineResourceOp is an operation which modify a PipelineResource struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineResourceOp = v1alpha1.PipelineResourceOp

// PipelineResourceBindingOp is an operation which modify a PipelineResourceBinding struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineResourceBindingOp = v1alpha1.PipelineResourceBindingOp

// PipelineResourceSpecOp is an operation which modify a PipelineResourceSpec struct.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineResourceSpecOp = v1alpha1.PipelineResourceSpecOp

// PipelineTaskInputResourceOp is an operation which modifies a PipelineTaskInputResource.
// Deprecated: moved to internal/builder/v1alpha1
type PipelineTaskInputResourceOp = v1alpha1.PipelineTaskInputResourceOp

// PipelineRunStatusOp is an operation which modifies a PipelineRunStatus
// Deprecated: moved to internal/builder/v1alpha1
type PipelineRunStatusOp = v1alpha1.PipelineRunStatusOp

// PipelineTaskConditionOp is an operation which modifies a PipelineTaskCondition
// Deprecated: moved to internal/builder/v1alpha1
type PipelineTaskConditionOp = v1alpha1.PipelineTaskConditionOp

var (

	// Pipeline creates a Pipeline with default values.
	// Any number of Pipeline modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	Pipeline = v1alpha1.Pipeline

	// PipelineNamespace sets the namespace on the Pipeline
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineNamespace = v1alpha1.PipelineNamespace

	// PipelineSpec sets the PipelineSpec to the Pipeline.
	// Any number of PipelineSpec modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineSpec = v1alpha1.PipelineSpec

	// PipelineCreationTimestamp sets the creation time of the pipeline
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineCreationTimestamp = v1alpha1.PipelineCreationTimestamp

	// PipelineDescription sets the description of the pipeline
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineDescription = v1alpha1.PipelineDescription

	// PipelineRunCancelled sets the status to cancel to the TaskRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunCancelled = v1alpha1.PipelineRunCancelled

	// PipelineDeclaredResource adds a resource declaration to the Pipeline Spec,
	// with the specified name and type.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineDeclaredResource = v1alpha1.PipelineDeclaredResource

	// PipelineParamSpec adds a param, with specified name and type, to the PipelineSpec.
	// Any number of PipelineParamSpec modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineParamSpec = v1alpha1.PipelineParamSpec

	// PipelineTask adds a PipelineTask, with specified name and task name, to the PipelineSpec.
	// Any number of PipelineTask modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTask = v1alpha1.PipelineTask

	// PipelineResult adds a PipelineResult, with specified name, value and description, to the PipelineSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResult = v1alpha1.PipelineResult

	// PipelineRunResult adds a PipelineResultStatus, with specified name, value and description, to the PipelineRunStatusSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunResult = v1alpha1.PipelineRunResult

	// PipelineTaskSpec sets the TaskSpec on a PipelineTask.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskSpec = v1alpha1.PipelineTaskSpec

	// Retries sets the number of retries on a PipelineTask.
	// Deprecated: moved to internal/builder/v1alpha1
	Retries = v1alpha1.Retries

	// RunAfter will update the provided Pipeline Task to indicate that it
	// should be run after the provided list of Pipeline Task names.
	// Deprecated: moved to internal/builder/v1alpha1
	RunAfter = v1alpha1.RunAfter

	// PipelineTaskRefKind sets the TaskKind to the PipelineTaskRef.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskRefKind = v1alpha1.PipelineTaskRefKind

	// PipelineTaskParam adds a ResourceParam, with specified name and value, to the PipelineTask.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskParam = v1alpha1.PipelineTaskParam

	// From will update the provided PipelineTaskInputResource to indicate that it
	// should come from tasks.
	// Deprecated: moved to internal/builder/v1alpha1
	From = v1alpha1.From

	// PipelineTaskInputResource adds an input resource to the PipelineTask with the specified
	// name, pointing at the declared resource.
	// Any number of PipelineTaskInputResource modifies can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskInputResource = v1alpha1.PipelineTaskInputResource

	// PipelineTaskOutputResource adds an output resource to the PipelineTask with the specified
	// name, pointing at the declared resource.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskOutputResource = v1alpha1.PipelineTaskOutputResource

	// PipelineTaskCondition adds a condition to the PipelineTask with the
	// specified conditionRef. Any number of PipelineTaskCondition modifiers can be passed
	// to transform it
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskCondition = v1alpha1.PipelineTaskCondition

	// PipelineTaskConditionParam adds a parameter to a PipelineTaskCondition
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskConditionParam = v1alpha1.PipelineTaskConditionParam

	// PipelineTaskConditionResource adds a resource to a PipelineTaskCondition
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskConditionResource = v1alpha1.PipelineTaskConditionResource

	// PipelineTaskWorkspaceBinding adds a workspace with the specified name, workspace and subpath on a PipelineTask.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskWorkspaceBinding = v1alpha1.PipelineTaskWorkspaceBinding

	// PipelineTaskTimeout sets the timeout for the PipelineTask.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineTaskTimeout = v1alpha1.PipelineTaskTimeout

	// PipelineRun creates a PipelineRun with default values.
	// Any number of PipelineRun modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRun = v1alpha1.PipelineRun

	// PipelineRunNamespace sets the namespace on a PipelineRun.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunNamespace = v1alpha1.PipelineRunNamespace

	// PipelineRunSpec sets the PipelineRunSpec, references Pipeline with specified name, to the PipelineRun.
	// Any number of PipelineRunSpec modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunSpec = v1alpha1.PipelineRunSpec

	// PipelineRunLabel adds a label to the PipelineRun.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunLabel = v1alpha1.PipelineRunLabel

	// PipelineRunAnnotation adds a annotation to the PipelineRun.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunAnnotation = v1alpha1.PipelineRunAnnotation

	// PipelineRunResourceBinding adds bindings from actual instances to a Pipeline's declared resources.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunResourceBinding = v1alpha1.PipelineRunResourceBinding

	// PipelineResourceBindingRef set the ResourceRef name to the Resource called Name.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResourceBindingRef = v1alpha1.PipelineResourceBindingRef

	// PipelineResourceBindingResourceSpec set the PipelineResourceResourceSpec to the PipelineResourceBinding.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResourceBindingResourceSpec = v1alpha1.PipelineResourceBindingResourceSpec

	// PipelineRunServiceAccountName sets the service account to the PipelineRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunServiceAccountName = v1alpha1.PipelineRunServiceAccountName

	// PipelineRunServiceAccountNameTask configures the service account for given Task in PipelineRun.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunServiceAccountNameTask = v1alpha1.PipelineRunServiceAccountNameTask

	// PipelineRunParam add a param, with specified name and value, to the PipelineRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunParam = v1alpha1.PipelineRunParam

	// PipelineRunTimeout sets the timeout to the PipelineRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunTimeout = v1alpha1.PipelineRunTimeout

	// PipelineRunNilTimeout sets the timeout to nil on the PipelineRunSpec
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunNilTimeout = v1alpha1.PipelineRunNilTimeout

	// PipelineRunNodeSelector sets the Node selector to the PipelineRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunNodeSelector = v1alpha1.PipelineRunNodeSelector

	// PipelineRunTolerations sets the Node selector to the PipelineRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunTolerations = v1alpha1.PipelineRunTolerations

	// PipelineRunAffinity sets the affinity to the PipelineRunSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunAffinity = v1alpha1.PipelineRunAffinity

	// PipelineRunPipelineSpec adds a PipelineSpec to the PipelineRunSpec.
	// Any number of PipelineSpec modifiers can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunPipelineSpec = v1alpha1.PipelineRunPipelineSpec

	// PipelineRunStatus sets the PipelineRunStatus to the PipelineRun.
	// Any number of PipelineRunStatus modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunStatus = v1alpha1.PipelineRunStatus

	// PipelineRunStatusCondition adds a StatusCondition to the TaskRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunStatusCondition = v1alpha1.PipelineRunStatusCondition

	// PipelineRunStartTime sets the start time to the PipelineRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunStartTime = v1alpha1.PipelineRunStartTime

	// PipelineRunCompletionTime sets the completion time  to the PipelineRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunCompletionTime = v1alpha1.PipelineRunCompletionTime

	// PipelineRunTaskRunsStatus sets the status of TaskRun to the PipelineRunStatus.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunTaskRunsStatus = v1alpha1.PipelineRunTaskRunsStatus

	// PipelineResource creates a PipelineResource with default values.
	// Any number of PipelineResource modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResource = v1alpha1.PipelineResource

	// PipelineResourceNamespace sets the namespace on a PipelineResource.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResourceNamespace = v1alpha1.PipelineResourceNamespace

	// PipelineResourceSpec set the PipelineResourceSpec, with specified type, to the PipelineResource.
	// Any number of PipelineResourceSpec modifier can be passed to transform it.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResourceSpec = v1alpha1.PipelineResourceSpec

	// PipelineResourceDescription sets the description of the pipeline resource
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResourceDescription = v1alpha1.PipelineResourceDescription

	// PipelineResourceSpecParam adds a ResourceParam, with specified name and value, to the PipelineResourceSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResourceSpecParam = v1alpha1.PipelineResourceSpecParam

	// PipelineResourceSpecSecretParam adds a SecretParam, with specified fieldname, secretKey and secretName, to the PipelineResourceSpec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineResourceSpecSecretParam = v1alpha1.PipelineResourceSpecSecretParam

	// PipelineWorkspaceDeclaration adds a Workspace to the workspaces listed in the pipeline spec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineWorkspaceDeclaration = v1alpha1.PipelineWorkspaceDeclaration

	// PipelineRunWorkspaceBindingEmptyDir adds an EmptyDir Workspace to the workspaces of a pipelinerun spec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunWorkspaceBindingEmptyDir = v1alpha1.PipelineRunWorkspaceBindingEmptyDir

	// PipelineRunWorkspaceBindingVolumeClaimTemplate adds an VolumeClaimTemplate Workspace to the workspaces of a pipelineRun spec.
	// Deprecated: moved to internal/builder/v1alpha1
	PipelineRunWorkspaceBindingVolumeClaimTemplate = v1alpha1.PipelineRunWorkspaceBindingVolumeClaimTemplate
)
