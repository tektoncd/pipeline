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
	"testing"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	tb "github.com/knative/build-pipeline/test/builder"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPipelineRunTimeout is an integration test that will
// verify that pipelinerun timeout works and leads to the the correct TaskRun statuses
// and pod deletions.
func TestPipelineRunTimeout(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	logger.Infof("Creating Task in namespace %s", namespace)
	task := tb.Task("banana", namespace, tb.TaskSpec(
		tb.Step("foo", "busybox", tb.Command("/bin/sh"), tb.Args("-c", "sleep 10"))))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "banana", err)
	}

	pipeline := tb.Pipeline("tomatoes", namespace,
		tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
	)
	pipelineRun := tb.PipelineRun("pear", namespace, tb.PipelineRunSpec(pipeline.Name,
		tb.PipelineRunTimeout(&metav1.Duration{Duration: 5 * time.Second}),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	logger.Infof("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(c, pipelineRun.Name, timeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipelineRun %s already finished!", pipelineRun.Name)
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

	logger.Infof("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", pipelineRun.Name, namespace)
	errChan := make(chan error, len(taskrunList.Items))
	defer close(errChan)

	for _, taskrunItem := range taskrunList.Items {
		go func() {
			err := WaitForTaskRunState(c, taskrunItem.Name, func(tr *v1alpha1.TaskRun) (bool, error) {
				c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
				if c != nil {
					if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
						return true, fmt.Errorf("taskRun %s already finished!", taskrunItem.Name)
					} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
						return true, nil
					}
				}
				return false, nil
			}, "TaskRunRunning")
			errChan <- err
		}()

	}
	for i := 1; i <= len(taskrunList.Items); i++ {
		if <-errChan != nil {
			t.Errorf("Error waiting for TaskRun %s to be running: %s", taskrunList.Items[i-1].Name, err)
		}
	}

	if _, err := c.PipelineRunClient.Get(pipelineRun.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	logger.Infof("Waiting for PipelineRun %s in namespace %s to be timed out", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(c, pipelineRun.Name, timeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionFalse {
				if c.Reason == resources.ReasonTimedOut {
					return true, nil
				}
				return true, fmt.Errorf("pipelineRun %s completed with the wrong reason: %s", pipelineRun.Name, c.Reason)
			} else if c.Status == corev1.ConditionTrue {
				return true, fmt.Errorf("pipelineRun %s completed successfully, should have been timed out", pipelineRun.Name)
			}
		}
		return false, nil
	}, "PipelineRunTimedOut"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
	}

	logger.Infof("Waiting for TaskRuns from PipelineRun %s in namespace %s to be cancelled", pipelineRun.Name, namespace)
	errChan2 := make(chan error, len(taskrunList.Items))
	defer close(errChan2)

	for _, taskrunItem := range taskrunList.Items {
		go func() {
			err := WaitForTaskRunState(c, taskrunItem.Name, func(tr *v1alpha1.TaskRun) (bool, error) {
				cond := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
				if cond != nil {
					if cond.Status == corev1.ConditionFalse {
						if cond.Reason == "TaskRunTimeout" {
							return true, nil
						}
						return true, fmt.Errorf("taskRun %s completed with the wrong reason: %s", taskrunItem.Name, cond.Reason)
					} else if cond.Status == corev1.ConditionTrue {
						return true, fmt.Errorf("taskRun %s completed successfully, should have been timed out", taskrunItem.Name)
					}
				}
				return false, nil
			}, "TaskRunTimeout")
			errChan2 <- err
		}()

	}
	for i := 1; i <= len(taskrunList.Items); i++ {
		if <-errChan2 != nil {
			t.Errorf("Error waiting for TaskRun %s to timeout: %s", taskrunList.Items[i-1].Name, err)
		}
	}

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

	logger.Infof("Waiting for PipelineRun %s in namespace %s to complete", secondPipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(c, secondPipelineRun.Name, timeout, PipelineRunSucceed(secondPipelineRun.Name), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", secondPipelineRun.Name, err)
	}
}

// TestTaskRunTimeout is an integration test that will verify a TaskRun can be timed out.
func TestTaskRunTimeout(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	logger.Infof("Creating Task and TaskRun in namespace %s", namespace)
	if _, err := c.TaskClient.Create(tb.Task("giraffe", namespace,
		tb.TaskSpec(tb.Step("amazing-busybox", "busybox", tb.Command("/bin/sh"), tb.Args("-c", "sleep 300"))))); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "giraffe", err)
	}
	if _, err := c.TaskRunClient.Create(tb.TaskRun("run-giraffe", namespace, tb.TaskRunSpec(tb.TaskRunTaskRef("giraffe"),
		tb.TaskRunTimeout(10*time.Second)))); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", "run-giraffe", err)
	}

	logger.Infof("Waiting for TaskRun %s in namespace %s to complete", "run-giraffe", namespace)
	if err := WaitForTaskRunState(c, "run-giraffe", func(tr *v1alpha1.TaskRun) (bool, error) {
		cond := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if cond != nil {
			if cond.Status == corev1.ConditionFalse {
				if cond.Reason == "TaskRunTimeout" {
					return true, nil
				}
				return true, fmt.Errorf("taskRun %s completed with the wrong reason: %s", "run-giraffe", cond.Reason)
			} else if cond.Status == corev1.ConditionTrue {
				return true, fmt.Errorf("taskRun %s completed successfully, should have been timed out", "run-giraffe")
			}
		}

		return false, nil
	}, "TaskRunTimeout"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", "run-giraffe", err)
	}
}
