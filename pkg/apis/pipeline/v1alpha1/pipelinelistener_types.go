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

package v1alpha1

import (
	duckv1beta1 "github.com/knative/pkg/apis/duck/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	pipelineListenerControllerName = "PipelineListener"
)

// PipelineRunSpec defines the desired state of PipelineRun
type PipelineListenerSpec struct {
	PipelineRef PipelineRef `json:"pipelineRef"`
	// The port the listener will bind to
	Port int `json:"Port"`
	// The specific type of event the listener will handle
	EventType string `json:"eventtype"`
	// The mechanism the listener will use to handle messages; defaults to cloudevent
	Event string `json:"event"`
	// The namespace the listener and pipelineruns should be created in
	Namespace string `json:"namespace"`
	// The spec of the desired pipeline run
	PipelineRunSpec PipelineRunSpec `json:"pipelinerunspec"`
	// The status of the listener
	PipelineListenerSpecStatus string `json:"pipelinespecstatus"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineListener defines listener status.
// +k8s:openapi-gen=true
type PipelineListener struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec PipelineListenerSpec `json:"spec,omitempty"`
	// +optional
	Status PipelineListenerStatus `json:"status,omitempty"`
}

// PipelineListenerSpecStatus defines the pipelinerun spec status the user can provide
type PipelineListenerSpecStatus string

// PipelineListenerStatus defines the observed state of PipelineListenerStatus
type PipelineListenerStatus struct {
	duckv1beta1.Status `json:",inline"`
	// namespace of the listener
	Namespace string `json:"namespace"`
	// statefulset name of the listeners set
	StatefulSetName string `json:"statefulsetname"`
	// +optional
	Results *Results `json:"results,omitempty"`
	// map of PipelineRunTaskRunStatus with the taskRun name as the key
	// +optional
	PipelineRuns map[string]*PipelineRunStatus `json:"pipelineRuns,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRunList contains a list of PipelineRun
type PipelineListenerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineListener `json:"items"`
}
