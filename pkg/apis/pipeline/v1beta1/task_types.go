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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// TaskRunResultType default task run result value
	TaskRunResultType ResultType = "TaskRunResult"
	// PipelineResourceResultType default pipeline result value
	PipelineResourceResultType ResultType = "PipelineResourceResult"
	// InternalTektonResultType default internal tekton result value
	InternalTektonResultType ResultType = "InternalTektonResult"
	// UnknownResultType default unknown result type value
	UnknownResultType ResultType = ""
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

func (t *Task) Copy() TaskObject {
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

	// Description is a user-facing description of the task that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`

	// Steps are the steps of the build; each step is run sequentially with the
	// source mounted into /workspace.
	Steps []Step `json:"steps,omitempty"`

	// Volumes is a collection of volumes that are available to mount into the
	// steps of the build.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// StepTemplate can be used as the basis for all step containers within the
	// Task, so that the steps inherit settings on the base container.
	StepTemplate *corev1.Container `json:"stepTemplate,omitempty"`

	// UsesTemplate allows you to define shared uses defaults for all steps
	// with the Task so that you can reuse many steps from the same Path if no Path is specified
	UsesTemplate *Uses `json:"usesTemplate,omitempty"`

	// Sidecars are run alongside the Task's step containers. They begin before
	// the steps start and end after the steps complete.
	Sidecars []Sidecar `json:"sidecars,omitempty"`

	// Workspaces are the volumes that this Task requires.
	Workspaces []WorkspaceDeclaration `json:"workspaces,omitempty"`

	// Results are values that this Task can output
	Results []TaskResult `json:"results,omitempty"`
}

// TaskResult used to describe the results of a task
type TaskResult struct {
	// Name the given name
	Name string `json:"name"`

	// Description is a human-readable description of the result
	// +optional
	Description string `json:"description"`
}

// Step embeds the Container type, which allows it to include fields not
// provided by Container.
type Step struct {
	corev1.Container `json:",inline"`

	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command and the Args will be passed to the Script.
	Script string `json:"script,omitempty"`
	// Timeout is the time after which the step times out. Defaults to never.
	// Refer to Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Uses allows one or more Steps to be used from a source such as git or OCI. The steps
	// can have their properties overridden.
	// see: https://github.com/tektoncd/pipeline/blob/master/docs/step-composition.md
	Uses *Uses `json:"uses,omitempty"`
}

// Sidecar has nearly the same data structure as Step, consisting of a Container and an optional Script, but does not have the ability to timeout.
type Sidecar struct {
	corev1.Container `json:",inline"`

	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have an Command or Args.
	Script string `json:"script,omitempty"`
}

// UsesType indicates the type of a uses remote
// Used to distinguish between git and OCI.
type UsesType string

// Valid UsesTypes:
const (
	UsesTypeGit UsesType = "git"
	UsesTypeOCI UsesType = "oci"
)

// AllUsesTypes can be used for UsesType validation.
var AllUsesTypes = []UsesType{UsesTypeGit, UsesTypeOCI}

// Uses allows one or more steps to be inherited from a Task in git or some other source.
type Uses struct {
	// Kind the kind of remote used. Defaults to git but can use OCI
	Kind UsesType `json:"kind,omitempty"`

	// Server the server host if using git kind. Defaults to github.com if none specified.
	Server string `json:"server,omitempty"`

	// Path the path relative to the remote source.
	// For github this is usually the 'repositoryOwner/repositoryName/path@branchTagOrSHA'
	Path string `json:"path,omitempty"`

	// Step the name of the step to be included. If not specified all of the steps of the given
	// Task will be included.
	Step string `json:"step,omitempty"`

	// Task the name of the task if if the Path points to a PipelineRun or Pipeline which has multiple tasks. Otherwise the first Task is chosen
	Task string `json:"task,omitempty"`
}

// String returns a useful string representation of the uses clause
func (u *Uses) String() string {
	if u == nil {
		return "nil"
	}
	if u.Step != "" {
		return u.Path + ":" + u.Step
	}
	return u.Path
}

// Key returns a unique string for the kind and server we can use for caching Remotes
func (u *Uses) Key() string {
	if u.Kind == UsesTypeOCI {
		return string(u.Kind)
	}
	k := u.Kind
	s := u.Server
	if string(k) == "" {
		k = UsesTypeGit
	}
	if s == "" {
		s = "github.com"
	}
	return string(k) + "/" + s
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

// TaskRef can be used to refer to a specific instance of a task.
// Copied from CrossVersionObjectReference: https://github.com/kubernetes/kubernetes/blob/169df7434155cbbc22f1532cba8e0a9588e29ad8/pkg/apis/autoscaling/types.go#L64
type TaskRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`
	// TaskKind indicates the kind of the task, namespaced or cluster scoped.
	Kind TaskKind `json:"kind,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// Bundle url reference to a Tekton Bundle.
	// +optional
	Bundle string `json:"bundle,omitempty"`
}

// Check that Pipeline may be validated and defaulted.
// TaskKind defines the type of Task used by the pipeline.
type TaskKind string

const (
	// NamespacedTaskKind indicates that the task type has a namespaced scope.
	NamespacedTaskKind TaskKind = "Task"
	// ClusterTaskKind indicates that task type has a cluster scope.
	ClusterTaskKind TaskKind = "ClusterTask"
)
