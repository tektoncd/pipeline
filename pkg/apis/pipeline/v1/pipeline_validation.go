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
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/substitution"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ apis.Validatable = (*Pipeline)(nil)
var _ resourcesemantics.VerbLimited = (*Pipeline)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (p *Pipeline) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate checks that the Pipeline structure is valid but does not validate
// that any references resources exist, that is done at run time.
func (p *Pipeline) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(p.GetObjectMeta()).ViaField("metadata")
	ctx = config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, false)
	return errs.Also(p.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
}

// Validate checks that taskNames in the Pipeline are valid and that the graph
// of Tasks expressed in the Pipeline makes sense.
func (ps *PipelineSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if equality.Semantic.DeepEqual(ps, &PipelineSpec{}) {
		errs = errs.Also(apis.ErrGeneric("expected at least one, got none", "description", "params", "resources", "tasks", "workspaces"))
	}
	// PipelineTask must have a valid unique label and at least one of taskRef or taskSpec should be specified
	errs = errs.Also(ValidatePipelineTasks(ctx, ps.Tasks, ps.Finally))
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
	errs = errs.Also(validatePipelineWorkspacesUsage(ctx, ps.Workspaces, ps.Tasks).ViaField("tasks"))
	errs = errs.Also(validatePipelineWorkspacesUsage(ctx, ps.Workspaces, ps.Finally).ViaField("finally"))
	// Validate the pipeline's results
	errs = errs.Also(validatePipelineResults(ps.Results, ps.Tasks, ps.Finally))
	errs = errs.Also(validateTasksAndFinallySection(ps))
	errs = errs.Also(validateFinalTasks(ps.Tasks, ps.Finally))
	errs = errs.Also(validateWhenExpressions(ps.Tasks, ps.Finally))
	errs = errs.Also(validateMatrix(ctx, ps.Tasks).ViaField("tasks"))
	errs = errs.Also(validateMatrix(ctx, ps.Finally).ViaField("finally"))
	errs = errs.Also(validateResultsFromMatrixedPipelineTasksNotConsumed(ps.Tasks, ps.Finally))
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

// validatePipelineWorkspacesUsage validates that all the referenced workspaces (by pipeline tasks) are specified in
// the pipeline
func validatePipelineWorkspacesUsage(ctx context.Context, wss []PipelineWorkspaceDeclaration, pts []PipelineTask) (errs *apis.FieldError) {
	if !config.ValidateParameterVariablesAndWorkspaces(ctx) {
		return nil
	}
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
// with the type of that param (3) ensures that the referenced param variable is defined is part of the param declarations
func ValidatePipelineParameterVariables(ctx context.Context, tasks []PipelineTask, params []ParamSpec) (errs *apis.FieldError) {
	parameterNames := sets.NewString()
	arrayParameterNames := sets.NewString()
	objectParameterNameKeys := map[string][]string{}

	// validates all the types within a slice of ParamSpecs
	errs = errs.Also(ValidateParameterTypes(ctx, params).ViaField("params"))

	for _, p := range params {
		if parameterNames.Has(p.Name) {
			errs = errs.Also(apis.ErrGeneric("parameter appears more than once", "").ViaFieldKey("params", p.Name))
		}
		// Add parameter name to parameterNames, and to arrayParameterNames if type is array.
		parameterNames.Insert(p.Name)
		if p.Type == ParamTypeArray {
			arrayParameterNames.Insert(p.Name)
		}

		if p.Type == ParamTypeObject {
			for k := range p.Properties {
				objectParameterNameKeys[p.Name] = append(objectParameterNameKeys[p.Name], k)
			}
		}
	}
	if config.ValidateParameterVariablesAndWorkspaces(ctx) {
		errs = errs.Also(validatePipelineParametersVariables(tasks, "params", parameterNames, arrayParameterNames, objectParameterNameKeys))
	}
	return errs
}

func validatePipelineParametersVariables(tasks []PipelineTask, prefix string, paramNames sets.String, arrayParamNames sets.String, objectParamNameKeys map[string][]string) (errs *apis.FieldError) {
	for idx, task := range tasks {
		errs = errs.Also(validatePipelineParametersVariablesInTaskParameters(task.Params, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaIndex(idx))
		if task.IsMatrixed() {
			errs = errs.Also(validatePipelineParametersVariablesInMatrixParameters(task.Matrix.Params, prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaIndex(idx))
		}
		errs = errs.Also(task.When.validatePipelineParametersVariables(prefix, paramNames, arrayParamNames, objectParamNameKeys).ViaIndex(idx))
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
		var matrixParams []Param
		if task.IsMatrixed() {
			matrixParams = task.Matrix.Params
		}
		for _, param := range append(task.Params, matrixParams...) {
			paramValues = append(paramValues, param.Value.StringVal)
			paramValues = append(paramValues, param.Value.ArrayVal...)
		}
	}
	errs := validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipelineRun", pipelineRunContextNames).
		Also(validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipeline", pipelineContextNames)).
		Also(validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipelineTask", pipelineTaskContextNames))
	return errs
}

func containsExecutionStatusRef(p string) bool {
	if strings.HasPrefix(p, "tasks.") && strings.HasSuffix(p, ".status") {
		return true
	}
	return false
}

// validate dag pipeline tasks, task params can not access execution status of any other task
// dag tasks cannot have param value as $(tasks.pipelineTask.status)
func validateExecutionStatusVariablesInTasks(tasks []PipelineTask) (errs *apis.FieldError) {
	for idx, t := range tasks {
		errs = errs.Also(t.validateExecutionStatusVariablesDisallowed()).ViaIndex(idx)
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

func validateExecutionStatusVariables(tasks []PipelineTask, finallyTasks []PipelineTask) (errs *apis.FieldError) {
	errs = errs.Also(validateExecutionStatusVariablesInTasks(tasks).ViaField("tasks"))
	errs = errs.Also(validateExecutionStatusVariablesInFinally(PipelineTaskList(tasks).Names(), finallyTasks).ViaField("finally"))
	return errs
}

func validatePipelineContextVariablesInParamValues(paramValues []string, prefix string, contextNames sets.String) (errs *apis.FieldError) {
	for _, paramValue := range paramValues {
		errs = errs.Also(substitution.ValidateVariableP(paramValue, prefix, contextNames).ViaField("value"))
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

		expressions = filter(expressions, looksLikeResultRef)
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
			pipelineTaskName, _, _, _, err := parseExpression(value)

			if err != nil {
				return false
			}

			if strings.HasPrefix(value, "tasks") && !pipelineTaskNames.Has(pipelineTaskName) {
				return false
			}
			if strings.HasPrefix(value, "finally") && !pipelineFinallyTaskNames.Has(pipelineTaskName) {
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
		for i, we := range t.When {
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

func validateWhenExpressions(tasks []PipelineTask, finalTasks []PipelineTask) (errs *apis.FieldError) {
	for i, t := range tasks {
		errs = errs.Also(t.When.validate().ViaFieldIndex("tasks", i))
	}
	for i, t := range finalTasks {
		errs = errs.Also(t.When.validate().ViaFieldIndex("finally", i))
	}
	return errs
}

// validateGraph ensures the Pipeline's dependency Graph (DAG) make sense: that there is no dependency
// cycle or that they rely on values from Tasks that ran previously.
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
	return errs
}

func validateResultsFromMatrixedPipelineTasksNotConsumed(tasks []PipelineTask, finally []PipelineTask) (errs *apis.FieldError) {
	matrixedPipelineTasks := sets.String{}
	for _, pt := range tasks {
		if pt.IsMatrixed() {
			matrixedPipelineTasks.Insert(pt.Name)
		}
	}
	for idx, pt := range tasks {
		errs = errs.Also(pt.validateResultsFromMatrixedPipelineTasksNotConsumed(matrixedPipelineTasks).ViaFieldIndex("tasks", idx))
	}
	for idx, pt := range finally {
		errs = errs.Also(pt.validateResultsFromMatrixedPipelineTasksNotConsumed(matrixedPipelineTasks).ViaFieldIndex("finally", idx))
	}
	return errs
}
