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

package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestWorkingDirInit(t *testing.T) {
	volumeMounts := []corev1.VolumeMount{{
		Name:      "my-volume-mount",
		MountPath: "/blah",
	}}

	names.TestingSeed()
	for _, c := range []struct {
		desc           string
		stepContainers []corev1.Container
		want           *corev1.Container
	}{{
		desc: "no workingDirs",
		stepContainers: []corev1.Container{{
			Name: "no-working-dir",
		}},
		want: nil,
	}, {
		desc: "workingDirs are unique and sorted, absolute dirs are ignored",
		stepContainers: []corev1.Container{{
			WorkingDir: "zzz",
		}, {
			WorkingDir: "aaa",
		}, {
			WorkingDir: "/ignored",
		}, {
			WorkingDir: "/workspace", // ignored
		}, {
			WorkingDir: "zzz",
		}, {
			// Even though it's specified absolute, it's relative
			// to /workspace, so we need to create it.
			WorkingDir: "/workspace/bbb",
		}},
		want: &corev1.Container{
			Name:         "working-dir-initializer-9l9zj",
			Image:        images.ShellImage,
			Command:      []string{"sh"},
			Args:         []string{"-c", "mkdir -p /workspace/bbb aaa zzz"},
			WorkingDir:   workspaceDir,
			VolumeMounts: volumeMounts,
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			// TODO(#1605): WorkingDirInit should take
			// Containers instead of Steps, but while we're
			// cleaning up pod.go it's easier to have it take
			// Steps. This test doesn't care, so let's hide this
			// conversion in the test where it's easier to remove
			// later.
			var steps []v1alpha1.Step
			for _, c := range c.stepContainers {
				steps = append(steps, v1alpha1.Step{Container: c})
			}

			got := workingDirInit(images.ShellImage, steps, volumeMounts)
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff (-want, +got): %s", d)
			}
		})
	}
}
