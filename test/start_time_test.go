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
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// TestStartTime tests that step start times are reported accurately.
//
// It runs a TaskRun with 5 steps that each sleep 10 seconds, then checks that
// the reported step start times are 10+ seconds apart from each other.
// Scheduling and reporting specifics can result in start times being reported
// more than 10s apart, but they shouldn't be less than 10s apart.
func TestStartTime(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)
	t.Logf("Creating TaskRun in namespace %q", namespace)
	tr, err := c.TaskRunClient.Create(&v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "start-time-test-",
			Namespace:    namespace,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []v1alpha1.Step{{
					Container: corev1.Container{Image: "ubuntu"},
					Script:    "sleep 10",
				}, {
					Container: corev1.Container{Image: "ubuntu"},
					Script:    "sleep 10",
				}, {
					Container: corev1.Container{Image: "ubuntu"},
					Script:    "sleep 10",
				}, {
					Container: corev1.Container{Image: "ubuntu"},
					Script:    "sleep 10",
				}, {
					Container: corev1.Container{Image: "ubuntu"},
					Script:    "sleep 10",
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Error creating TaskRun: %v", err)
	}
	t.Logf("Created TaskRun %q in namespace %q", tr.Name, namespace)
	// Wait for the TaskRun to complete.
	if err := WaitForTaskRunState(c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun to succeed: %v", err)
	}
	tr, err = c.TaskRunClient.Get(tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting TaskRun: %v", err)
	}
	if got, want := len(tr.Status.Steps), len(tr.Spec.TaskSpec.Steps); got != want {
		t.Errorf("Got unexpected number of step states: got %d, want %d", got, want)
	}
	var lastStart metav1.Time
	for idx, s := range tr.Status.Steps {
		if s.Terminated == nil {
			t.Errorf("Step state %d was not terminated", idx)
			continue
		}
		diff := s.Terminated.StartedAt.Time.Sub(lastStart.Time)
		if diff < 10*time.Second {
			t.Errorf("Step %d start time was %s since last start, wanted >10s", idx, diff)
		}
		lastStart = s.Terminated.StartedAt
	}
}
