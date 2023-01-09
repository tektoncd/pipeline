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
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/version"

	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
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
	Params []ParamSpec `json:"params,omitempty"`
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
	Params []Param `json:"params,omitempty"`

	// Matrix declares parameters used to fan out this task.
	// +optional
	Matrix *Matrix `json:"matrix,omitempty"`

	// Workspaces maps workspaces from the pipeline spec to the workspaces
	// declared in the Task.
	// +optional
	// +listType=atomic
	Workspaces []WorkspacePipelineTaskBinding `json:"workspaces,omitempty"`

	// Time after which the TaskRun times out. Defaults to 1 hour.
	// Specified TaskRun timeout should be less than 24h.
	// Refer Go's ParseDuration documentation for expected format: https://golang.org/pkg/time/#ParseDuration
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// Matrix is used to fan out Tasks in a Pipeline
type Matrix struct {
	// Params is a list of parameters used to fan out the pipelineTask
	// Params takes only `Parameters` of type `"array"`
	// Each array element is supplied to the `PipelineTask` by substituting `params` of type `"string"` in the underlying `Task`.
	// The names of the `params` in the `Matrix` must match the names of the `params` in the underlying `Task` that they will be substituting.
	// +listType=atomic
	Params []Param `json:"params,omitempty"`
}

// validateRefOrSpec validates at least one of taskRef or taskSpec is specified
func (pt PipelineTask) validateRefOrSpec() (errs *apis.FieldError) {
	// can't have both taskRef and taskSpec at the same time
	if pt.TaskRef != nil && pt.TaskSpec != nil {
		errs = errs.Also(apis.ErrMultipleOneOf("taskRef", "taskSpec"))
	}
	// Check that one of TaskRef and TaskSpec is present
	if pt.TaskRef == nil && pt.TaskSpec == nil {
		errs = errs.Also(apis.ErrMissingOneOf("taskRef", "taskSpec"))
	}
	return errs
}

// validateCustomTask validates custom task specifications - checking kind and fail if not yet supported features specified
func (pt PipelineTask) validateCustomTask() (errs *apis.FieldError) {
	if pt.TaskRef != nil && pt.TaskRef.Kind == "" {
		errs = errs.Also(apis.ErrInvalidValue("custom task ref must specify kind", "taskRef.kind"))
	}
	if pt.TaskSpec != nil && pt.TaskSpec.Kind == "" {
		errs = errs.Also(apis.ErrInvalidValue("custom task spec must specify kind", "taskSpec.kind"))
	}
	if pt.TaskRef != nil && pt.TaskRef.APIVersion == "" {
		errs = errs.Also(apis.ErrInvalidValue("custom task ref must specify apiVersion", "taskRef.apiVersion"))
	}
	if pt.TaskSpec != nil && pt.TaskSpec.APIVersion == "" {
		errs = errs.Also(apis.ErrInvalidValue("custom task spec must specify apiVersion", "taskSpec.apiVersion"))
	}
	return errs
}

// validateTask validates a pipeline task or a final task for taskRef and taskSpec
func (pt PipelineTask) validateTask(ctx context.Context) (errs *apis.FieldError) {
	cfg := config.FromContextOrDefaults(ctx)
	// Validate TaskSpec if it's present
	if pt.TaskSpec != nil {
		errs = errs.Also(pt.TaskSpec.Validate(ctx).ViaField("taskSpec"))
	}
	if pt.TaskRef != nil {
		if pt.TaskRef.Name != "" {
			// TaskRef name must be a valid k8s name
			if errSlice := validation.IsQualifiedName(pt.TaskRef.Name); len(errSlice) != 0 {
				errs = errs.Also(apis.ErrInvalidValue(strings.Join(errSlice, ","), "name"))
			}
		} else if pt.TaskRef.Resolver == "" {
			errs = errs.Also(apis.ErrInvalidValue("taskRef must specify name", "taskRef.name"))
		}
		if cfg.FeatureFlags.EnableAPIFields != config.BetaAPIFields && cfg.FeatureFlags.EnableAPIFields != config.AlphaAPIFields {
			// fail if resolver or resource are present when enable-api-fields is false.
			if pt.TaskRef.Resolver != "" {
				errs = errs.Also(apis.ErrDisallowedFields("taskref.resolver"))
			}
			if len(pt.TaskRef.Params) > 0 {
				errs = errs.Also(apis.ErrDisallowedFields("taskref.params"))
			}
		}
	}
	return errs
}

// IsMatrixed return whether pipeline task is matrixed
func (pt *PipelineTask) IsMatrixed() bool {
	return pt.Matrix != nil && len(pt.Matrix.Params) > 0
}

func (pt *PipelineTask) validateMatrix(ctx context.Context) (errs *apis.FieldError) {
	if pt.IsMatrixed() {
		// This is an alpha feature and will fail validation if it's used in a pipeline spec
		// when the enable-api-fields feature gate is anything but "alpha".
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "matrix", config.AlphaAPIFields))
		// Matrix requires "embedded-status" feature gate to be set to "minimal", and will fail
		// validation if it is anything but "minimal".
		errs = errs.Also(ValidateEmbeddedStatus(ctx, "matrix", config.MinimalEmbeddedStatus))
		errs = errs.Also(pt.validateMatrixCombinationsCount(ctx))
	}
	errs = errs.Also(validateParameterInOneOfMatrixOrParams(pt.Matrix, pt.Params))
	errs = errs.Also(validateParametersInTaskMatrix(pt.Matrix))
	return errs
}

func (pt *PipelineTask) validateMatrixCombinationsCount(ctx context.Context) (errs *apis.FieldError) {
	matrixCombinationsCount := pt.GetMatrixCombinationsCount()
	maxMatrixCombinationsCount := config.FromContextOrDefaults(ctx).Defaults.DefaultMaxMatrixCombinationsCount
	if matrixCombinationsCount > maxMatrixCombinationsCount {
		errs = errs.Also(apis.ErrOutOfBoundsValue(matrixCombinationsCount, 0, maxMatrixCombinationsCount, "matrix"))
	}
	return errs
}

func (pt PipelineTask) validateEmbeddedOrType() (errs *apis.FieldError) {
	// Reject cases where APIVersion and/or Kind are specified alongside an embedded Task.
	// We determine if this is an embedded Task by checking of TaskSpec.TaskSpec.Steps has items.
	if pt.TaskSpec != nil && len(pt.TaskSpec.TaskSpec.Steps) > 0 {
		if pt.TaskSpec.APIVersion != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "taskSpec.apiVersion cannot be specified when using taskSpec.steps",
				Paths:   []string{"taskSpec.apiVersion"},
			})
		}
		if pt.TaskSpec.Kind != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "taskSpec.kind cannot be specified when using taskSpec.steps",
				Paths:   []string{"taskSpec.kind"},
			})
		}
	}
	return
}

// GetMatrixCombinationsCount returns the count of combinations of Parameters generated from the Matrix in PipelineTask.
func (pt *PipelineTask) GetMatrixCombinationsCount() int {
	if !pt.IsMatrixed() {
		return 0
	}
	count := 1
	for _, param := range pt.Matrix.Params {
		count *= len(param.Value.ArrayVal)
	}
	return count
}

func (pt *PipelineTask) validateResultsFromMatrixedPipelineTasksNotConsumed(matrixedPipelineTasks sets.String) (errs *apis.FieldError) {
	for _, ref := range PipelineTaskResultRefs(pt) {
		if matrixedPipelineTasks.Has(ref.PipelineTask) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("consuming results from matrixed task %s is not allowed", ref.PipelineTask), ""))
		}
	}
	return errs
}

func (pt *PipelineTask) validateExecutionStatusVariablesDisallowed() (errs *apis.FieldError) {
	for _, param := range pt.Params {
		if expressions, ok := GetVarSubstitutionExpressionsForParam(param); ok {
			errs = errs.Also(validateContainsExecutionStatusVariablesDisallowed(expressions, "value").
				ViaFieldKey("params", param.Name))
		}
	}
	for i, we := range pt.When {
		if expressions, ok := we.GetVarSubstitutionExpressions(); ok {
			errs = errs.Also(validateContainsExecutionStatusVariablesDisallowed(expressions, "").
				ViaFieldIndex("when", i))
		}
	}
	return errs
}

func (pt *PipelineTask) validateExecutionStatusVariablesAllowed(ptNames sets.String) (errs *apis.FieldError) {
	for _, param := range pt.Params {
		if expressions, ok := GetVarSubstitutionExpressionsForParam(param); ok {
			errs = errs.Also(validateExecutionStatusVariablesExpressions(expressions, ptNames, "value").
				ViaFieldKey("params", param.Name))
		}
	}
	for i, we := range pt.When {
		if expressions, ok := we.GetVarSubstitutionExpressions(); ok {
			errs = errs.Also(validateExecutionStatusVariablesExpressions(expressions, ptNames, "").
				ViaFieldIndex("when", i))
		}
	}
	return errs
}

func validateContainsExecutionStatusVariablesDisallowed(expressions []string, path string) (errs *apis.FieldError) {
	if containsExecutionStatusReferences(expressions) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("pipeline tasks can not refer to execution status"+
			" of any other pipeline task or aggregate status of tasks"), path))
	}
	return errs
}

func containsExecutionStatusReferences(expressions []string) bool {
	// validate tasks.pipelineTask.status/tasks.status if this expression is not a result reference
	if !LooksLikeContainsResultRefs(expressions) {
		for _, e := range expressions {
			// check if it contains context variable accessing execution status - $(tasks.taskname.status)
			// or an aggregate status - $(tasks.status)
			if containsExecutionStatusRef(e) {
				return true
			}
		}
	}
	return false
}

func validateExecutionStatusVariablesExpressions(expressions []string, ptNames sets.String, fieldPath string) (errs *apis.FieldError) {
	// validate tasks.pipelineTask.status if this expression is not a result reference
	if !LooksLikeContainsResultRefs(expressions) {
		for _, expression := range expressions {
			// its a reference to aggregate status of dag tasks - $(tasks.status)
			if expression == PipelineTasksAggregateStatus {
				continue
			}
			// check if it contains context variable accessing execution status - $(tasks.taskname.status)
			if containsExecutionStatusRef(expression) {
				// strip tasks. and .status from tasks.taskname.status to further verify task name
				pt := strings.TrimSuffix(strings.TrimPrefix(expression, "tasks."), ".status")
				// report an error if the task name does not exist in the list of dag tasks
				if !ptNames.Has(pt) {
					errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("pipeline task %s is not defined in the pipeline", pt), fieldPath))
				}
			}
		}
	}
	return errs
}

func (pt *PipelineTask) validateWorkspaces(workspaceNames sets.String) (errs *apis.FieldError) {
	workspaceBindingNames := sets.NewString()
	for i, ws := range pt.Workspaces {
		if workspaceBindingNames.Has(ws.Name) {
			errs = errs.Also(apis.ErrGeneric(
				fmt.Sprintf("workspace name %q must be unique", ws.Name), "").ViaFieldIndex("workspaces", i))
		}

		if ws.Workspace == "" {
			if !workspaceNames.Has(ws.Name) {
				errs = errs.Also(apis.ErrInvalidValue(
					fmt.Sprintf("pipeline task %q expects workspace with name %q but none exists in pipeline spec", pt.Name, ws.Name),
					"",
				).ViaFieldIndex("workspaces", i))
			}
		} else if !workspaceNames.Has(ws.Workspace) {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("pipeline task %q expects workspace with name %q but none exists in pipeline spec", pt.Name, ws.Workspace),
				"",
			).ViaFieldIndex("workspaces", i))
		}

		workspaceBindingNames.Insert(ws.Name)
	}
	return errs
}

// TaskSpecMetadata returns the metadata of the PipelineTask's EmbeddedTask spec.
func (pt *PipelineTask) TaskSpecMetadata() PipelineTaskMetadata {
	return pt.TaskSpec.Metadata
}

// HashKey is the name of the PipelineTask, and is used as the key for this PipelineTask in the DAG
func (pt PipelineTask) HashKey() string {
	return pt.Name
}

// ValidateName checks whether the PipelineTask's name is a valid DNS label
func (pt PipelineTask) ValidateName() *apis.FieldError {
	if err := validation.IsDNS1123Label(pt.Name); len(err) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid value %q", pt.Name),
			Paths:   []string{"name"},
			Details: "Pipeline Task name must be a valid DNS Label." +
				"For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		}
	}
	return nil
}

// Validate classifies whether a task is a custom task or a regular task(dag/final)
// calls the validation routine based on the type of the task
func (pt PipelineTask) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(pt.validateRefOrSpec())

	errs = errs.Also(pt.validateEmbeddedOrType())

	// Pipeline task having taskRef/taskSpec with APIVersion is classified as custom task
	switch {
	case pt.TaskRef != nil && pt.TaskRef.APIVersion != "":
		errs = errs.Also(pt.validateCustomTask())
	case pt.TaskSpec != nil && pt.TaskSpec.APIVersion != "":
		errs = errs.Also(pt.validateCustomTask())
	default:
		errs = errs.Also(pt.validateTask(ctx))
	}
	return
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

// Validate a list of pipeline tasks including custom task
func (l PipelineTaskList) Validate(ctx context.Context, taskNames sets.String, path string) (errs *apis.FieldError) {
	for i, t := range l {
		// validate pipeline task name
		errs = errs.Also(t.ValidateName().ViaFieldIndex(path, i))
		// names cannot be duplicated - checking that pipelineTask names are unique
		if _, ok := taskNames[t.Name]; ok {
			errs = errs.Also(apis.ErrMultipleOneOf("name").ViaFieldIndex(path, i))
		}
		taskNames.Insert(t.Name)
		// validate custom task, dag, or final task
		errs = errs.Also(t.Validate(ctx).ViaFieldIndex(path, i))
	}
	return errs
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
