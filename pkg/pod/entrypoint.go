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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	binVolumeName    = "tekton-internal-bin"
	binDir           = "/tekton/bin"
	entrypointBinary = binDir + "/entrypoint"

	runVolumeName = "tekton-internal-run"

	// RunDir is the directory that contains runtime variable data for TaskRuns.
	// This includes files for handling container ordering, exit status codes, and more.
	// See [https://github.com/tektoncd/pipeline/blob/main/docs/developers/taskruns.md#tekton]
	// for more details.
	RunDir = "/tekton/run"

	downwardVolumeName     = "tekton-internal-downward"
	downwardMountPoint     = "/tekton/downward"
	terminationPath        = "/tekton/termination"
	downwardMountReadyFile = "ready"
	readyAnnotation        = "tekton.dev/ready"
	readyAnnotationValue   = "READY"

	stepPrefix    = "step-"
	sidecarPrefix = "sidecar-"

	downwardMountCancelFile = "cancel"
	cancelAnnotation        = "tekton.dev/cancel"
	cancelAnnotationValue   = "CANCEL"
)

var (
	// TODO(#1605): Generate volumeMount names, to avoid collisions.
	binMount = corev1.VolumeMount{
		Name:      binVolumeName,
		MountPath: binDir,
	}
	binROMount = corev1.VolumeMount{
		Name:      binVolumeName,
		MountPath: binDir,
		ReadOnly:  true,
	}
	binVolume = corev1.Volume{
		Name:         binVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	internalStepsMount = corev1.VolumeMount{
		Name:      "tekton-internal-steps",
		MountPath: pipeline.StepsDir,
	}

	downwardCancelVolumeItem = corev1.DownwardAPIVolumeFile{
		Path: downwardMountCancelFile,
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: fmt.Sprintf("metadata.annotations['%s']", cancelAnnotation),
		},
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
		// Marking this volume mount readonly is technically redundant,
		// since the volume itself is readonly, but including for completeness.
		ReadOnly: true,
	}
	// DownwardMountCancelFile is cancellation file mount to step, entrypoint will check this file to cancel the step.
	DownwardMountCancelFile = filepath.Join(downwardMountPoint, downwardMountCancelFile)
)

// orderContainers returns the specified steps, modified so that they are
// executed in order by overriding the entrypoint binary.
//
// Containers must have Command specified; if the user didn't specify a
// command, we must have fetched the image's ENTRYPOINT before calling this
// method, using entrypoint_lookup.go.
// Additionally, Step timeouts are added as entrypoint flag.
func orderContainers(ctx context.Context, commonExtraEntrypointArgs []string, steps []corev1.Container, taskSpec *v1.TaskSpec, breakpointConfig *v1.TaskRunDebug, waitForReadyAnnotation, enableKeepPodOnCancel bool) ([]corev1.Container, error) {
	if len(steps) == 0 {
		return nil, errors.New("no steps specified")
	}

	for i, s := range steps {
		var argsForEntrypoint = []string{}
		idx := strconv.Itoa(i)
		if i == 0 {
			if waitForReadyAnnotation {
				argsForEntrypoint = append(argsForEntrypoint,
					// First step waits for the Downward volume file.
					"-wait_file", filepath.Join(downwardMountPoint, downwardMountReadyFile),
					"-wait_file_content", // Wait for file contents, not just an empty file.
				)
			}
		} else { // Not the first step - wait for previous
			argsForEntrypoint = append(argsForEntrypoint, "-wait_file", filepath.Join(RunDir, strconv.Itoa(i-1), "out"))
		}
		argsForEntrypoint = append(argsForEntrypoint,
			// Start next step.
			"-post_file", filepath.Join(RunDir, idx, "out"),
			"-termination_path", terminationPath,
			"-step_metadata_dir", filepath.Join(RunDir, idx, "status"),
		)

		argsForEntrypoint = append(argsForEntrypoint, commonExtraEntrypointArgs...)
		if taskSpec != nil {
			if taskSpec.Steps != nil && len(taskSpec.Steps) >= i+1 {
				if taskSpec.Steps[i].OnError != "" {
					if taskSpec.Steps[i].OnError != v1.Continue && taskSpec.Steps[i].OnError != v1.StopAndFail {
						return nil, fmt.Errorf("task step onError must be either \"%s\" or \"%s\" but it is set to an invalid value \"%s\"",
							v1.Continue, v1.StopAndFail, taskSpec.Steps[i].OnError)
					}
					argsForEntrypoint = append(argsForEntrypoint, "-on_error", string(taskSpec.Steps[i].OnError))
				}
				if taskSpec.Steps[i].Timeout != nil {
					argsForEntrypoint = append(argsForEntrypoint, "-timeout", taskSpec.Steps[i].Timeout.Duration.String())
				}
				if taskSpec.Steps[i].StdoutConfig != nil {
					argsForEntrypoint = append(argsForEntrypoint, "-stdout_path", taskSpec.Steps[i].StdoutConfig.Path)
				}
				if taskSpec.Steps[i].StderrConfig != nil {
					argsForEntrypoint = append(argsForEntrypoint, "-stderr_path", taskSpec.Steps[i].StderrConfig.Path)
				}
				// add step results
				stepResultArgs := stepResultArgument(taskSpec.Steps[i].Results)

				argsForEntrypoint = append(argsForEntrypoint, stepResultArgs...)
				if len(taskSpec.Steps[i].When) > 0 {
					// marshal and pass to the entrypoint and unmarshal it there.
					marshal, err := json.Marshal(taskSpec.Steps[i].When)

					if err != nil {
						return nil, fmt.Errorf("faile to resolve when %w", err)
					}
					argsForEntrypoint = append(argsForEntrypoint, "--when_expressions", string(marshal))
				}
			}
			argsForEntrypoint = append(argsForEntrypoint, resultArgument(steps, taskSpec.Results)...)
		}

		if breakpointConfig != nil && breakpointConfig.NeedsDebugOnFailure() {
			argsForEntrypoint = append(argsForEntrypoint, "-breakpoint_on_failure")
		}
		if breakpointConfig != nil && breakpointConfig.NeedsDebugBeforeStep(s.Name) {
			argsForEntrypoint = append(argsForEntrypoint, "-debug_before_step")
		}

		cmd, args := s.Command, s.Args
		if len(cmd) > 0 {
			argsForEntrypoint = append(argsForEntrypoint, "-entrypoint", cmd[0])
		}
		if len(cmd) > 1 {
			args = append(cmd[1:], args...)
		}
		argsForEntrypoint = append(argsForEntrypoint, "--")
		argsForEntrypoint = append(argsForEntrypoint, args...)

		steps[i].Command = []string{entrypointBinary}
		steps[i].Args = argsForEntrypoint
		steps[i].TerminationMessagePath = terminationPath
		if (i == 0 && waitForReadyAnnotation) || enableKeepPodOnCancel {
			// Mount the Downward volume into the first step container.
			// if enableKeepPodOnCancel is true, mount the Downward volume into all the steps.
			steps[i].VolumeMounts = append(steps[i].VolumeMounts, downwardMount)
		}
	}

	return steps, nil
}

// stepResultArgument creates the cli arguments for step results to the entrypointer.
func stepResultArgument(stepResults []v1.StepResult) []string {
	if len(stepResults) == 0 {
		return nil
	}
	stepResultNames := []string{}
	for _, r := range stepResults {
		stepResultNames = append(stepResultNames, r.Name)
	}
	return []string{"-step_results", strings.Join(stepResultNames, ",")}
}

func resultArgument(steps []corev1.Container, results []v1.TaskResult) []string {
	if len(results) == 0 {
		return nil
	}
	return []string{"-results", collectResultsName(results)}
}

func collectResultsName(results []v1.TaskResult) string {
	var resultNames []string
	for _, r := range results {
		if r.Value == nil {
			resultNames = append(resultNames, r.Name)
		}
	}
	return strings.Join(resultNames, ",")
}

var replaceReadyPatchBytes, replaceCancelPatchBytes []byte

func init() {
	// https://stackoverflow.com/questions/55573724/create-a-patch-to-add-a-kubernetes-annotation
	readyAnnotationPath := "/metadata/annotations/" + strings.Replace(readyAnnotation, "/", "~1", 1)
	var err error
	replaceReadyPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{{
		Operation: "replace",
		Path:      readyAnnotationPath,
		Value:     readyAnnotationValue,
	}})
	if err != nil {
		log.Fatalf("failed to marshal replace ready patch bytes: %v", err)
	}

	cancelAnnotationPath := "/metadata/annotations/" + strings.Replace(cancelAnnotation, "/", "~1", 1)
	replaceCancelPatchBytes, err = json.Marshal([]jsonpatch.JsonPatchOperation{{
		Operation: "replace",
		Path:      cancelAnnotationPath,
		Value:     cancelAnnotationValue,
	}})
	if err != nil {
		log.Fatalf("failed to marshal replace cancel patch bytes: %v", err)
	}
}

// buildSidecarStopPatch creates a JSON Patch to replace sidecar container images with nop image
func buildSidecarStopPatch(pod *corev1.Pod, nopImage string, ctx context.Context) ([]byte, error) {
	var patchOps []jsonpatch.JsonPatchOperation

	// Iterate over container statuses to find running sidecars
	for _, s := range pod.Status.ContainerStatuses {
		// If the results-from is set to sidecar logs,
		// a sidecar container with name `sidecar-log-results` is injected by the reconciler.
		// Do not kill this sidecar. Let it exit gracefully.
		if config.FromContextOrDefaults(ctx).FeatureFlags.ResultExtractionMethod == config.ResultExtractionMethodSidecarLogs && s.Name == pipeline.ReservedResultsSidecarContainerName {
			continue
		}
		// Stop any running container that isn't a step.
		// An injected sidecar container might not have the
		// "sidecar-" prefix, so we can't just look for that prefix.
		if !IsContainerStep(s.Name) && s.State.Running != nil {
			// Find the corresponding container in the spec by name to get the correct index
			for i, c := range pod.Spec.Containers {
				if c.Name == s.Name && c.Image != nopImage {
					patchOps = append(patchOps, jsonpatch.JsonPatchOperation{
						Operation: "replace",
						Path:      fmt.Sprintf("/spec/containers/%d/image", i),
						Value:     nopImage,
					})
					break
				}
			}
		}
	}

	if len(patchOps) == 0 {
		return nil, nil
	}

	return json.Marshal(patchOps)
}

// CancelPod cancels the pod
func CancelPod(ctx context.Context, kubeClient kubernetes.Interface, namespace, podName string) error {
	// PATCH the Pod's annotations to replace the cancel annotation with the
	// "CANCEL" value, to signal the pod to be cancelled.
	_, err := kubeClient.CoreV1().Pods(namespace).Patch(ctx, podName, types.JSONPatchType, replaceCancelPatchBytes, metav1.PatchOptions{})
	return err
}

// UpdateReady updates the Pod's annotations to signal the first step to start
// by projecting the ready annotation via the Downward API.
func UpdateReady(ctx context.Context, kubeclient kubernetes.Interface, pod corev1.Pod) error {
	// Don't PATCH if the annotation is already Ready.
	if pod.Annotations[readyAnnotation] == readyAnnotationValue {
		return nil
	}

	// PATCH the Pod's annotations to replace the ready annotation with the
	// "READY" value, to signal the first step to start.
	_, err := kubeclient.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.JSONPatchType, replaceReadyPatchBytes, metav1.PatchOptions{})
	return err
}

// StopSidecars updates sidecar containers in the Pod to a nop image, which
// exits successfully immediately.
func StopSidecars(ctx context.Context, nopImage string, kubeclient kubernetes.Interface, namespace, name string) (*corev1.Pod, error) {
	pod, err := kubeclient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// return NotFound as-is, since the K8s error checks don't handle wrapping.
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting Pod %q when stopping sidecars: %w", name, err)
	}

	// Only attempt to stop sidecars if pod is running
	if pod.Status.Phase != corev1.PodRunning {
		return pod, nil
	}

	// Build JSON Patch operations to replace sidecar images
	patchBytes, err := buildSidecarStopPatch(pod, nopImage, ctx)
	if err != nil {
		return nil, fmt.Errorf("error building patch for stopping sidecars of Pod %q: %w", name, err)
	}

	// If no sidecars need to be stopped, return early
	if patchBytes == nil {
		return pod, nil
	}

	// PATCH the Pod's container images to stop sidecars, using the same pattern as UpdateReady and CancelPod
	patchedPod, err := kubeclient.CoreV1().Pods(namespace).Patch(ctx, name, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("error stopping sidecars of Pod %q: %w", name, err)
	}

	return patchedPod, nil
}

// IsSidecarStatusRunning determines if any SidecarStatus on a TaskRun
// is still running.
func IsSidecarStatusRunning(tr *v1.TaskRun) bool {
	for _, sidecar := range tr.Status.Sidecars {
		if sidecar.Terminated == nil {
			return true
		}
	}

	return false
}

// IsContainerStep returns true if the container name indicates that it
// represents a step.
func IsContainerStep(name string) bool { return strings.HasPrefix(name, stepPrefix) }

// IsContainerSidecar returns true if the container name indicates that it
// represents a sidecar.
func IsContainerSidecar(name string) bool { return strings.HasPrefix(name, sidecarPrefix) }

// TrimStepPrefix returns the container name, stripped of its step prefix.
func TrimStepPrefix(name string) string { return strings.TrimPrefix(name, stepPrefix) }

// TrimSidecarPrefix returns the container name, stripped of its sidecar
// prefix.
func TrimSidecarPrefix(name string) string { return strings.TrimPrefix(name, sidecarPrefix) }

// StepName returns the step name after adding "step-" prefix to the actual step name or
// returns "step-unnamed-<step-index>" if not specified
func StepName(name string, i int) string {
	if name != "" {
		return GetContainerName(name)
	}
	return fmt.Sprintf("%sunnamed-%d", stepPrefix, i)
}

// GetContainerName prefixes the input name with "step-"
func GetContainerName(name string) string {
	return fmt.Sprintf("%s%s", stepPrefix, name)
}
