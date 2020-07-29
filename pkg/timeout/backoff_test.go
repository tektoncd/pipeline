/*
Copyright 2020 The Tekton Authors

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

package timeout

import (
	"testing"
	"time"

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

// TestBackoffDuration asserts that the backoffDuration func returns Durations
// within the timeout handler's bounds.
func TestBackoffDuration(t *testing.T) {
	testcases := []struct {
		description      string
		inputCount       uint
		jitterFunc       func(int) int
		expectedDuration time.Duration
	}{
		{
			description:      "an input count that is too large is rounded to the maximum allowed exponent",
			inputCount:       uint(maxBackoffExponent + 1),
			jitterFunc:       func(in int) int { return in },
			expectedDuration: maxBackoffSeconds * time.Second,
		},
		{
			description:      "a jittered number of seconds that is above the maximum allowed is constrained",
			inputCount:       1,
			jitterFunc:       func(in int) int { return maxBackoffSeconds + 1 },
			expectedDuration: maxBackoffSeconds * time.Second,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			// TODO:  this is not an exported function
			result := backoffDuration(tc.inputCount, tc.jitterFunc)
			if result != tc.expectedDuration {
				t.Errorf("expected %q received %q", tc.expectedDuration.String(), result.String())
			}
		})
	}
}

func TestSetTimer(t *testing.T) {
	taskRun := tb.TaskRun("test-taskrun-arbitrary-timer", tb.TaskRunNamespace(testNs), tb.TaskRunSpec(
		tb.TaskRunTaskRef(simpleTask.Name, tb.TaskRefAPIVersion("a1")),
		tb.TaskRunTimeout(2*time.Second),
	), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown}),
		tb.TaskRunStartTime(time.Now().Add(-10*time.Second)),
	))

	stopCh := make(chan struct{})
	observer, _ := observer.New(zap.InfoLevel)
	backoff := NewBackoff(stopCh, zap.New(observer).Sugar())
	timerDuration := 50 * time.Millisecond
	timerFailDeadline := 100 * time.Millisecond
	doneCh := make(chan struct{})
	callback := func(_ interface{}) {
		close(doneCh)
	}
	backoff.SetTimeoutCallback(callback)
	go backoff.SetTimer(taskRun, timerDuration)
	select {
	case <-doneCh:
		// The task run timer executed before the failure deadline
	case <-time.After(timerFailDeadline):
		t.Errorf("timer did not execute task run callback func within expected time")
	}
}
