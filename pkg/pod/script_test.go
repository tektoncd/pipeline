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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestConvertScripts_NothingToConvert_EmptySidecars(t *testing.T) {
	gotInit, gotScripts, gotSidecars := convertScripts(images.ShellImage, []v1beta1.Step{{
		Container: corev1.Container{
			Image: "step-1",
		},
	}, {
		Container: corev1.Container{
			Image: "step-2",
		},
	}}, []v1beta1.Sidecar{})
	want := []corev1.Container{{
		Image: "step-1",
	}, {
		Image: "step-2",
	}}
	if d := cmp.Diff(want, gotScripts); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
	if gotInit != nil {
		t.Errorf("Wanted nil init container, got %v", gotInit)
	}

	if len(gotSidecars) != 0 {
		t.Errorf("Wanted 0 sidecars, got %v", len(gotSidecars))
	}
}

func TestConvertScripts_NothingToConvert_NilSidecars(t *testing.T) {
	gotInit, gotScripts, gotSidecars := convertScripts(images.ShellImage, []v1beta1.Step{{
		Container: corev1.Container{
			Image: "step-1",
		},
	}, {
		Container: corev1.Container{
			Image: "step-2",
		},
	}}, nil)
	want := []corev1.Container{{
		Image: "step-1",
	}, {
		Image: "step-2",
	}}
	if d := cmp.Diff(want, gotScripts); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
	if gotInit != nil {
		t.Errorf("Wanted nil init container, got %v", gotInit)
	}

	if len(gotSidecars) != 0 {
		t.Errorf("Wanted 0 sidecars, got %v", len(gotSidecars))
	}
}

func TestConvertScripts_NothingToConvert_WithSidecar(t *testing.T) {
	gotInit, gotScripts, gotSidecars := convertScripts(images.ShellImage, []v1beta1.Step{{
		Container: corev1.Container{
			Image: "step-1",
		},
	}, {
		Container: corev1.Container{
			Image: "step-2",
		},
	}}, []v1beta1.Sidecar{{
		Container: corev1.Container{
			Image: "sidecar-1",
		},
	}})
	want := []corev1.Container{{
		Image: "step-1",
	}, {
		Image: "step-2",
	}}
	wantSidecar := []corev1.Container{{
		Image: "sidecar-1",
	}}
	if d := cmp.Diff(want, gotScripts); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}

	if d := cmp.Diff(wantSidecar, gotSidecars); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}

	if gotInit != nil {
		t.Errorf("Wanted nil init container, got %v", gotInit)
	}

	if len(gotSidecars) != 1 {
		t.Errorf("Wanted 1 sidecar, got %v", len(gotSidecars))
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

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, []v1beta1.Step{{
		Script: `#!/bin/sh
script-1`,
		Container: corev1.Container{Image: "step-1"},
	}, {
		// No script to convert here.
		Container: corev1.Container{Image: "step-2"},
	}, {
		Script: `
#!/bin/sh
script-3`,
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: preExistingVolumeMounts,
			Args:         []string{"my", "args"},
		},
	}, {
		Script: `no-shebang`,
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: preExistingVolumeMounts,
			Args:         []string{"my", "args"},
		},
	}}, []v1beta1.Sidecar{})
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImage,
		Command: []string{"sh"},
		Args: []string{"-c", `tmpfile="/tekton/scripts/script-0-9l9zj"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-mz4c7'
#!/bin/sh
script-1
script-heredoc-randomly-generated-mz4c7
tmpfile="/tekton/scripts/script-2-mssqb"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-78c5n'

#!/bin/sh
script-3
script-heredoc-randomly-generated-78c5n
tmpfile="/tekton/scripts/script-3-6nl7g"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-j2tds'
#!/bin/sh
set -xe
no-shebang
script-heredoc-randomly-generated-j2tds
`},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}
	want := []corev1.Container{{
		Image:        "step-1",
		Command:      []string{"/tekton/scripts/script-0-9l9zj"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}, {
		Image: "step-2",
	}, {
		Image:        "step-3",
		Command:      []string{"/tekton/scripts/script-2-mssqb"},
		Args:         []string{"my", "args"},
		VolumeMounts: append(preExistingVolumeMounts, scriptsVolumeMount),
	}, {
		Image:   "step-3",
		Command: []string{"/tekton/scripts/script-3-6nl7g"},
		Args:    []string{"my", "args"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "pre-existing-volume-mount", MountPath: "/mount/path"},
			{Name: "another-one", MountPath: "/another/one"},
			scriptsVolumeMount,
		},
	}}
	if d := cmp.Diff(wantInit, gotInit); d != "" {
		t.Errorf("Init Container Diff %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(want, gotSteps); d != "" {
		t.Errorf("Containers Diff %s", diff.PrintWantGot(d))
	}

	if len(gotSidecars) != 0 {
		t.Errorf("Expected zero sidecars, got %v", len(gotSidecars))
	}
}

func TestConvertScripts_WithSidecar(t *testing.T) {
	names.TestingSeed()

	preExistingVolumeMounts := []corev1.VolumeMount{{
		Name:      "pre-existing-volume-mount",
		MountPath: "/mount/path",
	}, {
		Name:      "another-one",
		MountPath: "/another/one",
	}}

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, []v1beta1.Step{{
		Script: `#!/bin/sh
script-1`,
		Container: corev1.Container{Image: "step-1"},
	}, {
		// No script to convert here.
		Container: corev1.Container{Image: "step-2"},
	}, {
		Script: `#!/bin/sh
script-3`,
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: preExistingVolumeMounts,
			Args:         []string{"my", "args"},
		},
	}}, []v1beta1.Sidecar{{
		Script: `#!/bin/sh
sidecar-1`,
		Container: corev1.Container{Image: "sidecar-1"},
	}})
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImage,
		Command: []string{"sh"},
		Args: []string{"-c", `tmpfile="/tekton/scripts/script-0-9l9zj"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-mz4c7'
#!/bin/sh
script-1
script-heredoc-randomly-generated-mz4c7
tmpfile="/tekton/scripts/script-2-mssqb"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-78c5n'
#!/bin/sh
script-3
script-heredoc-randomly-generated-78c5n
tmpfile="/tekton/scripts/sidecar-script-0-6nl7g"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'sidecar-script-heredoc-randomly-generated-j2tds'
#!/bin/sh
sidecar-1
sidecar-script-heredoc-randomly-generated-j2tds
`},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}
	want := []corev1.Container{{
		Image:        "step-1",
		Command:      []string{"/tekton/scripts/script-0-9l9zj"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}, {
		Image: "step-2",
	}, {
		Image:   "step-3",
		Command: []string{"/tekton/scripts/script-2-mssqb"},
		Args:    []string{"my", "args"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "pre-existing-volume-mount", MountPath: "/mount/path"},
			{Name: "another-one", MountPath: "/another/one"},
			scriptsVolumeMount,
		},
	}}

	wantSidecars := []corev1.Container{{
		Image:        "sidecar-1",
		Command:      []string{"/tekton/scripts/sidecar-script-0-6nl7g"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}}
	if d := cmp.Diff(wantInit, gotInit); d != "" {
		t.Errorf("Init Container Diff %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(want, gotSteps); d != "" {
		t.Errorf("Step Containers Diff %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(wantSidecars, gotSidecars); d != "" {
		t.Errorf("Sidecar Containers Diff %s", diff.PrintWantGot(d))
	}

	if len(gotSidecars) != 1 {
		t.Errorf("Wanted 1 sidecar, got %v", len(gotSidecars))
	}

}
