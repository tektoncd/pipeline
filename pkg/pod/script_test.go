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
	gotInit, gotScripts, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
		Container: corev1.Container{
			Image: "step-1",
		},
	}, {
		Container: corev1.Container{
			Image: "step-2",
		},
	}}, []v1beta1.Sidecar{}, nil)
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
	gotInit, gotScripts, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
		Container: corev1.Container{
			Image: "step-1",
		},
	}, {
		Container: corev1.Container{
			Image: "step-2",
		},
	}}, nil, nil)
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
	gotInit, gotScripts, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
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
	}}, nil)
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

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
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
	}}, []v1beta1.Sidecar{}, nil)
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImage,
		Command: []string{"sh"},
		Args: []string{"-c", `scriptfile="/tekton/scripts/script-0-9l9zj"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
IyEvYmluL3NoCnNjcmlwdC0x
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
scriptfile="/tekton/scripts/script-2-mz4c7"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
CiMhL2Jpbi9zaApzY3JpcHQtMw==
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
scriptfile="/tekton/scripts/script-3-mssqb"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
IyEvYmluL3NoCnNldCAtZQpuby1zaGViYW5n
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
`},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, binMount},
	}
	want := []corev1.Container{{
		Image:        "step-1",
		Command:      []string{"/tekton/scripts/script-0-9l9zj"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}, {
		Image: "step-2",
	}, {
		Image:        "step-3",
		Command:      []string{"/tekton/scripts/script-2-mz4c7"},
		Args:         []string{"my", "args"},
		VolumeMounts: append(preExistingVolumeMounts, scriptsVolumeMount),
	}, {
		Image:   "step-3",
		Command: []string{"/tekton/scripts/script-3-mssqb"},
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

func TestConvertScripts_WithBreakpoint_OnFailure(t *testing.T) {
	names.TestingSeed()

	preExistingVolumeMounts := []corev1.VolumeMount{{
		Name:      "pre-existing-volume-mount",
		MountPath: "/mount/path",
	}, {
		Name:      "another-one",
		MountPath: "/another/one",
	}}

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
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
	}}, []v1beta1.Sidecar{}, &v1beta1.TaskRunDebug{
		Breakpoint: []string{breakpointOnFailure},
	})
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImage,
		Command: []string{"sh"},
		Args: []string{"-c", `scriptfile="/tekton/scripts/script-0-9l9zj"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
IyEvYmluL3NoCnNjcmlwdC0x
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
scriptfile="/tekton/scripts/script-2-mz4c7"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
CiMhL2Jpbi9zaApzY3JpcHQtMw==
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
scriptfile="/tekton/scripts/script-3-mssqb"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
IyEvYmluL3NoCnNldCAtZQpuby1zaGViYW5n
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
tmpfile="/tekton/debug/scripts/debug-continue"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'debug-continue-heredoc-randomly-generated-78c5n'
#!/bin/sh
set -e

numberOfSteps=4
debugInfo=/tekton/debug/info
tektonRun=/tekton/run

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	touch ${tektonRun}/${stepNumber}/out # Mark step as success
	echo "0" > ${tektonRun}/${stepNumber}/out.breakpointexit
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, breakpoint exiting !"
	exit 0
fi
debug-continue-heredoc-randomly-generated-78c5n
tmpfile="/tekton/debug/scripts/debug-fail-continue"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'debug-fail-continue-heredoc-randomly-generated-6nl7g'
#!/bin/sh
set -e

numberOfSteps=4
debugInfo=/tekton/debug/info
tektonRun=/tekton/run

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	touch ${tektonRun}/${stepNumber}/out.err # Mark step as a failure
	echo "1" > ${tektonRun}/${stepNumber}/out.breakpointexit
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, breakpoint exiting !"
	exit 0
fi
debug-fail-continue-heredoc-randomly-generated-6nl7g
`},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, binMount, debugScriptsVolumeMount},
	}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"/tekton/scripts/script-0-9l9zj"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount, debugScriptsVolumeMount,
			{Name: debugInfoVolumeName, MountPath: "/tekton/debug/info/0"}},
	}, {
		Image: "step-2",
	}, {
		Image:   "step-3",
		Command: []string{"/tekton/scripts/script-2-mz4c7"},
		Args:    []string{"my", "args"},
		VolumeMounts: append(preExistingVolumeMounts, scriptsVolumeMount, debugScriptsVolumeMount,
			corev1.VolumeMount{Name: debugInfoVolumeName, MountPath: "/tekton/debug/info/2"}),
	}, {
		Image:   "step-3",
		Command: []string{"/tekton/scripts/script-3-mssqb"},
		Args:    []string{"my", "args"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "pre-existing-volume-mount", MountPath: "/mount/path"},
			{Name: "another-one", MountPath: "/another/one"},
			scriptsVolumeMount, debugScriptsVolumeMount,
			{Name: debugInfoVolumeName, MountPath: "/tekton/debug/info/3"},
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

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
		Script: `#!/bin/sh
script-1`,
		Container: corev1.Container{Image: "step-1"},
	}, {
		// No script to convert here.:
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
	}}, nil)
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImage,
		Command: []string{"sh"},
		Args: []string{"-c", `scriptfile="/tekton/scripts/script-0-9l9zj"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
IyEvYmluL3NoCnNjcmlwdC0x
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
scriptfile="/tekton/scripts/script-2-mz4c7"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
IyEvYmluL3NoCnNjcmlwdC0z
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
scriptfile="/tekton/scripts/sidecar-script-0-mssqb"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '_EOF_'
IyEvYmluL3NoCnNpZGVjYXItMQ==
_EOF_
/tekton/bin/entrypoint decode-script "${scriptfile}"
`},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, binMount},
	}
	want := []corev1.Container{{
		Image:        "step-1",
		Command:      []string{"/tekton/scripts/script-0-9l9zj"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}, {
		Image: "step-2",
	}, {
		Image:   "step-3",
		Command: []string{"/tekton/scripts/script-2-mz4c7"},
		Args:    []string{"my", "args"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "pre-existing-volume-mount", MountPath: "/mount/path"},
			{Name: "another-one", MountPath: "/another/one"},
			scriptsVolumeMount,
		},
	}}

	wantSidecars := []corev1.Container{{
		Image:        "sidecar-1",
		Command:      []string{"/tekton/scripts/sidecar-script-0-mssqb"},
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

func TestConvertScripts_Windows(t *testing.T) {
	names.TestingSeed()

	preExistingVolumeMounts := []corev1.VolumeMount{{
		Name:      "pre-existing-volume-mount",
		MountPath: "/mount/path",
	}, {
		Name:      "another-one",
		MountPath: "/another/one",
	}}

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
		Script: `#!win pwsh -File
script-1`,
		Container: corev1.Container{Image: "step-1"},
	}, {
		// No script to convert here.
		Container: corev1.Container{Image: "step-2"},
	}, {
		Script: `#!win powershell -File
script-3`,
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: preExistingVolumeMounts,
			Args:         []string{"my", "args"},
		},
	}, {
		Script: `#!win
no-shebang`,
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: preExistingVolumeMounts,
			Args:         []string{"my", "args"},
		},
	}}, []v1beta1.Sidecar{}, nil)
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImageWin,
		Command: []string{"pwsh"},
		Args: []string{"-Command", `@"
#!win pwsh -File
script-1
"@ | Out-File -FilePath /tekton/scripts/script-0-9l9zj
@"
#!win powershell -File
script-3
"@ | Out-File -FilePath /tekton/scripts/script-2-mz4c7.ps1
@"
no-shebang
"@ | Out-File -FilePath /tekton/scripts/script-3-mssqb.cmd
`},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, binMount},
	}
	want := []corev1.Container{{
		Image:        "step-1",
		Command:      []string{"pwsh"},
		Args:         []string{"-File", "/tekton/scripts/script-0-9l9zj"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}, {
		Image: "step-2",
	}, {
		Image:        "step-3",
		Command:      []string{"powershell"},
		Args:         []string{"-File", "/tekton/scripts/script-2-mz4c7.ps1", "my", "args"},
		VolumeMounts: append(preExistingVolumeMounts, scriptsVolumeMount),
	}, {
		Image:   "step-3",
		Command: []string{"/tekton/scripts/script-3-mssqb.cmd"},
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

func TestConvertScripts_Windows_WithSidecar(t *testing.T) {
	names.TestingSeed()

	preExistingVolumeMounts := []corev1.VolumeMount{{
		Name:      "pre-existing-volume-mount",
		MountPath: "/mount/path",
	}, {
		Name:      "another-one",
		MountPath: "/another/one",
	}}

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
		Script: `#!win pwsh -File
script-1`,
		Container: corev1.Container{Image: "step-1"},
	}, {
		// No script to convert here.:
		Container: corev1.Container{Image: "step-2"},
	}, {
		Script: `#!win powershell -File
script-3`,
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: preExistingVolumeMounts,
			Args:         []string{"my", "args"},
		},
	}}, []v1beta1.Sidecar{{
		Script: `#!win pwsh -File
sidecar-1`,
		Container: corev1.Container{Image: "sidecar-1"},
	}}, nil)
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImageWin,
		Command: []string{"pwsh"},
		Args: []string{"-Command", `@"
#!win pwsh -File
script-1
"@ | Out-File -FilePath /tekton/scripts/script-0-9l9zj
@"
#!win powershell -File
script-3
"@ | Out-File -FilePath /tekton/scripts/script-2-mz4c7.ps1
@"
#!win pwsh -File
sidecar-1
"@ | Out-File -FilePath /tekton/scripts/sidecar-script-0-mssqb
`},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, binMount},
	}
	want := []corev1.Container{{
		Image:        "step-1",
		Command:      []string{"pwsh"},
		Args:         []string{"-File", "/tekton/scripts/script-0-9l9zj"},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}, {
		Image: "step-2",
	}, {
		Image:   "step-3",
		Command: []string{"powershell"},
		Args:    []string{"-File", "/tekton/scripts/script-2-mz4c7.ps1", "my", "args"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "pre-existing-volume-mount", MountPath: "/mount/path"},
			{Name: "another-one", MountPath: "/another/one"},
			scriptsVolumeMount,
		},
	}}

	wantSidecars := []corev1.Container{{
		Image:        "sidecar-1",
		Command:      []string{"pwsh"},
		Args:         []string{"-File", "/tekton/scripts/sidecar-script-0-mssqb"},
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

func TestConvertScripts_Windows_SidecarOnly(t *testing.T) {
	names.TestingSeed()

	gotInit, gotSteps, gotSidecars := convertScripts(images.ShellImage, images.ShellImageWin, []v1beta1.Step{{
		// No script to convert here.:
		Container: corev1.Container{Image: "step-1"},
	}}, []v1beta1.Sidecar{{
		Script: `#!win python
sidecar-1`,
		Container: corev1.Container{Image: "sidecar-1"},
	}}, nil)
	wantInit := &corev1.Container{
		Name:    "place-scripts",
		Image:   images.ShellImageWin,
		Command: []string{"pwsh"},
		Args: []string{"-Command", `@"
#!win python
sidecar-1
"@ | Out-File -FilePath /tekton/scripts/sidecar-script-0-9l9zj
`},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, binMount},
	}
	want := []corev1.Container{{
		Image: "step-1",
	}}

	wantSidecars := []corev1.Container{{
		Image:        "sidecar-1",
		Command:      []string{"python"},
		Args:         []string{"/tekton/scripts/sidecar-script-0-9l9zj"},
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
