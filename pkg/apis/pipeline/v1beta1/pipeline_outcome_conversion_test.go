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

package v1beta1_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineOutcomeConversionRoundTrip(t *testing.T) {
	for _, outcome := range []*v1.PipelineOutcomeSpec{nil, {Result: "verdict"}} {
		name := "omitted"
		if outcome != nil {
			name = "selected"
		}
		t.Run(name, func(t *testing.T) {
			source := &v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "pipeline", Annotations: map[string]string{"example.dev/keep": "value"}},
				Spec:       v1.PipelineSpec{Outcome: outcome},
			}
			before := source.DeepCopy()
			beta := &v1beta1.Pipeline{}
			if err := beta.ConvertFrom(t.Context(), source); err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(before, source); d != "" {
				t.Fatalf("ConvertFrom() mutated source (-want +got): %s", d)
			}
			wire, err := json.Marshal(beta)
			if err != nil {
				t.Fatal(err)
			}
			decoded := &v1beta1.Pipeline{}
			if err := json.Unmarshal(wire, decoded); err != nil {
				t.Fatal(err)
			}
			betaBefore := decoded.DeepCopy()
			got := &v1.Pipeline{}
			if err := decoded.ConvertTo(t.Context(), got); err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(source, got); d != "" {
				t.Errorf("round trip (-want +got): %s", d)
			}
			if d := cmp.Diff(betaBefore, decoded); d != "" {
				t.Errorf("ConvertTo() mutated source (-want +got): %s", d)
			}
		})
	}
}

func TestPipelineRunOutcomeConversionRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name     string
		spec     *v1.PipelineOutcomeSpec
		snapshot *v1.PipelineOutcomeSpec
		outcome  *v1.PipelineRunOutcome
	}{{
		name: "omitted",
	}, {
		name: "embedded selector only",
		spec: &v1.PipelineOutcomeSpec{Result: "selected"},
	}, {
		name:     "status snapshot selector only",
		snapshot: &v1.PipelineOutcomeSpec{Result: "resolved"},
	}, {
		name:    "status outcome only",
		outcome: &v1.PipelineRunOutcome{Reason: "Accepted", Message: "checks passed"},
	}, {
		name:     "independent selectors and status",
		spec:     &v1.PipelineOutcomeSpec{Result: "selected"},
		snapshot: &v1.PipelineOutcomeSpec{Result: "resolved"},
		outcome:  &v1.PipelineRunOutcome{Reason: "Rejected", Message: "policy failed"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			source := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "run", Annotations: map[string]string{"example.dev/keep": "value"}},
				Spec: v1.PipelineRunSpec{PipelineSpec: &v1.PipelineSpec{
					Description: "embedded", Outcome: tc.spec,
				}},
				Status: v1.PipelineRunStatus{PipelineRunStatusFields: v1.PipelineRunStatusFields{
					PipelineSpec: &v1.PipelineSpec{Description: "resolved", Outcome: tc.snapshot},
					Outcome:      tc.outcome,
				}},
			}
			before := source.DeepCopy()
			beta := &v1beta1.PipelineRun{}
			if err := beta.ConvertFrom(t.Context(), source); err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(before, source); d != "" {
				t.Fatalf("ConvertFrom() mutated source (-want +got): %s", d)
			}
			wire, err := json.Marshal(beta)
			if err != nil {
				t.Fatal(err)
			}
			decoded := &v1beta1.PipelineRun{}
			if err := json.Unmarshal(wire, decoded); err != nil {
				t.Fatal(err)
			}
			betaBefore := decoded.DeepCopy()
			got := &v1.PipelineRun{}
			if err := decoded.ConvertTo(t.Context(), got); err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(source, got); d != "" {
				t.Errorf("round trip (-want +got): %s", d)
			}
			if d := cmp.Diff(betaBefore, decoded); d != "" {
				t.Errorf("ConvertTo() mutated source (-want +got): %s", d)
			}
		})
	}
}

func TestPipelineOutcomeConversionMalformedAnnotations(t *testing.T) {
	t.Run("standalone selector", func(t *testing.T) {
		source := &v1beta1.Pipeline{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"tekton.dev/v1.pipeline-outcome": "{"},
		}}
		if err := source.ConvertTo(t.Context(), &v1.Pipeline{}); err == nil {
			t.Fatal("ConvertTo() accepted a malformed outcome annotation")
		}
	})
	for _, key := range []string{
		"tekton.dev/v1.pipelinerun-spec-outcome",
		"tekton.dev/v1.pipelinerun-status-spec-outcome",
		"tekton.dev/v1.pipelinerun-outcome",
	} {
		t.Run(key, func(t *testing.T) {
			source := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{key: "{"}},
				Spec:       v1beta1.PipelineRunSpec{PipelineSpec: &v1beta1.PipelineSpec{}},
				Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					PipelineSpec: &v1beta1.PipelineSpec{},
				}},
			}
			if err := source.ConvertTo(t.Context(), &v1.PipelineRun{}); err == nil {
				t.Fatal("ConvertTo() accepted a malformed outcome annotation")
			}
		})
	}
}
