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
	"strings"
	"testing"

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"

	// Mysteriously by k8s libs, or they fail to create `KubeClient`s from config. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	hwVolumeName        = "scratch"
	hwTaskName          = "helloworld"
	hwTaskRunName       = "helloworld-run"
	hwContainerName     = "helloworld-busybox"
	hwValidationPodName = "helloworld-validation-busybox"
	logPath             = "/workspace"
	logFile             = "out.txt"

	taskOutput  = "do you want to build a snowman"
	buildOutput = "Build successful"
)

func getHelloWorldValidationPod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwValidationPodName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				corev1.Container{
					Name:  hwValidationPodName,
					Image: "busybox",
					Args: []string{
						"cat", fmt.Sprintf("%s/%s", logPath, logFile),
					},
					VolumeMounts: []corev1.VolumeMount{
						corev1.VolumeMount{
							Name:      "scratch",
							MountPath: logPath,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				corev1.Volume{
					Name: "scratch",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "scratch",
						},
					},
				},
			},
		},
	}
}

func getHelloWorldTask(namespace string) *v1alpha1.Task {
	return &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwTaskName,
		},
		Spec: v1alpha1.TaskSpec{
			BuildSpec: &buildv1alpha1.BuildSpec{
				Steps: []corev1.Container{
					corev1.Container{
						Name:  hwContainerName,
						Image: "busybox",
						Args: []string{
							"/bin/sh", "-c", fmt.Sprintf("echo %s > /workspace/%s", taskOutput, logFile),
						},
						VolumeMounts: []corev1.VolumeMount{
							corev1.VolumeMount{
								Name:      "scratch",
								MountPath: logPath,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					corev1.Volume{
						Name: "scratch",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "scratch",
							},
						},
					},
				},
			},
		},
	}
}

func getHelloWorldVolumeClaim(namespace string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwVolumeName,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
				},
			},
		},
	}
}

func getHelloWorldTaskRun(namespace string) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwTaskRunName,
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: hwTaskName,
			},
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
		},
	}
}

// TestTaskRun is an integration test that will verify a very simple "hello world" TaskRun can be
// executed.
func TestTaskRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(logger, c.KubeClient, namespace) }, logger)
	defer tearDown(logger, c.KubeClient, namespace)

	// Create Volume
	if _, err := c.KubeClient.Kube.CoreV1().PersistentVolumeClaims(namespace).Create(getHelloWorldVolumeClaim(namespace)); err != nil {
		t.Fatalf("Failed to create Volume `%s`: %s", hwTaskName, err)
	}

	// Create Task
	if _, err := c.TaskClient.Create(getHelloWorldTask(namespace)); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", hwTaskName, err)
	}

	// Create TaskRun
	if _, err := c.TaskRunClient.Create(getHelloWorldTaskRun(namespace)); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", hwTaskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	if err := WaitForTaskRunState(c, hwTaskRunName, func(tr *v1alpha1.TaskRun) (bool, error) {
		if len(tr.Status.Conditions) > 0 && tr.Status.Conditions[0].Status == corev1.ConditionTrue {
			return true, nil
		}
		return false, nil
	}, "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", hwTaskRunName, err)
	}

	// The Build created by the TaskRun will have the same name
	b, err := c.BuildClient.Get(hwTaskRunName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Expected there to be a Build with the same name as TaskRun %s but got error: %s", hwTaskRunName, err)
	}
	cluster := b.Status.Cluster
	if cluster == nil || cluster.PodName == "" {
		t.Fatalf("Expected build status to have a podname but it didn't!")
	}
	podName := cluster.PodName
	pods := c.KubeClient.Kube.CoreV1().Pods(namespace)

	req := pods.GetLogs(podName, &corev1.PodLogOptions{})
	readCloser, err := req.Stream()
	if err != nil {
		t.Fatalf("Failed to open stream to read: %v", err)
	}
	defer readCloser.Close()
	var buf bytes.Buffer
	out := bufio.NewWriter(&buf)
	_, err = io.Copy(out, readCloser)
	if !strings.Contains(buf.String(), buildOutput) {
		t.Fatalf("Expected output %s from pod %s but got %s", buildOutput, podName, buf.String())
	}

	// Create Validation Pod
	if _, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Create(getHelloWorldValidationPod(namespace)); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", hwTaskRunName, err)
	}

	// Verify status of Pod (wait for it)
	if err := WaitForPodState(c, hwValidationPodName, namespace, func(p *corev1.Pod) (bool, error) {
		// the "Running" status is used as "Succeeded" caused issues as the pod succeeds and restarts quickly
		// there might be a race condition here and possibly a better way of handling this, perhaps using a Job or different state validation
		if p.Status.Phase == corev1.PodRunning {
			return true, nil
		}
		return false, nil
	}, "ValidationPodCompleted"); err != nil {
		t.Errorf("Error waiting for Pod %s to finish: %s", hwValidationPodName, err)
	}

	// Get validation pod logs and verify that the build executed a container w/ desired output
	req = pods.GetLogs(hwValidationPodName, &corev1.PodLogOptions{})
	readCloser, err = req.Stream()
	if err != nil {
		t.Fatalf("Failed to open stream to read: %v", err)
	}
	defer readCloser.Close()
	out = bufio.NewWriter(&buf)
	_, err = io.Copy(out, readCloser)
	if !strings.Contains(buf.String(), taskOutput) {
		t.Fatalf("Expected output %s from pod %s but got %s", buildOutput, podName, buf.String())
	}
}
