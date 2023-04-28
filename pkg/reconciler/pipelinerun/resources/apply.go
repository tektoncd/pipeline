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
	"context"
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/substitution"
)

const (
	// resultsParseNumber is the value of how many parts we split from result reference. e.g.  tasks.<taskName>.results.<objectResultName>
	resultsParseNumber = 4
	// objectElementResultsParseNumber is the value of how many parts we split from
	// object attribute result reference. e.g.  tasks.<taskName>.results.<objectResultName>.<individualAttribute>
	objectElementResultsParseNumber = 5
	// objectIndividualVariablePattern is the reference pattern for object individual keys params.<object_param_name>.<key_name>
	objectIndividualVariablePattern = "params.%s.%s"
)

var (
	paramPatterns = []string{
		"params.%s",
		"params[%q]",
		"params['%s']",
	}
)

// ApplyParameters applies the params from a PipelineRun.Params to a PipelineSpec.
func ApplyParameters(ctx context.Context, p *v1.PipelineSpec, pr *v1.PipelineRun) *v1.PipelineSpec {
	// This assumes that the PipelineRun inputs have been validated against what the Pipeline requests.

	// stringReplacements is used for standard single-string stringReplacements,
	// while arrayReplacements/objectReplacements contains arrays/objects that need to be further processed.
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}

	// Set all the default stringReplacements
	for _, p := range p.Params {
		if p.Default != nil {
			switch p.Default.Type {
			case v1.ParamTypeArray:
				for _, pattern := range paramPatterns {
					for i := 0; i < len(p.Default.ArrayVal); i++ {
						stringReplacements[fmt.Sprintf(pattern+"[%d]", p.Name, i)] = p.Default.ArrayVal[i]
					}
					arrayReplacements[fmt.Sprintf(pattern, p.Name)] = p.Default.ArrayVal
				}
			case v1.ParamTypeObject:
				for _, pattern := range paramPatterns {
					objectReplacements[fmt.Sprintf(pattern, p.Name)] = p.Default.ObjectVal
				}
				for k, v := range p.Default.ObjectVal {
					stringReplacements[fmt.Sprintf(objectIndividualVariablePattern, p.Name, k)] = v
				}
			case v1.ParamTypeString:
				fallthrough
			default:
				for _, pattern := range paramPatterns {
					stringReplacements[fmt.Sprintf(pattern, p.Name)] = p.Default.StringVal
				}
			}
		}
	}
	// Set and overwrite params with the ones from the PipelineRun
	prStrings, prArrays, prObjects := paramsFromPipelineRun(ctx, pr)

	for k, v := range prStrings {
		stringReplacements[k] = v
	}
	for k, v := range prArrays {
		arrayReplacements[k] = v
	}
	for k, v := range prObjects {
		objectReplacements[k] = v
	}

	return ApplyReplacements(p, stringReplacements, arrayReplacements, objectReplacements)
}

func paramsFromPipelineRun(ctx context.Context, pr *v1.PipelineRun) (map[string]string, map[string][]string, map[string]map[string]string) {
	// stringReplacements is used for standard single-string stringReplacements,
	// while arrayReplacements/objectReplacements contains arrays/objects that need to be further processed.
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}

	for _, p := range pr.Spec.Params {
		switch p.Value.Type {
		case v1.ParamTypeArray:
			for _, pattern := range paramPatterns {
				for i := 0; i < len(p.Value.ArrayVal); i++ {
					stringReplacements[fmt.Sprintf(pattern+"[%d]", p.Name, i)] = p.Value.ArrayVal[i]
				}
				arrayReplacements[fmt.Sprintf(pattern, p.Name)] = p.Value.ArrayVal
			}
		case v1.ParamTypeObject:
			for _, pattern := range paramPatterns {
				objectReplacements[fmt.Sprintf(pattern, p.Name)] = p.Value.ObjectVal
			}
			for k, v := range p.Value.ObjectVal {
				stringReplacements[fmt.Sprintf(objectIndividualVariablePattern, p.Name, k)] = v
			}
		case v1.ParamTypeString:
			fallthrough
		default:
			for _, pattern := range paramPatterns {
				stringReplacements[fmt.Sprintf(pattern, p.Name)] = p.Value.StringVal
			}
		}
	}

	return stringReplacements, arrayReplacements, objectReplacements
}

// GetContextReplacements returns the pipelineRun context which can be used to replace context variables in the specifications
func GetContextReplacements(pipelineName string, pr *v1.PipelineRun) map[string]string {
	return map[string]string{
		"context.pipelineRun.name":      pr.Name,
		"context.pipeline.name":         pipelineName,
		"context.pipelineRun.namespace": pr.Namespace,
		"context.pipelineRun.uid":       string(pr.ObjectMeta.UID),
	}
}

// ApplyContexts applies the substitution from $(context.(pipelineRun|pipeline).*) with the specified values.
// Currently supports only name substitution. Uses "" as a default if name is not specified.
func ApplyContexts(spec *v1.PipelineSpec, pipelineName string, pr *v1.PipelineRun) *v1.PipelineSpec {
	return ApplyReplacements(spec, GetContextReplacements(pipelineName, pr), map[string][]string{}, map[string]map[string]string{})
}

// ApplyPipelineTaskContexts applies the substitution from $(context.pipelineTask.*) with the specified values.
// Uses "0" as a default if a value is not available.
func ApplyPipelineTaskContexts(pt *v1.PipelineTask) *v1.PipelineTask {
	pt = pt.DeepCopy()
	replacements := map[string]string{
		"context.pipelineTask.retries": strconv.Itoa(pt.Retries),
	}
	pt.Params = pt.Params.ReplaceVariables(replacements, map[string][]string{}, map[string]map[string]string{})
	if pt.IsMatrixed() {
		pt.Matrix.Params = pt.Params.ReplaceVariables(replacements, map[string][]string{}, map[string]map[string]string{})
		for i := range pt.Matrix.Include {
			pt.Matrix.Include[i].Params = pt.Matrix.Include[i].Params.ReplaceVariables(replacements, map[string][]string{}, map[string]map[string]string{})
		}
	}
	return pt
}

// ApplyTaskResults applies the ResolvedResultRef to each PipelineTask.Params and Pipeline.When in targets
func ApplyTaskResults(targets PipelineRunState, resolvedResultRefs ResolvedResultRefs) {
	stringReplacements := resolvedResultRefs.getStringReplacements()
	arrayReplacements := resolvedResultRefs.getArrayReplacements()
	objectReplacements := resolvedResultRefs.getObjectReplacements()
	for _, resolvedPipelineRunTask := range targets {
		if resolvedPipelineRunTask.PipelineTask != nil {
			pipelineTask := resolvedPipelineRunTask.PipelineTask.DeepCopy()
			pipelineTask.Params = pipelineTask.Params.ReplaceVariables(stringReplacements, arrayReplacements, objectReplacements)
			if pipelineTask.IsMatrixed() {
				// Matrixed pipeline results replacements support:
				// 1. String replacements from string, array or object results
				// 2. array replacements from array results are supported
				pipelineTask.Matrix.Params = pipelineTask.Matrix.Params.ReplaceVariables(stringReplacements, arrayReplacements, nil)
				for i := range pipelineTask.Matrix.Include {
					// matrix include parameters can only be type string
					pipelineTask.Matrix.Include[i].Params = pipelineTask.Matrix.Include[i].Params.ReplaceVariables(stringReplacements, nil, nil)
				}
			}
			pipelineTask.When = pipelineTask.When.ReplaceVariables(stringReplacements, arrayReplacements)
			if pipelineTask.TaskRef != nil && pipelineTask.TaskRef.Params != nil {
				pipelineTask.TaskRef.Params = pipelineTask.TaskRef.Params.ReplaceVariables(stringReplacements, arrayReplacements, objectReplacements)
			}
			resolvedPipelineRunTask.PipelineTask = pipelineTask
		}
	}
}

// ApplyPipelineTaskStateContext replaces context variables referring to execution status with the specified status
func ApplyPipelineTaskStateContext(state PipelineRunState, replacements map[string]string) {
	for _, resolvedPipelineRunTask := range state {
		if resolvedPipelineRunTask.PipelineTask != nil {
			pipelineTask := resolvedPipelineRunTask.PipelineTask.DeepCopy()
			pipelineTask.Params = pipelineTask.Params.ReplaceVariables(replacements, nil, nil)
			pipelineTask.When = pipelineTask.When.ReplaceVariables(replacements, nil)
			if pipelineTask.TaskRef != nil && pipelineTask.TaskRef.Params != nil {
				pipelineTask.TaskRef.Params = pipelineTask.TaskRef.Params.ReplaceVariables(replacements, nil, nil)
			}
			resolvedPipelineRunTask.PipelineTask = pipelineTask
		}
	}
}

// ApplyWorkspaces replaces workspace variables in the given pipeline spec with their
// concrete values.
func ApplyWorkspaces(p *v1.PipelineSpec, pr *v1.PipelineRun) *v1.PipelineSpec {
	p = p.DeepCopy()
	replacements := map[string]string{}
	for _, declaredWorkspace := range p.Workspaces {
		key := fmt.Sprintf("workspaces.%s.bound", declaredWorkspace.Name)
		replacements[key] = "false"
	}
	for _, boundWorkspace := range pr.Spec.Workspaces {
		key := fmt.Sprintf("workspaces.%s.bound", boundWorkspace.Name)
		replacements[key] = "true"
	}
	return ApplyReplacements(p, replacements, map[string][]string{}, map[string]map[string]string{})
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(p *v1.PipelineSpec, replacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) *v1.PipelineSpec {
	p = p.DeepCopy()

	for i := range p.Tasks {
		p.Tasks[i].Params = p.Tasks[i].Params.ReplaceVariables(replacements, arrayReplacements, objectReplacements)
		if p.Tasks[i].IsMatrixed() {
			p.Tasks[i].Matrix.Params = p.Tasks[i].Matrix.Params.ReplaceVariables(replacements, arrayReplacements, nil)
			for j := range p.Tasks[i].Matrix.Include {
				p.Tasks[i].Matrix.Include[j].Params = p.Tasks[i].Matrix.Include[j].Params.ReplaceVariables(replacements, nil, nil)
			}
		}
		for j := range p.Tasks[i].Workspaces {
			p.Tasks[i].Workspaces[j].SubPath = substitution.ApplyReplacements(p.Tasks[i].Workspaces[j].SubPath, replacements)
		}
		p.Tasks[i].When = p.Tasks[i].When.ReplaceVariables(replacements, arrayReplacements)
		if p.Tasks[i].TaskRef != nil && p.Tasks[i].TaskRef.Params != nil {
			p.Tasks[i].TaskRef.Params = p.Tasks[i].TaskRef.Params.ReplaceVariables(replacements, arrayReplacements, objectReplacements)
		}
		p.Tasks[i] = propagateParams(p.Tasks[i], replacements, arrayReplacements, objectReplacements)
	}

	for i := range p.Finally {
		p.Finally[i].Params = p.Finally[i].Params.ReplaceVariables(replacements, arrayReplacements, objectReplacements)
		if p.Finally[i].IsMatrixed() {
			p.Finally[i].Matrix.Params = p.Finally[i].Matrix.Params.ReplaceVariables(replacements, arrayReplacements, nil)
			for j := range p.Finally[i].Matrix.Include {
				p.Finally[i].Matrix.Include[j].Params = p.Finally[i].Matrix.Include[j].Params.ReplaceVariables(replacements, nil, nil)
			}
		}
		for j := range p.Finally[i].Workspaces {
			p.Finally[i].Workspaces[j].SubPath = substitution.ApplyReplacements(p.Finally[i].Workspaces[j].SubPath, replacements)
		}
		p.Finally[i].When = p.Finally[i].When.ReplaceVariables(replacements, arrayReplacements)
		if p.Finally[i].TaskRef != nil && p.Finally[i].TaskRef.Params != nil {
			p.Finally[i].TaskRef.Params = p.Finally[i].TaskRef.Params.ReplaceVariables(replacements, arrayReplacements, objectReplacements)
		}
		p.Finally[i] = propagateParams(p.Finally[i], replacements, arrayReplacements, objectReplacements)
	}

	return p
}

// propagateParams returns a Pipeline Task spec that is the same as the input Pipeline Task spec, but with
// all parameter replacements from `stringReplacements`, `arrayReplacements`, and `objectReplacements` substituted.
// It does not modify `stringReplacements`, `arrayReplacements`, or `objectReplacements`.
func propagateParams(t v1.PipelineTask, stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) v1.PipelineTask {
	if t.TaskSpec == nil {
		return t
	}
	// check if there are task parameters defined that match the params at pipeline level
	if len(t.Params) > 0 {
		stringReplacementsDup := make(map[string]string)
		arrayReplacementsDup := make(map[string][]string)
		objectReplacementsDup := make(map[string]map[string]string)
		for k, v := range stringReplacements {
			stringReplacementsDup[k] = v
		}
		for k, v := range arrayReplacements {
			arrayReplacementsDup[k] = v
		}
		for k, v := range objectReplacements {
			objectReplacementsDup[k] = v
		}
		for _, par := range t.Params {
			for _, pattern := range paramPatterns {
				checkName := fmt.Sprintf(pattern, par.Name)
				// Scoping. Task Params will replace Pipeline Params
				if _, ok := stringReplacementsDup[checkName]; ok {
					stringReplacementsDup[checkName] = par.Value.StringVal
				}
				if _, ok := arrayReplacementsDup[checkName]; ok {
					arrayReplacementsDup[checkName] = par.Value.ArrayVal
				}
				if _, ok := objectReplacementsDup[checkName]; ok {
					objectReplacementsDup[checkName] = par.Value.ObjectVal
					for k, v := range par.Value.ObjectVal {
						stringReplacementsDup[fmt.Sprintf(objectIndividualVariablePattern, par.Name, k)] = v
					}
				}
			}
		}
		t.TaskSpec.TaskSpec = *resources.ApplyReplacements(&t.TaskSpec.TaskSpec, stringReplacementsDup, arrayReplacementsDup)
	} else {
		t.TaskSpec.TaskSpec = *resources.ApplyReplacements(&t.TaskSpec.TaskSpec, stringReplacements, arrayReplacements)
	}
	return t
}

// ApplyTaskResultsToPipelineResults applies the results of completed TasksRuns and Runs to a Pipeline's
// list of PipelineResults, returning the computed set of PipelineRunResults. References to
// non-existent TaskResults or failed TaskRuns or Runs result in a PipelineResult being considered invalid
// and omitted from the returned slice. A nil slice is returned if no results are passed in or all
// results are invalid.
func ApplyTaskResultsToPipelineResults(
	_ context.Context,
	results []v1.PipelineResult,
	taskRunResults map[string][]v1.TaskRunResult,
	customTaskResults map[string][]v1beta1.CustomRunResult,
	taskstatus map[string]string) ([]v1.PipelineRunResult, error) {
	var runResults []v1.PipelineRunResult
	var invalidPipelineResults []string

	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}
	for _, pipelineResult := range results {
		variablesInPipelineResult, _ := v1.GetVarSubstitutionExpressionsForPipelineResult(pipelineResult)
		if len(variablesInPipelineResult) == 0 {
			continue
		}
		validPipelineResult := true
		for _, variable := range variablesInPipelineResult {
			if _, isMemoized := stringReplacements[variable]; isMemoized {
				continue
			}
			if _, isMemoized := arrayReplacements[variable]; isMemoized {
				continue
			}
			if _, isMemoized := objectReplacements[variable]; isMemoized {
				continue
			}
			variableParts := strings.Split(variable, ".")

			if (variableParts[0] != v1beta1.ResultTaskPart && variableParts[0] != v1beta1.ResultFinallyPart) || variableParts[2] != v1beta1.ResultResultPart {
				validPipelineResult = false
				invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
				continue
			}
			switch len(variableParts) {
			// For string result: tasks.<taskName>.results.<stringResultName>
			// For array result: tasks.<taskName>.results.<arrayResultName>[*], tasks.<taskName>.results.<arrayResultName>[i]
			// For object result: tasks.<taskName>.results.<objectResultName>[*],
			case resultsParseNumber:
				taskName, resultName := variableParts[1], variableParts[3]
				resultName, stringIdx := v1.ParseResultName(resultName)
				if resultValue := taskResultValue(taskName, resultName, taskRunResults); resultValue != nil {
					switch resultValue.Type {
					case v1.ParamTypeString:
						stringReplacements[variable] = resultValue.StringVal
					case v1.ParamTypeArray:
						if stringIdx != "*" {
							intIdx, _ := strconv.Atoi(stringIdx)
							if intIdx < len(resultValue.ArrayVal) {
								stringReplacements[variable] = resultValue.ArrayVal[intIdx]
							} else {
								// referred array index out of bound
								invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
								validPipelineResult = false
							}
						} else {
							arrayReplacements[substitution.StripStarVarSubExpression(variable)] = resultValue.ArrayVal
						}
					case v1.ParamTypeObject:
						objectReplacements[substitution.StripStarVarSubExpression(variable)] = resultValue.ObjectVal
					}
				} else if resultValue := runResultValue(taskName, resultName, customTaskResults); resultValue != nil {
					stringReplacements[variable] = *resultValue
				} else {
					// if the task is not successful (e.g. skipped or failed) and the results is missing, don't return error
					if status, ok := taskstatus[PipelineTaskStatusPrefix+taskName+PipelineTaskStatusSuffix]; ok {
						if status != v1beta1.TaskRunReasonSuccessful.String() {
							validPipelineResult = false
							continue
						}
					}
					// referred result name is not existent
					invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
					validPipelineResult = false
				}
			// For object type result: tasks.<taskName>.results.<objectResultName>.<individualAttribute>
			case objectElementResultsParseNumber:
				taskName, resultName, objectKey := variableParts[1], variableParts[3], variableParts[4]
				resultName, _ = v1.ParseResultName(resultName)
				if resultValue := taskResultValue(taskName, resultName, taskRunResults); resultValue != nil {
					if _, ok := resultValue.ObjectVal[objectKey]; ok {
						stringReplacements[variable] = resultValue.ObjectVal[objectKey]
					} else {
						// referred object key is not existent
						invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
						validPipelineResult = false
					}
				} else {
					// if the task is not successful (e.g. skipped or failed) and the results is missing, don't return error
					if status, ok := taskstatus[PipelineTaskStatusPrefix+taskName+PipelineTaskStatusSuffix]; ok {
						if status != v1beta1.TaskRunReasonSuccessful.String() {
							validPipelineResult = false
							continue
						}
					}
					// referred result name is not existent
					invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
					validPipelineResult = false
				}
			default:
				invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
				validPipelineResult = false
			}
		}
		if validPipelineResult {
			finalValue := pipelineResult.Value
			finalValue.ApplyReplacements(stringReplacements, arrayReplacements, objectReplacements)
			runResults = append(runResults, v1.PipelineRunResult{
				Name:  pipelineResult.Name,
				Value: finalValue,
			})
		}
	}

	if len(invalidPipelineResults) > 0 {
		return runResults, fmt.Errorf("invalid pipelineresults %v, the referred results don't exist", invalidPipelineResults)
	}

	return runResults, nil
}

// taskResultValue returns the result value for a given pipeline task name and result name in a map of TaskRunResults for
// pipeline task names. It returns nil if either the pipeline task name isn't present in the map, or if there is no
// result with the result name in the pipeline task name's slice of results.
func taskResultValue(taskName string, resultName string, taskResults map[string][]v1.TaskRunResult) *v1.ResultValue {
	for _, trResult := range taskResults[taskName] {
		if trResult.Name == resultName {
			return &trResult.Value
		}
	}
	return nil
}

// runResultValue returns the result value for a given pipeline task name and result name in a map of RunResults for
// pipeline task names. It returns nil if either the pipeline task name isn't present in the map, or if there is no
// result with the result name in the pipeline task name's slice of results.
func runResultValue(taskName string, resultName string, runResults map[string][]v1beta1.CustomRunResult) *string {
	for _, runResult := range runResults[taskName] {
		if runResult.Name == resultName {
			return &runResult.Value
		}
	}
	return nil
}
