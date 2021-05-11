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
	scriptsDir            = "/tekton/scripts"
	defaultScriptPreamble = "#!/bin/sh\nset -xe\n"
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
)

// convertScripts converts any steps and sidecars that specify a Script field into a normal Container.
//
// It does this by prepending a container that writes specified Script bodies
// to executable files in a shared volumeMount, then produces Containers that
// simply run those executable files.
func convertScripts(shellImage string, steps []v1beta1.Step, sidecars []v1beta1.Sidecar) (*corev1.Container, []corev1.Container, []corev1.Container) {
	placeScripts := false
	placeScriptsInit := corev1.Container{
		Name:         "place-scripts",
		Image:        shellImage,
		Command:      []string{"sh"},
		Args:         []string{"-c", ""},
		VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
	}

	convertedStepContainers := convertListOfSteps(steps, &placeScriptsInit, &placeScripts, "script")

	sideCarSteps := []v1beta1.Step{}
	for _, step := range sidecars {
		sidecarStep := v1beta1.Step{
			Container: step.Container,
			Script:    step.Script,
			Timeout:   &metav1.Duration{},
		}
		sideCarSteps = append(sideCarSteps, sidecarStep)
	}
	sidecarContainers := convertListOfSteps(sideCarSteps, &placeScriptsInit, &placeScripts, "sidecar-script")

	if placeScripts {
		return &placeScriptsInit, convertedStepContainers, sidecarContainers
	}
	return nil, convertedStepContainers, sidecarContainers
}

// convertListOfSteps does the heavy lifting for convertScripts.
//
// It iterates through the list of steps (or sidecars), generates the script file name and heredoc termination string,
// adds an entry to the init container args, sets up the step container to run the script, and sets the volume mounts.
func convertListOfSteps(steps []v1beta1.Step, initContainer *corev1.Container, placeScripts *bool, namePrefix string) []corev1.Container {
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
		containers = append(containers, steps[i].Container)
	}
	return containers
}
