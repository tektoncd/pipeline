// +build e2e

/*
Copyright 2019 The Tekton Authors

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
	"io/ioutil"
	"os"
	"testing"
	"time"

	tb "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	knativetest "knative.dev/pkg/test"
)

const (
	helloworldResourceName    = "helloworldgit"
	addFileTaskName           = "add-file-to-resource-task"
	runFileTaskName           = "run-new-file-task"
	bucketTestPipelineName    = "bucket-test-pipeline"
	bucketTestPipelineRunName = "bucket-test-pipeline-run"
	systemNamespace           = "tekton-pipelines"
	bucketSecretName          = "bucket-secret"
	bucketSecretKey           = "bucket-secret-key"
)

// TestStorageBucketPipelineRun is an integration test that will verify a pipeline
// can use a bucket for temporary storage of artifacts shared between tasks
func TestStorageBucketPipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	configFilePath := os.Getenv("GCP_SERVICE_ACCOUNT_KEY_PATH")
	if configFilePath == "" {
		t.Skip("GCP_SERVICE_ACCOUNT_KEY_PATH variable is not set.")
	}
	c, namespace := setup(ctx, t)
	// Bucket tests can't run in parallel without causing issues with other tests.

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	bucketName := fmt.Sprintf("build-pipeline-test-%s-%d", namespace, time.Now().Unix())

	t.Logf("Creating Secret %s", bucketSecretName)
	if _, err := c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, getBucketSecret(t, configFilePath, namespace), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Secret %q: %v", bucketSecretName, err)
	}
	defer deleteBucketSecret(ctx, c, t, namespace)

	t.Logf("Creating GCS bucket %s", bucketName)
	createbuckettask := tb.Task("createbuckettask", tb.TaskSpec(
		tb.TaskVolume("bucket-secret-volume", tb.VolumeSource(corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: bucketSecretName,
			},
		})),
		tb.Step("gcr.io/google.com/cloudsdktool/cloud-sdk:alpine", tb.StepName("step1"),
			tb.StepCommand("/bin/bash"),
			tb.StepArgs("-c", fmt.Sprintf("gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key && gsutil mb gs://%s", bucketName)),
			tb.StepVolumeMount("bucket-secret-volume", fmt.Sprintf("/var/secret/%s", bucketSecretName)),
			tb.StepEnvVar("CREDENTIALS", fmt.Sprintf("/var/secret/%s/%s", bucketSecretName, bucketSecretKey)),
		),
	),
	)

	t.Logf("Creating Task %s", "createbuckettask")
	if _, err := c.TaskClient.Create(ctx, createbuckettask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "createbuckettask", err)
	}

	createbuckettaskrun := tb.TaskRun("createbuckettaskrun",
		tb.TaskRunSpec(tb.TaskRunTaskRef("createbuckettask")))

	t.Logf("Creating TaskRun %s", "createbuckettaskrun")
	if _, err := c.TaskRunClient.Create(ctx, createbuckettaskrun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", "createbuckettaskrun", err)
	}

	if err := WaitForTaskRunState(ctx, c, "createbuckettaskrun", TaskRunSucceed("createbuckettaskrun"), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", "createbuckettaskrun", err)
	}

	defer runTaskToDeleteBucket(ctx, c, t, namespace, bucketName, bucketSecretName, bucketSecretKey)

	originalConfigMap, err := c.KubeClient.CoreV1().ConfigMaps(systemNamespace).Get(ctx, config.GetArtifactBucketConfigName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetArtifactBucketConfigName(), err)
	}
	originalConfigMapData := originalConfigMap.Data

	t.Logf("Creating ConfigMap %s", config.GetArtifactBucketConfigName())
	configMapData := map[string]string{
		config.BucketLocationKey:                 fmt.Sprintf("gs://%s", bucketName),
		config.BucketServiceAccountSecretNameKey: bucketSecretName,
		config.BucketServiceAccountSecretKeyKey:  bucketSecretKey,
	}
	if err := updateConfigMap(ctx, c.KubeClient, systemNamespace, config.GetArtifactBucketConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
	defer resetConfigMap(ctx, t, c, systemNamespace, config.GetArtifactBucketConfigName(), originalConfigMapData)

	t.Logf("Creating Git PipelineResource %s", helloworldResourceName)
	helloworldResource := tb.PipelineResource(helloworldResourceName, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/pivotal-nader-ziada/gohelloworld"),
		tb.PipelineResourceSpecParam("Revision", "master"),
	),
	)
	if _, err := c.PipelineResourceClient.Create(ctx, helloworldResource, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", helloworldResourceName, err)
	}

	t.Logf("Creating Task %s", addFileTaskName)
	addFileTask := tb.Task(addFileTaskName, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource(helloworldResourceName, v1alpha1.PipelineResourceTypeGit)),
		tb.TaskOutputs(tb.OutputsResource(helloworldResourceName, v1alpha1.PipelineResourceTypeGit)),
		tb.Step("ubuntu", tb.StepName("addfile"), tb.StepCommand("/bin/bash"),
			tb.StepArgs("-c", "'#!/bin/bash\necho hello' > /workspace/helloworldgit/newfile"),
		),
		tb.Step("ubuntu", tb.StepName("make-executable"), tb.StepCommand("chmod"),
			tb.StepArgs("+x", "/workspace/helloworldgit/newfile")),
	))
	if _, err := c.TaskClient.Create(ctx, addFileTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", addFileTaskName, err)
	}

	t.Logf("Creating Task %s", runFileTaskName)
	readFileTask := tb.Task(runFileTaskName, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource(helloworldResourceName, v1alpha1.PipelineResourceTypeGit)),
		tb.Step("ubuntu", tb.StepName("runfile"), tb.StepCommand("/workspace/helloworld/newfile")),
	))
	if _, err := c.TaskClient.Create(ctx, readFileTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", runFileTaskName, err)
	}

	t.Logf("Creating Pipeline %s", bucketTestPipelineName)
	bucketTestPipeline := tb.Pipeline(bucketTestPipelineName, tb.PipelineSpec(
		tb.PipelineDeclaredResource("source-repo", "git"),
		tb.PipelineTask("addfile", addFileTaskName,
			tb.PipelineTaskInputResource("helloworldgit", "source-repo"),
			tb.PipelineTaskOutputResource("helloworldgit", "source-repo"),
		),
		tb.PipelineTask("runfile", runFileTaskName,
			tb.PipelineTaskInputResource("helloworldgit", "source-repo", tb.From("addfile")),
		),
	))
	if _, err := c.PipelineClient.Create(ctx, bucketTestPipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", bucketTestPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", bucketTestPipelineRunName)
	bucketTestPipelineRun := tb.PipelineRun(bucketTestPipelineRunName, tb.PipelineRunSpec(
		bucketTestPipelineName,
		tb.PipelineRunResourceBinding("source-repo", tb.PipelineResourceBindingRef(helloworldResourceName)),
	))
	if _, err := c.PipelineRunClient.Create(ctx, bucketTestPipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", bucketTestPipelineRunName, err)
	}

	// Verify status of PipelineRun (wait for it)
	if err := WaitForPipelineRunState(ctx, c, bucketTestPipelineRunName, timeout, PipelineRunSucceed(bucketTestPipelineRunName), "PipelineRunCompleted"); err != nil {
		t.Errorf("Error waiting for PipelineRun %s to finish: %s", bucketTestPipelineRunName, err)
		t.Fatalf("PipelineRun execution failed")
	}
}

// updateConfigMap updates the config map for specified @name with values. We can't use the one from knativetest because
// it assumes that Data is already a non-nil map, and by default, it isn't!
func updateConfigMap(ctx context.Context, client kubernetes.Interface, name string, configName string, values map[string]string) error {
	configMap, err := client.CoreV1().ConfigMaps(name).Get(ctx, configName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	for key, value := range values {
		configMap.Data[key] = value
	}

	_, err = client.CoreV1().ConfigMaps(name).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}

func getBucketSecret(t *testing.T, configFilePath, namespace string) *corev1.Secret {
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

func deleteBucketSecret(ctx context.Context, c *clients, t *testing.T, namespace string) {
	if err := c.KubeClient.CoreV1().Secrets(namespace).Delete(ctx, bucketSecretName, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete Secret `%s`: %s", bucketSecretName, err)
	}
}

func resetConfigMap(ctx context.Context, t *testing.T, c *clients, namespace, configName string, values map[string]string) {
	if err := updateConfigMap(ctx, c.KubeClient, namespace, configName, values); err != nil {
		t.Log(err)
	}
}

func runTaskToDeleteBucket(ctx context.Context, c *clients, t *testing.T, namespace, bucketName, bucketSecretName, bucketSecretKey string) {
	deletelbuckettask := tb.Task("deletelbuckettask", tb.TaskSpec(
		tb.TaskVolume("bucket-secret-volume", tb.VolumeSource(corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: bucketSecretName,
			},
		})),
		tb.Step("gcr.io/google.com/cloudsdktool/cloud-sdk:alpine", tb.StepName("step1"),
			tb.StepCommand("/bin/bash"),
			tb.StepArgs("-c", fmt.Sprintf("gcloud auth activate-service-account --key-file /var/secret/bucket-secret/bucket-secret-key && gsutil rm -r gs://%s", bucketName)),
			tb.StepVolumeMount("bucket-secret-volume", fmt.Sprintf("/var/secret/%s", bucketSecretName)),
			tb.StepEnvVar("CREDENTIALS", fmt.Sprintf("/var/secret/%s/%s", bucketSecretName, bucketSecretKey)),
		),
	),
	)

	t.Logf("Creating Task %s", "deletelbuckettask")
	if _, err := c.TaskClient.Create(ctx, deletelbuckettask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", "deletelbuckettask", err)
	}

	deletelbuckettaskrun := tb.TaskRun("deletelbuckettaskrun",
		tb.TaskRunSpec(tb.TaskRunTaskRef("deletelbuckettask")))

	t.Logf("Creating TaskRun %s", "deletelbuckettaskrun")
	if _, err := c.TaskRunClient.Create(ctx, deletelbuckettaskrun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", "deletelbuckettaskrun", err)
	}

	if err := WaitForTaskRunState(ctx, c, "deletelbuckettaskrun", TaskRunSucceed("deletelbuckettaskrun"), "TaskRunSuccess"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", "deletelbuckettaskrun", err)
	}
}
