/*
Copyright 2022 The Tekton Authors

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

package pipeline

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genclient:noStatus
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Task represents a collection of sequential steps that are run as part of a
// Pipeline using a set of inputs and producing a set of outputs. Tasks execute
// when TaskRuns are created that provide the input parameters and resources and
// output resources the Task requires.
//
// +k8s:openapi-gen=true
type Task struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// Spec holds the desired state of the Task from the client
	// +optional
	Spec TaskSpec
}

var _ kmeta.OwnerRefable = (*Task)(nil)

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*Task) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(TaskControllerName)
}

// TaskSpec defines the desired state of Task.
type TaskSpec struct {
	// Params is a list of input parameters required to run the task. Params
	// must be supplied as inputs in TaskRuns unless they declare a default
	// value.
	// +optional
	// +listType=atomic
	Params ParamSpecs

	// DisplayName is a user-facing name of the task that may be
	// used to populate a UI.
	// +optional
	DisplayName string

	// Description is a user-facing description of the task that may be
	// used to populate a UI.
	// +optional
	Description string

	// Steps are the steps of the build; each step is run sequentially with the
	// source mounted into /workspace.
	// +listType=atomic
	Steps []Step

	// Volumes is a collection of volumes that are available to mount into the
	// steps of the build.
	// +listType=atomic
	Volumes []corev1.Volume

	// StepTemplate can be used as the basis for all step containers within the
	// Task, so that the steps inherit settings on the base container.
	StepTemplate *StepTemplate

	// Sidecars are run alongside the Task's step containers. They begin before
	// the steps start and end after the steps complete.
	// +listType=atomic
	Sidecars []Sidecar

	// Workspaces are the volumes that this Task requires.
	// +listType=atomic
	Workspaces []WorkspaceDeclaration

	// Results are values that this Task can output
	// +listType=atomic
	Results []TaskResult
}

// TaskList contains a list of Task
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TaskList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta
	Items []Task
}
