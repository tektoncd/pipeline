package pod

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	toolsVolumeName  = "tools"
	mountPoint       = "/builder/tools"
	entrypointBinary = mountPoint + "/entrypoint"

	downwardVolumeName     = "downward"
	downwardMountPoint     = "/builder/downward"
	downwardMountReadyFile = "ready"
	ReadyAnnotation        = "tekton.dev/ready"
	ReadyAnnotationValue   = "READY"

	StepPrefix    = "step-"
	SidecarPrefix = "sidecar-"
)

var (
	// TODO(#1605): Generate volumeMount names, to avoid collisions.
	// TODO(#1605): Unexport these vars when Pod conversion is entirely within
	// this package.
	ToolsMount = corev1.VolumeMount{
		Name:      toolsVolumeName,
		MountPath: mountPoint,
	}
	ToolsVolume = corev1.Volume{
		Name:         toolsVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}

	// TODO(#1605): Signal sidecar readiness by injecting entrypoint,
	// remove dependency on Downward API.
	DownwardVolume = corev1.Volume{
		Name: downwardVolumeName,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{{
					Path: downwardMountReadyFile,
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: fmt.Sprintf("metadata.annotations['%s']", ReadyAnnotation),
					},
				}},
			},
		},
	}
	DownwardMount = corev1.VolumeMount{
		Name:      downwardVolumeName,
		MountPath: downwardMountPoint,
	}
)

// OrderContainers returns the specified steps, modified so that they are
// executed in order by overriding the entrypoint binary. It also returns the
// init container that places the entrypoint binary pulled from the
// entrypointImage.
//
// Containers must have Command specified; if the user didn't specify a
// command, we must have fetched the image's ENTRYPOINT before calling this
// method, using entrypoint_lookup.go.
//
// TODO(#1605): Also use entrypoint injection to order sidecar start/stop.
func OrderContainers(entrypointImage string, steps []corev1.Container) (corev1.Container, []corev1.Container, error) {
	toolsInit := corev1.Container{
		Name:         "place-tools",
		Image:        entrypointImage,
		Command:      []string{"cp", "/ko-app/entrypoint", entrypointBinary},
		VolumeMounts: []corev1.VolumeMount{ToolsMount},
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
			}
		default:
			// All other steps wait for previous file, write next file.
			argsForEntrypoint = []string{
				"-wait_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i-1)),
				"-post_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i)),
			}
		}

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
		steps[i].VolumeMounts = append(steps[i].VolumeMounts, ToolsMount)
	}
	// Mount the Downward volume into the first step container.
	steps[0].VolumeMounts = append(steps[0].VolumeMounts, DownwardMount)

	return toolsInit, steps, nil
}

// UpdateReady updates the Pod's annotations to signal the first step to start
// by projecting the ready annotation via the Downward API.
func UpdateReady(kubeclient kubernetes.Interface, pod corev1.Pod) error {
	newPod, err := kubeclient.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error getting Pod %q when updating ready annotation: %w", pod.Name, err)
	}

	// Update the Pod's "READY" annotation to signal the first step to
	// start.
	if newPod.ObjectMeta.Annotations == nil {
		newPod.ObjectMeta.Annotations = map[string]string{}
	}
	if newPod.ObjectMeta.Annotations[ReadyAnnotation] != ReadyAnnotationValue {
		newPod.ObjectMeta.Annotations[ReadyAnnotation] = ReadyAnnotationValue
		if _, err := kubeclient.CoreV1().Pods(newPod.Namespace).Update(newPod); err != nil {
			return fmt.Errorf("Error adding ready annotation to Pod %q: %w", pod.Name, err)
		}
	}
	return nil
}

// StopSidecars updates sidecar containers in the Pod to a nop image, which
// exits successfully immediately.
func StopSidecars(nopImage string, kubeclient kubernetes.Interface, pod corev1.Pod) error {
	newPod, err := kubeclient.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error getting Pod %q when stopping sidecars: %w", pod.Name, err)
	}

	updated := false
	if newPod.Status.Phase == corev1.PodRunning {
		for _, s := range newPod.Status.ContainerStatuses {
			if IsContainerSidecar(s.Name) && s.State.Running != nil {
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
			return fmt.Errorf("Error adding ready annotation to Pod %q: %w", pod.Name, err)
		}
	}
	return nil
}

func IsContainerStep(name string) bool    { return strings.HasPrefix(name, StepPrefix) }
func IsContainerSidecar(name string) bool { return strings.HasPrefix(name, SidecarPrefix) }

func TrimStepPrefix(name string) string    { return strings.TrimPrefix(name, StepPrefix) }
func TrimSidecarPrefix(name string) string { return strings.TrimPrefix(name, SidecarPrefix) }
