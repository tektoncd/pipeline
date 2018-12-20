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
	"github.com/knative/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineSpec defines the desired state of PipeLine.
type PipelineSpec struct {
	Tasks      []PipelineTask `json:"tasks"`
	Generation int64          `json:"generation,omitempty"`
}

// PipelineStatus does not contain anything because Pipelines on their own
// do not have a status, they just hold data which is later used by a
// PipelineRun.
type PipelineStatus struct {
}

// Check that Pipeline may be validated and defaulted.
var _ apis.Validatable = (*Pipeline)(nil)
var _ apis.Defaultable = (*Pipeline)(nil)

// TaskKind defines the type of Task used by the pipeline.
type TaskKind string

const (
	// NamespacedTaskKind indicates that the task type has a namepace scope.
	NamespacedTaskKind TaskKind = "Task"
	// ClusterTaskKind indicates that task type has a cluster scope.
	ClusterTaskKind TaskKind = "ClusterTask"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pipeline describes a DAG of Tasks to execute. It expresses how outputs
// of tasks feed into inputs of subsequent tasks. The DAG is constructed
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
// Params and from the output of previous tasks.
type PipelineTask struct {
	Name    string  `json:"name"`
	TaskRef TaskRef `json:"taskRef"`
	// +optional
	ResourceDependencies []ResourceDependency `json:"resources,omitempty"`
	// +optional
	Params []Param `json:"params,omitempty"`
}

// PipelineTaskParam is used to provide arbitrary string parameters to a Task.
type PipelineTaskParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ResourceDependency is used when a PipelineResource required by a Task is requird to be provided by
// a previous Task, i.e. that Task needs to operate on the PipelineResource before this Task can be
// executed. It is from this dependency that the Pipeline's DAG is constructed.
type ResourceDependency struct {
	// Name is the name of the Task's input that this Resource should be used for.
	Name string `json:"name"`
	// ProvidedBy is the list of PipelineTask names that the resource has to come from.
	// +optional
	ProvidedBy []string `json:"providedBy,omitempty"`
}

// TaskRef can be used to refer to a specific instance of a task.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type TaskRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// TaskKind inficates the kind of the task, namespaced or cluster scoped.
	Kind TaskKind `json:"kind,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineList contains a list of Pipeline
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}
