//go:build e2e

package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	caConfigMapName   = "config-ca-cert"
	caBundleFileName  = "ca-bundle.crt"
	caMountPath       = "/tekton/certs"
	caBundleFilePath  = caMountPath + "/" + caBundleFileName
	caFeatureFlagName = "enable-trusted-ca-certs"
)

// @test:execution=serial
// @test:reason=modifies enable-trusted-ca-certs in feature-flags ConfigMap
func TestTaskRunInjectsCACertsWhenFeatureFlagEnabled(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	cleanup := func() {
		if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
			caFeatureFlagName: "false",
		}); err != nil {
			t.Logf("Failed to restore feature flag %q: %v", caFeatureFlagName, err)
		}
		tearDown(ctx, t, c, namespace)
	}
	knativetest.CleanupOnInterrupt(cleanup, t.Logf)
	defer cleanup()

	// Given: CA ConfigMap exists and feature flag is enabled.
	if _, err := c.KubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			caBundleFileName: "sample-ca-cert",
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create ConfigMap %q: %s", caConfigMapName, err)
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
		caFeatureFlagName: "true",
	}); err != nil {
		t.Fatalf("Failed to update feature flag %q: %s", caFeatureFlagName, err)
	}

	taskName := helpers.ObjectNameForTest(t)
	taskRunName := helpers.ObjectNameForTest(t)

	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: %s
    script: |
      echo hello
`, taskName, namespace, getTestImage(busyboxImage)))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, taskName))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	// When: TaskRun completes.
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish: %s", err)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}
	if tr.Status.PodName == "" {
		t.Fatal("Error getting a PodName (empty)")
	}

	pod, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, tr.Status.PodName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting pod %q in namespace %q: %s", tr.Status.PodName, namespace, err)
	}

	// Then: pod has CA volume, mount, and SSL_CERT_FILE env.
	var caVolume *corev1.Volume
	for i := range pod.Spec.Volumes {
		if pod.Spec.Volumes[i].ConfigMap != nil && pod.Spec.Volumes[i].ConfigMap.Name == caConfigMapName {
			caVolume = &pod.Spec.Volumes[i]
			break
		}
	}
	if caVolume == nil {
		t.Fatalf("expected volume with ConfigMap name %q, got volumes: %#v", caConfigMapName, pod.Spec.Volumes)
	}

	stepContainers := make([]corev1.Container, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		if strings.HasPrefix(container.Name, "step-") {
			stepContainers = append(stepContainers, container)
		}
	}
	if len(stepContainers) == 0 {
		t.Fatalf("expected at least one step container, got containers: %#v", pod.Spec.Containers)
	}

	for _, container := range stepContainers {
		if !hasVolumeMount(container.VolumeMounts, caVolume.Name, caMountPath) {
			t.Fatalf("expected step container %q to mount volume %q at %q, got mounts: %#v", container.Name, caVolume.Name, caMountPath, container.VolumeMounts)
		}
		if !hasEnvVar(container.Env, "SSL_CERT_FILE", caBundleFilePath) {
			t.Fatalf("expected step container %q to set %q=%q, got env: %#v", container.Name, "SSL_CERT_FILE", caBundleFilePath, container.Env)
		}
	}
}

func hasVolumeMount(volumeMounts []corev1.VolumeMount, name string, mountPath string) bool {
	for _, volumeMount := range volumeMounts {
		if volumeMount.Name == name && volumeMount.MountPath == mountPath {
			return true
		}
	}
	return false
}

func hasEnvVar(envVars []corev1.EnvVar, name string, value string) bool {
	for _, envVar := range envVars {
		if envVar.Name == name && envVar.Value == value {
			return true
		}
	}
	return false
}
