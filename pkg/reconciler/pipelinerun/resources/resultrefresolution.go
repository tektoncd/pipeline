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
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	pipelineErrors "github.com/tektoncd/pipeline/pkg/apis/pipeline/errors"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// ErrInvalidTaskResultReference indicates that the reason for the failure status is that there
// is an invalid task result reference
var ErrInvalidTaskResultReference = pipelineErrors.WrapUserError(errors.New("Invalid task result reference"))

// ResolvedResultRefs represents all of the ResolvedResultRef for a pipeline task
type ResolvedResultRefs []*ResolvedResultRef

// ResolvedResultRef represents a result ref reference that has been fully resolved (value has been populated).
// If the value is from a Result, then the ResultReference will be populated to point to the ResultReference
// which resulted in the value
type ResolvedResultRef struct {
	Value           v1.ResultValue
	ResultReference v1.ResultRef
	FromTaskRun     string
	FromRun         string
}

// ResolveResultRef resolves any ResultReference that are found in the target ResolvedPipelineTask
func ResolveResultRef(pipelineRunState PipelineRunState, target *ResolvedPipelineTask) (ResolvedResultRefs, string, error) {
	resolvedResultRefs, pt, err := convertToResultRefs(pipelineRunState, target)
	if err != nil {
		return nil, pt, err
	}
	return removeDup(resolvedResultRefs), "", nil
}

// ResolveResultRefs resolves any ResultReference that are found in the target ResolvedPipelineTask
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

// validateArrayResultsIndex checks if the result array indexing reference is out of bound of the array size
func validateArrayResultsIndex(allResolvedResultRefs ResolvedResultRefs) error {
	for _, r := range allResolvedResultRefs {
		if r.Value.Type == v1.ParamTypeArray {
			if r.ResultReference.ResultsIndex != nil && *r.ResultReference.ResultsIndex >= len(r.Value.ArrayVal) {
				return fmt.Errorf("array Result Index %d for Task %s Result %s is out of bound of size %d", *r.ResultReference.ResultsIndex, r.ResultReference.PipelineTask, r.ResultReference.Result, len(r.Value.ArrayVal))
			}
		}
	}
	return nil
}

func removeDup(refs ResolvedResultRefs) ResolvedResultRefs {
	if refs == nil {
		return nil
	}
	resolvedResultRefByRef := make(map[v1.ResultRef]*ResolvedResultRef, len(refs))
	for _, resolvedResultRef := range refs {
		resolvedResultRefByRef[resolvedResultRef.ResultReference] = resolvedResultRef
	}
	deduped := make([]*ResolvedResultRef, 0, len(resolvedResultRefByRef))

	// Sort the resulting keys to produce a deterministic ordering.
	order := make([]v1.ResultRef, 0, len(refs))
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
func convertToResultRefs(pipelineRunState PipelineRunState, target *ResolvedPipelineTask) (ResolvedResultRefs, string, error) {
	var resolvedResultRefs ResolvedResultRefs
	for _, resultRef := range v1.PipelineTaskResultRefs(target.PipelineTask) {
		referencedPipelineTask := pipelineRunState.ToMap()[resultRef.PipelineTask]
		if referencedPipelineTask == nil {
			return nil, resultRef.PipelineTask, fmt.Errorf("could not find task %q referenced by result", resultRef.PipelineTask)
		}
		if !referencedPipelineTask.isSuccessful() && !referencedPipelineTask.isFailure() {
			return nil, resultRef.PipelineTask, fmt.Errorf("task %q referenced by result was not finished", referencedPipelineTask.PipelineTask.Name)
		}
		// Custom Task
		switch {
		case referencedPipelineTask.IsCustomTask():
			resolved, err := resolveCustomResultRef(referencedPipelineTask.CustomRuns, resultRef)
			if err != nil {
				return nil, resultRef.PipelineTask, err
			}
			resolvedResultRefs = append(resolvedResultRefs, resolved)
		default:
			// Matrixed referenced Pipeline Task
			if referencedPipelineTask.PipelineTask.IsMatrixed() {
				arrayValues, err := findResultValuesForMatrix(referencedPipelineTask, resultRef)
				if err != nil {
					return nil, resultRef.PipelineTask, err
				}
				for _, taskRun := range referencedPipelineTask.TaskRuns {
					resolved := createMatrixedTaskResultForParam(taskRun.Name, arrayValues, resultRef)
					resolvedResultRefs = append(resolvedResultRefs, resolved)
				}
			} else {
				// Regular PipelineTask
				resolved, err := resolveResultRef(referencedPipelineTask.TaskRuns, resultRef)
				if err != nil {
					return nil, resultRef.PipelineTask, err
				}
				resolvedResultRefs = append(resolvedResultRefs, resolved)
			}
		}
	}
	return resolvedResultRefs, "", nil
}

func resolveCustomResultRef(customRuns []*v1beta1.CustomRun, resultRef *v1.ResultRef) (*ResolvedResultRef, error) {
	customRun := customRuns[0]
	runName := customRun.GetObjectMeta().GetName()
	runValue, err := findRunResultForParam(customRun, resultRef)
	if err != nil {
		return nil, err
	}
	return &ResolvedResultRef{
		Value:           *paramValueFromCustomRunResult(runValue),
		FromTaskRun:     "",
		FromRun:         runName,
		ResultReference: *resultRef,
	}, nil
}

func paramValueFromCustomRunResult(result string) *v1.ParamValue {
	var arrayResult []string
	// for fan out array result, which is represented as string, we should make it to array type param value
	if err := json.Unmarshal([]byte(result), &arrayResult); err == nil && len(arrayResult) > 0 {
		if len(arrayResult) > 1 {
			return v1.NewStructuredValues(arrayResult[0], arrayResult[1:]...)
		}
		return &v1.ParamValue{
			Type:     v1.ParamTypeArray,
			ArrayVal: []string{arrayResult[0]},
		}
	}
	return v1.NewStructuredValues(result)
}

func resolveResultRef(taskRuns []*v1.TaskRun, resultRef *v1.ResultRef) (*ResolvedResultRef, error) {
	taskRun := taskRuns[0]
	taskRunName := taskRun.Name
	resultValue, err := findTaskResultForParam(taskRun, resultRef)
	if err != nil {
		return nil, err
	}
	return &ResolvedResultRef{
		Value:           resultValue,
		FromTaskRun:     taskRunName,
		FromRun:         "",
		ResultReference: *resultRef,
	}, nil
}

func findRunResultForParam(customRun *v1beta1.CustomRun, reference *v1.ResultRef) (string, error) {
	for _, result := range customRun.Status.Results {
		if result.Name == reference.Result {
			return result.Value, nil
		}
	}
	err := fmt.Errorf("%w: Could not find result with name %s for task %s", ErrInvalidTaskResultReference, reference.Result, reference.PipelineTask)
	return "", err
}

func findTaskResultForParam(taskRun *v1.TaskRun, reference *v1.ResultRef) (v1.ResultValue, error) {
	results := taskRun.Status.TaskRunStatusFields.Results
	for _, result := range results {
		if result.Name == reference.Result {
			return result.Value, nil
		}
	}
	err := fmt.Errorf("%w: Could not find result with name %s for task %s", ErrInvalidTaskResultReference, reference.Result, reference.PipelineTask)
	return v1.ResultValue{}, err
}

// findResultValuesForMatrix checks the resultsCache of the referenced Matrixed TaskRun to retrieve the resultValues and aggregate them into
// arrayValues. If the resultCache is empty, it will create the ResultCache so that the results can be accessed in subsequent tasks.
func findResultValuesForMatrix(referencedPipelineTask *ResolvedPipelineTask, resultRef *v1.ResultRef) (v1.ParamValue, error) {
	var resultsCache *map[string][]string
	if len(referencedPipelineTask.ResultsCache) == 0 {
		cache := createResultsCacheMatrixedTaskRuns(referencedPipelineTask)
		resultsCache = &cache
		referencedPipelineTask.ResultsCache = *resultsCache
	}
	if arrayValues, ok := referencedPipelineTask.ResultsCache[resultRef.Result]; ok {
		return v1.ParamValue{
			Type:     v1.ParamTypeArray,
			ArrayVal: arrayValues,
		}, nil
	}
	err := fmt.Errorf("%w: Could not find result with name %s for task %s", ErrInvalidTaskResultReference, resultRef.Result, resultRef.PipelineTask)
	return v1.ParamValue{}, err
}

func createMatrixedTaskResultForParam(taskRunName string, paramValue v1.ParamValue, resultRef *v1.ResultRef) *ResolvedResultRef {
	return &ResolvedResultRef{
		Value:           paramValue,
		FromTaskRun:     taskRunName,
		FromRun:         "",
		ResultReference: *resultRef,
	}
}

func (rs ResolvedResultRefs) getStringReplacements() map[string]string {
	replacements := map[string]string{}
	for _, r := range rs {
		switch r.Value.Type {
		case v1.ParamTypeArray:
			for i := range len(r.Value.ArrayVal) {
				for _, target := range r.getReplaceTargetfromArrayIndex(i) {
					replacements[target] = r.Value.ArrayVal[i]
				}
			}
		case v1.ParamTypeObject:
			for key, element := range r.Value.ObjectVal {
				for _, target := range r.getReplaceTargetfromObjectKey(key) {
					replacements[target] = element
				}
			}

		case v1.ParamTypeString:
			fallthrough
		default:
			for _, target := range r.getReplaceTarget() {
				replacements[target] = r.Value.StringVal
			}
		}
	}
	return replacements
}

func (rs ResolvedResultRefs) getArrayReplacements() map[string][]string {
	replacements := map[string][]string{}
	for _, r := range rs {
		if r.Value.Type == v1.ParamType(v1.ResultsTypeArray) {
			for _, target := range r.getReplaceTarget() {
				replacements[target] = r.Value.ArrayVal
			}
		}
	}
	return replacements
}

func (rs ResolvedResultRefs) getObjectReplacements() map[string]map[string]string {
	replacements := map[string]map[string]string{}
	for _, r := range rs {
		if r.Value.Type == v1.ParamType(v1.ResultsTypeObject) {
			for _, target := range r.getReplaceTarget() {
				replacements[target] = r.Value.ObjectVal
			}
		}
	}
	return replacements
}

func (r *ResolvedResultRef) getReplaceTarget() []string {
	return []string{
		fmt.Sprintf("%s.%s.%s.%s", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result),
		fmt.Sprintf("%s.%s.%s[%q]", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result),
		fmt.Sprintf("%s.%s.%s['%s']", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result),
	}
}

func (r *ResolvedResultRef) getReplaceTargetfromArrayIndex(idx int) []string {
	return []string{
		fmt.Sprintf("%s.%s.%s.%s[%d]", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result, idx),
		fmt.Sprintf("%s.%s.%s[%q][%d]", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result, idx),
		fmt.Sprintf("%s.%s.%s['%s'][%d]", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result, idx),
	}
}

func (r *ResolvedResultRef) getReplaceTargetfromObjectKey(key string) []string {
	return []string{
		fmt.Sprintf("%s.%s.%s.%s.%s", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result, key),
		fmt.Sprintf("%s.%s.%s[%q][%s]", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result, key),
		fmt.Sprintf("%s.%s.%s['%s'][%s]", v1.ResultTaskPart, r.ResultReference.PipelineTask, v1.ResultResultPart, r.ResultReference.Result, key),
	}
}
