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

package resources

import (
	"fmt"
	"sort"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

// ResolvedResultRefs represents all of the ResolvedResultRef for a pipeline task
type ResolvedResultRefs []*ResolvedResultRef

// ResolvedResultRef represents a result ref reference that has been fully resolved (value has been populated).
// If the value is from a Result, then the ResultReference will be populated to point to the ResultReference
// which resulted in the value
type ResolvedResultRef struct {
	Value           v1beta1.ArrayOrString
	ResultReference v1beta1.ResultRef
	FromTaskRun     string
}

// ResolveResultRefs resolves any ResultReference that are found in the target ResolvedPipelineRunTask
func ResolveResultRefs(pipelineRunState PipelineRunState, targets PipelineRunState) (ResolvedResultRefs, error) {
	var allResolvedResultRefs ResolvedResultRefs
	for _, target := range targets {
		resolvedResultRefs, err := convertToResultRefs(pipelineRunState, target)
		if err != nil {
			return nil, err
		}
		allResolvedResultRefs = append(allResolvedResultRefs, resolvedResultRefs...)
	}
	return removeDup(allResolvedResultRefs), nil
}

// ResolvePipelineResultRefs takes a list of PipelineResults and resolves any references they
// include to Task results in the given PipelineRunStatus
func ResolvePipelineResultRefs(pipelineStatus v1beta1.PipelineRunStatus, pipelineResults []v1beta1.PipelineResult) ResolvedResultRefs {
	var allResolvedResultRefs ResolvedResultRefs
	for _, result := range pipelineResults {
		resolvedResultRefs := convertPipelineResultToResultRefs(pipelineStatus, result)
		if resolvedResultRefs != nil {
			allResolvedResultRefs = append(allResolvedResultRefs, resolvedResultRefs...)
		}
	}
	return removeDup(allResolvedResultRefs)
}

// extractResultRefs resolves any ResultReference that are found in param or pipeline result
// Returns nil if none are found
func extractResultRefsForParam(pipelineRunState PipelineRunState, param v1beta1.Param) (ResolvedResultRefs, error) {
	expressions, ok := v1beta1.GetVarSubstitutionExpressionsForParam(param)
	if ok {
		return extractResultRefs(expressions, pipelineRunState)
	}
	return nil, nil
}

// extractResultRefs resolves any ResultReference that are found in param or pipeline result
// Returns nil if none are found
func extractResultRefsForPipelineResult(pipelineStatus v1beta1.PipelineRunStatus, result v1beta1.PipelineResult) (ResolvedResultRefs, error) {
	expressions, ok := v1beta1.GetVarSubstitutionExpressionsForPipelineResult(result)
	if ok {
		return extractResultRefsForPipelineResults(expressions, pipelineStatus)
	}
	return nil, nil
}

func extractResultRefsForPipelineResults(expressions []string, pipelineStatus v1beta1.PipelineRunStatus) (ResolvedResultRefs, error) {
	resultRefs := v1beta1.NewResultRefs(expressions)
	var resolvedResultRefs ResolvedResultRefs
	for _, resultRef := range resultRefs {
		resolvedResultRef, err := resolveResultRefForPipelineResult(pipelineStatus, resultRef)
		if err != nil {
			return nil, err
		}
		resolvedResultRefs = append(resolvedResultRefs, resolvedResultRef)
	}
	return removeDup(resolvedResultRefs), nil
}

func extractResultRefs(expressions []string, pipelineRunState PipelineRunState) (ResolvedResultRefs, error) {
	resultRefs := v1beta1.NewResultRefs(expressions)
	var resolvedResultRefs ResolvedResultRefs
	for _, resultRef := range resultRefs {
		resolvedResultRef, err := resolveResultRef(pipelineRunState, resultRef)
		if err != nil {
			return nil, err
		}
		resolvedResultRefs = append(resolvedResultRefs, resolvedResultRef)
	}
	return removeDup(resolvedResultRefs), nil
}

func removeDup(refs ResolvedResultRefs) ResolvedResultRefs {
	if refs == nil {
		return nil
	}
	resolvedResultRefByRef := make(map[v1beta1.ResultRef]*ResolvedResultRef, len(refs))
	for _, resolvedResultRef := range refs {
		resolvedResultRefByRef[resolvedResultRef.ResultReference] = resolvedResultRef
	}
	deduped := make([]*ResolvedResultRef, 0, len(resolvedResultRefByRef))

	// Sort the resulting keys to produce a deterministic ordering.
	order := make([]v1beta1.ResultRef, 0, len(refs))
	for key := range resolvedResultRefByRef {
		order = append(order, key)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].PipelineTask > order[j].PipelineTask {
			return false
		}
		if order[i].Result > order[j].Result {
			return false
		}
		return true
	})

	for _, key := range order {
		deduped = append(deduped, resolvedResultRefByRef[key])
	}
	return deduped
}

// convertToResultRefs replaces result references for all params and when expressions of the resolved pipeline run task
func convertToResultRefs(pipelineRunState PipelineRunState, target *ResolvedPipelineRunTask) (ResolvedResultRefs, error) {
	var resolvedResultRefs ResolvedResultRefs
	for _, condition := range target.PipelineTask.Conditions {
		condRefs, err := convertParams(condition.Params, pipelineRunState, condition.ConditionRef)
		if err != nil {
			return nil, err
		}
		resolvedResultRefs = append(resolvedResultRefs, condRefs...)
	}

	taskParamsRefs, err := convertParams(target.PipelineTask.Params, pipelineRunState, target.PipelineTask.Name)
	if err != nil {
		return nil, err
	}
	resolvedResultRefs = append(resolvedResultRefs, taskParamsRefs...)

	taskWhenExpressionsRefs, err := convertWhenExpressions(target.PipelineTask.WhenExpressions, pipelineRunState, target.PipelineTask.Name)
	if err != nil {
		return nil, err
	}
	resolvedResultRefs = append(resolvedResultRefs, taskWhenExpressionsRefs...)

	return resolvedResultRefs, nil
}

func convertParams(params []v1beta1.Param, pipelineRunState PipelineRunState, name string) (ResolvedResultRefs, error) {
	var resolvedParams ResolvedResultRefs
	for _, param := range params {
		resolvedResultRefs, err := extractResultRefsForParam(pipelineRunState, param)
		if err != nil {
			return nil, fmt.Errorf("unable to find result referenced by param %q in %q: %w", param.Name, name, err)
		}
		if resolvedResultRefs != nil {
			resolvedParams = append(resolvedParams, resolvedResultRefs...)
		}
	}
	return resolvedParams, nil
}

func convertWhenExpressions(whenExpressions []v1beta1.WhenExpression, pipelineRunState PipelineRunState, name string) (ResolvedResultRefs, error) {
	var resolvedWhenExpressions ResolvedResultRefs
	for _, whenExpression := range whenExpressions {
		expressions, ok := whenExpression.GetVarSubstitutionExpressions()
		if ok {
			resolvedResultRefs, err := extractResultRefs(expressions, pipelineRunState)
			if err != nil {
				return nil, fmt.Errorf("unable to find result referenced by when expression with input %q in task %q: %w", whenExpression.Input, name, err)
			}
			if resolvedResultRefs != nil {
				resolvedWhenExpressions = append(resolvedWhenExpressions, resolvedResultRefs...)
			}
		}
	}
	return resolvedWhenExpressions, nil
}

// convertPipelineResultToResultRefs converts all params of the resolved pipeline run task
func convertPipelineResultToResultRefs(pipelineStatus v1beta1.PipelineRunStatus, pipelineResult v1beta1.PipelineResult) ResolvedResultRefs {
	resolvedResultRefs, err := extractResultRefsForPipelineResult(pipelineStatus, pipelineResult)
	if err != nil {
		return nil
	}
	return resolvedResultRefs
}

func resolveResultRef(pipelineState PipelineRunState, resultRef *v1beta1.ResultRef) (*ResolvedResultRef, error) {
	referencedTaskRun, err := getReferencedTaskRun(pipelineState, resultRef)
	if err != nil {
		return nil, err
	}
	result, err := findTaskResultForParam(referencedTaskRun, resultRef)
	if err != nil {
		return nil, err
	}
	return &ResolvedResultRef{
		Value:           *v1beta1.NewArrayOrString(result.Value),
		FromTaskRun:     referencedTaskRun.Name,
		ResultReference: *resultRef,
	}, nil
}

func resolveResultRefForPipelineResult(pipelineStatus v1beta1.PipelineRunStatus, resultRef *v1beta1.ResultRef) (*ResolvedResultRef, error) {
	taskRunStatus, taskRunName, err := getTaskRunStatus(pipelineStatus, resultRef.PipelineTask)

	if err != nil {
		return nil, err
	}
	result, err := findTaskResultForPipelineResult(taskRunStatus, resultRef)
	if err != nil {
		return nil, err
	}
	return &ResolvedResultRef{
		Value:           *v1beta1.NewArrayOrString(result.Value),
		FromTaskRun:     taskRunName,
		ResultReference: *resultRef,
	}, nil
}

func getReferencedTaskRun(pipelineState PipelineRunState, reference *v1beta1.ResultRef) (*v1beta1.TaskRun, error) {
	referencedPipelineTask := pipelineState.ToMap()[reference.PipelineTask]

	if referencedPipelineTask == nil {
		return nil, fmt.Errorf("could not find task %q referenced by result", reference.PipelineTask)
	}
	if referencedPipelineTask.TaskRun == nil || referencedPipelineTask.IsFailure() {
		return nil, fmt.Errorf("could not find successful taskrun for task %q", referencedPipelineTask.PipelineTask.Name)
	}
	return referencedPipelineTask.TaskRun, nil
}

func getTaskRunStatus(pipelineStatus v1beta1.PipelineRunStatus, pipelineTaskName string) (*v1beta1.TaskRunStatus, string, error) {
	for key, taskRun := range pipelineStatus.PipelineRunStatusFields.TaskRuns {
		// check if the task run was successful
		if taskRun.PipelineTaskName == pipelineTaskName {
			c := taskRun.Status.GetCondition(apis.ConditionSucceeded)
			if c == nil {
				return nil, "", fmt.Errorf("could not find a successful task run status for task %q referenced by result", pipelineTaskName)
			}
			return taskRun.Status, key, nil
		}
	}
	return nil, "", fmt.Errorf("could not find task run status for task %q referenced by result", pipelineTaskName)
}

func findTaskResultForPipelineResult(taskStatus *v1beta1.TaskRunStatus, reference *v1beta1.ResultRef) (*v1beta1.TaskRunResult, error) {
	results := taskStatus.TaskRunStatusFields.TaskRunResults
	for _, result := range results {
		if result.Name == reference.Result {
			return &result, nil
		}
	}
	return nil, fmt.Errorf("Could not find result with name %s for task run %s", reference.Result, reference.PipelineTask)
}

func findTaskResultForParam(taskRun *v1beta1.TaskRun, reference *v1beta1.ResultRef) (*v1beta1.TaskRunResult, error) {
	results := taskRun.Status.TaskRunStatusFields.TaskRunResults
	for _, result := range results {
		if result.Name == reference.Result {
			return &result, nil
		}
	}
	return nil, fmt.Errorf("Could not find result with name %s for task run %s", reference.Result, reference.PipelineTask)
}

func (rs ResolvedResultRefs) getStringReplacements() map[string]string {
	replacements := map[string]string{}
	for _, r := range rs {
		replaceTarget := r.getReplaceTarget()
		replacements[replaceTarget] = r.Value.StringVal
	}
	return replacements
}

func (r *ResolvedResultRef) getReplaceTarget() string {
	return fmt.Sprintf("%s.%s.%s.%s", v1beta1.ResultTaskPart, r.ResultReference.PipelineTask, v1beta1.ResultResultPart, r.ResultReference.Result)
}
