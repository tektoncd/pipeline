/*
Copyright 2019 The Tekton Authors

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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Task represents a collection of sequential steps that are run as part of a
// Pipeline using a set of inputs and producing a set of outputs. Tasks execute
// when TaskRuns are created that provide the input parameters and resources and
// output resources the Task requires.
//
// +k8s:openapi-gen=true
type Task struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the Task from the client
	// +optional
	Spec TaskSpec `json:"spec"`
}

func (t *Task) TaskSpec() TaskSpec {
	return t.Spec
}

func (t *Task) TaskMetadata() metav1.ObjectMeta {
	return t.ObjectMeta
}

func (t *Task) Copy() TaskInterface {
	return t.DeepCopy()
}

// TaskSpec defines the desired state of Task.
type TaskSpec struct {
	// Resources is a list input and output resource to run the task
	// Resources are represented in TaskRuns as bindings to instances of
	// PipelineResources.
	// +optional
	Resources *TaskResources `json:"resources,omitempty"`

	// Params is a list of input parameters required to run the task. Params
	// must be supplied as inputs in TaskRuns unless they declare a default
	// value.
	// +optional
	Params []ParamSpec `json:"params,omitempty"`

	// Steps are the steps of the build; each step is run sequentially with the
	// source mounted into /workspace.
	Steps []Step `json:"steps,omitempty"`

	// Volumes is a collection of volumes that are available to mount into the
	// steps of the build.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// StepTemplate can be used as the basis for all step containers within the
	// Task, so that the steps inherit settings on the base container.
	StepTemplate *corev1.Container `json:"stepTemplate,omitempty"`

	// Sidecars are run alongside the Task's step containers. They begin before
	// the steps start and end after the steps complete.
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// Workspaces are the volumes that this Task requires.
	Workspaces []WorkspaceDeclaration
}

// Step embeds the Container type, which allows it to include fields not
// provided by Container.
type Step struct {
	corev1.Container

	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command or Args.
	Script string `json:"script,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}
