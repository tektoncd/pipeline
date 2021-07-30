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

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*Pipeline)(nil)

// Validate checks that the Pipeline structure is valid but does not validate
// that any references resources exist, that is done at run time.
func (p *Pipeline) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(p.GetObjectMeta()).ViaField("metadata")
	if apis.IsInDelete(ctx) {
		return nil
	}
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
	// All declared resources should be used, and the Pipeline shouldn't try to use any resources
	// that aren't declared
	errs = errs.Also(validateDeclaredResources(ps.Resources, ps.Tasks, ps.Finally))
	// The from values should make sense
	errs = errs.Also(validateFrom(ps.Tasks))
	// Validate the pipeline task graph
	errs = errs.Also(validateGraph(ps.Tasks))
	errs = errs.Also(validateParamResults(ps.Tasks))
	// The parameter variables should be valid
	errs = errs.Also(validatePipelineParameterVariables(ps.Tasks, ps.Params).ViaField("tasks"))
	errs = errs.Also(validatePipelineParameterVariables(ps.Finally, ps.Params).ViaField("finally"))
	errs = errs.Also(validatePipelineContextVariables(ps.Tasks).ViaField("tasks"))
	errs = errs.Also(validatePipelineContextVariables(ps.Finally).ViaField("finally"))
	errs = errs.Also(validateExecutionStatusVariables(ps.Tasks, ps.Finally))
	// Validate the pipeline's workspaces.
	errs = errs.Also(validatePipelineWorkspaces(ps.Workspaces, ps.Tasks, ps.Finally))
	// Validate the pipeline's results
	errs = errs.Also(validatePipelineResults(ps.Results))
	errs = errs.Also(validateTasksAndFinallySection(ps))
	errs = errs.Also(validateFinalTasks(ps.Tasks, ps.Finally))
	errs = errs.Also(validateWhenExpressions(ps.Tasks, ps.Finally))
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

// validatePipelineWorkspaces validates the specified workspaces, ensuring having unique name without any empty string,
// and validates that all the referenced workspaces (by pipeline tasks) are specified in the pipeline
func validatePipelineWorkspaces(wss []PipelineWorkspaceDeclaration, pts []PipelineTask, finalTasks []PipelineTask) (errs *apis.FieldError) {
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

	// Any workspaces used in PipelineTasks should have their name declared in the Pipeline's
	// Workspaces list.
	for i, pt := range pts {
		for j, ws := range pt.Workspaces {
			if !wsTable.Has(ws.Workspace) {
				errs = errs.Also(apis.ErrInvalidValue(
					fmt.Sprintf("pipeline task %q expects workspace with name %q but none exists in pipeline spec", pt.Name, ws.Workspace),
					"",
				).ViaFieldIndex("workspaces", j).ViaFieldIndex("tasks", i))
			}
		}
	}
	for i, t := range finalTasks {
		for j, ws := range t.Workspaces {
			if !wsTable.Has(ws.Workspace) {
				errs = errs.Also(apis.ErrInvalidValue(
					fmt.Sprintf("pipeline task %q expects workspace with name %q but none exists in pipeline spec", t.Name, ws.Workspace),
					"",
				).ViaFieldIndex("workspaces", j).ViaFieldIndex("finally", i))
			}
		}
	}
	return errs
}

// validatePipelineParameterVariables validates parameters with those specified by each pipeline task,
// (1) it validates the type of parameter is either string or array (2) parameter default value matches
// with the type of that param (3) ensures that the referenced param variable is defined is part of the param declarations
func validatePipelineParameterVariables(tasks []PipelineTask, params []ParamSpec) (errs *apis.FieldError) {
	parameterNames := sets.NewString()
	arrayParameterNames := sets.NewString()

	for _, p := range params {
		// Verify that p is a valid type.
		validType := false
		for _, allowedType := range AllParamTypes {
			if p.Type == allowedType {
				validType = true
			}
		}
		if !validType {
			errs = errs.Also(apis.ErrInvalidValue(string(p.Type), "type").ViaFieldKey("params", p.Name))
		}

		// If a default value is provided, ensure its type matches param's declared type.
		if (p.Default != nil) && (p.Default.Type != p.Type) {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("\"%v\" type does not match default value's type: \"%v\"", p.Type, p.Default.Type),
				"type", "default.type").ViaFieldKey("params", p.Name))
		}

		if parameterNames.Has(p.Name) {
			errs = errs.Also(apis.ErrGeneric("parameter appears more than once", "").ViaFieldKey("params", p.Name))
		}
		// Add parameter name to parameterNames, and to arrayParameterNames if type is array.
		parameterNames.Insert(p.Name)
		if p.Type == ParamTypeArray {
			arrayParameterNames.Insert(p.Name)
		}
	}

	return errs.Also(validatePipelineParametersVariables(tasks, "params", parameterNames, arrayParameterNames))
}

func validatePipelineParametersVariables(tasks []PipelineTask, prefix string, paramNames sets.String, arrayParamNames sets.String) (errs *apis.FieldError) {
	for idx, task := range tasks {
		errs = errs.Also(validatePipelineParametersVariablesInTaskParameters(task.Params, prefix, paramNames, arrayParamNames).ViaIndex(idx))
		errs = errs.Also(task.WhenExpressions.validatePipelineParametersVariables(prefix, paramNames, arrayParamNames).ViaIndex(idx))
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
	var paramValues []string
	for _, task := range tasks {
		for _, param := range task.Params {
			paramValues = append(paramValues, param.Value.StringVal)
			paramValues = append(paramValues, param.Value.ArrayVal...)
		}
	}
	errs := validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipelineRun", pipelineRunContextNames)
	return errs.Also(validatePipelineContextVariablesInParamValues(paramValues, "context\\.pipeline", pipelineContextNames))
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
		for _, param := range t.Params {
			// retrieve a list of substitution expression from a param
			if ps, ok := GetVarSubstitutionExpressionsForParam(param); ok {
				// validate tasks.pipelineTask.status/tasks.status if this expression is not a result reference
				if !LooksLikeContainsResultRefs(ps) {
					for _, p := range ps {
						// check if it contains context variable accessing execution status - $(tasks.taskname.status)
						// or an aggregate status - $(tasks.status)
						if containsExecutionStatusRef(p) {
							errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("pipeline tasks can not refer to execution status of any other pipeline task"+
								" or aggregate status of tasks"), "value").ViaFieldKey("params", param.Name).ViaFieldIndex("tasks", idx))
						}
					}
				}
			}
		}
		for i, we := range t.WhenExpressions {
			// retrieve a list of substitution expression from a when expression
			if expressions, ok := we.GetVarSubstitutionExpressions(); ok {
				// validate tasks.pipelineTask.status/tasks.status if this expression is not a result reference
				if !LooksLikeContainsResultRefs(expressions) {
					for _, e := range expressions {
						// check if it contains context variable accessing execution status - $(tasks.taskname.status)
						// or an aggregate status - $(tasks.status)
						if containsExecutionStatusRef(e) {
							errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("when expressions in pipeline tasks can not refer to execution status of any other pipeline task"+
								" or aggregate status of tasks"), "").ViaFieldIndex("when", i).ViaFieldIndex("tasks", idx))
						}
					}
				}
			}
		}
	}
	return errs
}

// validate finally tasks accessing execution status of a dag task specified in the pipeline
// $(tasks.pipelineTask.status) is invalid if pipelineTask is not defined as a dag task
func validateExecutionStatusVariablesInFinally(tasks []PipelineTask, finally []PipelineTask) (errs *apis.FieldError) {
	// creating a list of pipelineTask names to validate tasks.<name>.status
	ptNames := PipelineTaskList(tasks).Names()
	for idx, t := range finally {
		for _, param := range t.Params {
			if expressions, ok := GetVarSubstitutionExpressionsForParam(param); ok {
				errs = errs.Also(validateExecutionStatusVariablesExpressions(expressions, ptNames, "value").ViaFieldKey(
					"params", param.Name).ViaFieldIndex("finally", idx))
			}
		}
		for i, we := range t.WhenExpressions {
			if expressions, ok := we.GetVarSubstitutionExpressions(); ok {
				errs = errs.Also(validateExecutionStatusVariablesExpressions(expressions, ptNames, "").ViaFieldIndex(
					"when", i).ViaFieldIndex("finally", idx))
			}
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

func validateExecutionStatusVariables(tasks []PipelineTask, finallyTasks []PipelineTask) (errs *apis.FieldError) {
	errs = errs.Also(validateExecutionStatusVariablesInTasks(tasks))
	errs = errs.Also(validateExecutionStatusVariablesInFinally(tasks, finallyTasks))
	return errs
}

func validatePipelineContextVariablesInParamValues(paramValues []string, prefix string, contextNames sets.String) (errs *apis.FieldError) {
	for _, paramValue := range paramValues {
		errs = errs.Also(substitution.ValidateVariableP(paramValue, prefix, contextNames).ViaField("value"))
	}
	return errs
}

// validateParamResults ensures that task result variables are properly configured
func validateParamResults(tasks []PipelineTask) (errs *apis.FieldError) {
	for idx, task := range tasks {
		for _, param := range task.Params {
			expressions, ok := GetVarSubstitutionExpressionsForParam(param)
			if ok {
				if LooksLikeContainsResultRefs(expressions) {
					expressions = filter(expressions, looksLikeResultRef)
					resultRefs := NewResultRefs(expressions)
					if len(expressions) != len(resultRefs) {
						errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("expected all of the expressions %v to be result expressions but only %v were", expressions, resultRefs),
							"value").ViaFieldKey("params", param.Name).ViaFieldIndex("tasks", idx))
					}
				}
			}
		}
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
func validatePipelineResults(results []PipelineResult) (errs *apis.FieldError) {
	for idx, result := range results {
		expressions, ok := GetVarSubstitutionExpressionsForPipelineResult(result)
		if ok {
			if LooksLikeContainsResultRefs(expressions) {
				expressions = filter(expressions, looksLikeResultRef)
				resultRefs := NewResultRefs(expressions)
				if len(expressions) != len(resultRefs) {
					errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("expected all of the expressions %v to be result expressions but only %v were", expressions, resultRefs),
						"value").ViaFieldIndex("results", idx))
				}
			}
		}
	}

	return errs
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
		if len(f.Conditions) != 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("no conditions allowed under spec.finally, final task %s has conditions specified", f.Name), "").ViaFieldIndex("finally", idx))
		}
	}

	ts := PipelineTaskList(tasks).Names()
	fts := PipelineTaskList(finalTasks).Names()

	errs = errs.Also(validateTaskResultReferenceInFinallyTasks(finalTasks, ts, fts))
	errs = errs.Also(validateTasksInputFrom(finalTasks).ViaField("finally"))

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

func validateTasksInputFrom(tasks []PipelineTask) (errs *apis.FieldError) {
	for idx, t := range tasks {
		inputResources := []PipelineTaskInputResource{}
		if t.Resources != nil {
			inputResources = append(inputResources, t.Resources.Inputs...)
		}
		for i, rd := range inputResources {
			if len(rd.From) != 0 {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("no from allowed under inputs,"+
					" final task %s has from specified", rd.Name), "").ViaFieldIndex("inputs", i).ViaField("resources").ViaIndex(idx))
			}
		}
	}
	return errs
}

func validateWhenExpressions(tasks []PipelineTask, finalTasks []PipelineTask) (errs *apis.FieldError) {
	for i, t := range tasks {
		errs = errs.Also(validateOneOfWhenExpressionsOrConditions(t).ViaFieldIndex("tasks", i))
		errs = errs.Also(t.WhenExpressions.validate().ViaFieldIndex("tasks", i))
	}
	for i, t := range finalTasks {
		errs = errs.Also(t.WhenExpressions.validate().ViaFieldIndex("finally", i))
	}
	return errs
}

func validateOneOfWhenExpressionsOrConditions(t PipelineTask) *apis.FieldError {
	if t.WhenExpressions != nil && t.Conditions != nil {
		return apis.ErrMultipleOneOf("when", "conditions")
	}
	return nil
}

// validateDeclaredResources ensures that the specified resources have unique names and
// validates that all the resources referenced by pipeline tasks are declared in the pipeline
func validateDeclaredResources(resources []PipelineDeclaredResource, tasks []PipelineTask, finalTasks []PipelineTask) *apis.FieldError {
	encountered := sets.NewString()
	for _, r := range resources {
		if encountered.Has(r.Name) {
			return apis.ErrInvalidValue(fmt.Sprintf("resource with name %q appears more than once", r.Name), "resources")
		}
		encountered.Insert(r.Name)
	}
	required := []string{}
	for _, t := range tasks {
		if t.Resources != nil {
			for _, input := range t.Resources.Inputs {
				required = append(required, input.Resource)
			}
			for _, output := range t.Resources.Outputs {
				required = append(required, output.Resource)
			}
		}

		for _, condition := range t.Conditions {
			for _, cr := range condition.Resources {
				required = append(required, cr.Resource)
			}
		}
	}
	for _, t := range finalTasks {
		if t.Resources != nil {
			for _, input := range t.Resources.Inputs {
				required = append(required, input.Resource)
			}
			for _, output := range t.Resources.Outputs {
				required = append(required, output.Resource)
			}
		}
	}

	provided := make([]string, 0, len(resources))
	for _, resource := range resources {
		provided = append(provided, resource.Name)
	}
	missing := list.DiffLeft(required, provided)
	if len(missing) > 0 {
		return apis.ErrInvalidValue(fmt.Sprintf("pipeline declared resources didn't match usage in Tasks: Didn't provide required values: %s", missing), "resources")
	}
	return nil
}

func isOutput(outputs []PipelineTaskOutputResource, resource string) bool {
	for _, output := range outputs {
		if output.Resource == resource {
			return true
		}
	}
	return false
}

// validateFrom ensures that the `from` values make sense: that they rely on values from Tasks
// that ran previously, and that the PipelineResource is actually an output of the Task it should come from.
func validateFrom(tasks []PipelineTask) (errs *apis.FieldError) {
	taskOutputs := map[string][]PipelineTaskOutputResource{}
	for _, task := range tasks {
		var to []PipelineTaskOutputResource
		if task.Resources != nil {
			to = make([]PipelineTaskOutputResource, len(task.Resources.Outputs))
			copy(to, task.Resources.Outputs)
		}
		taskOutputs[task.Name] = to
	}
	for i, t := range tasks {
		inputResources := []PipelineTaskInputResource{}
		if t.Resources != nil {
			inputResources = append(inputResources, t.Resources.Inputs...)
		}

		for _, c := range t.Conditions {
			inputResources = append(inputResources, c.Resources...)
		}

		for j, rd := range inputResources {
			for _, pt := range rd.From {
				outputs, found := taskOutputs[pt]
				if !found {
					return apis.ErrInvalidValue(fmt.Sprintf("expected resource %s to be from task %s, but task %s doesn't exist", rd.Resource, pt, pt),
						"from").ViaFieldIndex("inputs", j).ViaField("resources").ViaFieldIndex("tasks", i)
				}
				if !isOutput(outputs, rd.Resource) {
					return apis.ErrInvalidValue(fmt.Sprintf("the resource %s from %s must be an output but is an input", rd.Resource, pt),
						"from").ViaFieldIndex("inputs", j).ViaField("resources").ViaFieldIndex("tasks", i)
				}
			}
		}
	}
	return errs
}

// validateGraph ensures the Pipeline's dependency Graph (DAG) make sense: that there is no dependency
// cycle or that they rely on values from Tasks that ran previously, and that the PipelineResource
// is actually an output of the Task it should come from.
func validateGraph(tasks []PipelineTask) *apis.FieldError {
	if _, err := dag.Build(PipelineTaskList(tasks), PipelineTaskList(tasks).Deps()); err != nil {
		return apis.ErrInvalidValue(err.Error(), "tasks")
	}
	return nil
}
