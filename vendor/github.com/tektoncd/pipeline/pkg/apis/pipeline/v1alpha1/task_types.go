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

	"github.com/knative/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (t *Task) TaskSpec() TaskSpec {
	return t.Spec
}

func (t *Task) TaskMetadata() metav1.ObjectMeta {
	return t.ObjectMeta
}

func (t *Task) Copy() TaskInterface {
	return t.DeepCopy()
}

// TaskSpec defines the desired state of Task
type TaskSpec struct {
	// +optional
	Inputs *Inputs `json:"inputs,omitempty"`
	// +optional
	Outputs *Outputs `json:"outputs,omitempty"`

	// Steps are the steps of the build; each step is run sequentially with the
	// source mounted into /workspace.
	Steps []corev1.Container `json:"steps,omitempty"`

	// Volumes is a collection of volumes that are available to mount into the
	// steps of the build.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// ContainerTemplate can be used as the basis for all step containers within the
	// Task, so that the steps inherit settings on the base container.
	ContainerTemplate *corev1.Container `json:"containerTemplate,omitempty"`
}

// Check that Task may be validated and defaulted.
var _ apis.Validatable = (*Task)(nil)
var _ apis.Defaultable = (*Task)(nil)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Task is the Schema for the tasks API
// +k8s:openapi-gen=true
type Task struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the Task from the client
	// +optional
	Spec TaskSpec `json:"spec"`
}

// Inputs are the requirements that a task needs to run a Build.
type Inputs struct {
	// +optional
	Resources []TaskResource `json:"resources,omitempty"`
	// +optional
	Params []TaskParam `json:"params,omitempty"`
}

// TaskParam defines arbitrary parameters needed by a task beyond typed inputs
// such as resources.
type TaskParam struct {
	Name string `json:"name"`
	// +optional
	Description string `json:"description,omitempty"`
	// +optional
	Default string `json:"default,omitempty"`
}

// Param declares a value to use for the Param called Name.
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
	Resources []TaskResource `json:"resources,omitempty"`
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
