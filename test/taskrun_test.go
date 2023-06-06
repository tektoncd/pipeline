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

	jsonpatch "gomodules.xyz/jsonpatch/v2"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestTaskRunFailure(t *testing.T) {
	ctx := context.Background()
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
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
  - image: busybox
    command: ['/bin/sh']
    args: ['-c', 'exit 1']
  - image: busybox
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

	if isSpireEnabled(ctx, t, c, namespace) {
		spireShouldFailTaskRunResultsVerify(t, taskrun)
		spireShouldPassSpireAnnotation(t, taskrun)
	}

	expectedStepState := []v1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:      "unnamed-0",
		Container: "step-unnamed-0",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:      "unnamed-1",
		Container: "step-unnamed-1",
	}, {
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:      "unnamed-2",
		Container: "step-unnamed-2",
	}}
	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}

	releaseAnnotation, ok := taskrun.Annotations[pod.ReleaseAnnotation]
	// This should always contain a commit truncated to 7 characters, possibly with "-dirty" suffix (based on knative.dev/pkg/changeset)
	commitIDRegexp := regexp.MustCompile(`^[a-f0-9]{7}(-dirty)?$`)
	if !ok || !commitIDRegexp.MatchString(releaseAnnotation) {
		t.Fatalf("expected Taskrun to be annotated with %s=devel or with nightly release tag, got %s=%s", pod.ReleaseAnnotation, pod.ReleaseAnnotation, releaseAnnotation)
	}
}

func TestTaskRunStatus(t *testing.T) {
	ctx := context.Background()
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

	if isSpireEnabled(ctx, t, c, namespace) {
		spireShouldPassTaskRunResultsVerify(t, taskrun)
		spireShouldPassSpireAnnotation(t, taskrun)
	}

	expectedStepState := []v1.StepState{{
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

// TestTaskRunModificationSpire is an exclusive test for SPIRE integration into taskrun.
// The test starts a taskrun which has a sleep. While the taskrun is "sleep"ing,
// the text modifies the taskrun results.
// This change is caught by the taskrun reconciler when it tries to verify the results.
func TestTaskRunModificationSpire(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var c *clients
	var namespace string
	var oldConfigMap map[string]string
	var configSpire = map[string]string{
		"results-from":              "termination-message",
		"enforce-nonfalsifiability": "spire",
	}

	c, namespace, oldConfigMap = setupWithFlags(ctx, t, configSpire, requireAllGates(spireFeatureGates))
	defer resetFlags(ctx, t, c, oldConfigMap)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := "non-falsifiable-provenance-fail"

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: non-falsifiable
  namespace: %s
spec:
  results:
  - name: foo
  - name: bar
  steps:
  - image: ubuntu
    script: |
      #!/usr/bin/env bash
      sleep 20
      printf "hello" > "$(results.foo.path)"
      printf "world" > "$(results.bar.path)"
`, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: non-falsifiable
`, taskRunName, namespace))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to be in running state", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, Running(taskRunName), "TaskRunRunning", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to start running: %s", err)
	}

	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "replace",
		Path:      "/status/taskSpec/steps/0/image",
		Value:     "not-ubuntu",
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		t.Fatalf("failed to marshal patch bytes in order to stop")
	}
	t.Logf("Patching TaskRun %s in namespace %s mid run for spire to catch the un-authorized changed", taskRunName, namespace)
	if _, err := c.V1TaskRunClient.Patch(ctx, taskRunName, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, "status"); err != nil {
		t.Fatalf("Failed to patch taskrun `%s`: %s", taskRunName, err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to succeed", taskRunName, namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunFailed(taskRunName), "TaskRunFailed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if isSpireEnabled(ctx, t, c, namespace) {
		spireShouldFailTaskRunResultsVerify(t, taskrun)
		spireShouldFailSpireAnnotation(t, taskrun)
	}

	expectedStepState := []v1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

func TestTaskRunSpire(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var c *clients
	var namespace string
	var oldConfigMap map[string]string
	var configSpire = map[string]string{
		"results-from":              "termination-message",
		"enforce-nonfalsifiability": "spire",
	}

	c, namespace, oldConfigMap = setupWithFlags(ctx, t, configSpire, requireAllGates(spireFeatureGates))
	defer resetFlags(ctx, t, c, oldConfigMap)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskRunName := "non-falsifiable-provenance-pass"

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: non-falsifiable
  namespace: %s
spec:
  results:
  - name: foo
  - name: bar
  steps:
  - image: ubuntu
    script: |
      #!/usr/bin/env bash
      sleep 20
      printf "hello" > "$(results.foo.path)"
      printf "world" > "$(results.bar.path)"
`, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: non-falsifiable
`, taskRunName, namespace))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to succeed", taskRunName, namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}

	if isSpireEnabled(ctx, t, c, namespace) {
		spireShouldPassTaskRunResultsVerify(t, taskrun)
		spireShouldPassSpireAnnotation(t, taskrun)
	}

	expectedStepState := []v1.StepState{{
		ContainerState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 0,
				Reason:   "Completed",
			},
		},
		Name:          "unnamed-0",
		ContainerName: "step-unnamed-0",
	}}

	ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
	ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID")
	if d := cmp.Diff(taskrun.Status.Steps, expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}

func setupWithFlags(ctx context.Context, t *testing.T, configMapData map[string]string, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string, map[string]string) {
	t.Helper()
	c, ns := setup(ctx, t, fn...)
	featureFlagsCM, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetFeatureFlagsConfigName(), err)
	}
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
	return c, ns, featureFlagsCM.Data
}

func resetFlags(ctx context.Context, t *testing.T, c *clients, configMapData map[string]string) {
	t.Helper()
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
}
