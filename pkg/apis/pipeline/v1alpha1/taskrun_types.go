/*
Copyright 2018 The Knative Authors.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
)

// TaskRunSpec defines the desired state of TaskRun
type TaskRunSpec struct {
	TaskRef TaskRef     `json:"taskRef"`
	Trigger TaskTrigger `json:"trigger"`
	// +optional
	Inputs TaskRunInputs `json:"inputs,omitempty"`
	// +optional
	Outputs Outputs `json:"outputs,omitempty"`
	Results Results `json:"results"`
}

// TaskRunInputs holds the input values that this task was invoked with.
type TaskRunInputs struct {
	Resources []StandardResourceVersion `json:"standardResourcesVersion"`
	// +optional
	Params []Param `json:"params,omitempty"`
}

// TaskTrigger defines a webhook style trigger to start a TaskRun
type TaskTrigger struct {
	TriggerRef TaskTriggerRef `json:"triggerRef"`
}

// TaskTriggerType indicates the mechanism by which this TaskRun was created.
type TaskTriggerType string

const (
	// TaskTriggerTypeManual indicates that this TaskRun was invoked manually by a user.
	TaskTriggerTypeManual TaskTriggerType = "manual"

	// TaskTriggerTypePipelineRun indicates that this TaskRun was created by a controller
	// attempting to realize a PipelineRun. In this case the `name` will refer to the name
	// of the PipelineRun.
	TaskTriggerTypePipelineRun TaskTriggerType = "pipelineRun"
)

// TaskTriggerRef describes what triggered this Task to run. It could be triggered manually,
// or it may have been part of a PipelineRun in which case this ref would refer
// to the corresponding PipelineRun.
type TaskTriggerRef struct {
	Type TaskTriggerType `json:"type"`
	// +optional
	Name string `json:"name,omitempty"`
}

// TaskRunStatus defines the observed state of TaskRun
// TODO(aaron-prindle) change back to being used in Task
type TaskRunStatus struct {
	Steps      []StepRun          `json:"steps"`
	Conditions []TaskRunCondition `json:"conditions"`
}

// StepRun reports the results of running a step in the Task. Each
// task has the potential to succeed or fail (based on the exit code)
// and produces logs.
type StepRun struct {
	Name     string `json:"name"`
	LogsURL  string `json:"logsURL"`
	ExitCode int    `json:"exitCode"`
}

// TaskRunConditionType indicates the status of the execution of the TaskRun.
type TaskRunConditionType string

const (
	// TaskRunConditionTypeStarted indicates whether or not the TaskRun
	// has started actually executing.
	TaskRunConditionTypeStarted TaskRunConditionType = "Started"

	//TaskRunConditionTypeCompleted indicates whether or not the TaskRun
	// has finished executing.
	TaskRunConditionTypeCompleted TaskRunConditionType = "Completed"

	// TaskRunConditionTypeSucceeded indicates whether or not the TaskRun
	// was successful.
	TaskRunConditionTypeSucceeded TaskRunConditionType = "Successful"
)

// TaskRunCondition holds a Condition that the TaskRun has entered into while being executed.
type TaskRunCondition struct {
	Type TaskRunConditionType `json:"type"`

	Status corev1.ConditionStatus `json:"status"`

	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskRun is the Schema for the taskruns API
// +k8s:openapi-gen=true
type TaskRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec TaskRunSpec `json:"spec,omitempty"`
	// +optional
	//TODO(aaron-prindle) change back to TaskRunStatus
	Status buildv1alpha1.BuildStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskRunList contains a list of TaskRun
type TaskRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TaskRun `json:"items"`
}
