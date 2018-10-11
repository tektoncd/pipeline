// +build e2e

/*
Copyright 2018 The Knative Authors

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
	"strings"
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

func TestPipelineRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(logger, c.KubeClient, namespace) }, logger)
	defer tearDown(logger, c.KubeClient, namespace)

	logger.Infof("Creating Pipeline Resources in namespace %s", namespace)
	if _, err := c.TaskClient.Create(getHelloWorldTask(namespace, []string{"echo", taskOutput})); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", hwTaskName, err)
	}
	if _, err := c.PipelineClient.Create(getHelloWorldPipeline(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", hwPipelineName, err)
	}
	if _, err := c.PipelineParamsClient.Create(getHelloWorldPipelineParams(namespace)); err != nil {
		t.Fatalf("Failed to create PipelineParams `%s`: %s", hwPipelineParamsName, err)
	}
	if _, err := c.PipelineRunClient.Create(getHelloWorldPipelineRun(namespace)); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", hwPipelineRunName, err)
	}

	logger.Infof("Waiting for PipelineRun %s in namespace %s to complete", hwPipelineRunName, namespace)
	if err := WaitForPipelineRunState(c, hwPipelineRunName, func(tr *v1alpha1.PipelineRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil && c.Status == corev1.ConditionTrue {
			return true, nil
		}
		return false, nil
	}, "PipelineRunSuccess"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", hwTaskRunName, err)
	}
	logger.Infof("Making sure the expected TaskRuns were created")
	expectedTaskRuns := []string{
		strings.Join([]string{hwPipelineRunName, hwPipelineTaskName1}, "-"),
		strings.Join([]string{hwPipelineRunName, hwPipelineTaskName2}, "-"),
	}
	for _, runName := range expectedTaskRuns {
		r, err := c.TaskRunClient.Get(runName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Couldn't get expected TaskRun %s: %s", runName, err)
		} else {
			c := r.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if c.Status != corev1.ConditionTrue {
				t.Errorf("Expected TaskRun %s to have succeeded but Status is %s", runName, c.Status)
			}
		}
	}
	VerifyBuildOutput(t, c, namespace, taskOutput)
}
