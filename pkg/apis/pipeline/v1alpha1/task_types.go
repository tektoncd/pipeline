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
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskSpec defines the desired state of Task
type TaskSpec struct {
	// +optional
	Inputs *Inputs `json:"inputs,omitempty"`
	// +optional
	Outputs   *Outputs                 `json:"outputs,omitempty"`
	BuildSpec buildv1alpha1.BuildSpec `json:"buildSpec"`
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
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the Task from the client
	// +optional
	Spec TaskSpec `json:"spec,omitempty"`
	// Status communicates the observed state of the Task from the controller
	// +optional
	Status TaskStatus `json:"status,omitempty"`
}

// Inputs are the requirements that a task needs to run a Build.
type Inputs struct {
	// +optional
	Sources []Source `json:"resources,omitempty"`
	// +optional
	Params []Param `json:"params,omitempty"`
	// +optional
	Clusters []Cluster `json:"clusters,omitempty"`
}

// Source is data which is required by a Build/Task for context
// (e.g. a repo from which to build an image). The name of the input will be
// used as the name of the volume containing this context which will be mounted
// into the container executed by the Build/Task, e.g. a Source with the
// name "workspace" would be mounted into "/workspace".
//
// TODO(#62): Something is wrong here, this should be a reference to a resource,
// could just be that the names and comments are out of date.
type Source struct {
	// name of the source should match the name of the SourceBinding in the pipeline
	Name string               `json:"name"`
	Type PipelineResourceType `json:"type"`
}

// Param defines arbitrary parameters needed by a task beyond typed inputs
// such as resources.
type Param struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Outputs allow a task to declare what data the Build/Task will be producing,
// i.e. results such as logs and artifacts such as images.
type Outputs struct {
	// +optional
	Results []TestResult `json:"results,omitempty"`
	// +optional
	Sources []Source `json:"resources,omitempty"`
}

// TestResult allows a task to specify the location where test logs
// can be found and what format they will be in.
type TestResult struct {
	Name string `json:"name"`
	// TODO: maybe this is an enum with types like "go test", "junit", etc.
	Format string `json:"format"`
	Path   string `json:"path"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}
