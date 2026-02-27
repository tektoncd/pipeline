/*
 Copyright 2022 The Tekton Authors

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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/run/v1alpha1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/run/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	now       = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	testClock = clock.NewFakePassiveClock(now)
)

func TestFromRunStatus(t *testing.T) {
	startTime := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(1 * time.Hour)

	runStatus := v1alpha1.RunStatus{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}},
		},
		RunStatusFields: v1alpha1.RunStatusFields{
			StartTime:      &metav1.Time{Time: startTime},
			CompletionTime: &metav1.Time{Time: endTime},
			Results: []v1alpha1.RunResult{{
				Name:  "foo",
				Value: "bar",
			}},
			RetriesStatus: []v1alpha1.RunStatus{{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
				RunStatusFields: v1alpha1.RunStatusFields{
					StartTime:      &metav1.Time{Time: startTime.Add(-30 * time.Minute)},
					CompletionTime: &metav1.Time{Time: startTime.Add(-15 * time.Minute)},
					Results: []v1alpha1.RunResult{{
						Name:  "foo",
						Value: "bad",
					}},
					ExtraFields: runtime.RawExtension{
						Raw: []byte(`{"complex":{"goodbye":["w","o","r","l","d"]},"simple":"goodbye"}`),
					},
				},
			}},
			ExtraFields: runtime.RawExtension{
				Raw: []byte(`{"complex":{"hello":["w","o","r","l","d"]},"simple":"hello"}`),
			},
		},
	}

	expectedCustomRunResult := v1beta1.CustomRunStatus{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}},
		},
		CustomRunStatusFields: v1beta1.CustomRunStatusFields{
			StartTime:      &metav1.Time{Time: startTime},
			CompletionTime: &metav1.Time{Time: endTime},
			Results: []v1beta1.CustomRunResult{{
				Name:  "foo",
				Value: "bar",
			}},
			RetriesStatus: []v1beta1.CustomRunStatus{{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					StartTime:      &metav1.Time{Time: startTime.Add(-30 * time.Minute)},
					CompletionTime: &metav1.Time{Time: startTime.Add(-15 * time.Minute)},
					Results: []v1beta1.CustomRunResult{{
						Name:  "foo",
						Value: "bad",
					}},
					ExtraFields: runtime.RawExtension{
						Raw: []byte(`{"complex":{"goodbye":["w","o","r","l","d"]},"simple":"goodbye"}`),
					},
				},
			}},
			ExtraFields: runtime.RawExtension{
				Raw: []byte(`{"complex":{"hello":["w","o","r","l","d"]},"simple":"hello"}`),
			},
		},
	}

	if d := cmp.Diff(expectedCustomRunResult, v1beta1.FromRunStatus(runStatus)); d != "" {
		t.Errorf("expected converted RunStatus to equal expected CustomRunStatus. Diff %s", diff.PrintWantGot(d))
	}
}

func TestInitializeCustomRunConditions(t *testing.T) {
	runStatus := &v1beta1.CustomRunStatus{}
	runStatus.InitializeConditions(testClock)

	if runStatus.StartTime.IsZero() || !runStatus.StartTime.Time.Equal(now) {
		t.Fatalf("CustomRun StartTime not initialized correctly")
	}

	condition := runStatus.GetCondition(apis.ConditionSucceeded)
	if condition.Reason != "Started" {
		t.Fatalf("CustomRun initialize reason should be Started, got %s instead", condition.Reason)
	}

	// Change the reason before we initialize again
	runStatus.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  "not just started",
		Message: "hello",
	})
	runStatus.InitializeConditions(testClock)

	newCondition := runStatus.GetCondition(apis.ConditionSucceeded)
	if newCondition.Reason != "not just started" {
		t.Fatalf("CustomRun initialize reset the condition reason to %s", newCondition.Reason)
	}
}
