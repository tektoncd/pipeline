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

package builder_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/knative/build-pipeline/test/builder"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuild(t *testing.T) {
	trueB := true
	volume := corev1.Volume{
		Name:         "tools-volume",
		VolumeSource: corev1.VolumeSource{},
	}
	build := tb.Build("build-foo", "foo",
		tb.BuildLabel("label", "label-value"),
		tb.BuildOwnerReference("TaskRun", "taskrun-foo",
			tb.OwnerReferenceAPIVersion("a1")),
		tb.BuildSpec(
			tb.BuildServiceAccountName("sa"),
			tb.BuildStep("simple-step", "foo",
				tb.Command("/mycmd"), tb.Args("my", "args"),
				tb.VolumeMount("tools-volume", "/tools"),
			),
			tb.BuildSource("foo", tb.BuildSourceGit("https://foo.git", "master")),
			tb.BuildVolume(volume),
		),
	)
	expectedBuild := &buildv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "build-foo",
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
		Spec: buildv1alpha1.BuildSpec{
			ServiceAccountName: "sa",
			Steps: []corev1.Container{{
				Name:    "simple-step",
				Image:   "foo",
				Command: []string{"/mycmd"},
				Args:    []string{"my", "args"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "tools-volume",
					MountPath: "/tools",
				}},
			}},
			Sources: []buildv1alpha1.SourceSpec{{
				Name: "foo",
				Git:  &buildv1alpha1.GitSourceSpec{Url: "https://foo.git", Revision: "master"},
			}},
			Volumes: []corev1.Volume{volume},
		},
	}
	if d := cmp.Diff(expectedBuild, build); d != "" {
		t.Fatalf("Build diff -want, +got: %v", d)
	}
}
