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

package resources

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	duckv1beta1 "github.com/knative/pkg/apis/duck/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "foo"
)

var pts = []v1alpha1.PipelineTask{{
	Name:    "mytask1",
	TaskRef: v1alpha1.TaskRef{Name: "task"},
}, {
	Name:    "mytask2",
	TaskRef: v1alpha1.TaskRef{Name: "task"},
}, {
	Name:    "mytask3",
	TaskRef: v1alpha1.TaskRef{Name: "clustertask"},
}, {
	Name:    "mytask4",
	TaskRef: v1alpha1.TaskRef{Name: "task"},
	Retries: 1,
}, {
	Name:    "mytask5",
	TaskRef: v1alpha1.TaskRef{Name: "cancelledTask"},
	Retries: 2,
}}

var p = &v1alpha1.Pipeline{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipeline",
	},
	Spec: v1alpha1.PipelineSpec{
		Tasks: pts,
	},
}

var task = &v1alpha1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name: "task",
	},
	Spec: v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name: "step1",
		}},
	},
}

var clustertask = &v1alpha1.ClusterTask{
	ObjectMeta: metav1.ObjectMeta{
		Name: "clustertask",
	},
	Spec: v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name: "step1",
		}},
	},
}

var trs = []v1alpha1.TaskRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask1",
	},
	Spec: v1alpha1.TaskRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask2",
	},
	Spec: v1alpha1.TaskRunSpec{},
}}

func makeStarted(tr v1alpha1.TaskRun) *v1alpha1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newTr
}

func makeSucceeded(tr v1alpha1.TaskRun) *v1alpha1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionTrue
	return newTr
}

func makeFailed(tr v1alpha1.TaskRun) *v1alpha1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionFalse
	return newTr
}

func withCancelled(tr *v1alpha1.TaskRun) *v1alpha1.TaskRun {
	tr.Status.Conditions[0].Reason = "TaskRunCancelled"
	return tr
}

func withCancelledBySpec(tr *v1alpha1.TaskRun) *v1alpha1.TaskRun {
	tr.Spec.Status = v1alpha1.TaskRunSpecStatusCancelled
	return tr
}

func makeRetried(tr v1alpha1.TaskRun) (newTr *v1alpha1.TaskRun) {
	newTr = newTaskRun(tr)
	newTr.Status.RetriesStatus = []v1alpha1.TaskRunStatus{{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return
}
func withRetries(tr *v1alpha1.TaskRun) *v1alpha1.TaskRun {
	tr.Status.RetriesStatus = []v1alpha1.TaskRunStatus{{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return tr
}

func newTaskRun(tr v1alpha1.TaskRun) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			Name:      tr.Name,
		},
		Spec: tr.Spec,
		Status: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}
}

var noneStartedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}
var oneStartedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeStarted(trs[0]),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}
var oneFinishedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}
var oneFailedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeFailed(trs[0]),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}
var allFinishedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}

var taskCancelled = PipelineRunState{{
	PipelineTask: &pts[4],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      withCancelled(makeRetried(trs[0])),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}

func TestGetNextTasks(t *testing.T) {
	tcs := []struct {
		name         string
		state        PipelineRunState
		candidates   map[string]v1alpha1.PipelineTask
		expectedNext []*ResolvedPipelineRunTask
	}{
		{
			name:         "no-tasks-started-no-candidates",
			state:        noneStartedState,
			candidates:   map[string]v1alpha1.PipelineTask{},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "no-tasks-started-one-candidate",
			state: noneStartedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
			},
			expectedNext: []*ResolvedPipelineRunTask{noneStartedState[0]},
		},
		{
			name:  "no-tasks-started-other-candidate",
			state: noneStartedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{noneStartedState[1]},
		},
		{
			name:  "no-tasks-started-both-candidates",
			state: noneStartedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{noneStartedState[0], noneStartedState[1]},
		},
		{
			name:         "one-task-started-no-candidates",
			state:        oneStartedState,
			candidates:   map[string]v1alpha1.PipelineTask{},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "one-task-started-one-candidate",
			state: oneStartedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "one-task-started-other-candidate",
			state: oneStartedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{oneStartedState[1]},
		},
		{
			name:  "one-task-started-both-candidates",
			state: oneStartedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{oneStartedState[1]},
		},
		{
			name:         "one-task-finished-no-candidates",
			state:        oneFinishedState,
			candidates:   map[string]v1alpha1.PipelineTask{},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "one-task-finished-one-candidate",
			state: oneFinishedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "one-task-finished-other-candidate",
			state: oneFinishedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{oneFinishedState[1]},
		},
		{
			name:  "one-task-finished-both-candidate",
			state: oneFinishedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{oneFinishedState[1]},
		},
		{
			name:         "one-task-failed-no-candidates",
			state:        oneFailedState,
			candidates:   map[string]v1alpha1.PipelineTask{},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "one-task-failed-one-candidate",
			state: oneFailedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "one-task-failed-other-candidate",
			state: oneFailedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{oneFailedState[1]},
		},
		{
			name:  "one-task-failed-both-candidates",
			state: oneFailedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{oneFailedState[1]},
		},
		{
			name:         "all-finished-no-candidates",
			state:        allFinishedState,
			candidates:   map[string]v1alpha1.PipelineTask{},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "all-finished-one-candidate",
			state: allFinishedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "all-finished-other-candidate",
			state: allFinishedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "all-finished-both-candidates",
			state: allFinishedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask1": pts[0],
				"mytask2": pts[1],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "one-cancelled-one-candidate",
			state: taskCancelled,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask5": pts[4],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.GetNextTasks(tc.candidates)
			if d := cmp.Diff(next, tc.expectedNext); d != "" {
				t.Errorf("Didn't get expected next Tasks: %v", d)
			}
		})
	}
}

func TestGetNextTaskWithRetries(t *testing.T) {

	var taskCancelledByStatusState = PipelineRunState{{
		PipelineTask: &pts[4], // 2 retries needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelled(makeRetried(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskCancelledBySpecState = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelledBySpec(makeRetried(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskRunningState = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      makeStarted(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskSucceededState = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      makeSucceeded(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskRetriedState = PipelineRunState{{
		PipelineTask: &pts[3], // 1 retry needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelled(makeRetried(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskExpectedState = PipelineRunState{{
		PipelineTask: &pts[4], // 2 retries needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withRetries(makeFailed(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	tcs := []struct {
		name         string
		state        PipelineRunState
		candidates   map[string]v1alpha1.PipelineTask
		expectedNext []*ResolvedPipelineRunTask
	}{
		{
			name:  "tasks-cancelled-no-candidates",
			state: taskCancelledByStatusState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask5": pts[4],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "tasks-cancelled-bySpec-no-candidates",
			state: taskCancelledBySpecState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask5": pts[4],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "tasks-running-no-candidates",
			state: taskRunningState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask5": pts[4],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "tasks-succeeded-bySpec-no-candidates",
			state: taskSucceededState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask5": pts[4],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "tasks-retried-no-candidates",
			state: taskRetriedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask5": pts[3],
			},
			expectedNext: []*ResolvedPipelineRunTask{},
		},
		{
			name:  "tasks-retried-one-candidates",
			state: taskExpectedState,
			candidates: map[string]v1alpha1.PipelineTask{
				"mytask5": pts[3],
			},
			expectedNext: []*ResolvedPipelineRunTask{taskExpectedState[0]},
		},
	}

	// iterate over *state* to get from candidate and check if TaskRun is there.
	// Cancelled TaskRun should have a TaskRun cancelled and with a retry but should not retry.

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.GetNextTasks(tc.candidates)
			if d := cmp.Diff(next, tc.expectedNext); d != "" {
				t.Errorf("Didn't get expected next Tasks: %v", d)
			}
		})
	}
}
func TestIsDone(t *testing.T) {

	var taskCancelledByStatusState = PipelineRunState{{
		PipelineTask: &pts[4], // 2 retries needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelled(makeRetried(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskCancelledBySpecState = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelledBySpec(makeRetried(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskRunningState = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      makeStarted(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskSucceededState = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      makeSucceeded(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskRetriedState = PipelineRunState{{
		PipelineTask: &pts[3], // 1 retry needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelled(makeRetried(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskExpectedState = PipelineRunState{{
		PipelineTask: &pts[4], // 2 retries needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withRetries(makeFailed(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var noPipelineTaskState = PipelineRunState{{
		PipelineTask: nil,
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withRetries(makeFailed(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var noTaskRunState = PipelineRunState{{
		PipelineTask: &pts[4], // 2 retries needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	tcs := []struct {
		name       string
		state      PipelineRunState
		expected   bool
		ptExpected []bool
	}{
		{
			name:       "tasks-cancelled-no-candidates",
			state:      taskCancelledByStatusState,
			expected:   false,
			ptExpected: []bool{false},
		},
		{
			name:       "tasks-cancelled-bySpec-no-candidates",
			state:      taskCancelledBySpecState,
			expected:   false,
			ptExpected: []bool{false},
		},
		{
			name:       "tasks-running-no-candidates",
			state:      taskRunningState,
			expected:   false,
			ptExpected: []bool{false},
		},
		{
			name:       "tasks-succeeded-bySpec-no-candidates",
			state:      taskSucceededState,
			expected:   true,
			ptExpected: []bool{true},
		},
		{
			name:       "tasks-retried-no-candidates",
			state:      taskRetriedState,
			expected:   false,
			ptExpected: []bool{false},
		},
		{
			name:       "tasks-retried-one-candidates",
			state:      taskExpectedState,
			expected:   false,
			ptExpected: []bool{false},
		},
		{
			name:       "no-pipelineTask",
			state:      noPipelineTaskState,
			expected:   false,
			ptExpected: []bool{false},
		},
		{
			name:       "No-taskrun",
			state:      noTaskRunState,
			expected:   false,
			ptExpected: []bool{false},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			isDone := tc.state.IsDone()
			if d := cmp.Diff(isDone, tc.expected); d != "" {
				t.Errorf("Didn't get expected IsDone: %v", d)
			}
			for i, pt := range tc.state {
				isDone = pt.IsDone()
				if d := cmp.Diff(isDone, tc.ptExpected[i]); d != "" {
					t.Errorf("Didn't get expected (ResolvedPipelineRunTask) IsDone: %v", d)
				}

			}
		})
	}
}

func TestSuccessfulPipelineTaskNames(t *testing.T) {
	tcs := []struct {
		name          string
		state         PipelineRunState
		expectedNames []string
	}{
		{
			name:          "no-tasks-started",
			state:         noneStartedState,
			expectedNames: []string{},
		},
		{
			name:          "one-task-started",
			state:         oneStartedState,
			expectedNames: []string{},
		},
		{
			name:          "one-task-finished",
			state:         oneFinishedState,
			expectedNames: []string{"mytask1"},
		},
		{
			name:          "one-task-failed",
			state:         oneFailedState,
			expectedNames: []string{},
		},
		{
			name:          "all-finished",
			state:         allFinishedState,
			expectedNames: []string{"mytask1", "mytask2"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			names := tc.state.SuccessfulPipelineTaskNames()
			if d := cmp.Diff(names, tc.expectedNames); d != "" {
				t.Errorf("Expected to get completed names %v but got something different: %v", tc.expectedNames, d)
			}
		})
	}
}

func TestGetPipelineConditionStatus(t *testing.T) {

	var taskRetriedState = PipelineRunState{{
		PipelineTask: &pts[3], // 1 retry needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelled(makeRetried(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	tcs := []struct {
		name           string
		state          []*ResolvedPipelineRunTask
		expectedStatus corev1.ConditionStatus
	}{
		{
			name:           "no-tasks-started",
			state:          noneStartedState,
			expectedStatus: corev1.ConditionUnknown,
		},
		{
			name:           "one-task-started",
			state:          oneStartedState,
			expectedStatus: corev1.ConditionUnknown,
		},
		{
			name:           "one-task-finished",
			state:          oneFinishedState,
			expectedStatus: corev1.ConditionUnknown,
		},
		{
			name:           "one-task-failed",
			state:          oneFailedState,
			expectedStatus: corev1.ConditionFalse,
		},
		{
			name:           "all-finished",
			state:          allFinishedState,
			expectedStatus: corev1.ConditionTrue,
		},
		{
			name:           "one-retry-needed",
			state:          taskRetriedState,
			expectedStatus: corev1.ConditionUnknown,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := GetPipelineConditionStatus("somepipelinerun", tc.state, zap.NewNop().Sugar(), &metav1.Time{Time: time.Now()},
				nil)
			if c.Status != tc.expectedStatus {
				t.Fatalf("Expected to get status %s but got %s for state %v", tc.expectedStatus, c.Status, tc.state)
			}
		})
	}
}

func TestGetResourcesFromBindings(t *testing.T) {
	p := tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
	))
	pr := tb.PipelineRun("pipelinerun", "namespace", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
	))
	m, err := GetResourcesFromBindings(p, pr)
	if err != nil {
		t.Fatalf("didn't expect error getting resources from bindings but got: %v", err)
	}
	expectedResources := map[string]v1alpha1.PipelineResourceRef{
		"git-resource": {
			Name: "sweet-resource",
		},
	}
	if d := cmp.Diff(expectedResources, m); d != "" {
		t.Fatalf("Expected resources didn't match actual -want, +got: %v", d)
	}
}

func TestGetResourcesFromBindings_Missing(t *testing.T) {
	p := tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
		tb.PipelineDeclaredResource("image-resource", "image"),
	))
	pr := tb.PipelineRun("pipelinerun", "namespace", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
	))
	_, err := GetResourcesFromBindings(p, pr)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was missing but got no error")
	}
}

func TestGetResourcesFromBindings_Extra(t *testing.T) {
	p := tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
	))
	pr := tb.PipelineRun("pipelinerun", "namespace", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
		tb.PipelineRunResourceBinding("image-resource", tb.PipelineResourceBindingRef("sweet-resource2")),
	))
	_, err := GetResourcesFromBindings(p, pr)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was extra but got no error")
	}
}
func TestResolvePipelineRun(t *testing.T) {
	names.TestingSeed()

	p := tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
		tb.PipelineTask("mytask1", "task",
			tb.PipelineTaskInputResource("input1", "git-resource"),
		),
		tb.PipelineTask("mytask2", "task",
			tb.PipelineTaskOutputResource("output1", "git-resource"),
		),
		tb.PipelineTask("mytask3", "task",
			tb.PipelineTaskOutputResource("output1", "git-resource"),
		),
	))
	providedResources := map[string]v1alpha1.PipelineResourceRef{
		"git-resource": {
			Name: "someresource",
		},
	}

	r := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "someresource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
		},
	}
	pr := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	// The Task "task" doesn't actually take any inputs or outputs, but validating
	// that is not done as part of Run resolution
	getTask := func(name string) (v1alpha1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) { return nil, nil }
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return nil, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return r, nil }

	pipelineState, err := ResolvePipelineRun(pr, getTask, getTaskRun, getClusterTask, getResource, p.Spec.Tasks, providedResources)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedState := PipelineRunState{{
		PipelineTask: &p.Spec.Tasks[0],
		TaskRunName:  "pipelinerun-mytask1-9l9zj",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs: map[string]*v1alpha1.PipelineResource{
				"input1": r,
			},
			Outputs: map[string]*v1alpha1.PipelineResource{},
		},
	}, {
		PipelineTask: &p.Spec.Tasks[1],
		TaskRunName:  "pipelinerun-mytask2-mz4c7",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs:   map[string]*v1alpha1.PipelineResource{},
			Outputs: map[string]*v1alpha1.PipelineResource{
				"output1": r,
			},
		},
	}, {
		PipelineTask: &p.Spec.Tasks[2],
		TaskRunName:  "pipelinerun-mytask3-mssqb",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs:   map[string]*v1alpha1.PipelineResource{},
			Outputs: map[string]*v1alpha1.PipelineResource{
				"output1": r,
			},
		},
	}}

	if d := cmp.Diff(pipelineState, expectedState, cmpopts.IgnoreUnexported(v1alpha1.TaskRunSpec{})); d != "" {
		t.Errorf("Expected to get current pipeline state %v, but actual differed: %s", expectedState, d)
	}
}

func TestResolvePipelineRun_PipelineTaskHasNoResources(t *testing.T) {
	pts := []v1alpha1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: v1alpha1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask2",
		TaskRef: v1alpha1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask3",
		TaskRef: v1alpha1.TaskRef{Name: "task"},
	}}
	providedResources := map[string]v1alpha1.PipelineResourceRef{}

	getTask := func(name string) (v1alpha1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) { return &trs[0], nil }
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return clustertask, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return nil, fmt.Errorf("should not get called") }
	pr := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	pipelineState, err := ResolvePipelineRun(pr, getTask, getTaskRun, getClusterTask, getResource, pts, providedResources)
	if err != nil {
		t.Fatalf("Did not expect error when resolving PipelineRun without Resources: %v", err)
	}
	if len(pipelineState) != 3 {
		t.Fatalf("Expected only 2 resolved PipelineTasks but got %d", len(pipelineState))
	}
	expectedTaskResources := &resources.ResolvedTaskResources{
		TaskName: task.Name,
		TaskSpec: &task.Spec,
		Inputs:   map[string]*v1alpha1.PipelineResource{},
		Outputs:  map[string]*v1alpha1.PipelineResource{},
	}
	if d := cmp.Diff(pipelineState[0].ResolvedTaskResources, expectedTaskResources, cmpopts.IgnoreUnexported(v1alpha1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but but actual differed: %s", d)
	}
	if d := cmp.Diff(pipelineState[1].ResolvedTaskResources, expectedTaskResources, cmpopts.IgnoreUnexported(v1alpha1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but but actual differed: %s", d)
	}
}

func TestResolvePipelineRun_TaskDoesntExist(t *testing.T) {
	pts := []v1alpha1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: v1alpha1.TaskRef{Name: "task"},
	}}
	providedResources := map[string]v1alpha1.PipelineResourceRef{}

	// Return an error when the Task is retrieved, as if it didn't exist
	getTask := func(name string) (v1alpha1.TaskInterface, error) {
		return nil, errors.NewNotFound(v1alpha1.Resource("task"), name)
	}
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) {
		return nil, errors.NewNotFound(v1alpha1.Resource("clustertask"), name)
	}

	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) {
		return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
	}
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return nil, fmt.Errorf("should not get called") }
	pr := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	_, err := ResolvePipelineRun(pr, getTask, getTaskRun, getClusterTask, getResource, pts, providedResources)
	switch err := err.(type) {
	case nil:
		t.Fatalf("Expected error getting non-existent Tasks for Pipeline %s but got none", p.Name)
	case *TaskNotFoundError:
		// expected error
	default:
		t.Fatalf("Expected specific error type returned by func for non-existent Task for Pipeline %s but got %s", p.Name, err)
	}
}

func TestResolvePipelineRun_ResourceBindingsDontExist(t *testing.T) {
	tests := []struct {
		name string
		p    *v1alpha1.Pipeline
	}{
		{
			name: "input doesnt exist",
			p: tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
				tb.PipelineTask("mytask1", "task",
					tb.PipelineTaskInputResource("input1", "git-resource"),
				),
			)),
		},
		{
			name: "output doesnt exist",
			p: tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
				tb.PipelineTask("mytask1", "task",
					tb.PipelineTaskOutputResource("input1", "git-resource"),
				),
			)),
		},
	}
	providedResources := map[string]v1alpha1.PipelineResourceRef{}

	getTask := func(name string) (v1alpha1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) { return &trs[0], nil }
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return clustertask, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return nil, fmt.Errorf("shouldnt be called") }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun",
				},
			}
			_, err := ResolvePipelineRun(pr, getTask, getTaskRun, getClusterTask, getResource, tt.p.Spec.Tasks, providedResources)
			if err == nil {
				t.Fatalf("Expected error when bindings are in incorrect state for Pipeline %s but got none", p.Name)
			}
		})
	}
}

func TestResolvePipelineRun_ResourcesDontExist(t *testing.T) {
	tests := []struct {
		name string
		p    *v1alpha1.Pipeline
	}{
		{
			name: "input doesnt exist",
			p: tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
				tb.PipelineTask("mytask1", "task",
					tb.PipelineTaskInputResource("input1", "git-resource"),
				),
			)),
		},
		{
			name: "output doesnt exist",
			p: tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
				tb.PipelineTask("mytask1", "task",
					tb.PipelineTaskOutputResource("input1", "git-resource"),
				),
			)),
		},
	}
	providedResources := map[string]v1alpha1.PipelineResourceRef{
		"git-resource": {
			Name: "doesnt-exist",
		},
	}

	getTask := func(name string) (v1alpha1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) { return &trs[0], nil }
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return clustertask, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) {
		return nil, errors.NewNotFound(v1alpha1.Resource("pipelineresource"), name)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun",
				},
			}
			_, err := ResolvePipelineRun(pr, getTask, getTaskRun, getClusterTask, getResource, tt.p.Spec.Tasks, providedResources)
			switch err := err.(type) {
			case nil:
				t.Fatalf("Expected error getting non-existent Resources for Pipeline %s but got none", p.Name)
			case *ResourceNotFoundError:
				// expected error
			default:
				t.Fatalf("Expected specific error type returned by func for non-existent Resource for Pipeline %s but got %s", p.Name, err)
			}
		})
	}
}

func TestValidateFrom(t *testing.T) {
	r := tb.PipelineResource("holygrail", namespace, tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeImage))
	state := []*ResolvedPipelineRunTask{{
		PipelineTask: &v1alpha1.PipelineTask{
			Name: "quest",
		},
		ResolvedTaskResources: tb.ResolvedTaskResources(
			tb.ResolvedTaskResourcesTaskSpec(
				tb.TaskOutputs(tb.OutputsResource("sweet-artifact", v1alpha1.PipelineResourceTypeImage)),
			),
			tb.ResolvedTaskResourcesOutputs("sweet-artifact", r),
		),
	}, {
		PipelineTask: &v1alpha1.PipelineTask{
			Name: "winning",
			Resources: &v1alpha1.PipelineTaskResources{
				Inputs: []v1alpha1.PipelineTaskInputResource{{
					Name: "awesome-thing",
					From: []string{"quest"},
				}},
			}},
		ResolvedTaskResources: tb.ResolvedTaskResources(
			tb.ResolvedTaskResourcesTaskSpec(
				tb.TaskInputs(tb.InputsResource("awesome-thing", v1alpha1.PipelineResourceTypeImage)),
			),
			tb.ResolvedTaskResourcesInputs("awesome-thing", r),
		),
	}}
	err := ValidateFrom(state)
	if err != nil {
		t.Fatalf("Didn't expect error when validating valid from clause but got: %v", err)
	}
}

func TestValidateFrom_Invalid(t *testing.T) {
	r := tb.PipelineResource("holygrail", namespace, tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeImage))
	otherR := tb.PipelineResource("holyhandgrenade", namespace, tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeImage))

	for _, tc := range []struct {
		name        string
		state       []*ResolvedPipelineRunTask
		errContains string
	}{{
		name: "from tries to reference input",
		state: []*ResolvedPipelineRunTask{{
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "quest",
			},
			ResolvedTaskResources: tb.ResolvedTaskResources(
				tb.ResolvedTaskResourcesTaskSpec(
					tb.TaskInputs(tb.InputsResource("sweet-artifact", v1alpha1.PipelineResourceTypeImage)),
				),
				tb.ResolvedTaskResourcesInputs("sweet-artifact", r),
			),
		}, {
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "winning",
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "awesome-thing",
						From: []string{"quest"},
					}},
				}},
			ResolvedTaskResources: tb.ResolvedTaskResources(
				tb.ResolvedTaskResourcesTaskSpec(
					tb.TaskInputs(tb.InputsResource("awesome-thing", v1alpha1.PipelineResourceTypeImage)),
				),
				tb.ResolvedTaskResourcesInputs("awesome-thing", r),
			),
		}},
		errContains: "ambiguous",
	}, {
		name: "from resource doesn't exist",
		state: []*ResolvedPipelineRunTask{{
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "quest",
			},
			ResolvedTaskResources: tb.ResolvedTaskResources(),
		}, {
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "winning",
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "awesome-thing",
						From: []string{"quest"},
					}},
				}},
			ResolvedTaskResources: tb.ResolvedTaskResources(
				tb.ResolvedTaskResourcesTaskSpec(
					tb.TaskInputs(tb.InputsResource("awesome-thing", v1alpha1.PipelineResourceTypeImage)),
				),
				tb.ResolvedTaskResourcesInputs("awesome-thing", r),
			),
		}},
		errContains: "ambiguous",
	}, {
		name: "from task doesn't exist",
		state: []*ResolvedPipelineRunTask{{
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "winning",
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "awesome-thing",
						From: []string{"quest"},
					}},
				}},
			ResolvedTaskResources: tb.ResolvedTaskResources(
				tb.ResolvedTaskResourcesTaskSpec(
					tb.TaskInputs(tb.InputsResource("awesome-thing", v1alpha1.PipelineResourceTypeImage)),
				),
				tb.ResolvedTaskResourcesInputs("awesome-thing", r),
			),
		}},
		errContains: "does not exist",
	}, {
		name: "from task refers to itself",
		state: []*ResolvedPipelineRunTask{{
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "winning",
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "awesome-thing",
						From: []string{"winning"},
					}},
				}},
			ResolvedTaskResources: tb.ResolvedTaskResources(
				tb.ResolvedTaskResourcesTaskSpec(
					tb.TaskInputs(tb.InputsResource("awesome-thing", v1alpha1.PipelineResourceTypeImage)),
				),
				tb.ResolvedTaskResourcesInputs("awesome-thing", r),
			),
		}},
		errContains: "from itself",
	}, {
		name: "from is bound to different resource",
		state: []*ResolvedPipelineRunTask{{
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "quest",
			},
			ResolvedTaskResources: tb.ResolvedTaskResources(
				tb.ResolvedTaskResourcesTaskSpec(
					tb.TaskOutputs(tb.OutputsResource("sweet-artifact", v1alpha1.PipelineResourceTypeImage)),
				),
				tb.ResolvedTaskResourcesOutputs("sweet-artifact", r),
			),
		}, {
			PipelineTask: &v1alpha1.PipelineTask{
				Name: "winning",
				Resources: &v1alpha1.PipelineTaskResources{
					Inputs: []v1alpha1.PipelineTaskInputResource{{
						Name: "awesome-thing",
						From: []string{"quest"},
					}},
				}},
			ResolvedTaskResources: tb.ResolvedTaskResources(
				tb.ResolvedTaskResourcesTaskSpec(
					tb.TaskInputs(tb.InputsResource("awesome-thing", v1alpha1.PipelineResourceTypeImage)),
				),
				tb.ResolvedTaskResourcesInputs("awesome-thing", otherR),
			),
		}},
		errContains: "ambiguous",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFrom(tc.state)
			if err == nil {
				t.Fatalf("Expected error when validating invalid from but got none")
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("Expected error to contain %q but was: %v", tc.errContains, err)
			}
		})
	}
}

func TestResolvePipelineRun_withExistingTaskRuns(t *testing.T) {
	names.TestingSeed()

	p := tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
		tb.PipelineTask("mytask-with-a-really-long-name-to-trigger-truncation", "task",
			tb.PipelineTaskInputResource("input1", "git-resource"),
		),
	))
	providedResources := map[string]v1alpha1.PipelineResourceRef{
		"git-resource": {
			Name: "someresource",
		},
	}

	r := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "someresource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
		},
	}
	taskrunStatus := map[string]*v1alpha1.PipelineRunTaskRunStatus{}
	taskrunStatus["pipelinerun-mytask-with-a-really-long-name-to-trigger-tru-9l9zj"] = &v1alpha1.PipelineRunTaskRunStatus{
		PipelineTaskName: "mytask-with-a-really-long-name-to-trigger-truncation",
		Status:           &v1alpha1.TaskRunStatus{},
	}

	pr := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
		Status: v1alpha1.PipelineRunStatus{
			TaskRuns: taskrunStatus,
		},
	}

	// The Task "task" doesn't actually take any inputs or outputs, but validating
	// that is not done as part of Run resolution
	getTask := func(name string) (v1alpha1.TaskInterface, error) { return task, nil }
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return nil, nil }
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) { return nil, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return r, nil }

	pipelineState, err := ResolvePipelineRun(pr, getTask, getTaskRun, getClusterTask, getResource, p.Spec.Tasks, providedResources)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedState := PipelineRunState{{
		PipelineTask: &p.Spec.Tasks[0],
		TaskRunName:  "pipelinerun-mytask-with-a-really-long-name-to-trigger-tru-9l9zj",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs: map[string]*v1alpha1.PipelineResource{
				"input1": r,
			},
			Outputs: map[string]*v1alpha1.PipelineResource{},
		},
	}}

	if d := cmp.Diff(pipelineState, expectedState, cmpopts.IgnoreUnexported(v1alpha1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected to get current pipeline state %v, but actual differed: %s", expectedState, d)
	}
}
