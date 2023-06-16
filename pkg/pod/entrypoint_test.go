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
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var volumeMount = corev1.VolumeMount{
	Name:      "my-mount",
	MountPath: "/mount/point",
}

func TestOrderContainers(t *testing.T) {
	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}, {
		Image:        "step-2",
		Command:      []string{"cmd1", "cmd2", "cmd3"}, // multiple cmd elements
		Args:         []string{"arg1", "arg2"},
		VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
	}, {
		Image:   "step-3",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/downward/ready",
			"-wait_file_content",
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{downwardMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-2",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/0/out",
			"-post_file", "/tekton/run/1/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/1/status",
			"-entrypoint", "cmd1", "--",
			"cmd2", "cmd3",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{volumeMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-3",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/1/out",
			"-post_file", "/tekton/run/2/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/2/status",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		TerminationMessagePath: "/tekton/termination",
	}}
	got, err := orderContainers([]string{}, steps, nil, nil, true)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestOrderContainersWithResultsSidecarLogs(t *testing.T) {
	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}, {
		Image:        "step-2",
		Command:      []string{"cmd1", "cmd2", "cmd3"}, // multiple cmd elements
		Args:         []string{"arg1", "arg2"},
		VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
	}, {
		Image:   "step-3",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/downward/ready",
			"-wait_file_content",
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-dont_send_results_to_termination_path",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{downwardMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-2",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/0/out",
			"-post_file", "/tekton/run/1/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/1/status",
			"-dont_send_results_to_termination_path",
			"-entrypoint", "cmd1", "--",
			"cmd2", "cmd3",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{volumeMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-3",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/1/out",
			"-post_file", "/tekton/run/2/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/2/status",
			"-dont_send_results_to_termination_path",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		TerminationMessagePath: "/tekton/termination",
	}}
	got, err := orderContainers([]string{"-dont_send_results_to_termination_path"}, steps, nil, nil, true)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestOrderContainersWithNoWait(t *testing.T) {
	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}, {
		Image:        "step-2",
		Command:      []string{"cmd1", "cmd2", "cmd3"}, // multiple cmd elements
		Args:         []string{"arg1", "arg2"},
		VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-2",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/0/out",
			"-post_file", "/tekton/run/1/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/1/status",
			"-entrypoint", "cmd1", "--",
			"cmd2", "cmd3",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{volumeMount},
		TerminationMessagePath: "/tekton/termination",
	}}
	got, err := orderContainers([]string{}, steps, nil, nil, false)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestOrderContainersWithDebugOnFailure(t *testing.T) {
	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/downward/ready",
			"-wait_file_content",
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-breakpoint_on_failure",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{downwardMount},
		TerminationMessagePath: "/tekton/termination",
	}}
	taskRunDebugConfig := &v1.TaskRunDebug{
		Breakpoint: []string{"onFailure"},
	}
	got, err := orderContainers([]string{}, steps, nil, taskRunDebugConfig, true)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestEntryPointResults(t *testing.T) {
	taskSpec := v1.TaskSpec{
		Results: []v1.TaskResult{{
			Name:        "sum",
			Description: "This is the sum result of the task",
		}, {
			Name:        "sub",
			Description: "This is the sub result of the task",
		}},
	}

	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}, {
		Image:        "step-2",
		Command:      []string{"cmd1", "cmd2", "cmd3"}, // multiple cmd elements
		Args:         []string{"arg1", "arg2"},
		VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
	}, {
		Image:   "step-3",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/downward/ready",
			"-wait_file_content",
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-results", "sum,sub",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{downwardMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-2",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/0/out",
			"-post_file", "/tekton/run/1/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/1/status",
			"-results", "sum,sub",
			"-entrypoint", "cmd1", "--",
			"cmd2", "cmd3",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{volumeMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-3",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/1/out",
			"-post_file", "/tekton/run/2/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/2/status",
			"-results", "sum,sub",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		TerminationMessagePath: "/tekton/termination",
	}}
	got, err := orderContainers([]string{}, steps, &taskSpec, nil, true)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestEntryPointResultsSingleStep(t *testing.T) {
	taskSpec := v1.TaskSpec{
		Results: []v1.TaskResult{{
			Name:        "sum",
			Description: "This is the sum result of the task",
		}, {
			Name:        "sub",
			Description: "This is the sub result of the task",
		}},
	}

	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/downward/ready",
			"-wait_file_content",
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-results", "sum,sub",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{downwardMount},
		TerminationMessagePath: "/tekton/termination",
	}}
	got, err := orderContainers([]string{}, steps, &taskSpec, nil, true)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}
func TestEntryPointSingleResultsSingleStep(t *testing.T) {
	taskSpec := v1.TaskSpec{
		Results: []v1.TaskResult{{
			Name:        "sum",
			Description: "This is the sum result of the task",
		}},
	}

	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/downward/ready",
			"-wait_file_content",
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-results", "sum",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{downwardMount},
		TerminationMessagePath: "/tekton/termination",
	}}
	got, err := orderContainers([]string{}, steps, &taskSpec, nil, true)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestEntryPointOnError(t *testing.T) {
	steps := []corev1.Container{{
		Name:    "failing-step",
		Image:   "step-1",
		Command: []string{"cmd"},
	}, {
		Name:    "passing-step",
		Image:   "step-2",
		Command: []string{"cmd"},
	}}

	for _, tc := range []struct {
		desc           string
		taskSpec       v1.TaskSpec
		wantContainers []corev1.Container
		err            error
	}{{
		taskSpec: v1.TaskSpec{
			Steps: []v1.Step{{
				OnError: v1.Continue,
			}, {
				OnError: v1.StopAndFail,
			}},
		},
		wantContainers: []corev1.Container{{
			Name:    "failing-step",
			Image:   "step-1",
			Command: []string{entrypointBinary},
			Args: []string{
				"-wait_file", "/tekton/downward/ready",
				"-wait_file_content",
				"-post_file", "/tekton/run/0/out",
				"-termination_path", "/tekton/termination",
				"-step_metadata_dir", "/tekton/run/0/status",
				"-on_error", "continue",
				"-entrypoint", "cmd", "--",
			},
			VolumeMounts:           []corev1.VolumeMount{downwardMount},
			TerminationMessagePath: "/tekton/termination",
		}, {
			Name:    "passing-step",
			Image:   "step-2",
			Command: []string{entrypointBinary},
			Args: []string{
				"-wait_file", "/tekton/run/0/out",
				"-post_file", "/tekton/run/1/out",
				"-termination_path", "/tekton/termination",
				"-step_metadata_dir", "/tekton/run/1/status",
				"-on_error", "stopAndFail",
				"-entrypoint", "cmd", "--",
			},
			TerminationMessagePath: "/tekton/termination",
		}},
	}, {
		taskSpec: v1.TaskSpec{
			Steps: []v1.Step{{
				OnError: "invalid-on-error",
			}},
		},
		err: errors.New("task step onError must be either \"continue\" or \"stopAndFail\" but it is set to an invalid value \"invalid-on-error\""),
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := orderContainers([]string{}, steps, &tc.taskSpec, nil, true)
			if len(tc.wantContainers) == 0 {
				if err == nil {
					t.Fatalf("expected an error for an invalid value for onError but received none")
				}
				if d := cmp.Diff(tc.err.Error(), err.Error()); d != "" {
					t.Errorf("Diff %s", diff.PrintWantGot(d))
				}
			} else {
				if err != nil {
					t.Fatalf("orderContainers: %v", err)
				}
				if d := cmp.Diff(tc.wantContainers, got); d != "" {
					t.Errorf("Diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestEntryPointStepOutputConfigs(t *testing.T) {
	taskSpec := v1.TaskSpec{
		Steps: []v1.Step{{
			StdoutConfig: &v1.StepOutputConfig{
				Path: "step-1-out",
			},
		}, {
			StderrConfig: &v1.StepOutputConfig{
				Path: "step-2-err",
			},
		}, {
			StdoutConfig: &v1.StepOutputConfig{
				Path: "step-3-out",
			},
			StderrConfig: &v1.StepOutputConfig{
				Path: "step-3-err",
			},
		}},
	}

	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}, {
		Image:        "step-2",
		Command:      []string{"cmd1", "cmd2", "cmd3"}, // multiple cmd elements
		Args:         []string{"arg1", "arg2"},
		VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
	}, {
		Image:   "step-3",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/downward/ready",
			"-wait_file_content",
			"-post_file", "/tekton/run/0/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/0/status",
			"-stdout_path", "step-1-out",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{downwardMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-2",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/0/out",
			"-post_file", "/tekton/run/1/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/1/status",
			"-stderr_path", "step-2-err",
			"-entrypoint", "cmd1", "--",
			"cmd2", "cmd3",
			"arg1", "arg2",
		},
		VolumeMounts:           []corev1.VolumeMount{volumeMount},
		TerminationMessagePath: "/tekton/termination",
	}, {
		Image:   "step-3",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/tekton/run/1/out",
			"-post_file", "/tekton/run/2/out",
			"-termination_path", "/tekton/termination",
			"-step_metadata_dir", "/tekton/run/2/status",
			"-stdout_path", "step-3-out",
			"-stderr_path", "step-3-err",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		TerminationMessagePath: "/tekton/termination",
	}}
	got, err := orderContainers([]string{}, steps, &taskSpec, nil, true)
	if err != nil {
		t.Fatalf("orderContainers: %v", err)
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}

func TestUpdateReady(t *testing.T) {
	for _, c := range []struct {
		desc            string
		pod             corev1.Pod
		wantAnnotations map[string]string
		wantErr         bool
		wantPatch       bool // Whether we expect PATCH to be called on the pod.
	}{{
		desc: "Pod without any annotations fails",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "pod",
				Annotations: nil,
			},
		},
		wantErr:   true, // Nothing to replace.
		wantPatch: true,
	}, {
		desc: "Pod without ready annotation adds it",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
				Annotations: map[string]string{
					"something": "else",
				},
			},
		},
		wantAnnotations: map[string]string{
			"something":     "else",
			readyAnnotation: "READY",
		},
		wantPatch: true,
	}, {
		desc: "Pod with empty annotation value has it replaced",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
				Annotations: map[string]string{
					"something":     "else",
					readyAnnotation: "",
				},
			},
		},
		wantAnnotations: map[string]string{
			"something":     "else",
			readyAnnotation: readyAnnotationValue,
		},
		wantPatch: true,
	}, {
		desc: "Pod with other annotation value has it replaced",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
				Annotations: map[string]string{
					"something":     "else",
					readyAnnotation: "something else",
				},
			},
		},
		wantAnnotations: map[string]string{
			"something":     "else",
			readyAnnotation: readyAnnotationValue,
		},
		wantPatch: true,
	}, {
		desc: "Pod with READY annotation already set isn't patched",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
				Annotations: map[string]string{
					"something":     "else",
					readyAnnotation: readyAnnotationValue,
				},
			},
		},
		wantAnnotations: map[string]string{
			"something":     "else",
			readyAnnotation: readyAnnotationValue,
		},
		wantPatch: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			kubeclient := fakek8s.NewSimpleClientset(&c.pod)
			patchCalled := false
			kubeclient.PrependReactor("patch", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
				if !c.wantPatch {
					t.Fatal("Pod was patched unexpectedly")
				}
				patchCalled = true
				return false, nil, nil
			})
			if err := UpdateReady(ctx, kubeclient, c.pod); (err != nil) != c.wantErr {
				t.Errorf("UpdateReady (wantErr=%t): %v", c.wantErr, err)
			}

			if c.wantPatch && !patchCalled {
				t.Fatal("Pod was not patched")
			}

			got, err := kubeclient.CoreV1().Pods(c.pod.Namespace).Get(ctx, c.pod.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Getting pod %q after update: %v", c.pod.Name, err)
			} else if d := cmp.Diff(c.wantAnnotations, got.Annotations); d != "" {
				t.Errorf("Annotations Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

const nopImage = "nop-image"

// TestStopSidecars tests stopping sidecars by updating their images to a nop
// image.
func TestStopSidecars(t *testing.T) {
	stepContainer := corev1.Container{
		Name:  stepPrefix + "my-step",
		Image: "foo",
	}
	sidecarContainer := corev1.Container{
		Name:  sidecarPrefix + "my-sidecar",
		Image: "original-image",
	}
	stoppedSidecarContainer := corev1.Container{
		Name:  sidecarContainer.Name,
		Image: nopImage,
	}

	// This is a container that doesn't start with the "sidecar-" prefix,
	// which indicates it was injected into the Pod by a Mutating Webhook
	// Admission Controller. Injected sidecars should also be stopped if
	// they're running.
	injectedSidecar := corev1.Container{
		Name:  "injected",
		Image: "original-injected-image",
	}
	stoppedInjectedSidecar := corev1.Container{
		Name:  injectedSidecar.Name,
		Image: nopImage,
	}
	// This is a container that is added by the controller for accessing sidecar logs.
	// This should not be stopped as long as results-from is set to sidecar-logs.
	resultsSidecar := corev1.Container{
		Name:  pipeline.ReservedResultsSidecarContainerName,
		Image: "original-injected-image",
	}
	// This container can be stopped if the results-from is not set to sidecar-logs.
	stoppedResultsSidecar := corev1.Container{
		Name:  pipeline.ReservedResultsSidecarContainerName,
		Image: nopImage,
	}

	for _, c := range []struct {
		desc                   string
		pod                    corev1.Pod
		resultExtractionMethod string
		wantContainers         []corev1.Container
	}{{
		desc: "Running sidecars (incl injected) should be stopped",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer, injectedSidecar},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					// Step state doesn't matter.
				}, {
					Name: sidecarContainer.Name,
					// Sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}, {
					Name: injectedSidecar.Name,
					// Injected sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}},
			},
		},
		wantContainers: []corev1.Container{stepContainer, stoppedSidecarContainer, stoppedInjectedSidecar},
	}, {
		desc: "Results Sidecar should not be stopped",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer, resultsSidecar},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					// Step state doesn't matter.
				}, {
					Name: sidecarContainer.Name,
					// Sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}, {
					Name: resultsSidecar.Name,
					// Results sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}},
			},
		},
		resultExtractionMethod: "sidecar-logs",
		wantContainers:         []corev1.Container{stepContainer, stoppedSidecarContainer, resultsSidecar},
	}, {
		desc: "Results Sidecar should be stopped result method is not sidecar logs",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer, resultsSidecar},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					// Step state doesn't matter.
				}, {
					Name: sidecarContainer.Name,
					// Sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}, {
					Name: resultsSidecar.Name,
					// Results sidecar is running.
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now())}},
				}},
			},
		},
		wantContainers: []corev1.Container{stepContainer, stoppedSidecarContainer, stoppedResultsSidecar},
	}, {
		desc: "Pending Pod should not be updated",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				// Container states don't matter.
			},
		},
		wantContainers: []corev1.Container{stepContainer, sidecarContainer},
	}, {
		desc: "Non-Running sidecars should not be stopped",
		pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{stepContainer, sidecarContainer, injectedSidecar},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					// Step state doesn't matter.
				}, {
					Name: sidecarContainer.Name,
					// Sidecar is waiting.
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
				}, {
					Name: injectedSidecar.Name,
					// Injected sidecar is waiting.
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
				}},
			},
		},
		wantContainers: []corev1.Container{stepContainer, sidecarContainer, injectedSidecar},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			if c.resultExtractionMethod != "" {
				ctx = config.ToContext(ctx, &config.Config{
					FeatureFlags: &config.FeatureFlags{
						ResultExtractionMethod: c.resultExtractionMethod,
					},
				})
			}
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			kubeclient := fakek8s.NewSimpleClientset(&c.pod)
			if got, err := StopSidecars(ctx, nopImage, kubeclient, c.pod.Namespace, c.pod.Name); err != nil {
				t.Errorf("error stopping sidecar: %v", err)
			} else if d := cmp.Diff(c.wantContainers, got.Spec.Containers); d != "" {
				t.Errorf("Containers Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
