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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// ApplyParameters applies the params from a PipelineRun.Params to a PipelineSpec.
func ApplyParameters(p *v1beta1.PipelineSpec, pr *v1beta1.PipelineRun) *v1beta1.PipelineSpec {
	// This assumes that the PipelineRun inputs have been validated against what the Pipeline requests.

	// stringReplacements is used for standard single-string stringReplacements, while arrayReplacements contains arrays
	// that need to be further processed.
	stringReplacements := map[string]string{}
	arrayReplacements := map[string][]string{}

	// Set all the default stringReplacements
	for _, p := range p.Params {
		if p.Default != nil {
			if p.Default.Type == v1beta1.ParamTypeString {
				stringReplacements[fmt.Sprintf("params.%s", p.Name)] = p.Default.StringVal
			} else {
				arrayReplacements[fmt.Sprintf("params.%s", p.Name)] = p.Default.ArrayVal
			}
		}
	}
	// Set and overwrite params with the ones from the PipelineRun
	for _, p := range pr.Spec.Params {
		if p.Value.Type == v1beta1.ParamTypeString {
			stringReplacements[fmt.Sprintf("params.%s", p.Name)] = p.Value.StringVal
		} else {
			arrayReplacements[fmt.Sprintf("params.%s", p.Name)] = p.Value.ArrayVal
		}
	}

	return ApplyReplacements(p, stringReplacements, arrayReplacements)
}

// ApplyContexts applies the substitution from $(context.(pipelineRun|pipeline).*) with the specified values.
// Currently supports only name substitution. Uses "" as a default if name is not specified.
func ApplyContexts(spec *v1beta1.PipelineSpec, pipelineName string, pr *v1beta1.PipelineRun) *v1beta1.PipelineSpec {
	replacements := map[string]string{
		"context.pipelineRun.name":      pr.Name,
		"context.pipeline.name":         pipelineName,
		"context.pipelineRun.namespace": pr.Namespace,
		"context.pipelineRun.uid":       string(pr.ObjectMeta.UID),
	}
	return ApplyReplacements(spec, replacements, map[string][]string{})
}

// ApplyTaskResults applies the ResolvedResultRef to each PipelineTask.Params in targets
func ApplyTaskResults(targets PipelineRunState, resolvedResultRefs ResolvedResultRefs) {
	stringReplacements := map[string]string{}

	for _, resolvedResultRef := range resolvedResultRefs {
		replaceTarget := fmt.Sprintf("%s.%s.%s.%s", v1beta1.ResultTaskPart, resolvedResultRef.ResultReference.PipelineTask, v1beta1.ResultResultPart, resolvedResultRef.ResultReference.Result)
		stringReplacements[replaceTarget] = resolvedResultRef.Value.StringVal
	}

	for _, resolvedPipelineRunTask := range targets {
		// also make substitution for resolved condition checks
		for _, resolvedConditionCheck := range resolvedPipelineRunTask.ResolvedConditionChecks {
			pipelineTaskCondition := resolvedConditionCheck.PipelineTaskCondition.DeepCopy()
			pipelineTaskCondition.Params = replaceParamValues(pipelineTaskCondition.Params, stringReplacements, nil)
			resolvedConditionCheck.PipelineTaskCondition = pipelineTaskCondition
		}
		if resolvedPipelineRunTask.PipelineTask != nil {
			pipelineTask := resolvedPipelineRunTask.PipelineTask.DeepCopy()
			pipelineTask.Params = replaceParamValues(pipelineTask.Params, stringReplacements, nil)
			resolvedPipelineRunTask.PipelineTask = pipelineTask
		}
	}
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(p *v1beta1.PipelineSpec, replacements map[string]string, arrayReplacements map[string][]string) *v1beta1.PipelineSpec {
	p = p.DeepCopy()

	// replace param values for both DAG and final tasks
	tasks := append(p.Tasks, p.Finally...)

	for i := range tasks {
		tasks[i].Params = replaceParamValues(tasks[i].Params, replacements, arrayReplacements)
		for j := range tasks[i].Conditions {
			c := tasks[i].Conditions[j]
			c.Params = replaceParamValues(c.Params, replacements, arrayReplacements)
		}
	}

	return p
}

func replaceParamValues(params []v1beta1.Param, stringReplacements map[string]string, arrayReplacements map[string][]string) []v1beta1.Param {
	for i := range params {
		params[i].Value.ApplyReplacements(stringReplacements, arrayReplacements)
	}
	return params
}
