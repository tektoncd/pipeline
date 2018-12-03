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
				{
					Name:  "Url",
					Value: "https://github.com/pivotal-nader-ziada/gohelloworld",
				},
			},
		},
	}
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
	file := os.Getenv("KANIKO_SECRET_CONFIG_FILE")
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
	task := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      kanikoTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{
					{
						Name: "workspace",
						Type: v1alpha1.PipelineResourceTypeGit,
					},
				},
			},
			Timeout: &metav1.Duration{Duration: 2 * time.Minute},
		},
	}

	step := corev1.Container{
		Name:  "kaniko",
		Image: "gcr.io/kaniko-project/executor",
		Args: []string{"--dockerfile=/workspace/Dockerfile",
			fmt.Sprintf("--destination=%s", repo),
		},
	}
	if withSecretConfig {
		step.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "kaniko-secret",
				MountPath: "/secrets",
			},
		}
		step.Env = []corev1.EnvVar{
			{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/secrets/config.json",
			},
		}
		task.Spec.Volumes = []corev1.Volume{
			{
				Name: "kaniko-secret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "kaniko-secret",
					},
				},
			},
		}
	}

	task.Spec.Steps = []corev1.Container{step}

	return task
}

func getTaskRun(namespace string) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      kanikoTaskRunName,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name: kanikoTaskName,
			},
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{
					{
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: kanikoResourceName,
						},
						Name: "workspace",
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
	if err := WaitForTaskRunState(c, kanikoTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("pipeline run %s failed", hwPipelineRunName)
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

	logs, err := getAllLogsFromPod(c.KubeClient.Kube, cluster.PodName, namespace)
	if err != nil {
		t.Fatalf("Expected to get logs from pod %s: %v", cluster.PodName, err)
	}
	// check the logs contain our success criteria
	if !strings.Contains(logs, kanikoBuildOutput) {
		t.Fatalf("Expected output %s from pod %s but got %s", kanikoBuildOutput, cluster.PodName, logs)
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
