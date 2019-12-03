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

package builder

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodOp is an operation which modifies a Pod struct.
type PodOp func(*corev1.Pod)

// PodSpecOp is an operation which modifies a PodSpec struct.
type PodSpecOp func(*corev1.PodSpec)

// PodStatusOp is an operation which modifies a PodStatus struct.
type PodStatusOp func(status *corev1.PodStatus)

// Pod creates a Pod with default values.
// Any number of Pod modifiers can be passed to transform it.
func Pod(name, namespace string, ops ...PodOp) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: map[string]string{},
		},
	}
	for _, op := range ops {
		op(pod)
	}
	return pod
}

// PodLabel adds an annotation to the Pod.
func PodAnnotation(key, value string) PodOp {
	return func(pod *corev1.Pod) {
		if pod.ObjectMeta.Annotations == nil {
			pod.ObjectMeta.Annotations = map[string]string{}
		}
		pod.ObjectMeta.Annotations[key] = value
	}
}

// PodLabel adds a label to the Pod.
func PodLabel(key, value string) PodOp {
	return func(pod *corev1.Pod) {
		if pod.ObjectMeta.Labels == nil {
			pod.ObjectMeta.Labels = map[string]string{}
		}
		pod.ObjectMeta.Labels[key] = value
	}
}

// PodOwnerReference adds an OwnerReference, with specified kind and name, to the Pod.
func PodOwnerReference(kind, name string, ops ...OwnerReferenceOp) PodOp {
	trueB := true
	return func(pod *corev1.Pod) {
		o := &metav1.OwnerReference{
			Kind:               kind,
			Name:               name,
			Controller:         &trueB,
			BlockOwnerDeletion: &trueB,
		}
		for _, op := range ops {
			op(o)
		}
		pod.ObjectMeta.OwnerReferences = append(pod.ObjectMeta.OwnerReferences, *o)
	}
}

// PodSpec creates a PodSpec with default values.
// Any number of PodSpec modifiers can be passed to transform it.
func PodSpec(ops ...PodSpecOp) PodOp {
	return func(pod *corev1.Pod) {
		podSpec := &pod.Spec
		for _, op := range ops {
			op(podSpec)
		}
		pod.Spec = *podSpec
	}
}

// PodRestartPolicy sets the restart policy on the PodSpec.
func PodRestartPolicy(restartPolicy corev1.RestartPolicy) PodSpecOp {
	return func(spec *corev1.PodSpec) {
		spec.RestartPolicy = restartPolicy
	}
}

// PodServiceAccountName sets the service account on the PodSpec.
func PodServiceAccountName(sa string) PodSpecOp {
	return func(spec *corev1.PodSpec) {
		spec.ServiceAccountName = sa
	}
}

// PodContainer adds a Container, with the specified name and image, to the PodSpec.
// Any number of Container modifiers can be passed to transform it.
func PodContainer(name, image string, ops ...ContainerOp) PodSpecOp {
	return func(spec *corev1.PodSpec) {
		c := &corev1.Container{
			Name:  name,
			Image: image,
			// By default, containers request zero resources. Ops
			// can override this.
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("0"),
					corev1.ResourceMemory:           resource.MustParse("0"),
					corev1.ResourceEphemeralStorage: resource.MustParse("0"),
				},
			},
		}
		for _, op := range ops {
			op(c)
		}
		spec.Containers = append(spec.Containers, *c)
	}
}

// PodInitContainer adds an InitContainer, with the specified name and image, to the PodSpec.
// Any number of Container modifiers can be passed to transform it.
func PodInitContainer(name, image string, ops ...ContainerOp) PodSpecOp {
	return func(spec *corev1.PodSpec) {
		c := &corev1.Container{
			Name:  name,
			Image: image,
			Args:  []string{},
		}
		for _, op := range ops {
			op(c)
		}
		spec.InitContainers = append(spec.InitContainers, *c)
	}
}

// PodVolume sets the Volumes on the PodSpec.
func PodVolumes(volumes ...corev1.Volume) PodSpecOp {
	return func(spec *corev1.PodSpec) {
		spec.Volumes = volumes
	}
}

// PodCreationTimestamp sets the creation time of the pod
func PodCreationTimestamp(t time.Time) PodOp {
	return func(p *corev1.Pod) {
		p.CreationTimestamp = metav1.Time{Time: t}
	}
}

// PodStatus creates a PodStatus with default values.
// Any number of PodStatus modifiers can be passed to transform it.
func PodStatus(ops ...PodStatusOp) PodOp {
	return func(pod *corev1.Pod) {
		podStatus := &pod.Status
		for _, op := range ops {
			op(podStatus)
		}
		pod.Status = *podStatus
	}
}

func PodStatusConditions(cond corev1.PodCondition) PodStatusOp {
	return func(status *corev1.PodStatus) {
		status.Conditions = append(status.Conditions, cond)
	}
}
