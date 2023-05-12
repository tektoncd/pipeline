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

const (
	// PipelineRunControllerName holds the name of the PipelineRun controller
	PipelineRunControllerName = "PipelineRun"

	// PipelineControllerName holds the name of the Pipeline controller
	PipelineControllerName = "Pipeline"

	// TaskRunControllerName holds the name of the TaskRun controller
	TaskRunControllerName = "TaskRun"

	// TaskControllerName holds the name of the Task controller
	TaskControllerName = "Task"

	// ClusterTaskControllerName holds the name of the Task controller
	ClusterTaskControllerName = "ClusterTask"

	// RunControllerName holds the name of the Custom Task controller
	RunControllerName = "Run"

	// CustomRunControllerName holds the name of the CustomRun controller
	CustomRunControllerName = "CustomRun"
)
