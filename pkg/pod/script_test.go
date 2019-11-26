package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestConvertScripts_NothingToConvert(t *testing.T) {
	gotInit, got := ConvertScripts(shellImage, []v1alpha1.Step{{Container: corev1.Container{
		Image: "step-1",
	}}, {Container: corev1.Container{
		Image: "step-2",
	}}})
	want := []corev1.Container{{
		Image: "step-1",
	}, {
		Image: "step-2",
	}}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff (-want, +got): %s", d)
	}
	if gotInit != nil {
		t.Errorf("Wanted nil init container, got %v", gotInit)
	}
}

func TestConvertScripts(t *testing.T) {
	names.TestingSeed()

	preExistingVolumeMounts := []corev1.VolumeMount{{
		Name:      "pre-existing-volume-mount",
		MountPath: "/mount/path",
	}, {
		Name:      "another-one",
		MountPath: "/another/one",
	}}

	gotInit, got := ConvertScripts(shellImage, []v1alpha1.Step{{
		Script:    "script-1",
		Container: corev1.Container{Image: "step-1"},
	}, {
		// No script to convert here.
		Container: corev1.Container{Image: "step-2"},
	}, {
		Script: "script-3",
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: preExistingVolumeMounts,
		},
	}})
	wantInit := &corev1.Container{
		Name:    "place-scripts-9l9zj",
		Image:   shellImage,
		TTY:     true,
		Command: []string{"sh"},
		Args: []string{"-c", `tmpfile="/tekton/scripts/script-0-mz4c7"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-mssqb'
script-1
script-heredoc-randomly-generated-mssqb
tmpfile="/tekton/scripts/script-2-78c5n"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-6nl7g'
script-3
script-heredoc-randomly-generated-6nl7g
`},
		VolumeMounts: []corev1.VolumeMount{ScriptsVolumeMount},
	}
	want := []corev1.Container{{
		Image:        "step-1",
		Command:      []string{"/tekton/scripts/script-0-mz4c7"},
		VolumeMounts: []corev1.VolumeMount{ScriptsVolumeMount},
	}, {
		Image: "step-2",
	}, {
		Image:        "step-3",
		Command:      []string{"/tekton/scripts/script-2-78c5n"},
		VolumeMounts: append(preExistingVolumeMounts, ScriptsVolumeMount),
	}}
	if d := cmp.Diff(wantInit, gotInit); d != "" {
		t.Errorf("Init Container Diff (-want, +got): %s", d)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Containers Diff (-want, +got): %s", d)
	}
}
