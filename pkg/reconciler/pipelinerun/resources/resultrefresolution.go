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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
	FromRun         string
}

// ResolveResultRef resolves any ResultReference that are found in the target ResolvedPipelineRunTask
func ResolveResultRef(pipelineRunState PipelineRunState, target *ResolvedPipelineRunTask) (ResolvedResultRefs, string, error) {
	resolvedResultRefs, pt, err := convertToResultRefs(pipelineRunState, target)
	if err != nil {
		return nil, pt, err
	}
	return removeDup(resolvedResultRefs), "", nil
}

// ResolveResultRefs resolves any ResultReference that are found in the target ResolvedPipelineRunTask
func ResolveResultRefs(pipelineRunState PipelineRunState, targets PipelineRunState) (ResolvedResultRefs, string, error) {
	var allResolvedResultRefs ResolvedResultRefs
	for _, target := range targets {
		resolvedResultRefs, pt, err := convertToResultRefs(pipelineRunState, target)
		if err != nil {
			return nil, pt, err
		}
		allResolvedResultRefs = append(allResolvedResultRefs, resolvedResultRefs...)
	}
	return removeDup(allResolvedResultRefs), "", nil
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

func extractResultRefs(expressions []string, pipelineRunState PipelineRunState) (ResolvedResultRefs, error) {
	resultRefs := v1beta1.NewResultRefs(expressions)
	var resolvedResultRefs ResolvedResultRefs
	for _, resultRef := range resultRefs {
		resolvedResultRef, _, err := resolveResultRef(pipelineRunState, resultRef)
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

// convertToResultRefs walks a PipelineTask looking for result references. If any are
// found they are resolved to a value by searching pipelineRunState. The list of resolved
// references are returned. If an error is encountered due to an invalid result reference
// then a nil list and error is returned instead.
func convertToResultRefs(pipelineRunState PipelineRunState, target *ResolvedPipelineRunTask) (ResolvedResultRefs, string, error) {
	var resolvedResultRefs ResolvedResultRefs
	for _, ref := range v1beta1.PipelineTaskResultRefs(target.PipelineTask) {
		resolved, pt, err := resolveResultRef(pipelineRunState, ref)
		if err != nil {
			return nil, pt, err
		}
		resolvedResultRefs = append(resolvedResultRefs, resolved)
	}
	return resolvedResultRefs, "", nil
}

func resolveResultRef(pipelineState PipelineRunState, resultRef *v1beta1.ResultRef) (*ResolvedResultRef, string, error) {
	referencedPipelineTask := pipelineState.ToMap()[resultRef.PipelineTask]
	if referencedPipelineTask == nil {
		return nil, resultRef.PipelineTask, fmt.Errorf("could not find task %q referenced by result", resultRef.PipelineTask)
	}
	if !referencedPipelineTask.IsSuccessful() {
		return nil, resultRef.PipelineTask, fmt.Errorf("task %q referenced by result was not successful", referencedPipelineTask.PipelineTask.Name)
	}

	var runName, taskRunName, resultValue string
	var err error
	if referencedPipelineTask.IsCustomTask() {
		runName = referencedPipelineTask.Run.Name
		resultValue, err = findRunResultForParam(referencedPipelineTask.Run, resultRef)
		if err != nil {
			return nil, resultRef.PipelineTask, err
		}
	} else {
		taskRunName = referencedPipelineTask.TaskRun.Name
		resultValue, err = findTaskResultForParam(referencedPipelineTask.TaskRun, resultRef)
		if err != nil {
			return nil, resultRef.PipelineTask, err
		}
	}

	return &ResolvedResultRef{
		Value:           *v1beta1.NewArrayOrString(resultValue),
		FromTaskRun:     taskRunName,
		FromRun:         runName,
		ResultReference: *resultRef,
	}, "", nil
}

func findRunResultForParam(run *v1alpha1.Run, reference *v1beta1.ResultRef) (string, error) {
	results := run.Status.Results
	for _, result := range results {
		if result.Name == reference.Result {
			return result.Value, nil
		}
	}
	return "", fmt.Errorf("Could not find result with name %s for task %s", reference.Result, reference.PipelineTask)
}

func findTaskResultForParam(taskRun *v1beta1.TaskRun, reference *v1beta1.ResultRef) (string, error) {
	results := taskRun.Status.TaskRunStatusFields.TaskRunResults
	for _, result := range results {
		if result.Name == reference.Result {
			return result.Value, nil
		}
	}
	return "", fmt.Errorf("Could not find result with name %s for task %s", reference.Result, reference.PipelineTask)
}

func (rs ResolvedResultRefs) getStringReplacements() map[string]string {
	replacements := map[string]string{}
	for _, r := range rs {
		for _, target := range r.getReplaceTarget() {
			replacements[target] = r.Value.StringVal
		}
	}
	return replacements
}

func (r *ResolvedResultRef) getReplaceTarget() []string {
	return []string{
		fmt.Sprintf("%s.%s.%s.%s", v1beta1.ResultTaskPart, r.ResultReference.PipelineTask, v1beta1.ResultResultPart, r.ResultReference.Result),
		fmt.Sprintf("%s.%s.%s[%q]", v1beta1.ResultTaskPart, r.ResultReference.PipelineTask, v1beta1.ResultResultPart, r.ResultReference.Result),
		fmt.Sprintf("%s.%s.%s['%s']", v1beta1.ResultTaskPart, r.ResultReference.PipelineTask, v1beta1.ResultResultPart, r.ResultReference.Result),
	}
}
