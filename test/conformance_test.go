//go:build conformance
// +build conformance

/*
Copyright 2020 The Tekton Authors

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
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

type conditionFn func(name string) ConditionAccessorFn

func TestTaskRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	fqImageName := getTestImage(busyboxImage)

	for _, tc := range []struct {
		name                    string
		tr                      *v1.TaskRun
		fn                      conditionFn
		expectedConditionStatus corev1.ConditionStatus
		expectedStepState       []v1.StepState
	}{{
		name: "successful-task-run",
		tr: parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: %s
      command: ['echo', '"hello"']
`, helpers.ObjectNameForTest(t), namespace, fqImageName)),
		fn:                      TaskRunSucceed,
		expectedConditionStatus: corev1.ConditionTrue,
		expectedStepState: []v1.StepState{{
			ContainerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 0,
					Reason:   "Completed",
				},
			},
		}},
	}, {
		name: "failed-task-run",
		tr: parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: %s
      command: ['/bin/sh']
      args: ['-c', 'echo hello']
    - image: %s
      command: ['/bin/sh']
      args: ['-c', 'exit 1']
    - image: %s
      command: ['/bin/sh']
      args: ['-c', 'sleep 30s']
`, helpers.ObjectNameForTest(t), namespace, fqImageName, fqImageName, fqImageName)),
		fn:                      TaskRunFailed,
		expectedConditionStatus: corev1.ConditionFalse,
		expectedStepState: []v1.StepState{{
			ContainerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 0,
					Reason:   "Completed",
				},
			},
		}, {
			ContainerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1,
					Reason:   "Error",
				},
			},
		}, {
			ContainerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1,
					Reason:   "Error",
				},
			},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Creating TaskRun %s", tc.tr.Name)
			if _, err := c.V1TaskRunClient.Create(ctx, tc.tr, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun `%s`: %s", tc.tr.Name, err)
			}

			if err := WaitForTaskRunState(ctx, c, tc.tr.Name, tc.fn(tc.tr.Name), "WaitTaskRunDone", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
				return
			}
			tr, err := c.V1TaskRunClient.Get(ctx, tc.tr.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get TaskRun `%s`: %s", tc.tr.Name, err)
			}

			// Check required fields in TaskRun ObjectMeta
			if tr.ObjectMeta.Name == "" {
				t.Errorf("TaskRun ObjectMeta doesn't have the Name.")
			}
			if len(tr.ObjectMeta.Labels) == 0 {
				t.Errorf("TaskRun ObjectMeta doesn't have the Labels.")
			}
			if len(tr.ObjectMeta.Annotations) == 0 {
				t.Errorf("TaskRun ObjectMeta doesn't have the Annotations.")
			}
			if tr.ObjectMeta.CreationTimestamp.IsZero() {
				t.Errorf("TaskRun ObjectMeta doesn't have the CreationTimestamp.")
			}

			// Check required fields in TaskRun Status
			if len(tr.Status.Conditions) == 0 {
				t.Errorf("TaskRun Status doesn't have the Conditions.")
			}
			if tr.Status.StartTime.IsZero() {
				t.Errorf("TaskRun Status doesn't have the StartTime.")
			}
			if tr.Status.CompletionTime.IsZero() {
				t.Errorf("TaskRun Status doesn't have the CompletionTime.")
			}
			if len(tr.Status.Steps) == 0 {
				t.Errorf("TaskRun Status doesn't have the Steps.")
			}

			condition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil {
				t.Errorf("Expected a succeeded Condition but got nil.")
			}
			if condition.Status != tc.expectedConditionStatus {
				t.Errorf("TaskRun Status Condition doesn't have the right Status.")
			}
			if condition.Reason == "" {
				t.Errorf("TaskRun Status Condition doesn't have the Reason.")
			}
			if condition.Message == "" {
				t.Errorf("TaskRun Status Condition doesn't have the Message.")
			}
			if condition.Severity != apis.ConditionSeverityError && condition.Severity != apis.ConditionSeverityWarning && condition.Severity != apis.ConditionSeverityInfo {
				t.Errorf("TaskRun Status Condition doesn't have the right Severity.")
			}

			ignoreTerminatedFields := cmpopts.IgnoreFields(corev1.ContainerStateTerminated{}, "StartedAt", "FinishedAt", "ContainerID")
			ignoreStepFields := cmpopts.IgnoreFields(v1.StepState{}, "ImageID", "Name", "ContainerName")
			if d := cmp.Diff(tr.Status.Steps, tc.expectedStepState, ignoreTerminatedFields, ignoreStepFields); d != "" {
				t.Fatalf("-got, +want: %v", d)
			}
			// Note(chmouel): Sometime we have docker-pullable:// or docker.io/library as prefix, so let only compare the suffix
			if !strings.HasSuffix(tr.Status.Steps[0].ImageID, fqImageName) {
				t.Fatalf("`ImageID: %s` does not end with `%s`", tr.Status.Steps[0].ImageID, fqImageName)
			}
		})
	}
}
