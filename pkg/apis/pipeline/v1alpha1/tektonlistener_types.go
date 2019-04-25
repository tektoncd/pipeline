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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	tektonListenerControllerName = "TektonListener"
)

// TektonListenerSpec defines the desired state of PipelineRun
type TektonListenerSpec struct {
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
	PipelineRunSpec *PipelineRunSpec `json:"pipelinerunspec"`
	// The status of the listener
	TektonListenerSpecStatus string `json:"pipelinespecstatus"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TektonListener defines listener status.
// +k8s:openapi-gen=true
type TektonListener struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec TektonListenerSpec `json:"spec,omitempty"`
	// +optional
	Status TektonListenerStatus `json:"status,omitempty"`
}

// TektonListenerSpecStatus defines the pipelinerun spec status the user can provide
type TektonListenerSpecStatus string

// TektonListenerStatus defines the observed state of TektonListenerStatus
type TektonListenerStatus struct {
	duckv1alpha1.Status `json:",inline"`
	// namespace of the listener
	Namespace string `json:"namespace"`
	// statefulset name of the listeners set
	StatefulSetName string `json:"statefulsetname"`
	// +optional
	Results *Results `json:"results,omitempty"`
	// map of PipelineRunTaskRunStatus with the taskRun name as the key
	// +optional
	PipelineRuns map[string]*PipelineRunStatus `json:"pipelineRuns,omitempty"`
	// The listener is addressable
	// +optional
	Address duckv1alpha1.Addressable `json:"address,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TektonListenerList contains a list of PipelineRun
type TektonListenerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonListener `json:"items"`
}
