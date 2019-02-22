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
	"testing"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helloworldResourceName    = "helloworldgit"
	addFileTaskName           = "add-file-to-resource-task"
	readFileTaskName          = "read-new-file-task"
	bucketTestPipelineName    = "bucket-test-pipeline"
	bucketTestPipelineRunName = "bucket-test-pipeline-run"
	systemNamespace           = "tekton-pipelines"
	bucketSecretName          = "bucket-secret"
	bucketSecretKey           = "bucket-secret-key"
)

// TestStorageBucketPipelineRun is an integration test that will verify a pipeline
// can use a bucket for temporary storage of artifacts shared between tasks
func TestStorageBucketPipelineRun(t *testing.T) {
	configFilePath := os.Getenv("GCP_SERVICE_ACCOUNT_KEY_PATH")
	if configFilePath == "" {
		t.Skip("GCP_SERVICE_ACCOUNT_KEY_PATH variable is not set.")
	}
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	bucketName := fmt.Sprintf("build-pipeline-test-%s-%d", namespace, time.Now().Unix())

	logger.Infof("Creating Secret %s", bucketSecretName)
	if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(getBucketSecret(t, logger, configFilePath, namespace)); err != nil {
		t.Fatalf("Failed to create Secret `%s`: %s", bucketSecretName, err)
	}
	defer deleteBucketSecret(c, t, logger, namespace)

	logger.Infof("Creating GCS bucket %s", bucketName)
	createbuckettask := tb.Task("createbuckettask", namespace, tb.TaskSpec(
		tb.TaskVolume("bucket-secret-volume", tb.VolumeSource(corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: bucketSecretName,
			},
		})),
		tb.Step("step1", "google/cloud-sdk:alpine",
			tb.Command("/bin/bash"),
			tb.Args("-c", fmt.Sprintf("gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key && gsutil mb gs://%s", bucketName)),
			tb.VolumeMount("bucket-secret-volume", fmt.Sprintf("/var/secret/%s", bucketSecretName)),
			tb.EnvVar("CREDENTIALS", fmt.Sprintf("/var/secret/%s/%s", bucketSecretName, bucketSecretKey)),
		),
	),
	)

	logger.Infof("Creating Task %s", "createbuckettask")
	if _, err := c.TaskClient.Create(createbuckettask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "createbuckettask", err)
	}

	createbuckettaskrun := tb.TaskRun("createbuckettaskrun", namespace,
		tb.TaskRunSpec(tb.TaskRunTaskRef("createbuckettask")))

	logger.Infof("Creating TaskRun %s", "createbuckettaskrun")
	if _, err := c.TaskRunClient.Create(createbuckettaskrun); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", "createbuckettaskrun", err)
	}

	if err := WaitForTaskRunState(c, "createbuckettaskrun", TaskRunSucceed("createbuckettaskrun"), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", "createbuckettaskrun", err)
	}

	defer runTaskToDeleteBucket(c, t, logger, namespace, bucketName, bucketSecretName, bucketSecretKey)

	originalConfigMap, err := c.KubeClient.Kube.CoreV1().ConfigMaps(systemNamespace).Get(v1alpha1.BucketConfigName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", v1alpha1.BucketConfigName, err)
	}
	originalConfigMapData := originalConfigMap.Data

	logger.Infof("Creating ConfigMap %s", v1alpha1.BucketConfigName)
	configMapData := map[string]string{
		v1alpha1.BucketLocationKey:              fmt.Sprintf("gs://%s", bucketName),
		v1alpha1.BucketServiceAccountSecretName: bucketSecretName,
		v1alpha1.BucketServiceAccountSecretKey:  bucketSecretKey,
	}
	c.KubeClient.UpdateConfigMap(systemNamespace, v1alpha1.BucketConfigName, configMapData)
	defer resetConfigMap(c, systemNamespace, v1alpha1.BucketConfigName, originalConfigMapData)

	logger.Infof("Creating Git PipelineResource %s", helloworldResourceName)
	helloworldResource := tb.PipelineResource(helloworldResourceName, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/pivotal-nader-ziada/gohelloworld"),
		tb.PipelineResourceSpecParam("Revision", "master"),
	),
	)
	if _, err := c.PipelineResourceClient.Create(helloworldResource); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", helloworldResourceName, err)
	}

	logger.Infof("Creating Task %s", addFileTaskName)
	addFileTask := tb.Task(addFileTaskName, namespace, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource(helloworldResourceName, v1alpha1.PipelineResourceTypeGit)),
		tb.TaskOutputs(tb.OutputsResource(helloworldResourceName, v1alpha1.PipelineResourceTypeGit)),
		tb.Step("addfile", "ubuntu", tb.Command("/bin/bash"),
			tb.Args("-c", "echo stuff > /workspace/helloworldgit/newfile"),
		),
	))
	if _, err := c.TaskClient.Create(addFileTask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", addFileTaskName, err)
	}

	logger.Infof("Creating Task %s", readFileTaskName)
	readFileTask := tb.Task(readFileTaskName, namespace, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource(helloworldResourceName, v1alpha1.PipelineResourceTypeGit)),
		tb.Step("readfile", "ubuntu", tb.Command("/bin/bash"),
			tb.Args("-c", "cat /workspace/helloworldgit/newfile"),
		),
	))
	if _, err := c.TaskClient.Create(readFileTask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", readFileTaskName, err)
	}

	logger.Infof("Creating Pipeline %s", bucketTestPipelineName)
	bucketTestPipeline := tb.Pipeline(bucketTestPipelineName, namespace, tb.PipelineSpec(
		tb.PipelineDeclaredResource("source-repo", "git"),
		tb.PipelineTask("addfile", addFileTaskName,
			tb.PipelineTaskInputResource("helloworldgit", "source-repo"),
			tb.PipelineTaskOutputResource("helloworldgit", "source-repo"),
		),
		tb.PipelineTask("readfile", readFileTaskName,
			tb.PipelineTaskInputResource("helloworldgit", "source-repo", tb.From("addfile")),
		),
	))
	if _, err := c.PipelineClient.Create(bucketTestPipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", bucketTestPipelineName, err)
	}

	logger.Infof("Creating PipelineRun %s", bucketTestPipelineRunName)
	bucketTestPipelineRun := tb.PipelineRun(bucketTestPipelineRunName, namespace, tb.PipelineRunSpec(
		bucketTestPipelineName,
		tb.PipelineRunResourceBinding("source-repo", tb.PipelineResourceBindingRef(helloworldResourceName)),
	))
	if _, err := c.PipelineRunClient.Create(bucketTestPipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", bucketTestPipelineRunName, err)
	}

	// Verify status of PipelineRun (wait for it)
	if err := WaitForPipelineRunState(c, bucketTestPipelineRunName, timeout, PipelineRunSucceed(bucketTestPipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", bucketTestPipelineRunName, err)
		taskruns, err := c.TaskRunClient.List(metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting TaskRun list for PipelineRun %s %s", bucketTestPipelineRunName, err)
		}
		for _, tr := range taskruns.Items {
			if tr.Status.PodName != "" {
				CollectBuildLogs(c, tr.Status.PodName, namespace, logger)
			}
		}
		t.Fatalf("PipelineRun execution failed")
	}
}

func getBucketSecret(t *testing.T, logger *logging.BaseLogger, configFilePath, namespace string) *corev1.Secret {
	t.Helper()
	f, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		t.Fatalf("Failed to read json key file %s at path %s", err, configFilePath)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      bucketSecretName,
		},
		StringData: map[string]string{
			bucketSecretKey: string(f),
		},
	}
}

func deleteBucketSecret(c *clients, t *testing.T, logger *logging.BaseLogger, namespace string) {
	if err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Delete(bucketSecretName, &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete Secret `%s`: %s", bucketSecretName, err)
	}
}

func resetConfigMap(c *clients, namespace, configName string, values map[string]string) error {
	return c.KubeClient.UpdateConfigMap(namespace, configName, values)
}

func runTaskToDeleteBucket(c *clients, t *testing.T, logger *logging.BaseLogger, namespace, bucketName, bucketSecretName, bucketSecretKey string) {
	deletelbuckettask := tb.Task("deletelbuckettask", namespace, tb.TaskSpec(
		tb.TaskVolume("bucket-secret-volume", tb.VolumeSource(corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: bucketSecretName,
			},
		})),
		tb.Step("step1", "google/cloud-sdk:alpine",
			tb.Command("/bin/bash"),
			tb.Args("-c", fmt.Sprintf("gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key && gsutil rm -r gs://%s", bucketName)),
			tb.VolumeMount("bucket-secret-volume", fmt.Sprintf("/var/secret/%s", bucketSecretName)),
			tb.EnvVar("CREDENTIALS", fmt.Sprintf("/var/secret/%s/%s", bucketSecretName, bucketSecretKey)),
		),
	),
	)

	logger.Infof("Creating Task %s", "deletelbuckettask")
	if _, err := c.TaskClient.Create(deletelbuckettask); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "deletelbuckettask", err)
	}

	deletelbuckettaskrun := tb.TaskRun("deletelbuckettaskrun", namespace,
		tb.TaskRunSpec(tb.TaskRunTaskRef("deletelbuckettask")))

	logger.Infof("Creating TaskRun %s", "deletelbuckettaskrun")
	if _, err := c.TaskRunClient.Create(deletelbuckettaskrun); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", "deletelbuckettaskrun", err)
	}

	if err := WaitForTaskRunState(c, "deletelbuckettaskrun", TaskRunSucceed("deletelbuckettaskrun"), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", "deletelbuckettaskrun", err)
	}
}
