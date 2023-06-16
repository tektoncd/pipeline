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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	scriptsVolumeName      = "tekton-internal-scripts"
	debugScriptsVolumeName = "tekton-internal-debug-scripts"
	debugInfoVolumeName    = "tekton-internal-debug-info"
	scriptsDir             = "/tekton/scripts"
	debugScriptsDir        = "/tekton/debug/scripts"
	defaultScriptPreamble  = "#!/bin/sh\nset -e\n"
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

// convertScripts creates an init container that mounts any Scripts specified by
// the input Steps and Sidecars. It returns the init container, plus two slices of Containers
// representing the Steps and Sidecars, respectively, that use the scripts from the init container.
// Other inputs:
//   - shellImageLinux and shellImageWindows: the images that should be used by the init container,
//     depending on the OS the Task will run on
//   - debugConfig: the TaskRun's debug configuration
//   - setSecurityContext: whether the init container should include a security context that will
//     allow it to run in a namespace with "restricted" pod security admission
func convertScripts(shellImageLinux string, shellImageWin string, steps []v1.Step, sidecars []v1.Sidecar, debugConfig *v1.TaskRunDebug, setSecurityContext bool) (*corev1.Container, []corev1.Container, []corev1.Container) {
	// Place scripts is an init container used for creating scripts in the
	// /tekton/scripts directory which would be later used by the step containers
	// as a Command
	requiresWindows := checkWindowsRequirement(steps, sidecars)

	shellImage := shellImageLinux
	shellCommand := "sh"
	shellArg := "-c"
	securityContext := linuxSecurityContext
	// Set windows variants for Image, Command and Args
	if requiresWindows {
		shellImage = shellImageWin
		shellCommand = "pwsh"
		shellArg = "-Command"
		securityContext = windowsSecurityContext
	}

	placeScriptsInit := corev1.Container{
		Name:         "place-scripts",
		Image:        shellImage,
		Command:      []string{shellCommand},
		Args:         []string{shellArg, ""},
		VolumeMounts: []corev1.VolumeMount{writeScriptsVolumeMount, binMount},
	}
	if setSecurityContext {
		placeScriptsInit.SecurityContext = securityContext
	}

	breakpoints := []string{}

	// Add mounts for debug
	if debugConfig != nil && len(debugConfig.Breakpoint) > 0 {
		breakpoints = debugConfig.Breakpoint
		placeScriptsInit.VolumeMounts = append(placeScriptsInit.VolumeMounts, debugScriptsVolumeMount)
	}

	convertedStepContainers := convertListOfSteps(steps, &placeScriptsInit, breakpoints, "script")
	sidecarContainers := convertListOfSidecars(sidecars, &placeScriptsInit, "sidecar-script")

	if hasScripts(steps, sidecars, breakpoints) {
		return &placeScriptsInit, convertedStepContainers, sidecarContainers
	}
	return nil, convertedStepContainers, sidecarContainers
}

// convertListOfSidecars iterates through the list of sidecars, generates the script file name and heredoc termination string,
// adds an entry to the init container args, sets up the step container to run the script, and sets the volume mounts.
func convertListOfSidecars(sidecars []v1.Sidecar, initContainer *corev1.Container, namePrefix string) []corev1.Container {
	containers := []corev1.Container{}
	for i, s := range sidecars {
		c := s.ToK8sContainer()
		if s.Script != "" {
			placeScriptInContainer(s.Script, getScriptFile(scriptsDir, fmt.Sprintf("%s-%d", namePrefix, i)), c, initContainer)
		}
		containers = append(containers, *c)
	}
	return containers
}

// convertListOfSteps iterates through the list of steps, generates the script file name and heredoc termination string,
// adds an entry to the init container args, sets up the step container to run the script, and sets the volume mounts.
func convertListOfSteps(steps []v1.Step, initContainer *corev1.Container, breakpoints []string, namePrefix string) []corev1.Container {
	containers := []corev1.Container{}
	for i, s := range steps {
		c := steps[i].ToK8sContainer()
		if s.Script != "" {
			placeScriptInContainer(s.Script, getScriptFile(scriptsDir, fmt.Sprintf("%s-%d", namePrefix, i)), c, initContainer)
		}
		containers = append(containers, *c)
	}
	if len(breakpoints) > 0 {
		placeDebugScriptInContainers(containers, initContainer)
	}
	return containers
}

func getScriptFile(scriptsDir, scriptName string) string {
	return filepath.Join(scriptsDir, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(scriptName))
}

// placeScriptInContainer given a piece of script to be executed, placeScriptInContainer firstly modifies initContainer
// so that it capsules the target script into scriptFile, then it modifies the container so that it can execute the scriptFile
// in runtime.
func placeScriptInContainer(script, scriptFile string, c *corev1.Container, initContainer *corev1.Container) {
	if script == "" {
		return
	}
	cleaned := strings.TrimSpace(script)
	hasShebang := strings.HasPrefix(cleaned, "#!")
	requiresWindows := strings.HasPrefix(cleaned, "#!win")

	if !hasShebang {
		script = defaultScriptPreamble + script
	}

	// Append to the place-scripts script to place the
	// script file in a known location in the scripts volume.
	if requiresWindows {
		command, args, script, scriptFile := extractWindowsScriptComponents(script, scriptFile)
		initContainer.Args[1] += fmt.Sprintf(`@"
%s
"@ | Out-File -FilePath %s
`, script, scriptFile)

		c.Command = command
		// Append existing args field to end of derived args
		args = append(args, c.Args...)
		c.Args = args
	} else {
		// Only encode the script for linux scripts
		// The decode-script subcommand of the entrypoint does not work under windows
		script = encodeScript(script)
		heredoc := "_EOF_" // underscores because base64 doesn't include them in its alphabet
		initContainer.Args[1] += fmt.Sprintf(`scriptfile="%s"
touch ${scriptfile} && chmod +x ${scriptfile}
cat > ${scriptfile} << '%s'
%s
%s
/tekton/bin/entrypoint decode-script "${scriptfile}"
`, scriptFile, heredoc, script, heredoc)

		// Set the command to execute the correct script in the mounted volume.
		// A previous merge with stepTemplate may have populated
		// Command and Args, even though this is not normally valid, so
		// we'll clear out the Args and overwrite Command.
		c.Command = []string{scriptFile}
	}
	c.VolumeMounts = append(c.VolumeMounts, scriptsVolumeMount)
}

// encodeScript encodes a script field into a format that avoids kubernetes' built-in processing of container args,
// which can mangle dollar signs and unexpectedly replace variable references in the user's script.
func encodeScript(script string) string {
	return base64.StdEncoding.EncodeToString([]byte(script))
}

// placeDebugScriptInContainers inserts debug scripts into containers. It capsules those scripts to files in initContainer,
// then executes those scripts in target containers.
func placeDebugScriptInContainers(containers []corev1.Container, initContainer *corev1.Container) {
	for i := 0; i < len(containers); i++ {
		debugInfoVolumeMount := corev1.VolumeMount{
			Name:      debugInfoVolumeName,
			MountPath: filepath.Join(debugInfoDir, fmt.Sprintf("%d", i)),
		}
		(&containers[i]).VolumeMounts = append((&containers[i]).VolumeMounts, debugScriptsVolumeMount, debugInfoVolumeMount)
	}

	type script struct {
		name    string
		content string
	}
	debugScripts := []script{{
		name:    "continue",
		content: defaultScriptPreamble + fmt.Sprintf(debugContinueScriptTemplate, len(containers), debugInfoDir, RunDir),
	}, {
		name:    "fail-continue",
		content: defaultScriptPreamble + fmt.Sprintf(debugFailScriptTemplate, len(containers), debugInfoDir, RunDir),
	}}

	// Add debug or breakpoint related scripts to /tekton/debug/scripts
	// Iterate through the debugScripts and add routine for each of them in the initContainer for their creation
	for _, debugScript := range debugScripts {
		tmpFile := filepath.Join(debugScriptsDir, fmt.Sprintf("%s-%s", "debug", debugScript.name))
		heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-%s-heredoc-randomly-generated", "debug", debugScript.name))

		initContainer.Args[1] += fmt.Sprintf(initScriptDirective, tmpFile, heredoc, debugScript.content, heredoc)
	}
}

// hasScripts determines if we need to generate scripts in InitContainer given steps, sidecars and breakpoints.
func hasScripts(steps []v1.Step, sidecars []v1.Sidecar, breakpoints []string) bool {
	for _, s := range steps {
		if s.Script != "" {
			return true
		}
	}
	for _, s := range sidecars {
		if s.Script != "" {
			return true
		}
	}
	return len(breakpoints) > 0
}

func checkWindowsRequirement(steps []v1.Step, sidecars []v1.Sidecar) bool {
	// Detect windows shebangs
	for _, step := range steps {
		cleaned := strings.TrimSpace(step.Script)
		if strings.HasPrefix(cleaned, "#!win") {
			return true
		}
	}
	// If no step needs windows, then check sidecars to be sure
	for _, sidecar := range sidecars {
		cleaned := strings.TrimSpace(sidecar.Script)
		if strings.HasPrefix(cleaned, "#!win") {
			return true
		}
	}
	return false
}

func extractWindowsScriptComponents(script string, fileName string) ([]string, []string, string, string) {
	// Set the command to execute the correct script in the mounted volume.
	shebangLine := strings.Split(script, "\n")[0]
	splitLine := strings.Split(shebangLine, " ")
	var command, args []string
	if len(splitLine) > 1 {
		strippedCommand := splitLine[1:]
		command = strippedCommand[0:1]
		// Handle legacy powershell limitation
		if strings.HasPrefix(command[0], "powershell") {
			fileName += ".ps1"
		}
		if len(strippedCommand) > 1 {
			args = strippedCommand[1:]
			args = append(args, fileName)
		} else {
			args = []string{fileName}
		}
	} else {
		// If no interpreter is specified then strip the shebang and
		// create a .cmd file
		fileName += ".cmd"
		commandLines := strings.Split(script, "\n")[1:]
		script = strings.Join(commandLines, "\n")
		command = []string{fileName}
		args = []string{}
	}

	return command, args, script, fileName
}
