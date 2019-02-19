/*
Copyright 2019 The Knative Authors
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

package builder_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/knative/build-pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPod(t *testing.T) {
	trueB := true
	volume := corev1.Volume{
		Name:         "tools-volume",
		VolumeSource: corev1.VolumeSource{},
	}
	pod := tb.Pod("foo-pod-123456", "foo",
		tb.PodAnnotation("annotation", "annotation-value"),
		tb.PodLabel("label", "label-value"),
		tb.PodOwnerReference("TaskRun", "taskrun-foo",
			tb.OwnerReferenceAPIVersion("a1")),
		tb.PodSpec(
			tb.PodServiceAccountName("sa"),
			tb.PodRestartPolicy(corev1.RestartPolicyNever),
			tb.PodContainer("nop", "nop:latest"),
			tb.PodInitContainer("basic", "ubuntu",
				tb.Command("/bin/sh"),
				tb.Args("-c", "ls -l"),
				tb.WorkingDir("/workspace"),
				tb.EnvVar("HOME", "/builder/home"),
				tb.VolumeMount("tools-volume", "/tools"),
			),
			tb.PodVolumes(volume),
		),
	)
	expectedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-pod-123456",
			Annotations: map[string]string{
				"annotation": "annotation-value",
			},
			Labels: map[string]string{
				"label": "label-value",
			},
			OwnerReferences: []metav1.OwnerReference{{
				Kind:               "TaskRun",
				Name:               "taskrun-foo",
				APIVersion:         "a1",
				Controller:         &trueB,
				BlockOwnerDeletion: &trueB,
			}},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "sa",
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:  "nop",
				Image: "nop:latest",
			}},
			InitContainers: []corev1.Container{{
				Name:       "basic",
				Image:      "ubuntu",
				Command:    []string{"/bin/sh"},
				Args:       []string{"-c", "ls -l"},
				WorkingDir: "/workspace",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/builder/home",
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "tools-volume",
					MountPath: "/tools",
				}},
			}},
			Volumes: []corev1.Volume{volume},
		},
	}
	if d := cmp.Diff(expectedPod, pod); d != "" {
		t.Fatalf("Pod diff -want, +got: %v", d)
	}
}
