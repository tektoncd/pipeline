/*
Copyright 2018 The Knative Authors

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

package taskrun

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCancelTaskRun(t *testing.T) {
	testCases := []struct {
		name           string
		taskRun        *v1alpha1.TaskRun
		pod            *corev1.Pod
		expectedStatus apis.Condition
	}{{
		name: "no-pod-scheduled",
		taskRun: tb.TaskRun("test-taskrun-run-cancelled", "foo", tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name),
			tb.TaskRunCancelled,
		), tb.TaskRunStatus(tb.Condition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		}))),
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "TaskRunCancelled",
			Message: `TaskRun "test-taskrun-run-cancelled" was cancelled`,
		},
	}, {
		name: "pod-scheduled",
		taskRun: tb.TaskRun("test-taskrun-run-cancelled", "foo", tb.TaskRunSpec(
			tb.TaskRunTaskRef(simpleTask.Name),
			tb.TaskRunCancelled,
		), tb.TaskRunStatus(tb.Condition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		}), tb.PodName("foo-is-bar"))),
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo-is-bar",
		}},
		expectedStatus: apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "TaskRunCancelled",
			Message: `TaskRun "test-taskrun-run-cancelled" was cancelled`,
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				TaskRuns: []*v1alpha1.TaskRun{tc.taskRun},
			}
			if tc.pod != nil {
				d.Pods = []*corev1.Pod{tc.pod}
			}

			observer, _ := observer.New(zap.InfoLevel)
			c, _ := test.SeedTestData(t, d)
			err := cancelTaskRun(tc.taskRun, c.Kube, zap.New(observer).Sugar())
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(tc.taskRun.Status.GetCondition(apis.ConditionSucceeded), &tc.expectedStatus, ignoreLastTransitionTime); d != "" {
				t.Fatalf("-want, +got: %v", d)
			}
		})
	}
}
