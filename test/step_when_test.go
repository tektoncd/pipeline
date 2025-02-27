//go:build e2e
// +build e2e

// /*
// Copyright 2024 The Tekton Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
package test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

func TestWhenExpressionsInStep(t *testing.T) {
	tests := []struct {
		desc         string
		expected     []v1.StepState
		taskTemplate string
	}{
		{
			desc: "single step, when is false, skipped",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Skipped",
				Name:              "foo",
				Container:         "step-foo",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - input: "foo"
      operator: in
      values: [ "bar" ]
`,
		},
		{
			desc: "single step, when is true, completed",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "foo",
				Container:         "step-foo",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - input: "foo"
      operator: in
      values: [ "foo" ]
`,
		},
		{
			desc: "two steps, first when is false, skipped and second step complete",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Skipped",
				Name:              "foo",
				Container:         "step-foo",
			}, {
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "bar",
				Container:         "step-bar",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - input: "foo"
      operator: in
      values: [ "bar" ]
  - name: bar
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
`,
		},
		{
			desc: "two steps, when is based on step-results",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "foo",
				Container:         "step-foo",
				Results: []v1.TaskRunStepResult{
					{
						Name:  "result1",
						Type:  "string",
						Value: v1.ParamValue{Type: "string", StringVal: "bar"},
					},
				},
			}, {
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "bar",
				Container:         "step-bar",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    results:
      - name: result1
    command: ['/bin/sh']
    args: ['-c', 'echo -n bar >> $(step.results.result1.path)']
  - name: bar
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - input: "$(steps.foo.results.result1)"
      operator: in
      values: [ "bar" ]
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)

			knativetest.CleanupOnInterrupt(func() {
				tearDown(ctx, t, c, namespace)
			}, t.Logf)

			defer tearDown(ctx, t, c, namespace)

			taskRunName := helpers.ObjectNameForTest(t)

			t.Logf("Creating Task and TaskRun in namespace %s", namespace)
			task := parse.MustParseV1Task(t, fmt.Sprintf(tc.taskTemplate, taskRunName, namespace))
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

			if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceeded", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
			}

			taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}
			var ops cmp.Options
			ops = append(ops, cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID", "Message"))
			ops = append(ops, cmpopts.IgnoreFields(v1.StepState{}, "ImageID"))
			if d := cmp.Diff(taskrun.Status.Steps, tc.expected, ops); d != "" {
				t.Fatalf("-got, +want: %v", d)
			}
		})
	}
}

func TestWhenExpressionsCELInStep(t *testing.T) {
	tests := []struct {
		desc         string
		expected     []v1.StepState
		taskTemplate string
	}{
		{
			desc: "single step, when is false, skipped",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Skipped",
				Name:              "foo",
				Container:         "step-foo",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - cel: "'foo'=='bar'"
`,
		},
		{
			desc: "single step, when CEL is true, completed",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "foo",
				Container:         "step-foo",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - cel: "'foo'=='foo'"
`,
		},
		{
			desc: "two steps, first when CEL is false, skipped and second step complete",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Skipped",
				Name:              "foo",
				Container:         "step-foo",
			}, {
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "bar",
				Container:         "step-bar",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - cel: "'foo'=='bar'"
  - name: bar
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
`,
		},
		{
			desc: "two steps, when cel is based on step-results",
			expected: []v1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "foo",
				Container:         "step-foo",
				Results: []v1.TaskRunStepResult{
					{
						Name:  "result1",
						Type:  "string",
						Value: v1.ParamValue{Type: "string", StringVal: "bar"},
					},
				},
			}, {
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 0,
						Reason:   "Completed",
					},
				},
				TerminationReason: "Completed",
				Name:              "bar",
				Container:         "step-bar",
			}},
			taskTemplate: `
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: foo
    image: busybox
    results:
      - name: result1
    command: ['/bin/sh']
    args: ['-c', 'echo -n bar >> $(step.results.result1.path)']
  - name: bar
    image: busybox
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
    when:
    - cel: "'$(steps.foo.results.result1)'=='bar'"
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			featureFlags := getFeatureFlagsBaseOnAPIFlag(t)

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)

			previous := featureFlags.EnableCELInWhenExpression
			updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
				config.EnableCELInWhenExpression: "true",
			})

			knativetest.CleanupOnInterrupt(func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					config.EnableCELInWhenExpression: strconv.FormatBool(previous),
				})
				tearDown(ctx, t, c, namespace)
			}, t.Logf)
			defer func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					config.EnableCELInWhenExpression: strconv.FormatBool(previous),
				})
				tearDown(ctx, t, c, namespace)
			}()

			taskRunName := helpers.ObjectNameForTest(t)

			t.Logf("Creating Task and TaskRun in namespace %s", namespace)
			task := parse.MustParseV1Task(t, fmt.Sprintf(tc.taskTemplate, taskRunName, namespace))
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

			if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceeded", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
			}

			taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}
			var ops cmp.Options
			ops = append(ops, cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID", "Message"))
			ops = append(ops, cmpopts.IgnoreFields(v1.StepState{}, "ImageID"))
			if d := cmp.Diff(taskrun.Status.Steps, tc.expected, ops); d != "" {
				t.Fatalf("-got, +want: %v", d)
			}
		})
	}
}
