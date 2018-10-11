/*
Copyright 2018 The Knative Authors

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

// crd contains defintions of resource instances which are useful across integration tests

package test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

const (
	hwVolumeName         = "scratch"
	hwTaskName           = "helloworld"
	hwTaskRunName        = "helloworld-run"
	hwValidationPodName  = "helloworld-validation-busybox"
	hwPipelineName       = "helloworld-pipeline"
	hwPipelineRunName    = "helloworld-pipelinerun"
	hwPipelineParamsName = "helloworld-pipelineparams"
	hwPipelineTaskName1  = "helloworld-task-1"
	hwPipelineTaskName2  = "helloworld-task-2"

	logPath = "/workspace"
	logFile = "out.txt"

	hwContainerName = "helloworld-busybox"
	taskOutput      = "do you want to build a snowman"
	buildOutput     = "Build successful"
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

func getHelloWorldTask(namespace string, args []string) *v1alpha1.Task {
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
						Args:  args,
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

func getHelloWorldPipeline(namespace string) *v1alpha1.Pipeline {
	return &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwPipelineName,
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{
				v1alpha1.PipelineTask{
					Name: hwPipelineTaskName1,
					TaskRef: v1alpha1.TaskRef{
						Name: hwTaskName,
					},
				},
				v1alpha1.PipelineTask{
					Name: hwPipelineTaskName2,
					TaskRef: v1alpha1.TaskRef{
						Name: hwTaskName,
					},
				},
			},
		},
	}
}
func getHelloWorldPipelineParams(namespace string) *v1alpha1.PipelineParams {
	return &v1alpha1.PipelineParams{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwPipelineParamsName,
		},
		Spec: v1alpha1.PipelineParamsSpec{},
	}
}

func getHelloWorldPipelineRun(namespace string) *v1alpha1.PipelineRun {
	return &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwPipelineRunName,
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: hwPipelineName,
			},
			PipelineParamsRef: v1alpha1.PipelineParamsRef{
				Name: hwPipelineParamsName,
			},
			PipelineTriggerRef: v1alpha1.PipelineTriggerRef{
				Type: v1alpha1.PipelineTriggerTypeManual,
			},
		},
	}
}

func VerifyBuildOutput(t *testing.T, c *clients, namespace string, testStr string) {
	// Create Validation Pod
	pods := c.KubeClient.Kube.CoreV1().Pods(namespace)

	if _, err := pods.Create(getHelloWorldValidationPod(namespace)); err != nil {
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
	req := pods.GetLogs(hwValidationPodName, &corev1.PodLogOptions{})
	readCloser, err := req.Stream()
	if err != nil {
		t.Fatalf("Failed to open stream to read: %v", err)
	}
	defer readCloser.Close()
	var buf bytes.Buffer
	out := bufio.NewWriter(&buf)
	_, err = io.Copy(out, readCloser)
	if !strings.Contains(buf.String(), testStr) {
		t.Fatalf("Expected output %s from pod %s but got %s", buildOutput, hwValidationPodName, buf.String())
	}
}
