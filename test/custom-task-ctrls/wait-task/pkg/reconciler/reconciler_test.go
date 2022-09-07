/*
Copyright 2021 The Tekton Authors

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

package reconciler

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/test/helpers"
)

const (
	apiVersion string = "wait.testing.tekton.dev/v1alpha1"
	kind       string = "Wait"
)

var (
	filterTypeMeta          = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	filterObjectMeta        = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields")
	filterCondition         = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	filterRunStatus         = cmpopts.IgnoreFields(v1alpha1.RunStatusFields{}, "StartTime", "CompletionTime")
	filterPipelineRunStatus = cmpopts.IgnoreFields(v1beta1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")

	now       = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	testClock = clock.NewFakePassiveClock(now)
)

func TestReconcile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                   string
		refName                string
		timeout                string
		params                 string
		wantRunConditionType   apis.ConditionType
		wantRunConditionStatus corev1.ConditionStatus
		wantRunConditionReason string
		isParentPRCancelled    bool
	}{{
		name: "duration elapsed",
		params: `
  params:
  - name: duration
    value: 1s
`,
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionTrue,
		wantRunConditionReason: "DurationElapsed",
	}, {
		name:    "unexpected ref name",
		refName: "meow",
		params: `
  params:
  - name: duration
    value: 1s
`,
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "UnexpectedName",
		isParentPRCancelled:    false,
	}, {
		name:                   "no duration param",
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "MissingDuration",
		isParentPRCancelled:    false,
	}, {
		name: "extra param",
		params: `
  params:
  - name: duration
    value: 1s
  - name: not-duration
    value: blah
`, wantRunConditionType: apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "UnexpectedParams",
		isParentPRCancelled:    false,
	}, {
		name: "duration param is not a string",
		params: `
  params:
  - name: duration
    value:
    - blah
    - blah
    - blah
`,
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "MissingDuration",
		isParentPRCancelled:    false,
	}, {
		name: "invalid duration value",
		params: `
  params:
  - name: duration
    value: blah
`,
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "InvalidDuration",
		isParentPRCancelled:    false,
	}, {
		name:    "timeout",
		timeout: "1s",
		params: `
  params:
  - name: duration
    value: 2s
`,
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "TimedOut",
		isParentPRCancelled:    false,
	}, {
		name:    "timeout equals duration",
		timeout: "1s",
		params: `
  params:
  - name: duration
    value: 1s
`,
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "InvalidTimeOut",
		isParentPRCancelled:    false,
	}, {
		name:    "parent pr timeout",
		timeout: "1s",
		params: `
  params:
  - name: duration
    value: 2s
`,
		wantRunConditionType:   apis.ConditionSucceeded,
		wantRunConditionStatus: corev1.ConditionFalse,
		wantRunConditionReason: "Cancelled",
		isParentPRCancelled:    true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			rec := &Reconciler{
				Clock: testClock,
			}

			runName := helpers.ObjectNameForTest(t)
			runYAML := fmt.Sprintf(`
metadata:
  name: %s
spec:
  timeout: %s
  ref:
    apiVersion: %s
    kind: %s
    name: %s
`, runName, tc.timeout, apiVersion, kind, tc.refName)
			if tc.params != "" {
				runYAML = runYAML + tc.params
			}
			r := parse.MustParseRun(t, runYAML)
			if tc.isParentPRCancelled {
				r.Spec.Status = v1alpha1.RunSpecStatusCancelled
				r.Spec.StatusMessage = v1alpha1.RunCancelledByPipelineMsg
			}

			// Start reconciling the Run.
			// This will not return until the second Reconcile is done.
			err := rec.ReconcileKind(ctx, r)

			t.Logf("%#v", r)

			for err != nil {
				if strings.Contains(err.Error(), "requeue") {
					// simulate EnqueueAfter
					var dur time.Duration
					var dr error
					for _, p := range r.Spec.Params {
						if p.Name == "duration" {
							dur, dr = time.ParseDuration(p.Value.StringVal)
							if dr != nil {
								t.Error("failed to parse duration")
							}
							break
						}
					}
					to := r.GetTimeout()
					sleep := int(math.Min(to.Seconds(), dur.Seconds()))
					testClock.SetTime(testClock.Now().Add(time.Duration(sleep) * time.Second))
					rec.Clock = testClock
					err = rec.ReconcileKind(ctx, r)
				} else {
					t.Fatalf("ReconcileKind() = %v", err)
				}
			}

			// Compose expected Run
			wantRunYAML := fmt.Sprintf(`
metadata:
  name: %s
spec:
  timeout: %s
  ref:
    apiVersion: %s
    kind: %s
    name: %s
`, runName, tc.timeout, apiVersion, kind, tc.refName)
			if tc.params != "" {
				wantRunYAML = wantRunYAML + tc.params
			}
			wantRunYAML = wantRunYAML + fmt.Sprintf(`
status:
  conditions:
  - reason: %s
    status: %q
    type: %s
  observedGeneration: 0
`, tc.wantRunConditionReason, tc.wantRunConditionStatus, tc.wantRunConditionType)
			wantRun := parse.MustParseRun(t, wantRunYAML)
			if tc.isParentPRCancelled {
				wantRun.Spec.Status = v1alpha1.RunSpecStatusCancelled
				wantRun.Spec.StatusMessage = v1alpha1.RunCancelledByPipelineMsg
			}

			if d := cmp.Diff(wantRun, r,
				filterTypeMeta,
				filterObjectMeta,
				filterCondition,
				filterRunStatus,
			); d != "" {
				t.Errorf("-got +want: %v", d)
			}
		})
	}
}
