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
	"testing"

	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"

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

	// TODO wait for the Run to be successful
	logger.Infof("Waiting for PipelineRun %s in namespace %s to be updated by controller", hwPipelineRunName, namespace)
	if err := WaitForPipelineRunState(c, hwPipelineRunName, func(tr *v1alpha1.PipelineRun) (bool, error) {
		if len(tr.Status.Conditions) > 0 {
			// TODO: use actual conditions
			return true, nil
		}
		return false, nil
	}, "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", hwTaskRunName, err)
	}

	// TODO check that TaskRuns created

	// Verify that the init containers Build ran had 'taskOutput' written
	// VerifyBuildOutput(t, c, namespace, taskOutput)
}
