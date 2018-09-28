/*
Copyright 2018 The Knative Authors

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

package constants

const (
	//PipelineRunAgent is the logging agent for pipeline run controller
	PipelineRunAgent = "pipelinerun-controller"

	//TaskRunAgent is the logging agent for task run controller
	TaskRunAgent = "taskrun-controller"

	//TaskAgent is the logging agent for task controller
	TaskAgent = "task-controller"

	// SuccessSyncedReason is used as part of the Event 'reason' when a resource is synced
	SuccessSyncedReason = "Synced"
	// ErrResourceExistsReason is used as part of the Event 'reason' when a resource fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExistsReason = "ErrResourceExists"

	// PipelineRunSynced is the message used for an Event fired when a PipelineResource
	// is synced successfully
	PipelineRunSynced = "PipelineRun synced successfully"

	// TaskRunSynced is the message used for an Event fired when a TaskRun
	// is synced successfully
	TaskRunSynced = "TaskRun synced successfully"

	// TaskSynced is the message used for an Event fired when a Task
	// is synced successfully
	TaskSynced = "Task synced successfully"
)
