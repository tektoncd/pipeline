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

package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"knative.dev/pkg/apis"
)

// PipelineRunOutcome is the author-defined reason and message for a PipelineRun.
type PipelineRunOutcome struct {
	// Reason is the machine-readable outcome reason: Succeeded or Neutral.
	Reason string `json:"reason"`
	// Message is the human-readable outcome message, limited to 4096 UTF-8 bytes.
	// +optional
	Message string `json:"message,omitempty"`
}

func (ps *PipelineSpec) validateOutcome(ctx context.Context) *apis.FieldError {
	if ps.Outcome == nil {
		return nil
	}
	if !config.FromContextOrDefaults(ctx).FeatureFlags.EnablePipelineRunOutcomes {
		return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use PipelineRun outcomes", config.EnablePipelineRunOutcomes), "outcome")
	}
	if ps.Outcome.Result == "" {
		return apis.ErrMissingField("outcome.result")
	}
	if strings.Contains(ps.Outcome.Result, "$(") {
		return apis.ErrInvalidValue(ps.Outcome.Result, "outcome.result", "must be a literal result name")
	}
	for _, result := range ps.Results {
		if result.Name == ps.Outcome.Result {
			if result.Type != ResultsTypeObject {
				return apis.ErrInvalidValue(ps.Outcome.Result, "outcome.result", "must select an object-valued Pipeline result")
			}
			return nil
		}
	}
	return apis.ErrInvalidValue(ps.Outcome.Result, "outcome.result", "must select a declared Pipeline result")
}
