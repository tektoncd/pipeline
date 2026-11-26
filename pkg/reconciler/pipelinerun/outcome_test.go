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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineRunOutcome(t *testing.T) {
	for _, tc := range []struct {
		name       string
		report     map[string]string
		execution  corev1.ConditionStatus
		reason     string
		noSelector bool
		stringType bool
		want       *v1.PipelineRunOutcome
		invalid    bool
	}{
		{name: "neutral", report: map[string]string{"reason": "Neutral", "message": "review findings"}, want: &v1.PipelineRunOutcome{Reason: "Neutral", Message: "review findings"}},
		{name: "success", report: map[string]string{"reason": "Succeeded"}, want: &v1.PipelineRunOutcome{Reason: "Succeeded"}},
		{name: "maximum UTF-8 bytes", report: map[string]string{"reason": "Neutral", "message": strings.Repeat("\u00e9", 2048)}, want: &v1.PipelineRunOutcome{Reason: "Neutral", Message: strings.Repeat("\u00e9", 2048)}},
		{name: "oversized UTF-8 message", report: map[string]string{"reason": "Neutral", "message": strings.Repeat("\u00e9", 2049)}, invalid: true},
		{name: "missing selected result", invalid: true},
		{name: "missing reason", report: map[string]string{"message": "finding"}, invalid: true},
		{name: "unknown reason", report: map[string]string{"reason": "Failed"}, invalid: true},
		{name: "unknown key", report: map[string]string{"reason": "Neutral", "extra": "value"}, invalid: true},
		{name: "string report", report: map[string]string{"reason": "Neutral"}, stringType: true, invalid: true},
		{name: "no selector", report: map[string]string{"reason": "Failed"}, noSelector: true},
		{name: "running", execution: corev1.ConditionUnknown, reason: "Running", report: map[string]string{"reason": "Neutral"}},
		{name: "execution failure", execution: corev1.ConditionFalse, reason: "Failed", report: map[string]string{"reason": "Neutral"}},
		{name: "cancelled", execution: corev1.ConditionFalse, reason: "Cancelled", report: map[string]string{"reason": "Succeeded"}},
		{name: "timeout", execution: corev1.ConditionFalse, reason: "PipelineRunTimeout", report: map[string]string{"reason": "Neutral"}},
		{name: "result processing failure", execution: corev1.ConditionFalse, reason: "CouldntGetPipelineResult", report: map[string]string{"reason": "Neutral"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1.PipelineRun{}
			pr.Status.InitializeConditions(testClock)
			status, reason := tc.execution, tc.reason
			if status == "" {
				status, reason = corev1.ConditionTrue, "Completed"
			}
			pr.Status.SetCondition(&apis.Condition{Type: apis.ConditionSucceeded, Status: status, Reason: reason, Message: "execution state"})
			before := *pr.Status.GetCondition(apis.ConditionSucceeded)
			spec := &v1.PipelineSpec{}
			if !tc.noSelector {
				spec.Outcome = &v1.PipelineOutcomeSpec{Result: "report"}
			}
			if tc.report != nil {
				value := v1.ResultValue{Type: v1.ParamTypeObject, ObjectVal: tc.report}
				if tc.stringType {
					value = v1.ResultValue{Type: v1.ParamTypeString, StringVal: "Neutral"}
				}
				pr.Status.Results = []v1.PipelineRunResult{{Name: "unrelated", Value: v1.ResultValue{Type: v1.ParamTypeString}}, {Name: "report", Value: value}}
			}

			setPipelineRunOutcome(pr, spec)

			if diff := cmp.Diff(tc.want, pr.Status.Outcome); diff != "" {
				t.Errorf("outcome (-want +got): %s", diff)
			}
			condition := pr.Status.GetCondition(apis.ConditionSucceeded)
			if tc.invalid {
				if !condition.IsFalse() || condition.Reason != "InvalidPipelineOutcome" {
					t.Errorf("invalid report condition = %+v", condition)
				}
			} else if diff := cmp.Diff(before, *condition); diff != "" {
				t.Errorf("execution condition changed (-want +got): %s", diff)
			}
		})
	}
}
