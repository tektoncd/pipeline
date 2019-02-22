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
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
)

// TestTaskRunPipelineRunCancel is an integration test that will
// verify that pipelinerun cancel lead to the the correct TaskRun statuses
// and pod deletions.
func TestTaskRunPipelineRunCancel(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	logger.Infof("Creating Task in namespace %s", namespace)
	task := tb.Task("banana", namespace, tb.TaskSpec(
		tb.Step("foo", "ubuntu", tb.Command("/bin/bash"), tb.Args("-c", "sleep 500")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task `banana`: %s", err)
	}

	logger.Infof("Creating Pipeline in namespace %s", namespace)
	pipeline := tb.Pipeline("tomatoes", namespace,
		tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
	)
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", "tomatoes", err)
	}

	pipelineRun := tb.PipelineRun("pear", namespace, tb.PipelineRunSpec(pipeline.Name))

	logger.Infof("Creating PipelineRun in namespace %s", namespace)
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", "pear", err)
	}

	logger.Infof("Waiting for Pipelinerun %s in namespace %s to be started", "pear", namespace)
	if err := WaitForPipelineRunState(c, "pear", pipelineRunTimeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipelineRun %s already finished", "pear")
			} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
				return true, nil
			}
		}
		return false, nil
	}, "PipelineRunRunning"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be running: %s", "pear", err)
	}

	taskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=pear"})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", "pear", err)
	}

	logger.Infof("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", "pear", namespace)
	errChan := make(chan error, len(taskrunList.Items))
	defer close(errChan)

	for _, taskrunItem := range taskrunList.Items {
		go func(chan error) {
			err := WaitForTaskRunState(c, taskrunItem.Name, func(tr *v1alpha1.TaskRun) (bool, error) {
				if c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded); c != nil {
					if c.IsTrue() || c.IsFalse() {
						return true, fmt.Errorf("taskRun %s already finished!", taskrunItem.Name)
					} else if c.IsUnknown() && (c.Reason == "Running" || c.Reason == "Pending") {
						return true, nil
					}
				}
				return false, nil
			}, "TaskRunRunning")
			errChan <- err
		}(errChan)
	}

	for i := 1; i <= len(taskrunList.Items); i++ {
		if <-errChan != nil {
			t.Errorf("Error waiting for TaskRun %s to be running: %v", taskrunList.Items[i-1].Name, err)
		}
	}

	pr, err := c.PipelineRunClient.Get("pear", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun `%s`: %s", "pear", err)
	}

	pr.Spec.Status = v1alpha1.PipelineRunSpecStatusCancelled
	if _, err := c.PipelineRunClient.Update(pr); err != nil {
		t.Fatalf("Failed to cancel PipelineRun `%s`: %s", "pear", err)
	}

	logger.Infof("Waiting for PipelineRun %s in namespace %s to be cancelled", "pear", namespace)
	if err := WaitForPipelineRunState(c, "pear", pipelineRunTimeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		if c := pr.Status.GetCondition(duckv1alpha1.ConditionSucceeded); c != nil {
			if c.IsFalse() {
				if c.Reason == "PipelineRunCancelled" {
					return true, nil
				}
				return true, fmt.Errorf("pipelineRun %s completed with the wrong reason: %s", "pear", c.Reason)
			} else if c.IsTrue() {
				return true, fmt.Errorf("pipelineRun %s completed successfully, should have been cancelled", "pear")
			}
		}
		return false, nil
	}, "PipelineRunCancelled"); err != nil {
		t.Errorf("Error waiting for PipelineRun `pear` to finished: %s", err)
	}

	logger.Infof("Waiting for TaskRuns in PipelineRun %s in namespace %s to be cancelled", "pear", namespace)
	errChan2 := make(chan error, len(taskrunList.Items))
	defer close(errChan2)
	for _, taskrunItem := range taskrunList.Items {
		go func(errChan2 chan error) {
			err := WaitForTaskRunState(c, taskrunItem.Name, func(tr *v1alpha1.TaskRun) (bool, error) {
				if c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded); c != nil {
					if c.IsFalse() {
						if c.Reason == "TaskRunCancelled" {
							return true, nil
						}
						return true, fmt.Errorf("taskRun %s completed with the wrong reason: %s", taskrunItem.Name, c.Reason)
					} else if c.IsTrue() {
						return true, fmt.Errorf("taskRun %s completed successfully, should have been cancelled", taskrunItem.Name)
					}
				}
				return false, nil
			}, "TaskRunCancelled")
			errChan2 <- err
		}(errChan2)

	}
	for i := 1; i <= len(taskrunList.Items); i++ {
		if <-errChan2 != nil {
			t.Errorf("Error waiting for TaskRun %s to be finished: %v", taskrunList.Items[i-1].Name, err)
		}
	}
}
