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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PipelineRunSpec defines the desired state of PipelineRun
type PipelineRunSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// PipelineRunStatus defines the observed state of PipelineRun
type PipelineRunStatus struct {
	TaskRuns   []PipelineTaskRun      `json:"taskRuns,omitempty"`
	Conditions []PipelineRunCondition `json:"conditions"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRun is the Schema for the pipelineruns API
// +k8s:openapi-gen=true
type PipelineRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineRunSpec   `json:"spec,omitempty"`
	Status PipelineRunStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRunList contains a list of PipelineRun
type PipelineRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineRun `json:"items"`
}

// PipelineTaskRun reports the results of running a step in the Task. Each
// task has the potential to succeed or fail (based on the exit code)
// and produces logs.
type PipelineTaskRun struct {
	Name string `json:"name"`
}

// PipelineTaskRunRef refers to a TaskRun and also indicates which TaskRuns
// executed before and after it.
type PipelineTaskRunRef struct {
	TaskRunRef
	NextTasks []TaskRunRef `json:"nextTasks"`
	PrevTasks []TaskRunRef `json:"prevTasks"`
}

// TaskRunRef can be used to refer to a specific instance of a TaskRun.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type TaskRunRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineRunConditionType indicates the status of the execution of the PipelineRun.
type PipelineRunConditionType string

const (
	// PipelineRunConditionTypeStarted indicates whether or not the PipelineRun
	// has started actually executing.
	PipelineRunConditionTypeStarted PipelineRunConditionType = "Started"

	//PipelineRunConditionTypeCompleted indicates whether or not the PipelineRun
	// has finished executing.
	PipelineRunConditionTypeCompleted PipelineRunConditionType = "Completed"

	// PipelineRunConditionTypeSucceeded indicates whether or not the PipelineRun
	// was successful.
	PipelineRunConditionTypeSucceeded PipelineRunConditionType = "Successful"
)

// PipelineRunCondition holds a Condition that the PipelineRun has entered into while being executed.
type PipelineRunCondition struct {
	Type PipelineRunConditionType `json:"type"`

	Status corev1.ConditionStatus `json:"status"`

	// TODO(bobcatfish): when I use `metav1.Time` here I can't figure out
	// how to serialize :(
	LastTransitionTime string `json:"lastTransitionTime"`

	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

func init() {
	SchemeBuilder.Register(&PipelineRun{}, &PipelineRunList{})
}
