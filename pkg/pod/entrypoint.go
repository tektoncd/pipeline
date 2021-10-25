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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/pod/script"
	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	binVolumeName    = "tekton-internal-bin" // FIXME(vdemeester) remove duplication with script package
	binDir           = "/tekton/bin"         // FIXME(vdemeester) remove duplication with script package
	entrypointBinary = binDir + "/entrypoint"

	runVolumeName = "tekton-internal-run"
	runDir        = "/tekton/run" // FIXME(vdemeester) remove duplication with script package

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
	// FIXME(vdemeester) remove duplication with script package
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
)

// orderContainers returns the specified steps, modified so that they are
// executed in order by overriding the entrypoint binary.
//
// Containers must have Command specified; if the user didn't specify a
// command, we must have fetched the image's ENTRYPOINT before calling this
// method, using entrypoint_lookup.go.
// Additionally, Step timeouts are added as entrypoint flag.
func orderContainers(commonExtraEntrypointArgs []string, steps []corev1.Container, taskSpec *v1beta1.TaskSpec, breakpointConfig *v1beta1.TaskRunDebug) ([]corev1.Container, error) {
	if len(steps) == 0 {
		return nil, errors.New("No steps specified")
	}

	for i, s := range steps {
		var argsForEntrypoint []string
		name := StepName(steps[i].Name, i)
		switch i {
		case 0:
			argsForEntrypoint = []string{
				// First step waits for the Downward volume file.
				"-wait_file", filepath.Join(downwardMountPoint, downwardMountReadyFile),
				"-wait_file_content", // Wait for file contents, not just an empty file.
				// Start next step.
				"-post_file", filepath.Join(runDir, strconv.Itoa(i), "out"),
				"-termination_path", terminationPath,
				"-step_metadata_dir", filepath.Join(pipeline.StepsDir, name),
				"-step_metadata_dir_link", filepath.Join(pipeline.StepsDir, fmt.Sprintf("%d", i)),
			}
		default:
			// All other steps wait for previous file, write next file.
			argsForEntrypoint = []string{
				"-wait_file", filepath.Join(runDir, strconv.Itoa(i-1), "out"),
				"-post_file", filepath.Join(runDir, strconv.Itoa(i), "out"),
				"-termination_path", terminationPath,
				"-step_metadata_dir", filepath.Join(pipeline.StepsDir, name),
				"-step_metadata_dir_link", filepath.Join(pipeline.StepsDir, fmt.Sprintf("%d", i)),
			}
		}
		argsForEntrypoint = append(argsForEntrypoint, commonExtraEntrypointArgs...)
		if taskSpec != nil {
			if taskSpec.Steps != nil && len(taskSpec.Steps) >= i+1 {
				if taskSpec.Steps[i].Timeout != nil {
					argsForEntrypoint = append(argsForEntrypoint, "-timeout", taskSpec.Steps[i].Timeout.Duration.String())
				}
				if taskSpec.Steps[i].OnError != "" {
					argsForEntrypoint = append(argsForEntrypoint, "-on_error", taskSpec.Steps[i].OnError)
				}
			}
			argsForEntrypoint = append(argsForEntrypoint, resultArgument(steps, taskSpec.Results)...)
		}

		cmd, args := s.Command, s.Args
		if len(cmd) == 0 {
			return nil, fmt.Errorf("Step %d did not specify command", i)
		}
		if len(cmd) > 1 {
			args = append(cmd[1:], args...)
			cmd = []string{cmd[0]}
		}

		if breakpointConfig != nil && len(breakpointConfig.Breakpoint) > 0 {
			breakpoints := breakpointConfig.Breakpoint
			for _, b := range breakpoints {
				// TODO(TEP #0042): Add other breakpoints
				if b == script.BreakpointOnFailure {
					argsForEntrypoint = append(argsForEntrypoint, "-breakpoint_on_failure")
				}
			}
		}

		argsForEntrypoint = append(argsForEntrypoint, "-entrypoint", cmd[0], "--")
		argsForEntrypoint = append(argsForEntrypoint, args...)

		steps[i].Command = []string{entrypointBinary}
		steps[i].Args = argsForEntrypoint
		steps[i].TerminationMessagePath = terminationPath
	}
	// Mount the Downward volume into the first step container.
	steps[0].VolumeMounts = append(steps[0].VolumeMounts, downwardMount)

	return steps, nil
}

func resultArgument(steps []corev1.Container, results []v1beta1.TaskResult) []string {
	if len(results) == 0 {
		return nil
	}
	return []string{"-results", collectResultsName(results)}
}

func collectResultsName(results []v1beta1.TaskResult) string {
	var resultNames []string
	for _, r := range results {
		resultNames = append(resultNames, r.Name)
	}
	return strings.Join(resultNames, ",")
}

var replaceReadyPatchBytes []byte

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
}

// UpdateReady updates the Pod's annotations to signal the first step to start
// by projecting the ready annotation via the Downward API.
func UpdateReady(ctx context.Context, kubeclient kubernetes.Interface, pod corev1.Pod) error {
	// PATCH the Pod's annotations to replace the ready annotation with the
	// "READY" value, to signal the first step to start.
	_, err := kubeclient.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.JSONPatchType, replaceReadyPatchBytes, metav1.PatchOptions{})
	return err
}

// isContainerStep returns true if the container name indicates that it
// represents a step.
func IsContainerStep(name string) bool { return strings.HasPrefix(name, stepPrefix) }

// trimStepPrefix returns the container name, stripped of its step prefix.
func TrimStepPrefix(name string) string { return strings.TrimPrefix(name, stepPrefix) }

// StepName returns the step name after adding "step-" prefix to the actual step name or
// returns "step-unnamed-<step-index>" if not specified
func StepName(name string, i int) string {
	if name != "" {
		return fmt.Sprintf("%s%s", stepPrefix, name)
	}
	return fmt.Sprintf("%sunnamed-%d", stepPrefix, i)
}
