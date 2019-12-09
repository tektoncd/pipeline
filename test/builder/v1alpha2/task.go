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

package v1alpha2

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskOp is an operation which modify a Task struct.
type TaskOp func(*v1alpha2.Task)

// TaskSpeOp is an operation which modify a TaskSpec struct.
type TaskSpecOp func(*v1alpha2.TaskSpec)

// TaskResourceOp is an operation which modify a TaskResource struct.
type TaskResourceOp func(*v1alpha2.TaskResource)

// VolumeOp is an operation which modify a Volume struct.
type VolumeOp func(*corev1.Volume)

var (
	trueB = true
)

// Task creates a Task with default values.
// Any number of Task modifier can be passed to transform it.
func Task(name, namespace string, ops ...TaskOp) *v1alpha2.Task {
	t := &v1alpha2.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	for _, op := range ops {
		op(t)
	}

	return t
}

// TaskSpec sets the specified spec of the task.
// Any number of TaskSpec modifier can be passed to create/modify it.
func TaskSpec(ops ...TaskSpecOp) TaskOp {
	return func(t *v1alpha2.Task) {
		spec := &t.Spec
		for _, op := range ops {
			op(spec)
		}
		t.Spec = *spec
	}
}

// Step adds a step with the specified name and image to the TaskSpec.
// Any number of Container modifier can be passed to transform it.
func Step(name, image string, ops ...StepOp) TaskSpecOp {
	return func(spec *v1alpha2.TaskSpec) {
		if spec.Steps == nil {
			spec.Steps = []v1alpha2.Step{}
		}
		step := v1alpha2.Step{Container: corev1.Container{
			Name:  name,
			Image: image,
		}}
		for _, op := range ops {
			op(&step)
		}
		spec.Steps = append(spec.Steps, step)
	}
}

func Sidecar(name, image string, ops ...ContainerOp) TaskSpecOp {
	return func(spec *v1alpha2.TaskSpec) {
		c := corev1.Container{
			Name:  name,
			Image: image,
		}
		for _, op := range ops {
			op(&c)
		}
		spec.Sidecars = append(spec.Sidecars, c)
	}
}

// TaskStepTemplate adds a base container for all steps in the task.
func TaskStepTemplate(ops ...ContainerOp) TaskSpecOp {
	return func(spec *v1alpha2.TaskSpec) {
		base := &corev1.Container{}
		for _, op := range ops {
			op(base)
		}
		spec.StepTemplate = base
	}
}

// TaskVolume adds a volume with specified name to the TaskSpec.
// Any number of Volume modifier can be passed to transform it.
func TaskVolume(name string, ops ...VolumeOp) TaskSpecOp {
	return func(spec *v1alpha2.TaskSpec) {
		v := &corev1.Volume{Name: name}
		for _, op := range ops {
			op(v)
		}
		spec.Volumes = append(spec.Volumes, *v)
	}
}

// VolumeSource sets the VolumeSource to the Volume.
func VolumeSource(s corev1.VolumeSource) VolumeOp {
	return func(v *corev1.Volume) {
		v.VolumeSource = s
	}
}

func ResourceTargetPath(path string) TaskResourceOp {
	return func(r *v1alpha2.TaskResource) {
		r.TargetPath = path
	}
}

func Controller(o *metav1.OwnerReference) {
	o.Controller = &trueB
}

func BlockOwnerDeletion(o *metav1.OwnerReference) {
	o.BlockOwnerDeletion = &trueB
}
