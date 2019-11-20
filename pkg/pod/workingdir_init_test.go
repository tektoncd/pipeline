package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

const shellImage = "shell-image"

func TestWorkingDirInit(t *testing.T) {
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
			Name:       "working-dir-initializer-9l9zj",
			Image:      shellImage,
			Command:    []string{"sh"},
			Args:       []string{"-c", "mkdir -p /workspace/bbb aaa zzz"},
			WorkingDir: workspaceDir,
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			// TODO(jasonhall): WorkingDirInit should take
			// Containers instead of Steps, but while we're
			// cleaning up pod.go it's easier to have it take
			// Steps. This test doesn't care, so let's hide this
			// conversion in the test where it's easier to remove
			// later.
			var steps []v1alpha1.Step
			for _, c := range c.stepContainers {
				steps = append(steps, v1alpha1.Step{Container: c})
			}

			got := WorkingDirInit(shellImage, steps)
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff (-want, +got): %s", d)
			}
		})
	}
}
