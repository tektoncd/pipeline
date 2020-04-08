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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

type scripTool struct {
	Path     string
	Contents string
}

const (
	scriptsVolumeName       = "tekton-internal-scripts"
	debugScriptsVolumeName  = "tekton-debug-internal-scripts"
	scriptsDir              = "/tekton/scripts"
	debugScriptsDebugDir    = "/tekton/debug/scripts"
	defaultScriptPreamble   = "#!/bin/sh\nset -xe\n"
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
	debugScriptsMount = corev1.VolumeMount{
		Name:      debugScriptsVolumeName,
		MountPath: debugScriptsDebugDir,
	}
	debugStartScriptPath    = filepath.Join(debugScriptsDebugDir,"start-task")
	debugContinueScriptPath = filepath.Join(debugScriptsDebugDir,"continue-task")
	//superExitScriptPath = filepath.Join(scriptsDir, "super-exit")
)

// convertScripts converts any steps and sidecars that specify a Script field into a normal Container.
//
// It does this by prefixing a container that writes specified Script bodies
// to executable files in a shared volumeMount, then produces Containers that
// simply run those executable files.
func convertScripts(shellImage string, steps []v1alpha1.Step, sidecars []v1alpha1.Sidecar, debugMode bool) (*corev1.Container, []corev1.Container, []corev1.Container) {
	placeScripts := false
	placeScriptsArgs := []string{"-c", ""}
	placeScriptsVolumeMounts := []corev1.VolumeMount{scriptsVolumeMount}

	// number of steps used to understand when do we stop with the debugging
	numberOfSteps := len(steps)

	placeScriptsInit := corev1.Container{
		Name:         "place-scripts",
		Image:        shellImage,
		TTY:          true,
		Command:      []string{"sh"},
		Args:         placeScriptsArgs,
		VolumeMounts: placeScriptsVolumeMounts,
	}
	addSuperScripts(&placeScriptsInit)
	if debugMode {
		addDebugScripts(&placeScriptsInit, numberOfSteps)
	}

	convertedStepContainers := convertListOfSteps(steps, &placeScriptsInit, &placeScripts, "script", debugMode)
	sidecarContainers := convertListOfSteps(sidecars, &placeScriptsInit, &placeScripts, "sidecar-script", debugMode)
	if placeScripts {
		return &placeScriptsInit, convertedStepContainers, sidecarContainers
	}
	return nil, convertedStepContainers, sidecarContainers
}

func addSuperScripts(initContainer *corev1.Container) {
	heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-heredoc-randomly-generated", "super"))
	superEntrypointContents := "tail -f /dev/null"

	tooling := []scripTool{
		{Path: superEntrypointPath, Contents: superEntrypointContents},
	}

	for _, st := range tooling {
		// Script which will run in the debug container
		initContainer.Args[1] += fmt.Sprintf(`superFile="%s"
touch ${superFile} && chmod +x ${superFile}
cat > ${superFile} << '%s'
%s
%s
`, st.Path, heredoc, st.Contents, heredoc)
	}

}

func addDebugScripts(initContainer *corev1.Container, numberOfSteps int) {
	initContainer.VolumeMounts = append(initContainer.VolumeMounts, debugToolsMount, debugScriptsMount)
	debugReadyFilePath := filepath.Join(debugMountPoint, readyFile)
	debugStartContent := fmt.Sprintf(`echo "ready" > "%s"
echo "Executing step 0..."
`, debugReadyFilePath)

	debugContinueContent := fmt.Sprintf(`debugMountPoint="%s"
mountPoint="%s"
numberOfSteps=%d
	
debugPostFile="$(ls ${debugMountPoint} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${debugPostFile} | sed 's/[^0-9]*//g')"
nextStepNumber=$((stepNumber+1))
if [ $nextStepNumber -lt $numberOfSteps ]; then
	cp ${debugMountPoint}/${debugPostFile} ${mountPoint}/${stepNumber}
	echo "Executing step $nextStepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed !"
fi
`, debugMountPoint, mountPoint, numberOfSteps)

	debugScriptContents := defaultScriptPreamble+fmt.Sprintf(`debugReady="%s"
while true
do
if [[ ! -f ${debugReady} ]]; then
	sleep 2
fi
done
`, debugReadyFilePath)

	tooling := []scripTool{
		{Path: debugEntrypointPath, Contents: debugScriptContents},
		{Path: debugStartScriptPath, Contents: debugStartContent},
		{Path: debugContinueScriptPath, Contents: debugContinueContent},
	}

	for _, dt := range tooling {
		heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("%s-heredoc-randomly-generated", "debug"))
		// Script which will run in the debug container
		initContainer.Args[1] += fmt.Sprintf(`tmpFile="%s"
touch ${tmpFile} && chmod +x ${tmpFile}
cat > ${tmpFile} << '%s'
%s
%s
`, dt.Path, heredoc, dt.Contents, heredoc)
	}
}

// convertListOfSteps does the heavy lifting for convertScripts.
//
// It iterates through the list of steps (or sidecars), generates the script file name and heredoc termination string,
// adds an entry to the init container args, sets up the step container to run the script, and sets the volume mounts.
func convertListOfSteps(steps []v1alpha1.Step, initContainer *corev1.Container, placeScripts *bool, namePrefix string, debugMode bool) []corev1.Container {
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

		// Set the command to execute the correct script in the mounted
		// volume.
		// A previous merge with stepTemplate may have populated
		// Command and Args, even though this is not normally valid, so
		// we'll clear out the Args and overwrite Command.
		steps[i].Command = []string{tmpFile}
		steps[i].VolumeMounts = append(steps[i].VolumeMounts, scriptsVolumeMount)
		if debugMode {
			steps[i].VolumeMounts = append(steps[i].VolumeMounts, debugScriptsMount)
		}
		containers = append(containers, steps[i].Container)
	}
	return containers
}
