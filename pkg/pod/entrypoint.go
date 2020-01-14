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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	toolsVolumeName  = "tekton-internal-tools"
	mountPoint       = "/tekton/tools"
	entrypointBinary = mountPoint + "/entrypoint"

	downwardVolumeName     = "tekton-internal-downward"
	downwardMountPoint     = "/tekton/downward"
	terminationPath        = "/tekton/termination"
	downwardMountReadyFile = "ready"
	readyAnnotation        = "tekton.dev/ready"
	readyAnnotationValue   = "READY"

	stepPrefix    = "step-"
	sidecarPrefix = "sidecar-"
)

var (
	// TODO(#1605): Generate volumeMount names, to avoid collisions.
	toolsMount = corev1.VolumeMount{
		Name:      toolsVolumeName,
		MountPath: mountPoint,
	}
	toolsVolume = corev1.Volume{
		Name:         toolsVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}

	// TODO(#1605): Signal sidecar readiness by injecting entrypoint,
	// remove dependency on Downward API.
	downwardVolume = corev1.Volume{
		Name: downwardVolumeName,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{{
					Path: downwardMountReadyFile,
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fmt.Sprintf("metadata.annotations['%s']", readyAnnotation),
					},
				}},
			},
		},
	}
	downwardMount = corev1.VolumeMount{
		Name:      downwardVolumeName,
		MountPath: downwardMountPoint,
	}
)

// orderContainers returns the specified steps, modified so that they are
// executed in order by overriding the entrypoint binary. It also returns the
// init container that places the entrypoint binary pulled from the
// entrypointImage.
//
// Containers must have Command specified; if the user didn't specify a
// command, we must have fetched the image's ENTRYPOINT before calling this
// method, using entrypoint_lookup.go.
//
// TODO(#1605): Also use entrypoint injection to order sidecar start/stop.
func orderContainers(entrypointImage string, steps []corev1.Container, results []v1alpha1.TaskResult) (corev1.Container, []corev1.Container, error) {
	initContainer := corev1.Container{
		Name:         "place-tools",
		Image:        entrypointImage,
		Command:      []string{"cp", "/ko-app/entrypoint", entrypointBinary},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}

	if len(steps) == 0 {
		return corev1.Container{}, nil, errors.New("No steps specified")
	}

	for i, s := range steps {
		var argsForEntrypoint []string
		switch i {
		case 0:
			argsForEntrypoint = []string{
				// First step waits for the Downward volume file.
				"-wait_file", filepath.Join(downwardMountPoint, downwardMountReadyFile),
				"-wait_file_content", // Wait for file contents, not just an empty file.
				// Start next step.
				"-post_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i)),
				"-termination_path", terminationPath,
			}
		default:
			// All other steps wait for previous file, write next file.
			argsForEntrypoint = []string{
				"-wait_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i-1)),
				"-post_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i)),
				"-termination_path", terminationPath,
			}
		}
		argsForEntrypoint = append(argsForEntrypoint, resultArgument(steps, results)...)

		cmd, args := s.Command, s.Args
		if len(cmd) == 0 {
			return corev1.Container{}, nil, fmt.Errorf("Step %d did not specify command", i)
		}
		if len(cmd) > 1 {
			args = append(cmd[1:], args...)
			cmd = []string{cmd[0]}
		}
		argsForEntrypoint = append(argsForEntrypoint, "-entrypoint", cmd[0], "--")
		argsForEntrypoint = append(argsForEntrypoint, args...)

		steps[i].Command = []string{entrypointBinary}
		steps[i].Args = argsForEntrypoint
		steps[i].VolumeMounts = append(steps[i].VolumeMounts, toolsMount)
		steps[i].TerminationMessagePath = terminationPath
	}
	// Mount the Downward volume into the first step container.
	steps[0].VolumeMounts = append(steps[0].VolumeMounts, downwardMount)

	return initContainer, steps, nil
}

func resultArgument(steps []corev1.Container, results []v1alpha1.TaskResult) []string {
	if len(results) == 0 {
		return nil
	}
	return []string{"-results", collectResultsName(results)}
}

func collectResultsName(results []v1alpha1.TaskResult) string {
	var resultNames []string
	for _, r := range results {
		resultNames = append(resultNames, r.Name)
	}
	return strings.Join(resultNames, ",")
}

// UpdateReady updates the Pod's annotations to signal the first step to start
// by projecting the ready annotation via the Downward API.
func UpdateReady(kubeclient kubernetes.Interface, pod corev1.Pod) error {
	newPod, err := kubeclient.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting Pod %q when updating ready annotation: %w", pod.Name, err)
	}

	// Update the Pod's "READY" annotation to signal the first step to
	// start.
	if newPod.ObjectMeta.Annotations == nil {
		newPod.ObjectMeta.Annotations = map[string]string{}
	}
	if newPod.ObjectMeta.Annotations[readyAnnotation] != readyAnnotationValue {
		newPod.ObjectMeta.Annotations[readyAnnotation] = readyAnnotationValue
		if _, err := kubeclient.CoreV1().Pods(newPod.Namespace).Update(newPod); err != nil {
			return fmt.Errorf("error adding ready annotation to Pod %q: %w", pod.Name, err)
		}
	}
	return nil
}

// StopSidecars updates sidecar containers in the Pod to a nop image, which
// exits successfully immediately.
func StopSidecars(nopImage string, kubeclient kubernetes.Interface, pod corev1.Pod) error {
	newPod, err := kubeclient.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting Pod %q when stopping sidecars: %w", pod.Name, err)
	}

	updated := false
	if newPod.Status.Phase == corev1.PodRunning {
		for _, s := range newPod.Status.ContainerStatuses {
			// Stop any running container that isn't a step.
			// An injected sidecar container might not have the
			// "sidecar-" prefix, so we can't just look for that
			// prefix.
			if !isContainerStep(s.Name) && s.State.Running != nil {
				for j, c := range newPod.Spec.Containers {
					if c.Name == s.Name && c.Image != nopImage {
						updated = true
						newPod.Spec.Containers[j].Image = nopImage
					}
				}
			}
		}
	}
	if updated {
		if _, err := kubeclient.CoreV1().Pods(newPod.Namespace).Update(newPod); err != nil {
			return fmt.Errorf("error adding ready annotation to Pod %q: %w", pod.Name, err)
		}
	}
	return nil
}

// isContainerStep returns true if the container name indicates that it
// represents a step.
func isContainerStep(name string) bool { return strings.HasPrefix(name, stepPrefix) }

// isContainerSidecar returns true if the container name indicates that it
// represents a sidecar.
func isContainerSidecar(name string) bool { return strings.HasPrefix(name, sidecarPrefix) }

// trimStepPrefix returns the container name, stripped of its step prefix.
func trimStepPrefix(name string) string { return strings.TrimPrefix(name, stepPrefix) }

// trimSidecarPrefix returns the container name, stripped of its sidecar
// prefix.
func trimSidecarPrefix(name string) string { return strings.TrimPrefix(name, sidecarPrefix) }
