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
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	repo := ensureDockerRepo(t)
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	hasSecretConfig, err := CreateGCPServiceAccountSecret(t, c.KubeClient, namespace, "kaniko-secret")
	if err != nil {
		t.Fatalf("Expected to create kaniko creds: %v", err)
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
	if _, err := c.TaskClient.Create(getTask(repo, namespace, hasSecretConfig)); err != nil {
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
	remoteDigest, err := getRemoteDigest(repo)
	if err != nil {
		t.Fatalf("Expected to get digest for remote image %s", repo)
	}
	if digest != remoteDigest {
		t.Fatalf("Expected local digest %s to match remote digest %s", digest, remoteDigest)
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

func getTask(repo, namespace string, withSecretConfig bool) *v1alpha1.Task {
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
		),
	}
	if withSecretConfig {
		stepOps = append(stepOps,
			tb.StepVolumeMount("kaniko-secret", "/secrets"),
			tb.StepEnvVar("GOOGLE_APPLICATION_CREDENTIALS", "/secrets/config.json"),
		)
		taskSpecOps = append(taskSpecOps, tb.TaskVolume("kaniko-secret", tb.VolumeSource(corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "kaniko-secret",
			},
		})))
	}
	step := tb.Step("kaniko", "gcr.io/kaniko-project/executor:v0.13.0", stepOps...)
	taskSpecOps = append(taskSpecOps, step)

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

func getRemoteDigest(image string) (string, error) {
	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return "", xerrors.Errorf("could not parse image reference %q: %w", image, err)
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", xerrors.Errorf("could not pull remote ref %s: %w", ref, err)
	}
	digest, err := img.Digest()
	if err != nil {
		return "", xerrors.Errorf("could not get digest for image %s: %w", img, err)
	}
	return digest.String(), nil
}
