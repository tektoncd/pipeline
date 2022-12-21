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

package resources

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var now = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)

var testClock = clock.NewFakePassiveClock(now)

func TestPipelineRunFacts_CheckDAGTasksDoneDone(t *testing.T) {
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
		TaskRun:      withRetries(makeToBeRetried(trs[0])),
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

	var runRunningState = PipelineRunState{{
		PipelineTask:  &pts[12],
		CustomTask:    true,
		RunObjectName: "pipelinerun-mytask13",
		RunObject:     makeRunStarted(runs[0]),
	}}

	var runSucceededState = PipelineRunState{{
		PipelineTask:  &pts[12],
		CustomTask:    true,
		RunObjectName: "pipelinerun-mytask13",
		RunObject:     makeRunSucceeded(runs[0]),
	}}

	var runFailedState = PipelineRunState{{
		PipelineTask:  &pts[12],
		CustomTask:    true,
		RunObjectName: "pipelinerun-mytask13",
		RunObject:     makeRunFailed(runs[0]),
	}}

	var taskCancelledFailedWithRetries = PipelineRunState{{
		PipelineTask: &pts[4], // 2 retries needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelled(makeFailed(trs[0])),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	tcs := []struct {
		name       string
		state      PipelineRunState
		expected   bool
		ptExpected []bool
	}{{
		name:       "tasks-cancelled-no-candidates",
		state:      taskCancelledByStatusState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "tasks-cancelled-bySpec-no-candidates",
		state:      taskCancelledBySpecState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "tasks-running-no-candidates",
		state:      taskRunningState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "tasks-succeeded-bySpec-no-candidates",
		state:      taskSucceededState,
		expected:   true,
		ptExpected: []bool{true},
	}, {
		name:       "tasks-retried-no-candidates",
		state:      taskRetriedState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "tasks-retried-one-candidates",
		state:      taskExpectedState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "No-taskrun",
		state:      noTaskRunState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "run-running-no-candidates",
		state:      runRunningState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "run-succeeded-no-candidates",
		state:      runSucceededState,
		expected:   true,
		ptExpected: []bool{true},
	}, {
		name:       "run-failed-no-candidates",
		state:      runFailedState,
		expected:   true,
		ptExpected: []bool{true},
	}, {
		name:       "run-cancelled-failed-with-retries",
		state:      taskCancelledFailedWithRetries,
		expected:   true,
		ptExpected: []bool{true},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Unexpected error while building DAG for state %v: %v", tc.state, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}

			isDone := facts.checkTasksDone(d)
			if d := cmp.Diff(tc.expected, isDone); d != "" {
				t.Errorf("Didn't get expected checkTasksDone %s", diff.PrintWantGot(d))
			}
			for i, pt := range tc.state {
				isDone = pt.isDone(&facts)
				if d := cmp.Diff(tc.ptExpected[i], isDone); d != "" {
					t.Errorf("Didn't get expected (ResolvedPipelineTask) isDone %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestIsBeforeFirstTaskRun_WithNotStartedTask(t *testing.T) {
	if !noneStartedState.IsBeforeFirstTaskRun() {
		t.Fatalf("Expected state to be before first taskrun")
	}
}

func TestIsBeforeFirstTaskRun_WithNotStartedRun(t *testing.T) {
	if !noRunStartedState.IsBeforeFirstTaskRun() {
		t.Fatalf("Expected state to be before first taskrun (Run test)")
	}
}

func TestIsBeforeFirstTaskRun_WithStartedTask(t *testing.T) {
	if oneStartedState.IsBeforeFirstTaskRun() {
		t.Fatalf("Expected state to be after first taskrun")
	}
}

func TestIsBeforeFirstTaskRun_WithStartedRun(t *testing.T) {
	if oneRunStartedState.IsBeforeFirstTaskRun() {
		t.Fatalf("Expected state to be after first taskrun (Run test)")
	}
}
func TestIsBeforeFirstTaskRun_WithSucceededTask(t *testing.T) {
	if finalScheduledState.IsBeforeFirstTaskRun() {
		t.Fatalf("Expected state to be after first taskrun")
	}
}

func TestGetNextTasks(t *testing.T) {
	tcs := []struct {
		name         string
		state        PipelineRunState
		candidates   sets.String
		expectedNext []*ResolvedPipelineTask
	}{{
		name:         "no-tasks-started-no-candidates",
		state:        noneStartedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "no-tasks-started-one-candidate",
		state:        noneStartedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineTask{noneStartedState[0]},
	}, {
		name:         "no-tasks-started-other-candidate",
		state:        noneStartedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineTask{noneStartedState[1]},
	}, {
		name:         "no-tasks-started-both-candidates",
		state:        noneStartedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineTask{noneStartedState[0], noneStartedState[1]},
	}, {
		name:         "one-task-started-no-candidates",
		state:        oneStartedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-started-one-candidate",
		state:        oneStartedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-started-other-candidate",
		state:        oneStartedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineTask{oneStartedState[1]},
	}, {
		name:         "one-task-started-both-candidates",
		state:        oneStartedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineTask{oneStartedState[1]},
	}, {
		name:         "one-task-finished-no-candidates",
		state:        oneFinishedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-finished-one-candidate",
		state:        oneFinishedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-finished-other-candidate",
		state:        oneFinishedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineTask{oneFinishedState[1]},
	}, {
		name:         "one-task-finished-both-candidate",
		state:        oneFinishedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineTask{oneFinishedState[1]},
	}, {
		name:         "one-task-failed-no-candidates",
		state:        oneFailedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-failed-one-candidate",
		state:        oneFailedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-failed-other-candidate",
		state:        oneFailedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineTask{oneFailedState[1]},
	}, {
		name:         "one-task-failed-both-candidates",
		state:        oneFailedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineTask{oneFailedState[1]},
	}, {
		name:         "final-task-scheduled-no-candidates",
		state:        finalScheduledState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "final-task-finished-one-candidate",
		state:        finalScheduledState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "final-task-finished-other-candidate",
		state:        finalScheduledState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "final-task-finished-both-candidate",
		state:        finalScheduledState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-no-candidates",
		state:        allFinishedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-one-candidate",
		state:        allFinishedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-other-candidate",
		state:        allFinishedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-both-candidates",
		state:        allFinishedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-cancelled-one-candidate",
		state:        taskCancelled,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "no-runs-started-both-candidates",
		state:        noRunStartedState,
		candidates:   sets.NewString("mytask13", "mytask14"),
		expectedNext: []*ResolvedPipelineTask{noRunStartedState[0], noRunStartedState[1]},
	}, {
		name:         "one-run-started-both-candidates",
		state:        oneRunStartedState,
		candidates:   sets.NewString("mytask13", "mytask14"),
		expectedNext: []*ResolvedPipelineTask{oneRunStartedState[1]},
	}, {
		name:         "one-run-failed-both-candidates",
		state:        oneRunFailedState,
		candidates:   sets.NewString("mytask13", "mytask14"),
		expectedNext: []*ResolvedPipelineTask{oneRunFailedState[1]},
	}, {
		name:         "no-tasks-started-no-candidates-matrix",
		state:        noneStartedStateMatrix,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "no-tasks-started-one-candidate-matrix",
		state:        noneStartedStateMatrix,
		candidates:   sets.NewString("mytask16"),
		expectedNext: []*ResolvedPipelineTask{noneStartedStateMatrix[0]},
	}, {
		name:         "no-tasks-started-other-candidate-matrix",
		state:        noneStartedStateMatrix,
		candidates:   sets.NewString("mytask17"),
		expectedNext: []*ResolvedPipelineTask{noneStartedStateMatrix[1]},
	}, {
		name:         "no-tasks-started-both-candidates-matrix",
		state:        noneStartedStateMatrix,
		candidates:   sets.NewString("mytask16", "mytask17"),
		expectedNext: []*ResolvedPipelineTask{noneStartedStateMatrix[0], noneStartedStateMatrix[1]},
	}, {
		name:         "one-task-started-no-candidates-matrix",
		state:        oneStartedStateMatrix,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-started-one-candidate-matrix",
		state:        oneStartedStateMatrix,
		candidates:   sets.NewString("mytask16"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-started-other-candidate-matrix",
		state:        oneStartedStateMatrix,
		candidates:   sets.NewString("mytask17"),
		expectedNext: []*ResolvedPipelineTask{oneStartedStateMatrix[1]},
	}, {
		name:         "one-task-started-both-candidates-matrix",
		state:        oneStartedStateMatrix,
		candidates:   sets.NewString("mytask16", "mytask17"),
		expectedNext: []*ResolvedPipelineTask{oneStartedStateMatrix[1]},
	}, {
		name:         "one-task-finished-no-candidates-matrix",
		state:        oneFinishedStateMatrix,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-finished-one-candidate-matrix",
		state:        oneFinishedStateMatrix,
		candidates:   sets.NewString("mytask16"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-finished-other-candidate-matrix",
		state:        oneFinishedStateMatrix,
		candidates:   sets.NewString("mytask17"),
		expectedNext: []*ResolvedPipelineTask{oneFinishedStateMatrix[1]},
	}, {
		name:         "one-task-finished-both-candidate-matrix",
		state:        oneFinishedStateMatrix,
		candidates:   sets.NewString("mytask16", "mytask17"),
		expectedNext: []*ResolvedPipelineTask{oneFinishedStateMatrix[1]},
	}, {
		name:         "one-task-failed-no-candidates-matrix",
		state:        oneFailedStateMatrix,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-failed-one-candidate-matrix",
		state:        oneFailedStateMatrix,
		candidates:   sets.NewString("mytask16"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-task-failed-other-candidate-matrix",
		state:        oneFailedStateMatrix,
		candidates:   sets.NewString("mytask17"),
		expectedNext: []*ResolvedPipelineTask{oneFailedStateMatrix[1]},
	}, {
		name:         "one-task-failed-both-candidates-matrix",
		state:        oneFailedStateMatrix,
		candidates:   sets.NewString("mytask16", "mytask17"),
		expectedNext: []*ResolvedPipelineTask{oneFailedStateMatrix[1]},
	}, {
		name:         "final-task-scheduled-no-candidates-matrix",
		state:        finalScheduledStateMatrix,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "final-task-finished-one-candidate-matrix",
		state:        finalScheduledStateMatrix,
		candidates:   sets.NewString("mytask16"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "final-task-finished-other-candidate-matrix",
		state:        finalScheduledStateMatrix,
		candidates:   sets.NewString("mytask17"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "final-task-finished-both-candidate-matrix",
		state:        finalScheduledStateMatrix,
		candidates:   sets.NewString("mytask16", "mytask17"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-no-candidates-matrix",
		state:        allFinishedStateMatrix,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-one-candidate-matrix",
		state:        allFinishedStateMatrix,
		candidates:   sets.NewString("mytask16"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-other-candidate-matrix",
		state:        allFinishedStateMatrix,
		candidates:   sets.NewString("mytask17"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "all-finished-both-candidates-matrix",
		state:        allFinishedStateMatrix,
		candidates:   sets.NewString("mytask16", "mytask17"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "one-cancelled-one-candidate-matrix",
		state:        taskCancelledMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "no-runs-started-both-candidates-matrix",
		state:        noRunStartedStateMatrix,
		candidates:   sets.NewString("mytask19", "mytask20"),
		expectedNext: []*ResolvedPipelineTask{noRunStartedStateMatrix[0], noRunStartedStateMatrix[1]},
	}, {
		name:         "one-run-started-both-candidates-matrix",
		state:        oneRunStartedStateMatrix,
		candidates:   sets.NewString("mytask19", "mytask20"),
		expectedNext: []*ResolvedPipelineTask{oneRunStartedStateMatrix[1]},
	}, {
		name:         "one-run-failed-both-candidates-matrix",
		state:        oneRunFailedStateMatrix,
		candidates:   sets.NewString("mytask19", "mytask20"),
		expectedNext: []*ResolvedPipelineTask{oneRunFailedStateMatrix[1]},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.getNextTasks(tc.candidates)
			if d := cmp.Diff(tc.expectedNext, next); d != "" {
				t.Errorf("Didn't get expected next Tasks %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetNextTaskWithRetries(t *testing.T) {
	var failedV1alpha1Run = parse.MustParseRun(t, `
metadata:
  name: custom-task
  namespace: namespace
spec:
  retries: 1
  params:
  - name: param1
    value: value1
  ref:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: Timedout
`)
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

	var runCancelledByStatusState = PipelineRunState{{
		PipelineTask:  &pts[4], // 2 retries needed
		RunObjectName: "pipelinerun-mytask1",
		RunObject:     withRunCancelled(withRunRetries(newRun(runs[0]))),
		CustomTask:    true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runCancelledBySpecState = PipelineRunState{{
		PipelineTask:  &pts[4],
		RunObjectName: "pipelinerun-mytask1",
		RunObject:     withRunCancelledBySpec(withRunRetries(newRun(runs[0]))),
		CustomTask:    true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runRunningState = PipelineRunState{{
		PipelineTask:  &pts[4],
		RunObjectName: "pipelinerun-mytask1",
		RunObject:     makeRunStarted(runs[0]),
		CustomTask:    true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runSucceededState = PipelineRunState{{
		PipelineTask:  &pts[4],
		RunObjectName: "pipelinerun-mytask1",
		RunObject:     makeRunSucceeded(runs[0]),
		CustomTask:    true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runRetriedState = PipelineRunState{{
		PipelineTask:  &pts[3], // 1 retry needed
		RunObjectName: "pipelinerun-mytask1",
		RunObject:     withRunCancelled(withRunRetries(newRun(runs[0]))),
		CustomTask:    true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runExpectedState = PipelineRunState{{
		PipelineTask:  &pts[4], // 2 retries needed
		RunObjectName: "pipelinerun-mytask1",
		RunObject:     failedV1alpha1Run,
		CustomTask:    true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskCancelledByStatusStateMatrix = PipelineRunState{{
		PipelineTask: &pts[20], // 2 retries needed
		TaskRunNames: []string{"pipelinerun-mytask1"},
		TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeRetried(trs[0]))},
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskCancelledBySpecStateMatrix = PipelineRunState{{
		PipelineTask: &pts[20], // 2 retries needed
		TaskRunNames: []string{"pipelinerun-mytask1"},
		TaskRuns:     []*v1beta1.TaskRun{withCancelledBySpec(makeRetried(trs[0]))},
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskRunningStateMatrix = PipelineRunState{{
		PipelineTask: &pts[20], // 2 retries needed
		TaskRunNames: []string{"pipelinerun-mytask1"},
		TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0])},
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskSucceededStateMatrix = PipelineRunState{{
		PipelineTask: &pts[20], // 2 retries needed
		TaskRunNames: []string{"pipelinerun-mytask1"},
		TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0])},
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var taskRetriedStateMatrix = PipelineRunState{{
		PipelineTask: &pts[17], // 1 retry needed
		TaskRunNames: []string{"pipelinerun-mytask1"},
		TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeRetried(trs[0]))},
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runCancelledByStatusStateMatrix = PipelineRunState{{
		PipelineTask:   &pts[20], // 2 retries needed
		RunObjectNames: []string{"pipelinerun-mytask1"},
		RunObjects:     []v1beta1.RunObject{withRunCancelled(withRunRetries(newRun(runs[0])))},
		CustomTask:     true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runCancelledBySpecStateMatrix = PipelineRunState{{
		PipelineTask:   &pts[20], // 2 retries needed
		RunObjectNames: []string{"pipelinerun-mytask1"},
		RunObjects:     []v1beta1.RunObject{withRunCancelledBySpec(withRunRetries(newRun(runs[0])))},
		CustomTask:     true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runRunningStateMatrix = PipelineRunState{{
		PipelineTask:   &pts[20], // 2 retries needed
		RunObjectNames: []string{"pipelinerun-mytask1"},
		RunObjects:     []v1beta1.RunObject{makeRunStarted(runs[0])},
		CustomTask:     true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runSucceededStateMatrix = PipelineRunState{{
		PipelineTask:   &pts[20], // 2 retries needed
		RunObjectNames: []string{"pipelinerun-mytask1"},
		RunObjects:     []v1beta1.RunObject{makeRunSucceeded(runs[0])},
		CustomTask:     true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runRetriedStateMatrix = PipelineRunState{{
		PipelineTask:   &pts[17], // 1 retry needed
		RunObjectNames: []string{"pipelinerun-mytask1"},
		RunObjects:     []v1beta1.RunObject{withRunCancelled(withRunRetries(newRun(runs[0])))},
		CustomTask:     true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var runExpectedStateMatrix = PipelineRunState{{
		PipelineTask:   &pts[20], // 2 retries needed
		RunObjectNames: []string{"pipelinerun-mytask1"},
		RunObjects:     []v1beta1.RunObject{failedV1alpha1Run},
		CustomTask:     true,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	tcs := []struct {
		name         string
		state        PipelineRunState
		candidates   sets.String
		expectedNext []*ResolvedPipelineTask
	}{{
		name:         "tasks-cancelled-no-candidates",
		state:        taskCancelledByStatusState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-cancelled-bySpec-no-candidates",
		state:        taskCancelledBySpecState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-running-no-candidates",
		state:        taskRunningState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-succeeded-bySpec-no-candidates",
		state:        taskSucceededState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-retried-no-candidates",
		state:        taskRetriedState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "runs-cancelled-no-candidates",
		state:        runCancelledByStatusState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "runs-cancelled-bySpec-no-candidates",
		state:        runCancelledBySpecState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "runs-running-no-candidates",
		state:        runRunningState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "run-succeeded-bySpec-no-candidates",
		state:        runSucceededState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "run-retried-no-candidates",
		state:        runRetriedState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "run-retried-one-candidates",
		state:        runExpectedState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineTask{runExpectedState[0]},
	}, {
		name:         "tasks-cancelled-no-candidates-matrix",
		state:        taskCancelledByStatusStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-cancelled-bySpec-no-candidates-matrix",
		state:        taskCancelledBySpecStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-running-no-candidates-matrix",
		state:        taskRunningStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-succeeded-bySpec-no-candidates-matrix",
		state:        taskSucceededStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "tasks-retried-no-candidates-matrix",
		state:        taskRetriedStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "runs-cancelled-no-candidates-matrix",
		state:        runCancelledByStatusStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "runs-cancelled-bySpec-no-candidates-matrix",
		state:        runCancelledBySpecStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "runs-running-no-candidates-matrix",
		state:        runRunningStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "run-succeeded-bySpec-no-candidates-matrix",
		state:        runSucceededStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "run-retried-no-candidates-matrix",
		state:        runRetriedStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{},
	}, {
		name:         "run-retried-one-candidates-matrix",
		state:        runExpectedStateMatrix,
		candidates:   sets.NewString("mytask21"),
		expectedNext: []*ResolvedPipelineTask{runExpectedStateMatrix[0]},
	}}

	// iterate over *state* to get from candidate and check if TaskRun or Run is there.
	// Cancelled TaskRun should have a TaskRun cancelled and with a retry but should not retry and likewise for Runs.

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.getNextTasks(tc.candidates)
			if d := cmp.Diff(next, tc.expectedNext); d != "" {
				t.Errorf("Didn't get expected next Tasks %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestDAGExecutionQueue tests the DAGExecutionQueue function for PipelineTasks
// in different states (without dependencies on each other) and the PipelineRun in different states.
func TestDAGExecutionQueue(t *testing.T) {
	createdTask := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "createdtask",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		TaskRunName: "createdtask",
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}
	createdRun := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "createdrun",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		RunObjectName: "createdrun",
		CustomTask:    true,
	}
	runningTask := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "runningtask",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		TaskRunName: "runningtask",
		TaskRun:     newTaskRun(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}
	runningRun := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "runningrun",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		RunObjectName: "runningrun",
		RunObject:     newRun(runs[0]),
		CustomTask:    true,
	}
	successfulTask := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "successfultask",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		TaskRunName: "successfultask",
		TaskRun:     makeSucceeded(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}
	successfulRun := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "successfulrun",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		RunObjectName: "successfulrun",
		RunObject:     makeRunSucceeded(runs[0]),
		CustomTask:    true,
	}
	failedTask := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "failedtask",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		TaskRunName: "failedtask",
		TaskRun:     makeFailed(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}
	failedRun := parse.MustParseRun(t, `
metadata:
  name: task
  namespace: namespace
spec:
  retries: 1
  params:
  - name: param1
    value: value1
  ref:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: Timedout
`)
	failedCustomRun := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "failedrun",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		RunObjectName: "failedrun",
		RunObject:     makeRunFailed(runs[0]),
		CustomTask:    true,
	}
	failedRunWithRetries := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "failedrunwithretries",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			Retries: 1,
		},
		RunObjectName: "failedrunwithretries",
		RunObject:     failedRun,
		CustomTask:    true,
	}
	tcs := []struct {
		name       string
		state      PipelineRunState
		specStatus v1beta1.PipelineRunSpecStatus
		want       PipelineRunState
	}{{
		name:       "cancelled",
		specStatus: v1beta1.PipelineRunSpecStatusCancelled,
		state: PipelineRunState{
			&createdTask, &createdRun,
			&runningTask, &runningRun, &successfulTask, &successfulRun,
			&failedRunWithRetries,
		},
	}, {
		name:       "gracefully cancelled",
		specStatus: v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		state: PipelineRunState{
			&createdTask, &createdRun,
			&runningTask, &runningRun, &successfulTask, &successfulRun,
			&failedRunWithRetries,
		},
	}, {
		name:       "gracefully stopped",
		specStatus: v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		state: PipelineRunState{
			&createdTask, &createdRun, &runningTask, &runningRun, &successfulTask, &successfulRun,
		},
	}, {
		name:       "gracefully stopped with retryable tasks",
		specStatus: v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		state: PipelineRunState{
			&createdTask, &createdRun, &runningTask, &runningRun, &successfulTask, &successfulRun,
			&failedTask, &failedCustomRun, &failedRunWithRetries,
		},
		want: PipelineRunState{&failedRunWithRetries},
	}, {
		name: "running",
		state: PipelineRunState{
			&createdTask, &createdRun, &runningTask, &runningRun,
			&successfulTask, &successfulRun,
			&failedRunWithRetries,
		},
		want: PipelineRunState{&createdTask, &createdRun, &failedRunWithRetries},
	}, {
		name: "stopped",
		state: PipelineRunState{
			&createdTask, &createdRun, &runningTask, &runningRun,
			&successfulTask, &successfulRun, &failedTask, &failedCustomRun,
		},
	}, {
		name: "stopped with retryable tasks",
		state: PipelineRunState{
			&createdTask, &createdRun, &runningTask, &runningRun,
			&successfulTask, &successfulRun, &failedCustomRun, &failedRunWithRetries,
		},
		want: PipelineRunState{&failedRunWithRetries},
	}, {
		name:  "all tasks finished",
		state: PipelineRunState{&successfulTask, &successfulRun, &failedTask, &failedCustomRun},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Unexpected error while building DAG for state %v: %v", tc.state, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				SpecStatus:      tc.specStatus,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			queue, err := facts.DAGExecutionQueue()
			if err != nil {
				t.Errorf("unexpected error getting DAG execution queue: %s", err)
			}
			if d := cmp.Diff(tc.want, queue); d != "" {
				t.Errorf("Didn't get expected execution queue: %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestDAGExecutionQueueSequentialTasks tests the DAGExecutionQueue function for sequential TaskRuns
// in different states for a running or stopping PipelineRun.
func TestDAGExecutionQueueSequentialTasks(t *testing.T) {
	firstTask := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "task-1",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		TaskRunName: "task-1",
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}
	secondTask := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:     "task-2",
			TaskRef:  &v1beta1.TaskRef{Name: "task"},
			RunAfter: []string{"task-1"},
		},
		TaskRunName: "task-2",
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}

	tcs := []struct {
		name          string
		firstTaskRun  *v1beta1.TaskRun
		secondTaskRun *v1beta1.TaskRun
		specStatus    v1beta1.PipelineRunSpecStatus
		wantFirst     bool
		wantSecond    bool
	}{{
		name:      "not started",
		wantFirst: true,
	}, {
		name:         "first task running",
		firstTaskRun: newTaskRun(trs[0]),
	}, {
		name:         "first task succeeded",
		firstTaskRun: makeSucceeded(trs[0]),
		wantSecond:   true,
	}, {
		name:         "first task failed",
		firstTaskRun: makeFailed(trs[0]),
	}, {
		name:          "first task succeeded, second task running",
		firstTaskRun:  makeSucceeded(trs[0]),
		secondTaskRun: newTaskRun(trs[1]),
	}, {
		name:          "first task succeeded, second task succeeded",
		firstTaskRun:  makeSucceeded(trs[0]),
		secondTaskRun: makeSucceeded(trs[1]),
	}, {
		name:          "first task succeeded, second task failed",
		firstTaskRun:  makeSucceeded(trs[0]),
		secondTaskRun: makeFailed(trs[1]),
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			firstTask.TaskRun = tc.firstTaskRun
			defer func() { firstTask.TaskRun = nil }()
			secondTask.TaskRun = tc.secondTaskRun
			defer func() { secondTask.TaskRun = nil }()
			state := PipelineRunState{&firstTask, &secondTask}
			d, err := dagFromState(state)
			if err != nil {
				t.Fatalf("Unexpected error while building DAG for state %v: %v", state, err)
			}
			facts := PipelineRunFacts{
				State:           state,
				SpecStatus:      tc.specStatus,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			queue, err := facts.DAGExecutionQueue()
			if err != nil {
				t.Errorf("unexpected error getting DAG execution queue but got error %s", err)
			}
			var expectedQueue PipelineRunState
			if tc.wantFirst {
				expectedQueue = append(expectedQueue, &firstTask)
			}
			if tc.wantSecond {
				expectedQueue = append(expectedQueue, &secondTask)
			}
			if d := cmp.Diff(expectedQueue, queue, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("Didn't get expected execution queue: %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestDAGExecutionQueueSequentialRuns tests the DAGExecutionQueue function for sequential Runs
// in different states for a running or stopping PipelineRun.
func TestDAGExecutionQueueSequentialRuns(t *testing.T) {
	firstRun := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "task-1",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		RunObjectName: "task-1",
		CustomTask:    true,
	}
	secondRun := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:     "task-2",
			TaskRef:  &v1beta1.TaskRef{Name: "task"},
			RunAfter: []string{"task-1"},
		},
		RunObjectName: "task-2",
		CustomTask:    true,
	}

	tcs := []struct {
		name       string
		firstRun   *v1beta1.CustomRun
		secondRun  *v1beta1.CustomRun
		specStatus v1beta1.PipelineRunSpecStatus
		wantFirst  bool
		wantSecond bool
	}{{
		name:      "not started",
		wantFirst: true,
	}, {
		name:     "first run running",
		firstRun: newRun(runs[0]),
	}, {
		name:       "first run succeeded",
		firstRun:   makeRunSucceeded(runs[0]),
		wantSecond: true,
	}, {
		name:     "first run failed",
		firstRun: makeRunFailed(runs[0]),
	}, {
		name:      "first run succeeded, second run running",
		firstRun:  makeRunSucceeded(runs[0]),
		secondRun: newRun(runs[1]),
	}, {
		name:      "first run succeeded, second run succeeded",
		firstRun:  makeRunSucceeded(runs[0]),
		secondRun: makeRunSucceeded(runs[1]),
	}, {
		name:      "first run succeeded, second run failed",
		firstRun:  makeRunSucceeded(runs[0]),
		secondRun: makeRunFailed(runs[1]),
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.firstRun != nil {
				firstRun.RunObject = tc.firstRun
			}
			defer func() { firstRun.RunObject = nil }()
			if tc.secondRun != nil {
				secondRun.RunObject = tc.secondRun
			}
			defer func() { secondRun.RunObject = nil }()
			state := PipelineRunState{&firstRun, &secondRun}
			d, err := dagFromState(state)
			if err != nil {
				t.Fatalf("Unexpected error while building DAG for state %v: %v", state, err)
			}
			facts := PipelineRunFacts{
				State:           state,
				SpecStatus:      tc.specStatus,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			queue, err := facts.DAGExecutionQueue()
			if err != nil {
				t.Errorf("unexpected error getting DAG execution queue but got error %s", err)
			}
			var expectedQueue PipelineRunState
			if tc.wantFirst {
				expectedQueue = append(expectedQueue, &firstRun)
			}
			if tc.wantSecond {
				expectedQueue = append(expectedQueue, &secondRun)
			}
			if d := cmp.Diff(expectedQueue, queue, cmpopts.EquateEmpty()); d != "" {
				t.Errorf("Didn't get expected execution queue: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunState_CompletedOrSkippedDAGTasks(t *testing.T) {
	largePipelineState := buildPipelineStateWithLargeDependencyGraph(t)
	tcs := []struct {
		name          string
		state         PipelineRunState
		specStatus    v1beta1.PipelineRunSpecStatus
		expectedNames []string
	}{{
		name:          "no-tasks-started",
		state:         noneStartedState,
		expectedNames: []string{},
	}, {
		name:          "no-tasks-started-run-cancelled-gracefully",
		state:         noneStartedState,
		specStatus:    v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		expectedNames: []string{pts[0].Name, pts[1].Name},
	}, {
		name:          "one-task-started",
		state:         oneStartedState,
		expectedNames: []string{},
	}, {
		name:          "one-task-started-run-stopped-gracefully",
		state:         oneStartedState,
		specStatus:    v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		expectedNames: []string{pts[1].Name},
	}, {
		name:          "one-task-finished",
		state:         oneFinishedState,
		expectedNames: []string{pts[0].Name},
	}, {
		name:          "one-task-finished-run-cancelled-forcefully",
		state:         oneFinishedState,
		specStatus:    v1beta1.PipelineRunSpecStatusCancelled,
		expectedNames: []string{pts[0].Name},
	}, {
		name:          "one-task-failed",
		state:         oneFailedState,
		expectedNames: []string{pts[0].Name, pts[1].Name},
	}, {
		name:          "all-finished",
		state:         allFinishedState,
		expectedNames: []string{pts[0].Name, pts[1].Name},
	}, {
		name:          "large deps, not started",
		state:         largePipelineState,
		expectedNames: []string{},
	}, {
		name:          "large deps through params, not started",
		state:         buildPipelineStateWithMultipleTaskResults(t, false),
		expectedNames: []string{},
	}, {
		name:          "large deps through params and when expressions, not started",
		state:         buildPipelineStateWithMultipleTaskResults(t, true),
		expectedNames: []string{},
	}, {
		name:          "one-run-started",
		state:         oneRunStartedState,
		expectedNames: []string{},
	}, {
		name:          "one-run-finished",
		state:         oneRunFinishedState,
		expectedNames: []string{pts[12].Name},
	}, {
		name:          "one-run-failed",
		state:         oneRunFailedState,
		expectedNames: []string{pts[12].Name, pts[13].Name},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Unexpected error while building DAG for state %v: %v", tc.state, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				SpecStatus:      tc.specStatus,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			names := facts.completedOrSkippedDAGTasks()
			if d := cmp.Diff(names, tc.expectedNames); d != "" {
				t.Errorf("Expected to get completed names %v but got something different %s", tc.expectedNames, diff.PrintWantGot(d))
			}
		})
	}
}

func buildPipelineStateWithLargeDependencyGraph(t *testing.T) PipelineRunState {
	t.Helper()
	var task = &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "step1",
			}},
		},
	}
	var pipelineRunState PipelineRunState
	pipelineRunState = []*ResolvedPipelineTask{{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "t1",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		TaskRun: nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}
	for i := 2; i < 400; i++ {
		dependFrom := 1
		if i > 10 {
			if i%10 == 0 {
				dependFrom = i - 10
			} else {
				dependFrom = i - (i % 10)
			}
		}
		params := []v1beta1.Param{}
		var alpha byte
		for alpha = 'a'; alpha <= 'j'; alpha++ {
			params = append(params, v1beta1.Param{
				Name: fmt.Sprintf("%c", alpha),
				Value: v1beta1.ParamValue{
					Type:      v1beta1.ParamTypeString,
					StringVal: fmt.Sprintf("$(tasks.t%d.results.%c)", dependFrom, alpha),
				},
			})
		}
		pipelineRunState = append(pipelineRunState, &ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{
				Name:    fmt.Sprintf("t%d", i),
				Params:  params,
				TaskRef: &v1beta1.TaskRef{Name: "task"},
			},
			TaskRun: nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		},
		)
	}
	return pipelineRunState
}

func buildPipelineStateWithMultipleTaskResults(t *testing.T, includeWhen bool) PipelineRunState {
	t.Helper()
	var task = &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "step1",
			}},
		},
	}
	var pipelineRunState PipelineRunState
	pipelineRunState = []*ResolvedPipelineTask{{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "t1",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		TaskRun: nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}
	for i := 2; i < 400; i++ {
		var params []v1beta1.Param
		whenExpressions := v1beta1.WhenExpressions{}
		var alpha byte
		// the task has a reference to multiple task results (a through j) from each parent task - causing a redundant references
		// the task dependents on all predecessors in a graph through params and/or whenExpressions
		for j := 1; j < i; j++ {
			for alpha = 'a'; alpha <= 'j'; alpha++ {
				// include param with task results
				params = append(params, v1beta1.Param{
					Name: fmt.Sprintf("%c", alpha),
					Value: v1beta1.ParamValue{
						Type:      v1beta1.ParamTypeString,
						StringVal: fmt.Sprintf("$(tasks.t%d.results.%c)", j, alpha),
					},
				})
			}
			if includeWhen {
				for alpha = 'a'; alpha <= 'j'; alpha++ {
					// include when expressions with task results
					whenExpressions = append(whenExpressions, v1beta1.WhenExpression{
						Input:    fmt.Sprintf("$(tasks.t%d.results.%c)", j, alpha),
						Operator: selection.In,
						Values:   []string{"true"},
					})
				}
			}
		}
		pipelineRunState = append(pipelineRunState, &ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{
				Name:            fmt.Sprintf("t%d", i),
				Params:          params,
				TaskRef:         &v1beta1.TaskRef{Name: "task"},
				WhenExpressions: whenExpressions,
			},
			TaskRun: nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		},
		)
	}
	return pipelineRunState
}

func TestPipelineRunState_GetFinalTasksAndNames(t *testing.T) {
	tcs := []struct {
		name               string
		desc               string
		state              PipelineRunState
		DAGTasks           []v1beta1.PipelineTask
		finalTasks         []v1beta1.PipelineTask
		expectedFinalTasks PipelineRunState
		expectedFinalNames sets.String
		expectedTaskNames  sets.String
	}{{
		// tasks: [ mytask1, mytask2]
		// none finally
		name: "01 - DAG tasks done, no final tasks",
		desc: "DAG tasks (mytask1 and mytask2) finished successfully -" +
			" do not schedule final tasks since pipeline didnt have any",
		state:              oneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[1]},
		finalTasks:         []v1beta1.PipelineTask{},
		expectedFinalTasks: PipelineRunState{},
		expectedFinalNames: nil,
		expectedTaskNames:  sets.NewString(pts[0].Name, pts[1].Name),
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "02 - DAG task not started, no final tasks",
		desc:               "DAG tasks (mytask1) not started yet - do not schedule final tasks (mytask2)",
		state:              noneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{},
		expectedFinalNames: sets.NewString(pts[1].Name),
		expectedTaskNames:  sets.NewString(pts[0].Name),
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "03 - DAG task not finished, no final tasks",
		desc:               "DAG tasks (mytask1) started but not finished - do not schedule final tasks (mytask2)",
		state:              oneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{},
		expectedFinalNames: sets.NewString(pts[1].Name),
		expectedTaskNames:  sets.NewString(pts[0].Name),
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "04 - DAG task done, return final tasks",
		desc:               "DAG tasks (mytask1) done - schedule final tasks (mytask2)",
		state:              oneFinishedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{oneFinishedState[1]},
		expectedFinalNames: sets.NewString(pts[1].Name),
		expectedTaskNames:  sets.NewString(pts[0].Name),
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "05 - DAG task failed, return final tasks",
		desc:               "DAG task (mytask1) failed - schedule final tasks (mytask2)",
		state:              oneFailedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{oneFinishedState[1]},
		expectedFinalNames: sets.NewString(pts[1].Name),
		expectedTaskNames:  sets.NewString(pts[0].Name),
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "06 - DAG tasks succeeded, final tasks scheduled - no final tasks",
		desc:               "DAG task (mytask1) finished successfully - final task (mytask2) scheduled - no final tasks",
		state:              finalScheduledState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{},
		expectedFinalNames: sets.NewString(pts[1].Name),
		expectedTaskNames:  sets.NewString(pts[0].Name),
	}}
	for _, tc := range tcs {
		dagGraph, err := dag.Build(v1beta1.PipelineTaskList(tc.DAGTasks), v1beta1.PipelineTaskList(tc.DAGTasks).Deps())
		if err != nil {
			t.Fatalf("Unexpected error while building DAG for pipelineTasks %v: %v", tc.DAGTasks, err)
		}
		finalGraph, err := dag.Build(v1beta1.PipelineTaskList(tc.finalTasks), map[string][]string{})
		if err != nil {
			t.Fatalf("Unexpected error while building DAG for final pipelineTasks %v: %v", tc.finalTasks, err)
		}
		t.Run(tc.name, func(t *testing.T) {
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      dagGraph,
				FinalTasksGraph: finalGraph,
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			next := facts.GetFinalTasks()
			if d := cmp.Diff(tc.expectedFinalTasks, next); d != "" {
				t.Errorf("Didn't get expected final Tasks for %s (%s): %s", tc.name, tc.desc, diff.PrintWantGot(d))
			}

			finalTaskNames := facts.GetFinalTaskNames()
			if d := cmp.Diff(tc.expectedFinalNames, finalTaskNames); d != "" {
				t.Errorf("Didn't get expected final Task names for %s (%s): %s", tc.name, tc.desc, diff.PrintWantGot(d))
			}

			nonFinalTaskNames := facts.GetTaskNames()
			if d := cmp.Diff(tc.expectedTaskNames, nonFinalTaskNames); d != "" {
				t.Errorf("Didn't get expected non-final Task names for %s (%s): %s", tc.name, tc.desc, diff.PrintWantGot(d))
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

	var taskCancelledFailed = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelled(makeFailed(trs[0])),
	}}

	var taskCancelledFailedTimedOut = PipelineRunState{{
		PipelineTask: &pts[4],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      withCancelledForTimeout(makeFailed(trs[0])),
	}}

	var cancelledTask = PipelineRunState{{
		PipelineTask: &pts[3], // 1 retry needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
					Reason: v1beta1.TaskRunSpecStatusCancelled,
				}}},
			},
		},
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}

	var cancelledRun = PipelineRunState{{
		PipelineTask:  &pts[12],
		CustomTask:    true,
		RunObjectName: "pipelinerun-mytask13",
		RunObject: &v1beta1.CustomRun{
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
					Reason: v1beta1.CustomRunReasonCancelled.String(),
				}}},
			},
		},
	}}

	var timedOutRun = PipelineRunState{{
		PipelineTask:  &pts[12],
		CustomTask:    true,
		RunObjectName: "pipelinerun-mytask14",
		RunObject: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				StatusMessage: v1beta1.CustomRunCancelledByPipelineTimeoutMsg,
			},
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
					Reason: v1beta1.CustomRunReasonCancelled.String(),
				}}},
			},
		},
	}}

	var notRunningRun = PipelineRunState{{
		PipelineTask:  &pts[12],
		CustomTask:    true,
		RunObjectName: "pipelinerun-mytask14",
	}}

	// 6 Tasks, 4 that run in parallel in the beginning
	// Of the 4, 1 passed, 1 cancelled, 2 failed
	// 1 runAfter the passed one, currently running
	// 1 runAfter the failed one, which is marked as incomplete
	var taskMultipleFailuresSkipRunning = PipelineRunState{{
		TaskRunName:  "task0taskrun",
		PipelineTask: &pts[5],
		TaskRun:      makeSucceeded(trs[0]),
	}, {
		TaskRunName:  "runningTaskRun", // this is running
		PipelineTask: &pts[6],
		TaskRun:      makeStarted(trs[1]),
	}, {
		TaskRunName:  "failedTaskRun", // this failed
		PipelineTask: &pts[0],
		TaskRun:      makeFailed(trs[0]),
	}}

	var taskMultipleFailuresOneCancel = taskMultipleFailuresSkipRunning
	taskMultipleFailuresOneCancel = append(taskMultipleFailuresOneCancel, cancelledTask[0])

	var taskNotRunningWithSuccesfulParentsOneFailed = PipelineRunState{{
		TaskRunName:  "task0taskrun",
		PipelineTask: &pts[5],
		TaskRun:      makeSucceeded(trs[0]),
	}, {
		TaskRunName:  "notRunningTaskRun", // runAfter pts[5], not started yet
		PipelineTask: &pts[6],
		TaskRun:      nil,
	}, {
		TaskRunName:  "failedTaskRun", // this failed
		PipelineTask: &pts[0],
		TaskRun:      makeFailed(trs[0]),
	}}

	tenMinutesAgo := now.Add(-10 * time.Minute)
	fiveMinuteDuration := 5 * time.Minute

	tcs := []struct {
		name               string
		state              PipelineRunState
		finallyState       PipelineRunState
		specStatus         v1beta1.PipelineRunSpecStatus
		timeoutsState      PipelineRunTimeoutsState
		expectedStatus     corev1.ConditionStatus
		expectedReason     string
		expectedSucceeded  int
		expectedIncomplete int
		expectedSkipped    int
		expectedFailed     int
		expectedCancelled  int
	}{{
		name:               "no-tasks-started",
		state:              noneStartedState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedIncomplete: 2,
	}, {
		name:            "no-tasks-started-pipeline-run-gracefully-cancelled",
		state:           noneStartedState,
		specStatus:      v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		expectedStatus:  corev1.ConditionFalse,
		expectedReason:  v1beta1.PipelineRunReasonCancelled.String(),
		expectedSkipped: 2,
	}, {
		name:               "no-tasks-started-pipeline-run-with-finally-gracefully-cancelled",
		state:              noneStartedState,
		finallyState:       noneStartedState,
		specStatus:         v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonCancelledRunningFinally.String(),
		expectedIncomplete: 2,
	}, {
		name:               "no-tasks-started-pipeline-run-with-finally-gracefully-stopped",
		state:              noneStartedState,
		finallyState:       noneStartedState,
		specStatus:         v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonStoppedRunningFinally.String(),
		expectedIncomplete: 2,
	}, {
		name:               "one-task-started",
		state:              oneStartedState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedIncomplete: 2,
	}, {
		name:               "one-task-finished",
		state:              oneFinishedState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedSucceeded:  1,
		expectedIncomplete: 1,
	}, {
		name:              "one-task-finished-pipeline-run-gracefully-stopped",
		state:             oneFinishedState,
		specStatus:        v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		expectedStatus:    corev1.ConditionFalse,
		expectedReason:    v1beta1.PipelineRunReasonCancelled.String(),
		expectedSucceeded: 1,
		expectedSkipped:   1,
	}, {
		name:            "one-task-failed",
		state:           oneFailedState,
		expectedStatus:  corev1.ConditionFalse,
		expectedReason:  v1beta1.PipelineRunReasonFailed.String(),
		expectedFailed:  1,
		expectedSkipped: 1,
	}, {
		name:              "all-finished",
		state:             allFinishedState,
		expectedStatus:    corev1.ConditionTrue,
		expectedReason:    v1beta1.PipelineRunReasonSuccessful.String(),
		expectedSucceeded: 2,
	}, {
		name:               "one-retry-needed",
		state:              taskRetriedState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedIncomplete: 1,
	}, {
		name:              "task that was cancelled",
		state:             taskCancelledFailed,
		expectedReason:    v1beta1.PipelineRunReasonCancelled.String(),
		expectedStatus:    corev1.ConditionFalse,
		expectedCancelled: 1,
	}, {
		name:           "task that was cancelled for timeout",
		state:          taskCancelledFailedTimedOut,
		expectedReason: v1beta1.PipelineRunReasonFailed.String(),
		expectedStatus: corev1.ConditionFalse,
		expectedFailed: 1,
	}, {
		name:               "task with multiple failures",
		state:              taskMultipleFailuresSkipRunning,
		expectedReason:     v1beta1.PipelineRunReasonStopping.String(),
		expectedStatus:     corev1.ConditionUnknown,
		expectedSucceeded:  1,
		expectedFailed:     1,
		expectedIncomplete: 1,
		expectedCancelled:  0,
		expectedSkipped:    0,
	}, {
		name:               "task with multiple failures; one cancelled",
		state:              taskMultipleFailuresOneCancel,
		expectedReason:     v1beta1.PipelineRunReasonStopping.String(),
		expectedStatus:     corev1.ConditionUnknown,
		expectedSucceeded:  1,
		expectedFailed:     1,
		expectedIncomplete: 1,
		expectedCancelled:  1,
		expectedSkipped:    0,
	}, {
		name:              "task not started with passed parent; one failed",
		state:             taskNotRunningWithSuccesfulParentsOneFailed,
		expectedReason:    v1beta1.PipelineRunReasonFailed.String(),
		expectedStatus:    corev1.ConditionFalse,
		expectedSucceeded: 1,
		expectedFailed:    1,
		expectedSkipped:   1,
	}, {
		name:              "cancelled task should result in cancelled pipeline",
		state:             cancelledTask,
		expectedStatus:    corev1.ConditionFalse,
		expectedReason:    v1beta1.PipelineRunReasonCancelled.String(),
		expectedCancelled: 1,
	}, {
		name:              "cancelled run should result in cancelled pipeline",
		state:             cancelledRun,
		expectedStatus:    corev1.ConditionFalse,
		expectedReason:    v1beta1.PipelineRunReasonCancelled.String(),
		expectedCancelled: 1,
	}, {
		name:           "cancelled for timeout run should result in failed pipeline",
		state:          timedOutRun,
		expectedStatus: corev1.ConditionFalse,
		expectedReason: v1beta1.PipelineRunReasonFailed.String(),
		expectedFailed: 1,
	}, {
		name:  "skipped for timeout run should result in failed pipeline",
		state: notRunningRun,
		timeoutsState: PipelineRunTimeoutsState{
			StartTime:    &tenMinutesAgo,
			TasksTimeout: &fiveMinuteDuration,
		},
		expectedStatus:  corev1.ConditionFalse,
		expectedReason:  v1beta1.PipelineRunReasonFailed.String(),
		expectedSkipped: 1,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somepipelinerun",
				},
				Spec: v1beta1.PipelineRunSpec{
					Status: tc.specStatus,
				},
			}
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Unexpected error while building DAG for state %v: %v", tc.state, err)
			}
			dfinally, err := dagFromState(tc.finallyState)
			if err != nil {
				t.Fatalf("Unexpected error while building DAG for finally state %v: %v", tc.finallyState, err)
			}

			timeoutsState := tc.timeoutsState
			if timeoutsState.Clock == nil {
				timeoutsState.Clock = testClock
			}

			facts := PipelineRunFacts{
				State:           tc.state,
				SpecStatus:      tc.specStatus,
				TasksGraph:      d,
				FinalTasksGraph: dfinally,
				TimeoutsState:   timeoutsState,
			}
			c := facts.GetPipelineConditionStatus(context.Background(), pr, zap.NewNop().Sugar(), testClock)
			wantCondition := &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: tc.expectedStatus,
				Reason: tc.expectedReason,
				Message: getExpectedMessage(pr.Name, tc.specStatus, tc.expectedStatus, tc.expectedSucceeded,
					tc.expectedIncomplete, tc.expectedSkipped, tc.expectedFailed, tc.expectedCancelled),
			}
			if d := cmp.Diff(wantCondition, c); d != "" {
				t.Fatalf("Mismatch in condition %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetPipelineConditionStatus_WithFinalTasks(t *testing.T) {
	// pipeline state with one DAG successful, one final task failed
	dagSucceededFinalFailed := PipelineRunState{{
		TaskRunName:  "task0taskrun",
		PipelineTask: &pts[0],
		TaskRun:      makeSucceeded(trs[0]),
	}, {
		TaskRunName:  "failedTaskRun",
		PipelineTask: &pts[1],
		TaskRun:      makeFailed(trs[0]),
	}}

	// pipeline state with one DAG failed, no final started
	dagFailedFinalNotStarted := PipelineRunState{{
		TaskRunName:  "task0taskrun",
		PipelineTask: &pts[0],
		TaskRun:      makeFailed(trs[0]),
	}, {
		TaskRunName:  "notRunningTaskRun",
		PipelineTask: &pts[1],
		TaskRun:      nil,
	}}

	// pipeline state with one DAG failed, one final task failed
	dagFailedFinalFailed := PipelineRunState{{
		TaskRunName:  "task0taskrun",
		PipelineTask: &pts[0],
		TaskRun:      makeFailed(trs[0]),
	}, {
		TaskRunName:  "failedTaskRun",
		PipelineTask: &pts[1],
		TaskRun:      makeFailed(trs[0]),
	}}

	// pipeline state with one DAG failed, one final task skipped
	dagFailedFinalSkipped := PipelineRunState{{
		TaskRunName:  "task0taskrun",
		PipelineTask: &pts[0],
		TaskRun:      makeFailed(trs[0]),
	}, {
		PipelineTask: &pts[14],
	}}

	tcs := []struct {
		name               string
		state              PipelineRunState
		dagTasks           []v1beta1.PipelineTask
		finalTasks         []v1beta1.PipelineTask
		expectedStatus     corev1.ConditionStatus
		expectedReason     string
		expectedSucceeded  int
		expectedIncomplete int
		expectedSkipped    int
		expectedFailed     int
		expectedCancelled  int
	}{{
		name:               "pipeline with one successful DAG task and failed final task",
		state:              dagSucceededFinalFailed,
		dagTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedStatus:     corev1.ConditionFalse,
		expectedReason:     v1beta1.PipelineRunReasonFailed.String(),
		expectedSucceeded:  1,
		expectedIncomplete: 0,
		expectedSkipped:    0,
		expectedFailed:     1,
		expectedCancelled:  0,
	}, {
		name:               "pipeline with one failed DAG task and not started final task",
		state:              dagFailedFinalNotStarted,
		dagTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedSucceeded:  0,
		expectedIncomplete: 1,
		expectedSkipped:    0,
		expectedFailed:     1,
		expectedCancelled:  0,
	}, {
		name:               "pipeline with one failed DAG task and failed final task",
		state:              dagFailedFinalFailed,
		dagTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedStatus:     corev1.ConditionFalse,
		expectedReason:     v1beta1.PipelineRunReasonFailed.String(),
		expectedSucceeded:  0,
		expectedIncomplete: 0,
		expectedSkipped:    0,
		expectedFailed:     2,
		expectedCancelled:  0,
	}, {
		name:               "pipeline with one failed DAG task and skipped final task",
		state:              dagFailedFinalSkipped,
		dagTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[14]},
		expectedStatus:     corev1.ConditionFalse,
		expectedReason:     v1beta1.PipelineRunReasonFailed.String(),
		expectedSucceeded:  0,
		expectedIncomplete: 0,
		expectedSkipped:    1,
		expectedFailed:     1,
		expectedCancelled:  0,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun-final-tasks",
				},
				Spec: v1beta1.PipelineRunSpec{},
			}
			d, err := dag.Build(v1beta1.PipelineTaskList(tc.dagTasks), v1beta1.PipelineTaskList(tc.dagTasks).Deps())
			if err != nil {
				t.Fatalf("Unexpected error while building graph for DAG tasks %v: %v", tc.dagTasks, err)
			}
			df, err := dag.Build(v1beta1.PipelineTaskList(tc.finalTasks), map[string][]string{})
			if err != nil {
				t.Fatalf("Unexpected error while building graph for final tasks %v: %v", tc.finalTasks, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: df,
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			c := facts.GetPipelineConditionStatus(context.Background(), pr, zap.NewNop().Sugar(), testClock)
			wantCondition := &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: tc.expectedStatus,
				Reason: tc.expectedReason,
				Message: getExpectedMessage(pr.Name, "", tc.expectedStatus, tc.expectedSucceeded,
					tc.expectedIncomplete, tc.expectedSkipped, tc.expectedFailed, tc.expectedCancelled),
			}
			if d := cmp.Diff(wantCondition, c); d != "" {
				t.Fatalf("Mismatch in condition %s", diff.PrintWantGot(d))
			}
		})
	}
}

// pipeline should result in timeout if its runtime exceeds its spec.Timeout based on its status.Timeout
func TestGetPipelineConditionStatus_PipelineTimeoutDeprecated(t *testing.T) {
	d, err := dagFromState(oneFinishedState)
	if err != nil {
		t.Fatalf("Unexpected error while building DAG for state %v: %v", oneFinishedState, err)
	}
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-no-tasks-started"},
		Spec: v1beta1.PipelineRunSpec{
			Timeout: &metav1.Duration{Duration: 1 * time.Minute},
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now.Add(-2 * time.Minute)},
			},
		},
	}
	facts := PipelineRunFacts{
		State:           oneFinishedState,
		TasksGraph:      d,
		FinalTasksGraph: &dag.Graph{},
		TimeoutsState: PipelineRunTimeoutsState{
			Clock: testClock,
		},
	}
	c := facts.GetPipelineConditionStatus(context.Background(), pr, zap.NewNop().Sugar(), testClock)
	if c.Status != corev1.ConditionFalse && c.Reason != v1beta1.PipelineRunReasonTimedOut.String() {
		t.Fatalf("Expected to get status %s but got %s for state %v", corev1.ConditionFalse, c.Status, oneFinishedState)
	}
}

// pipeline should result in timeout if its runtime exceeds its spec.Timeouts.Pipeline based on its status.Timeout
func TestGetPipelineConditionStatus_PipelineTimeouts(t *testing.T) {
	d, err := dagFromState(oneFinishedState)
	if err != nil {
		t.Fatalf("Unexpected error while building DAG for state %v: %v", oneFinishedState, err)
	}
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-no-tasks-started"},
		Spec: v1beta1.PipelineRunSpec{
			Timeouts: &v1beta1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Minute},
			},
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now.Add(-2 * time.Minute)},
			},
		},
	}
	facts := PipelineRunFacts{
		State:           oneFinishedState,
		TasksGraph:      d,
		FinalTasksGraph: &dag.Graph{},
		TimeoutsState: PipelineRunTimeoutsState{
			Clock: testClock,
		},
	}
	c := facts.GetPipelineConditionStatus(context.Background(), pr, zap.NewNop().Sugar(), testClock)
	if c.Status != corev1.ConditionFalse && c.Reason != v1beta1.PipelineRunReasonTimedOut.String() {
		t.Fatalf("Expected to get status %s but got %s for state %v", corev1.ConditionFalse, c.Status, oneFinishedState)
	}
}

func TestAdjustStartTime(t *testing.T) {
	baseline := metav1.Time{Time: now}

	tests := []struct {
		name string
		prs  PipelineRunState
		want time.Time
	}{{
		name: "same times",
		prs: PipelineRunState{{
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah",
					CreationTimestamp: baseline,
				},
			},
		}},
		want: baseline.Time,
	}, {
		name: "taskrun starts later",
		prs: PipelineRunState{{
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah",
					CreationTimestamp: metav1.Time{Time: baseline.Time.Add(1 * time.Second)},
				},
			},
		}},
		// Stay where you are, you are before the TaskRun.
		want: baseline.Time,
	}, {
		name: "taskrun starts earlier",
		prs: PipelineRunState{{
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah",
					CreationTimestamp: metav1.Time{Time: baseline.Time.Add(-1 * time.Second)},
				},
			},
		}},
		// We expect this to adjust to the earlier time.
		want: baseline.Time.Add(-1 * time.Second),
	}, {
		name: "multiple taskruns, some earlier",
		prs: PipelineRunState{{
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah1",
					CreationTimestamp: metav1.Time{Time: baseline.Time.Add(-1 * time.Second)},
				},
			},
		}, {
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah2",
					CreationTimestamp: metav1.Time{Time: baseline.Time.Add(-2 * time.Second)},
				},
			},
		}, {
			TaskRun: nil,
		}, {
			TaskRun: &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah3",
					CreationTimestamp: metav1.Time{Time: baseline.Time.Add(2 * time.Second)},
				},
			},
		}},
		// We expect this to adjust to the earlier time.
		want: baseline.Time.Add(-2 * time.Second),
	}, {
		name: "run starts later",
		prs: PipelineRunState{{
			RunObject: &v1beta1.CustomRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah",
					CreationTimestamp: metav1.Time{Time: baseline.Time.Add(1 * time.Second)},
				},
			},
		}},
		// Stay where you are, you are before the Run.
		want: baseline.Time,
	}, {
		name: "run starts earlier",
		prs: PipelineRunState{{
			RunObject: &v1beta1.CustomRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "blah",
					CreationTimestamp: metav1.Time{Time: baseline.Time.Add(-1 * time.Second)},
				},
			},
		}},
		// We expect this to adjust to the earlier time.
		want: baseline.Time.Add(-1 * time.Second),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.prs.AdjustStartTime(&baseline)
			if got.Time != test.want {
				t.Errorf("AdjustStartTime() = %v, wanted %v", got.Time, test.want)
			}
		})
	}
}

func TestPipelineRunFacts_GetPipelineTaskStatus(t *testing.T) {
	tcs := []struct {
		name           string
		state          PipelineRunState
		dagTasks       []v1beta1.PipelineTask
		expectedStatus map[string]string
	}{{
		name:     "no-tasks-started",
		state:    noneStartedState,
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[1]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[0].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			PipelineTaskStatusPrefix + pts[1].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                              PipelineTaskStateNone,
		},
	}, {
		name:     "one-task-started",
		state:    oneStartedState,
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[1]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[0].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			PipelineTaskStatusPrefix + pts[1].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                              PipelineTaskStateNone,
		},
	}, {
		name:     "one-task-finished",
		state:    oneFinishedState,
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[1]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[0].Name + PipelineTaskStatusSuffix: v1beta1.TaskRunReasonSuccessful.String(),
			PipelineTaskStatusPrefix + pts[1].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                              PipelineTaskStateNone,
		},
	}, {
		name:     "one-task-failed",
		state:    oneFailedState,
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[1]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[0].Name + PipelineTaskStatusSuffix: v1beta1.TaskRunReasonFailed.String(),
			PipelineTaskStatusPrefix + pts[1].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                              v1beta1.PipelineRunReasonFailed.String(),
		},
	}, {
		name:     "all-finished",
		state:    allFinishedState,
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[1]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[0].Name + PipelineTaskStatusSuffix: v1beta1.TaskRunReasonSuccessful.String(),
			PipelineTaskStatusPrefix + pts[1].Name + PipelineTaskStatusSuffix: v1beta1.TaskRunReasonSuccessful.String(),
			v1beta1.PipelineTasksAggregateStatus:                              v1beta1.PipelineRunReasonSuccessful.String(),
		},
	}, {
		name: "task-with-when-expressions-passed",
		state: PipelineRunState{{
			PipelineTask: &pts[9],
			TaskRunName:  "pr-guard-succeeded-task-not-started",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		dagTasks: []v1beta1.PipelineTask{pts[9]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[9].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                              PipelineTaskStateNone,
		},
	}, {
		name: "tasks-when-expression-failed-and-task-skipped",
		state: PipelineRunState{{
			PipelineTask: &pts[10],
			TaskRunName:  "pr-guardedtask-skipped",
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		dagTasks: []v1beta1.PipelineTask{pts[10]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[10].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                               v1beta1.PipelineRunReasonCompleted.String(),
		},
	}, {
		name: "when-expression-task-with-parent-started",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRun:      makeStarted(trs[0]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[11],
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[11]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[0].Name + PipelineTaskStatusSuffix:  PipelineTaskStateNone,
			PipelineTaskStatusPrefix + pts[11].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                               PipelineTaskStateNone,
		},
	}, {
		name:     "task-cancelled",
		state:    taskCancelled,
		dagTasks: []v1beta1.PipelineTask{pts[4]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[4].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                              PipelineTaskStateNone,
		},
	}, {
		name: "one-skipped-one-failed-aggregate-status-must-be-failed",
		state: PipelineRunState{{
			PipelineTask: &pts[10],
			TaskRunName:  "pr-guardedtask-skipped",
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[0],
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[10]},
		expectedStatus: map[string]string{
			PipelineTaskStatusPrefix + pts[0].Name + PipelineTaskStatusSuffix:  v1beta1.PipelineRunReasonFailed.String(),
			PipelineTaskStatusPrefix + pts[10].Name + PipelineTaskStatusSuffix: PipelineTaskStateNone,
			v1beta1.PipelineTasksAggregateStatus:                               v1beta1.PipelineRunReasonFailed.String(),
		},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dag.Build(v1beta1.PipelineTaskList(tc.dagTasks), v1beta1.PipelineTaskList(tc.dagTasks).Deps())
			if err != nil {
				t.Fatalf("Unexpected error while building graph for DAG tasks %v: %v", tc.dagTasks, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			s := facts.GetPipelineTaskStatus()
			if d := cmp.Diff(tc.expectedStatus, s); d != "" {
				t.Fatalf("Test failed: %s Mismatch in pipelineTask execution state %s", tc.name, diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunFacts_GetSkippedTasks(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		state                PipelineRunState
		dagTasks             []v1beta1.PipelineTask
		finallyTasks         []v1beta1.PipelineTask
		expectedSkippedTasks []v1beta1.SkippedTask
	}{{
		name: "stopping-skip-taskruns",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRun:      makeFailed(trs[0]),
		}, {
			PipelineTask: &pts[14],
		}},
		dagTasks: []v1beta1.PipelineTask{pts[0], pts[14]},
		expectedSkippedTasks: []v1beta1.SkippedTask{{
			Name:   pts[14].Name,
			Reason: v1beta1.StoppingSkip,
		}},
	}, {
		name: "missing-results-skip-finally",
		state: PipelineRunState{{
			TaskRunName:  "task0taskrun",
			PipelineTask: &pts[0],
			TaskRun:      makeFailed(trs[0]),
		}, {
			PipelineTask: &pts[14],
		}},
		dagTasks:     []v1beta1.PipelineTask{pts[0]},
		finallyTasks: []v1beta1.PipelineTask{pts[14]},
		expectedSkippedTasks: []v1beta1.SkippedTask{{
			Name:   pts[14].Name,
			Reason: v1beta1.MissingResultsSkip,
		}},
	}, {
		name: "when-expressions-skip-finally",
		state: PipelineRunState{{
			PipelineTask: &pts[10],
		}},
		finallyTasks: []v1beta1.PipelineTask{pts[10]},
		expectedSkippedTasks: []v1beta1.SkippedTask{{
			Name:   pts[10].Name,
			Reason: v1beta1.WhenExpressionsSkip,
			WhenExpressions: []v1beta1.WhenExpression{{
				Input:    "foo",
				Operator: "notin",
				Values:   []string{"foo", "bar"},
			}},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dag.Build(v1beta1.PipelineTaskList(tc.dagTasks), v1beta1.PipelineTaskList(tc.dagTasks).Deps())
			if err != nil {
				t.Fatalf("Unexpected error while building graph for DAG tasks %v: %v", v1beta1.PipelineTaskList{pts[0]}, err)
			}
			df, err := dag.Build(v1beta1.PipelineTaskList(tc.finallyTasks), map[string][]string{})
			if err != nil {
				t.Fatalf("Unexpected error while building graph for final tasks %v: %v", v1beta1.PipelineTaskList{pts[14]}, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: df,
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			actualSkippedTasks := facts.GetSkippedTasks()
			if d := cmp.Diff(tc.expectedSkippedTasks, actualSkippedTasks); d != "" {
				t.Fatalf("Mismatch skipped tasks %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunFacts_IsRunning(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    PipelineRunState
		expected bool
	}{{
		name:     "one-started",
		state:    oneStartedState,
		expected: true,
	}, {
		name:     "one-finished",
		state:    oneFinishedState,
		expected: false,
	}, {
		name:     "one-failed",
		state:    oneFailedState,
		expected: false,
	}, {
		name:     "all-finished",
		state:    allFinishedState,
		expected: false,
	}, {
		name:     "no-run-started",
		state:    noRunStartedState,
		expected: false,
	}, {
		name:     "one-run-started",
		state:    oneRunStartedState,
		expected: true,
	}, {
		name:     "one-run-failed",
		state:    oneRunFailedState,
		expected: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Could not get a dag from the TC state %#v: %v", tc.state, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			if tc.expected != facts.IsRunning() {
				t.Errorf("IsRunning expected to be %v", tc.expected)
			}
		})
	}
}

// TestUpdateTaskRunsState runs "getTaskRunsStatus" and verifies how it updates a PipelineRun status
// from a TaskRun associated to the PipelineRun
func TestUpdateTaskRunsState(t *testing.T) {
	pr := parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
`)

	pipelineTask := v1beta1.PipelineTask{
		Name: "unit-test-1",
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
		TaskRef: &v1beta1.TaskRef{Name: "unit-test-task"},
	}
	unscheduledPipelineTask := v1beta1.PipelineTask{
		Name: "unit-test-2",
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
		TaskRef: &v1beta1.TaskRef{Name: "unit-test-task"},
	}

	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: unit-test-task
  namespace: foo
spec:
  resources:
    inputs:
      - name: workspace
        type: %s
`, resourcev1alpha1.PipelineResourceTypeGit))

	taskrun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(`
metadata:
  name: test-pipeline-run-success-unit-test-1
  namespace: foo
spec:
  taskRef:
    name: unit-test-task
  serviceAccountName: test-sa
  timeout: 1h0m0s
status:
  conditions:
    - type: Succeeded
  steps:
    - container:
      terminated:
        exitCode: 0
`))

	state := PipelineRunState{{
		PipelineTask: &pipelineTask,
		TaskRunName:  "test-pipeline-run-success-unit-test-1",
		TaskRun:      taskrun,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}, {
		PipelineTask: &unscheduledPipelineTask,
		TaskRunName:  "test-pipeline-run-success-unit-test-2",
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}
	pr.Status.InitializeConditions(testClock)
	status := state.GetTaskRunsStatus(pr)

	expectedPipelineRunStatus := parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: pipelinerun
  namespace: foo
status:
  taskRuns:
    test-pipeline-run-success-unit-test-1:
      pipelineTaskName: unit-test-1
      status:
        conditions:
          - type: Succeeded
        steps:
          - container:
            terminated:
              exitCode: 0
      whenExpressions:
        - input: foo
          operator: in
          values: ["foo", "bar"]
`)

	if d := cmp.Diff(expectedPipelineRunStatus.Status.TaskRuns, status); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", diff.PrintWantGot(d))
	}
}

// TestUpdateRunsState runs "getRunsStatus" and verifies how it updates a PipelineRun status
// from a Run associated to the PipelineRun
func TestUpdateRunsState(t *testing.T) {
	pr := parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: test-pipeline-run
  namespace: foo
spec:
  pipelineRef:
    name: test-pipeline
`)

	pipelineTask := v1beta1.PipelineTask{
		Name: "unit-test-1",
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
		TaskRef: &v1beta1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
			Name:       "unit-test-run",
		},
	}
	unscheduledPipelineTask := v1beta1.PipelineTask{
		Name: "unit-test-2",
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
		TaskRef: &v1beta1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
			Name:       "unit-test-run",
		},
	}

	run := parse.MustParseCustomRun(t, fmt.Sprintf(`
metadata:
  name: unit-test-run
  namespace: foo
status:
  conditions:
    - type: Succeeded
      status: "True"
`))

	state := PipelineRunState{{
		PipelineTask:  &pipelineTask,
		CustomTask:    true,
		RunObjectName: "test-pipeline-run-success-unit-test-1",
		RunObject:     run,
	}, {
		PipelineTask:  &unscheduledPipelineTask,
		CustomTask:    true,
		RunObjectName: "test-pipeline-run-success-unit-test-2",
	}}
	pr.Status.InitializeConditions(testClock)
	status := state.GetRunsStatus(pr)

	expectedPipelineRunStatus := parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: pipelinerun
  namespace: foo
status:
  runs:
    test-pipeline-run-success-unit-test-1:
      pipelineTaskName: unit-test-1
      status:
        conditions:
          - type: Succeeded
            status: "True"
        steps:
          - container:
            terminated:
              exitCode: 0
      whenExpressions:
        - input: foo
          operator: in
          values: ["foo", "bar"]
`)

	if d := cmp.Diff(expectedPipelineRunStatus.Status.Runs, status); d != "" {
		t.Fatalf("Expected PipelineRun status to match Run(s) status, but got a mismatch: %s", diff.PrintWantGot(d))
	}
}

func TestPipelineRunState_GetResultsFuncs(t *testing.T) {
	state := PipelineRunState{{
		TaskRunName: "successful-task-with-results",
		PipelineTask: &v1beta1.PipelineTask{
			Name: "successful-task-with-results-1",
		},
		TaskRun: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}}},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{{
						Name:  "foo",
						Value: *v1beta1.NewStructuredValues("oof"),
					}, {
						Name:  "bar",
						Value: *v1beta1.NewStructuredValues("rab"),
					}},
				},
			},
		},
	}, {
		TaskRunName: "successful-task-without-results",
		PipelineTask: &v1beta1.PipelineTask{
			Name: "successful-task-without-results-1",
		},
		TaskRun: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}}},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{},
			},
		},
	}, {
		TaskRunName: "failed-task",
		PipelineTask: &v1beta1.PipelineTask{
			Name: "failed-task-1",
		},
		TaskRun: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}}},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{{
						Name:  "fail-foo",
						Value: *v1beta1.NewStructuredValues("fail-oof"),
					}},
				},
			},
		},
	}, {
		TaskRunName: "incomplete-task",
		PipelineTask: &v1beta1.PipelineTask{
			Name: "incomplete-task-1",
		},
		TaskRun: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
				}}},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{{
						Name:  "unknown-foo",
						Value: *v1beta1.NewStructuredValues("unknown-oof"),
					}},
				},
			},
		},
	}, {
		TaskRunName: "nil-taskrun",
		PipelineTask: &v1beta1.PipelineTask{
			Name: "nil-taskrun-1",
		},
	}, {
		RunObjectName: "successful-run-with-results",
		CustomTask:    true,
		PipelineTask: &v1beta1.PipelineTask{
			Name: "successful-run-with-results-1",
		},
		RunObject: &v1beta1.CustomRun{
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}}},
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					Results: []v1beta1.CustomRunResult{{
						Name:  "foo",
						Value: "oof",
					}, {
						Name:  "bar",
						Value: "rab",
					}},
				},
			},
		},
	}, {
		RunObjectName: "successful-run-without-results",
		CustomTask:    true,
		PipelineTask: &v1beta1.PipelineTask{
			Name: "successful-run-without-results-1",
		},
		RunObject: &v1beta1.CustomRun{
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}}},
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{},
			},
		},
	}, {
		RunObjectName: "failed-run",
		PipelineTask: &v1beta1.PipelineTask{
			Name: "failed-run-1",
		},
		RunObject: &v1beta1.CustomRun{
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}}},
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					Results: []v1beta1.CustomRunResult{{
						Name:  "fail-foo",
						Value: "fail-oof",
					}},
				},
			},
		},
	}, {
		RunObjectName: "incomplete-run",
		PipelineTask: &v1beta1.PipelineTask{
			Name: "incomplete-run-1",
		},
		RunObject: &v1beta1.CustomRun{
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
				}}},
				CustomRunStatusFields: v1beta1.CustomRunStatusFields{
					Results: []v1beta1.CustomRunResult{{
						Name:  "unknown-foo",
						Value: "unknown-oof",
					}},
				},
			},
		},
	}, {
		RunObjectName: "nil-run",
		CustomTask:    true,
		PipelineTask: &v1beta1.PipelineTask{
			Name: "nil-run-1",
		},
	}, {
		TaskRunNames: []string{
			"matrixed-task-run-0",
			"matrixed-task-run-1",
			"matrixed-task-run-2",
			"matrixed-task-run-3",
		},
		PipelineTask: &v1beta1.PipelineTask{
			Name: "matrixed-task",
			TaskRef: &v1beta1.TaskRef{
				Name:       "task",
				Kind:       "Task",
				APIVersion: "v1beta1",
			},
			Matrix: &v1beta1.Matrix{
				Params: []v1beta1.Param{{
					Name:  "foobar",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}, {
					Name:  "quxbaz",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"qux", "baz"}},
				}}},
		},
		TaskRuns: []*v1beta1.TaskRun{{
			TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
			ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-0"},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1beta1.TaskRunReasonSuccessful.String(),
				}}},
			},
		}, {
			TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
			ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-1"},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1beta1.TaskRunReasonSuccessful.String(),
				}}},
			},
		}, {
			TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
			ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-2"},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1beta1.TaskRunReasonSuccessful.String(),
				}}},
			},
		}, {
			TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
			ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-3"},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1beta1.TaskRunReasonSuccessful.String(),
				}}},
			},
		}},
	}, {
		RunObjectNames: []string{
			"matrixed-run-0",
			"matrixed-run-1",
			"matrixed-run-2",
			"matrixed-run-3",
		},
		PipelineTask: &v1beta1.PipelineTask{
			Name: "matrixed-task",
			TaskRef: &v1beta1.TaskRef{
				Kind:       "Example",
				APIVersion: "example.dev/v0",
			},
			Matrix: &v1beta1.Matrix{
				Params: []v1beta1.Param{{
					Name:  "foobar",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}, {
					Name:  "quxbaz",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"qux", "baz"}},
				}}},
		},
		RunObjects: []v1beta1.RunObject{
			&v1beta1.CustomRun{
				TypeMeta:   metav1.TypeMeta{APIVersion: "example.dev/v0"},
				ObjectMeta: metav1.ObjectMeta{Name: "matrixed-run-0"},
				Status: v1beta1.CustomRunStatus{
					Status: duckv1.Status{Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}}},
					CustomRunStatusFields: v1beta1.CustomRunStatusFields{
						Results: []v1beta1.CustomRunResult{{
							Name:  "foo",
							Value: "oof",
						}, {
							Name:  "bar",
							Value: "rab",
						}},
					},
				},
			}, &v1beta1.CustomRun{
				TypeMeta:   metav1.TypeMeta{APIVersion: "example.dev/v0"},
				ObjectMeta: metav1.ObjectMeta{Name: "matrixed-run-1"},
				Status: v1beta1.CustomRunStatus{
					Status: duckv1.Status{Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}}},
					CustomRunStatusFields: v1beta1.CustomRunStatusFields{
						Results: []v1beta1.CustomRunResult{{
							Name:  "foo",
							Value: "oof",
						}, {
							Name:  "bar",
							Value: "rab",
						}},
					},
				},
			}, &v1beta1.CustomRun{
				TypeMeta:   metav1.TypeMeta{APIVersion: "example.dev/v0"},
				ObjectMeta: metav1.ObjectMeta{Name: "matrixed-run-2"},
				Status: v1beta1.CustomRunStatus{
					Status: duckv1.Status{Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}}},
					CustomRunStatusFields: v1beta1.CustomRunStatusFields{
						Results: []v1beta1.CustomRunResult{{
							Name:  "foo",
							Value: "oof",
						}, {
							Name:  "bar",
							Value: "rab",
						}},
					},
				},
			}, &v1beta1.CustomRun{
				TypeMeta:   metav1.TypeMeta{APIVersion: "example.dev/v0"},
				ObjectMeta: metav1.ObjectMeta{Name: "matrixed-run-3"},
				Status: v1beta1.CustomRunStatus{
					Status: duckv1.Status{Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}}},
					CustomRunStatusFields: v1beta1.CustomRunStatusFields{
						Results: []v1beta1.CustomRunResult{{
							Name:  "foo",
							Value: "oof",
						}, {
							Name:  "bar",
							Value: "rab",
						}},
					},
				},
			},
		},
	}}

	expectedTaskResults := map[string][]v1beta1.TaskRunResult{
		"successful-task-with-results-1": {{
			Name:  "foo",
			Value: *v1beta1.NewStructuredValues("oof"),
		}, {
			Name:  "bar",
			Value: *v1beta1.NewStructuredValues("rab"),
		}},
		"successful-task-without-results-1": nil,
	}
	expectedRunResults := map[string][]v1beta1.CustomRunResult{
		"successful-run-with-results-1": {{
			Name:  "foo",
			Value: "oof",
		}, {
			Name:  "bar",
			Value: "rab",
		}},
		"successful-run-without-results-1": nil,
	}

	actualTaskResults := state.GetTaskRunsResults()
	if d := cmp.Diff(expectedTaskResults, actualTaskResults); d != "" {
		t.Errorf("Didn't get expected TaskRun results map: %s", diff.PrintWantGot(d))
	}

	actualRunResults := state.GetRunsResults()
	if d := cmp.Diff(expectedRunResults, actualRunResults); d != "" {
		t.Errorf("Didn't get expected Run results map: %s", diff.PrintWantGot(d))
	}
}

func TestPipelineRunState_GetChildReferences(t *testing.T) {
	testCases := []struct {
		name      string
		state     PipelineRunState
		childRefs []v1beta1.ChildStatusReference
	}{
		{
			name:      "no-tasks",
			state:     PipelineRunState{},
			childRefs: nil,
		},
		{
			name: "unresolved-task",
			state: PipelineRunState{{
				TaskRunName: "unresolved-task-run",
				PipelineTask: &v1beta1.PipelineTask{
					Name: "unresolved-task-1",
					TaskRef: &v1beta1.TaskRef{
						Name:       "unresolved-task",
						Kind:       "Task",
						APIVersion: "v1beta1",
					},
				},
			}},
			childRefs: nil,
		},
		{
			name: "unresolved-custom-task",
			state: PipelineRunState{{
				RunObjectName: "unresolved-custom-task-run",
				CustomTask:    true,
				PipelineTask: &v1beta1.PipelineTask{
					Name: "unresolved-custom-task-1",
					TaskRef: &v1beta1.TaskRef{
						APIVersion: "example.dev/v0",
						Kind:       "Example",
						Name:       "unresolved-custom-task",
					},
				},
			}},
			childRefs: nil,
		},
		{
			name: "single-task",
			state: PipelineRunState{{
				TaskRunName: "single-task-run",
				PipelineTask: &v1beta1.PipelineTask{
					Name: "single-task-1",
					TaskRef: &v1beta1.TaskRef{
						Name:       "single-task",
						Kind:       "Task",
						APIVersion: "v1beta1",
					},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"foo", "bar"},
					}},
				},
				TaskRun: &v1beta1.TaskRun{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "single-task-run"},
				},
			}},
			childRefs: []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "TaskRun",
				},
				Name:             "single-task-run",
				PipelineTaskName: "single-task-1",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}},
		},
		{
			name: "single-custom-task",
			state: PipelineRunState{{
				RunObjectName: "single-custom-task-run",
				CustomTask:    true,
				PipelineTask: &v1beta1.PipelineTask{
					Name: "single-custom-task-1",
					TaskRef: &v1beta1.TaskRef{
						APIVersion: "example.dev/v0",
						Kind:       "Example",
						Name:       "single-custom-task",
					},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"foo", "bar"},
					}},
				},
				RunObject: &v1beta1.CustomRun{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "single-custom-task-run"},
				},
			}},
			childRefs: []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "CustomRun",
				},
				Name:             "single-custom-task-run",
				PipelineTaskName: "single-custom-task-1",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}},
		},
		{
			name: "task-and-custom-task",
			state: PipelineRunState{{
				TaskRunName: "single-task-run",
				PipelineTask: &v1beta1.PipelineTask{
					Name: "single-task-1",
					TaskRef: &v1beta1.TaskRef{
						Name:       "single-task",
						Kind:       "Task",
						APIVersion: "v1beta1",
					},
				},
				TaskRun: &v1beta1.TaskRun{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "single-task-run"},
				},
			}, {
				RunObjectName: "single-custom-task-run",
				CustomTask:    true,
				PipelineTask: &v1beta1.PipelineTask{
					Name: "single-custom-task-1",
					TaskRef: &v1beta1.TaskRef{
						APIVersion: "example.dev/v0",
						Kind:       "Example",
						Name:       "single-custom-task",
					},
				},
				RunObject: &v1beta1.CustomRun{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "single-custom-task-run"},
				},
			}},
			childRefs: []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "TaskRun",
				},
				Name:             "single-task-run",
				PipelineTaskName: "single-task-1",
			}, {
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "CustomRun",
				},
				Name:             "single-custom-task-run",
				PipelineTaskName: "single-custom-task-1",
			}},
		},
		{
			name: "unresolved-matrixed-task",
			state: PipelineRunState{{
				TaskRunNames: []string{"task-run-0", "task-run-1", "task-run-2", "task-run-3"},
				PipelineTask: &v1beta1.PipelineTask{
					Name: "matrixed-task",
					TaskRef: &v1beta1.TaskRef{
						Name:       "task",
						Kind:       "Task",
						APIVersion: "v1beta1",
					},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"foo", "bar"},
					}},
					Matrix: &v1beta1.Matrix{
						Params: []v1beta1.Param{{
							Name:  "foobar",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
						}, {
							Name:  "quxbaz",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"qux", "baz"}},
						}}},
				},
				TaskRuns: []*v1beta1.TaskRun{nil, nil, nil, nil},
			}},
			childRefs: nil,
		},
		{
			name: "matrixed-task",
			state: PipelineRunState{{
				TaskRunName: "matrixed-task-run-0",
				PipelineTask: &v1beta1.PipelineTask{
					Name: "matrixed-task",
					TaskRef: &v1beta1.TaskRef{
						Name:       "task",
						Kind:       "Task",
						APIVersion: "v1beta1",
					},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"foo", "bar"},
					}},
					Matrix: &v1beta1.Matrix{
						Params: []v1beta1.Param{{
							Name:  "foobar",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
						}, {
							Name:  "quxbaz",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"qux", "baz"}},
						}}},
				},
				TaskRuns: []*v1beta1.TaskRun{{
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-0"},
				}, {
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-1"},
				}, {
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-2"},
				}, {
					TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "matrixed-task-run-3"},
				}},
			}},
			childRefs: []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "TaskRun",
				},
				Name:             "matrixed-task-run-0",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}, {
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "TaskRun",
				},
				Name:             "matrixed-task-run-1",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}, {
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "TaskRun",
				},
				Name:             "matrixed-task-run-2",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}, {
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "TaskRun",
				},
				Name:             "matrixed-task-run-3",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}},
		},
		{
			name: "unresolved-matrixed-custom-task",
			state: PipelineRunState{{
				PipelineTask: &v1beta1.PipelineTask{
					Name: "matrixed-task",
					TaskRef: &v1beta1.TaskRef{
						Kind:       "Example",
						APIVersion: "example.dev/v0",
					},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"foo", "bar"},
					}},
					Matrix: &v1beta1.Matrix{
						Params: []v1beta1.Param{{
							Name:  "foobar",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
						}, {
							Name:  "quxbaz",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"qux", "baz"}},
						}}},
				},
				CustomTask: true,
			}},
			childRefs: nil,
		},
		{
			name: "matrixed-custom-task",
			state: PipelineRunState{{
				PipelineTask: &v1beta1.PipelineTask{
					Name: "matrixed-task",
					TaskRef: &v1beta1.TaskRef{
						APIVersion: "example.dev/v0",
						Kind:       "Example",
					},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"foo", "bar"},
					}},
					Matrix: &v1beta1.Matrix{
						Params: []v1beta1.Param{{
							Name:  "foobar",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
						}, {
							Name:  "quxbaz",
							Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"qux", "baz"}},
						}}},
				},
				CustomTask: true,
				RunObjects: []v1beta1.RunObject{
					customRunWithName("matrixed-run-0"),
					customRunWithName("matrixed-run-1"),
					customRunWithName("matrixed-run-2"),
					customRunWithName("matrixed-run-3"),
				},
			}},
			childRefs: []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "CustomRun",
				},
				Name:             "matrixed-run-0",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}, {
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "CustomRun",
				},
				Name:             "matrixed-run-1",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}, {
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "CustomRun",
				},
				Name:             "matrixed-run-2",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}, {
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "CustomRun",
				},
				Name:             "matrixed-run-3",
				PipelineTaskName: "matrixed-task",
				WhenExpressions: []v1beta1.WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{"foo", "bar"},
				}},
			}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			childRefs := tc.state.GetChildReferences()
			if d := cmp.Diff(tc.childRefs, childRefs); d != "" {
				t.Errorf("Didn't get expected child references for %s: %s", tc.name, diff.PrintWantGot(d))
			}
		})
	}
}

func customRunWithName(name string) *v1beta1.CustomRun {
	return &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestGetRetriableTasks(t *testing.T) {
	failedRun := parse.MustParseRun(t, `
metadata:
  name: custom-task
  namespace: namespace
spec:
  retries: 1
  params:
  - name: param1
    value: value1
  ref:
    apiVersion: example.dev/v0
    kind: Example
status:
  conditions:
  - type: Succeeded
    status: "False"
    reason: Timedout
`)
	failedCustomRunPT := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "failedrun",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
		RunObjectName: "failedrun",
		RunObject:     makeRunFailed(runs[0]),
		CustomTask:    true,
	}
	failedTaskRunPTWithRetries := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "failedtask",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			Retries: 1,
		},
		TaskRunName: "failedtask",
		TaskRun:     makeFailed(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}
	failedRunPTWithRetries := ResolvedPipelineTask{
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "failedrunwithretries",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			Retries: 1,
		},
		RunObjectName: "failedrunwithretries",
		RunObject:     failedRun,
		CustomTask:    true,
	}
	for _, tc := range []struct {
		name       string
		state      PipelineRunState
		candidates sets.String
		want       []*ResolvedPipelineTask
	}{{
		name:       "Failed Custom Task with Retries",
		state:      PipelineRunState{&failedRunPTWithRetries},
		candidates: sets.String{}.Insert(failedCustomRunPT.RunObjectName).Insert(failedRunPTWithRetries.RunObjectName),
		want:       []*ResolvedPipelineTask{&failedRunPTWithRetries},
	}, {
		name:       "Failed Custom Task and TaskRuns with Retries",
		state:      PipelineRunState{&failedRunPTWithRetries},
		candidates: sets.String{}.Insert(failedRunPTWithRetries.RunObjectName).Insert(failedTaskRunPTWithRetries.TaskRunName),
		want:       []*ResolvedPipelineTask{&failedRunPTWithRetries},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			queue := tc.state.getRetriableRuns(tc.candidates)
			if d := cmp.Diff(tc.want, queue); d != "" {
				t.Errorf("Didn't get expected execution queue: %s", diff.PrintWantGot(d))
			}
		})
	}
}
