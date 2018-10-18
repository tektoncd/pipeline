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
	"encoding/base64"
	"fmt"
	"io"

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/pkg/test/logging"
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
	hwSecret             = "helloworld-secret"
	hwSA                 = "helloworld-sa"

	logPath = "/workspace"
	logFile = "out.txt"

	hwContainerName = "helloworld-busybox"
	taskOutput      = "do you want to build a snowman"
	buildOutput     = "Build successful"
)

func getHelloWorldValidationPod(namespace, volumeClaimName string) *corev1.Pod {
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
							ClaimName: volumeClaimName,
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
					},
				},
			},
		},
	}
}

func getHelloWorldTaskWithVolume(namespace string, args []string) *v1alpha1.Task {
	t := getHelloWorldTask(namespace, args)
	t.Spec.BuildSpec.Steps[0].VolumeMounts = []corev1.VolumeMount{
		corev1.VolumeMount{
			Name:      "scratch",
			MountPath: logPath,
		},
	}
	t.Spec.BuildSpec.Volumes = []corev1.Volume{
		corev1.Volume{
			Name: "scratch",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: hwVolumeName,
				},
			},
		},
	}
	return t
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

func getHelloWorldPipelineWithSingularTask(namespace string) *v1alpha1.Pipeline {
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

func getBuildOutputFromVolume(logger *logging.BaseLogger, c *clients, namespace, testStr string) (string, error) {
	// Create Validation Pod
	pods := c.KubeClient.Kube.CoreV1().Pods(namespace)

	if _, err := pods.Create(getHelloWorldValidationPod(namespace, hwVolumeName)); err != nil {
		return "", fmt.Errorf("failed to create Volume `%s`: %s", hwVolumeName, err)
	}

	logger.Infof("Waiting for pod with test volume %s to come up so we can read logs from it", hwVolumeName)
	if err := WaitForPodState(c, hwValidationPodName, namespace, func(p *corev1.Pod) (bool, error) {
		// the "Running" status is used as "Succeeded" caused issues as the pod succeeds and restarts quickly
		// there might be a race condition here and possibly a better way of handling this, perhaps using a Job or different state validation
		if p.Status.Phase == corev1.PodRunning {
			return true, nil
		}
		return false, nil
	}, "ValidationPodCompleted"); err != nil {
		return "", fmt.Errorf("error waiting for Pod %s to finish: %s", hwValidationPodName, err)
	}

	// Get validation pod logs and verify that the build executed a container w/ desired output
	req := pods.GetLogs(hwValidationPodName, &corev1.PodLogOptions{})
	readCloser, err := req.Stream()
	if err != nil {
		return "", fmt.Errorf("failed to open stream to read: %v", err)
	}
	defer readCloser.Close()
	var buf bytes.Buffer
	out := bufio.NewWriter(&buf)
	_, err = io.Copy(out, readCloser)
	return buf.String(), nil
}

func createPipelineRunServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwSA,
		},
		Secrets: []corev1.ObjectReference{{
			Name: hwSecret,
		}},
	}
}

func createPipelineRunSecret(namespace string) *corev1.Secret {
	// Generated by:
	//   cat /tmp/key.json | base64 -w 0
	// This service account is JUST a storage reader on gcr.io/build-crd-testing
	encoedDockercred := "ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAiYnVpbGQtY3JkLXRlc3RpbmciLAogICJwcml2YXRlX2tleV9pZCI6ICIwNTAyYTQxYTgxMmZiNjRjZTU2YTY4ZWM1ODMyYWIwYmExMWMxMWU2IiwKICAicHJpdmF0ZV9rZXkiOiAiLS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tXG5NSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUM5WDRFWU9BUmJ4UU04XG5EMnhYY2FaVGsrZ1k4ZWp1OTh0THFDUXFUckdNVzlSZVQyeE9ZNUF5Z2FsUFArcDd5WEVja3dCRC9IaE0wZ2xJXG43TVRMZGVlS1dyK3JBMUx3SFp5V0ZXN0gwT25mN3duWUhFSExXVW1jM0JDT1JFRHRIUlo3WnJQQmYxSFRBQS8zXG5Nblc1bFpIU045b2p6U1NGdzZBVnU2ajZheGJCSUlKNzU0THJnS2VBWXVyd2ZJUTJSTFR1MjAxazJJcUxZYmhiXG4zbVNWRzVSK3RiS3oxQ3ZNNTNuSENiN0NmdVZlV3NyQThrazd4SHJyTFFLTW1JOXYyc2dSdWd5TUF6d3ovNnpOXG5oNS9pTXh4Z2VxNVc4eGtWeDNKMm5ZOEpKZEhhZi9UNkFHc09ORW80M3B4ZWlRVmpuUmYvS24xMFRDYzJFc0lZXG5TNDlVc1o3QkFnTUJBQUVDZ2dFQUF1cGxkdWtDUVF1RDVVL2dhbUh0N0dnVzNBTVYxOGVxbkhuQ2EyamxhaCtTXG5BZVVHbmhnSmpOdkUrcE1GbFN2NXVmMnAySzRlZC9veEQ2K0NwOVpYRFJqZ3ZmdEl5cWpsemJ3dkZjZ3p3TnVEXG55Z1VrdXA3SGVjRHNEOFR0ZUFvYlQvVnB3cTZ6S01yQndDdk5rdnk2YlZsb0VqNXgzYlhzYXhlOTVETy95cHU2XG53MFc5N3p4d3dESlk2S1FjSVdNamhyR3h2d1g3bmlVQ2VNNGxlV0JEeUd0dzF6ZUpuNGhFYzZOM2FqUWFjWEtjXG4rNFFseGNpYW1ZcVFXYlBudHhXUWhoUXpjSFdMaTJsOWNGYlpENyt1SkxGNGlONnk4bVZOVTNLM0sxYlJZclNEXG5SVXAzYVVWQlhtRmcrWi8ycHVWTCttVTNqM0xMV1l5Qk9rZXZ1T21kZ1FLQmdRRGUzR0lRa3lXSVMxNFRkTU9TXG5CaUtCQ0R5OGg5NmVoTDBIa0RieU9rU3RQS2RGOXB1RXhaeGh5N29qSENJTTVGVnJwUk4yNXA0c0V6d0ZhYyt2XG5KSUZnRXZxN21YZm1YaVhJTmllUG9FUWFDbm54RHhXZ21yMEhVS0VtUzlvTWRnTGNHVStrQ1ZHTnN6N0FPdW0wXG5LcVkzczIyUTlsUTY3Rk95cWl1OFdGUTdRUUtCZ1FEWmlGaFRFWmtQRWNxWmpud0pwVEI1NlpXUDlLVHNsWlA3XG53VTRiemk2eSttZXlmM01KKzRMMlN5SGMzY3BTTWJqdE5PWkN0NDdiOTA4RlVtTFhVR05oY3d1WmpFUXhGZXkwXG5tNDFjUzVlNFA0OWI5bjZ5TEJqQnJCb3FzMldCYWwyZWdkaE5KU3NDV29pWlA4L1pUOGVnWHZoN2I5MWp6b0syXG5xMlBVbUE0RGdRS0JnQVdMMklqdkVJME95eDJTMTFjbi9lM1dKYVRQZ05QVEc5MDNVcGErcW56aE9JeCtNYXFoXG5QRjRXc3VBeTBBb2dHSndnTkpiTjhIdktVc0VUdkE1d3l5TjM5WE43dzBjaGFyRkwzN29zVStXT0F6RGpuamNzXG5BcTVPN0dQR21YdWI2RUJRQlBKaEpQMXd5NHYvSzFmSGcvRjQ3cTRmNDBMQUpPa2FZUkpENUh6QkFvR0JBTlVoXG5uSUJQSnFxNElNdlE2Y0M5ZzhCKzF4WURlYTkvWWsxdytTbVBHdndyRVh5M0dLeDRLN2xLcGJQejdtNFgzM3N4XG5zRVUvK1kyVlFtd1JhMXhRbS81M3JLN1YybDVKZi9ENDAwalJtNlpmU0FPdmdEVHJ0Wm5VR0pNcno5RTd1Tnc3XG5sZ1VIM0pyaXZ5Ri9meE1JOHFzelFid1hQMCt4bnlxQXhFQWdkdUtCQW9HQUlNK1BTTllXQ1pYeERwU0hJMThkXG5qS2tvQWJ3Mk1veXdRSWxrZXVBbjFkWEZhZDF6c1hRR2RUcm1YeXY3TlBQKzhHWEJrbkJMaTNjdnhUaWxKSVN5XG51Y05yQ01pcU5BU24vZHE3Y1dERlVBQmdqWDE2SkgyRE5GWi9sL1VWRjNOREFKalhDczFYN3lJSnlYQjZveC96XG5hU2xxbElNVjM1REJEN3F4Unl1S3Nnaz1cbi0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS1cbiIsCiAgImNsaWVudF9lbWFpbCI6ICJwdWxsLXNlY3JldC10ZXN0aW5nQGJ1aWxkLWNyZC10ZXN0aW5nLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwKICAiY2xpZW50X2lkIjogIjEwNzkzNTg2MjAzMzAyNTI1MTM1MiIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi90b2tlbiIsCiAgImF1dGhfcHJvdmlkZXJfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9vYXV0aDIvdjEvY2VydHMiLAogICJjbGllbnRfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9yb2JvdC92MS9tZXRhZGF0YS94NTA5L3B1bGwtc2VjcmV0LXRlc3RpbmclNDBidWlsZC1jcmQtdGVzdGluZy5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIKfQo="

	decoded, err := base64.StdEncoding.DecodeString(encoedDockercred)
	if err != nil {
		return nil
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      hwSecret,
			Annotations: map[string]string{
				"build.knative.dev/docker-0": "https://us.gcr.io",
				"build.knative.dev/docker-1": "https://eu.gcr.io",
				"build.knative.dev/docker-2": "https://asia.gcr.io",
				"build.knative.dev/docker-3": "https://gcr.io",
			},
		},
		Type: "kubernetes.io/basic-auth",
		Data: map[string][]byte{
			"username": []byte("_json_key"),
			"password": decoded,
		},
	}
}
