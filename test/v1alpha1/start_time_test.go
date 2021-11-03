// +build e2e

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
package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/test/parse"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// TestStartTime tests that step start times are reported accurately.
//
// It runs a TaskRun with multiple steps that each sleep for several
// seconds, then checks that the reported step start times are several
// seconds apart from each other.  Scheduling and reporting specifics
// can result in start times being later than expected, but they
// shouldn't be earlier.
//
// The number of seconds between each step has a big impact on the total
// duration of the test so smaller is better (while still supporting the
// test's intended purpose).
func TestStartTime(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)
	t.Logf("Creating TaskRun in namespace %q", namespace)
	tr, err := c.TaskRunClient.Create(ctx, parse.MustParseAlphaTaskRun(t, fmt.Sprintf(`
metadata:
  generateName: start-time-test-
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: busybox
      script: sleep 2
    - image: busybox
      script: sleep 2
    - image: busybox
      script: sleep 2
    - image: busybox
      script: sleep 2
    - image: busybox
      script: sleep 2
`, namespace)), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating TaskRun: %v", err)
	}
	t.Logf("Created TaskRun %q in namespace %q", tr.Name, namespace)
	// Wait for the TaskRun to complete.
	if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun to succeed: %v", err)
	}
	tr, err = c.TaskRunClient.Get(ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting TaskRun: %v", err)
	}
	if got, want := len(tr.Status.Steps), len(tr.Spec.TaskSpec.Steps); got != want {
		t.Errorf("Got unexpected number of step states: got %d, want %d", got, want)
	}
	minimumDiff := 2 * time.Second
	var lastStart metav1.Time
	for idx, s := range tr.Status.Steps {
		if s.Terminated == nil {
			t.Errorf("Step state %d was not terminated", idx)
			continue
		}
		diff := s.Terminated.StartedAt.Time.Sub(lastStart.Time)
		if diff < minimumDiff {
			t.Errorf("Step %d start time was %s since last start, wanted > %s", idx, diff, minimumDiff)
		}
		lastStart = s.Terminated.StartedAt
	}
}
