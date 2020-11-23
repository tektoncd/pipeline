// +build load

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

package test

import (
	"context"
	"flag"
	"sync"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	taskrunNum = flag.Int("taskrun-num", 50, "Number of TaskRun")
)

// TestPerformance creates multiple builds and checks that each of them has only one build pod.
func TestPerformance(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	var wg sync.WaitGroup
	for i := 0; i < *taskrunNum; i++ {
		wg.Add(1)
		taskrunName := helpers.ObjectNameForTest(t)
		t.Logf("Creating taskrun %q.", taskrunName)

		taskrun := &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: taskrunName, Namespace: namespace},
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image:   "busybox",
						Command: []string{"/bin/echo"},
						Args:    []string{"simple"},
					}}},
				},
			},
		}
		if _, err := c.TaskRunClient.Create(ctx, taskrun, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Error creating taskrun: %v", err)
		}
		go func(t *testing.T) {
			defer wg.Done()

			if err := WaitForTaskRunState(ctx, c, taskrunName, TaskRunSucceed(taskrunName), "TaskRunDuplicatePodTaskRunSuccess"); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
				return
			}
		}(t)
	}
	wg.Wait()
}
