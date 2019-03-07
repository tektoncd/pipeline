/*
 Copyright 2019 Knative Authors LLC
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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/templating"
)

// ApplyParameters applies the params from a PipelineRun.Params to a PipelineSpec.
func ApplyParameters(p *v1alpha1.Pipeline, pr *v1alpha1.PipelineRun) *v1alpha1.Pipeline {
	// This assumes that the PipelineRun inputs have been validated against what the Pipeline requests.
	replacements := map[string]string{}
	// Set all the default replacements
	for _, p := range p.Spec.Params {
		if p.Default != "" {
			replacements[fmt.Sprintf("params.%s", p.Name)] = p.Default
		}
	}
	// Set and overwrite params with the ones from the PipelineRun
	for _, p := range pr.Spec.Params {
		replacements[fmt.Sprintf("params.%s", p.Name)] = p.Value
	}

	return ApplyReplacements(p, replacements)
}

// ApplyReplacements replaces placeholders for declared parameters with the specified replacements.
func ApplyReplacements(p *v1alpha1.Pipeline, replacements map[string]string) *v1alpha1.Pipeline {
	p = p.DeepCopy()

	tasks := p.Spec.Tasks

	for i := range tasks {
		params := tasks[i].Params

		for j := range params {
			params[j].Value = templating.ApplyReplacements(params[j].Value, replacements)
		}

		tasks[i].Params = params
	}

	return p
}
