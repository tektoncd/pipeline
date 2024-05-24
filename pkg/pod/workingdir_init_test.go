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
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestWorkingDirInit(t *testing.T) {
	names.TestingSeed()
	for _, c := range []struct {
		desc               string
		stepContainers     []corev1.Container
		windows            bool
		setSecurityContext bool
		want               *corev1.Container
	}{{
		desc: "no workingDirs",
		stepContainers: []corev1.Container{{
			Name: "no-working-dir",
		}},
		want: nil,
	}, {
		desc: "workingDirs are unique",
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
			Name:         "working-dir-initializer",
			Image:        images.WorkingDirInitImage,
			Command:      []string{"/ko-app/workingdirinit"},
			Args:         []string{"/workspace/zzz", "/workspace/aaa", "/workspace", "/workspace/bbb"},
			VolumeMounts: implicitVolumeMounts,
		},
	}, {
		desc: "workingDirs are unique + securitycontext",
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
		setSecurityContext: true,
		want: &corev1.Container{
			Name:            "working-dir-initializer",
			Image:           images.WorkingDirInitImage,
			Command:         []string{"/ko-app/workingdirinit"},
			Args:            []string{"/workspace/zzz", "/workspace/aaa", "/workspace", "/workspace/bbb"},
			VolumeMounts:    implicitVolumeMounts,
			SecurityContext: linuxSecurityContext,
		},
	}, {
		desc: "workingDirs are unique, uses windows",
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
		windows: true,
		want: &corev1.Container{
			Name:         "working-dir-initializer",
			Image:        images.WorkingDirInitImage,
			Command:      []string{"/ko-app/workingdirinit"},
			Args:         []string{"/workspace/zzz", "/workspace/aaa", "/workspace", "/workspace/bbb"},
			VolumeMounts: implicitVolumeMounts,
		},
	}, {
		desc: "workingDirs are unique, uses windows, + securityContext",
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
		windows:            true,
		setSecurityContext: true,
		want: &corev1.Container{
			Name:            "working-dir-initializer",
			Image:           images.WorkingDirInitImage,
			Command:         []string{"/ko-app/workingdirinit"},
			Args:            []string{"/workspace/zzz", "/workspace/aaa", "/workspace", "/workspace/bbb"},
			VolumeMounts:    implicitVolumeMounts,
			SecurityContext: windowsSecurityContext,
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := workingDirInit(images.WorkingDirInitImage, c.stepContainers, c.setSecurityContext, c.windows)
			trans := cmp.Transformer("Sort", func(inp corev1.Container) []string {
				out := append([]string(nil), inp.Args...) // Copy input to avoid mutating it
				sort.Strings(out)
				return out
			})
			if d := cmp.Diff(c.want, got, trans); d != "" {
				t.Fatalf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
