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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/run/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/substitution"
)

const (
	// resultsParseNumber is the value of how many parts we split from result reference. e.g.  tasks.<taskName>.results.<objectResultName>
	resultsParseNumber = 4
	// objectElementResultsParseNumber is the value of how many parts we split from
	// object attribute result reference. e.g.  tasks.<taskName>.results.<objectResultName>.<individualAttribute>
	objectElementResultsParseNumber = 5
)

// ApplyParameters applies the params from a PipelineRun.Params to a PipelineSpec.
func ApplyParameters(ctx context.Context, p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) *v1beta1.PipelineSpec {
	// This assumes that the PipelineRun inputs have been validated against what the Pipeline requests.

	// stringReplacements is used for standard single-string stringReplacements,
	// while arrayReplacements/objectReplacements contains arrays/objects that need to be further processed.
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}
	cfg := config.FromContextOrDefaults(ctx)

	patterns := []string{
		"params.%s",
		"params[%q]",
		"params['%s']",
	}

	// reference pattern for object individual keys params.<object_param_name>.<key_name>
	objectIndividualVariablePattern := "params.%s.%s"

	// Set all the default stringReplacements
	for _, p := range p.Params {
		if p.Default != nil {
			switch p.Default.Type {
			case v1beta1.ParamTypeArray:
				for _, pattern := range patterns {
					// array indexing for param is alpha feature
					if cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
						for i := 0; i < len(p.Default.ArrayVal); i++ {
							stringReplacements[fmt.Sprintf(pattern+"[%d]", p.Name, i)] = p.Default.ArrayVal[i]
						}
					}
					arrayReplacements[fmt.Sprintf(pattern, p.Name)] = p.Default.ArrayVal
				}
			case v1beta1.ParamTypeObject:
				for _, pattern := range patterns {
					objectReplacements[fmt.Sprintf(pattern, p.Name)] = p.Default.ObjectVal
				}
				for k, v := range p.Default.ObjectVal {
					stringReplacements[fmt.Sprintf(objectIndividualVariablePattern, p.Name, k)] = v
				}
			default:
				for _, pattern := range patterns {
					stringReplacements[fmt.Sprintf(pattern, p.Name)] = p.Default.StringVal
				}
			}
		}
	}
	// Set and overwrite params with the ones from the PipelineRun
	for _, p := range pr.Spec.Params {
		switch p.Value.Type {
		case v1beta1.ParamTypeArray:
			for _, pattern := range patterns {
				// array indexing for param is alpha feature
				if cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
					for i := 0; i < len(p.Value.ArrayVal); i++ {
						stringReplacements[fmt.Sprintf(pattern+"[%d]", p.Name, i)] = p.Value.ArrayVal[i]
					}
				}
				arrayReplacements[fmt.Sprintf(pattern, p.Name)] = p.Value.ArrayVal
			}
		case v1beta1.ParamTypeObject:
			for _, pattern := range patterns {
				objectReplacements[fmt.Sprintf(pattern, p.Name)] = p.Value.ObjectVal
			}
			for k, v := range p.Value.ObjectVal {
				stringReplacements[fmt.Sprintf(objectIndividualVariablePattern, p.Name, k)] = v
			}
		default:
			for _, pattern := range patterns {
				stringReplacements[fmt.Sprintf(pattern, p.Name)] = p.Value.StringVal
			}
		}
	}

	return ApplyReplacements(ctx, p, stringReplacements, arrayReplacements, objectReplacements)
}

// ApplyContexts applies the substitution from $(context.(pipelineRun|pipeline).*) with the specified values.
// Currently supports only name substitution. Uses "" as a default if name is not specified.
func ApplyContexts(ctx context.Context, spec *v1beta1.PipelineSpec, pipelineName string, pr *v1beta1.PipelineRun) *v1beta1.PipelineSpec {
	replacements := map[string]string{
		"context.pipelineRun.name":      pr.Name,
		"context.pipeline.name":         pipelineName,
		"context.pipelineRun.namespace": pr.Namespace,
		"context.pipelineRun.uid":       string(pr.ObjectMeta.UID),
	}
	return ApplyReplacements(ctx, spec, replacements, map[string][]string{}, map[string]map[string]string{})
}

// ApplyPipelineTaskContexts applies the substitution from $(context.pipelineTask.*) with the specified values.
// Uses "0" as a default if a value is not available.
func ApplyPipelineTaskContexts(pt *v1beta1.PipelineTask) *v1beta1.PipelineTask {
	pt = pt.DeepCopy()
	replacements := map[string]string{
		"context.pipelineTask.retries": strconv.Itoa(pt.Retries),
	}
	pt.Params = replaceParamValues(pt.Params, replacements, map[string][]string{}, map[string]map[string]string{})
	pt.Matrix = replaceParamValues(pt.Matrix, replacements, map[string][]string{}, map[string]map[string]string{})
	return pt
}

// ApplyTaskResults applies the ResolvedResultRef to each PipelineTask.Params and Pipeline.WhenExpressions in targets
func ApplyTaskResults(targets PipelineRunState, resolvedResultRefs ResolvedResultRefs) {
	stringReplacements := resolvedResultRefs.getStringReplacements()
	arrayReplacements := resolvedResultRefs.getArrayReplacements()
	objectReplacements := resolvedResultRefs.getObjectReplacements()
	for _, resolvedPipelineRunTask := range targets {
		if resolvedPipelineRunTask.PipelineTask != nil {
			pipelineTask := resolvedPipelineRunTask.PipelineTask.DeepCopy()
			pipelineTask.Params = replaceParamValues(pipelineTask.Params, stringReplacements, arrayReplacements, objectReplacements)
			pipelineTask.Matrix = replaceParamValues(pipelineTask.Matrix, stringReplacements, nil, nil)
			pipelineTask.WhenExpressions = pipelineTask.WhenExpressions.ReplaceWhenExpressionsVariables(stringReplacements, arrayReplacements)
			resolvedPipelineRunTask.PipelineTask = pipelineTask
		}
	}
}

// ApplyPipelineTaskStateContext replaces context variables referring to execution status with the specified status
func ApplyPipelineTaskStateContext(state PipelineRunState, replacements map[string]string) {
	for _, resolvedPipelineRunTask := range state {
		if resolvedPipelineRunTask.PipelineTask != nil {
			pipelineTask := resolvedPipelineRunTask.PipelineTask.DeepCopy()
			pipelineTask.Params = replaceParamValues(pipelineTask.Params, replacements, nil, nil)
			pipelineTask.WhenExpressions = pipelineTask.WhenExpressions.ReplaceWhenExpressionsVariables(replacements, nil)
			resolvedPipelineRunTask.PipelineTask = pipelineTask
		}
	}
}

// ApplyWorkspaces replaces workspace variables in the given pipeline spec with their
// concrete values.
func ApplyWorkspaces(ctx context.Context, p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) *v1beta1.PipelineSpec {
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
	return ApplyReplacements(ctx, p, replacements, map[string][]string{}, map[string]map[string]string{})
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(ctx context.Context, p *v1beta1.PipelineSpec, replacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) *v1beta1.PipelineSpec {
	p = p.DeepCopy()

	for i := range p.Tasks {
		p.Tasks[i].Params = replaceParamValues(p.Tasks[i].Params, replacements, arrayReplacements, objectReplacements)
		p.Tasks[i].Matrix = replaceParamValues(p.Tasks[i].Matrix, replacements, arrayReplacements, objectReplacements)
		for j := range p.Tasks[i].Workspaces {
			p.Tasks[i].Workspaces[j].SubPath = substitution.ApplyReplacements(p.Tasks[i].Workspaces[j].SubPath, replacements)
		}
		p.Tasks[i].WhenExpressions = p.Tasks[i].WhenExpressions.ReplaceWhenExpressionsVariables(replacements, arrayReplacements)
		p.Tasks[i], replacements, arrayReplacements, objectReplacements = propagateParams(ctx, p.Tasks[i], replacements, arrayReplacements, objectReplacements)
	}

	for i := range p.Finally {
		p.Finally[i].Params = replaceParamValues(p.Finally[i].Params, replacements, arrayReplacements, objectReplacements)
		p.Finally[i].Matrix = replaceParamValues(p.Finally[i].Matrix, replacements, arrayReplacements, objectReplacements)
		p.Finally[i].WhenExpressions = p.Finally[i].WhenExpressions.ReplaceWhenExpressionsVariables(replacements, arrayReplacements)
	}

	return p
}

func propagateParams(ctx context.Context, t v1beta1.PipelineTask, replacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) (v1beta1.PipelineTask, map[string]string, map[string][]string, map[string]map[string]string) {
	if t.TaskSpec != nil && config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha" {
		patterns := []string{
			"params.%s",
			"params[%q]",
			"params['%s']",
		}

		// reference pattern for object individual keys params.<object_param_name>.<key_name>
		objectIndividualVariablePattern := "params.%s.%s"

		// check if there are task parameters defined that match the params at pipeline level
		if len(t.Params) > 0 {
			for _, par := range t.Params {
				for _, pattern := range patterns {
					checkName := fmt.Sprintf(pattern, par.Name)
					// Scoping. Task Params will replace Pipeline Params
					if _, ok := replacements[checkName]; ok {
						replacements[checkName] = par.Value.StringVal
					}
					if _, ok := arrayReplacements[checkName]; ok {
						arrayReplacements[checkName] = par.Value.ArrayVal
					}
					if _, ok := objectReplacements[checkName]; ok {
						objectReplacements[checkName] = par.Value.ObjectVal
						for k, v := range par.Value.ObjectVal {
							replacements[fmt.Sprintf(objectIndividualVariablePattern, par.Name, k)] = v
						}
					}
				}
			}
		}
		t.TaskSpec.TaskSpec = *resources.ApplyReplacements(&t.TaskSpec.TaskSpec, replacements, arrayReplacements)
	}
	return t, replacements, arrayReplacements, objectReplacements
}

func replaceParamValues(params []v1beta1.Param, stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) []v1beta1.Param {
	for i := range params {
		params[i].Value.ApplyReplacements(stringReplacements, arrayReplacements, objectReplacements)
	}
	return params
}

// ApplyTaskResultsToPipelineResults applies the results of completed TasksRuns and Runs to a Pipeline's
// list of PipelineResults, returning the computed set of PipelineRunResults. References to
// non-existent TaskResults or failed TaskRuns or Runs result in a PipelineResult being considered invalid
// and omitted from the returned slice. A nil slice is returned if no results are passed in or all
// results are invalid.
func ApplyTaskResultsToPipelineResults(
	results []v1beta1.PipelineResult,
	taskRunResults map[string][]v1beta1.TaskRunResult,
	customTaskResults map[string][]v1alpha1.RunResult) ([]v1beta1.PipelineRunResult, error) {
	var runResults []v1beta1.PipelineRunResult
	var invalidPipelineResults []string
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}
	for _, pipelineResult := range results {
		variablesInPipelineResult, _ := v1beta1.GetVarSubstitutionExpressionsForPipelineResult(pipelineResult)
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
			if variableParts[0] != v1beta1.ResultTaskPart || variableParts[2] != v1beta1.ResultResultPart {
				validPipelineResult = false
				invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
				continue
			}
			switch len(variableParts) {
			// For string result: tasks.<taskName>.results.<objectResultName>
			// For array result: tasks.<taskName>.results.<objectResultName>[*], tasks.<taskName>.results.<objectResultName>[i]
			// For object result: tasks.<taskName>.results.<objectResultName>[*],
			case resultsParseNumber:
				taskName, resultName := variableParts[1], variableParts[3]
				resultName, stringIdx := v1beta1.ParseResultName(resultName)
				if resultValue := taskResultValue(taskName, resultName, taskRunResults); resultValue != nil {
					switch resultValue.Type {
					case v1beta1.ParamTypeString:
						stringReplacements[variable] = resultValue.StringVal
					case v1beta1.ParamTypeArray:
						if stringIdx != "*" {
							intIdx, _ := strconv.Atoi(stringIdx)
							if intIdx < len(resultValue.ArrayVal) {
								stringReplacements[variable] = resultValue.ArrayVal[intIdx]
							} else {
								invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
								validPipelineResult = false
							}
						} else {
							arrayReplacements[substitution.StripStarVarSubExpression(variable)] = resultValue.ArrayVal
						}
					case v1beta1.ParamTypeObject:
						objectReplacements[substitution.StripStarVarSubExpression(variable)] = resultValue.ObjectVal
					}
				} else if resultValue := runResultValue(taskName, resultName, customTaskResults); resultValue != nil {
					stringReplacements[variable] = *resultValue
				} else {
					// referred array index out of bound
					invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
					validPipelineResult = false
				}
			// For object type result: tasks.<taskName>.results.<objectResultName>.<individualAttribute>
			case objectElementResultsParseNumber:
				taskName, resultName, objectKey := variableParts[1], variableParts[3], variableParts[4]
				resultName, _ = v1beta1.ParseResultName(resultName)
				if resultValue := taskResultValue(taskName, resultName, taskRunResults); resultValue != nil {
					if _, ok := resultValue.ObjectVal[objectKey]; ok {
						stringReplacements[variable] = resultValue.ObjectVal[objectKey]
					} else {
						// referred object key is not existent
						invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
						validPipelineResult = false
					}
				}
			default:
				invalidPipelineResults = append(invalidPipelineResults, pipelineResult.Name)
				validPipelineResult = false
			}
		}
		if validPipelineResult {
			finalValue := pipelineResult.Value
			finalValue.ApplyReplacements(stringReplacements, arrayReplacements, objectReplacements)
			runResults = append(runResults, v1beta1.PipelineRunResult{
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
func taskResultValue(taskName string, resultName string, taskResults map[string][]v1beta1.TaskRunResult) *v1beta1.ArrayOrString {
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
func runResultValue(taskName string, resultName string, runResults map[string][]v1alpha1.RunResult) *string {
	for _, runResult := range runResults[taskName] {
		if runResult.Name == resultName {
			return &runResult.Value
		}
	}
	return nil
}
