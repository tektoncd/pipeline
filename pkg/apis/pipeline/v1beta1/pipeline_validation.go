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
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*Pipeline)(nil)

// Validate checks that the Pipeline structure is valid but does not validate
// that any references resources exist, that is done at run time.
func (p *Pipeline) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(p.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	return p.Spec.Validate(ctx)
}

// validateDeclaredResources ensures that the specified resources have unique names and
// validates that all the resources referenced by pipeline tasks are declared in the pipeline
func validateDeclaredResources(resources []PipelineDeclaredResource, tasks []PipelineTask, finalTasks []PipelineTask) error {
	encountered := map[string]struct{}{}
	for _, r := range resources {
		if _, ok := encountered[r.Name]; ok {
			return fmt.Errorf("resource with name %q appears more than once", r.Name)
		}
		encountered[r.Name] = struct{}{}
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
		return fmt.Errorf("pipeline declared resources didn't match usage in Tasks: Didn't provide required values: %s", missing)
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
func validateFrom(tasks []PipelineTask) *apis.FieldError {
	taskOutputs := map[string][]PipelineTaskOutputResource{}
	for _, task := range tasks {
		var to []PipelineTaskOutputResource
		if task.Resources != nil {
			to = make([]PipelineTaskOutputResource, len(task.Resources.Outputs))
			copy(to, task.Resources.Outputs)
		}
		taskOutputs[task.Name] = to
	}
	for _, t := range tasks {
		inputResources := []PipelineTaskInputResource{}
		if t.Resources != nil {
			inputResources = append(inputResources, t.Resources.Inputs...)
		}

		for _, c := range t.Conditions {
			inputResources = append(inputResources, c.Resources...)
		}

		for _, rd := range inputResources {
			for _, pt := range rd.From {
				outputs, found := taskOutputs[pt]
				if !found {
					return apis.ErrInvalidValue(fmt.Sprintf("expected resource %s to be from task %s, but task %s doesn't exist", rd.Resource, pt, pt),
						"spec.tasks.resources.inputs.from")
				}
				if !isOutput(outputs, rd.Resource) {
					return apis.ErrInvalidValue(fmt.Sprintf("the resource %s from %s must be an output but is an input", rd.Resource, pt),
						"spec.tasks.resources.inputs.from")
				}
			}
		}
	}
	return nil
}

// validateGraph ensures the Pipeline's dependency Graph (DAG) make sense: that there is no dependency
// cycle or that they rely on values from Tasks that ran previously, and that the PipelineResource
// is actually an output of the Task it should come from.
func validateGraph(tasks []PipelineTask) error {
	if _, err := dag.Build(PipelineTaskList(tasks)); err != nil {
		return err
	}
	return nil
}

// Validate checks that taskNames in the Pipeline are valid and that the graph
// of Tasks expressed in the Pipeline makes sense.
func (ps *PipelineSpec) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(ps, &PipelineSpec{}) {
		return apis.ErrGeneric("expected at least one, got none", "spec.description", "spec.params", "spec.resources", "spec.tasks", "spec.workspaces")
	}

	// PipelineTask must have a valid unique label and at least one of taskRef or taskSpec should be specified
	if err := validatePipelineTasks(ctx, ps.Tasks, ps.Finally); err != nil {
		return err
	}

	// All declared resources should be used, and the Pipeline shouldn't try to use any resources
	// that aren't declared
	if err := validateDeclaredResources(ps.Resources, ps.Tasks, ps.Finally); err != nil {
		return apis.ErrInvalidValue(err.Error(), "spec.resources")
	}

	// The from values should make sense
	if err := validateFrom(ps.Tasks); err != nil {
		return err
	}

	// Validate the pipeline task graph
	if err := validateGraph(ps.Tasks); err != nil {
		return apis.ErrInvalidValue(err.Error(), "spec.tasks")
	}

	if err := validateParamResults(ps.Tasks); err != nil {
		return apis.ErrInvalidValue(err.Error(), "spec.tasks.params.value")
	}

	// The parameter variables should be valid
	if err := validatePipelineParameterVariables(ps.Tasks, ps.Params); err != nil {
		return err
	}

	if err := validatePipelineParameterVariables(ps.Finally, ps.Params); err != nil {
		return err
	}

	// Validate the pipeline's workspaces.
	if err := validatePipelineWorkspaces(ps.Workspaces, ps.Tasks, ps.Finally); err != nil {
		return err
	}

	// Validate the pipeline's results
	if err := validatePipelineResults(ps.Results); err != nil {
		return apis.ErrInvalidValue(err.Error(), "spec.tasks.params.value")
	}

	if err := validateTasksAndFinallySection(ps); err != nil {
		return err
	}

	if err := validateFinalTasks(ps.Finally); err != nil {
		return err
	}

	return nil
}

// validatePipelineTasks ensures that pipeline tasks has unique label, pipeline tasks has specified one of
// taskRef or taskSpec, and in case of a pipeline task with taskRef, it has a reference to a valid task (task name)
func validatePipelineTasks(ctx context.Context, tasks []PipelineTask, finalTasks []PipelineTask) *apis.FieldError {
	// Names cannot be duplicated
	taskNames := map[string]struct{}{}
	var err *apis.FieldError
	for i, t := range tasks {
		if err = validatePipelineTaskName(ctx, "spec.tasks", i, t, taskNames); err != nil {
			return err
		}
	}
	for i, t := range finalTasks {
		if err = validatePipelineTaskName(ctx, "spec.finally", i, t, taskNames); err != nil {
			return err
		}
	}
	return nil
}

func validatePipelineTaskName(ctx context.Context, prefix string, i int, t PipelineTask, taskNames map[string]struct{}) *apis.FieldError {
	if errs := validation.IsDNS1123Label(t.Name); len(errs) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid value %q", t.Name),
			Paths:   []string{fmt.Sprintf(prefix+"[%d].name", i)},
			Details: "Pipeline Task name must be a valid DNS Label." +
				"For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		}
	}
	// can't have both taskRef and taskSpec at the same time
	if (t.TaskRef != nil && t.TaskRef.Name != "") && t.TaskSpec != nil {
		return apis.ErrMultipleOneOf(fmt.Sprintf(prefix+"[%d].taskRef", i), fmt.Sprintf(prefix+"[%d].taskSpec", i))
	}
	// Check that one of TaskRef and TaskSpec is present
	if (t.TaskRef == nil || (t.TaskRef != nil && t.TaskRef.Name == "")) && t.TaskSpec == nil {
		return apis.ErrMissingOneOf(fmt.Sprintf(prefix+"[%d].taskRef", i), fmt.Sprintf(prefix+"[%d].taskSpec", i))
	}
	// Validate TaskSpec if it's present
	if t.TaskSpec != nil {
		if err := t.TaskSpec.Validate(ctx); err != nil {
			return err
		}
	}
	if t.TaskRef != nil && t.TaskRef.Name != "" {
		// Task names are appended to the container name, which must exist and
		// must be a valid k8s name
		if errSlice := validation.IsQualifiedName(t.Name); len(errSlice) != 0 {
			return apis.ErrInvalidValue(strings.Join(errSlice, ","), fmt.Sprintf(prefix+"[%d].name", i))
		}
		// TaskRef name must be a valid k8s name
		if errSlice := validation.IsQualifiedName(t.TaskRef.Name); len(errSlice) != 0 {
			return apis.ErrInvalidValue(strings.Join(errSlice, ","), fmt.Sprintf(prefix+"[%d].taskRef.name", i))
		}
		if _, ok := taskNames[t.Name]; ok {
			return apis.ErrMultipleOneOf(fmt.Sprintf(prefix+"[%d].name", i))
		}
		taskNames[t.Name] = struct{}{}
	}
	return nil
}

// validatePipelineWorkspaces validates the specified workspaces, ensuring having unique name without any empty string,
// and validates that all the referenced workspaces (by pipeline tasks) are specified in the pipeline
func validatePipelineWorkspaces(wss []PipelineWorkspaceDeclaration, pts []PipelineTask, finalTasks []PipelineTask) *apis.FieldError {
	// Workspace names must be non-empty and unique.
	wsTable := make(map[string]struct{})
	for i, ws := range wss {
		if ws.Name == "" {
			return apis.ErrInvalidValue(fmt.Sprintf("workspace %d has empty name", i), "spec.workspaces")
		}
		if _, ok := wsTable[ws.Name]; ok {
			return apis.ErrInvalidValue(fmt.Sprintf("workspace with name %q appears more than once", ws.Name), "spec.workspaces")
		}
		wsTable[ws.Name] = struct{}{}
	}

	// Any workspaces used in PipelineTasks should have their name declared in the Pipeline's
	// Workspaces list.
	for i, pt := range pts {
		for j, ws := range pt.Workspaces {
			if _, ok := wsTable[ws.Workspace]; !ok {
				return apis.ErrInvalidValue(
					fmt.Sprintf("pipeline task %q expects workspace with name %q but none exists in pipeline spec", pt.Name, ws.Workspace),
					fmt.Sprintf("spec.tasks[%d].workspaces[%d]", i, j),
				)
			}
		}
	}
	for i, t := range finalTasks {
		for j, ws := range t.Workspaces {
			if _, ok := wsTable[ws.Workspace]; !ok {
				return apis.ErrInvalidValue(
					fmt.Sprintf("pipeline task %q expects workspace with name %q but none exists in pipeline spec", t.Name, ws.Workspace),
					fmt.Sprintf("spec.finally[%d].workspaces[%d]", i, j),
				)
			}
		}
	}
	return nil
}

// validatePipelineParameterVariables validates parameters with those specified by each pipeline task,
// (1) it validates the type of parameter is either string or array (2) parameter default value matches
// with the type of that param (3) ensures that the referenced param variable is defined is part of the param declarations
func validatePipelineParameterVariables(tasks []PipelineTask, params []ParamSpec) *apis.FieldError {
	parameterNames := map[string]struct{}{}
	arrayParameterNames := map[string]struct{}{}

	for _, p := range params {
		// Verify that p is a valid type.
		validType := false
		for _, allowedType := range AllParamTypes {
			if p.Type == allowedType {
				validType = true
			}
		}
		if !validType {
			return apis.ErrInvalidValue(string(p.Type), fmt.Sprintf("spec.params.%s.type", p.Name))
		}

		// If a default value is provided, ensure its type matches param's declared type.
		if (p.Default != nil) && (p.Default.Type != p.Type) {
			return &apis.FieldError{
				Message: fmt.Sprintf(
					"\"%v\" type does not match default value's type: \"%v\"", p.Type, p.Default.Type),
				Paths: []string{
					fmt.Sprintf("spec.params.%s.type", p.Name),
					fmt.Sprintf("spec.params.%s.default.type", p.Name),
				},
			}
		}

		// Add parameter name to parameterNames, and to arrayParameterNames if type is array.
		parameterNames[p.Name] = struct{}{}
		if p.Type == ParamTypeArray {
			arrayParameterNames[p.Name] = struct{}{}
		}
	}

	return validatePipelineVariables(tasks, "params", parameterNames, arrayParameterNames)
}

func validatePipelineVariables(tasks []PipelineTask, prefix string, paramNames map[string]struct{}, arrayParamNames map[string]struct{}) *apis.FieldError {
	for _, task := range tasks {
		for _, param := range task.Params {
			if param.Value.Type == ParamTypeString {
				if err := validatePipelineVariable(fmt.Sprintf("param[%s]", param.Name), param.Value.StringVal, prefix, paramNames); err != nil {
					return err
				}
				if err := validatePipelineNoArrayReferenced(fmt.Sprintf("param[%s]", param.Name), param.Value.StringVal, prefix, arrayParamNames); err != nil {
					return err
				}
			} else {
				for _, arrayElement := range param.Value.ArrayVal {
					if err := validatePipelineVariable(fmt.Sprintf("param[%s]", param.Name), arrayElement, prefix, paramNames); err != nil {
						return err
					}
					if err := validatePipelineArraysIsolated(fmt.Sprintf("param[%s]", param.Name), arrayElement, prefix, arrayParamNames); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func validatePipelineVariable(name, value, prefix string, vars map[string]struct{}) *apis.FieldError {
	return substitution.ValidateVariable(name, value, prefix, "task parameter", "pipelinespec.params", vars)
}

func validatePipelineNoArrayReferenced(name, value, prefix string, vars map[string]struct{}) *apis.FieldError {
	return substitution.ValidateVariableProhibited(name, value, prefix, "task parameter", "pipelinespec.params", vars)
}

func validatePipelineArraysIsolated(name, value, prefix string, vars map[string]struct{}) *apis.FieldError {
	return substitution.ValidateVariableIsolated(name, value, prefix, "task parameter", "pipelinespec.params", vars)
}

// validateParamResults ensures that task result variables are properly configured
func validateParamResults(tasks []PipelineTask) error {
	for _, task := range tasks {
		for _, param := range task.Params {
			expressions, ok := GetVarSubstitutionExpressionsForParam(param)
			if ok {
				if LooksLikeContainsResultRefs(expressions) {
					expressions = filter(expressions, looksLikeResultRef)
					resultRefs := NewResultRefs(expressions)
					if len(expressions) != len(resultRefs) {
						return fmt.Errorf("expected all of the expressions %v to be result expressions but only %v were", expressions, resultRefs)
					}
				}
			}
		}
	}
	return nil
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
func validatePipelineResults(results []PipelineResult) error {
	for _, result := range results {
		expressions, ok := GetVarSubstitutionExpressionsForPipelineResult(result)
		if ok {
			if LooksLikeContainsResultRefs(expressions) {
				expressions = filter(expressions, looksLikeResultRef)
				resultRefs := NewResultRefs(expressions)
				if len(expressions) != len(resultRefs) {
					return fmt.Errorf("expected all of the expressions %v to be result expressions but only %v were", expressions, resultRefs)
				}
			}
		}
	}
	return nil
}

func validateTasksAndFinallySection(ps *PipelineSpec) *apis.FieldError {
	if len(ps.Finally) != 0 && len(ps.Tasks) == 0 {
		return apis.ErrInvalidValue(fmt.Sprintf("spec.tasks is empty but spec.finally has %d tasks", len(ps.Finally)), "spec.finally")
	}
	return nil
}

func validateFinalTasks(finalTasks []PipelineTask) *apis.FieldError {
	for _, f := range finalTasks {
		if len(f.RunAfter) != 0 {
			return apis.ErrInvalidValue(fmt.Sprintf("no runAfter allowed under spec.finally, final task %s has runAfter specified", f.Name), "spec.finally")
		}
		if len(f.Conditions) != 0 {
			return apis.ErrInvalidValue(fmt.Sprintf("no conditions allowed under spec.finally, final task %s has conditions specified", f.Name), "spec.finally")
		}
	}

	if err := validateTaskResultReferenceNotUsed(finalTasks); err != nil {
		return err
	}

	if err := validateTasksInputFrom(finalTasks); err != nil {
		return err
	}

	return nil
}

func validateTaskResultReferenceNotUsed(tasks []PipelineTask) *apis.FieldError {
	for _, t := range tasks {
		for _, p := range t.Params {
			expressions, ok := GetVarSubstitutionExpressionsForParam(p)
			if ok {
				if LooksLikeContainsResultRefs(expressions) {
					return apis.ErrInvalidValue(fmt.Sprintf("no task result allowed under params,"+
						"final task param %s has set task result as its value", p.Name), "spec.finally.task.params")
				}
			}
		}
	}
	return nil
}

func validateTasksInputFrom(tasks []PipelineTask) *apis.FieldError {
	for _, t := range tasks {
		inputResources := []PipelineTaskInputResource{}
		if t.Resources != nil {
			inputResources = append(inputResources, t.Resources.Inputs...)
		}
		for _, rd := range inputResources {
			if len(rd.From) != 0 {
				return apis.ErrDisallowedFields(fmt.Sprintf("no from allowed under inputs,"+
					" final task %s has from specified", rd.Name), "spec.finally.task.resources.inputs")
			}
		}
	}
	return nil
}
