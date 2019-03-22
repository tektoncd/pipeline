// +build e2e

/*
Copyright 2018 Knative Authors LLC
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
	"fmt"
	"sync"
	"testing"

	knativetest "github.com/knative/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	tb "github.com/tektoncd/pipeline/test/builder"
)

// TestDuplicatePodTaskRun creates 10 builds and checks that each of them has only one build pod.
func TestDuplicatePodTaskRun(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		taskrunName := fmt.Sprintf("duplicate-pod-taskrun-%d", i)
		t.Logf("Creating taskrun %q.", taskrunName)

		taskrun := tb.TaskRun(taskrunName, namespace, tb.TaskRunSpec(
			tb.TaskRunTaskSpec(tb.Step("echo", "busybox",
				tb.Command("/bin/echo"),
				tb.Args("simple"),
			)),
		))
		if _, err := c.TaskRunClient.Create(taskrun); err != nil {
			t.Fatalf("Error creating taskrun: %v", err)
		}
		go func(t *testing.T) {
			defer wg.Done()

			if err := WaitForTaskRunState(c, taskrunName, TaskRunSucceed(taskrunName), "TaskRunDuplicatePodTaskRunFailed"); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
				return
			}

			pods, err := c.KubeClient.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", pipeline.GroupName+pipeline.TaskRunLabelKey, taskrunName),
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
