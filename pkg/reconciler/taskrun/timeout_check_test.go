/*
Copyright 2019 The Tekton Authors

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

package taskrun

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"knative.dev/pkg/apis"
)

func TestCheckTimeout(t *testing.T) {
	// IsZero reports whether t represents the zero time instant, January 1, year 1, 00:00:00 UTC
	zeroTime := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	testCases := []struct {
		name           string
		taskRun        *v1alpha1.TaskRun
		expectedStatus bool
	}{{
		name: "TaskRun not started",
		taskRun: tb.TaskRun("test-taskrun-not-started", "foo", tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name),
		), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{}), tb.TaskRunStartTime(zeroTime))),
		expectedStatus: false,
	}, {
		name: "TaskRun no timeout",
		taskRun: tb.TaskRun("test-taskrun-no-timeout", "foo", tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name), tb.TaskRunTimeout(0),
		), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{}), tb.TaskRunStartTime(time.Now().Add(-15*time.Hour)))),
		expectedStatus: false,
	}, {
		name: "TaskRun timed out",
		taskRun: tb.TaskRun("test-taskrun-timeout", "foo", tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name), tb.TaskRunTimeout(10*time.Second),
		), tb.TaskRunStatus(tb.StatusCondition(apis.Condition{}), tb.TaskRunStartTime(time.Now().Add(-15*time.Second)))),
		expectedStatus: true,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CheckTimeout(tc.taskRun)
			if d := cmp.Diff(result, tc.expectedStatus); d != "" {
				t.Fatalf("-want, +got: %v", d)
			}
		})
	}
}
