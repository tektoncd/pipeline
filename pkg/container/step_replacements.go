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

package container

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/substitution"
)

// ApplyStepReplacements applies variable interpolation on a Step.
func ApplyStepReplacements(step *v1.Step, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	step.Script = substitution.ApplyReplacements(step.Script, stringReplacements)
	step.OnError = (v1.OnErrorType)(substitution.ApplyReplacements(string(step.OnError), stringReplacements))
	if step.StdoutConfig != nil {
		step.StdoutConfig.Path = substitution.ApplyReplacements(step.StdoutConfig.Path, stringReplacements)
	}
	if step.StderrConfig != nil {
		step.StderrConfig.Path = substitution.ApplyReplacements(step.StderrConfig.Path, stringReplacements)
	}
	applyStepReplacements(step, stringReplacements, arrayReplacements)
}

// ApplyStepTemplateReplacements applies variable interpolation on a StepTemplate (aka a container)
func ApplyStepTemplateReplacements(stepTemplate *v1.StepTemplate, stringReplacements map[string]string, arrayReplacements map[string][]string) {
	container := stepTemplate.ToK8sContainer()
	applyContainerReplacements(container, stringReplacements, arrayReplacements)
	stepTemplate.SetContainerFields(*container)
}
