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
)

// PipelineRunSpec defines the desired state of PipelineRun
type PipelineRunSpec struct {
	PipelineRef        PipelineRef        `json:"pipelineRef"`
	PipelineParamsRef  PipelineParamsRef  `json:"pipelineParamsRef"`
	PipelineTriggerRef PipelineTriggerRef `json:"triggerRef"`
}

// PipelineRef can be used to refer to a specific instance of a Pipeline.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type PipelineRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineParamsRef can be used to refer to a specific instance of a Pipeline.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type PipelineParamsRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineTriggerType indicates the mechanism by which this PipelineRun was created.
type PipelineTriggerType string

const (
	// PipelineTriggerTypeManual indicates that this PipelineRun was invoked manually by a user.
	PipelineTriggerTypeManual PipelineTriggerType = "manual"
)

// PipelineTriggerRef describes what triggered this Pipeline to run. It could be triggered manually,
// or it could have been some kind of external event (not yet designed).
type PipelineTriggerRef struct {
	Type PipelineTriggerType `json:"type"`
	// +optional
	Name string `json:"name,omitempty"`
}

// PipelineRunStatus defines the observed state of PipelineRun
type PipelineRunStatus struct {
	//+optional
	TaskRuns []PipelineTaskRun `json:"taskRuns,omitempty"`
	// If there is no version, that means use latest
	// +optional
	ResourceVersion []StandardResourceVersion `json:"resourceVersion,omitempty"`
	Conditions      []PipelineRunCondition    `json:"conditions"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRun is the Schema for the pipelineruns API
// +k8s:openapi-gen=true
type PipelineRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PipelineRunSpec `json:"spec,omitempty"`
	// +optional
	Status PipelineRunStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRunList contains a list of PipelineRun
type PipelineRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineRun `json:"items"`
}

// PipelineTaskRun reports the results of running a step in the Task. Each
// task has the potential to succeed or fail (based on the exit code)
// and produces logs.
type PipelineTaskRun struct {
	Name string `json:"name"`
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

	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}
