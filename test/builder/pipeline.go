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

// PipelineOp is an operation which modify a Pipeline struct.
type PipelineOp = builder.PipelineOp

// PipelineSpecOp is an operation which modify a PipelineSpec struct.
type PipelineSpecOp = builder.PipelineSpecOp

// PipelineTaskOp is an operation which modify a PipelineTask struct.
type PipelineTaskOp = builder.PipelineTaskOp

// PipelineRunOp is an operation which modify a PipelineRun struct.
type PipelineRunOp = builder.PipelineRunOp

// PipelineRunSpecOp is an operation which modify a PipelineRunSpec struct.
type PipelineRunSpecOp = builder.PipelineRunSpecOp

// PipelineResourceOp is an operation which modify a PipelineResource struct.
type PipelineResourceOp = builder.PipelineResourceOp

// PipelineResourceBindingOp is an operation which modify a PipelineResourceBinding struct.
type PipelineResourceBindingOp = builder.PipelineResourceBindingOp

// PipelineResourceSpecOp is an operation which modify a PipelineResourceSpec struct.
type PipelineResourceSpecOp = builder.PipelineResourceSpecOp

// PipelineTaskInputResourceOp is an operation which modifies a PipelineTaskInputResource.
type PipelineTaskInputResourceOp = builder.PipelineTaskInputResourceOp

// PipelineRunStatusOp is an operation which modifies a PipelineRunStatus
type PipelineRunStatusOp = builder.PipelineRunStatusOp

// PipelineTaskConditionOp is an operation which modifies a PipelineTaskCondition
type PipelineTaskConditionOp = builder.PipelineTaskConditionOp

var (

	// Pipeline creates a Pipeline with default values.
	// Any number of Pipeline modifier can be passed to transform it.
	Pipeline = builder.Pipeline

	// PipelineSpec sets the PipelineSpec to the Pipeline.
	// Any number of PipelineSpec modifier can be passed to transform it.
	PipelineSpec = builder.PipelineSpec

	// PipelineCreationTimestamp sets the creation time of the pipeline
	PipelineCreationTimestamp = builder.PipelineCreationTimestamp

	// PipelineRunCancelled sets the status to cancel to the TaskRunSpec.
	PipelineRunCancelled = builder.PipelineRunCancelled

	// PipelineDeclaredResource adds a resource declaration to the Pipeline Spec,
	// with the specified name and type.
	PipelineDeclaredResource = builder.PipelineDeclaredResource

	// PipelineParamSpec adds a param, with specified name and type, to the PipelineSpec.
	// Any number of PipelineParamSpec modifiers can be passed to transform it.
	PipelineParamSpec = builder.PipelineParamSpec

	// PipelineTask adds a PipelineTask, with specified name and task name, to the PipelineSpec.
	// Any number of PipelineTask modifier can be passed to transform it.
	PipelineTask = builder.PipelineTask

	Retries = builder.Retries

	// RunAfter will update the provided Pipeline Task to indicate that it
	// should be run after the provided list of Pipeline Task names.
	RunAfter = builder.RunAfter

	// PipelineTaskRefKind sets the TaskKind to the PipelineTaskRef.
	PipelineTaskRefKind = builder.PipelineTaskRefKind

	// PipelineTaskParam adds a ResourceParam, with specified name and value, to the PipelineTask.
	PipelineTaskParam = builder.PipelineTaskParam

	// From will update the provided PipelineTaskInputResource to indicate that it
	// should come from tasks.
	From = builder.From

	// PipelineTaskInputResource adds an input resource to the PipelineTask with the specified
	// name, pointing at the declared resource.
	// Any number of PipelineTaskInputResource modifies can be passed to transform it.
	PipelineTaskInputResource = builder.PipelineTaskInputResource

	// PipelineTaskOutputResource adds an output resource to the PipelineTask with the specified
	// name, pointing at the declared resource.
	PipelineTaskOutputResource = builder.PipelineTaskOutputResource

	// PipelineTaskCondition adds a condition to the PipelineTask with the
	// specified conditionRef. Any number of PipelineTaskCondition modifiers can be passed
	// to transform it
	PipelineTaskCondition = builder.PipelineTaskCondition

	// PipelineTaskConditionParam adds a parameter to a PipelineTaskCondition
	PipelineTaskConditionParam = builder.PipelineTaskConditionParam

	// PipelineTaskConditionResource adds a resource to a PipelineTaskCondition
	PipelineTaskConditionResource = builder.PipelineTaskConditionResource

	// PipelineRun creates a PipelineRun with default values.
	// Any number of PipelineRun modifier can be passed to transform it.
	PipelineRun = builder.PipelineRun

	// PipelineRunSpec sets the PipelineRunSpec, references Pipeline with specified name, to the PipelineRun.
	// Any number of PipelineRunSpec modifier can be passed to transform it.
	PipelineRunSpec = builder.PipelineRunSpec

	// PipelineRunLabels adds a label to the PipelineRun.
	PipelineRunLabel = builder.PipelineRunLabel

	// PipelineRunAnnotations adds a annotation to the PipelineRun.
	PipelineRunAnnotation = builder.PipelineRunAnnotation

	// PipelineRunResourceBinding adds bindings from actual instances to a Pipeline's declared resources.
	PipelineRunResourceBinding = builder.PipelineRunResourceBinding

	// PipelineResourceBindingRef set the ResourceRef name to the Resource called Name.
	PipelineResourceBindingRef = builder.PipelineResourceBindingRef

	// PipelineResourceBindingResourceSpec set the PipelineResourceResourceSpec to the PipelineResourceBinding.
	PipelineResourceBindingResourceSpec = builder.PipelineResourceBindingResourceSpec

	// PipelineRunServiceAccount sets the service account to the PipelineRunSpec.
	PipelineRunServiceAccountName = builder.PipelineRunServiceAccountName

	// PipelineRunServiceAccountTask configures the service account for given Task in PipelineRun.
	PipelineRunServiceAccountNameTask = builder.PipelineRunServiceAccountNameTask

	// PipelineRunParam add a param, with specified name and value, to the PipelineRunSpec.
	PipelineRunParam = builder.PipelineRunParam

	// PipelineRunTimeout sets the timeout to the PipelineRunSpec.
	PipelineRunTimeout = builder.PipelineRunTimeout

	// PipelineRunNilTimeout sets the timeout to nil on the PipelineRunSpec
	PipelineRunNilTimeout = builder.PipelineRunNilTimeout

	// PipelineRunNodeSelector sets the Node selector to the PipelineSpec.
	PipelineRunNodeSelector = builder.PipelineRunNodeSelector

	// PipelineRunTolerations sets the Node selector to the PipelineSpec.
	PipelineRunTolerations = builder.PipelineRunTolerations

	// PipelineRunAffinity sets the affinity to the PipelineSpec.
	PipelineRunAffinity = builder.PipelineRunAffinity

	// PipelineRunStatus sets the PipelineRunStatus to the PipelineRun.
	// Any number of PipelineRunStatus modifier can be passed to transform it.
	PipelineRunStatus = builder.PipelineRunStatus

	// PipelineRunStatusCondition adds a StatusCondition to the TaskRunStatus.
	PipelineRunStatusCondition = builder.PipelineRunStatusCondition

	// PipelineRunStartTime sets the start time to the PipelineRunStatus.
	PipelineRunStartTime = builder.PipelineRunStartTime

	// PipelineRunCompletionTime sets the completion time  to the PipelineRunStatus.
	PipelineRunCompletionTime = builder.PipelineRunCompletionTime

	// PipelineRunTaskRunsStatus sets the status of TaskRun to the PipelineRunStatus.
	PipelineRunTaskRunsStatus = builder.PipelineRunTaskRunsStatus

	// PipelineResource creates a PipelineResource with default values.
	// Any number of PipelineResource modifier can be passed to transform it.
	PipelineResource = builder.PipelineResource

	// PipelineResourceSpec set the PipelineResourceSpec, with specified type, to the PipelineResource.
	// Any number of PipelineResourceSpec modifier can be passed to transform it.
	PipelineResourceSpec = builder.PipelineResourceSpec

	// PipelineResourceSpecParam adds a ResourceParam, with specified name and value, to the PipelineResourceSpec.
	PipelineResourceSpecParam = builder.PipelineResourceSpecParam

	// PipelineResourceSpecSecretParam adds a SecretParam, with specified fieldname, secretKey and secretName, to the PipelineResourceSpec.
	PipelineResourceSpecSecretParam = builder.PipelineResourceSpecSecretParam
)
