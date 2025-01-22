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
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/internal/artifactref"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/container"
	"github.com/tektoncd/pipeline/pkg/internal/resultref"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"github.com/tektoncd/pipeline/pkg/workspace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// objectIndividualVariablePattern is the reference pattern for object individual keys params.<object_param_name>.<key_name>
	objectIndividualVariablePattern = "params.%s.%s"
)

var (
	paramPatterns = []string{
		"params.%s",
		"params[%q]",
		"params['%s']",
		// FIXME(vdemeester) Remove that with deprecating v1beta1
		"inputs.params.%s",
	}

	substitutionToParamNamePatterns = []string{
		`^params\.(\w+)$`,
		`^params\["([^"]+)"\]$`,
		`^params\['([^']+)'\]$`,
		// FIXME(vdemeester) Remove that with deprecating v1beta1
		`^inputs\.params\.(\w+)$`,
	}

	paramIndexRegexPatterns = []string{
		`\$\(params.%s\[([0-9]*)*\*?\]\)`,
		`\$\(params\[%q\]\[([0-9]*)*\*?\]\)`,
		`\$\(params\['%s'\]\[([0-9]*)*\*?\]\)`,
	}
)

// applyStepActionParameters applies the params from the Task and the underlying Step to the referenced StepAction.
// substitution order:
// 1. taskrun parameter values
// 2. step-provided parameter values
// 3. default values that reference other parameters
// 4. simple default values
// 5. step result references
func applyStepActionParameters(step *v1.Step, spec *v1.TaskSpec, tr *v1.TaskRun, stepParams v1.Params, defaults []v1.ParamSpec) (*v1.Step, error) {
	// 1. taskrun parameter substitutions to step parameters
	if stepParams != nil {
		stringR, arrayR, objectR := getTaskParameters(spec, tr, spec.Params...)
		stepParams = stepParams.ReplaceVariables(stringR, arrayR, objectR)
	}
	// 2. step provided parameters
	stepProvidedParams := make(map[string]v1.ParamValue)
	for _, sp := range stepParams {
		stepProvidedParams[sp.Name] = sp.Value
	}
	// 3,4. get replacements from default params (both referenced and simple)
	stringReplacements, arrayReplacements, objectReplacements := replacementsFromDefaultParams(defaults)
	// process parameter values in order of precedence (2,3,4)
	processedParams := make([]v1.Param, 0, len(defaults))
	for _, p := range defaults {
		// 2. if parameter is provided by the step, use that value
		if value, exists := stepProvidedParams[p.Name]; exists {
			processedParams = append(processedParams, v1.Param{
				Name:  p.Name,
				Value: value,
			})
			continue
		}
		// 3,4. if parameter has a default value, process it
		if p.Default != nil {
			defaultValue := *p.Default
			// 3. if default value references other parameters, substitute them
			if strings.Contains(defaultValue.StringVal, "$(params.") {
				matches, _ := substitution.ExtractVariableExpressions(defaultValue.StringVal, "params")
				resolvedValue := defaultValue.StringVal

				for _, match := range matches {
					paramName := strings.TrimSuffix(strings.TrimPrefix(match, "$(params."), ")")

					for _, processed := range processedParams {
						if processed.Name == paramName {
							resolvedValue = strings.ReplaceAll(resolvedValue, match, processed.Value.StringVal)
							break
						}
					}
				}
				defaultValue.StringVal = resolvedValue
			}
			// 4. apply simple default value
			processedParams = append(processedParams, v1.Param{
				Name:  p.Name,
				Value: defaultValue,
			})
		}
	}
	// apply the processed parameters (2,3,4)
	procStringReplacements, procArrayReplacements, procObjectReplacements := replacementsFromParams(processedParams)
	// merge replacements from defaults and processed params
	for k, v := range procStringReplacements {
		stringReplacements[k] = v
	}
	for k, v := range procArrayReplacements {
		arrayReplacements[k] = v
	}
	for k, v := range procObjectReplacements {
		if objectReplacements[k] == nil {
			objectReplacements[k] = v
		} else {
			for key, val := range v {
				objectReplacements[k][key] = val
			}
		}
	}

	// Check if there are duplicate keys in the replacements
	// If the same key is present in both stringReplacements and arrayReplacements, it means
	// that the default value and the passed value have different types.
	if err := checkForDuplicateKeys(stringReplacements, arrayReplacements); err != nil {
		return nil, err
	}

	// 5. handle step result references in parameters last
	stepResultReplacements, err := replacementsFromStepResults(step, stepParams, defaults)
	if err != nil {
		return nil, err
	}

	// 5. merge step result replacements into string replacements last
	for k, v := range stepResultReplacements {
		stringReplacements[k] = v
	}

	// apply all replacements at once
	container.ApplyStepReplacements(step, stringReplacements, arrayReplacements)

	return step, nil
}

// checkForDuplicateKeys checks if there are duplicate keys in the replacements
func checkForDuplicateKeys(stringReplacements map[string]string, arrayReplacements map[string][]string) error {
	keys := make([]string, 0, len(stringReplacements))
	for k := range stringReplacements {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := arrayReplacements[k]; ok {
			paramName := paramNameFromReplacementKey(k)
			return fmt.Errorf("invalid parameter substitution: %s. Please check the types of the default value and the passed value", paramName)
		}
	}
	return nil
}

// paramNameFromReplacementKey returns the param name from the replacement key in best effort
func paramNameFromReplacementKey(key string) string {
	for _, regexPattern := range substitutionToParamNamePatterns {
		re := regexp.MustCompile(regexPattern)
		if matches := re.FindStringSubmatch(key); matches != nil {
			return matches[1]
		}
	}
	// If no match is found, return the key
	return key
}

// findArrayIndexParamUsage finds the array index in a string using array param substitution
func findArrayIndexParamUsage(s string, paramName string, stepName string, resultName string, stringReplacements map[string]string) map[string]string {
	for _, pattern := range paramIndexRegexPatterns {
		arrayIndexingRegex := regexp.MustCompile(fmt.Sprintf(pattern, paramName))
		matches := arrayIndexingRegex.FindAllStringSubmatch(s, -1)
		for _, match := range matches {
			if len(match) == 2 {
				key := strings.TrimSuffix(strings.TrimPrefix(match[0], "$("), ")")
				if match[1] != "" {
					stringReplacements[key] = fmt.Sprintf("$(steps.%s.results.%s[%s])", stepName, resultName, match[1])
				}
			}
		}
	}
	return stringReplacements
}

// replacementsArrayIdxStepResults looks for Step Result array usage with index in the Step's command, args, env and script.
func replacementsArrayIdxStepResults(step *v1.Step, paramName string, stepName string, resultName string) map[string]string {
	stringReplacements := map[string]string{}
	for _, c := range step.Command {
		stringReplacements = findArrayIndexParamUsage(c, paramName, stepName, resultName, stringReplacements)
	}
	for _, a := range step.Args {
		stringReplacements = findArrayIndexParamUsage(a, paramName, stepName, resultName, stringReplacements)
	}
	for _, e := range step.Env {
		stringReplacements = findArrayIndexParamUsage(e.Value, paramName, stepName, resultName, stringReplacements)
	}
	return stringReplacements
}

// replacementsFromStepResults generates string replacements for params whose values is a variable substitution of a step result.
func replacementsFromStepResults(step *v1.Step, stepParams v1.Params, defaults []v1.ParamSpec) (map[string]string, error) {
	stringReplacements := map[string]string{}
	for _, sp := range stepParams {
		if sp.Value.StringVal != "" && strings.HasPrefix(sp.Value.StringVal, "$(steps.") {
			// eg: when parameter p1 references a step result, replace:
			// $(params.p1) with $(steps.step1.results.foo)
			value := strings.TrimSuffix(strings.TrimPrefix(sp.Value.StringVal, "$("), ")")
			pr, err := resultref.ParseStepExpression(value)
			if err != nil {
				return nil, err
			}
			for _, d := range defaults {
				if d.Name == sp.Name {
					switch d.Type {
					case v1.ParamTypeObject:
						for k := range d.Properties {
							stringReplacements[fmt.Sprintf("params.%s.%s", d.Name, k)] = fmt.Sprintf("$(steps.%s.results.%s.%s)", pr.ResourceName, pr.ResultName, k)
						}
					case v1.ParamTypeArray:
						// for array parameters

						// with star notation, replace:
						// $(params.p1[*]) with $(steps.step1.results.foo[*])
						for _, pattern := range paramPatterns {
							stringReplacements[fmt.Sprintf(pattern+"[*]", d.Name)] = fmt.Sprintf("$(steps.%s.results.%s[*])", pr.ResourceName, pr.ResultName)
						}
						// with index notation, replace:
						// $(params.p1[idx]) with $(steps.step1.results.foo[idx])
						for k, v := range replacementsArrayIdxStepResults(step, d.Name, pr.ResourceName, pr.ResultName) {
							stringReplacements[k] = v
						}
					case v1.ParamTypeString:
						fallthrough
					default:
						// for string parameters and default case,
						// replace any reference to the parameter with the step result reference
						// since both use simple value substitution
						// eg: replace $(params.p1) with $(steps.step1.results.foo)
						for _, pattern := range paramPatterns {
							stringReplacements[fmt.Sprintf(pattern, d.Name)] = sp.Value.StringVal
						}
					}
				}
			}
		}
	}
	return stringReplacements, nil
}

// getTaskParameters gets the string, array and object parameter variable replacements needed in the Task
func getTaskParameters(spec *v1.TaskSpec, tr *v1.TaskRun, defaults ...v1.ParamSpec) (map[string]string, map[string][]string, map[string]map[string]string) {
	// This assumes that the TaskRun inputs have been validated against what the Task requests.
	// Set params from Task defaults
	stringReplacements, arrayReplacements, objectReplacements := replacementsFromDefaultParams(defaults)

	// Set and overwrite params with the ones from the TaskRun
	trStrings, trArrays, trObjects := replacementsFromParams(tr.Spec.Params)
	for k, v := range trStrings {
		stringReplacements[k] = v
	}
	for k, v := range trArrays {
		arrayReplacements[k] = v
	}
	for k, v := range trObjects {
		for key, val := range v {
			if objectReplacements != nil {
				if objectReplacements[k] != nil {
					objectReplacements[k][key] = val
				} else {
					objectReplacements[k] = v
				}
			}
		}
	}

	return stringReplacements, arrayReplacements, objectReplacements
}

// ApplyParameters applies the params from a TaskRun.Parameters to a TaskSpec
func ApplyParameters(spec *v1.TaskSpec, tr *v1.TaskRun, defaults ...v1.ParamSpec) *v1.TaskSpec {
	stringReplacements, arrayReplacements, objectReplacements := getTaskParameters(spec, tr, defaults...)
	return ApplyReplacements(spec, stringReplacements, arrayReplacements, objectReplacements)
}

func replacementsFromDefaultParams(defaults v1.ParamSpecs) (map[string]string, map[string][]string, map[string]map[string]string) {
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}

	// First pass: collect all non-reference default values
	for _, p := range defaults {
		if p.Default != nil && !strings.Contains(p.Default.StringVal, "$(params.") {
			switch p.Default.Type {
			case v1.ParamTypeArray:
				for _, pattern := range paramPatterns {
					for i := range len(p.Default.ArrayVal) {
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

	// Second pass: handle parameter references in default values
	for _, p := range defaults {
		if p.Default != nil && strings.Contains(p.Default.StringVal, "$(params.") {
			// extract referenced parameter name
			matches, _ := substitution.ExtractVariableExpressions(p.Default.StringVal, "params")
			for _, match := range matches {
				paramName := strings.TrimPrefix(match, "$(params.")
				paramName = strings.TrimSuffix(paramName, ")")

				// find referenced parameter value
				for _, pattern := range paramPatterns {
					key := fmt.Sprintf(pattern, paramName)
					if value, exists := stringReplacements[key]; exists {
						// Apply the value to this parameter's default
						resolvedValue := strings.ReplaceAll(p.Default.StringVal, match, value)
						for _, pattern := range paramPatterns {
							stringReplacements[fmt.Sprintf(pattern, p.Name)] = resolvedValue
						}
						break
					}
				}
			}
		}
	}

	return stringReplacements, arrayReplacements, objectReplacements
}

func replacementsFromParams(params v1.Params) (map[string]string, map[string][]string, map[string]map[string]string) {
	// stringReplacements is used for standard single-string stringReplacements, while arrayReplacements contains arrays
	// and objectReplacements contains objects that need to be further processed.
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}
	objectReplacements := map[string]map[string]string{}

	for _, p := range params {
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

func getContextReplacements(taskName string, tr *v1.TaskRun) map[string]string {
	return map[string]string{
		"context.taskRun.name":      tr.Name,
		"context.task.name":         taskName,
		"context.taskRun.namespace": tr.Namespace,
		"context.taskRun.uid":       string(tr.ObjectMeta.UID),
		"context.task.retry-count":  strconv.Itoa(len(tr.Status.RetriesStatus)),
	}
}

// ApplyContexts applies the substitution from $(context.(taskRun|task).*) with the specified values.
// Uses "" as a default if a value is not available.
func ApplyContexts(spec *v1.TaskSpec, taskName string, tr *v1.TaskRun) *v1.TaskSpec {
	return ApplyReplacements(spec, getContextReplacements(taskName, tr), map[string][]string{}, map[string]map[string]string{})
}

// ApplyWorkspaces applies the substitution from paths that the workspaces in declarations mounted to, the
// volumes that bindings are realized with in the task spec and the PersistentVolumeClaim names for the
// workspaces.
func ApplyWorkspaces(ctx context.Context, spec *v1.TaskSpec, declarations []v1.WorkspaceDeclaration, bindings []v1.WorkspaceBinding, vols map[string]corev1.Volume) *v1.TaskSpec {
	stringReplacements := map[string]string{}

	bindNames := sets.NewString()
	for _, binding := range bindings {
		bindNames.Insert(binding.Name)
	}

	for _, declaration := range declarations {
		prefix := fmt.Sprintf("workspaces.%s.", declaration.Name)
		if declaration.Optional && !bindNames.Has(declaration.Name) {
			stringReplacements[prefix+"bound"] = "false"
			stringReplacements[prefix+"path"] = ""
		} else {
			stringReplacements[prefix+"bound"] = "true"
			spec = applyWorkspaceMountPath(prefix+"path", spec, declaration)
		}
	}

	for name, vol := range vols {
		stringReplacements[fmt.Sprintf("workspaces.%s.volume", name)] = vol.Name
	}
	for _, binding := range bindings {
		if binding.PersistentVolumeClaim != nil {
			stringReplacements[fmt.Sprintf("workspaces.%s.claim", binding.Name)] = binding.PersistentVolumeClaim.ClaimName
		} else {
			stringReplacements[fmt.Sprintf("workspaces.%s.claim", binding.Name)] = ""
		}
	}
	return ApplyReplacements(spec, stringReplacements, map[string][]string{}, map[string]map[string]string{})
}

// ApplyParametersToWorkspaceBindings applies parameters to the WorkspaceBindings of a TaskRun. It takes a TaskSpec and a TaskRun as input and returns the modified TaskRun.
func ApplyParametersToWorkspaceBindings(ts *v1.TaskSpec, tr *v1.TaskRun) *v1.TaskRun {
	tsCopy := ts.DeepCopy()
	parameters, _, _ := getTaskParameters(tsCopy, tr, tsCopy.Params...)
	tr.Spec.Workspaces = workspace.ReplaceWorkspaceBindingsVars(tr.Spec.Workspaces, parameters)
	return tr
}

// applyWorkspaceMountPath accepts a workspace path variable of the form $(workspaces.foo.path) and replaces
// it in the fields of the TaskSpec. A new updated TaskSpec is returned. Steps or Sidecars in the TaskSpec
// that override the mountPath will receive that mountPath in place of the variable's value. Other Steps and
// Sidecars will see either the workspace's declared mountPath or the default of /workspaces/<name>.
func applyWorkspaceMountPath(variable string, spec *v1.TaskSpec, declaration v1.WorkspaceDeclaration) *v1.TaskSpec {
	stringReplacements := map[string]string{variable: ""}
	emptyArrayReplacements := map[string][]string{}
	defaultMountPath := declaration.GetMountPath()
	// Replace instances of the workspace path variable that are overridden per-Step
	for i := range spec.Steps {
		step := &spec.Steps[i]
		for _, usage := range step.Workspaces {
			if usage.Name == declaration.Name && usage.MountPath != "" {
				stringReplacements[variable] = usage.MountPath
				container.ApplyStepReplacements(step, stringReplacements, emptyArrayReplacements)
			}
		}
	}
	// Replace instances of the workspace path variable that are overridden per-Sidecar
	for i := range spec.Sidecars {
		sidecar := &spec.Sidecars[i]
		for _, usage := range sidecar.Workspaces {
			if usage.Name == declaration.Name && usage.MountPath != "" {
				stringReplacements[variable] = usage.MountPath
				container.ApplySidecarReplacements(sidecar, stringReplacements, emptyArrayReplacements)
			}
		}
	}
	// Replace any remaining instances of the workspace path variable, which should fall
	// back to the mount path specified in the declaration.
	stringReplacements[variable] = defaultMountPath
	return ApplyReplacements(spec, stringReplacements, emptyArrayReplacements, map[string]map[string]string{})
}

// ApplyResults applies the substitution from values in results and step results which are referenced in spec as subitems
// of the replacementStr.
func ApplyResults(spec *v1.TaskSpec) *v1.TaskSpec {
	// Apply all the Step Result replacements
	for i := range spec.Steps {
		stringReplacements := getStepResultReplacements(spec.Steps[i], i)
		container.ApplyStepReplacements(&spec.Steps[i], stringReplacements, map[string][]string{})
	}
	stringReplacements := getTaskResultReplacements(spec)
	return ApplyReplacements(spec, stringReplacements, map[string][]string{}, map[string]map[string]string{})
}

// getStepResultReplacements creates all combinations of string replacements from Step Results.
func getStepResultReplacements(step v1.Step, idx int) map[string]string {
	stringReplacements := map[string]string{}

	patterns := []string{
		"step.results.%s.path",
		"step.results[%q].path",
		"step.results['%s'].path",
	}
	stepName := pod.StepName(step.Name, idx)
	for _, result := range step.Results {
		for _, pattern := range patterns {
			stringReplacements[fmt.Sprintf(pattern, result.Name)] = filepath.Join(pipeline.StepsDir, stepName, "results", result.Name)
		}
	}
	return stringReplacements
}

// getTaskResultReplacements creates all combinations of string replacements from TaskResults.
func getTaskResultReplacements(spec *v1.TaskSpec) map[string]string {
	stringReplacements := map[string]string{}

	patterns := []string{
		"results.%s.path",
		"results[%q].path",
		"results['%s'].path",
	}

	for _, result := range spec.Results {
		for _, pattern := range patterns {
			stringReplacements[fmt.Sprintf(pattern, result.Name)] = filepath.Join(pipeline.DefaultResultPath, result.Name)
		}
	}
	return stringReplacements
}

// ApplyArtifacts replaces the occurrences of artifacts.path and step.artifacts.path with the absolute tekton internal path
func ApplyArtifacts(spec *v1.TaskSpec) *v1.TaskSpec {
	for i := range spec.Steps {
		stringReplacements := getArtifactReplacements(spec.Steps[i], i)
		container.ApplyStepReplacements(&spec.Steps[i], stringReplacements, map[string][]string{})
	}
	return spec
}

func getArtifactReplacements(step v1.Step, idx int) map[string]string {
	stringReplacements := map[string]string{}
	stepName := pod.StepName(step.Name, idx)
	stringReplacements[artifactref.StepArtifactPathPattern] = filepath.Join(pipeline.StepsDir, stepName, "artifacts", "provenance.json")
	stringReplacements[artifactref.TaskArtifactPathPattern] = filepath.Join(pipeline.ArtifactsDir, "provenance.json")

	return stringReplacements
}

// ApplyStepExitCodePath replaces the occurrences of exitCode path with the absolute tekton internal path
// Replace $(steps.<step-name>.exitCode.path) with pipeline.StepPath/<step-name>/exitCode
func ApplyStepExitCodePath(spec *v1.TaskSpec) *v1.TaskSpec {
	stringReplacements := map[string]string{}

	for i, step := range spec.Steps {
		stringReplacements[fmt.Sprintf("steps.%s.exitCode.path", pod.StepName(step.Name, i))] = filepath.Join(pipeline.StepsDir, pod.StepName(step.Name, i), "exitCode")
	}
	return ApplyReplacements(spec, stringReplacements, map[string][]string{}, map[string]map[string]string{})
}

// ApplyCredentialsPath applies a substitution of the key $(credentials.path) with the path that credentials
// from annotated secrets are written to.
func ApplyCredentialsPath(spec *v1.TaskSpec, path string) *v1.TaskSpec {
	stringReplacements := map[string]string{
		"credentials.path": path,
	}
	return ApplyReplacements(spec, stringReplacements, map[string][]string{}, map[string]map[string]string{})
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(spec *v1.TaskSpec, stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string) *v1.TaskSpec {
	spec = spec.DeepCopy()

	// Apply variable expansion to steps fields.
	steps := spec.Steps
	for i := range steps {
		if steps[i].Params != nil {
			steps[i].Params = steps[i].Params.ReplaceVariables(stringReplacements, arrayReplacements, objectReplacements)
		}
		container.ApplyStepReplacements(&steps[i], stringReplacements, arrayReplacements)
	}

	// Apply variable expansion to stepTemplate fields.
	if spec.StepTemplate != nil {
		container.ApplyStepTemplateReplacements(spec.StepTemplate, stringReplacements, arrayReplacements)
	}

	// Apply variable expansion to the build's volumes
	for i, v := range spec.Volumes {
		spec.Volumes[i].Name = substitution.ApplyReplacements(v.Name, stringReplacements)
		if v.VolumeSource.ConfigMap != nil {
			spec.Volumes[i].ConfigMap.Name = substitution.ApplyReplacements(v.ConfigMap.Name, stringReplacements)
			for index, item := range v.ConfigMap.Items {
				spec.Volumes[i].ConfigMap.Items[index].Key = substitution.ApplyReplacements(item.Key, stringReplacements)
				spec.Volumes[i].ConfigMap.Items[index].Path = substitution.ApplyReplacements(item.Path, stringReplacements)
			}
		}
		if v.VolumeSource.Secret != nil {
			spec.Volumes[i].Secret.SecretName = substitution.ApplyReplacements(v.Secret.SecretName, stringReplacements)
			for index, item := range v.Secret.Items {
				spec.Volumes[i].Secret.Items[index].Key = substitution.ApplyReplacements(item.Key, stringReplacements)
				spec.Volumes[i].Secret.Items[index].Path = substitution.ApplyReplacements(item.Path, stringReplacements)
			}
		}
		if v.PersistentVolumeClaim != nil {
			spec.Volumes[i].PersistentVolumeClaim.ClaimName = substitution.ApplyReplacements(v.PersistentVolumeClaim.ClaimName, stringReplacements)
		}
		if v.Projected != nil {
			for _, s := range spec.Volumes[i].Projected.Sources {
				if s.ConfigMap != nil {
					s.ConfigMap.Name = substitution.ApplyReplacements(s.ConfigMap.Name, stringReplacements)
				}
				if s.Secret != nil {
					s.Secret.Name = substitution.ApplyReplacements(s.Secret.Name, stringReplacements)
				}
				if s.ServiceAccountToken != nil {
					s.ServiceAccountToken.Audience = substitution.ApplyReplacements(s.ServiceAccountToken.Audience, stringReplacements)
				}
			}
		}
		if v.CSI != nil {
			if v.CSI.NodePublishSecretRef != nil {
				spec.Volumes[i].CSI.NodePublishSecretRef.Name = substitution.ApplyReplacements(v.CSI.NodePublishSecretRef.Name, stringReplacements)
			}
			if v.CSI.VolumeAttributes != nil {
				for key, value := range v.CSI.VolumeAttributes {
					spec.Volumes[i].CSI.VolumeAttributes[key] = substitution.ApplyReplacements(value, stringReplacements)
				}
			}
		}
	}

	for i, v := range spec.Workspaces {
		spec.Workspaces[i].MountPath = substitution.ApplyReplacements(v.MountPath, stringReplacements)
	}

	// Apply variable substitution to the sidecar definitions
	sidecars := spec.Sidecars
	for i := range sidecars {
		container.ApplySidecarReplacements(&sidecars[i], stringReplacements, arrayReplacements)
	}

	return spec
}
