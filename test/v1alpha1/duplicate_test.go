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
	"sync"
	"testing"

	"github.com/tektoncd/pipeline/test/parse"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestDuplicatePodTaskRun creates multiple builds and checks that each of them has only one build pod.
func TestDuplicatePodTaskRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	var wg sync.WaitGroup
	// The number of builds generated has a direct impact on test
	// runtime and is traded off against proving the taskrun
	// reconciler's efficacy at not duplicating pods.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskrunName := helpers.ObjectNameForTest(t)
		t.Logf("Creating taskrun %q.", taskrunName)

		taskrun := parse.MustParseAlphaTaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - image: busybox
      command: ['/bin/echo']
      args: ['simple']
`, taskrunName))
		if _, err := c.TaskRunClient.Create(ctx, taskrun, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Error creating taskrun: %v", err)
		}
		go func(t *testing.T) {
			defer wg.Done()

			if err := WaitForTaskRunState(ctx, c, taskrunName, TaskRunSucceed(taskrunName), "TaskRunDuplicatePodTaskRunFailed"); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
				return
			}

			pods, err := c.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", pipeline.TaskRunLabelKey, taskrunName),
			})
			if err != nil {
				t.Errorf("Error getting TaskRun pod list: %v", err)
				return
			}
			if n := len(pods.Items); n != 1 {
				t.Errorf("Error matching the number of build pods: expecting 1 pod, got %d", n)
				return
			}
		}(t)
	}
	wg.Wait()
}
