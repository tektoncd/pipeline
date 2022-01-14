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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var now = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)

type testClock struct{}

func (testClock) Now() time.Time                  { return now }
func (testClock) Since(t time.Time) time.Duration { return now.Sub(t) }

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

	var runRunningState = PipelineRunState{{
		PipelineTask: &pts[12],
		CustomTask:   true,
		RunName:      "pipelinerun-mytask13",
		Run:          makeRunStarted(runs[0]),
	}}

	var runSucceededState = PipelineRunState{{
		PipelineTask: &pts[12],
		CustomTask:   true,
		RunName:      "pipelinerun-mytask13",
		Run:          makeRunSucceeded(runs[0]),
	}}

	var runFailedState = PipelineRunState{{
		PipelineTask: &pts[12],
		CustomTask:   true,
		RunName:      "pipelinerun-mytask13",
		Run:          makeRunFailed(runs[0]),
	}}

	var taskFailedWithRetries = PipelineRunState{{
		PipelineTask: &pts[4], // 2 retries needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      makeFailed(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
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
		name:       "run-failed-with-retries",
		state:      taskFailedWithRetries,
		expected:   false,
		ptExpected: []bool{false},
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
				t.Fatalf("Unexpected error while buildig DAG for state %v: %v", tc.state, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
			}

			isDone := facts.checkTasksDone(d)
			if d := cmp.Diff(tc.expected, isDone); d != "" {
				t.Errorf("Didn't get expected checkTasksDone %s", diff.PrintWantGot(d))
			}
			for i, pt := range tc.state {
				isDone = pt.IsDone(&facts)
				if d := cmp.Diff(tc.ptExpected[i], isDone); d != "" {
					t.Errorf("Didn't get expected (ResolvedPipelineRunTask) IsDone %s", diff.PrintWantGot(d))
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

func TestGetNextTasks(t *testing.T) {
	tcs := []struct {
		name         string
		state        PipelineRunState
		candidates   sets.String
		expectedNext []*ResolvedPipelineRunTask
	}{{
		name:         "no-tasks-started-no-candidates",
		state:        noneStartedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "no-tasks-started-one-candidate",
		state:        noneStartedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineRunTask{noneStartedState[0]},
	}, {
		name:         "no-tasks-started-other-candidate",
		state:        noneStartedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{noneStartedState[1]},
	}, {
		name:         "no-tasks-started-both-candidates",
		state:        noneStartedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{noneStartedState[0], noneStartedState[1]},
	}, {
		name:         "one-task-started-no-candidates",
		state:        oneStartedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "one-task-started-one-candidate",
		state:        oneStartedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "one-task-started-other-candidate",
		state:        oneStartedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{oneStartedState[1]},
	}, {
		name:         "one-task-started-both-candidates",
		state:        oneStartedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{oneStartedState[1]},
	}, {
		name:         "one-task-finished-no-candidates",
		state:        oneFinishedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "one-task-finished-one-candidate",
		state:        oneFinishedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "one-task-finished-other-candidate",
		state:        oneFinishedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{oneFinishedState[1]},
	}, {
		name:         "one-task-finished-both-candidate",
		state:        oneFinishedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{oneFinishedState[1]},
	}, {
		name:         "one-task-failed-no-candidates",
		state:        oneFailedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "one-task-failed-one-candidate",
		state:        oneFailedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "one-task-failed-other-candidate",
		state:        oneFailedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{oneFailedState[1]},
	}, {
		name:         "one-task-failed-both-candidates",
		state:        oneFailedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{oneFailedState[1]},
	}, {
		name:         "all-finished-no-candidates",
		state:        allFinishedState,
		candidates:   sets.NewString(),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "all-finished-one-candidate",
		state:        allFinishedState,
		candidates:   sets.NewString("mytask1"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "all-finished-other-candidate",
		state:        allFinishedState,
		candidates:   sets.NewString("mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "all-finished-both-candidates",
		state:        allFinishedState,
		candidates:   sets.NewString("mytask1", "mytask2"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "one-cancelled-one-candidate",
		state:        taskCancelled,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "no-runs-started-both-candidates",
		state:        noRunStartedState,
		candidates:   sets.NewString("mytask13", "mytask14"),
		expectedNext: []*ResolvedPipelineRunTask{noRunStartedState[0], noRunStartedState[1]},
	}, {
		name:         "one-run-started-both-candidates",
		state:        oneRunStartedState,
		candidates:   sets.NewString("mytask13", "mytask14"),
		expectedNext: []*ResolvedPipelineRunTask{oneRunStartedState[1]},
	}, {
		name:         "one-run-failed-both-candidates",
		state:        oneRunFailedState,
		candidates:   sets.NewString("mytask13", "mytask14"),
		expectedNext: []*ResolvedPipelineRunTask{oneRunFailedState[1]},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.getNextTasks(tc.candidates)
			if d := cmp.Diff(next, tc.expectedNext); d != "" {
				t.Errorf("Didn't get expected next Tasks %s", diff.PrintWantGot(d))
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
		candidates   sets.String
		expectedNext []*ResolvedPipelineRunTask
	}{{
		name:         "tasks-cancelled-no-candidates",
		state:        taskCancelledByStatusState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "tasks-cancelled-bySpec-no-candidates",
		state:        taskCancelledBySpecState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "tasks-running-no-candidates",
		state:        taskRunningState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "tasks-succeeded-bySpec-no-candidates",
		state:        taskSucceededState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "tasks-retried-no-candidates",
		state:        taskRetriedState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineRunTask{},
	}, {
		name:         "tasks-retried-one-candidates",
		state:        taskExpectedState,
		candidates:   sets.NewString("mytask5"),
		expectedNext: []*ResolvedPipelineRunTask{taskExpectedState[0]},
	}}

	// iterate over *state* to get from candidate and check if TaskRun is there.
	// Cancelled TaskRun should have a TaskRun cancelled and with a retry but should not retry.

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.getNextTasks(tc.candidates)
			if d := cmp.Diff(next, tc.expectedNext); d != "" {
				t.Errorf("Didn't get expected next Tasks %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunState_SuccessfulOrSkippedDAGTasks(t *testing.T) {
	largePipelineState := buildPipelineStateWithLargeDepencyGraph(t)
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
		expectedNames: []string{pts[1].Name},
	}, {
		name:          "all-finished",
		state:         allFinishedState,
		expectedNames: []string{pts[0].Name, pts[1].Name},
	}, {
		name:          "conditional task not skipped as the condition execution was successful",
		state:         conditionCheckSuccessNoTaskStartedState,
		expectedNames: []string{},
	}, {
		name:          "conditional task not skipped as the condition has not started executing yet",
		state:         conditionCheckStartedState,
		expectedNames: []string{},
	}, {
		name:          "conditional task skipped as the condition execution resulted in failure",
		state:         conditionCheckFailedWithNoOtherTasksState,
		expectedNames: []string{pts[5].Name},
	}, {
		name: "conditional task skipped as the condition execution resulted in failure but the other pipeline task" +
			"not skipped since it finished execution successfully",
		state:         conditionCheckFailedWithOthersPassedState,
		expectedNames: []string{pts[5].Name, pts[0].Name},
	}, {
		name: "conditional task skipped as the condition execution resulted in failure but the other pipeline task" +
			"not skipped since it failed",
		state:         conditionCheckFailedWithOthersFailedState,
		expectedNames: []string{pts[5].Name},
	}, {
		name:          "large deps, not started",
		state:         largePipelineState,
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
		expectedNames: []string{pts[13].Name},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Unexpected error while buildig DAG for state %v: %v", tc.state, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				SpecStatus:      tc.specStatus,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
			}
			names := facts.successfulOrSkippedDAGTasks()
			if d := cmp.Diff(names, tc.expectedNames); d != "" {
				t.Errorf("Expected to get completed names %v but got something different %s", tc.expectedNames, diff.PrintWantGot(d))
			}
		})
	}
}

func buildPipelineStateWithLargeDepencyGraph(t *testing.T) PipelineRunState {
	t.Helper()
	var task = &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name: "step1",
			}}},
		},
	}
	var pipelineRunState PipelineRunState
	pipelineRunState = []*ResolvedPipelineRunTask{{
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
				Value: v1beta1.ArrayOrString{
					Type:      v1beta1.ParamTypeString,
					StringVal: fmt.Sprintf("$(tasks.t%d.results.%c)", dependFrom, alpha),
				},
			})
		}
		pipelineRunState = append(pipelineRunState, &ResolvedPipelineRunTask{
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

func TestPipelineRunState_GetFinalTasks(t *testing.T) {
	tcs := []struct {
		name               string
		desc               string
		state              PipelineRunState
		DAGTasks           []v1beta1.PipelineTask
		finalTasks         []v1beta1.PipelineTask
		expectedFinalTasks PipelineRunState
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
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "02 - DAG task not started, no final tasks",
		desc:               "DAG tasks (mytask1) not started yet - do not schedule final tasks (mytask2)",
		state:              noneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{},
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "03 - DAG task not finished, no final tasks",
		desc:               "DAG tasks (mytask1) started but not finished - do not schedule final tasks (mytask2)",
		state:              oneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{},
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "04 - DAG task done, return final tasks",
		desc:               "DAG tasks (mytask1) done - schedule final tasks (mytask2)",
		state:              oneFinishedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{oneFinishedState[1]},
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "05 - DAG task failed, return final tasks",
		desc:               "DAG task (mytask1) failed - schedule final tasks (mytask2)",
		state:              oneFailedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{oneFinishedState[1]},
	}, {
		// tasks: [ mytask6 with condition]
		// finally: [mytask2]
		name:               "06 - DAG task condition started, no final tasks",
		desc:               "DAG task (mytask6) condition started - do not schedule final tasks (mytask1)",
		state:              append(conditionCheckStartedState, noneStartedState[0]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5]},
		finalTasks:         []v1beta1.PipelineTask{pts[0]},
		expectedFinalTasks: PipelineRunState{},
	}, {
		// tasks: [ mytask6 with condition]
		// finally: [mytask2]
		name:               "07 - DAG task condition done, no final tasks",
		desc:               "DAG task (mytask6) condition finished, mytask6 not started - do not schedule final tasks (mytask2)",
		state:              append(conditionCheckSuccessNoTaskStartedState, noneStartedState[0]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5]},
		finalTasks:         []v1beta1.PipelineTask{pts[0]},
		expectedFinalTasks: PipelineRunState{},
	}, {
		// tasks: [ mytask6 with condition]
		// finally: [mytask2]
		name:               "08 - DAG task skipped, return final tasks",
		desc:               "DAG task (mytask6) condition failed - schedule final tasks (mytask2) ",
		state:              append(conditionCheckFailedWithNoOtherTasksState, noneStartedState[0]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5]},
		finalTasks:         []v1beta1.PipelineTask{pts[0]},
		expectedFinalTasks: PipelineRunState{noneStartedState[0]},
	}, {
		// tasks: [ mytask1, mytask6 with condition]
		// finally: [mytask2]
		name:               "09 - DAG task succeeded/skipped, return final tasks ",
		desc:               "DAG task (mytask1) finished, mytask6 condition failed - schedule final tasks (mytask2)",
		state:              append(conditionCheckFailedWithOthersPassedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5], pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{noneStartedState[1]},
	}, {
		// tasks: [ mytask1, mytask6 with condition]
		// finally: [mytask2]
		name:               "10 - DAG task failed/skipped, return final tasks",
		desc:               "DAG task (mytask1) failed, mytask6 condition failed - schedule final tasks (mytask2)",
		state:              append(conditionCheckFailedWithOthersFailedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5], pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{noneStartedState[1]},
	}, {
		// tasks: [ mytask6 with condition, mytask7 runAfter mytask6]
		// finally: [mytask2]
		name:               "11 - DAG task skipped, return final tasks",
		desc:               "DAG task (mytask6) condition failed, mytask6 and mytask7 skipped - schedule final tasks (mytask2)",
		state:              append(taskWithParentSkippedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5], pts[6]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{noneStartedState[1]},
	}, {
		// tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6]
		// finally: [mytask2]
		name:               "12 - DAG task succeeded/skipped, return final tasks",
		desc:               "DAG task (mytask1) finished - DAG task (mytask6) condition failed, mytask6 and mytask8 skipped - schedule final tasks (mytask2)",
		state:              append(taskWithMultipleParentsSkippedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{noneStartedState[1]},
	}, {
		// tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6, mytask9 runAfter mytask1 and mytask6]
		// finally: [mytask2]
		name: "13 - DAG task succeeded/skipped - return final tasks",
		desc: "DAG task (mytask1) finished - DAG task (mytask6) condition failed, mytask6, mytask8, and mytask9 skipped" +
			"- schedule final tasks (mytask2)",
		state:              append(taskWithGrandParentSkippedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7], pts[8]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{noneStartedState[1]},
	}, {
		//tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6, mytask9 runAfter mytask1 and mytask6]
		//finally: [mytask2]
		name: "14 - DAG task succeeded, skipped - return final tasks",
		desc: "DAG task (mytask1) finished - DAG task (mytask6) failed - mytask8 and mytask9 skipped" +
			"- schedule final tasks (mytask2)",
		state:              append(taskWithGrandParentsOneFailedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7], pts[8]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{noneStartedState[1]},
	}, {
		//tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6, mytask9 runAfter mytask1 and mytask6]
		//finally: [mytask2]
		name:               "15 - DAG task succeeded/started - no final tasks",
		desc:               "DAG task (mytask1) finished - DAG task (mytask6) started - do no schedule final tasks",
		state:              append(taskWithGrandParentsOneNotRunState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7], pts[8]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: PipelineRunState{},
	}}
	for _, tc := range tcs {
		dagGraph, err := dag.Build(v1beta1.PipelineTaskList(tc.DAGTasks), v1beta1.PipelineTaskList(tc.DAGTasks).Deps())
		if err != nil {
			t.Fatalf("Unexpected error while buildig DAG for pipelineTasks %v: %v", tc.DAGTasks, err)
		}
		finalGraph, err := dag.Build(v1beta1.PipelineTaskList(tc.finalTasks), map[string][]string{})
		if err != nil {
			t.Fatalf("Unexpected error while buildig DAG for final pipelineTasks %v: %v", tc.finalTasks, err)
		}
		t.Run(tc.name, func(t *testing.T) {
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      dagGraph,
				FinalTasksGraph: finalGraph,
			}
			next := facts.GetFinalTasks()
			if d := cmp.Diff(tc.expectedFinalTasks, next); d != "" {
				t.Errorf("Didn't get expected final Tasks for %s (%s): %s", tc.name, tc.desc, diff.PrintWantGot(d))
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

	var cancelledTask = PipelineRunState{{
		PipelineTask: &pts[3], // 1 retry needed
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{Conditions: []apis.Condition{{
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
		PipelineTask: &pts[12],
		CustomTask:   true,
		RunName:      "pipelinerun-mytask13",
		Run: &v1alpha1.Run{
			Status: v1alpha1.RunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
					Reason: v1alpha1.RunReasonCancelled,
				}}},
			},
		},
	}}

	// 6 Tasks, 4 that run in parallel in the beginning
	// Of the 4, 1 passed, 1 cancelled, 2 failed
	// 1 runAfter the passed one, currently running
	// 1 runAfter the failed one, which is marked as incomplete
	var taskMultipleFailuresSkipRunning = PipelineRunState{{
		TaskRunName:             "task0taskrun",
		PipelineTask:            &pts[5],
		TaskRun:                 makeSucceeded(trs[0]),
		ResolvedConditionChecks: successTaskConditionCheckState,
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
		TaskRunName:             "task0taskrun",
		PipelineTask:            &pts[5],
		TaskRun:                 makeSucceeded(trs[0]),
		ResolvedConditionChecks: successTaskConditionCheckState,
	}, {
		TaskRunName:  "notRunningTaskRun", // runAfter pts[5], not started yet
		PipelineTask: &pts[6],
		TaskRun:      nil,
	}, {
		TaskRunName:  "failedTaskRun", // this failed
		PipelineTask: &pts[0],
		TaskRun:      makeFailed(trs[0]),
	}}

	tcs := []struct {
		name               string
		state              PipelineRunState
		finallyState       PipelineRunState
		specStatus         v1beta1.PipelineRunSpecStatus
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
		name:               "condition-success-no-task started",
		state:              conditionCheckSuccessNoTaskStartedState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedIncomplete: 1,
	}, {
		name:               "condition-check-in-progress",
		state:              conditionCheckStartedState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedIncomplete: 1,
	}, {
		name:               "condition-failed-no-other-tasks", // 1 task pipeline with a condition that fails
		state:              conditionCheckFailedWithNoOtherTasksState,
		expectedStatus:     corev1.ConditionTrue,
		expectedReason:     v1beta1.PipelineRunReasonCompleted.String(),
		expectedSkipped:    1,
		expectedIncomplete: 1,
	}, {
		name:              "condition-failed-another-task-succeeded", // 1 task skipped due to condition, but others pass
		state:             conditionCheckFailedWithOthersPassedState,
		expectedStatus:    corev1.ConditionTrue,
		expectedReason:    v1beta1.PipelineRunReasonCompleted.String(),
		expectedSucceeded: 1,
		expectedSkipped:   1,
	}, {
		name:            "condition-failed-another-task-failed", // 1 task skipped due to condition, but others failed
		state:           conditionCheckFailedWithOthersFailedState,
		expectedStatus:  corev1.ConditionFalse,
		expectedReason:  v1beta1.PipelineRunReasonFailed.String(),
		expectedFailed:  1,
		expectedSkipped: 1,
	}, {
		name:            "task skipped due to condition failure in parent",
		state:           taskWithParentSkippedState,
		expectedStatus:  corev1.ConditionTrue,
		expectedReason:  v1beta1.PipelineRunReasonCompleted.String(),
		expectedSkipped: 2,
	}, {
		name:              "task with multiple parent tasks -> one of which is skipped",
		state:             taskWithMultipleParentsSkippedState,
		expectedStatus:    corev1.ConditionTrue,
		expectedReason:    v1beta1.PipelineRunReasonCompleted.String(),
		expectedSkipped:   2,
		expectedSucceeded: 1,
	}, {
		name:              "task with grand parent task skipped",
		state:             taskWithGrandParentSkippedState,
		expectedStatus:    corev1.ConditionTrue,
		expectedReason:    v1beta1.PipelineRunReasonCompleted.String(),
		expectedSkipped:   3,
		expectedSucceeded: 1,
	}, {
		name:              "task with grand parents; one parent failed",
		state:             taskWithGrandParentsOneFailedState,
		expectedStatus:    corev1.ConditionFalse,
		expectedReason:    v1beta1.PipelineRunReasonFailed.String(),
		expectedSucceeded: 1,
		expectedSkipped:   2,
		expectedFailed:    1,
	}, {
		name:               "task with grand parents; one not run yet",
		state:              taskWithGrandParentsOneNotRunState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedSucceeded:  1,
		expectedIncomplete: 3,
	}, {
		name:              "task that was cancelled",
		state:             taskCancelledFailed,
		expectedReason:    v1beta1.PipelineRunReasonCancelled.String(),
		expectedStatus:    corev1.ConditionFalse,
		expectedCancelled: 1,
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
		name:               "task with grand parents; one not run yet",
		state:              taskWithGrandParentsOneNotRunState,
		expectedStatus:     corev1.ConditionUnknown,
		expectedReason:     v1beta1.PipelineRunReasonRunning.String(),
		expectedSucceeded:  1,
		expectedIncomplete: 3,
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
				t.Fatalf("Unexpected error while buildig DAG for state %v: %v", tc.state, err)
			}
			dfinally, err := dagFromState(tc.finallyState)
			if err != nil {
				t.Fatalf("Unexpected error while buildig DAG for finally state %v: %v", tc.finallyState, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				SpecStatus:      tc.specStatus,
				TasksGraph:      d,
				FinalTasksGraph: dfinally,
			}
			c := facts.GetPipelineConditionStatus(context.Background(), pr, zap.NewNop().Sugar(), testClock{})
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
				t.Fatalf("Unexpected error while buildig graph for DAG tasks %v: %v", tc.dagTasks, err)
			}
			df, err := dag.Build(v1beta1.PipelineTaskList(tc.finalTasks), map[string][]string{})
			if err != nil {
				t.Fatalf("Unexpected error while buildig graph for final tasks %v: %v", tc.finalTasks, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: df,
			}
			c := facts.GetPipelineConditionStatus(context.Background(), pr, zap.NewNop().Sugar(), testClock{})
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
func TestGetPipelineConditionStatus_PipelineTimeouts(t *testing.T) {
	d, err := dagFromState(oneFinishedState)
	if err != nil {
		t.Fatalf("Unexpected error while buildig DAG for state %v: %v", oneFinishedState, err)
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
	}
	c := facts.GetPipelineConditionStatus(context.Background(), pr, zap.NewNop().Sugar(), testClock{})
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
			Run: &v1alpha1.Run{
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
			Run: &v1alpha1.Run{
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
				t.Fatalf("Unexpected error while buildig graph for DAG tasks %v: %v", tc.dagTasks, err)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
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
			Name: pts[14].Name,
		}},
	}, {
		name: "when-expressions-skip-finally",
		state: PipelineRunState{{
			PipelineTask: &pts[10],
		}},
		finallyTasks: []v1beta1.PipelineTask{pts[10]},
		expectedSkippedTasks: []v1beta1.SkippedTask{{
			Name: pts[10].Name,
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
			}
			if tc.expected != facts.IsRunning() {
				t.Errorf("IsRunning expected to be %v", tc.expected)
			}
		})
	}
}
