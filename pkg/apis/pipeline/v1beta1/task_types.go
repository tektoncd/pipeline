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

package v1beta1

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
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
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the Task from the client
	// +optional
	Spec TaskSpec `json:"spec"`
}

var _ kmeta.OwnerRefable = (*Task)(nil)

// TaskSpec returns the task's spec
func (t *Task) TaskSpec() TaskSpec {
	return t.Spec
}

// TaskMetadata returns the task's ObjectMeta
func (t *Task) TaskMetadata() metav1.ObjectMeta {
	return t.ObjectMeta
}

// Copy returns a deep copy of the task
func (t *Task) Copy() TaskObject {
	return t.DeepCopy()
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*Task) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.TaskControllerName)
}

// TaskSpec defines the desired state of Task.
type TaskSpec struct {
	// Resources is a list input and output resource to run the task
	// Resources are represented in TaskRuns as bindings to instances of
	// PipelineResources.
	//
	// Deprecated: Unused, preserved only for backwards compatibility
	// +optional
	Resources *TaskResources `json:"resources,omitempty"`

	// Params is a list of input parameters required to run the task. Params
	// must be supplied as inputs in TaskRuns unless they declare a default
	// value.
	// +optional
	// +listType=atomic
	Params ParamSpecs `json:"params,omitempty"`

	// DisplayName is a user-facing name of the task that may be
	// used to populate a UI.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Description is a user-facing description of the task that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`

	// Steps are the steps of the build; each step is run sequentially with the
	// source mounted into /workspace.
	// +listType=atomic
	Steps []Step `json:"steps,omitempty"`

	// Volumes is a collection of volumes that are available to mount into the
	// steps of the build.
	// +listType=atomic
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// StepTemplate can be used as the basis for all step containers within the
	// Task, so that the steps inherit settings on the base container.
	StepTemplate *StepTemplate `json:"stepTemplate,omitempty"`

	// Sidecars are run alongside the Task's step containers. They begin before
	// the steps start and end after the steps complete.
	// +listType=atomic
	Sidecars []Sidecar `json:"sidecars,omitempty"`

	// Workspaces are the volumes that this Task requires.
	// +listType=atomic
	Workspaces []WorkspaceDeclaration `json:"workspaces,omitempty"`

	// Results are values that this Task can output
	// +listType=atomic
	Results []TaskResult `json:"results,omitempty"`
}

// TaskList contains a list of Task
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

// HasDeprecatedFields returns true if the TaskSpec has deprecated field specified.
func (ts *TaskSpec) HasDeprecatedFields() bool {
	if ts == nil {
		return false
	}
	if len(ts.Steps) > 0 {
		for _, s := range ts.Steps {
			if len(s.DeprecatedPorts) > 0 ||
				s.DeprecatedLivenessProbe != nil ||
				s.DeprecatedReadinessProbe != nil ||
				s.DeprecatedStartupProbe != nil ||
				s.DeprecatedLifecycle != nil ||
				s.DeprecatedTerminationMessagePath != "" ||
				s.DeprecatedTerminationMessagePolicy != "" ||
				s.DeprecatedStdin ||
				s.DeprecatedStdinOnce ||
				s.DeprecatedTTY {
				return true
			}
		}
	}
	if ts.StepTemplate != nil {
		if len(ts.StepTemplate.DeprecatedPorts) > 0 ||
			ts.StepTemplate.DeprecatedName != "" ||
			ts.StepTemplate.DeprecatedReadinessProbe != nil ||
			ts.StepTemplate.DeprecatedStartupProbe != nil ||
			ts.StepTemplate.DeprecatedLifecycle != nil ||
			ts.StepTemplate.DeprecatedTerminationMessagePath != "" ||
			ts.StepTemplate.DeprecatedTerminationMessagePolicy != "" ||
			ts.StepTemplate.DeprecatedStdin ||
			ts.StepTemplate.DeprecatedStdinOnce ||
			ts.StepTemplate.DeprecatedTTY {
			return true
		}
	}
	return false
}
