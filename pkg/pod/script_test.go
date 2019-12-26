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

func TestConvertScripts_NothingToConvert(t *testing.T) {
	gotInit, got := convertScripts(images.ShellImage, []v1alpha1.Step{{Container: corev1.Container{
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

	gotInit, got := convertScripts(images.ShellImage, []v1alpha1.Step{{
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
	}})
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImage,
		TTY:     true,
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
		t.Errorf("Init Container Diff (-want, +got): %s", d)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Containers Diff (-want, +got): %s", d)
	}
}
