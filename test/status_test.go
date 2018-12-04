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

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// TestTaskRunStatus is an integration test that will verify a very simple "hello world" TaskRun
// failure execution lead to the correct TaskRun status.
func TestTaskRunStatus(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	logger.Infof("Creating Task and TaskRun in namespace %s", namespace)
	task := Task("banana", namespace, TaskSpec(
		Step("foo", "busybox", Command("ls", "-la")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", hwTaskName, err)
	}
	taskRun := TaskRun("apple", namespace, TaskRunSpec("banana", TaskRunServiceAccount("inexistent")))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", hwTaskRunName, err)
	}

	logger.Infof("Waiting for TaskRun %s in namespace %s to fail", hwTaskRunName, namespace)
	if err := WaitForTaskRunState(c, "apple", func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, fmt.Errorf("task run %s succeeded!", "apple")
			} else if c.Status == corev1.ConditionFalse {
				return true, nil
			}
		}
		return false, nil
	}, "BuildValidationFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", hwTaskRunName, err)
	}

}

// TestPipelineRunStatus is an integration test that will verify a very simple "hello world" TaskRun
// failure execution lead to the correct PipelineRun status.
func TestPipelineRunStatus(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	logger.Infof("Creating Task and TaskRun in namespace %s", namespace)
	task := Task("banana", namespace, TaskSpec(
		Step("foo", "busybox", Command("ls", "-la")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", hwTaskName, err)
	}
	pipeline := Pipeline("tomatoes", namespace,
		PipelineSpec(PipelineTask("foo", "banana")),
	)
	pipelineRun := PipelineRun("pear", namespace,
		PipelineRunSpec("tomatoes", PipelineRunServiceAccount("inexistent")),
	)

	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", "tomatoes", err)
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", "pear", err)
	}

	logger.Infof("Waiting for PipelineRun %s in namespace %s to fail", hwTaskRunName, namespace)
	if err := WaitForPipelineRunState(c, "pear", pipelineRunTimeout, func(tr *v1alpha1.PipelineRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, fmt.Errorf("task run %s succeeded!", "apple")
			} else if c.Status == corev1.ConditionFalse {
				return true, nil
			}
		}
		return false, nil
	}, "BuildValidationFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", hwTaskRunName, err)
	}

}
