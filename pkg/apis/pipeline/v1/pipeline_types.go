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

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/kmeta"
)

const (
	// PipelineTasksAggregateStatus is a param representing aggregate status of all dag pipelineTasks
	PipelineTasksAggregateStatus = "tasks.status"
	// PipelineTasks is a value representing a task is a member of "tasks" section of the pipeline
	PipelineTasks = "tasks"
	// PipelineFinallyTasks is a value representing a task is a member of "finally" section of the pipeline
	PipelineFinallyTasks = "finally"
)

// +genclient
// +genclient:noStatus
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pipeline describes a list of Tasks to execute. It expresses how outputs
// of tasks feed into inputs of subsequent tasks.
// +k8s:openapi-gen=true
type Pipeline struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the Pipeline from the client
	// +optional
	Spec PipelineSpec `json:"spec"`
}

var _ kmeta.OwnerRefable = (*Pipeline)(nil)

// PipelineMetadata returns the Pipeline's ObjectMeta, implementing PipelineObject
func (p *Pipeline) PipelineMetadata() metav1.ObjectMeta {
	return p.ObjectMeta
}

// PipelineSpec returns the Pipeline's Spec, implementing PipelineObject
func (p *Pipeline) PipelineSpec() PipelineSpec {
	return p.Spec
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*Pipeline) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.PipelineControllerName)
}

// PipelineSpec defines the desired state of Pipeline.
type PipelineSpec struct {
	// DisplayName is a user-facing name of the pipeline that may be
	// used to populate a UI.
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// Description is a user-facing description of the pipeline that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`
	// Tasks declares the graph of Tasks that execute when this Pipeline is run.
	// +listType=atomic
	Tasks []PipelineTask `json:"tasks,omitempty"`
	// Params declares a list of input parameters that must be supplied when
	// this Pipeline is run.
	// +listType=atomic
	Params ParamSpecs `json:"params,omitempty"`
	// Workspaces declares a set of named workspaces that are expected to be
	// provided by a PipelineRun.
	// +optional
	// +listType=atomic
	Workspaces []PipelineWorkspaceDeclaration `json:"workspaces,omitempty"`
	// Results are values that this pipeline can output once run
	// +optional
	// +listType=atomic
	Results []PipelineResult `json:"results,omitempty"`
	// Finally declares the list of Tasks that execute just before leaving the Pipeline
	// i.e. either after all Tasks are finished executing successfully
	// or after a failure which would result in ending the Pipeline
	// +listType=atomic
	Finally []PipelineTask `json:"finally,omitempty"`
}

// PipelineResult used to describe the results of a pipeline
type PipelineResult struct {
	// Name the given name
	Name string `json:"name"`

	// Type is the user-specified type of the result.
	// The possible types are 'string', 'array', and 'object', with 'string' as the default.
	// 'array' and 'object' types are alpha features.
	Type ResultsType `json:"type,omitempty"`

	// Description is a human-readable description of the result
	// +optional
	Description string `json:"description"`

	// Value the expression used to retrieve the value
	Value ResultValue `json:"value"`
}

// PipelineTaskMetadata contains the labels or annotations for an EmbeddedTask
type PipelineTaskMetadata struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// EmbeddedTask is used to define a Task inline within a Pipeline's PipelineTasks.
type EmbeddedTask struct {
	// +optional
	runtime.TypeMeta `json:",inline,omitempty"`

	// Spec is a specification of a custom task
	// +optional
	Spec runtime.RawExtension `json:"spec,omitempty"`

	// +optional
	Metadata PipelineTaskMetadata `json:"metadata,omitempty"`

	// TaskSpec is a specification of a task
	// +optional
	TaskSpec `json:",inline,omitempty"`
}

// PipelineTask defines a task in a Pipeline, passing inputs from both
// Params and from the output of previous tasks.
type PipelineTask struct {
	// Name is the name of this task within the context of a Pipeline. Name is
	// used as a coordinate with the `from` and `runAfter` fields to establish
	// the execution order of tasks relative to one another.
	Name string `json:"name,omitempty"`

	// DisplayName is the display name of this task within the context of a Pipeline.
	// This display name may be used to populate a UI.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Description is the description of this task within the context of a Pipeline.
	// This description may be used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`

	// TaskRef is a reference to a task definition.
	// +optional
	TaskRef *TaskRef `json:"taskRef,omitempty"`

	// TaskSpec is a specification of a task
	// +optional
	TaskSpec *EmbeddedTask `json:"taskSpec,omitempty"`

	// When is a list of when expressions that need to be true for the task to run
	// +optional
	When WhenExpressions `json:"when,omitempty"`

	// Retries represents how many times this task should be retried in case of task failure: ConditionSucceeded set to False
	// +optional
	Retries int `json:"retries,omitempty"`

	// RunAfter is the list of PipelineTask names that should be executed before
	// this Task executes. (Used to force a specific ordering in graph execution.)
	// +optional
	// +listType=atomic
	RunAfter []string `json:"runAfter,omitempty"`

	// Parameters declares parameters passed to this task.
	// +optional
	// +listType=atomic
	Params Params `json:"params,omitempty"`

	// Matrix declares parameters used to fan out this task.
	// +optional
	Matrix *Matrix `json:"matrix,omitempty"`

	// Workspaces maps workspaces from the pipeline spec to the workspaces
	// declared in the Task.
	// +optional
	// +listType=atomic
	Workspaces []WorkspacePipelineTaskBinding `json:"workspaces,omitempty"`

	// Time after which the TaskRun times out. Defaults to 1 hour.
	// Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// IsCustomTask checks whether an embedded TaskSpec is a Custom Task
func (et *EmbeddedTask) IsCustomTask() bool {
	// Note that if `apiVersion` is set to `"tekton.dev/v1beta1"` and `kind` is set to `"Task"`,
	// the reference will be considered a Custom Task - https://github.com/tektoncd/pipeline/issues/6457
	return et != nil && et.APIVersion != "" && et.Kind != ""
}

// IsMatrixed return whether pipeline task is matrixed
func (pt *PipelineTask) IsMatrixed() bool {
	return pt.Matrix.HasParams() || pt.Matrix.HasInclude()
}

// TaskSpecMetadata returns the metadata of the PipelineTask's EmbeddedTask spec.
func (pt *PipelineTask) TaskSpecMetadata() PipelineTaskMetadata {
	return pt.TaskSpec.Metadata
}

// HashKey is the name of the PipelineTask, and is used as the key for this PipelineTask in the DAG
func (pt PipelineTask) HashKey() string {
	return pt.Name
}

// Deps returns all other PipelineTask dependencies of this PipelineTask, based on resource usage or ordering
func (pt PipelineTask) Deps() []string {
	// hold the list of dependencies in a set to avoid duplicates
	deps := sets.NewString()

	// add any new dependents from result references - resource dependency
	for _, ref := range PipelineTaskResultRefs(&pt) {
		deps.Insert(ref.PipelineTask)
	}

	// add any new dependents from runAfter - order dependency
	for _, runAfter := range pt.RunAfter {
		deps.Insert(runAfter)
	}

	return deps.List()
}

// PipelineTaskList is a list of PipelineTasks
type PipelineTaskList []PipelineTask

// Deps returns a map with key as name of a pipelineTask and value as a list of its dependencies
func (l PipelineTaskList) Deps() map[string][]string {
	deps := map[string][]string{}
	for _, pt := range l {
		// get the list of deps for this pipelineTask
		d := pt.Deps()
		// add the pipelineTask into the map if it has any deps
		if len(d) > 0 {
			deps[pt.HashKey()] = d
		}
	}
	return deps
}

// Items returns a slice of all tasks in the PipelineTaskList, converted to dag.Tasks
func (l PipelineTaskList) Items() []dag.Task {
	tasks := []dag.Task{}
	for _, t := range l {
		tasks = append(tasks, dag.Task(t))
	}
	return tasks
}

// Names returns a set of pipeline task names from the given list of pipeline tasks
func (l PipelineTaskList) Names() sets.String {
	names := sets.String{}
	for _, pt := range l {
		names.Insert(pt.Name)
	}
	return names
}

// PipelineTaskParam is used to provide arbitrary string parameters to a Task.
type PipelineTaskParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineList contains a list of Pipeline
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}
