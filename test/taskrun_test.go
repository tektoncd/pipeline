//go:build e2e
// +build e2e

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

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/test/parse"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestTaskRunFailure(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := helpers.ObjectNameForTest(t)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: mirror.gcr.io/busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
  - image: mirror.gcr.io/busybox
    command: ['/bin/sh']
    args: ['-c', 'exit 1']
  - image: mirror.gcr.io/busybox
    command: ['/bin/sh']
    args: ['-c', 'sleep 30s']
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedStepState := []v1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		TerminationReason: "Completed",
		Name:              "unnamed-0",
		Container:         "step-unnamed-0",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		TerminationReason: "Error",
		Name:              "unnamed-1",
		Container:         "step-unnamed-1",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		TerminationReason: "Skipped",
		Name:              "unnamed-2",
		Container:         "step-unnamed-2",
	}}
	expectedStepNumber := len(expectedStepState)
	if len(taskrun.Status.Steps) < expectedStepNumber {
		t.Fatalf("expected at least %d steps, got %d", expectedStepNumber, len(taskrun.Status.Steps))
	}
	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID", "Running")
	lastStepIndex := len(expectedStepState) - 1
	for i := range lastStepIndex {
		if d := cmp.Diff(taskrun.Status.Steps[i], expectedStepState[i], ignoreTerminatedFields, ignoreStepFields); d != "" {
			t.Fatalf("taskrun.Status.Steps[%d]:\n-got, +want: %v", i, d)
		}
	}

	// Sometimes, the state of the last container in the Pod is still running,
	// and the state content of the final step is not skipped.
	// In this case, we should compare the state of the last step with the normal state.
	otherLatestExpectedStepState := v1.StepState{
		Name:      "unnamed-2",
		Container: "step-unnamed-2",
	}

	if d := cmp.Diff(taskrun.Status.Steps[lastStepIndex], expectedStepState[lastStepIndex], ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Logf("taskrun.Status.Steps[%d]:\n-got, +want: %v", lastStepIndex, d)
		// try to compare the state of the last step with the other state
		if d := cmp.Diff(taskrun.Status.Steps[lastStepIndex], otherLatestExpectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
			t.Fatalf("taskrun.Status.Steps[%d]:\n-got, +want: %v", lastStepIndex, d)
		}
	}

	releaseAnnotation, ok := taskrun.Annotations[pod.ReleaseAnnotation]
	// This should always contain a commit truncated to 7 characters, possibly with "-dirty" suffix (based on knative.dev/pkg/changeset)
	commitIDRegexp := regexp.MustCompile(`^[a-f0-9]{7}(-dirty)?$`)
	if !ok || !commitIDRegexp.MatchString(releaseAnnotation) {
		t.Fatalf("expected Taskrun to be annotated with %s=devel or with nightly release tag, got %s=%s", pod.ReleaseAnnotation, pod.ReleaseAnnotation, releaseAnnotation)
	}
}

func TestTaskRunStatus(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := helpers.ObjectNameForTest(t)

	fqImageName := getTestImage(busyboxImage)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: %s
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
`, helpers.ObjectNameForTest(t), namespace, fqImageName))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	expectedStepState := []v1.StepState{{
		TerminationReason: "Completed",
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:      "unnamed-0",
		Container: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
	// Note(chmouel): Sometime we have docker-pullable:// or docker.io/library as prefix, so let only compare the suffix
	if !strings.HasSuffix(taskrun.Status.Steps[0].ImageID, fqImageName) {
		t.Fatalf("`ImageID: %s` does not end with `%s`", taskrun.Status.Steps[0].ImageID, fqImageName)
	}

	if d := cmp.Diff(taskrun.Status.TaskSpec, &task.Spec); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

func TestTaskRunStepsTerminationReasons(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t)
	defer tearDown(ctx, t, c, namespace)
	fqImageName := getTestImage(busyboxImage)

	tests := []struct {
		description        string
		shouldSucceed      bool
		taskRun            string
		shouldCancel       bool
		expectedStepStatus []v1.StepState
	}{
		{
			description:   "termination completed",
			shouldSucceed: true,
			taskRun: `
metadata:
  name: %v
  namespace: %v
spec:
  taskSpec:
    steps:
    - image: %v
      name: first
      command: ['/bin/sh']
      args: ['-c', 'echo hello']`,
			expectedStepStatus: []v1.StepState{
				{
					Container:         "step-first",
					Name:              "first",
					TerminationReason: "Completed",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Reason:   "Completed",
						},
					},
				},
			},
		},
		{
			description:   "termination continued (continue)",
			shouldSucceed: true,
			taskRun: `
metadata:
  name: %v
  namespace: %v
spec:
  taskSpec:
    steps:
    - image: %v
      onError: continue
      name: first
      command: ['/bin/sh']
      args: ['-c', 'echo hello; exit 1']`,
			expectedStepStatus: []v1.StepState{
				{
					Container:         "step-first",
					Name:              "first",
					TerminationReason: "Continued",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Completed",
						},
					},
				},
			},
		},
		{
			description:   "termination continued (continueAndFail)",
			shouldSucceed: false,
			taskRun: `
metadata:
  name: %v
  namespace: %v
spec:
  taskSpec:
    steps:
    - image: %v
      onError: continueAndFail
      name: first
      command: ['/bin/sh']
      args: ['-c', 'echo hello; exit 1']
    - image: %v
      name: second
      command: ['/bin/sh']
      args: ['-c', 'echo hello']`,
			expectedStepStatus: []v1.StepState{
				{
					Container:         "step-first",
					Name:              "first",
					TerminationReason: "Continued",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
				{
					Container:         "step-second",
					Name:              "second",
					TerminationReason: "Completed",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Reason:   "Completed",
						},
					},
				},
			},
		},
		{
			description:   "termination errored",
			shouldSucceed: false,
			taskRun: `
metadata:
  name: %v
  namespace: %v
spec:
  taskSpec:
    steps:
    - image: %v
      name: first
      command: ['/bin/sh']
      args: ['-c', 'echo hello; exit 1']`,
			expectedStepStatus: []v1.StepState{
				{
					Container:         "step-first",
					Name:              "first",
					TerminationReason: "Error",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
			},
		},
		{
			description:   "termination timedout",
			shouldSucceed: false,
			taskRun: `
metadata:
  name: %v
  namespace: %v
spec:
  taskSpec:
    steps:
    - image: %v
      name: first
      timeout: 1s
      command: ['/bin/sh']
      args: ['-c', 'echo hello; sleep 5s']`,
			expectedStepStatus: []v1.StepState{
				{
					Container:         "step-first",
					Name:              "first",
					TerminationReason: "TimeoutExceeded",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
			},
		},
		{
			description:   "termination skipped",
			shouldSucceed: false,
			taskRun: `
metadata:
  name: %v
  namespace: %v
spec:
  taskSpec:
    steps:
    - image: %v
      name: first
      command: ['/bin/sh']
      args: ['-c', 'echo hello; exit 1']
    - image: %v
      name: second
      command: ['/bin/sh']
      args: ['-c', 'echo hello']`,
			expectedStepStatus: []v1.StepState{
				{
					Container:         "step-first",
					Name:              "first",
					TerminationReason: "Error",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
				{
					Container:         "step-second",
					Name:              "second",
					TerminationReason: "Skipped",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
			},
		},
		{
			description:   "termination cancelled",
			shouldSucceed: false,
			shouldCancel:  true,
			taskRun: `
metadata:
  name: %v
  namespace: %v
spec:
  taskSpec:
    steps:
    - image: %v
      name: first
      command: ['/bin/sh']
      args: ['-c', 'sleep infinity; echo hello']`,
			expectedStepStatus: []v1.StepState{
				{
					Container:         "step-first",
					Name:              "first",
					TerminationReason: "TaskRunCancelled",
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "TaskRunCancelled",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			taskRunName := helpers.ObjectNameForTest(t)
			values := []interface{}{taskRunName, namespace}
			for range test.expectedStepStatus {
				values = append(values, fqImageName)
			}
			taskRunYaml := fmt.Sprintf(test.taskRun, values...)
			taskRun := parse.MustParseV1TaskRun(t, taskRunYaml)

			if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			expectedTaskRunState := TaskRunFailed(taskRunName)
			finalStatus := "Failed"
			if test.shouldSucceed {
				expectedTaskRunState = TaskRunSucceed(taskRunName)
				finalStatus = "Succeeded"
			}

			if test.shouldCancel {
				expectedTaskRunState = FailedWithReason("TaskRunCancelled", taskRunName)
				if err := cancelTaskRun(t, ctx, taskRunName, c); err != nil {
					t.Fatalf("Error cancelling taskrun: %s", err)
				}
			}

			err := WaitForTaskRunState(ctx, c, taskRunName, expectedTaskRunState, finalStatus, v1Version)
			if err != nil {
				t.Fatalf("Error waiting for TaskRun to finish: %s", err)
			}

			taskRunState, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}

			ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID", "Message")
			ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID")
			if d := cmp.Diff(taskRunState.Status.Steps, test.expectedStepStatus, ignoreTerminatedFields, ignoreStepFields); d != "" {
				t.Fatalf("-got, +want: %v", d)
			}
		})
	}
}

func cancelTaskRun(t *testing.T, ctx context.Context, taskRunName string, c *clients) error {
	t.Helper()

	err := WaitForTaskRunState(ctx, c, taskRunName, Running(taskRunName), "Running", v1Version)
	if err != nil {
		t.Fatalf("Error waiting for TaskRun to start running before cancelling: %s", err)
	}

	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     "TaskRunCancelled",
	}}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return err
	}

	if _, err := c.V1TaskRunClient.Patch(ctx, taskRunName, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
		return err
	}

	return nil
}

func TestTaskRunRetryFailure(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := helpers.ObjectNameForTest(t)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: mirror.gcr.io/busybox
    command: ['/bin/sh']
    args: ['-c', 'exit 1']
    volumeMounts:
    - mountPath: /cache
      name: $(workspaces.cache.volume)
  workspaces:
  - description: cache
    name: cache
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  retries: 1
  workspaces:
  - name: cache
    emptyDir: {}
`, taskRunName, namespace, task.Name))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if !isFailed(t, taskrun.GetName(), taskrun.Status.Conditions) {
		t.Fatalf("task should have been a failure")
	}

	expectedReason := "Failed"
	actualReason := taskrun.Status.GetCondition(apis.ConditionSucceeded).GetReason()
	if actualReason != expectedReason {
		t.Fatalf("expected TaskRun to have failed reason %s, got %s", expectedReason, actualReason)
	}

	expectedStepState := []v1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		TerminationReason: "Error",
		Name:              "unnamed-0",
		Container:         "step-unnamed-0",
	}}
	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
	if len(taskrun.Status.RetriesStatus) != 1 {
		t.Fatalf("expected 1 retry status, got %d", len(taskrun.Status.RetriesStatus))
	}
}

func TestTaskRunResolveDefaultParameterSubstitutionOnStepAction(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t, requireAllGates(map[string]string{
		"enable-api-fields": "beta",
	}))

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
    - name: repository
      type: string
      default: https://github.com/tektoncd/catalog.git
    - name: revision
      type: string
      default: main
  steps:
    - name: clone
      ref:
        resolver: git
        params:
        - name: url
          value: "$(params.repository)"
        - name: pathInRepo
          value: /stepaction/git-clone/0.1/git-clone.yaml
        - name: revision
          value: "$(params.revision)"
      params:
        - name: output-path
          value: "/tmp"
        - name: url
          value: $(params.repository)
        - name: revision
          value: $(params.revision)
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	taskRunName := helpers.ObjectNameForTest(t)
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
  retries: 1
`, taskRunName, namespace, task.Name))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to complete", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if !isSuccessful(t, taskrun.GetName(), taskrun.Status.Conditions) {
		t.Fatalf("task should have succeeded")
	}

	expectedReason := "Succeeded"
	actualReason := taskrun.Status.GetCondition(apis.ConditionSucceeded).GetReason()
	if actualReason != expectedReason {
		t.Fatalf("expected TaskRun to have failed reason %s, got %s", expectedReason, actualReason)
	}

	expectedStepState := []v1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		TerminationReason: "Completed",
		Name:              "clone",
		Container:         "step-clone",
	}}
	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID", "Message")
	ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID", "Results", "Provenance")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}
