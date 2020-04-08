/*
Copyright 2020 The Tekton Authors

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
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

var (
	debugEntrypointPath = filepath.Join(debugScriptsDebugDir, "debug")
	superEntrypointPath = filepath.Join(scriptsDir, "super")
	superExitFilePath   = filepath.Join(mountPoint, "super-exit")
)

func createHelperContainers(shellImage, entrypointImage string, debugMode bool) (corev1.Container, corev1.Container) {
	var superContainer, debugContainer corev1.Container
	volumeMounts := append(implicitVolumeMounts, scriptsVolumeMount, toolsMount)
	if debugMode {
		volumeMounts = append(volumeMounts, debugScriptsMount, debugToolsMount)
		debugContainer = createDebugContainer(entrypointImage, volumeMounts)
	}

	superContainer = corev1.Container{
		Name:  "super",
		Image: shellImage,
		Command: []string{"sh", "-c"},
		Args:    []string{superEntrypointPath},
		TTY:          true,
		VolumeMounts: volumeMounts,
	}

	return superContainer, debugContainer

}

func createDebugContainer(entrypointImage string, volumeMounts []corev1.VolumeMount) corev1.Container {
	argsForEntrypoint := []string{
		"-wait_file", filepath.Join(downwardMountPoint, readyFile),
		"-post_file", filepath.Join(debugMountPoint, readyFile),
		"-termination_path", terminationPath,
		"-entrypoint", debugEntrypointPath,
		"--",
	}

	debugContainer := corev1.Container{
		Name:                   "debug",
		Image:                  entrypointImage,
		Command:                []string{entrypointBinary},
		Args:                   argsForEntrypoint,
		VolumeMounts:           volumeMounts,
		TerminationMessagePath: terminationPath,

	}

	return debugContainer
}
