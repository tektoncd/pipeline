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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	scriptsVolumeName     = "tekton-internal-scripts"
	debugScriptsVolumeName     = "tekton-internal-debug-scripts"
	debugInfoVolumeName = "tekton-internal-debug-info"
	scriptsDir            = "/tekton/scripts"
	debugScriptsDir            = "/tekton/debug/scripts"
	defaultScriptPreamble = "#!/bin/sh\nset -xe\n"
	debugInfoDir = "/tekton/debug/info"
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
func convertScripts(shellImage string, steps []v1beta1.Step, sidecars []v1beta1.Sidecar, breakpointConfig *v1beta1.TaskRunBreakpoint) (*corev1.Container, []corev1.Container, []corev1.Container) {
	placeScripts := false
	// Place scripts is an init container used for creating scripts in the
	// /tekton/scripts directory which would be later used by the step containers
	// as a Command
	placeScriptsInit := corev1.Container{
		Name:         "place-scripts",
		Image:        shellImage,
		Command:      []string{"sh"},
		Args:         []string{"-c", ""},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}

	var convertedStepContainers []corev1.Container

	breakpointEnabled := false
	sideCarSteps := []v1beta1.Step{}
	for _, step := range sidecars {
		sidecarStep := v1beta1.Step{
			Container: step.Container,
			Script:    step.Script,
			Timeout:   &metav1.Duration{},
		}
		sideCarSteps = append(sideCarSteps, sidecarStep)
	}

	// Disable breakpoint in sidecar step to container conversion to not rewrite the scripts
	sidecarContainers := convertListOfSteps(sideCarSteps, &placeScriptsInit, &placeScripts, &breakpointEnabled,  "sidecar-script")

	if breakpointConfig != nil && breakpointConfig.OnFailure {
		breakpointEnabled = true
		placeScriptsInit.VolumeMounts = append(placeScriptsInit.VolumeMounts, debugScriptsVolumeMount)
	}

	convertedStepContainers = convertListOfSteps(steps, &placeScriptsInit, &placeScripts, &breakpointEnabled,"script")

	if placeScripts {
		return &placeScriptsInit, convertedStepContainers, sidecarContainers
	}
	return nil, convertedStepContainers, sidecarContainers
}

// convertListOfSteps does the heavy lifting for convertScripts.
//
// It iterates through the list of steps (or sidecars), generates the script file name and heredoc termination string,
// adds an entry to the init container args, sets up the step container to run the script, and sets the volume mounts.
func convertListOfSteps(steps []v1beta1.Step, initContainer *corev1.Container, placeScripts, breakpointEnabled *bool, namePrefix string) []corev1.Container {
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

		// Append to the place-scripts script to place the
		// script file in a known location in the scripts volume.
		tmpFile := filepath.Join(scriptsDir, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-%d", namePrefix, i)))
		// heredoc is the "here document" placeholder string
		// used to cat script contents into the file. Typically
		// this is the string "EOF" but if this value were
		// "EOF" it would prevent users from including the
		// string "EOF" in their own scripts. Instead we
		// randomly generate a string to (hopefully) prevent
		// collisions.
		heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-heredoc-randomly-generated", namePrefix))
		initContainer.Args[1] += fmt.Sprintf(`tmpfile="%s"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << '%s'
%s
%s
`, tmpFile, heredoc, script, heredoc)

		debugInfoVolumeMount := corev1.VolumeMount{
			Name: debugInfoVolumeName,
			MountPath: filepath.Join(debugInfoDir,fmt.Sprintf("%d",i)),
		}

		// Set the command to execute the correct script in the mounted
		// volume.
		// A previous merge with stepTemplate may have populated
		// Command and Args, even though this is not normally valid, so
		// we'll clear out the Args and overwrite Command.
		steps[i].Command = []string{tmpFile}
		steps[i].VolumeMounts = append(steps[i].VolumeMounts, scriptsVolumeMount, debugScriptsVolumeMount, debugInfoVolumeMount)
		containers = append(containers, steps[i].Container)
	}

	// Setup the breakpoint container and place scripts for if breakpoint is enabled
	if *breakpointEnabled {
		// Debug script names
		debugBreakpointExitScriptName := "breakpoint-exit"
		debugContinueScriptName := "continue"
		debugContinueFailScriptName := "continue-fail"
		// Initial profile path which would be later changed by the debug-run command
		profilePath := filepath.Join("etc","profile")
		aliasheredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-heredoc-randomly-generated", "debug-alias"))
		aliases := ``

		// debugBreakpointExitScript is used by the step-breakpoint container as an entrypoint
		debugBreakpointExitScript := defaultScriptPreamble+fmt.Sprintf(`
numberOfSteps=%d
debugInfo=%s
tektonTools=%s

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	echo "debug" > ${tektonTools}/${stepNumber}.breakpointexit
	echo "Clearing breapoint for step $stepNumber..."
else
	echo "Last breakpoint (no. $stepNumber) has already been executed, exiting !"
	exit 0
fi`, len(steps), debugInfoDir, mountPoint)

		// debugContinueScript can be used by the user to mark the failing step as a success
		debugContinueScript := defaultScriptPreamble+fmt.Sprintf(`
numberOfSteps=%d
debugInfo=%s
tektonTools=%s

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	echo "debug" > ${tektonTools}/${stepNumber} # Mark step as success
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, breakpoint exiting !"
	exit 0
fi`, len(steps), debugInfoDir, mountPoint)

		// debugContinueScript can be used by the user to mark the failing step as a success
		debugContinueFailScript := defaultScriptPreamble+fmt.Sprintf(`
numberOfSteps=%d
debugInfo=%s
tektonTools=%s

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	echo "debug" > ${tektonTools}/${stepNumber}.err # Mark step as success
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, breakpoint exiting !"
	exit 0
fi`, len(steps), debugInfoDir, mountPoint)

		// Script names and their content
		debugScripts := map[string]string{
			debugBreakpointExitScriptName: debugBreakpointExitScript,
			debugContinueScriptName:       debugContinueScript,
			debugContinueFailScriptName:   debugContinueFailScript,
		}

		// Script names and their file paths which would be added ahead
		debugScriptsPaths := map[string]string{
			debugBreakpointExitScriptName: filepath.Join(debugScriptsDir, fmt.Sprintf("%s-%s", "debug", debugBreakpointExitScriptName)),
			debugContinueScriptName:       filepath.Join(debugScriptsDir, fmt.Sprintf("%s-%s", "debug", debugContinueScriptName)),
		}

		// Add /etc/profile

		// Make a list of aliases for the scripts which would be added to /.profile
		for debugScriptName, debugScriptPath := range debugScriptsPaths {
			aliases += fmt.Sprintf(`
alias debug-%s=%s`, debugScriptName, debugScriptPath)
		}

		// Add routine in the initContainer to create /etc/profile
		initContainer.Args[1] += fmt.Sprintf(`tmpfile="%s"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << '%s'
%s
%s
`, profilePath, aliasheredoc, aliases, aliasheredoc)

		// Add debug/breakpoint related scripts to /tekton/debug/scripts
		// Iterate through the debugScripts and add routine for each of them in the initContainer for their creation
		for debugScriptName, debugScript := range debugScripts {
			tmpFile := filepath.Join(debugScriptsDir, fmt.Sprintf("%s-%s", "debug", debugScriptName))
			heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-heredoc-randomly-generated", "debug", debugScriptName))

			initContainer.Args[1] += fmt.Sprintf(`tmpfile="%s"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << '%s'
%s
%s
`, tmpFile, heredoc, debugScript, heredoc)
		}

	}

	return containers
}
