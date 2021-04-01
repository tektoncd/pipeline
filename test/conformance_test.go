// +build conformance

/*
Copyright 2021 The Tekton Authors

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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
)

type conditionFn func(name string) ConditionAccessorFn

func TestTaskRunConformance(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	fqImageName := getTestImage(busyboxImage)

	for _, tc := range []struct {
		name                    string
		trName                  string
		tr                      *v1beta1.TaskRun
		fn                      conditionFn
		expectedConditionStatus corev1.ConditionStatus
		expectedStepState       []v1beta1.StepState
	}{{
		name:   "successful-task-run",
		trName: "echo-hello-task-run",
		tr: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "echo-hello-task-run", Namespace: namespace},
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image:   fqImageName,
						Command: []string{"echo", "\"hello\""},
					}}},
				},
			},
		},
		fn:                      TaskRunSucceed,
		expectedConditionStatus: corev1.ConditionTrue,
		expectedStepState: []v1beta1.StepState{{
			ContainerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 0,
					Reason:   "Completed",
				},
			},
		}},
	}, {
		name:   "failed-task-run",
		trName: "failed-echo-hello-task-run",
		tr: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "failed-echo-hello-task-run", Namespace: namespace},
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image:   fqImageName,
						Command: []string{"/bin/sh"},
						Args:    []string{"-c", "echo hello"},
					}}, {Container: corev1.Container{
						Image:   fqImageName,
						Command: []string{"/bin/sh"},
						Args:    []string{"-c", "exit 1"},
					}}, {Container: corev1.Container{
						Image:   fqImageName,
						Command: []string{"/bin/sh"},
						Args:    []string{"-c", "sleep 30s"},
					}}},
				},
			},
		},
		fn:                      TaskRunFailed,
		expectedConditionStatus: corev1.ConditionFalse,
		expectedStepState: []v1beta1.StepState{{
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
			t.Logf("Creating TaskRun %s", tc.trName)
			if _, err := c.TaskRunClient.Create(ctx, tc.tr, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun `%s`: %s", tc.trName, err)
			}

			if err := WaitForTaskRunState(ctx, c, tc.trName, tc.fn(tc.trName), "WaitTaskRunDone"); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
				return
			}
			tr, err := c.TaskRunClient.Get(ctx, tc.trName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get TaskRun `%s`: %s", tc.trName, err)
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
			ignoreStepFields := cmpopts.IgnoreFields(v1beta1.StepState{}, "ImageID", "Name", "ContainerName")
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
