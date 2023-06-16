//go:build e2e
// +build e2e

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

package test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestPipelineRunTimeout is an integration test that will
// verify that pipelinerun timeout works and leads to the correct TaskRun statuses
// and pod deletions.
func TestPipelineRunTimeout(t *testing.T) {
	t.Parallel()
	// cancel the context after we have waited a suitable buffer beyond the given deadline.
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	t.Logf("Creating Task in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 10']
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: foo
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace, task.Name))
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  timeouts:
    pipeline: 5s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonTimedOut.String(), pipelineRun.Name), "PipelineRunTimedOut", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonTimedOut.String(), pipelineRun.Name), "PipelineRunTimedOut", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for TaskRuns from PipelineRun %s in namespace %s to time out and be cancelled", pipelineRun.Name, namespace)
	var wg sync.WaitGroup
	for _, taskrunItem := range taskrunList.Items {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := WaitForTaskRunState(ctx, c, name, FailedWithReason(v1.TaskRunReasonCancelled.String(), name), v1.TaskRunReasonCancelled.String(), v1Version)
			if err != nil {
				t.Errorf("Error waiting for TaskRun %s to timeout: %s", name, err)
			}
		}(taskrunItem.Name)
	}
	wg.Wait()

	if _, err := c.V1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	// Verify that we can create a second Pipeline using the same Task without a Pipeline-level timeout that will not
	// time out
	secondPipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: foo
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace, task.Name))
	secondPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, helpers.ObjectNameForTest(t), namespace, secondPipeline.Name))
	if _, err := c.V1PipelineClient.Create(ctx, secondPipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", secondPipeline.Name, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, secondPipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", secondPipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", secondPipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, secondPipelineRun.Name, timeout, PipelineRunSucceed(secondPipelineRun.Name), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", secondPipelineRun.Name, err)
	}
}

// TestStepTimeout is an integration test that will verify a Step can be timed out.
func TestStepTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	t.Logf("Creating Task with Step step-no-timeout, Step step-timeout, and Step step-canceled in namespace %s", namespace)

	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - name: no-timeout
      image: busybox
      script: sleep 1
      timeout: 2s
    - name: timeout
      image: busybox
      script: sleep 1
      timeout: 1ms
    - name: canceled
      image: busybox
      script: sleep 1
`, helpers.ObjectNameForTest(t), namespace))
	t.Logf("Creating TaskRun %s in namespace %s", taskRun.Name, namespace)
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", taskRun.Name, err)
	}

	failMsg := "\"step-timeout\" exited because the step exceeded the specified timeout limit"
	t.Logf("Waiting for %s in namespace %s to time out", "step-timeout", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRun.Name, FailedWithMessage(failMsg, taskRun.Name), "StepTimeout", v1Version); err != nil {
		t.Logf("Error in taskRun %s status: %s\n", taskRun.Name, err)
		t.Errorf("Expected: %s", failMsg)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting Taskrun: %v", err)
	}
	if tr == nil {
		t.Fatalf("no TaskRun details available")
	}
	if tr.Status.Steps[0].Terminated == nil {
		t.Errorf("step-no-timeout should have Completed.")
	} else if tr.Status.Steps[0].Terminated.Reason != "Completed" {
		t.Errorf("step-no-timeout should not have been terminated")
	}
	if tr.Status.Steps[2].Terminated == nil {
		t.Errorf("step-canceled should have been canceled after step-timeout timed out")
	} else if exitcode := tr.Status.Steps[2].Terminated.ExitCode; exitcode != 1 {
		t.Logf("step-canceled exited with exit code %d, expected exit code 1", exitcode)
	}
}

// TestStepTimeoutWithWS is an integration test that will verify a Step can be timed out.
func TestStepTimeoutWithWS(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	taskRun := parse.MustParseV1TaskRun(t, `
metadata:
  name: taskrun-with-timeout-step
spec:
  workspaces:
    - name: test
      emptyDir: {}
  taskSpec:
    workspaces:
      - name: test
    steps:
      - name: timeout
        image: busybox
        script: sleep 1
        timeout: 1ms`)

	t.Logf("Creating TaskRun %s in namespace %s", taskRun.Name, namespace)
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", taskRun.Name, err)
	}

	failMsg := "\"step-timeout\" exited because the step exceeded the specified timeout limit"
	t.Logf("Waiting for %s in namespace %s to time out", "step-timeout", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRun.Name, FailedWithMessage(failMsg, taskRun.Name), "StepTimeout", v1Version); err != nil {
		t.Logf("Error in taskRun %s status: %s\n", taskRun.Name, err)
		t.Errorf("Expected: %s", failMsg)
	}
}

// TestTaskRunTimeout is an integration test that will verify a TaskRun can be timed out.
func TestTaskRunTimeout(t *testing.T) {
	t.Parallel()
	timeout := 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 3000']
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  timeout: %s
`, helpers.ObjectNameForTest(t), namespace, task.Name, timeout))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", taskRun.Name, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", taskRun.Name, namespace)
	if err := WaitForTaskRunState(ctx, c, taskRun.Name, FailedWithReason(v1.TaskRunReasonTimedOut.String(), taskRun.Name), v1.TaskRunReasonTimedOut.String(), v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", taskRun.Name, err)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, taskRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving TaskRun %s: %v", taskRun.Name, err)
	}

	for _, step := range tr.Status.Steps {
		if step.Terminated == nil {
			t.Errorf("TaskRun %s step %s does not have a terminated state but should", taskRun.Name, step.Name)
		}
		if d := cmp.Diff(step.Terminated.Reason, v1.TaskRunReasonTimedOut.String()); d != "" {
			t.Fatalf("-got, +want: %v", d)
		}
	}
}

func TestPipelineTaskTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	t.Logf("Creating Tasks in namespace %s", namespace)
	task1 := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 1s']
`, helpers.ObjectNameForTest(t), namespace))
	task2 := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 10s']
`, helpers.ObjectNameForTest(t), namespace))

	if _, err := c.V1TaskClient.Create(ctx, task1, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task1.Name, err)
	}
	if _, err := c.V1TaskClient.Create(ctx, task2, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task2.Name, err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: pipelinetask1
    taskRef:
      name: %s
    timeout: 60s
  - name: pipelinetask2
    taskRef:
      name: %s
    timeout: 5s
`, helpers.ObjectNameForTest(t), namespace, task1.Name, task2.Name))
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))

	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s with PipelineTask timeout in namespace %s to fail", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonFailed.String(), pipelineRun.Name), "PipelineRunTimedOut", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for TaskRun from PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	var wg sync.WaitGroup
	for _, taskrunItem := range taskrunList.Items {
		wg.Add(1)
		go func(tr v1.TaskRun) {
			defer wg.Done()
			name := tr.Name
			err := WaitForTaskRunState(ctx, c, name, func(ca apis.ConditionAccessor) (bool, error) {
				cond := ca.GetCondition(apis.ConditionSucceeded)
				if cond != nil {
					if tr.Spec.TaskRef.Name == task1.Name && cond.Status == corev1.ConditionTrue {
						if cond.Reason == "Succeeded" {
							return true, nil
						}
						return true, fmt.Errorf("taskRun %q completed with the wrong reason: %s", task1.Name, cond.Reason)
					} else if tr.Spec.TaskRef.Name == task1.Name && cond.Status == corev1.ConditionFalse {
						return true, fmt.Errorf("taskRun %q failed, but should have been Succeeded", name)
					}

					if tr.Spec.TaskRef.Name == task2.Name && cond.Status == corev1.ConditionFalse {
						if cond.Reason == v1.TaskRunReasonTimedOut.String() {
							return true, nil
						}
						return true, fmt.Errorf("taskRun %q completed with the wrong reason: %s", task2.Name, cond.Reason)
					} else if tr.Spec.TaskRef.Name == task2.Name && cond.Status == corev1.ConditionTrue {
						return true, fmt.Errorf("taskRun %q should have timed out", name)
					}
				}
				return false, nil
			}, v1.TaskRunReasonCancelled.String(), v1Version)
			if err != nil {
				t.Errorf("Error waiting for TaskRun %s to timeout: %s", name, err)
			}
		}(taskrunItem)
	}
	wg.Wait()
}

// TestPipelineRunTasksTimeout is an integration test that will
// verify that pipelinerun tasksTimeout works and leads to the correct PipelineRun and TaskRun statuses
// and pod deletions.
func TestPipelineRunTasksTimeout(t *testing.T) {
	t.Parallel()
	// cancel the context after we have waited a suitable buffer beyond the given deadline.
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Minute)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(context.Background(), t, c, namespace) }, t.Logf)
	defer tearDown(context.Background(), t, c, namespace)

	t.Logf("Creating Task in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 30']
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}

	t.Logf("Creating Finally Task in namespace %s", namespace)
	fTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 1']
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, fTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", fTask.Name, err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: dagtask
    taskRef:
      name: %s
  finally:
  - name: finallytask
    taskRef:
      name: %s
`, helpers.ObjectNameForTest(t), namespace, task.Name, fTask.Name))
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  timeouts:
    pipeline: 60s
    tasks: 20s
    finally: 20s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be failed", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, FailedWithReason(v1.PipelineRunReasonFailed.String(), pipelineRun.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for TaskRun from PipelineRun %s in namespace %s to time out and finally TaskRun to be successful", pipelineRun.Name, namespace)
	var wg sync.WaitGroup
	for _, taskrunItem := range taskrunList.Items {
		wg.Add(1)
		go func(tr v1.TaskRun) {
			defer wg.Done()
			name := tr.Name
			err := WaitForTaskRunState(ctx, c, name, func(ca apis.ConditionAccessor) (bool, error) {
				cond := ca.GetCondition(apis.ConditionSucceeded)
				if cond != nil {
					if tr.Spec.TaskRef.Name == fTask.Name && cond.Status == corev1.ConditionTrue {
						if cond.Reason == "Succeeded" {
							return true, nil
						}
						return true, fmt.Errorf("taskRun %q completed with the wrong reason: %s", fTask.Name, cond.Reason)
					} else if tr.Spec.TaskRef.Name == fTask.Name && cond.Status == corev1.ConditionFalse {
						return true, fmt.Errorf("taskRun %q failed, but should have been Succeeded", name)
					}

					if tr.Spec.TaskRef.Name == task.Name && cond.Status == corev1.ConditionFalse {
						if !strings.Contains(cond.Message, string(v1.TaskRunCancelledByPipelineTimeoutMsg)) {
							return true, fmt.Errorf("taskRun %s completed with the wrong message: %s", task.Name, cond.Message)
						}
						if cond.Reason == v1.TaskRunReasonCancelled.String() {
							return true, nil
						}
						return true, fmt.Errorf("taskRun %q completed with the wrong reason: %s", task.Name, cond.Reason)
					} else if tr.Spec.TaskRef.Name == task.Name && cond.Status == corev1.ConditionTrue {
						return true, fmt.Errorf("taskRun %q should have timed out", name)
					}
				}
				return false, nil
			}, v1.TaskRunReasonCancelled.String(), v1Version)

			if err != nil {
				t.Errorf("Error waiting for TaskRun %s to timeout: %s", name, err)
			}
		}(taskrunItem)
	}
	wg.Wait()
}
