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
	"regexp"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
)

const (
	kanikoTaskName     = "kanikotask"
	kanikoTaskRunName  = "kanikotask-run"
	kanikoResourceName = "go-example-git"
	kanikoBuildOutput  = "Build successful"
)

func getGitResource(namespace string) *v1alpha1.PipelineResource {
	return tb.PipelineResource(kanikoResourceName, namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/pivotal-nader-ziada/gohelloworld"),
	))
}

func getDockerRepo() (string, error) {
	// according to knative/test-infra readme (https://github.com/knative/test-infra/blob/13055d769cc5e1756e605fcb3bcc1c25376699f1/scripts/README.md)
	// the KO_DOCKER_REPO will be set with according to the project where the cluster is created
	// it is used here to dynamically get the docker registry to push the image to
	dockerRepo := os.Getenv("KO_DOCKER_REPO")
	if dockerRepo == "" {
		return "", fmt.Errorf("KO_DOCKER_REPO env variable is required")
	}
	return fmt.Sprintf("%s/kanikotasktest", dockerRepo), nil
}

func createSecret(c *knativetest.KubeClient, namespace string) (bool, error) {
	// when running e2e in cluster, this will not be set so just hop out early
	file := os.Getenv("GCP_SERVICE_ACCOUNT_KEY_PATH")
	if file == "" {
		return false, nil
	}

	sec := &corev1.Secret{}
	sec.Name = "kaniko-secret"
	sec.Namespace = namespace

	bs, err := ioutil.ReadFile(file)
	if err != nil {
		return false, fmt.Errorf("couldn't read kaniko secret json: %v", err)
	}

	sec.Data = map[string][]byte{
		"config.json": bs,
	}
	_, err = c.Kube.CoreV1().Secrets(namespace).Create(sec)
	return true, err
}

func getTask(repo, namespace string, withSecretConfig bool) *v1alpha1.Task {
	taskSpecOps := []tb.TaskSpecOp{
		tb.TaskInputs(tb.InputsResource("gitsource", v1alpha1.PipelineResourceTypeGit)),
	}
	stepOps := []tb.ContainerOp{
		tb.Args(
			"--dockerfile=/workspace/gitsource/Dockerfile",
			fmt.Sprintf("--destination=%s", repo),
			"--context=/workspace/gitsource",
		),
	}
	if withSecretConfig {
		stepOps = append(stepOps,
			tb.VolumeMount("kaniko-secret", "/secrets"),
			tb.EnvVar("GOOGLE_APPLICATION_CREDENTIALS", "/secrets/config.json"),
		)
		taskSpecOps = append(taskSpecOps, tb.TaskVolume("kaniko-secret", tb.VolumeSource(corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "kaniko-secret",
			},
		})))
	}
	step := tb.Step("kaniko", "gcr.io/kaniko-project/executor", stepOps...)
	taskSpecOps = append(taskSpecOps, step)

	return tb.Task(kanikoTaskName, namespace, tb.TaskSpec(taskSpecOps...))
}

func getTaskRun(namespace string) *v1alpha1.TaskRun {
	return tb.TaskRun(kanikoTaskRunName, namespace, tb.TaskRunSpec(
		tb.TaskRunTaskRef(kanikoTaskName),
		tb.TaskRunTimeout(2*time.Minute),
		tb.TaskRunInputs(tb.TaskRunInputsResource("gitsource", tb.TaskResourceBindingRef(kanikoResourceName))),
	))
}

// TestTaskRun is an integration test that will verify a TaskRun using kaniko
func TestKanikoTaskRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)
	t.Parallel()

	repo, err := getDockerRepo()
	if err != nil {
		t.Errorf("Expected to get docker repo")
	}

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	hasSecretConfig, err := createSecret(c.KubeClient, namespace)
	if err != nil {
		t.Fatalf("Expected to create kaniko creds: %v", err)
	}
	if hasSecretConfig {
		logger.Info("Creating service account secret")
	} else {
		logger.Info("Not creating service account secret. This could cause the test to fail locally!")
	}

	logger.Infof("Creating Git PipelineResource %s", kanikoResourceName)
	if _, err := c.PipelineResourceClient.Create(getGitResource(namespace)); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoResourceName, err)
	}

	logger.Infof("Creating Task %s", kanikoTaskName)
	if _, err := c.TaskClient.Create(getTask(repo, namespace, hasSecretConfig)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", kanikoTaskName, err)
	}

	logger.Infof("Creating TaskRun %s", kanikoTaskRunName)
	if _, err := c.TaskRunClient.Create(getTaskRun(namespace)); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", kanikoTaskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	var podName string
	if err := WaitForTaskRunState(c, kanikoTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		podName = tr.Status.PodName
		return TaskRunSucceed(kanikoTaskRunName)(tr)
	}, "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", kanikoTaskRunName, err)
	}

	// There will be a Pod with the expected name.
	if _, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{}); err != nil {
		t.Fatalf("Error getting build pod: %v", err)
	}

	logs, err := getAllLogsFromPod(c.KubeClient.Kube, podName, namespace)
	if err != nil {
		t.Fatalf("Expected to get logs from pod %s: %v", podName, err)
	}
	// check the logs contain our success criteria
	if !strings.Contains(logs, kanikoBuildOutput) {
		t.Fatalf("Expected output %s from pod %s but got %s", kanikoBuildOutput, podName, logs)
	}
	// make sure the pushed digest matches the one we pushed
	re := regexp.MustCompile("digest: (sha256:\\w+)")
	match := re.FindStringSubmatch(logs)
	// make sure we found a match and it has the capture group
	if len(match) != 2 {
		t.Fatalf("Expected to find an image digest in the build output")
	}
	// match the local digest, which is first capture group against the remote image
	digest := match[1]
	remoteDigest, err := getRemoteDigest(repo)
	if err != nil {
		t.Fatalf("Expected to get digest for remote image %s", repo)
	}
	if digest != remoteDigest {
		t.Fatalf("Expected local digest %s to match remote digest %s", digest, remoteDigest)
	}
}

func getContainerLogs(c kubernetes.Interface, pod, namespace string, containers ...string) (string, error) {
	sb := strings.Builder{}
	for _, container := range containers {
		req := c.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Follow: true, Container: container})
		rc, err := req.Stream()
		if err != nil {
			return "", err
		}
		bs, err := ioutil.ReadAll(rc)
		if err != nil {
			return "", err
		}
		sb.Write(bs)
	}
	return sb.String(), nil
}

func getAllLogsFromPod(c kubernetes.Interface, pod, namespace string) (string, error) {
	p, err := c.CoreV1().Pods(namespace).Get(pod, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var containers []string
	for _, initContainer := range p.Spec.InitContainers {
		containers = append(containers, initContainer.Name)
	}
	for _, container := range p.Spec.Containers {
		containers = append(containers, container.Name)
	}

	return getContainerLogs(c, pod, namespace, containers...)
}

func getRemoteDigest(image string) (string, error) {
	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return "", fmt.Errorf("could not parse image reference %q: %v", image, err)
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", fmt.Errorf("could not pull remote ref %s: %v", ref, err)
	}
	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("could not get digest for image %s: %v", img, err)
	}
	return digest.String(), nil
}
