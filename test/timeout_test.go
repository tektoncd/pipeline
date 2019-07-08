// +build e2e

/*
Copyright 2019 Knative Authors LLC
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
	"time"

	"github.com/knative/pkg/apis"
	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	tb "github.com/tektoncd/pipeline/test/builder"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPipelineRunTimeout is an integration test that will
// verify that pipelinerun timeout works and leads to the the correct TaskRun statuses
// and pod deletions.
func TestPipelineRunTimeout(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Task in namespace %s", namespace)
	task := tb.Task("banana", namespace, tb.TaskSpec(
		tb.Step("foo", "busybox", tb.Command("/bin/sh"), tb.Args("-c", "sleep 10"))))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "banana", err)
	}

	pipeline := tb.Pipeline("tomatoes", namespace,
		tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
	)
	pipelineRun := tb.PipelineRun("pear", namespace, tb.PipelineRunSpec(pipeline.Name,
		tb.PipelineRunTimeout(5*time.Second),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(c, pipelineRun.Name, timeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("pipelineRun %s already finished!", pipelineRun.Name)
			} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
				return true, nil
			}
		}
		return false, nil
	}, "PipelineRunRunning"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRun.Name, err)
	}

	taskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun.Name)})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", pipelineRun.Name, namespace)
	errChan := make(chan error, len(taskrunList.Items))
	defer close(errChan)

	for _, taskrunItem := range taskrunList.Items {
		go func(name string) {
			err := WaitForTaskRunState(c, name, func(tr *v1alpha1.TaskRun) (bool, error) {
				c := tr.Status.GetCondition(apis.ConditionSucceeded)
				if c != nil {
					if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
						return true, xerrors.Errorf("taskRun %s already finished!", name)
					} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
						return true, nil
					}
				}
				return false, nil
			}, "TaskRunRunning")
			errChan <- err
		}(taskrunItem.Name)
	}

	for i := 1; i <= len(taskrunList.Items); i++ {
		if <-errChan != nil {
			t.Errorf("Error waiting for TaskRun %s to be running: %s", taskrunList.Items[i-1].Name, err)
		}
	}

	if _, err := c.PipelineRunClient.Get(pipelineRun.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(c, pipelineRun.Name, timeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionFalse {
				if c.Reason == resources.ReasonTimedOut {
					return true, nil
				}
				return true, xerrors.Errorf("pipelineRun %s completed with the wrong reason: %s", pipelineRun.Name, c.Reason)
			} else if c.Status == corev1.ConditionTrue {
				return true, xerrors.Errorf("pipelineRun %s completed successfully, should have been timed out", pipelineRun.Name)
			}
		}
		return false, nil
	}, "PipelineRunTimedOut"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be cancelled", pipelineRun.Name, namespace)
	var wg sync.WaitGroup
	for _, taskrunItem := range taskrunList.Items {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := WaitForTaskRunState(c, name, func(tr *v1alpha1.TaskRun) (bool, error) {
				cond := tr.Status.GetCondition(apis.ConditionSucceeded)
				if cond != nil {
					if cond.Status == corev1.ConditionFalse {
						if cond.Reason == "TaskRunTimeout" {
							return true, nil
						}
						return true, xerrors.Errorf("taskRun %s completed with the wrong reason: %s", task.Name, cond.Reason)
					} else if cond.Status == corev1.ConditionTrue {
						return true, xerrors.Errorf("taskRun %s completed successfully, should have been timed out", name)
					}
				}
				return false, nil
			}, "TaskRunTimeout")
			if err != nil {
				t.Errorf("Error waiting for TaskRun %s to timeout: %s", name, err)
			}
		}(taskrunItem.Name)
	}
	wg.Wait()

	if _, err := c.PipelineRunClient.Get(pipelineRun.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	// Verify that we can create a second Pipeline using the same Task without a Pipeline-level timeout that will not
	// time out
	secondPipeline := tb.Pipeline("peppers", namespace,
		tb.PipelineSpec(tb.PipelineTask("foo", "banana")))
	secondPipelineRun := tb.PipelineRun("kiwi", namespace, tb.PipelineRunSpec("peppers"))
	if _, err := c.PipelineClient.Create(secondPipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", secondPipeline.Name, err)
	}
	if _, err := c.PipelineRunClient.Create(secondPipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", secondPipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", secondPipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(c, secondPipelineRun.Name, timeout, PipelineRunSucceed(secondPipelineRun.Name), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", secondPipelineRun.Name, err)
	}
}

func TestPipelineRunFailedAndRetry(t *testing.T) {
	numberOfRetries := 2
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Task in namespace %s", namespace)
	task := tb.Task("banana", namespace, tb.TaskSpec(
		tb.Step("foo", "busybox", tb.Command("/bin/sh"), tb.Args("-c", "exit 1")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "banana", err)
	}

	pipeline := tb.Pipeline("tomatoes", namespace,
		tb.PipelineSpec(tb.PipelineTask("foo", "banana", tb.Retries(numberOfRetries))),
	)
	pipelineRun := tb.PipelineRun("pear", namespace, tb.PipelineRunSpec(pipeline.Name))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "PipelineRunRunning"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be failed: %s", pipelineRun.Name, err)
	}

	r, err := c.PipelineRunClient.Get(pipelineRun.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting pipeline %s", pipelineRun.Name)
	}

	if len(r.Status.TaskRuns) != 1 {
		t.Fatalf("Only one TaskRun is expected, but got %d", len(r.Status.TaskRuns))
	}

	for taskRunName := range r.Status.TaskRuns {
		taskrun, err := c.TaskRunClient.Get(taskRunName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Error getting task run %s", taskRunName)
		}
		if len(taskrun.Status.RetriesStatus) != numberOfRetries {
			t.Fatalf("expected %d retry, but got %d", numberOfRetries, len(r.Status.TaskRuns))
		}
	}
}

// TestTaskRunTimeout is an integration test that will verify a TaskRun can be timed out.
func TestTaskRunTimeout(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	if _, err := c.TaskClient.Create(tb.Task("giraffe", namespace,
		tb.TaskSpec(tb.Step("amazing-busybox", "busybox", tb.Command("/bin/sh"), tb.Args("-c", "sleep 3000"))))); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "giraffe", err)
	}
	if _, err := c.TaskRunClient.Create(tb.TaskRun("run-giraffe", namespace, tb.TaskRunSpec(tb.TaskRunTaskRef("giraffe"),
		// Do not reduce this timeout. Taskrun e2e test is also verifying
		// if reconcile is triggered from timeout handler and not by pod informers
		tb.TaskRunTimeout(30*time.Second)))); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", "run-giraffe", err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", "run-giraffe", namespace)
	if err := WaitForTaskRunState(c, "run-giraffe", func(tr *v1alpha1.TaskRun) (bool, error) {
		cond := tr.Status.GetCondition(apis.ConditionSucceeded)
		if cond != nil {
			if cond.Status == corev1.ConditionFalse {
				if cond.Reason == "TaskRunTimeout" {
					return true, nil
				}
				return true, xerrors.Errorf("taskRun %s completed with the wrong reason: %s", "run-giraffe", cond.Reason)
			} else if cond.Status == corev1.ConditionTrue {
				return true, xerrors.Errorf("taskRun %s completed successfully, should have been timed out", "run-giraffe")
			}
		}

		return false, nil
	}, "TaskRunTimeout"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", "run-giraffe", err)
	}
}
