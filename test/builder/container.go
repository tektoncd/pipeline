/*
Copyright 2019 The Knative Authors
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

package builder

import (
	corev1 "k8s.io/api/core/v1"
)

// ContainerOp is an operation which modifies a Container struct.
type ContainerOp func(*corev1.Container)

// VolumeMountOp is an operation which modifies a VolumeMount struct.
type VolumeMountOp func(*corev1.VolumeMount)

// Command sets the command to the Container (step in this case).
func Command(args ...string) ContainerOp {
	return func(container *corev1.Container) {
		container.Command = args
	}
}

// Args sets the command arguments to the Container (step in this case).
func Args(args ...string) ContainerOp {
	return func(container *corev1.Container) {
		container.Args = args
	}
}

// EnvVar add an environment variable, with specified name and value, to the Container (step).
func EnvVar(name, value string) ContainerOp {
	return func(c *corev1.Container) {
		c.Env = append(c.Env, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}
}

// WorkingDir sets the WorkingDir on the Container.
func WorkingDir(workingDir string) ContainerOp {
	return func(c *corev1.Container) {
		c.WorkingDir = workingDir
	}
}

// VolumeMount add a VolumeMount to the Container (step).
func VolumeMount(name, mountPath string, ops ...VolumeMountOp) ContainerOp {
	return func(c *corev1.Container) {
		mount := &corev1.VolumeMount{
			Name:      name,
			MountPath: mountPath,
		}
		for _, op := range ops {
			op(mount)
		}
		c.VolumeMounts = append(c.VolumeMounts, *mount)
	}
}
