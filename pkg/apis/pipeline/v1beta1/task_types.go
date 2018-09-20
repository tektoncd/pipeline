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
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskSpec defines the desired state of Task
type TaskSpec struct {
	Inputs    Inputs    `json:"inputs,omitempty"`
	Outputs   Outputs   `json:"outputs,omitempty"`
	BuildSpec BuildSpec `json:"buildSpec"`
}

// TaskStatus defines the observed state of Task
type TaskStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Task is the Schema for the tasks API
// +k8s:openapi-gen=true
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskSpec   `json:"spec,omitempty"`
	Status TaskStatus `json:"status,omitempty"`
}

// Inputs are the requirements that a task needs to run a Build.
type Inputs struct {
	Sources []SourceInput `json:"sources,omitempty"`
	Params  []Param       `json:"params,omitempty"`
}

// SourceInput is data which is required by a Build/Task for context
// (e.g. a repo from which to build an image). The name of the input will be
// used as the name of the volume containing this context which will be mounted
// into the container executed by the Build/Task, e.g. a SourceInput with the
// name "workspace" would be mounted into "/workspace".
type SourceInput struct {
	Name string `json:"name"`
}

// ParamType represents the type of the parameter.
type ParamType string

const (
	// ParamTypeString indicates this parameter is just a string.
	ParamTypeString ParamType = "string"
)

// Param defines arbitrary parameters needed by a task beyond typed inputs
// such as resources.
type Param struct {
	Name string    `json:"name"`
	Type ParamType `json:"type"`
}

// Outputs allow a task to declare what data the Build/Task will be producing,
// i.e. results such as logs and artifacts such as images.
type Outputs struct {
	Results   []TestResult `json:"results,omitempty"`
	Artifacts []Artifact   `json:"artifacts,omitempty"`
}

// TestResult allows a task to specify the location where test logs
// can be found and what format they will be in.
type TestResult struct {
	Name string `json:"name"`
	// TODO: maybe this is an enum with types like "go test", "junit", etc.
	Format string `json:"format"`
	Path   string `json:"path"`
}

// ArtifactType indicates what type of artifact store this is, so the controller
// will know how to publish artifacts from it.
type ArtifactType string

const (
	// ArtifactTypeImage indicates that this artifact is a container image.
	ArtifactTypeImage ArtifactType = "image"
)

// Artifact allows a Task to describe what artifacts it will be producing
// and specify where they will be stored.
type Artifact struct {
	Name     string       `json:"name"`
	Type     ArtifactType `json:"type"`
	StoreKey string       `json:"storeKey"`
}

// BuildSpec describes how to create a Build for this Task.
// A BuildSpec will contain either a Template or a series of Steps.
type BuildSpec struct {
	// Trying to emulate https://github.com/knative/build/blob/master/pkg/apis/build/v1alpha1/build_types.go
	Steps    []corev1.Container                      `json:"steps,omitempty"`
	Template buildv1alpha1.TemplateInstantiationSpec `json:"template,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Task{}, &TaskList{})
}
