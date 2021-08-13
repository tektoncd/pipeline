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

package pipeline

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	// GroupName is the Kubernetes resource group name for Pipeline types.
	GroupName = "tekton.dev"

	// ClusterTaskLabelKey is used as the label identifier for a ClusterTask
	ClusterTaskLabelKey = GroupName + "/clusterTask"

	// TaskLabelKey is used as the label identifier for a Task
	TaskLabelKey = GroupName + "/task"

	// TaskRunLabelKey is used as the label identifier for a TaskRun
	TaskRunLabelKey = GroupName + "/taskRun"

	// PipelineLabelKey is used as the label identifier for a Pipeline
	PipelineLabelKey = GroupName + "/pipeline"

	// PipelineRunLabelKey is used as the label identifier for a PipelineRun
	PipelineRunLabelKey = GroupName + "/pipelineRun"

	// PipelineTaskLabelKey is used as the label identifier for a PipelineTask
	PipelineTaskLabelKey = GroupName + "/pipelineTask"

	// ConditionCheckKey is used as the label identifier for a ConditionCheck
	ConditionCheckKey = GroupName + "/conditionCheck"

	// ConditionNameKey is used as the label identifier for a Condition
	ConditionNameKey = GroupName + "/conditionName"

	// RunKey is used as the label identifier for a Run
	RunKey = GroupName + "/run"

	// MemberOfLabelKey is used as the label identifier for a PipelineTask
	// Set to Tasks/Finally depending on the position of the PipelineTask
	MemberOfLabelKey = GroupName + "/memberOf"
)

var (
	// TaskResource represents a Tekton Task
	TaskResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "tasks",
	}
	// ClusterTaskResource represents a Tekton ClusterTask
	ClusterTaskResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "clustertasks",
	}
	// TaskRunResource represents a Tekton TaskRun
	TaskRunResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "taskruns",
	}
	// RunResource represents a Tekton Run
	RunResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "runs",
	}
	// PipelineResource represents a Tekton Pipeline
	PipelineResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "pipelines",
	}
	// PipelineRunResource represents a Tekton PipelineRun
	PipelineRunResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "pipelineruns",
	}

	// PipelineResourceResource represents a Tekton PipelineResource
	PipelineResourceResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "pipelineresources",
	}
	// ConditionResource represents a Tekton Condition
	ConditionResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "conditions",
	}
)
