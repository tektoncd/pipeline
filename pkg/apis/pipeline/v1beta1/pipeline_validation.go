/*
Copyright 2020 The Tekton Authors

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
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/internal/artifactref"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/internal/resultref"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/substitution"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/strings/slices"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var (
	_ apis.Validatable              = (*Pipeline)(nil)
	_ resourcesemantics.VerbLimited = (*Pipeline)(nil)
)

const (
	taskRef      = "taskRef"
	taskSpec     = "taskSpec"
	pipelineRef  = "pipelineRef"
	pipelineSpec = "pipelineSpec"
)

// SupportedVerbs returns the operations that validation should be called for
func (p *Pipeline) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate checks that the Pipeline structure is valid but does not validate
// that any references resources exist, that is done at run time.
func (p *Pipeline) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(p.GetObjectMeta()).ViaField("metadata")
	errs = errs.Also(p.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
	// When a Pipeline is created directly, instead of declared inline in a PipelineRun,
	// we do not support propagated parameters and workspaces.
	// Validate that all params and workspaces it uses are declared.
	errs = errs.Also(p.Spec.validatePipelineParameterUsage(ctx).ViaField("spec"))
	return errs.Also(p.Spec.validatePipelineWorkspacesUsage().ViaField("spec"))
}

// Validate checks that taskNames in the Pipeline are valid and that the graph
// of Tasks expressed in the Pipeline makes sense.
func (ps *PipelineSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(ps.ValidateBetaFields(ctx))
	if equality.Semantic.DeepEqual(ps, &PipelineSpec{}) {
		errs = errs.Also(apis.ErrGeneric("expected at least one, got none", "description", "params", "resources", "tasks", "workspaces"))
	}
	// PipelineTask must have a valid unique label and at least one of taskRef or taskSpec should be specified
	errs = errs.Also(ValidatePipelineTasks(ctx, ps.Tasks, ps.Finally))
	if len(ps.Resources) > 0 {
		errs = errs.Also(apis.ErrDisallowedFields("resources"))
	}
	// Validate the pipeline task graph
	errs = errs.Also(validateGraph(ps.Tasks))
	// The parameter variables should be valid
	errs = errs.Also(ValidatePipelineParameterVariables(ctx, ps.Tasks, ps.Params).ViaField("tasks"))
	errs = errs.Also(ValidatePipelineParameterVariables(ctx, ps.Finally, ps.Params).ViaField("finally"))
	errs = errs.Also(validatePipelineContextVariables(ps.Tasks).ViaField("tasks"))
	errs = errs.Also(validatePipelineContextVariables(ps.Finally).ViaField("finally"))
	errs = errs.Also(validateExecutionStatusVariables(ps.Tasks, ps.Finally))
	// Validate the pipeline's workspaces.
	errs = errs.Also(validatePipelineWorkspacesDeclarations(ps.Workspaces))
	// Validate the pipeline's results
	errs = errs.Also(validatePipelineResults(ps.Results, ps.Tasks, ps.Finally))
	errs = errs.Also(validateTasksAndFinallySection(ps))
	errs = errs.Also(validateFinalTasks(ps.Tasks, ps.Finally))
	errs = errs.Also(validateWhenExpressions(ctx, ps.Tasks, ps.Finally))
	errs = errs.Also(validateArtifactReference(ctx, ps.Tasks, ps.Finally))
	errs = errs.Also(validateMatrix(ctx, ps.Tasks).ViaField("tasks"))
	errs = errs.Also(validateMatrix(ctx, ps.Finally).ViaField("finally"))
	return errs
}

// ValidateBetaFields returns an error if the PipelineSpec uses beta specifications governed by
// `enable-api-fields` but does not have "enable-api-fields" set to "alpha" or "beta".
func (ps *PipelineSpec) ValidateBetaFields(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	for i, pt := range ps.Tasks {
		errs = errs.Also(pt.validateBetaFields(ctx).ViaFieldIndex("tasks", i))
	}
	for i, pt := range ps.Finally {
		errs = errs.Also(pt.validateBetaFields(ctx).ViaFieldIndex("finally", i))
	}
	return errs
}

// validateBetaFields returns an error if the PipelineTask uses beta features but does not
// have "enable-api-fields" set to "alpha" or "beta".
func (pt *PipelineTask) validateBetaFields(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if pt.TaskRef != nil {
		// Resolvers
		if pt.TaskRef.Resolver != "" {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "taskref.resolver", config.BetaAPIFields))
		}
		if len(pt.TaskRef.Params) > 0 {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "taskref.params", config.BetaAPIFields))
		}
	}
	return errs
}

// ValidatePipelineTasks ensures that pipeline tasks has unique label, pipeline tasks has specified one of
// taskRef or taskSpec, and in case of a pipeline task with taskRef, it has a reference to a valid task (task name)
func ValidatePipelineTasks(ctx context.Context, tasks []PipelineTask, finalTasks []PipelineTask) *apis.FieldError {
	taskNames := sets.NewString()
	var errs *apis.FieldError
	errs = errs.Also(PipelineTaskList(tasks).Validate(ctx, taskNames, "tasks"))
	errs = errs.Also(PipelineTaskList(finalTasks).Validate(ctx, taskNames, "finally"))
	return errs
}

// Validate a list of pipeline tasks including custom task and bundles
func (l PipelineTaskList) Validate(ctx context.Context, taskNames sets.String, path string) (errs *apis.FieldError) {
	for i, t := range l {
		// validate pipeline task name
		errs = errs.Also(t.ValidateName().ViaFieldIndex(path, i))
		// names cannot be duplicated - checking that pipelineTask names are unique
		if _, ok := taskNames[t.Name]; ok {
			errs = errs.Also(apis.ErrMultipleOneOf("name").ViaFieldIndex(path, i))
		}
		taskNames.Insert(t.Name)
		// validate custom task, bundle, dag, or final task
		errs = errs.Also(t.Validate(ctx).ViaFieldIndex(path, i))
	}
	return errs
}

// validateUsageOfDeclaredPipelineTaskParameters validates that all parameters referenced in the pipeline Task are declared by the pipeline Task.
func (l PipelineTaskList) validateUsageOfDeclaredPipelineTaskParameters(ctx context.Context, additionalParams []ParamSpec, path string) (errs *apis.FieldError) {
	for i, t := range l {
		if t.TaskSpec != nil {
			errs = errs.Also(ValidateUsageOfDeclaredParameters(ctx, t.TaskSpec.Steps, append(t.TaskSpec.Params, additionalParams...)).ViaFieldIndex(path, i))
		}
	}
	return errs
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

// Validate classifies whether a task is a custom task, bundle, or a regular task(dag/final)
// calls the validation routine based on the type of the task
func (pt PipelineTask) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(pt.validateRefOrSpec(ctx))

	errs = errs.Also(pt.validateEnabledInlineSpec(ctx))

	errs = errs.Also(pt.validateEmbeddedOrType())

	if pt.Resources != nil {
		errs = errs.Also(apis.ErrDisallowedFields("resources"))
	}
	// taskKinds contains the kinds when the apiVersion is not set, they are not custom tasks,
	// if apiVersion is set they are custom tasks.
	taskKinds := map[TaskKind]bool{
		"":                 true,
		NamespacedTaskKind: true,
	}

	errs = errs.Also(pt.ValidateOnError(ctx))

	// Pipeline task having taskRef/taskSpec with APIVersion is classified as custom task
	switch {
	case pt.TaskRef != nil && !taskKinds[pt.TaskRef.Kind]:
		errs = errs.Also(pt.validateCustomTask())
	case pt.TaskRef != nil && pt.TaskRef.APIVersion != "":
		errs = errs.Also(pt.validateCustomTask())
	case pt.TaskSpec != nil && !taskKinds[TaskKind(pt.TaskSpec.Kind)]:
		errs = errs.Also(pt.validateCustomTask())
	case pt.TaskSpec != nil && pt.TaskSpec.APIVersion != "":
		errs = errs.Also(pt.validateCustomTask())
	default:
		errs = errs.Also(pt.validateTask(ctx))
	}
	return //nolint:nakedret
}

// ValidateOnError validates the OnError field of a PipelineTask
func (pt PipelineTask) ValidateOnError(ctx context.Context) (errs *apis.FieldError) {
	if pt.OnError != "" && !isParamRefs(string(pt.OnError)) {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "OnError", config.BetaAPIFields))
		if pt.OnError != PipelineTaskContinue && pt.OnError != PipelineTaskStopAndFail {
			errs = errs.Also(apis.ErrInvalidValue(pt.OnError, "OnError", "PipelineTask OnError must be either \"continue\" or \"stopAndFail\""))
		}
		if pt.OnError == PipelineTaskContinue && pt.Retries > 0 {
			errs = errs.Also(apis.ErrGeneric("PipelineTask OnError cannot be set to \"continue\" when Retries is greater than 0"))
		}
	}
	return errs
}

// validateEnabledInlineSpec validates that pipelineSpec or taskSpec is allowed by checking
// disable-inline-spec field
func (pt PipelineTask) validateEnabledInlineSpec(ctx context.Context) (errs *apis.FieldError) {
	if pt.TaskSpec != nil {
		if slices.Contains(strings.Split(
			config.FromContextOrDefaults(ctx).FeatureFlags.DisableInlineSpec, ","), "pipeline") {
			errs = errs.Also(apis.ErrDisallowedFields("taskSpec"))
		}
	}
	if pt.PipelineSpec != nil {
		if slices.Contains(strings.Split(
			config.FromContextOrDefaults(ctx).FeatureFlags.DisableInlineSpec, ","), "pipeline") {
			errs = errs.Also(apis.ErrDisallowedFields("pipelineSpec"))
		}
	}
	return errs
}

func (pt *PipelineTask) validateMatrix(ctx context.Context) (errs *apis.FieldError) {
	if pt.IsMatrixed() {
		// This is a beta feature and will fail validation if it's used in a pipeline spec
		// when the enable-api-fields feature gate is set to "stable".
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "matrix", config.BetaAPIFields))
		errs = errs.Also(pt.Matrix.validateCombinationsCount(ctx))
		errs = errs.Also(pt.Matrix.validateUniqueParams())
	}
	errs = errs.Also(pt.Matrix.validateParameterInOneOfMatrixOrParams(pt.Params))
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

// validateRefOrSpec validates at least one of taskRef or taskSpec or pipelineRef or pipelineSpec is specified
func (pt PipelineTask) validateRefOrSpec(ctx context.Context) (errs *apis.FieldError) {
	// collect all the specified specifications
	nonNilFields := []string{}
	if pt.TaskRef != nil {
		nonNilFields = append(nonNilFields, taskRef)
	}
	if pt.TaskSpec != nil {
		nonNilFields = append(nonNilFields, taskSpec)
	}
	if pt.PipelineRef != nil {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, pipelineRef, config.AlphaAPIFields))
		nonNilFields = append(nonNilFields, pipelineRef)
	}
	if pt.PipelineSpec != nil {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, pipelineSpec, config.AlphaAPIFields))
		nonNilFields = append(nonNilFields, pipelineSpec)
	}

	// check the length of nonNilFields
	// if one of taskRef or taskSpec or pipelineRef or pipelineSpec is specified,
	// the length of nonNilFields should exactly be 1
	if len(nonNilFields) > 1 {
		errs = errs.Also(apis.ErrGeneric("expected exactly one, got multiple", nonNilFields...))
	} else if len(nonNilFields) == 0 {
		cfg := config.FromContextOrDefaults(ctx)
		// check for TaskRef or TaskSpec or PipelineRef or PipelineSpec with alpha feature flag
		if cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
			errs = errs.Also(apis.ErrMissingOneOf(taskRef, taskSpec, pipelineRef, pipelineSpec))
		} else {
			// check for taskRef and taskSpec with beta/stable feature flag
			errs = errs.Also(apis.ErrMissingOneOf(taskRef, taskSpec))
		}
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
	if pt.TaskSpec != nil {
		errs = errs.Also(pt.TaskSpec.Validate(ctx).ViaField("taskSpec"))
	}
	if pt.TaskRef != nil {
		errs = errs.Also(pt.TaskRef.Validate(ctx).ViaField("taskRef"))
	}
	return errs
}

// validatePipelineWorkspacesDeclarations validates the specified workspaces, ensuring having unique name without any
// empty string,
func validatePipelineWorkspacesDeclarations(wss []PipelineWorkspaceDeclaration) (errs *apis.FieldError) {
	// Workspace names must be non-empty and unique.
	wsTable := sets.NewString()
	for i, ws := range wss {
		if ws.Name == "" {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("workspace %d has empty name", i),
				"").ViaFieldIndex("workspaces", i))
		}
		if wsTable.Has(ws.Name) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("workspace with name %q appears more than once", ws.Name),
				"").ViaFieldIndex("workspaces", i))
		}
		wsTable.Insert(ws.Name)
	}
	return errs
}

// validatePipelineParameterUsage validates that parameters referenced in the Pipeline are declared by the Pipeline
func (ps *PipelineSpec) validatePipelineParameterUsage(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(PipelineTaskList(ps.Tasks).validateUsageOfDeclaredPipelineTaskParameters(ctx, ps.Params, "tasks"))
	errs = errs.Also(PipelineTaskList(ps.Finally).validateUsageOfDeclaredPipelineTaskParameters(ctx, ps.Params, "finally"))
	errs = errs.Also(validatePipelineTaskParameterUsage(ps.Tasks, ps.Params).ViaField("tasks"))
	errs = errs.Also(validatePipelineTaskParameterUsage(ps.Finally, ps.Params).ViaField("finally"))
	return errs
}

// validatePipelineTaskParameterUsage validates that parameters referenced in the Pipeline Tasks are declared by the Pipeline
func validatePipelineTaskParameterUsage(tasks []PipelineTask, params ParamSpecs) (errs *apis.FieldError) {
	allParamNames := sets.NewString(params.getNames()...)
	_, arrayParams, objectParams := params.sortByType()
	arrayParamNames := sets.NewString(arrayParams.getNames()...)
	objectParameterNameKeys := map[string][]string{}
	for _, p := range objectParams {
		for k := range p.Properties {
			objectParameterNameKeys[p.Name] = append(objectParameterNameKeys[p.Name], k)
		}
	}
	errs = errs.Also(validatePipelineParametersVariables(tasks, "params", allParamNames, arrayParamNames, objectParameterNameKeys))
	for i, task := range tasks {
		errs = errs.Also(task.Params.validateDuplicateParameters().ViaField("params").ViaIndex(i))
	}
	return errs
}

// validatePipelineWorkspacesUsage validates that Workspaces referenced in the Pipeline are declared by the Pipeline
func (ps *PipelineSpec) validatePipelineWorkspacesUsage() (errs *apis.FieldError) {
	errs = errs.Also(validatePipelineTasksWorkspacesUsage(ps.Workspaces, ps.Tasks).ViaField("tasks"))
	errs = errs.Also(validatePipelineTasksWorkspacesUsage(ps.Workspaces, ps.Finally).ViaField("finally"))
	return errs
}

// validatePipelineTasksWorkspacesUsage validates that all the referenced workspaces (by pipeline tasks) are specified in
// the pipeline
func validatePipelineTasksWorkspacesUsage(wss []PipelineWorkspaceDeclaration, pts []PipelineTask) (errs *apis.FieldError) {
	workspaceNames := sets.NewString()
	for _, ws := range wss {
		workspaceNames.Insert(ws.Name)
	}
	// Any workspaces used in PipelineTasks should have their name declared in the Pipeline's Workspaces list.
	for i, pt := range pts {
		errs = errs.Also(pt.validateWorkspaces(workspaceNames).ViaIndex(i))
	}
	return errs
}

// ValidatePipelineParameterVariables validates parameters with those specified by each pipeline task,
// (1) it validates the type of parameter is either string or array (2) parameter default value matches
// with the type of that param (3) no duplication, feature flag and allowed param type when using param enum
func ValidatePipelineParameterVariables(ctx context.Context, tasks []PipelineTask, params ParamSpecs) (errs *apis.FieldError) {
	// validates all the types within a slice of ParamSpecs
	errs = errs.Also(ValidateParameterTypes(ctx, params).ViaField("params"))
	errs = errs.Also(params.validateNoDuplicateNames())
	errs = errs.Also(params.validateParamEnums(ctx).ViaField("params"))
	for i, task := range tasks {
		errs = errs.Also(task.Params.validateDuplicateParameters().ViaField("params").ViaIndex(i))
	}
	return errs
}

func validatePipelineParametersVariables(tasks []PipelineTask, prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	for idx, task := range tasks {
		errs = errs.Also(validatePipelineParametersVariablesInTaskParameters(task.Params, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaIndex(idx))
		if task.IsMatrixed() {
			errs = errs.Also(task.Matrix.validatePipelineParametersVariablesInMatrixParameters(prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaIndex(idx))
		}
		errs = errs.Also(task.WhenExpressions.validatePipelineParametersVariables(prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaIndex(idx))
	}
	return errs
}

func validatePipelineContextVariables(tasks []PipelineTask) *apis.FieldError {
	pipelineRunContextNames := sets.NewString().Insert(
		"name",
		"namespace",
		"uid",
	)
	pipelineContextNames := sets.NewString().Insert(
		"name",
	)
	pipelineTaskContextNames := sets.NewString().Insert(
		"retries",
	)
	var paramValues []string
	for _, task := range tasks {
		paramValues = task.extractAllParams().extractValues()
	}
	errs := validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipelineRun", pipelineRunContextNames).
		Also(validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipeline", pipelineContextNames)).
		Also(validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipelineTask", pipelineTaskContextNames))
	return errs
}

// extractAllParams extracts all the parameters in a PipelineTask:
// - pt.Params
// - pt.Matrix.Params
// - pt.Matrix.Include.Params
func (pt *PipelineTask) extractAllParams() Params {
	allParams := pt.Params
	if pt.Matrix.HasParams() {
		allParams = append(allParams, pt.Matrix.Params...)
	}
	if pt.Matrix.HasInclude() {
		for _, include := range pt.Matrix.Include {
			allParams = append(allParams, include.Params...)
		}
	}
	return allParams
}

// containsExecutionStatusRef checks if a specified param has a reference to execution status or reason
// $(tasks.<task-name>.status), $(tasks.status), or $(tasks.<task-name>.reason)
func containsExecutionStatusRef(p string) bool {
	if strings.HasPrefix(p, "tasks.") {
		if strings.HasSuffix(p, ".status") || strings.HasSuffix(p, ".reason") {
			return true
		}
	}
	return false
}

func validateExecutionStatusVariables(tasks []PipelineTask, finallyTasks []PipelineTask) (errs *apis.FieldError) {
	errs = errs.Also(validateExecutionStatusVariablesInTasks(tasks).ViaField("tasks"))
	errs = errs.Also(validateExecutionStatusVariablesInFinally(PipelineTaskList(tasks).Names(), finallyTasks).ViaField("finally"))
	return errs
}

// validate dag pipeline tasks, task params can not access execution status of any other task
// dag tasks cannot have param value as $(tasks.pipelineTask.status)
func validateExecutionStatusVariablesInTasks(tasks []PipelineTask) (errs *apis.FieldError) {
	for idx, t := range tasks {
		errs = errs.Also(t.validateExecutionStatusVariablesDisallowed().ViaIndex(idx))
	}
	return errs
}

// validate finally tasks accessing execution status of a dag task specified in the pipeline
// $(tasks.pipelineTask.status) is invalid if pipelineTask is not defined as a dag task
func validateExecutionStatusVariablesInFinally(tasksNames sets.String, finally []PipelineTask) (errs *apis.FieldError) {
	for idx, t := range finally {
		errs = errs.Also(t.validateExecutionStatusVariablesAllowed(tasksNames).ViaIndex(idx))
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
	for i, we := range pt.WhenExpressions {
		if expressions, ok := we.GetVarSubstitutionExpressions(); ok {
			errs = errs.Also(validateContainsExecutionStatusVariablesDisallowed(expressions, "").
				ViaFieldIndex("when", i))
		}
	}
	return errs
}

func validateContainsExecutionStatusVariablesDisallowed(expressions []string, path string) (errs *apis.FieldError) {
	if containsExecutionStatusReferences(expressions) {
		errs = errs.Also(apis.ErrInvalidValue("pipeline tasks can not refer to execution status"+
			" of any other pipeline task or aggregate status of tasks", path))
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

func (pt *PipelineTask) validateExecutionStatusVariablesAllowed(ptNames sets.String) (errs *apis.FieldError) {
	for _, param := range pt.Params {
		if expressions, ok := GetVarSubstitutionExpressionsForParam(param); ok {
			errs = errs.Also(validateExecutionStatusVariablesExpressions(expressions, ptNames, "value").
				ViaFieldKey("params", param.Name))
		}
	}
	for i, we := range pt.WhenExpressions {
		if expressions, ok := we.GetVarSubstitutionExpressions(); ok {
			errs = errs.Also(validateExecutionStatusVariablesExpressions(expressions, ptNames, "").
				ViaFieldIndex("when", i))
		}
	}
	return errs
}

func validateExecutionStatusVariablesExpressions(expressions []string, ptNames sets.String, fieldPath string) (errs *apis.FieldError) {
	// validate tasks.pipelineTask.status if this expression is not a result reference
	if !LooksLikeContainsResultRefs(expressions) {
		for _, expression := range expressions {
			// its a reference to aggregate status of dag tasks - $(tasks.status)
			if expression == PipelineTasksAggregateStatus {
				continue
			}
			// check if it contains context variable accessing execution status - $(tasks.taskname.status) | $(tasks.taskname.reason)
			if containsExecutionStatusRef(expression) {
				var pt string
				if strings.HasSuffix(expression, ".status") {
					// strip tasks. and .status from tasks.taskname.status to further verify task name
					pt = strings.TrimSuffix(strings.TrimPrefix(expression, "tasks."), ".status")
				}
				if strings.HasSuffix(expression, ".reason") {
					// strip tasks. and .reason from tasks.taskname.reason to further verify task name
					pt = strings.TrimSuffix(strings.TrimPrefix(expression, "tasks."), ".reason")
				}
				// report an error if the task name does not exist in the list of dag tasks
				if !ptNames.Has(pt) {
					errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("pipeline task %s is not defined in the pipeline", pt), fieldPath))
				}
			}
		}
	}
	return errs
}

func validatePipelineContextVariablesInParamValues(paramValues []string, prefix string, contextNames sets.String) (errs *apis.FieldError) {
	for _, paramValue := range paramValues {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(paramValue, prefix, contextNames).ViaField("value"))
	}
	return errs
}

func filter(arr []string, cond func(string) bool) []string {
	result := []string{}
	for i := range arr {
		if cond(arr[i]) {
			result = append(result, arr[i])
		}
	}
	return result
}

// validatePipelineResults ensure that pipeline result variables are properly configured
func validatePipelineResults(results []PipelineResult, tasks []PipelineTask, finally []PipelineTask) (errs *apis.FieldError) {
	pipelineTaskNames := getPipelineTasksNames(tasks)
	pipelineFinallyTaskNames := getPipelineTasksNames(finally)
	for idx, result := range results {
		expressions, ok := GetVarSubstitutionExpressionsForPipelineResult(result)
		if !ok {
			errs = errs.Also(apis.ErrInvalidValue("expected pipeline results to be task result expressions but no expressions were found",
				"value").ViaFieldIndex("results", idx))
		}

		if !LooksLikeContainsResultRefs(expressions) {
			errs = errs.Also(apis.ErrInvalidValue("expected pipeline results to be task result expressions but an invalid expressions was found",
				"value").ViaFieldIndex("results", idx))
		}

		expressions = filter(expressions, resultref.LooksLikeResultRef)
		resultRefs := NewResultRefs(expressions)
		if len(expressions) != len(resultRefs) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("expected all of the expressions %v to be result expressions but only %v were", expressions, resultRefs),
				"value").ViaFieldIndex("results", idx))
		}

		if !taskContainsResult(result.Value.StringVal, pipelineTaskNames, pipelineFinallyTaskNames) {
			errs = errs.Also(apis.ErrInvalidValue("referencing a nonexistent task",
				"value").ViaFieldIndex("results", idx))
		}
	}

	return errs
}

// put task names in a set
func getPipelineTasksNames(pipelineTasks []PipelineTask) sets.String {
	pipelineTaskNames := make(sets.String)
	for _, pipelineTask := range pipelineTasks {
		pipelineTaskNames.Insert(pipelineTask.Name)
	}

	return pipelineTaskNames
}

// taskContainsResult ensures the result value is referenced within the
// task names
func taskContainsResult(resultExpression string, pipelineTaskNames sets.String, pipelineFinallyTaskNames sets.String) bool {
	// split incase of multiple resultExpressions in the same result.Value string
	// i.e "$(task.<task-name).result.<result-name>) - $(task2.<task2-name).result2.<result2-name>)"
	split := strings.Split(resultExpression, "$")
	for _, expression := range split {
		if expression != "" {
			value := stripVarSubExpression("$" + expression)
			pr, err := resultref.ParseTaskExpression(value)
			if err != nil {
				return false
			}

			if strings.HasPrefix(value, "tasks") && !pipelineTaskNames.Has(pr.ResourceName) {
				return false
			}
			if strings.HasPrefix(value, "finally") && !pipelineFinallyTaskNames.Has(pr.ResourceName) {
				return false
			}
		}
	}
	return true
}

func validateTasksAndFinallySection(ps *PipelineSpec) *apis.FieldError {
	if len(ps.Finally) != 0 && len(ps.Tasks) == 0 {
		return apis.ErrInvalidValue(fmt.Sprintf("spec.tasks is empty but spec.finally has %d tasks", len(ps.Finally)), "finally")
	}
	return nil
}

func validateFinalTasks(tasks []PipelineTask, finalTasks []PipelineTask) (errs *apis.FieldError) {
	for idx, f := range finalTasks {
		if len(f.RunAfter) != 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("no runAfter allowed under spec.finally, final task %s has runAfter specified", f.Name), "").ViaFieldIndex("finally", idx))
		}
	}

	ts := PipelineTaskList(tasks).Names()
	fts := PipelineTaskList(finalTasks).Names()

	errs = errs.Also(validateTaskResultReferenceInFinallyTasks(finalTasks, ts, fts))

	return errs
}

func validateTaskResultReferenceInFinallyTasks(finalTasks []PipelineTask, ts sets.String, fts sets.String) (errs *apis.FieldError) {
	for idx, t := range finalTasks {
		for _, p := range t.Params {
			if expressions, ok := GetVarSubstitutionExpressionsForParam(p); ok {
				errs = errs.Also(validateResultsVariablesExpressionsInFinally(expressions, ts, fts, "value").ViaFieldKey(
					"params", p.Name).ViaFieldIndex("finally", idx))
			}
		}
		for i, we := range t.WhenExpressions {
			if expressions, ok := we.GetVarSubstitutionExpressions(); ok {
				errs = errs.Also(validateResultsVariablesExpressionsInFinally(expressions, ts, fts, "").ViaFieldIndex(
					"when", i).ViaFieldIndex("finally", idx))
			}
		}
	}
	return errs
}

func validateResultsVariablesExpressionsInFinally(expressions []string, pipelineTasksNames sets.String, finalTasksNames sets.String, fieldPath string) (errs *apis.FieldError) {
	if LooksLikeContainsResultRefs(expressions) {
		resultRefs := NewResultRefs(expressions)
		for _, resultRef := range resultRefs {
			pt := resultRef.PipelineTask
			if finalTasksNames.Has(pt) {
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("invalid task result reference, "+
					"final task has task result reference from a final task %s", pt), fieldPath))
			} else if !pipelineTasksNames.Has(resultRef.PipelineTask) {
				errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("invalid task result reference, "+
					"final task has task result reference from a task %s which is not defined in the pipeline", pt), fieldPath))
			}
		}
	}
	return errs
}

func validateWhenExpressions(ctx context.Context, tasks []PipelineTask, finalTasks []PipelineTask) (errs *apis.FieldError) {
	for i, t := range tasks {
		errs = errs.Also(t.WhenExpressions.validate(ctx).ViaFieldIndex("tasks", i))
	}
	for i, t := range finalTasks {
		errs = errs.Also(t.WhenExpressions.validate(ctx).ViaFieldIndex("finally", i))
	}
	return errs
}

// validateGraph ensures the Pipeline's dependency Graph (DAG) make sense: that there is no dependency
// cycle or that they rely on values from Tasks that ran previously, and that the PipelineResource
// is actually an output of the Task it should come from.
func validateGraph(tasks []PipelineTask) (errs *apis.FieldError) {
	if _, err := dag.Build(PipelineTaskList(tasks), PipelineTaskList(tasks).Deps()); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err.Error(), "tasks"))
	}
	return errs
}

func validateMatrix(ctx context.Context, tasks []PipelineTask) (errs *apis.FieldError) {
	for idx, task := range tasks {
		errs = errs.Also(task.validateMatrix(ctx).ViaIndex(idx))
	}
	errs = errs.Also(validateTaskResultsFromMatrixedPipelineTasksConsumed(tasks))
	return errs
}

// findAndValidateResultRefsForMatrix checks that any result references to Matrixed PipelineTasks if consumed
// by another PipelineTask that the entire array of results produced by a matrix is consumed in aggregate
// since consuming a singular result produced by a matrix is currently not supported
func findAndValidateResultRefsForMatrix(tasks []PipelineTask, taskMapping map[string]PipelineTask) (resultRefs []*ResultRef, errs *apis.FieldError) {
	for _, t := range tasks {
		for _, p := range t.Params {
			if expressions, ok := GetVarSubstitutionExpressionsForParam(p); ok {
				if LooksLikeContainsResultRefs(expressions) {
					resultRefs, errs = validateMatrixedPipelineTaskConsumed(expressions, taskMapping)
					if errs != nil {
						return nil, errs
					}
				}
			}
		}
	}
	return resultRefs, errs
}

// validateMatrixedPipelineTaskConsumed checks that any Matrixed Pipeline Task that the is being consumed is consumed in
// aggregate [*] since consuming a singular result produced by a matrix is currently not supported
func validateMatrixedPipelineTaskConsumed(expressions []string, taskMapping map[string]PipelineTask) (resultRefs []*ResultRef, errs *apis.FieldError) {
	var filteredExpressions []string
	for _, expression := range expressions {
		// ie. "tasks.<pipelineTaskName>.results.<resultName>[*]"
		subExpressions := strings.Split(expression, ".")
		pipelineTask := subExpressions[1] // pipelineTaskName
		taskConsumed := taskMapping[pipelineTask]
		if taskConsumed.IsMatrixed() {
			if !strings.HasSuffix(expression, "[*]") {
				errs = errs.Also(apis.ErrGeneric("A matrixed pipelineTask can only be consumed in aggregate using [*] notation, but is currently set to " + expression))
			}
			filteredExpressions = append(filteredExpressions, expression)
		}
	}
	return NewResultRefs(filteredExpressions), errs
}

// validateTaskResultsFromMatrixedPipelineTasksConsumed checks that any Matrixed Pipeline Task that the is being consumed
// is consumed in aggregate [*] since consuming a singular result produced by a matrix is currently not supported.
// It also validates that a matrix emitting results can only emit results with the underlying type string
// if those results are being consumed by another PipelineTask.
func validateTaskResultsFromMatrixedPipelineTasksConsumed(tasks []PipelineTask) (errs *apis.FieldError) {
	taskMapping := createTaskMapping(tasks)
	resultRefs, errs := findAndValidateResultRefsForMatrix(tasks, taskMapping)
	if errs != nil {
		return errs
	}

	errs = errs.Also(validateMatrixEmittingStringResults(resultRefs, taskMapping))
	return errs
}

// createTaskMapping maps the PipelineTaskName to the PipelineTask to easily access
// the pipelineTask by Name
func createTaskMapping(tasks []PipelineTask) (taskMap map[string]PipelineTask) {
	taskMapping := make(map[string]PipelineTask)
	for _, task := range tasks {
		taskMapping[task.Name] = task
	}
	return taskMapping
}

// validateMatrixEmittingStringResults checks a matrix emitting results can only emit results with the underlying type string
// if those results are being consumed by another PipelineTask.
func validateMatrixEmittingStringResults(resultRefs []*ResultRef, taskMapping map[string]PipelineTask) (errs *apis.FieldError) {
	for _, resultRef := range resultRefs {
		task := taskMapping[resultRef.PipelineTask]
		resultName := resultRef.Result
		if task.TaskRef != nil {
			referencedTaskName := task.TaskRef.Name
			referencedTask := taskMapping[referencedTaskName]
			if referencedTask.TaskSpec != nil {
				errs = errs.Also(validateStringResults(referencedTask.TaskSpec.Results, resultName))
			}
		} else if task.TaskSpec != nil {
			errs = errs.Also(validateStringResults(task.TaskSpec.Results, resultName))
		}
	}
	return errs
}

// validateStringResults ensure that the result type is string
func validateStringResults(results []TaskResult, resultName string) (errs *apis.FieldError) {
	for _, result := range results {
		if result.Name == resultName {
			if result.Type != ResultsTypeString {
				errs = errs.Also(apis.ErrInvalidValue(
					fmt.Sprintf("Matrixed PipelineTasks emitting results must have an underlying type string, but result %s has type %s in pipelineTask", resultName, string(result.Type)),
					"",
				))
			}
		}
	}
	return errs
}

// validateArtifactReference ensure that the feature flag enableArtifacts is set to true when using artifacts
func validateArtifactReference(ctx context.Context, tasks []PipelineTask, finalTasks []PipelineTask) (errs *apis.FieldError) {
	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableArtifacts {
		return errs
	}
	for i, t := range tasks {
		for _, v := range t.Params.extractValues() {
			if len(artifactref.TaskArtifactRegex.FindAllStringSubmatch(v, -1)) > 0 {
				return errs.Also(apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts), "").ViaField("params").ViaFieldIndex("tasks", i))
			}
		}
	}
	for i, t := range finalTasks {
		for _, v := range t.Params.extractValues() {
			if len(artifactref.TaskArtifactRegex.FindAllStringSubmatch(v, -1)) > 0 {
				return errs.Also(apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts), "").ViaField("params").ViaFieldIndex("finally", i))
			}
		}
	}
	return errs
}

// GetIndexingReferencesToArrayParams returns all strings referencing indices of PipelineRun array parameters
// from parameters, workspaces, and when expressions defined in the Pipeline's Tasks and Finally Tasks.
// For example, if a Task in the Pipeline has a parameter with a value "$(params.array-param-name[1])",
// this would be one of the strings returned.
func (ps *PipelineSpec) GetIndexingReferencesToArrayParams() sets.String {
	paramsRefs := []string{}
	for i := range ps.Tasks {
		paramsRefs = append(paramsRefs, ps.Tasks[i].Params.extractValues()...)
		if ps.Tasks[i].IsMatrixed() {
			paramsRefs = append(paramsRefs, ps.Tasks[i].Matrix.Params.extractValues()...)
		}
		for j := range ps.Tasks[i].Workspaces {
			paramsRefs = append(paramsRefs, ps.Tasks[i].Workspaces[j].SubPath)
		}
		for _, wes := range ps.Tasks[i].WhenExpressions {
			paramsRefs = append(paramsRefs, wes.Input)
			paramsRefs = append(paramsRefs, wes.Values...)
		}
	}
	for i := range ps.Finally {
		paramsRefs = append(paramsRefs, ps.Finally[i].Params.extractValues()...)
		if ps.Finally[i].IsMatrixed() {
			paramsRefs = append(paramsRefs, ps.Finally[i].Matrix.Params.extractValues()...)
		}
		for _, wes := range ps.Finally[i].WhenExpressions {
			paramsRefs = append(paramsRefs, wes.Input)
			paramsRefs = append(paramsRefs, wes.Values...)
		}
	}
	// extract all array indexing references, for example []{"$(params.array-params[1])"}
	arrayIndexParamRefs := []string{}
	for _, p := range paramsRefs {
		arrayIndexParamRefs = append(arrayIndexParamRefs, extractArrayIndexingParamRefs(p)...)
	}
	return sets.NewString(arrayIndexParamRefs...)
}
