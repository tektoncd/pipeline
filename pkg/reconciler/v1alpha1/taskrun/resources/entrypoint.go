/*
Copyright 2018 The Knative Authors

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

package resources

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	// MountName is the name of the pvc being mounted (which
	// will contain the entrypoint binary and eventually the logs)
	MountName = "tools"

	mountPoint                 = "/tools"
	entrypointBin              = mountPoint + "/entrypoint"
	entrypointJSONConfigEnvVar = "ENTRYPOINT_OPTIONS"
	EntrypointImage            = "gcr.io/k8s-prow/entrypoint@sha256:7c7cd8906ce4982ffee326218e9fc75da2d4896d53cabc9833b9cc8d2d6b2b8f"
)

var toolsMount = corev1.VolumeMount{
	Name:      MountName,
	MountPath: mountPoint,
}

// GetCopyStep will return a Build Step (Container) that will
// copy the entrypoint binary from the entrypoint image into the
// volume mounted at mountPoint, so that it can be mounted by
// subsequent steps and used to capture logs.
func GetCopyStep() corev1.Container {
	return corev1.Container{
		Name:         "place-tools",
		Image:        EntrypointImage,
		Command:      []string{"/bin/cp"},
		Args:         []string{"/entrypoint", entrypointBin},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}
}

type entrypointArgs struct {
	Args       []string `json:"args"`
	ProcessLog string   `json:"process_log"`
	MarkerFile string   `json:"marker_file"`
}

func getEnvVar(cmd, args []string) (string, error) {
	entrypointArgs := entrypointArgs{
		Args:       append(cmd, args...),
		ProcessLog: "/tools/process-log.txt",
		MarkerFile: "/tools/marker-file.txt",
	}
	j, err := json.Marshal(entrypointArgs)
	if err != nil {
		return "", fmt.Errorf("couldn't marshal arguments %q for entrypoint env var: %s", entrypointArgs, err)
	}
	return string(j), nil
}

// TODO: add more test cases after all, e.g. with existing env
// var and volume mounts

// AddEntrypoint will modify each of the steps/containers such that
// the binary being run is no longer the one specified by the Command
// and the Args, but is instead the entrypoint binary, which will
// itself invoke the Command and Args, but also capture logs.
// TODO: This will not work when a step uses an image that has its
// own entrypoint, i.e. `Command` is a required field. In later iterations
// we can update the controller to inspect the image's `Entrypoint`
// and use that if required.
func AddEntrypoint(steps []corev1.Container) error {
	for i := range steps {
		step := &steps[i]
		e, err := getEnvVar(step.Command, step.Args)
		if err != nil {
			return fmt.Errorf("couldn't get env var for entrypoint: %s", err)
		}
		step.Command = []string{entrypointBin}
		step.Args = []string{}

		step.Env = append(step.Env, corev1.EnvVar{
			Name:  entrypointJSONConfigEnvVar,
			Value: e,
		})
		step.VolumeMounts = append(step.VolumeMounts, toolsMount)
	}
	return nil
}
