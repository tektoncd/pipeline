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

package builder_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPod(t *testing.T) {
	trueB := true
	resourceQuantityCmp := cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})
	volume := corev1.Volume{
		Name:         "tools-volume",
		VolumeSource: corev1.VolumeSource{},
	}
	got := tb.Pod("foo-pod-123456",
		tb.PodNamespace("foo"),
		tb.PodAnnotation("annotation", "annotation-value"),
		tb.PodLabel("label", "label-value"),
		tb.PodOwnerReference("TaskRun", "taskrun-foo",
			tb.OwnerReferenceAPIVersion("a1")),
		tb.PodSpec(
			tb.PodServiceAccountName("sa"),
			tb.PodRestartPolicy(corev1.RestartPolicyNever),
			tb.PodContainer("nop", "nop:latest"),
			tb.PodInitContainer("basic", "ubuntu",
				tb.Command("ls", "-l"),
				tb.Args(),
				tb.WorkingDir("/workspace"),
				tb.EnvVar("HOME", "/tekton/home"),
				tb.VolumeMount("tools-volume", "/tools"),
				tb.Resources(
					tb.Limits(tb.Memory("1.5Gi")),
					tb.Requests(
						tb.CPU("100m"),
						tb.Memory("1Gi"),
						tb.EphemeralStorage("500Mi"),
					),
				),
			),
			tb.PodVolumes(volume),
		),
	)
	want := &corev1.Pod{
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
				Command:    []string{"ls", "-l"},
				WorkingDir: "/workspace",
				Env: []corev1.EnvVar{{
					Name:  "HOME",
					Value: "/tekton/home",
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "tools-volume",
					MountPath: "/tools",
				}},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1.5Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("100m"),
						corev1.ResourceMemory:           resource.MustParse("1Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("500Mi"),
					},
				},
			}},
			Volumes: []corev1.Volume{volume},
		},
	}
	if d := cmp.Diff(want, got, resourceQuantityCmp); d != "" {
		t.Fatalf("Pod diff -want, +got: %v", d)
	}
}
