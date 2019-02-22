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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestStorageTaskRun is an integration test that will verify GCS input resource runtime contract
// - adds volumes with expected name
// - places files in expected place

func TestStorageTaskRun(t *testing.T) {
	configFile := os.Getenv("GCP_SERVICE_ACCOUNT_KEY_PATH")
	if configFile == "" {
		t.Skip("GCP_SERVICE_ACCOUNT_KEY_PATH variable is not set.")
	}
	logger := logging.GetContextLogger(t.Name())
	t.Parallel()

	c, namespace := setup(t, logger)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	authSec := createGCSSecret(t, logger, namespace, configFile)
	if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(authSec); err != nil {
		t.Fatalf("Failed to create secret %s", err)
	}

	resName := "gcs-resource"
	if _, err := c.PipelineResourceClient.Create(getResources(namespace, resName, authSec.Name, filepath.Base(configFile))); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", resName, err)
	}

	taskRunName := "gcs-taskrun"
	logger.Infof("Creating Task and TaskRun %s in namespace %s", taskRunName, namespace)

	if _, err := c.TaskClient.Create(getGCSStorageTask(namespace, authSec.Name, filepath.Base(configFile))); err != nil {
		t.Fatalf("Failed to create Task gcs-file : %s", err)
	}

	if _, err := c.TaskRunClient.Create(getGCSTaskRun(namespace, taskRunName, resName)); err != nil {
		t.Fatalf("Failed to create TaskRun %s: %s", taskRunName, err)
	}

	logger.Infof("Waiting for TaskRun %s in namespace %s to complete", taskRunName, namespace)

	if err := WaitForTaskRunState(c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", taskRunName, err)
	}
	logger.Infof("TaskRun %s succeeded", taskRunName)
}

func getGCSStorageTask(namespace, secretName, secretKey string) *v1alpha1.Task {
	return tb.Task("gcs-file", namespace, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("gcsbucket", v1alpha1.PipelineResourceTypeStorage,
			tb.ResourceTargetPath("gcs-workspace"),
		)),
		tb.Step("read-gcs-bucket", "ubuntu", tb.Command("/bin/bash"),
			tb.Args("-c", "ls -la /workspace/gcs-workspace/rules_docker-master.zip"),
		),
		tb.Step("read-secret-env", "ubuntu", tb.Command("/bin/bash"),
			tb.Args("-c", "ls -la $CREDENTIALS"),
			tb.VolumeMount(fmt.Sprintf("volume-gcs-resource-%s", secretName),
				// this build should have volume with
				// name volume-(resource_name)-(secret_name) because of storage resource(gcs)
				fmt.Sprintf("/var/secret/%s", secretName),
			),
			tb.EnvVar("CREDENTIALS", fmt.Sprintf("/var/secret/%s/%s", secretName, secretKey)),
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

func getResources(namespace, name, secretName, secretKey string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(name, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("location", "gs://build-crd-tests/rules_docker-master.zip"),
		tb.PipelineResourceSpecParam("type", "gcs"),
		tb.PipelineResourceSpecSecretParam("GOOGLE_APPLICATION_CREDENTIALS", secretName, secretKey),
	))
}

func createGCSSecret(t *testing.T, logger *logging.BaseLogger, namespace, authFilePath string) *corev1.Secret {
	t.Helper()

	f, err := ioutil.ReadFile(authFilePath)
	if err != nil {
		t.Fatalf("Failed to read json key file %s at path %s", err, authFilePath)
	}

	data := map[string][]byte{filepath.Base(authFilePath): f}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "auth-secret",
		},
		Data: data,
	}
}

type GCPProject struct {
	GCPconfig `json:"core"`
}
type GCPconfig struct {
	Name string `json:"project"`
}
