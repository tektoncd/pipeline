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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/run/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
)

const (
	apiVersion string = "wait.testing.tekton.dev/v1beta1"
	kind       string = "Wait"
)

var (
	filterTypeMeta        = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	filterObjectMeta      = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields")
	filterCondition       = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	filterCustomRunStatus = cmpopts.IgnoreFields(v1beta1.CustomRunStatusFields{}, "StartTime", "CompletionTime")

	now       = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	testClock = clock.NewFakePassiveClock(now)

	succeededCustomRunYAML = `
metadata:
  name: custom-run-succeeded
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
status:
  startTime: "2021-12-31T23:59:57Z"
`
	expectedSucceededCustomRunYAML = `
metadata:
  name: custom-run-succeeded
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
status:
  startTime: "2021-12-31T23:59:57Z"
  completionTime: "2022-01-01T00:00:00Z"
  conditions:
  - reason: DurationElapsed
    status: "True"
    type: Succeeded
`
	timedOutCustomRunYAML = `
metadata:
  name: custom-run-timed-out
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
status:
  startTime: "2021-12-31T00:00:00Z"
`
	expectedTimedOutCustomRunYAML = `
metadata:
  name: custom-run-timed-out
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
status:
  startTime: "2021-12-31T00:00:00Z"
  completeTime: "2022-01-00T00:00:00Z"
  conditions:
  - reason: TimedOut
    status: "False"
    type: Succeeded
`
	unexpectedRefNameCustomRunYAML = `
metadata:
  name: custom-run-unexpected-ref-name
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
    name: meow
  params:
  - name: duration
    value: 2s
`
	expectedUnexpectedRefNameCustomRunYAML = `
metadata:
  name: custom-run-unexpected-ref-name
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
    name: meow
  params:
  - name: duration
    value: 2s
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: UnexpectedName
`
	extraParamsCustomRunYAML = `
metadata:
  name: custom-run-extra-params
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
  - name: not-duration
    value: blah
`
	expectedExtraParamsCustomRunYAML = `
metadata:
  name: custom-run-extra-params
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
  - name: not-duration
    value: blah
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: UnexpectedParams
`
	noDurationCustomRunYAML = `
metadata:
  name: custom-run-no-duration
spec:
  timeout: "1s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
`
	expectedNoDurationCustomRunYAML = `
metadata:
  name: custom-run-no-duration
spec:
  timeout: "1s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: MissingDuration
`
	invalidDurationCustomRunYAML = `
metadata:
  name: custom-run-invalid-duration
spec:
  timeout: "1s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: blah
`
	expectedInvalidDurationCustomRunYAML = `
metadata:
  name: custom-run-invalid-duration
spec:
  timeout: "1s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: blah
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: InvalidDuration
`
	timeOutEqualsDurationCustomRunYAML = `
metadata:
  name: custom-run-timeout-equals-to-duration
spec:
  timeout: "1s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 1s
`
	expectedTimeOutEqualsDurationCustomRunYAML = `
metadata:
  name: custom-run-timeout-equals-to-duration
spec:
  timeout: "1s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 1s
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: InvalidTimeOut
`
	canceledCustomRunYAML = `
metadata:
  name: custom-run-cancelled
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  status: RunCancelled
  params:
  - name: duration
    value: 2s
`
	expectedCanceledCustomRunYAML = `
metadata:
  name: custom-run-cancelled
spec:
  timeout: "5s"
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  status: RunCancelled
  params:
  - name: duration
    value: 2s
status:
  conditions:
  - type: Succeeded
    reason: Cancelled
    status: "False"
`

	retryTimedOutCustomRunYAML = `
metadata:
  name: custom-run-timed-out-retry
spec:
  timeout: "5s"
  retries: 1
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
status:
  startTime: "2021-12-31T00:00:00Z"
`
	expectedRetryTimedOutCustomRunYAML = `
metadata:
  name: custom-run-timed-out-retry
spec:
  timeout: "5s"
  retries: 1
  customRef:
    apiVersion: "wait.testing.tekton.dev/v1beta1"
    kind: Wait
  params:
  - name: duration
    value: 2s
status:
  conditions:
  - reason: Retrying
    status: Unknown
    type: Succeeded
  retriesStatus:
    - startTime: "2021-12-31T00:00:00Z"
      conditions:
      - reason: TimedOut
        status: "False"
        type: Succeeded
`
)

func TestReconcile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		customRun         string
		expectedCustomRun string
	}{{
		name:              "duration elapsed",
		customRun:         succeededCustomRunYAML,
		expectedCustomRun: expectedSucceededCustomRunYAML,
	}, {
		name:              "unexpected ref name",
		customRun:         unexpectedRefNameCustomRunYAML,
		expectedCustomRun: expectedUnexpectedRefNameCustomRunYAML,
	}, {
		name:              "no duration param",
		customRun:         noDurationCustomRunYAML,
		expectedCustomRun: expectedNoDurationCustomRunYAML,
	}, {
		name:              "extra param",
		customRun:         extraParamsCustomRunYAML,
		expectedCustomRun: expectedExtraParamsCustomRunYAML,
	}, {
		name:              "invalid duration value",
		customRun:         invalidDurationCustomRunYAML,
		expectedCustomRun: expectedInvalidDurationCustomRunYAML,
	}, {
		name:              "timeout",
		customRun:         timedOutCustomRunYAML,
		expectedCustomRun: expectedTimedOutCustomRunYAML,
	}, {
		name:              "timeout equals duration",
		customRun:         timeOutEqualsDurationCustomRunYAML,
		expectedCustomRun: expectedTimeOutEqualsDurationCustomRunYAML,
	}, {
		name:              "parent pr timeout",
		customRun:         canceledCustomRunYAML,
		expectedCustomRun: expectedCanceledCustomRunYAML,
	}, {
		name:              "retry when timeout",
		customRun:         retryTimedOutCustomRunYAML,
		expectedCustomRun: expectedRetryTimedOutCustomRunYAML,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			rec := &Reconciler{
				Clock: testClock,
			}

			r := MustParseCustomRun(t, tc.customRun)

			err := rec.ReconcileKind(ctx, r)
			if err != nil && !strings.Contains(err.Error(), "requeue") {
				t.Fatalf("Failed to reconcile: %v", err)
			}

			// Compose expected CustomRun
			wantCustomRun := MustParseCustomRun(t, tc.expectedCustomRun)

			if d := cmp.Diff(r, wantCustomRun, filterTypeMeta, filterObjectMeta, filterCondition, filterCustomRunStatus); d != "" {
				t.Errorf("-got +want: %v", d)
			}
		})
	}
}

// MustParseCustomRun takes YAML and parses it into a *pipelinev1beta1.CustomRun
func MustParseCustomRun(t *testing.T, yaml string) *pipelinev1beta1.CustomRun {
	var r pipelinev1beta1.CustomRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: CustomRun
` + yaml
	if _, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, &r); err != nil {
		t.Fatalf("MustParseCustomRun (%s): %v", yaml, err)
	}
	return &r
}
