/*
Copyright 2026 The Tekton Authors

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

package pipelinerun

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// setPipelineRunOutcome runs after result collection, before the terminal status
// is persisted. The resolved PipelineSpec has already passed admission validation.
func setPipelineRunOutcome(pr *v1.PipelineRun, spec *v1.PipelineSpec) {
	pr.Status.Outcome = nil
	if spec.Outcome == nil || !pr.IsSuccessful() {
		return
	}

	for _, result := range pr.Status.Results {
		if result.Name != spec.Outcome.Result {
			continue
		}
		value := result.Value
		if value.Type != v1.ParamTypeObject {
			pr.Status.MarkFailed(v1.PipelineRunReasonInvalidPipelineOutcome.String(),
				"Pipeline outcome result %q must be an object", result.Name)
			return
		}
		for key := range value.ObjectVal {
			if key != "reason" && key != "message" {
				pr.Status.MarkFailed(v1.PipelineRunReasonInvalidPipelineOutcome.String(),
					"Pipeline outcome result %q has unknown key %q", result.Name, key)
				return
			}
		}
		reason, message := value.ObjectVal["reason"], value.ObjectVal["message"]
		if reason != "Succeeded" && reason != "Neutral" {
			pr.Status.MarkFailed(v1.PipelineRunReasonInvalidPipelineOutcome.String(),
				"Pipeline outcome result %q must have reason Succeeded or Neutral", result.Name)
			return
		}
		if len(message) > 4096 {
			pr.Status.MarkFailed(v1.PipelineRunReasonInvalidPipelineOutcome.String(),
				"Pipeline outcome result %q message exceeds 4096 UTF-8 bytes", result.Name)
			return
		}
		pr.Status.Outcome = &v1.PipelineRunOutcome{Reason: reason, Message: message}
		return
	}
	pr.Status.MarkFailed(v1.PipelineRunReasonInvalidPipelineOutcome.String(),
		"Pipeline outcome result %q is missing", spec.Outcome.Result)
}
