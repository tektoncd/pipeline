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
	"os"
	"path/filepath"
	"testing"

	knativetest "github.com/knative/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

// TestStorageTaskRun is an integration test that will verify GCS input resource runtime contract
// - adds volumes with expected name
// - places files in expected place

func TestStorageTaskRun(t *testing.T) {
	configFilePath := os.Getenv("GCP_SERVICE_ACCOUNT_KEY_PATH")
	if configFilePath == "" {
		t.Skip("GCP_SERVICE_ACCOUNT_KEY_PATH variable is not set.")
	}
	t.Parallel()

	c, namespace := setup(t)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	secretName := "auth-secret"
	_, err := CreateGCPServiceAccountSecret(t, c.KubeClient, namespace, secretName)
	if err != nil {
		t.Fatalf("could not create secret %s", err)
	}

	resName := "gcs-resource"
	if _, err := c.PipelineResourceClient.Create(getResources(namespace, resName, secretName, configFilePath)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", resName, err)
	}

	taskRunName := "gcs-taskrun"
	t.Logf("Creating Task and TaskRun %s in namespace %s", taskRunName, namespace)

	if _, err := c.TaskClient.Create(getGCSStorageTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task gcs-file : %s", err)
	}

	if _, err := c.TaskRunClient.Create(getGCSTaskRun(namespace, taskRunName, resName)); err != nil {
		t.Fatalf("Failed to create TaskRun %s: %s", taskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", taskRunName, namespace)

	if err := WaitForTaskRunState(c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", taskRunName, err)
	}
	t.Logf("TaskRun %s succeeded", taskRunName)
}

func getGCSStorageTask(namespace string) *v1alpha1.Task {
	return tb.Task("gcs-file", namespace, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("gcsbucket", v1alpha1.PipelineResourceTypeStorage,
			tb.ResourceTargetPath("gcs-workspace"),
		)),
		tb.Step("read-gcs-bucket", "ubuntu", tb.Command("/bin/bash"),
			tb.Args("-c", "ls -la /workspace/gcs-workspace/rules_docker-master.zip"),
		),
	))
}

func getGCSTaskRun(namespace, name, resName string) *v1alpha1.TaskRun {
	return tb.TaskRun(name, namespace,
		tb.TaskRunSpec(tb.TaskRunTaskRef("gcs-file"),
			tb.TaskRunInputs(
				tb.TaskRunInputsResource("gcsbucket", tb.TaskResourceBindingRef(resName)),
			)))
}

func getResources(namespace, name, secretName, configFile string) *v1alpha1.PipelineResource {
	res := tb.PipelineResource(name, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("location", "gs://build-crd-tests/rules_docker-master.zip"),
		tb.PipelineResourceSpecParam("type", "gcs"),
	))
	if configFile != "" {
		jsonKeyFilename := filepath.Base(configFile)
		tb.PipelineResourceSpecSecretParam("GOOGLE_APPLICATION_CREDENTIALS", secretName, jsonKeyFilename)(&res.Spec)
	}
	return res
}
