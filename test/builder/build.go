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

package builder

import (
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// BuildSpecOp is an operation which modify a BuildSpec struct.
type BuildSpecOp func(*buildv1alpha1.BuildSpec)

// SourceSpecOp is an operation which modify a SourceSpec struct.
type SourceSpecOp func(*buildv1alpha1.SourceSpec)

// ContainerOp is an operation which modify a Container struct.
type ContainerOp func(*corev1.Container)

// BuildSpec creates a BuildSpec with default values.
// Any number of BuildSpec modifier can be passed to transform it.
func BuildSpec(ops ...BuildSpecOp) buildv1alpha1.BuildSpec {
	buildSpec := &buildv1alpha1.BuildSpec{}
	for _, op := range ops {
		op(buildSpec)
	}
	return *buildSpec
}

// BuildServiceAccountName sets the service account to the BuildSpec.
func BuildServiceAccountName(sa string) BuildSpecOp {
	return func(spec *buildv1alpha1.BuildSpec) {
		spec.ServiceAccountName = sa
	}
}

// BuildSource adds a SourceSpec, with specified name, to the BuildSpec.
// Any number of SourceSpec modifier can be passed to transform it.
func BuildSource(name string, ops ...SourceSpecOp) BuildSpecOp {
	return func(spec *buildv1alpha1.BuildSpec) {
		sourceSpec := &buildv1alpha1.SourceSpec{
			Name: name,
		}
		for _, op := range ops {
			op(sourceSpec)
		}
		spec.Sources = append(spec.Sources, *sourceSpec)
	}
}

// BuildSourceGit set the GitSourceSpec, with specified url and revision, to the SourceSpec.
func BuildSourceGit(url, revision string) SourceSpecOp {
	return func(spec *buildv1alpha1.SourceSpec) {
		spec.Git = &buildv1alpha1.GitSourceSpec{
			Url:      url,
			Revision: revision,
		}
	}
}

// BuildStep adds a step, with the specified name and image, to the BuildSpec.
// Any number of Container modifier can be passed to transform it.
func BuildStep(name, image string, ops ...func(*corev1.Container)) BuildSpecOp {
	return func(spec *buildv1alpha1.BuildSpec) {
		c := &corev1.Container{
			Name:  name,
			Image: image,
			Args:  []string{},
		}
		for _, op := range ops {
			op(c)
		}
		spec.Steps = append(spec.Steps, *c)
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

// VolumeMount add a VolumeMount to the Container (step).
func VolumeMount(mount corev1.VolumeMount) ContainerOp {
	return func(c *corev1.Container) {
		c.VolumeMounts = append(c.VolumeMounts, mount)
	}
}

// BuildVolume adds a Volume to the BuildSpec (step).fl
func BuildVolume(volume corev1.Volume) BuildSpecOp {
	return func(spec *buildv1alpha1.BuildSpec) {
		spec.Volumes = append(spec.Volumes, volume)
	}
}

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
