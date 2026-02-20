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
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"github.com/tektoncd/pipeline/pkg/workspace"
)

const (
	// resultsParseNumber is the value of how many parts we split from result reference. e.g.  tasks.<taskName>.results.<objectResultName>
	resultsParseNumber = 4
	// objectElementResultsParseNumber is the value of how many parts we split from
	// object attribute result reference. e.g.  tasks.<taskName>.results.<objectResultName>.<individualAttribute>
	objectElementResultsParseNumber = 5
	// objectIndividualVariablePattern is the reference pattern for object individual keys params.<object_param_name>.<key_name>
	objectIndividualVariablePattern = "params.%s.%s"
	// maxParamReferenceDepth limits recursive parameter resolution to prevent stack overflow attacks
	maxParamReferenceDepth = 10
)

var paramPatterns = []string{
	"params.%s",
	"params[%q]",
	"params['%s']",
}

// ApplyParameters applies the params from a PipelineRun.Params to a PipelineSpec.
//
// It resolves parameter defaults that reference other parameters using recursive resolution
// with caching. The algorithm supports dependency chains up to 10 levels deep, handles
// parameters in any declaration order, and detects circular dependencies.
//
// Example - Container image build pipeline:
//
//	# PipelineRun provides:
//	params:
//	  - name: repo
//	    value: "my-app"
//
//	# PipelineSpec defaults (unresolved references):
//	params:
//	  - name: registry
//	    default: "gcr.io/my-project"
//	  - name: tag
//	    default: "v1.0"
//	  - name: image-name
//	    default: "$(params.registry)/$(params.repo):$(params.tag)"
//	  - name: build-args
//	    type: array
//	    default: ["IMAGE=$(params.image-name)", "VERSION=$(params.tag)"]
//
//	# After ApplyParameters resolves to:
//	  repo: "my-app"                                    # from PipelineRun
//	  registry: "gcr.io/my-project"                     # from default
//	  tag: "v1.0"                                       # from default
//	  image-name: "gcr.io/my-project/my-app:v1.0"       # resolved recursively
//	  build-args: ["IMAGE=gcr.io/my-project/my-app:v1.0", "VERSION=v1.0"]  # resolved from strings
//
// The function uses a 6-phase approach:
//  1. Extract params from PipelineRun (already resolved values)
//  2. Build unresolved maps for defaults (by type: string, array, object)
//  3. Recursively resolve string params with dependency tracking (max depth: 10)
//  4. Resolve array params with substitution
//  5. Resolve object params with substitution
//  6. Apply all replacements to PipelineSpec
func ApplyParameters(p *v1.PipelineSpec, pr *v1.PipelineRun) (*v1.PipelineSpec, error) {
	// ===== Phase 1: Get params from PipelineRun =====
	resolvedStringParams, resolvedArrayParams, resolvedObjectParams := paramsFromPipelineRun(pr)

	// ===== Phase 2: Build unresolved maps for defaults =====
	unresolvedStringParams := buildUnresolvedStringParamDefaults(p.Params, resolvedStringParams)
	unresolvedArrayParams := buildUnresolvedArrayParamDefaults(p.Params, resolvedArrayParams)
	unresolvedObjectParams := buildUnresolvedObjectParamDefaults(p.Params, resolvedObjectParams)

	// ===== Phase 3: Recursively resolve string params =====
	visiting := map[string]bool{}
	for paramKey, paramValue := range unresolvedStringParams {
		if _, exist := resolvedStringParams[paramKey]; exist {
			continue
		}

		if err := resolveStringParamRecursively(
			paramKey,
			paramValue,
			resolvedStringParams,
			unresolvedStringParams,
			visiting,
			0,
		); err != nil {
			return nil, err
		}
	}

	// ===== Phase 4: Resolve array params =====
	for paramKey, paramValue := range unresolvedArrayParams {
		resolveArrayParam(paramKey, paramValue, resolvedStringParams, resolvedArrayParams)
	}

	// ===== Phase 5: Resolve object params =====
	for paramKey, paramValue := range unresolvedObjectParams {
		resolveObjectParam(paramKey, paramValue, resolvedStringParams, resolvedObjectParams)
	}

	// ===== Phase 6: Apply all replacements to PipelineSpec =====
	return ApplyReplacements(p, resolvedStringParams, resolvedArrayParams, resolvedObjectParams), nil
}

func paramsFromPipelineRun(pr *v1.PipelineRun) (map[string]string, map[string][]string, map[string]map[string]string) {
	// stringReplacements is used for standard single-string stringReplacements,
	// while arrayReplacements/objectReplacements contains arrays/objects that need to be further processed.
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}

	for _, p := range pr.Spec.Params {
		switch p.Value.Type {
		case v1.ParamTypeArray:
			for _, pattern := range paramPatterns {
				for i := range len(p.Value.ArrayVal) {
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

// buildUnresolvedStringParamDefaults builds a map of parameter defaults that haven't been provided by the PipelineRun.
// It filters for string-type parameters only and excludes those already present in resolvedStringParams.
func buildUnresolvedStringParamDefaults(params []v1.ParamSpec, resolvedStringParams map[string]string) map[string]string {
	result := map[string]string{}

	for _, param := range params {
		if param.Type != v1.ParamTypeString && param.Type != "" {
			continue
		}

		if param.Default == nil {
			continue
		}

		if paramExists(param.Name, resolvedStringParams) {
			continue
		}

		addPatternEntry(param.Name, param.Default.StringVal, result)
	}

	return result
}

type paramValue interface {
	string | []string | map[string]string
}

func paramExists[T paramValue](paramName string, bucket map[string]T) bool {
	for _, pattern := range paramPatterns {
		key := fmt.Sprintf(pattern, paramName)
		if _, exists := bucket[key]; exists {
			return true
		}
	}
	return false
}

func addPatternEntry[T paramValue](key string, value T, bucket map[string]T) {
	for _, pattern := range paramPatterns {
		patternKey := fmt.Sprintf(pattern, key)
		bucket[patternKey] = value
	}
}

// buildUnresolvedArrayParamDefaults builds a map of parameter defaults that haven't been provided by the PipelineRun.
// It filters for array-type parameters only and excludes those already present in resolvedArrayParams.
func buildUnresolvedArrayParamDefaults(params []v1.ParamSpec, resolvedArrayParams map[string][]string) map[string][]string {
	result := map[string][]string{}

	for _, param := range params {
		if param.Type != v1.ParamTypeArray {
			continue
		}

		if param.Default == nil {
			continue
		}

		if paramExists(param.Name, resolvedArrayParams) {
			continue
		}

		addPatternEntry(param.Name, param.Default.ArrayVal, result)
	}

	return result
}

// buildUnresolvedObjectParamDefaults builds a map of parameter defaults that haven't been provided by the PipelineRun.
// It filters for object-type parameters only and excludes those already present in resolvedObjectParams.
func buildUnresolvedObjectParamDefaults(params []v1.ParamSpec, resolvedObjectParams map[string]map[string]string) map[string]map[string]string {
	result := map[string]map[string]string{}

	for _, param := range params {
		if param.Type != v1.ParamTypeObject {
			continue
		}

		if param.Default == nil {
			continue
		}

		if paramExists(param.Name, resolvedObjectParams) {
			continue
		}

		addPatternEntry(param.Name, param.Default.ObjectVal, result)
	}

	return result
}

// resolveStringParamRecursively resolves a string parameter by recursively resolving any $(params.*) references.
// It handles dependency chains of any depth and detects circular dependencies.
//
// Example: If registry="docker.io", image-url="$(params.registry)/app" resolves to "docker.io/app".
// For chains like base="docker.io" → registry="$(params.base)/org" → url="$(params.registry)/app",
// it recursively resolves base first, then registry, then url, resulting in "docker.io/org/app".
//
// Returns error if circular dependency or undefined parameter reference is detected.
func resolveStringParamRecursively(
	paramKey string,
	paramValue string,
	resolvedStringParams,
	unresolvedStringParams map[string]string,
	visiting map[string]bool,
	depth uint,
) error {
	// Circular dependency check
	if visiting[paramKey] {
		return fmt.Errorf("parameter resolution failed: circular dependency detected in param %q", paramKey)
	}
	visiting[paramKey] = true
	defer delete(visiting, paramKey)

	// Protection against stack overflow attack
	if depth > maxParamReferenceDepth {
		return fmt.Errorf("parameter resolution failed: maximum recursion depth (%d) exceeded for param %q", maxParamReferenceDepth, paramKey)
	}

	paramRefsFromParamValue := extractParamReferences(paramValue)

	// If value has no references it is already resolved and needs no
	// substitution, store it as resolved
	if len(paramRefsFromParamValue) == 0 {
		resolvedStringParams[paramKey] = paramValue
		return nil
	}

	// Resolve each reference
	for _, paramRef := range paramRefsFromParamValue {
		if _, exist := resolvedStringParams[paramRef]; exist {
			continue
		}

		// Non-existing parameter reference check
		unresolvedValue, exist := unresolvedStringParams[paramRef]
		if !exist {
			return fmt.Errorf("parameter resolution failed: param %q references undefined param %q", paramKey, paramRef)
		}

		if err := resolveStringParamRecursively(
			paramRef,
			unresolvedValue,
			resolvedStringParams,
			unresolvedStringParams,
			visiting,
			depth+1,
		); err != nil {
			return err
		}
	}

	// Substitute references (executes after recursion resolves all dependencies)
	// e.g., resolvedStringParams["params.image-url"] = "$(params.registry)/app" becomes "docker.io/app"
	resolvedStringParams[paramKey] = substitution.ApplyReplacements(paramValue, resolvedStringParams)

	return nil
}

// extractParamReferences extracts all $(params.*) variable references from a parameter value string.
// Example: "$(params.registry)/app:$(params.tag)" → ["params.registry", "params.tag"]
func extractParamReferences(paramValue string) []string {
	param := v1.Param{Value: *v1.NewStructuredValues(paramValue)}
	variableRefs, ok := param.GetVarSubstitutionExpressions()
	if !ok {
		return []string{}
	}

	// Filter references to non-parameter variables
	paramRefs := make([]string, 0, len(variableRefs))
	for _, variable := range variableRefs {
		if strings.HasPrefix(variable, "params.") {
			paramRefs = append(paramRefs, variable)
		}
	}
	return paramRefs
}

// resolveArrayParam resolves an array parameter by applying string parameter substitutions to each element and adds
// indexed access to enable references like $(params.tags[0]) in other parameters.
//
// Example: If registry="docker.io", tags=["$(params.registry)/app:v1", "alpine"] resolves to:
//   - resolvedArrayParam["params.tags"] = ["docker.io/app:v1", "alpine"]
//   - resolvedStringParams["params.tags[0]"] = "docker.io/app:v1"
//   - resolvedStringParams["params.tags[1]"] = "alpine"
func resolveArrayParam(paramKey string, paramValue []string, resolvedStringParams map[string]string, resolvedArrayParam map[string][]string) {
	resolvedValues := make([]string, len(paramValue))
	for key, value := range paramValue {
		resolvedValues[key] = substitution.ApplyReplacements(value, resolvedStringParams)
	}
	resolvedArrayParam[paramKey] = resolvedValues

	// Add indexed access to enable references like $(params.tags[0]) in other params
	for i, value := range resolvedValues {
		key := fmt.Sprintf("%s[%d]", paramKey, i)
		resolvedStringParams[key] = value
	}
}

// resolveObjectParam resolves an object parameter by applying string parameter substitutions to each value and adds
// keyed access to enable references like $(params.config.host) in other parameters.
//
// Example: If registry="docker.io", config={"host": "$(params.registry)", "port": "5000"} resolves to:
//   - resolvedObjectParams["params.config"] = {"host": "docker.io", "port": "5000"}
//   - resolvedStringParams["params.config.host"] = "docker.io"
//   - resolvedStringParams["params.config.port"] = "5000"
func resolveObjectParam(paramKey string, paramValue map[string]string, resolvedStringParams map[string]string, resolvedObjectParams map[string]map[string]string) {
	resolvedValues := make(map[string]string, len(paramValue))
	for key, value := range paramValue {
		resolvedValues[key] = substitution.ApplyReplacements(value, resolvedStringParams)
	}
	resolvedObjectParams[paramKey] = resolvedValues

	// Add keyed access to enable references like $(params.config.host) in other params
	for key, value := range resolvedValues {
		stringsKey := fmt.Sprintf("%s.%s", paramKey, key)
		resolvedStringParams[stringsKey] = value
	}
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
	for i := range spec.Tasks {
		spec.Tasks[i].DisplayName = substitution.ApplyReplacements(spec.Tasks[i].DisplayName, GetContextReplacements(pipelineName, pr))
	}
	for i := range spec.Finally {
		spec.Finally[i].DisplayName = substitution.ApplyReplacements(spec.Finally[i].DisplayName, GetContextReplacements(pipelineName, pr))
	}
	return ApplyReplacements(spec, GetContextReplacements(pipelineName, pr), map[string][]string{}, map[string]map[string]string{})
}

// filterMatrixContextVar returns a list of params which contain any matrix context variables such as
// $(tasks.<pipelineTaskName>.matrix.length) and $(tasks.<pipelineTaskName>.matrix.<resultName>.length)
func filterMatrixContextVar(params v1.Params) v1.Params {
	var filteredParams v1.Params
	for _, param := range params {
		if expressions, ok := param.GetVarSubstitutionExpressions(); ok {
			for _, expression := range expressions {
				// tasks.<pipelineTaskName>.matrix.length
				// tasks.<pipelineTaskName>.matrix.<resultName>.length
				subExpressions := strings.Split(expression, ".")
				if len(subExpressions) >= 4 && subExpressions[2] == "matrix" && subExpressions[len(subExpressions)-1] == "length" {
					filteredParams = append(filteredParams, param)
				}
			}
		}
	}
	return filteredParams
}

// ApplyPipelineTaskContexts applies the substitution from $(context.pipelineTask.*) with the specified values.
// Uses "0" as a default if a value is not available as well as matrix context variables
// $(tasks.<pipelineTaskName>.matrix.length) and $(tasks.<pipelineTaskName>.matrix.<resultName>.length)
func ApplyPipelineTaskContexts(pt *v1.PipelineTask, pipelineRunStatus v1.PipelineRunStatus, facts *PipelineRunFacts) *v1.PipelineTask {
	pt = pt.DeepCopy()
	var pipelineTaskName string
	var resultName string
	var matrixLength int

	replacements := map[string]string{
		"context.pipelineTask.retries": strconv.Itoa(pt.Retries),
	}

	filteredParams := filterMatrixContextVar(pt.Params)

	for _, p := range filteredParams {
		pipelineTaskName, resultName = p.ParseTaskandResultName()
		// find the referenced pipelineTask to count the matrix combinations
		if pipelineTaskName != "" && pipelineRunStatus.PipelineSpec != nil {
			for _, task := range pipelineRunStatus.PipelineSpec.Tasks {
				if task.Name == pipelineTaskName {
					matrixLength = task.Matrix.CountCombinations()
					replacements["tasks."+pipelineTaskName+".matrix.length"] = strconv.Itoa(matrixLength)
					continue
				}
			}
		}
		// find the resultName from the ResultsCache
		if pipelineTaskName != "" && resultName != "" {
			for _, pt := range facts.State {
				if pt.PipelineTask.Name == pipelineTaskName {
					if len(pt.ResultsCache) == 0 {
						pt.ResultsCache = createResultsCacheMatrixedTaskRuns(pt)
					}
					resultLength := len(pt.ResultsCache[resultName])
					replacements["tasks."+pipelineTaskName+".matrix."+resultName+".length"] = strconv.Itoa(resultLength)
					continue
				}
			}
		}
	}

	pt.Params = pt.Params.ReplaceVariables(replacements, map[string][]string{}, map[string]map[string]string{})
	if pt.IsMatrixed() {
		pt.Matrix.Params = pt.Matrix.Params.ReplaceVariables(replacements, map[string][]string{}, map[string]map[string]string{})
		for i := range pt.Matrix.Include {
			pt.Matrix.Include[i].Params = pt.Matrix.Include[i].Params.ReplaceVariables(replacements, map[string][]string{}, map[string]map[string]string{})
		}
	}
	pt.DisplayName = substitution.ApplyReplacements(pt.DisplayName, replacements)
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
			if pipelineTask.TaskRef != nil {
				if pipelineTask.TaskRef.Params != nil {
					pipelineTask.TaskRef.Params = pipelineTask.TaskRef.Params.ReplaceVariables(stringReplacements, arrayReplacements, objectReplacements)
				}
				pipelineTask.TaskRef.Name = substitution.ApplyReplacements(pipelineTask.TaskRef.Name, stringReplacements)
			}
			pipelineTask.DisplayName = substitution.ApplyReplacements(pipelineTask.DisplayName, stringReplacements)
			for i, workspace := range pipelineTask.Workspaces {
				pipelineTask.Workspaces[i].SubPath = substitution.ApplyReplacements(workspace.SubPath, stringReplacements)
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
			if pipelineTask.TaskRef != nil {
				if pipelineTask.TaskRef.Params != nil {
					pipelineTask.TaskRef.Params = pipelineTask.TaskRef.Params.ReplaceVariables(replacements, nil, nil)
				}
				pipelineTask.TaskRef.Name = substitution.ApplyReplacements(pipelineTask.TaskRef.Name, replacements)
			}
			pipelineTask.DisplayName = substitution.ApplyReplacements(pipelineTask.DisplayName, replacements)
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

// replaceVariablesInPipelineTasks handles variable replacement for a slice of PipelineTasks in-place
func replaceVariablesInPipelineTasks(tasks []v1.PipelineTask, replacements map[string]string,
	arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) {
	for i := range tasks {
		tasks[i].Params = tasks[i].Params.ReplaceVariables(replacements, arrayReplacements, objectReplacements)
		if tasks[i].IsMatrixed() {
			tasks[i].Matrix.Params = tasks[i].Matrix.Params.ReplaceVariables(replacements, arrayReplacements, nil)
			for j := range tasks[i].Matrix.Include {
				tasks[i].Matrix.Include[j].Params = tasks[i].Matrix.Include[j].Params.ReplaceVariables(replacements, nil, nil)
			}
		} else {
			tasks[i].DisplayName = substitution.ApplyReplacements(tasks[i].DisplayName, replacements)
		}
		for j := range tasks[i].Workspaces {
			tasks[i].Workspaces[j].SubPath = substitution.ApplyReplacements(tasks[i].Workspaces[j].SubPath, replacements)
		}
		tasks[i].When = tasks[i].When.ReplaceVariables(replacements, arrayReplacements)
		if tasks[i].TaskRef != nil {
			if tasks[i].TaskRef.Params != nil {
				tasks[i].TaskRef.Params = tasks[i].TaskRef.Params.ReplaceVariables(replacements, arrayReplacements, objectReplacements)
			}
			tasks[i].TaskRef.Name = substitution.ApplyReplacements(tasks[i].TaskRef.Name, replacements)
		}
		tasks[i].OnError = v1.PipelineTaskOnErrorType(substitution.ApplyReplacements(string(tasks[i].OnError), replacements))
		tasks[i] = propagateParams(tasks[i], replacements, arrayReplacements, objectReplacements)
	}
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(p *v1.PipelineSpec, replacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) *v1.PipelineSpec {
	p = p.DeepCopy()

	// Replace variables in Tasks and Finally tasks
	replaceVariablesInPipelineTasks(p.Tasks, replacements, arrayReplacements, objectReplacements)
	replaceVariablesInPipelineTasks(p.Finally, replacements, arrayReplacements, objectReplacements)

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
		t.TaskSpec.TaskSpec = *resources.ApplyReplacements(&t.TaskSpec.TaskSpec, stringReplacementsDup, arrayReplacementsDup, objectReplacementsDup)
	} else {
		t.TaskSpec.TaskSpec = *resources.ApplyReplacements(&t.TaskSpec.TaskSpec, stringReplacements, arrayReplacements, objectReplacements)
	}
	return t
}

// ApplyResultsToWorkspaceBindings applies results from TaskRuns to  WorkspaceBindings in a PipelineRun. It replaces placeholders in
// various binding types with values from TaskRun results.
func ApplyResultsToWorkspaceBindings(trResults map[string][]v1.TaskRunResult, pr *v1.PipelineRun) {
	stringReplacements := map[string]string{}
	for taskName, taskResults := range trResults {
		for _, res := range taskResults {
			switch res.Type {
			case v1.ResultsTypeString:
				stringReplacements[fmt.Sprintf("tasks.%s.results.%s", taskName, res.Name)] = res.Value.StringVal
			case v1.ResultsTypeArray:
				continue
			case v1.ResultsTypeObject:
				for k, v := range res.Value.ObjectVal {
					stringReplacements[fmt.Sprintf("tasks.%s.results.%s.%s", taskName, res.Name, k)] = v
				}
			}
		}
	}

	pr.Spec.Workspaces = workspace.ReplaceWorkspaceBindingsVars(pr.Spec.Workspaces, stringReplacements)
}

// PropagateResults propagate the result of the completed task to the unfinished task that is not explicitly specify in the params
func PropagateResults(rpt *ResolvedPipelineTask, runStates PipelineRunState) {
	if rpt.ResolvedTask == nil || rpt.ResolvedTask.TaskSpec == nil {
		return
	}
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	for taskName, taskResults := range runStates.GetTaskRunsResults() {
		for _, res := range taskResults {
			switch res.Type {
			case v1.ResultsTypeString:
				stringReplacements[fmt.Sprintf("tasks.%s.results.%s", taskName, res.Name)] = res.Value.StringVal
			case v1.ResultsTypeArray:
				arrayReplacements[fmt.Sprintf("tasks.%s.results.%s", taskName, res.Name)] = res.Value.ArrayVal
			case v1.ResultsTypeObject:
				for k, v := range res.Value.ObjectVal {
					stringReplacements[fmt.Sprintf("tasks.%s.results.%s.%s", taskName, res.Name, k)] = v
				}
			}
		}
	}
	rpt.ResolvedTask.TaskSpec = resources.ApplyReplacements(rpt.ResolvedTask.TaskSpec, stringReplacements, arrayReplacements, map[string]map[string]string{})
}

// PropagateArtifacts propagates artifact values from previous task runs into the TaskSpec of the current task.
func PropagateArtifacts(rpt *ResolvedPipelineTask, runStates PipelineRunState) error {
	if rpt.ResolvedTask == nil || rpt.ResolvedTask.TaskSpec == nil {
		return nil
	}
	stringReplacements := map[string]string{}
	for taskName, artifacts := range runStates.GetTaskRunsArtifacts() {
		if artifacts != nil {
			for i, input := range artifacts.Inputs {
				ib, err := json.Marshal(input.Values)
				if err != nil {
					return err
				}
				stringReplacements[fmt.Sprintf("tasks.%s.inputs.%s", taskName, input.Name)] = string(ib)
				if i == 0 {
					stringReplacements[fmt.Sprintf("tasks.%s.inputs", taskName)] = string(ib)
				}
			}
			for i, output := range artifacts.Outputs {
				ob, err := json.Marshal(output.Values)
				if err != nil {
					return err
				}
				stringReplacements[fmt.Sprintf("tasks.%s.outputs.%s", taskName, output.Name)] = string(ob)
				if i == 0 {
					stringReplacements[fmt.Sprintf("tasks.%s.outputs", taskName)] = string(ob)
				}
			}
		}
	}
	rpt.ResolvedTask.TaskSpec = resources.ApplyReplacements(rpt.ResolvedTask.TaskSpec, stringReplacements, map[string][]string{}, map[string]map[string]string{})
	return nil
}

// ApplyTaskResultsToPipelineResults applies the results of completed TasksRuns and Runs to a Pipeline's
// list of PipelineResults, returning the computed set of PipelineRunResults. References to
// non-existent TaskResults or failed TaskRuns or Runs result in a PipelineResult being considered invalid
// and omitted from the returned slice. A nil slice is returned if no results are passed in or all
// results are invalid.
func ApplyTaskResultsToPipelineResults(
	results []v1.PipelineResult,
	taskRunResults map[string][]v1.TaskRunResult,
	customTaskResults map[string][]v1beta1.CustomRunResult,
	taskstatus map[string]string,
) ([]v1.PipelineRunResult, error) {
	var runResults []v1.PipelineRunResult
	var invalidPipelineResults []string

	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}
	for _, pipelineResult := range results {
		variablesInPipelineResult, _ := pipelineResult.GetVarSubstitutionExpressions()
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

			if (variableParts[0] != v1.ResultTaskPart && variableParts[0] != v1.ResultFinallyPart) || variableParts[2] != v1beta1.ResultResultPart {
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
						if status != v1.TaskRunReasonSuccessful.String() {
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
						if status != v1.TaskRunReasonSuccessful.String() {
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
		return runResults, fmt.Errorf("invalid pipelineresults %v, the referenced results don't exist", invalidPipelineResults)
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

// ApplyParametersToWorkspaceBindings applies parameters from PipelineSpec and  PipelineRun to the WorkspaceBindings in a PipelineRun. It replaces
// placeholders in various binding types with values from provided parameters.
func ApplyParametersToWorkspaceBindings(pr *v1.PipelineRun) {
	parameters, _, _ := paramsFromPipelineRun(pr)
	pr.Spec.Workspaces = workspace.ReplaceWorkspaceBindingsVars(pr.Spec.Workspaces, parameters)
}
