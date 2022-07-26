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

package v1

// TaskRef can be used to refer to a specific instance of a task.
type TaskRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`
	// TaskKind indicates the kind of the task, namespaced or cluster scoped.
	Kind TaskKind `json:"kind,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// ResolverRef allows referencing a Task in a remote location
	// like a git repo. This field is only supported when the alpha
	// feature gate is enabled.
	// +optional
	ResolverRef `json:",omitempty"`
}

// TaskKind defines the type of Task used by the pipeline.
type TaskKind string

const (
	// NamespacedTaskKind indicates that the task type has a namespaced scope.
	NamespacedTaskKind TaskKind = "Task"
)
