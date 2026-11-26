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
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineSpecValidateOutcome(t *testing.T) {
	for _, tc := range []struct {
		name       string
		outcome    *PipelineOutcomeSpec
		resultType ResultsType
		enabled    bool
		apiFields  string
		wantPath   string
	}{{
		name: "omitted with flag disabled",
	}, {
		name:    "object result",
		outcome: &PipelineOutcomeSpec{Result: "verdict"}, resultType: ResultsTypeObject,
		enabled: true, apiFields: config.StableAPIFields,
	}, {
		name:    "object result with flag disabled",
		outcome: &PipelineOutcomeSpec{Result: "verdict"}, resultType: ResultsTypeObject,
		apiFields: config.AlphaAPIFields, wantPath: "outcome",
	}, {
		name:    "empty selector",
		outcome: &PipelineOutcomeSpec{}, enabled: true, wantPath: "outcome.result",
	}, {
		name:    "undeclared result",
		outcome: &PipelineOutcomeSpec{Result: "missing"}, enabled: true, wantPath: "outcome.result",
	}, {
		name:    "string result",
		outcome: &PipelineOutcomeSpec{Result: "verdict"}, resultType: ResultsTypeString,
		enabled: true, wantPath: "outcome.result",
	}, {
		name:    "implicit string result",
		outcome: &PipelineOutcomeSpec{Result: "verdict"},
		enabled: true, wantPath: "outcome.result",
	}, {
		name:    "array result",
		outcome: &PipelineOutcomeSpec{Result: "verdict"}, resultType: ResultsTypeArray,
		enabled: true, wantPath: "outcome.result",
	}, {
		name:    "parameter expression matching a declaration",
		outcome: &PipelineOutcomeSpec{Result: "$(params.selector)"}, resultType: ResultsTypeObject,
		enabled: true, wantPath: "outcome.result",
	}, {
		name:    "result expression matching a declaration",
		outcome: &PipelineOutcomeSpec{Result: "$(tasks.produce.results.verdict)"}, resultType: ResultsTypeObject,
		enabled: true, wantPath: "outcome.result",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			flags := *config.DefaultFeatureFlags
			flags.EnablePipelineRunOutcomes = tc.enabled
			if tc.apiFields != "" {
				flags.EnableAPIFields = tc.apiFields
			}
			ctx := config.ToContext(t.Context(), &config.Config{FeatureFlags: &flags})
			resultName := "verdict"
			if tc.outcome != nil && strings.Contains(tc.outcome.Result, "$(") {
				resultName = tc.outcome.Result
			}
			spec := PipelineSpec{
				Outcome: tc.outcome,
				Tasks:   []PipelineTask{{Name: "produce", TaskRef: &TaskRef{Name: "produce"}}},
				Results: []PipelineResult{{
					Name: resultName, Type: tc.resultType,
					Value: ResultValue{Type: ParamTypeObject, ObjectVal: map[string]string{
						"reason": "$(tasks.produce.results.reason)",
					}},
				}},
			}
			for _, surface := range []struct {
				name     string
				validate func(context.Context) *apis.FieldError
				prefix   string
			}{{
				name: "resolved spec", validate: spec.Validate,
			}, {
				name: "pipeline admission",
				validate: (&Pipeline{
					ObjectMeta: metav1.ObjectMeta{Name: "pipeline"}, Spec: spec,
				}).Validate,
				prefix: "spec.",
			}, {
				name: "embedded spec admission",
				validate: (&PipelineRun{
					ObjectMeta: metav1.ObjectMeta{Name: "run"},
					Spec:       PipelineRunSpec{PipelineSpec: &spec},
				}).Validate,
				prefix: "spec.pipelineSpec.",
			}} {
				t.Run(surface.name, func(t *testing.T) {
					err := surface.validate(ctx)
					if tc.wantPath == "" {
						if err != nil {
							t.Fatalf("Validate() = %v, want nil", err)
						}
					} else if err == nil || !strings.Contains(err.Error(), surface.prefix+tc.wantPath) {
						t.Fatalf("Validate() = %v, want error at %s%s", err, surface.prefix, tc.wantPath)
					}
				})
			}
		})
	}
}

func TestPipelineOutcomeDeepCopy(t *testing.T) {
	run := &PipelineRun{
		Spec: PipelineRunSpec{PipelineSpec: &PipelineSpec{Outcome: &PipelineOutcomeSpec{Result: "verdict"}}},
		Status: PipelineRunStatus{PipelineRunStatusFields: PipelineRunStatusFields{
			PipelineSpec: &PipelineSpec{Outcome: &PipelineOutcomeSpec{Result: "snapshot"}},
			Outcome:      &PipelineRunOutcome{Reason: "Accepted", Message: "checks passed"},
		}},
	}
	copied := run.DeepCopy()
	copied.Spec.PipelineSpec.Outcome.Result = "changed"
	copied.Status.PipelineSpec.Outcome.Result = "changed"
	copied.Status.Outcome.Reason = "changed"
	if run.Spec.PipelineSpec.Outcome.Result != "verdict" ||
		run.Status.PipelineSpec.Outcome.Result != "snapshot" ||
		run.Status.Outcome.Reason != "Accepted" {
		t.Fatal("DeepCopy() shares outcome fields with its source")
	}
}
