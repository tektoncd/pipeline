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
	// TaskKind indicates the Kind of the Task:
	// 1. Namespaced Task when Kind is set to "Task". If Kind is "", it defaults to "Task".
	// 2. Custom Task when Kind is non-empty and APIVersion is non-empty
	Kind TaskKind `json:"kind,omitempty"`
	// API version of the referent
	// Note: A Task with non-empty APIVersion and Kind is considered a Custom Task
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
	// ClusterTaskRefKind is the task type for a reference to a task with cluster scope.
	// ClusterTasks are not supported in v1, but v1 types may reference ClusterTasks.
	ClusterTaskRefKind TaskKind = "ClusterTask"
)

// IsCustomTask checks whether the reference is to a Custom Task
func (tr *TaskRef) IsCustomTask() bool {
	// Note that if `apiVersion` is set to `"tekton.dev/v1beta1"` and `kind` is set to `"Task"`,
	// the reference will be considered a Custom Task - https://github.com/tektoncd/pipeline/issues/6457
	return tr != nil && tr.APIVersion != "" && tr.Kind != ""
}
