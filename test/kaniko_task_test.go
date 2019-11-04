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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	knativetest "knative.dev/pkg/test"
)

const (
	kanikoTaskName          = "kanikotask"
	kanikoTaskRunName       = "kanikotask-run"
	kanikoGitResourceName   = "go-example-git"
	kanikoImageResourceName = "go-example-image"
	// This is a random revision chosen on 10/11/2019
	revision = "1c9d566ecd13535f93789595740f20932f655905"
)

// TestTaskRun is an integration test that will verify a TaskRun using kaniko
func TestKanikoTaskRun(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	repo := fmt.Sprintf("registry.%s:5000/kanikotasktest", namespace)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	if _, err := c.KubeClient.Kube.AppsV1().Deployments(namespace).Create(getRegistryDeployment(namespace)); err != nil {
		t.Fatalf("Failed to create the local registry deployment: %v", err)
	}
	service := getRegistryService(namespace)
	if _, err := c.KubeClient.Kube.CoreV1().Services(namespace).Create(service); err != nil {
		t.Fatalf("Failed to create the local registry service: %v", err)
	}
	set := labels.Set(service.Spec.Selector)
	if pods, err := c.KubeClient.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: set.AsSelector().String()}); err != nil {
		t.Fatalf("Failed to list Pods of service[%s] error:%v", service.GetName(), err)
	} else {
		if len(pods.Items) != 1 {
			t.Fatalf("Only 1 pod for service %s should be running: %v", service, pods.Items)
		}

		if err := WaitForPodState(c, pods.Items[0].Name, namespace, func(pod *corev1.Pod) (bool, error) {
			return pod.Status.Phase == "Running", nil
		}, "PodContainersRunning"); err != nil {
			t.Fatalf("Error waiting for Pod %q to run: %v", pods.Items[0].Name, err)
		}
	}

	t.Logf("Creating Git PipelineResource %s", kanikoGitResourceName)
	if _, err := c.PipelineResourceClient.Create(getGitResource(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoGitResourceName, err)
	}

	t.Logf("Creating Image PipelineResource %s", repo)
	if _, err := c.PipelineResourceClient.Create(getImageResource(namespace, repo)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoGitResourceName, err)
	}

	t.Logf("Creating Task %s", kanikoTaskName)
	if _, err := c.TaskClient.Create(getTask(repo, namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", kanikoTaskName, err)
	}

	t.Logf("Creating TaskRun %s", kanikoTaskRunName)
	if _, err := c.TaskRunClient.Create(getTaskRun(namespace)); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", kanikoTaskRunName, err)
	}

	// Verify status of TaskRun (wait for it)

	if err := WaitForTaskRunState(c, kanikoTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		return TaskRunSucceed(kanikoTaskRunName)(tr)
	}, "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", kanikoTaskRunName, err)
	}

	tr, err := c.TaskRunClient.Get(kanikoTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	digest := ""
	commit := ""
	for _, rr := range tr.Status.ResourcesResult {
		switch rr.Key {
		case "digest":
			digest = rr.Value
		case "commit":
			commit = rr.Value
		}
	}
	if digest == "" {
		t.Errorf("Digest not found in TaskRun.Status: %v", tr.Status)
	}
	if commit == "" {
		t.Errorf("Commit not found in TaskRun.Status: %v", tr.Status)
	}

	if revision != commit {
		t.Fatalf("Expected remote commit to match local revision: %s, %s", commit, revision)
	}

	// match the local digest, which is first capture group against the remote image
	remoteDigest, err := getRemoteDigest(t, c, namespace, repo)
	if err != nil {
		t.Fatalf("Expected to get digest for remote image %s: %v", repo, err)
	}
	if d := cmp.Diff(digest, remoteDigest); d != "" {
		t.Fatalf("Expected local digest %s to match remote digest %s: %s", digest, remoteDigest, d)
	}
}

func getRegistryDeployment(namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "registry",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "registry",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "registry",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "registry",
						Image: "registry",
					}},
				},
			},
		},
	}
}

func getRegistryService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "registry",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port: 5000,
			}},
			Selector: map[string]string{
				"app": "registry",
			},
		},
	}
}

func getGitResource(namespace string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(kanikoGitResourceName, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/GoogleContainerTools/kaniko"),
		tb.PipelineResourceSpecParam("Revision", revision),
	))
}

func getImageResource(namespace, repo string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(kanikoImageResourceName, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeImage,
		tb.PipelineResourceSpecParam("url", repo),
	))
}

func getTask(repo, namespace string) *v1alpha1.Task {
	taskSpecOps := []tb.TaskSpecOp{
		tb.TaskInputs(tb.InputsResource("gitsource", v1alpha1.PipelineResourceTypeGit)),
		tb.TaskOutputs(tb.OutputsResource("builtImage", v1alpha1.PipelineResourceTypeImage)),
	}
	stepOps := []tb.StepOp{
		tb.StepArgs(
			"--dockerfile=/workspace/gitsource/integration/dockerfiles/Dockerfile_test_label",
			fmt.Sprintf("--destination=%s", repo),
			"--context=/workspace/gitsource",
			"--oci-layout-path=/workspace/output/builtImage",
			"--insecure",
			"--insecure-pull",
			"--insecure-registry=registry"+namespace+":5000/",
		),
	}
	step := tb.Step("kaniko", "gcr.io/kaniko-project/executor:v0.13.0", stepOps...)
	taskSpecOps = append(taskSpecOps, step)
	sidecar := tb.Sidecar("registry", "registry")
	taskSpecOps = append(taskSpecOps, sidecar)

	return tb.Task(kanikoTaskName, namespace, tb.TaskSpec(taskSpecOps...))
}

func getTaskRun(namespace string) *v1alpha1.TaskRun {
	return tb.TaskRun(kanikoTaskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(kanikoTaskName),
		tb.TaskRunTimeout(2*time.Minute),
		tb.TaskRunInputs(tb.TaskRunInputsResource("gitsource", tb.TaskResourceBindingRef(kanikoGitResourceName))),
		tb.TaskRunOutputs(tb.TaskRunOutputsResource("builtImage", tb.TaskResourceBindingRef(kanikoImageResourceName))),
	))
}

func getRemoteDigest(t *testing.T, c *clients, namespace, image string) (string, error) {
	t.Helper()
	podName := "skopeo-jq"
	if _, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "skopeo",
				Image:   "gcr.io/tekton-releases/dogfooding/skopeo:latest",
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"skopeo inspect --tls-verify=false docker://" + image + ":latest| jq '.Digest'"},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}); err != nil {
		t.Fatalf("Failed to create the local registry service: %v", err)
	}
	if err := WaitForPodState(c, podName, namespace, func(pod *corev1.Pod) (bool, error) {
		return pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed", nil
	}, "PodContainersTerminated"); err != nil {
		t.Fatalf("Error waiting for Pod %q to terminate: %v", podName, err)
	}
	logs, err := getContainerLogsFromPod(c.KubeClient.Kube, podName, "skopeo", namespace)
	if err != nil {
		t.Fatalf("Could not get logs for pod %s: %s", podName, err)
	}
	return strings.TrimSpace(strings.ReplaceAll(logs, "\"", "")), nil
}
