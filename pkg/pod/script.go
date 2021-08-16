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
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	scriptsVolumeName      = "tekton-internal-scripts"
	debugScriptsVolumeName = "tekton-internal-debug-scripts"
	debugInfoVolumeName    = "tekton-internal-debug-info"
	scriptsDir             = "/tekton/scripts"
	debugScriptsDir        = "/tekton/debug/scripts"
	defaultScriptPreamble  = "#!/bin/sh\nset -xe\n"
	debugInfoDir           = "/tekton/debug/info"
)

var (
	// Volume definition attached to Pods generated from TaskRuns that have
	// steps that specify a Script.
	scriptsVolume = corev1.Volume{
		Name:         scriptsVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	scriptsVolumeMount = corev1.VolumeMount{
		Name:      scriptsVolumeName,
		MountPath: scriptsDir,
		ReadOnly:  true,
	}
	writeScriptsVolumeMount = corev1.VolumeMount{
		Name:      scriptsVolumeName,
		MountPath: scriptsDir,
		ReadOnly:  false,
	}
	debugScriptsVolume = corev1.Volume{
		Name:         debugScriptsVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	debugScriptsVolumeMount = corev1.VolumeMount{
		Name:      debugScriptsVolumeName,
		MountPath: debugScriptsDir,
	}
	debugInfoVolume = corev1.Volume{
		Name:         debugInfoVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
)

// convertScripts converts any steps and sidecars that specify a Script field into a normal Container.
//
// It does this by prepending a container that writes specified Script bodies
// to executable files in a shared volumeMount, then produces Containers that
// simply run those executable files.
func convertScripts(shellImage string, steps []v1beta1.Step, sidecars []v1beta1.Sidecar, debugConfig *v1beta1.TaskRunDebug) (*corev1.Container, []corev1.Container, []corev1.Container) {
	placeScripts := false
	// Place scripts is an init container used for creating scripts in the
	// /tekton/scripts directory which would be later used by the step containers
	// as a Command
	placeScriptsInit := corev1.Container{
		Name:         "place-scripts",
		Image:        shellImage,
		Command:      []string{"sh"},
		Args:         []string{"-c", ""},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, toolsMount},
	}

	breakpoints := []string{}
	sideCarSteps := []v1beta1.Step{}
	for _, step := range sidecars {
		sidecarStep := v1beta1.Step{
			Container: step.Container,
			Script:    step.Script,
			Timeout:   &metav1.Duration{},
		}
		sideCarSteps = append(sideCarSteps, sidecarStep)
	}

	// Add mounts for debug
	if debugConfig != nil && len(debugConfig.Breakpoint) > 0 {
		breakpoints = debugConfig.Breakpoint
		placeScriptsInit.VolumeMounts = append(placeScriptsInit.VolumeMounts, debugScriptsVolumeMount)
	}

	convertedStepContainers := convertListOfSteps(steps, &placeScriptsInit, &placeScripts, breakpoints, "script")

	// Pass empty breakpoint list in "sidecar step to container" converter to not rewrite the scripts and add breakpoints to sidecar
	sidecarContainers := convertListOfSteps(sideCarSteps, &placeScriptsInit, &placeScripts, []string{}, "sidecar-script")
	if placeScripts {
		return &placeScriptsInit, convertedStepContainers, sidecarContainers
	}
	return nil, convertedStepContainers, sidecarContainers
}

// convertListOfSteps does the heavy lifting for convertScripts.
//
// It iterates through the list of steps (or sidecars), generates the script file name and heredoc termination string,
// adds an entry to the init container args, sets up the step container to run the script, and sets the volume mounts.
func convertListOfSteps(steps []v1beta1.Step, initContainer *corev1.Container, placeScripts *bool, breakpoints []string, namePrefix string) []corev1.Container {
	containers := []corev1.Container{}
	for i, s := range steps {
		if s.Script == "" {
			// Nothing to convert.
			containers = append(containers, s.Container)
			continue
		}

		// Check for a shebang, and add a default if it's not set.
		// The shebang must be the first non-empty line.
		cleaned := strings.TrimSpace(s.Script)
		hasShebang := strings.HasPrefix(cleaned, "#!")

		script := s.Script
		if !hasShebang {
			script = defaultScriptPreamble + s.Script
		}

		// At least one step uses a script, so we should return a
		// non-nil init container.
		*placeScripts = true

		script = encodeScript(script)

		// Append to the place-scripts script to place the
		// script file in a known location in the scripts volume.
		scriptFile := filepath.Join(scriptsDir, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-%d", namePrefix, i)))
		heredoc := "_EOF_" // underscores because base64 doesnt include them in its alphabet
		initContainer.Args[1] += fmt.Sprintf(`scriptfile="%s"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '%s'
%s
%s
/tekton/tools/entrypoint decode-script "${scriptfile}"
`, scriptFile, heredoc, script, heredoc)

		// Set the command to execute the correct script in the mounted
		// volume.
		// A previous merge with stepTemplate may have populated
		// Command and Args, even though this is not normally valid, so
		// we'll clear out the Args and overwrite Command.
		steps[i].Command = []string{scriptFile}
		steps[i].VolumeMounts = append(steps[i].VolumeMounts, scriptsVolumeMount)

		// Add debug mounts if breakpoints are present
		if len(breakpoints) > 0 {
			debugInfoVolumeMount := corev1.VolumeMount{
				Name:      debugInfoVolumeName,
				MountPath: filepath.Join(debugInfoDir, fmt.Sprintf("%d", i)),
			}
			steps[i].VolumeMounts = append(steps[i].VolumeMounts, debugScriptsVolumeMount, debugInfoVolumeMount)
		}
		containers = append(containers, steps[i].Container)
	}

	// Place debug scripts if breakpoints are enabled
	if len(breakpoints) > 0 {
		// Debug script names
		debugContinueScriptName := "continue"
		debugFailContinueScriptName := "fail-continue"

		// debugContinueScript can be used by the user to mark the failing step as a success
		debugContinueScript := defaultScriptPreamble + fmt.Sprintf(debugContinueScriptTemplate, len(steps), debugInfoDir, mountPoint)

		// debugFailContinueScript can be used by the user to mark the failing step as a failure
		debugFailContinueScript := defaultScriptPreamble + fmt.Sprintf(debugFailScriptTemplate, len(steps), debugInfoDir, mountPoint)

		// Script names and their content
		debugScripts := map[string]string{
			debugContinueScriptName:     debugContinueScript,
			debugFailContinueScriptName: debugFailContinueScript,
		}

		// Add debug or breakpoint related scripts to /tekton/debug/scripts
		// Iterate through the debugScripts and add routine for each of them in the initContainer for their creation
		for debugScriptName, debugScript := range debugScripts {
			tmpFile := filepath.Join(debugScriptsDir, fmt.Sprintf("%s-%s", "debug", debugScriptName))
			heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-%s-heredoc-randomly-generated", "debug", debugScriptName))

			initContainer.Args[1] += fmt.Sprintf(initScriptDirective, tmpFile, heredoc, debugScript, heredoc)
		}

	}

	return containers
}

// encodeScript encodes a script field into a format that avoids kubernetes' built-in processing of container args,
// which can mangle dollar signs and unexpectedly replace variable references in the user's script.
func encodeScript(script string) string {
	return base64.StdEncoding.EncodeToString([]byte(script))
}
