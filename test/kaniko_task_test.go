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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

const (
	kanikoTaskName     = "kanikotask"
	kanikoTaskRunName  = "kanikotask-run"
	kanikoResourceName = "go-example-git"
	kanikoBuildOutput  = "Build successful"
)

func getGitResource(namespace string) *v1alpha1.PipelineResource {
	return &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kanikoResourceName,
			Namespace: namespace,
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.Param{
				v1alpha1.Param{
					Name:  "Url",
					Value: "https://github.com/pivotal-nader-ziada/gohelloworld",
				},
			},
		},
	}
}

func getTask(namespace string, t *testing.T) *v1alpha1.Task {
	// according to knative/test-infra readme (https://github.com/knative/test-infra/blob/13055d769cc5e1756e605fcb3bcc1c25376699f1/scripts/README.md)
	// the KO_DOCKER_REPO will be set with according to the porject where the cluster is created
	// it is used here to dunamically get the docker registery to push the image to
	dockerRepo := os.Getenv("KO_DOCKER_REPO")
	if dockerRepo == "" {
		t.Fatalf("KO_DOCKER_REPO env variable is required")
	}

	return &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      kanikoTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					v1alpha1.TaskResource{
						Name: kanikoResourceName,
						Type: v1alpha1.PipelineResourceTypeGit,
					},
				},
			},
			BuildSpec: &buildv1alpha1.BuildSpec{
				Timeout: &metav1.Duration{Duration: 2 * time.Minute},
				Steps: []corev1.Container{{
					Name:  "kaniko",
					Image: "gcr.io/kaniko-project/executor",
					Args: []string{"--dockerfile=/workspace/Dockerfile",
						fmt.Sprintf("--destination=%s/kanikotasktest", dockerRepo),
					},
				}},
			},
		},
	}
}

func getTaskRun(namespace string) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      kanikoTaskRunName,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: kanikoTaskName,
			},
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.PipelineResourceVersion{
					v1alpha1.PipelineResourceVersion{
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: kanikoResourceName,
						},
						Version: "master",
					},
				},
			},
		},
	}
}

// TestTaskRun is an integration test that will verify a TaskRun using kaniko
func TestKanikoTaskRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(logger, c.KubeClient, namespace) }, logger)
	defer tearDown(logger, c.KubeClient, namespace)

	logger.Infof("Creating Git PipelineResource %s", kanikoResourceName)
	if _, err := c.PipelineResourceClient.Create(getGitResource(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoResourceName, err)
	}

	logger.Infof("Creating Task %s", kanikoTaskName)
	if _, err := c.TaskClient.Create(getTask(namespace, t)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", kanikoTaskName, err)
	}

	logger.Infof("Creating TaskRun %s", kanikoTaskRunName)
	if _, err := c.TaskRunClient.Create(getTaskRun(namespace)); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", kanikoTaskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	if err := WaitForTaskRunState(c, kanikoTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipeline run %s failed!", hwPipelineRunName)
			}
		}
		return false, nil
	}, "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", kanikoTaskRunName, err)
	}

	// The Build created by the TaskRun will have the same name
	b, err := c.BuildClient.Get(kanikoTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Expected there to be a Build with the same name as TaskRun %s but got error: %s", kanikoTaskRunName, err)
	}
	cluster := b.Status.Cluster
	if cluster == nil || cluster.PodName == "" {
		t.Fatalf("Expected build status to have a podname but it didn't!")
	}
	podName := cluster.PodName
	pods := c.KubeClient.Kube.CoreV1().Pods(namespace)
	t.Logf("Retrieved pods for podname %s: %s\n", podName, pods)

	req := pods.GetLogs(podName, &corev1.PodLogOptions{})
	readCloser, err := req.Stream()
	if err != nil {
		t.Fatalf("Failed to open stream to read: %v", err)
	}
	defer readCloser.Close()
	var buf bytes.Buffer
	out := bufio.NewWriter(&buf)
	_, err = io.Copy(out, readCloser)
	if !strings.Contains(buf.String(), kanikoBuildOutput) {
		t.Fatalf("Expected output %s from pod %s but got %s", kanikoBuildOutput, podName, buf.String())
	}

}
