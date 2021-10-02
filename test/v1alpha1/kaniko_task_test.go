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
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

const (
	kanikoTaskName          = "kanikotask"
	kanikoTaskRunName       = "kanikotask-run"
	kanikoGitResourceName   = "go-example-git"
	kanikoImageResourceName = "go-example-image"
	// This is a random revision chosen on 2020/10/09
	revision = "a310cc6d1cd449f95cedd23393de766fdc649651"
)

// TestTaskRun is an integration test that will verify a TaskRun using kaniko
func TestKanikoTaskRun(t *testing.T) {
	if skipRootUserTests {
		t.Skip("Skip test as skipRootUserTests set to true")
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, withRegistry)
	t.Parallel()

	repo := fmt.Sprintf("registry.%s:5000/kanikotasktest", namespace)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating Git PipelineResource %s", kanikoGitResourceName)
	if _, err := c.PipelineResourceClient.Create(ctx, getGitResource(namespace), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoGitResourceName, err)
	}

	t.Logf("Creating Image PipelineResource %s", repo)
	if _, err := c.PipelineResourceClient.Create(ctx, getImageResource(namespace, repo), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoGitResourceName, err)
	}

	t.Logf("Creating Task %s", kanikoTaskName)
	if _, err := c.TaskClient.Create(ctx, getTask(repo, namespace), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", kanikoTaskName, err)
	}

	t.Logf("Creating TaskRun %s", kanikoTaskRunName)
	if _, err := c.TaskRunClient.Create(ctx, getTaskRun(namespace), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", kanikoTaskRunName, err)
	}

	// Verify status of TaskRun (wait for it)

	if err := WaitForTaskRunState(ctx, c, kanikoTaskRunName, Succeed(kanikoTaskRunName), "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", kanikoTaskRunName, err)
	}

	tr, err := c.TaskRunClient.Get(ctx, kanikoTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	digest := ""
	commit := ""
	url := ""
	for _, rr := range tr.Status.ResourcesResult {
		switch rr.Key {
		case "digest":
			digest = rr.Value
		case "commit":
			commit = rr.Value
		case "url":
			url = rr.Value
		}
	}
	if digest == "" {
		t.Errorf("Digest not found in TaskRun.Status: %v", tr.Status)
	}
	if commit == "" {
		t.Errorf("Commit not found in TaskRun.Status: %v", tr.Status)
	}

	if url == "" {
		t.Errorf("URL not found in TaskRun.Status: %v", tr.Status)
	}

	if revision != commit {
		t.Fatalf("Expected remote commit to match local revision: %s, %s", commit, revision)
	}

	// match the local digest, which is first capture group against the remote image
	remoteDigest, err := getRemoteDigest(ctx, t, c, namespace, repo)
	if err != nil {
		t.Fatalf("Expected to get digest for remote image %s: %v", repo, err)
	}
	if d := cmp.Diff(digest, remoteDigest); d != "" {
		t.Fatalf("Expected local digest %s to match remote digest %s: %s", digest, remoteDigest, d)
	}
}

func getGitResource(namespace string) *v1alpha1.PipelineResource {
	return &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: kanikoGitResourceName,
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.ResourceParam{
				{
					Name:  "Url",
					Value: "https://github.com/GoogleContainerTools/kaniko",
				},
				{
					Name:  "Revision",
					Value: revision,
				},
			},
		},
	}
}

func getImageResource(namespace, repo string) *v1alpha1.PipelineResource {
	return &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: kanikoImageResourceName,
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeImage,
			Params: []v1alpha1.ResourceParam{{
				Name:  "url",
				Value: repo,
			}},
		},
	}
}

func getTask(repo, namespace string) *v1alpha1.Task {
	root := int64(0)
	return &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: kanikoTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name: "gitsource",
						Type: v1alpha1.PipelineResourceTypeGit,
					},
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					ResourceDeclaration: v1alpha1.ResourceDeclaration{
						Name: "builtImage",
						Type: v1alpha1.PipelineResourceTypeImage,
					},
				}},
			},
			TaskSpec: v1beta1.TaskSpec{
				Steps: []v1alpha1.Step{{
					Container: corev1.Container{
						Image: "gcr.io/kaniko-project/executor:v1.3.0",
						Name:  "kaniko",
						Args: []string{
							"--dockerfile=/workspace/gitsource/integration/dockerfiles/Dockerfile_test_label",
							fmt.Sprintf("--destination=%s", repo),
							"--context=/workspace/gitsource",
							"--oci-layout-path=/workspace/output/builtImage",
							"--insecure",
							"--insecure-pull",
							"--insecure-registry=registry." + namespace + ":5000/",
						},
						SecurityContext: &corev1.SecurityContext{RunAsUser: &root},
					},
				}},
				Sidecars: []v1alpha1.Sidecar{{
					Container: corev1.Container{
						Image: "registry",
						Name:  "registry",
					},
				}},
			},
		},
	}
}

func getTaskRun(namespace string) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kanikoTaskRunName,
			Namespace: namespace,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name: kanikoTaskName,
			},
			Timeout: &metav1.Duration{Duration: 2 * time.Minute},
			Inputs: &v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
						Name: "gitsource",
						ResourceRef: &v1alpha1.PipelineResourceRef{
							Name: kanikoGitResourceName,
						},
					},
				}},
			},
			Outputs: &v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					PipelineResourceBinding: v1alpha1.PipelineResourceBinding{
						Name: "builtImage",
						ResourceRef: &v1alpha1.PipelineResourceRef{
							Name: kanikoImageResourceName,
						},
					},
				}},
			},
		},
	}
}

// getRemoteDigest starts a pod to query the registry from the namespace itself, using skopeo (and jq).
// The reason we have to do that is because the image is pushed on a local registry that is not exposed
// to the "outside" of the test, this means it can be query by the test itself. It can only be query from
// a pod in the namespace. skopeo is able to do that query and we use jq to extract the digest from its
// output. The image used for this pod is build in the tektoncd/plumbing repository.
func getRemoteDigest(ctx context.Context, t *testing.T, c *clients, namespace, image string) (string, error) {
	t.Helper()
	podName := "skopeo-jq"
	if _, err := c.KubeClient.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
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
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create the skopeo-jq pod: %v", err)
	}
	if err := WaitForPodState(ctx, c, podName, namespace, func(pod *corev1.Pod) (bool, error) {
		return pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed", nil
	}, "PodContainersTerminated"); err != nil {
		t.Fatalf("Error waiting for Pod %q to terminate: %v", podName, err)
	}
	logs, err := getContainerLogsFromPod(ctx, c.KubeClient, podName, "skopeo", namespace)
	if err != nil {
		t.Fatalf("Could not get logs for pod %s: %s", podName, err)
	}
	return strings.TrimSpace(strings.ReplaceAll(logs, "\"", "")), nil
}
