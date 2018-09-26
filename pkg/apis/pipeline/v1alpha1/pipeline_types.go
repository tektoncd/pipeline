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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineSpec defines the desired state of PipeLine.
type PipelineSpec struct {
	Tasks   []PipelineTask     `json:"tasks"`
	Sources []PipelineResource `json:"resources"`
}

// PipelineStatus defines the observed state of Pipeline
type PipelineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pipeline describes a DAG of Tasks to execute. It expresses how outputs
// of tasks feed into inputs of subsequent tasks, and how parameters from
// a PipelineParams should be fed into each task. The DAG is constructed
// from the 'prev' and 'next' of each PipelineTask as well as Task dependencies.
// +k8s:openapi-gen=true
type Pipeline struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the Pipeline from the client
	// +optional
	Spec PipelineSpec `json:"spec,omitempty"`
	// Status communicates the observed state of the Pipeline form the controller
	// +optional
	Status PipelineStatus `json:"status,omitempty"`
}

// PipelineTask defines a task in a Pipeline, passing inputs from both
// PipelineParams and from the output of previous tasks.
type PipelineTask struct {
	Name    string  `json:"name"`
	TaskRef TaskRef `json:"taskRef"`
	// +optional
	InputSourceBindings []SourceBinding `json:"inputSourceBindings,omitempty"`
	// +optional
	OutputSourceBindings []SourceBinding `json:"outputSourceBindings,omitempty"`
	// +optional
	Params []Param `json:"params,omitempty"`
}

// PipelineTaskParam is used to provide arbitrary string parameters to a Task.
type PipelineTaskParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ClusterBinding is used to bind a cluster from a PipelineParams to a cluster required
// as an input for a task.
type ClusterBinding struct {
	// Key is the string that the PipelineParams will use to identify this source.
	Key string `json:"key"`
	// InputName is the string the Task will use to identify this cluster in its inputs.
	InputName string `json:"inputName"`
}

// SourceBinding is used to bind a Source from a PipelineParams to a source required
// as an input for a task.
type SourceBinding struct {
	// InputName is the string the Task will use to identify this resource in its inputs.
	Name string `json:"name"`
	// Key is the string that the PipelineParams will use to identify this source.
	Key string `json:"key"`
	// TODO: validate the passedConstraints values match previous Task names
	// PassedConstraints is the list of Task names that the resource has to pass through.
	// +optional
	PassedConstraints []string `json:"passedConstraints,omitempty"`
}

// TaskRef can be used to refer to a specific instance of a task.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type TaskRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// StandardResourceRef can be used to refer to a specific instance of a Resource
type StandardResourceRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineResource defines set of resources required by all Tasks in the pipeline.
type PipelineResource struct {
	Name        string              `json:"name"`
	ResourceRef StandardResourceRef `json:"resourceRef"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineList contains a list of Pipeline
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}
