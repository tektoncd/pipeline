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
	"fmt"
	"testing"
	"time"

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	rtesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

var timerFailDeadline = 100 * time.Millisecond

func TestGet(t *testing.T) {
	type expected struct {
		inProgress  bool
		numAttempts uint
		nextAttempt time.Time
	}
	now := time.Now()
	tr := tb.TaskRun("tr", tb.TaskRunStatus(tb.TaskRunStartTime(now)))
	trHourTimeout := tb.TaskRun("tr",
		tb.TaskRunSpec(tb.TaskRunTimeout(time.Hour)),
		tb.TaskRunStatus(tb.TaskRunStartTime(now)))
	trSecTimeout := tb.TaskRun("tr",
		tb.TaskRunSpec(tb.TaskRunTimeout(time.Second)),
		tb.TaskRunStatus(tb.TaskRunStartTime(now)))
	for _, tc := range []struct {
		name     string
		attempts map[string]Attempts
		currTime time.Time
		tr       *v1beta1.TaskRun
		e        expected
	}{{
		name:     "first attempt, no timeout",
		attempts: map[string]Attempts{},
		currTime: now,
		tr:       tr,
		e: expected{
			inProgress:  false,
			numAttempts: 1,
			nextAttempt: now.Add(time.Second * 4),
		},
	}, {
		name:     "first attempt with timeout",
		attempts: map[string]Attempts{},
		currTime: now,
		tr:       trHourTimeout,
		e: expected{
			inProgress:  false,
			numAttempts: 1,
			nextAttempt: now.Add(time.Second * 4),
		},
	}, {
		name:     "first attempt backout exceeds timeout",
		attempts: map[string]Attempts{},
		currTime: now,
		tr:       trSecTimeout,
		e: expected{
			inProgress:  false,
			numAttempts: 1,
			nextAttempt: now.Add(time.Second * 1),
		},
	}, {
		name: "subsequent attempt in progress",
		attempts: map[string]Attempts{
			fmt.Sprintf("TaskRun/%p", tr): Attempts{
				NumAttempts: 5,
				NextAttempt: now.Add(time.Second * 15),
			},
		},
		currTime: now,
		tr:       tr,
		e: expected{
			inProgress:  true,
			numAttempts: 5,
			nextAttempt: now.Add(time.Second * 15),
		},
	}, {
		name: "subsequent attempt time for next attempt",
		attempts: map[string]Attempts{
			fmt.Sprintf("TaskRun/%p", tr): Attempts{
				NumAttempts: 5,
				NextAttempt: now.Add(time.Second * -15),
			},
		},
		currTime: now,
		tr:       tr,
		e: expected{
			inProgress:  false,
			numAttempts: 6,
			nextAttempt: now.Add(time.Second * 66),
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			backoff := Backoff{
				attempts: tc.attempts,
				j:        func(i int) int { return i + 1 },
				now:      func() time.Time { return now },
				logger:   rtesting.TestLogger(t),
			}
			a, inProgress := backoff.Get(tc.tr)
			if inProgress != tc.e.inProgress {
				t.Errorf("expected inProgress to be %t but was %t", tc.e.inProgress, inProgress)
			}
			if a.NextAttempt != tc.e.nextAttempt {
				t.Errorf("expected next attempt to be at %s but is %s", tc.e.nextAttempt, a.NextAttempt)
			}
			if a.NumAttempts != tc.e.numAttempts {
				t.Errorf("expected num attempts to be %d but is %d", tc.e.numAttempts, a.NumAttempts)
			}
		})
	}
}

func TestGetExponentialBackoffWithJitter(t *testing.T) {
	testcases := []struct {
		name             string
		inputCount       uint
		jitterFunc       func(int) int
		expectedDuration time.Duration
	}{
		{
			name:             "an input count that is too large is rounded to the maximum allowed exponent",
			inputCount:       uint(maxBackoffExponent + 1),
			jitterFunc:       func(in int) int { return in },
			expectedDuration: maxBackoffSeconds * time.Second,
		},
		{
			name:             "a jittered number of seconds that is above the maximum allowed is constrained",
			inputCount:       1,
			jitterFunc:       func(in int) int { return maxBackoffSeconds + 1 },
			expectedDuration: maxBackoffSeconds * time.Second,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetExponentialBackoffWithJitter(tc.inputCount, tc.jitterFunc)
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
	doneCh := make(chan struct{})
	callback := func(_ interface{}) {
		close(doneCh)
	}
	backoff.SetTimeoutCallback(callback)
	go backoff.SetTimer(taskRun, time.Millisecond)
	select {
	case <-doneCh:
		// The task run timer executed before the failure deadline
	case <-time.After(timerFailDeadline):
		t.Errorf("timer did not execute task run callback func within expected time")
	}
}
