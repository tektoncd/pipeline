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

package resources

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tbv1alpha1 "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var pts = []v1beta1.PipelineTask{{
	Name:    "mytask1",
	TaskRef: &v1beta1.TaskRef{Name: "task"},
}, {
	Name:    "mytask2",
	TaskRef: &v1beta1.TaskRef{Name: "task"},
}, {
	Name:    "mytask3",
	TaskRef: &v1beta1.TaskRef{Name: "clustertask"},
}, {
	Name:    "mytask4",
	TaskRef: &v1beta1.TaskRef{Name: "task"},
	Retries: 1,
}, {
	Name:    "mytask5",
	TaskRef: &v1beta1.TaskRef{Name: "cancelledTask"},
	Retries: 2,
}, {
	Name:    "mytask6",
	TaskRef: &v1beta1.TaskRef{Name: "taskWithConditions"},
	Conditions: []v1beta1.PipelineTaskCondition{{
		ConditionRef: "always-true",
	}},
}, {
	Name:     "mytask7",
	TaskRef:  &v1beta1.TaskRef{Name: "taskWithOneParent"},
	RunAfter: []string{"mytask6"},
}, {
	Name:     "mytask8",
	TaskRef:  &v1beta1.TaskRef{Name: "taskWithTwoParents"},
	RunAfter: []string{"mytask1", "mytask6"},
}, {
	Name:     "mytask9",
	TaskRef:  &v1beta1.TaskRef{Name: "taskHasParentWithRunAfter"},
	RunAfter: []string{"mytask8"},
}}

var p = &v1beta1.Pipeline{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipeline",
	},
	Spec: v1beta1.PipelineSpec{
		Tasks: pts,
	},
}

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

var clustertask = &v1beta1.ClusterTask{
	ObjectMeta: metav1.ObjectMeta{
		Name: "clustertask",
	},
	Spec: v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{Container: corev1.Container{
			Name: "step1",
		}}},
	},
}

var trs = []v1beta1.TaskRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask1",
	},
	Spec: v1beta1.TaskRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask2",
	},
	Spec: v1beta1.TaskRunSpec{},
}}

var condition = v1alpha1.Condition{
	ObjectMeta: metav1.ObjectMeta{
		Name: "always-true",
	},
	Spec: v1alpha1.ConditionSpec{
		Check: v1beta1.Step{},
	},
}

var conditionChecks = []v1beta1.TaskRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "always-true",
	},
	Spec: v1beta1.TaskRunSpec{},
}}

func makeStarted(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newTr
}

func makeSucceeded(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionTrue
	return newTr
}

func makeFailed(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionFalse
	return newTr
}

func withCancelled(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Status.Conditions[0].Reason = v1beta1.TaskRunSpecStatusCancelled
	return tr
}

func withCancelledBySpec(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Spec.Status = v1beta1.TaskRunSpecStatusCancelled
	return tr
}

func makeRetried(tr v1beta1.TaskRun) (newTr *v1beta1.TaskRun) {
	newTr = newTaskRun(tr)
	newTr.Status.RetriesStatus = []v1beta1.TaskRunStatus{{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return
}
func withRetries(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Status.RetriesStatus = []v1beta1.TaskRunStatus{{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return tr
}

func newTaskRun(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			Name:      tr.Name,
		},
		Spec: tr.Spec,
		Status: v1beta1.TaskRunStatus{
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

var successTaskConditionCheckState = TaskConditionCheckState{{
	ConditionCheckName: "myconditionCheck",
	Condition:          &condition,
	ConditionCheck:     v1beta1.NewConditionCheck(makeSucceeded(conditionChecks[0])),
}}

var failedTaskConditionCheckState = TaskConditionCheckState{{
	ConditionCheckName: "myconditionCheck",
	Condition:          &condition,
	ConditionCheck:     v1beta1.NewConditionCheck(makeFailed(conditionChecks[0])),
}}

var conditionCheckSuccessNoTaskStartedState = PipelineRunState{{
	PipelineTask: &pts[5],
	TaskRunName:  "pipelinerun-conditionaltask",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
	ResolvedConditionChecks: successTaskConditionCheckState,
}}

var conditionCheckStartedState = PipelineRunState{{
	PipelineTask: &pts[5],
	TaskRunName:  "pipelinerun-conditionaltask",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
	ResolvedConditionChecks: TaskConditionCheckState{{
		ConditionCheckName: "myconditionCheck",
		Condition:          &condition,
		ConditionCheck:     v1beta1.NewConditionCheck(makeStarted(conditionChecks[0])),
	}},
}}

var conditionCheckFailedWithNoOtherTasksState = PipelineRunState{{
	PipelineTask: &pts[5],
	TaskRunName:  "pipelinerun-conditionaltask",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
	ResolvedConditionChecks: failedTaskConditionCheckState,
}}

var conditionCheckFailedWithOthersPassedState = PipelineRunState{{
	PipelineTask: &pts[5],
	TaskRunName:  "pipelinerun-conditionaltask",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
	ResolvedConditionChecks: failedTaskConditionCheckState,
},
	{
		PipelineTask: &pts[0],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      makeSucceeded(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	},
}

var conditionCheckFailedWithOthersFailedState = PipelineRunState{{
	PipelineTask: &pts[5],
	TaskRunName:  "pipelinerun-conditionaltask",
	TaskRun:      nil,
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
	ResolvedConditionChecks: failedTaskConditionCheckState,
},
	{
		PipelineTask: &pts[0],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      makeFailed(trs[0]),
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	},
}

// skipped == condition check failure
// task -> parent.cc skipped
var taskWithParentSkippedState = PipelineRunState{{
	TaskRunName:             "taskrunName",
	PipelineTask:            &pts[5],
	ResolvedConditionChecks: failedTaskConditionCheckState,
}, {
	TaskRunName:  "childtaskrun",
	PipelineTask: &pts[6],
}}

// task -> 2 parents -> one of which is skipped
var taskWithMultipleParentsSkippedState = PipelineRunState{{
	TaskRunName:  "task0taskrun",
	PipelineTask: &pts[0],
	TaskRun:      makeSucceeded(trs[0]), // This parent is successful
}, {
	TaskRunName:             "taskrunName",
	PipelineTask:            &pts[5],
	ResolvedConditionChecks: failedTaskConditionCheckState, // This one was skipped
}, {
	TaskRunName:  "childtaskrun",
	PipelineTask: &pts[7], // This should also be skipped.
}}

// task -> parent -> grandparent is a skipped task
var taskWithGrandParentSkippedState = PipelineRunState{{
	TaskRunName:  "task0taskrun",
	PipelineTask: &pts[0],
	TaskRun:      makeSucceeded(trs[0]), // Passed
}, {
	TaskRunName:             "skippedTaskRun",
	PipelineTask:            &pts[5],
	ResolvedConditionChecks: failedTaskConditionCheckState, // Skipped
}, {
	TaskRunName:  "anothertaskrun",
	PipelineTask: &pts[7], // Should be skipped
}, {
	TaskRunName:  "taskrun",
	PipelineTask: &pts[8], // Should also be skipped
}}

var taskWithGrandParentsOneFailedState = PipelineRunState{{
	TaskRunName:  "task0taskrun",
	PipelineTask: &pts[0],
	TaskRun:      makeSucceeded(trs[0]), // Passed
}, {
	TaskRunName:             "skippedTaskRun",
	PipelineTask:            &pts[5],
	TaskRun:                 makeFailed(trs[0]), // Failed; so pipeline should fail
	ResolvedConditionChecks: successTaskConditionCheckState,
}, {
	TaskRunName:  "anothertaskrun",
	PipelineTask: &pts[7],
}, {
	TaskRunName:  "taskrun",
	PipelineTask: &pts[8],
}}

var taskWithGrandParentsOneNotRunState = PipelineRunState{{
	TaskRunName:  "task0taskrun",
	PipelineTask: &pts[0],
	TaskRun:      makeSucceeded(trs[0]), // Passed
}, {
	TaskRunName:  "skippedTaskRun",
	PipelineTask: &pts[5],
	// No TaskRun so task has not been run, and pipeline should be running
	ResolvedConditionChecks: successTaskConditionCheckState,
}, {
	TaskRunName:  "anothertaskrun",
	PipelineTask: &pts[7],
}, {
	TaskRunName:  "taskrun",
	PipelineTask: &pts[8],
}}

var taskCancelled = PipelineRunState{{
	PipelineTask: &pts[4],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      withCancelled(makeRetried(trs[0])),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}

var taskWithOptionalResourcesDeprecated = &v1beta1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name: "task",
	},
	Spec: v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{Container: corev1.Container{
			Name: "step1",
		}}},
		Resources: &v1beta1.TaskResources{
			Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "optional-input",
				Type:     "git",
				Optional: true,
			}}, {ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "required-input",
				Type:     "git",
				Optional: false,
			}}},
			Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "optional-output",
				Type:     "git",
				Optional: true,
			}}, {ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "required-output",
				Type:     "git",
				Optional: false,
			}}},
		},
	},
}
var taskWithOptionalResources = &v1beta1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name: "task",
	},
	Spec: v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{Container: corev1.Container{
			Name: "step1",
		}}},
		Resources: &v1beta1.TaskResources{
			Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "optional-input",
				Type:     "git",
				Optional: true,
			}}, {ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "required-input",
				Type:     "git",
				Optional: false,
			}}},
			Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "optional-output",
				Type:     "git",
				Optional: true,
			}}, {ResourceDeclaration: v1beta1.ResourceDeclaration{
				Name:     "required-output",
				Type:     "git",
				Optional: false,
			}}},
		},
	},
}

func DagFromState(state PipelineRunState) (*dag.Graph, error) {
	pts := []v1beta1.PipelineTask{}
	for _, rprt := range state {
		pts = append(pts, *rprt.PipelineTask)
	}
	return dag.Build(v1beta1.PipelineTaskList(pts))
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
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.GetNextTasks(tc.candidates)
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
			next := tc.state.GetNextTasks(tc.candidates)
			if d := cmp.Diff(next, tc.expectedNext); d != "" {
				t.Errorf("Didn't get expected next Tasks %s", diff.PrintWantGot(d))
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
		name:       "no-pipelineTask",
		state:      noPipelineTaskState,
		expected:   false,
		ptExpected: []bool{false},
	}, {
		name:       "No-taskrun",
		state:      noTaskRunState,
		expected:   false,
		ptExpected: []bool{false},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			isDone := tc.state.IsDone()
			if d := cmp.Diff(isDone, tc.expected); d != "" {
				t.Errorf("Didn't get expected IsDone %s", diff.PrintWantGot(d))
			}
			for i, pt := range tc.state {
				isDone = pt.IsDone()
				if d := cmp.Diff(isDone, tc.ptExpected[i]); d != "" {
					t.Errorf("Didn't get expected (ResolvedPipelineRunTask) IsDone %s", diff.PrintWantGot(d))
				}

			}
		})
	}
}

func TestIsSkipped(t *testing.T) {

	tcs := []struct {
		name     string
		taskName string
		state    PipelineRunState
		expected bool
	}{{
		name:     "tasks-condition-passed",
		taskName: "mytask1",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRunName:  "pipelinerun-conditionaltask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
			ResolvedConditionChecks: successTaskConditionCheckState,
		}},
		expected: false,
	}, {
		name:     "tasks-condition-failed",
		taskName: "mytask1",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRunName:  "pipelinerun-conditionaltask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
			ResolvedConditionChecks: failedTaskConditionCheckState,
		}},
		expected: true,
	}, {
		name:     "tasks-multiple-conditions-passed-failed",
		taskName: "mytask1",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRunName:  "pipelinerun-conditionaltask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
			ResolvedConditionChecks: TaskConditionCheckState{{
				ConditionCheckName: "myconditionCheck",
				Condition:          &condition,
				ConditionCheck:     v1beta1.NewConditionCheck(makeFailed(conditionChecks[0])),
			}, {
				ConditionCheckName: "myconditionCheck",
				Condition:          &condition,
				ConditionCheck:     v1beta1.NewConditionCheck(makeSucceeded(conditionChecks[0])),
			}},
		}},
		expected: true,
	}, {
		name:     "tasks-condition-running",
		taskName: "mytask6",
		state:    conditionCheckStartedState,
		expected: false,
	}, {
		name:     "tasks-parent-condition-passed",
		taskName: "mytask7",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-conditionaltask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
			ResolvedConditionChecks: successTaskConditionCheckState,
		}, {
			PipelineTask: &pts[6],
		}},
		expected: false,
	}, {
		name:     "tasks-parent-condition-failed",
		taskName: "mytask7",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-conditionaltask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
			ResolvedConditionChecks: failedTaskConditionCheckState,
		}, {
			PipelineTask: &pts[6],
		}},
		expected: true,
	}, {
		name:     "tasks-parent-condition-running",
		taskName: "mytask7",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-conditionaltask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
			ResolvedConditionChecks: TaskConditionCheckState{{
				ConditionCheckName: "myconditionCheck",
				Condition:          &condition,
				ConditionCheck:     v1beta1.NewConditionCheck(makeStarted(conditionChecks[0])),
			}},
		}, {
			PipelineTask: &pts[6],
		}},
		expected: false,
	}, {
		name:     "tasks-failed",
		taskName: "mytask1",
		state:    oneFailedState,
		expected: false,
	}, {
		name:     "tasks-passed",
		taskName: "mytask1",
		state:    oneFinishedState,
		expected: false,
	}, {
		name:     "tasks-cancelled",
		taskName: "mytask5",
		state:    taskCancelled,
		expected: false,
	}, {
		name:     "tasks-parent-failed",
		taskName: "mytask7",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		expected: true,
	}, {
		name:     "tasks-parent-cancelled",
		taskName: "mytask7",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      withCancelled(makeFailed(trs[0])),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		expected: true,
	}, {
		name:     "tasks-grandparent-failed",
		taskName: "mytask10",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask10",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask7"},
			}, // mytask10 runAfter mytask7 runAfter mytask6
			TaskRunName: "pipelinerun-mytask3",
			TaskRun:     nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		expected: true,
	}, {
		name:     "tasks-parents-failed-passed",
		taskName: "mytask8",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeSucceeded(trs[0]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[0],
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[7], // mytask8 runAfter mytask1, mytask6
			TaskRunName:  "pipelinerun-mytask3",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		expected: true,
	}, {
		name:     "task-failed-pipeline-stopping",
		taskName: "mytask7",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      makeStarted(trs[1]),
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask3",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		expected: true,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := DagFromState(tc.state)
			if err != nil {
				t.Fatalf("Could not get a dag from the TC state %#v: %v", tc.state, err)
			}
			stateMap := tc.state.ToMap()
			rprt := stateMap[tc.taskName]
			if rprt == nil {
				t.Fatalf("Could not get task %s from the state: %v", tc.taskName, tc.state)
			}
			isSkipped := rprt.IsSkipped(tc.state, dag)
			if d := cmp.Diff(isSkipped, tc.expected); d != "" {
				t.Errorf("Didn't get expected isSkipped %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineRunState_SuccessfulOrSkippedDAGTasks(t *testing.T) {
	tcs := []struct {
		name          string
		state         PipelineRunState
		expectedNames []string
	}{{
		name:          "no-tasks-started",
		state:         noneStartedState,
		expectedNames: []string{},
	}, {
		name:          "one-task-started",
		state:         oneStartedState,
		expectedNames: []string{},
	}, {
		name:          "one-task-finished",
		state:         oneFinishedState,
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
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := DagFromState(tc.state)
			if err != nil {
				t.Fatalf("Unexpected error while buildig DAG for state %v: %v", tc.state, err)
			}
			names := tc.state.SuccessfulOrSkippedDAGTasks(dag)
			if d := cmp.Diff(names, tc.expectedNames); d != "" {
				t.Errorf("Expected to get completed names %v but got something different %s", tc.expectedNames, diff.PrintWantGot(d))
			}
		})
	}
}

func getExpectedMessage(status corev1.ConditionStatus, successful, incomplete, skipped, failed, cancelled int) string {
	if status == corev1.ConditionFalse || status == corev1.ConditionTrue {
		return fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Skipped: %d",
			successful+failed+cancelled, failed, cancelled, skipped)
	}
	return fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Incomplete: %d, Skipped: %d",
		successful+failed+cancelled, failed, cancelled, incomplete, skipped)
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
		state              []*ResolvedPipelineRunTask
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
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pr := tb.PipelineRun("somepipelinerun")
			d, err := DagFromState(tc.state)
			if err != nil {
				t.Fatalf("Unexpected error while buildig DAG for state %v: %v", tc.state, err)
			}
			c := GetPipelineConditionStatus(pr, tc.state, zap.NewNop().Sugar(), d, &dag.Graph{})
			wantCondition := &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: tc.expectedStatus,
				Reason: tc.expectedReason,
				Message: getExpectedMessage(tc.expectedStatus, tc.expectedSucceeded,
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
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pr := tb.PipelineRun("pipelinerun-final-tasks")
			d, err := dag.Build(v1beta1.PipelineTaskList(tc.dagTasks))
			if err != nil {
				t.Fatalf("Unexpected error while buildig graph for DAG tasks %v: %v", tc.dagTasks, err)
			}
			df, err := dag.Build(v1beta1.PipelineTaskList(tc.finalTasks))
			if err != nil {
				t.Fatalf("Unexpected error while buildig graph for final tasks %v: %v", tc.finalTasks, err)
			}
			c := GetPipelineConditionStatus(pr, tc.state, zap.NewNop().Sugar(), d, df)
			wantCondition := &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: tc.expectedStatus,
				Reason: tc.expectedReason,
				Message: getExpectedMessage(tc.expectedStatus, tc.expectedSucceeded,
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
	d, err := DagFromState(oneFinishedState)
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
				StartTime: &metav1.Time{Time: time.Now().Add(-2 * time.Minute)},
			},
		},
	}
	c := GetPipelineConditionStatus(pr, oneFinishedState, zap.NewNop().Sugar(), d, &dag.Graph{})
	if c.Status != corev1.ConditionFalse && c.Reason != v1beta1.PipelineRunReasonTimedOut.String() {
		t.Fatalf("Expected to get status %s but got %s for state %v", corev1.ConditionFalse, c.Status, oneFinishedState)
	}
}

func TestGetResourcesFromBindings(t *testing.T) {
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
		tb.PipelineRunResourceBinding("image-resource", tb.PipelineResourceBindingResourceSpec(
			&resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeImage,
				Params: []v1beta1.ResourceParam{{
					Name:  "url",
					Value: "gcr.io/sven",
				}},
			}),
		),
	))
	r := tb.PipelineResource("sweet-resource")
	getResource := func(name string) (*resourcev1alpha1.PipelineResource, error) {
		if name != "sweet-resource" {
			return nil, fmt.Errorf("request for unexpected resource %s", name)
		}
		return r, nil
	}
	m, err := GetResourcesFromBindings(pr, getResource)
	if err != nil {
		t.Fatalf("didn't expect error getting resources from bindings but got: %v", err)
	}

	r1, ok := m["git-resource"]
	if !ok {
		t.Errorf("Missing expected resource git-resource: %v", m)
	} else if d := cmp.Diff(r, r1); d != "" {
		t.Errorf("Expected resources didn't match actual %s", diff.PrintWantGot(d))
	}

	r2, ok := m["image-resource"]
	if !ok {
		t.Errorf("Missing expected resource image-resource: %v", m)
	} else if r2.Spec.Type != resourcev1alpha1.PipelineResourceTypeImage ||
		len(r2.Spec.Params) != 1 ||
		r2.Spec.Params[0].Name != "url" ||
		r2.Spec.Params[0].Value != "gcr.io/sven" {
		t.Errorf("Did not get expected image resource, got %v", r2.Spec)
	}
}

func TestGetResourcesFromBindings_Missing(t *testing.T) {
	//p := tb.Pipeline("pipelines", "namespace", tb.PipelineSpec(
	//	tb.PipelineDeclaredResource("git-resource", "git"),
	//	tb.PipelineDeclaredResource("image-resource", "image"),
	//))
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
	))
	getResource := func(name string) (*resourcev1alpha1.PipelineResource, error) {
		return nil, fmt.Errorf("request for unexpected resource %s", name)
	}
	_, err := GetResourcesFromBindings(pr, getResource)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was missing but got no error")
	}
}

func TestGetResourcesFromBindings_ErrorGettingResource(t *testing.T) {
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
	))
	getResource := func(name string) (*resourcev1alpha1.PipelineResource, error) {
		return nil, fmt.Errorf("iT HAS ALL GONE WRONG")
	}
	_, err := GetResourcesFromBindings(pr, getResource)
	if err == nil {
		t.Fatalf("Expected error indicating resource couldnt be retrieved but got no error")
	}
}

func TestResolvePipelineRun(t *testing.T) {
	names.TestingSeed()

	p := tb.Pipeline("pipelines", tb.PipelineSpec(
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
		tb.PipelineTask("mytask4", "",
			tb.PipelineTaskSpec(&v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Name: "step1",
				}}},
			})),
	))

	r := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "someresource",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeGit,
		},
	}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{"git-resource": r}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	// The Task "task" doesn't actually take any inputs or outputs, but validating
	// that is not done as part of Run resolution
	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, nil }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return nil, nil }

	pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, p.Spec.Tasks, providedResources)
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
			Inputs: map[string]*resourcev1alpha1.PipelineResource{
				"input1": r,
			},
			Outputs: map[string]*resourcev1alpha1.PipelineResource{},
		},
	}, {
		PipelineTask: &p.Spec.Tasks[1],
		TaskRunName:  "pipelinerun-mytask2-mz4c7",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs:   map[string]*resourcev1alpha1.PipelineResource{},
			Outputs: map[string]*resourcev1alpha1.PipelineResource{
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
			Inputs:   map[string]*resourcev1alpha1.PipelineResource{},
			Outputs: map[string]*resourcev1alpha1.PipelineResource{
				"output1": r,
			},
		},
	}, {
		PipelineTask: &p.Spec.Tasks[3],
		TaskRunName:  "pipelinerun-mytask4-78c5n",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: "",
			TaskSpec: &task.Spec,
			Inputs:   map[string]*resourcev1alpha1.PipelineResource{},
			Outputs:  map[string]*resourcev1alpha1.PipelineResource{},
		},
	}}

	if d := cmp.Diff(expectedState, pipelineState, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Errorf("Expected to get current pipeline state %v, but actual differed %s", expectedState, diff.PrintWantGot(d))
	}
}

func TestResolvePipelineRun_PipelineTaskHasNoResources(t *testing.T) {
	pts := []v1beta1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask2",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask3",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return clustertask, nil }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return nil, nil }
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, pts, providedResources)
	if err != nil {
		t.Fatalf("Did not expect error when resolving PipelineRun without Resources: %v", err)
	}
	if len(pipelineState) != 3 {
		t.Fatalf("Expected only 2 resolved PipelineTasks but got %d", len(pipelineState))
	}
	expectedTaskResources := &resources.ResolvedTaskResources{
		TaskName: task.Name,
		TaskSpec: &task.Spec,
		Inputs:   map[string]*resourcev1alpha1.PipelineResource{},
		Outputs:  map[string]*resourcev1alpha1.PipelineResource{},
	}
	if d := cmp.Diff(pipelineState[0].ResolvedTaskResources, expectedTaskResources, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but but actual differed %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(pipelineState[1].ResolvedTaskResources, expectedTaskResources, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but but actual differed %s", diff.PrintWantGot(d))
	}
}

func TestResolvePipelineRun_TaskDoesntExist(t *testing.T) {
	pts := []v1beta1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	// Return an error when the Task is retrieved, as if it didn't exist
	getTask := func(name string) (v1beta1.TaskInterface, error) {
		return nil, kerrors.NewNotFound(v1beta1.Resource("task"), name)
	}
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) {
		return nil, kerrors.NewNotFound(v1beta1.Resource("clustertask"), name)
	}

	getTaskRun := func(name string) (*v1beta1.TaskRun, error) {
		return nil, kerrors.NewNotFound(v1beta1.Resource("taskrun"), name)
	}
	getCondition := func(name string) (*v1alpha1.Condition, error) {
		return nil, nil
	}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	_, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, pts, providedResources)
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
		p    *v1beta1.Pipeline
	}{{
		name: "input doesnt exist",
		p: tb.Pipeline("pipelines", tb.PipelineSpec(
			tb.PipelineTask("mytask1", "task",
				tb.PipelineTaskInputResource("input1", "git-resource"),
			),
		)),
	}, {
		name: "output doesnt exist",
		p: tb.Pipeline("pipelines", tb.PipelineSpec(
			tb.PipelineTask("mytask1", "task",
				tb.PipelineTaskOutputResource("input1", "git-resource"),
			),
		)),
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return clustertask, nil }
	getCondition := func(name string) (*v1alpha1.Condition, error) {
		return nil, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun",
				},
			}
			_, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, tt.p.Spec.Tasks, providedResources)
			if err == nil {
				t.Fatalf("Expected error when bindings are in incorrect state for Pipeline %s but got none", p.Name)
			}
		})
	}
}

func TestResolvePipelineRun_withExistingTaskRuns(t *testing.T) {
	names.TestingSeed()

	p := tb.Pipeline("pipelines", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
		tb.PipelineTask("mytask-with-a-really-long-name-to-trigger-truncation", "task",
			tb.PipelineTaskInputResource("input1", "git-resource"),
		),
	))

	r := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "someresource",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeGit,
		},
	}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{"git-resource": r}
	taskrunStatus := map[string]*v1beta1.PipelineRunTaskRunStatus{}
	taskrunStatus["pipelinerun-mytask-with-a-really-long-name-to-trigger-tru-9l9zj"] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "mytask-with-a-really-long-name-to-trigger-truncation",
		Status:           &v1beta1.TaskRunStatus{},
	}

	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: taskrunStatus,
			},
		},
	}

	// The Task "task" doesn't actually take any inputs or outputs, but validating
	// that is not done as part of Run resolution
	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return nil, nil }
	pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, p.Spec.Tasks, providedResources)
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
			Inputs: map[string]*resourcev1alpha1.PipelineResource{
				"input1": r,
			},
			Outputs: map[string]*resourcev1alpha1.PipelineResource{},
		},
	}}

	if d := cmp.Diff(pipelineState, expectedState, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected to get current pipeline state %v, but actual differed %s", expectedState, diff.PrintWantGot(d))
	}
}

func TestResolvedPipelineRun_PipelineTaskHasOptionalResources(t *testing.T) {
	names.TestingSeed()
	p := tb.Pipeline("pipelines", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
		tb.PipelineTask("mytask1", "task",
			tb.PipelineTaskInputResource("required-input", "git-resource"),
			tb.PipelineTaskOutputResource("required-output", "git-resource"),
		),
	))

	r := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "someresource",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeGit,
		},
	}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{"git-resource": r}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return taskWithOptionalResourcesDeprecated, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, nil }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return nil, nil }

	pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, p.Spec.Tasks, providedResources)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedState := PipelineRunState{{
		PipelineTask: &p.Spec.Tasks[0],
		TaskRunName:  "pipelinerun-mytask1-9l9zj",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: taskWithOptionalResources.Name,
			TaskSpec: &taskWithOptionalResources.Spec,
			Inputs: map[string]*resourcev1alpha1.PipelineResource{
				"required-input": r,
			},
			Outputs: map[string]*resourcev1alpha1.PipelineResource{
				"required-output": r,
			},
		},
	}}

	if d := cmp.Diff(expectedState, pipelineState, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Errorf("Expected to get current pipeline state %v, but actual differed %s", expectedState, diff.PrintWantGot(d))
	}
}

func TestResolveConditionChecks(t *testing.T) {
	names.TestingSeed()
	ccName := "pipelinerun-mytask1-9l9zj-always-true-mz4c7"

	cc := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: ccName,
		},
	}

	ptc := v1beta1.PipelineTaskCondition{
		ConditionRef: "always-true",
	}

	pts := []v1beta1.PipelineTask{{
		Name:       "mytask1",
		TaskRef:    &v1beta1.TaskRef{Name: "task"},
		Conditions: []v1beta1.PipelineTaskCondition{ptc},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, errors.New("should not get called") }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return &condition, nil }
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	tcs := []struct {
		name                   string
		getTaskRun             resources.GetTaskRun
		expectedConditionCheck TaskConditionCheckState
	}{
		{
			name: "conditionCheck exists",
			getTaskRun: func(name string) (*v1beta1.TaskRun, error) {
				switch name {
				case "pipelinerun-mytask1-9l9zj-always-true-0-mz4c7":
					return cc, nil
				case "pipelinerun-mytask1-9l9zj":
					return &trs[0], nil
				default:
					return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
				}
			},
			expectedConditionCheck: TaskConditionCheckState{{
				ConditionRegisterName: "always-true-0",
				ConditionCheckName:    "pipelinerun-mytask1-9l9zj-always-true-0-mz4c7",
				Condition:             &condition,
				ConditionCheck:        v1beta1.NewConditionCheck(cc),
				PipelineTaskCondition: &ptc,
				ResolvedResources:     providedResources,
			}},
		},
		{
			name: "conditionCheck doesn't exist",
			getTaskRun: func(name string) (*v1beta1.TaskRun, error) {
				if name == "pipelinerun-mytask1-mssqb-always-true-0-78c5n" {
					return nil, nil
				} else if name == "pipelinerun-mytask1-mssqb" {
					return &trs[0], nil
				}
				return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
			},
			expectedConditionCheck: TaskConditionCheckState{{
				ConditionRegisterName: "always-true-0",
				ConditionCheckName:    "pipelinerun-mytask1-mssqb-always-true-0-78c5n",
				Condition:             &condition,
				PipelineTaskCondition: &ptc,
				ResolvedResources:     providedResources,
			}},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, tc.getTaskRun, getClusterTask, getCondition, pts, providedResources)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun without Conditions: %v", err)
			}

			if d := cmp.Diff(tc.expectedConditionCheck, pipelineState[0].ResolvedConditionChecks, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{}, ResolvedConditionCheck{})); d != "" {
				t.Fatalf("ConditionChecks did not resolve as expected for case %s %s", tc.name, diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveConditionChecks_MultipleConditions(t *testing.T) {
	names.TestingSeed()
	ccName1 := "pipelinerun-mytask1-9l9zj-always-true-mz4c7"
	ccName2 := "pipelinerun-mytask1-9l9zj-always-true-mssqb"

	cc1 := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: ccName1,
		},
	}

	cc2 := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: ccName2,
		},
	}

	ptc1 := v1beta1.PipelineTaskCondition{
		ConditionRef: "always-true",
		Params:       []v1beta1.Param{{Name: "path", Value: *tb.ArrayOrString("$(params.path)")}, {Name: "image", Value: *tb.ArrayOrString("$(params.image)")}},
	}

	ptc2 := v1beta1.PipelineTaskCondition{
		ConditionRef: "always-true",
		Params:       []v1beta1.Param{{Name: "path", Value: *tb.ArrayOrString("$(params.path-test)")}, {Name: "image", Value: *tb.ArrayOrString("$(params.image-test)")}},
	}

	pts := []v1beta1.PipelineTask{{
		Name:       "mytask1",
		TaskRef:    &v1beta1.TaskRef{Name: "task"},
		Conditions: []v1beta1.PipelineTaskCondition{ptc1, ptc2},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, errors.New("should not get called") }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return &condition, nil }
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	tcs := []struct {
		name                   string
		getTaskRun             resources.GetTaskRun
		expectedConditionCheck TaskConditionCheckState
	}{
		{
			name: "conditionCheck exists",
			getTaskRun: func(name string) (*v1beta1.TaskRun, error) {
				switch name {
				case "pipelinerun-mytask1-9l9zj-always-true-0-mz4c7":
					return cc1, nil
				case "pipelinerun-mytask1-9l9zj":
					return &trs[0], nil
				case "pipelinerun-mytask1-9l9zj-always-true-1-mssqb":
					return cc2, nil
				}
				return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
			},
			expectedConditionCheck: TaskConditionCheckState{{
				ConditionRegisterName: "always-true-0",
				ConditionCheckName:    "pipelinerun-mytask1-9l9zj-always-true-0-mz4c7",
				Condition:             &condition,
				ConditionCheck:        v1beta1.NewConditionCheck(cc1),
				PipelineTaskCondition: &ptc1,
				ResolvedResources:     providedResources,
			}, {
				ConditionRegisterName: "always-true-1",
				ConditionCheckName:    "pipelinerun-mytask1-9l9zj-always-true-1-mssqb",
				Condition:             &condition,
				ConditionCheck:        v1beta1.NewConditionCheck(cc2),
				PipelineTaskCondition: &ptc2,
				ResolvedResources:     providedResources,
			}},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, tc.getTaskRun, getClusterTask, getCondition, pts, providedResources)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun without Conditions: %v", err)
			}

			if d := cmp.Diff(tc.expectedConditionCheck, pipelineState[0].ResolvedConditionChecks, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{}, ResolvedConditionCheck{})); d != "" {
				t.Fatalf("ConditionChecks did not resolve as expected for case %s %s", tc.name, diff.PrintWantGot(d))
			}
		})
	}
}
func TestResolveConditionChecks_ConditionDoesNotExist(t *testing.T) {
	names.TestingSeed()
	trName := "pipelinerun-mytask1-9l9zj"
	ccName := "pipelinerun-mytask1-9l9zj-does-not-exist-mz4c7"

	pts := []v1beta1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
		Conditions: []v1beta1.PipelineTaskCondition{{
			ConditionRef: "does-not-exist",
		}},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) {
		if name == ccName {
			return nil, fmt.Errorf("should not be called")
		} else if name == trName {
			return &trs[0], nil
		}
		return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
	}
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, errors.New("should not get called") }
	getCondition := func(name string) (*v1alpha1.Condition, error) {
		return nil, kerrors.NewNotFound(v1beta1.Resource("condition"), name)
	}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	_, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, pts, providedResources)

	switch err := err.(type) {
	case nil:
		t.Fatalf("Expected error getting non-existent Conditions but got none")
	case *ConditionNotFoundError:
		// expected error
	default:
		t.Fatalf("Expected specific error type returned by func for non-existent Condition got %s", err)
	}
}

func TestResolveConditionCheck_UseExistingConditionCheckName(t *testing.T) {
	names.TestingSeed()

	trName := "pipelinerun-mytask1-9l9zj"
	ccName := "some-random-name"

	cc := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: ccName,
		},
		Spec: v1beta1.TaskRunSpec{},
	}

	ptc := v1beta1.PipelineTaskCondition{
		ConditionRef: "always-true",
	}

	pts := []v1beta1.PipelineTask{{
		Name:       "mytask1",
		TaskRef:    &v1beta1.TaskRef{Name: "task"},
		Conditions: []v1beta1.PipelineTaskCondition{ptc},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) {
		if name == ccName {
			return cc, nil
		} else if name == trName {
			return &trs[0], nil
		}
		return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
	}
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, errors.New("should not get called") }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return &condition, nil }

	ccStatus := make(map[string]*v1beta1.PipelineRunConditionCheckStatus)
	ccStatus[ccName] = &v1beta1.PipelineRunConditionCheckStatus{
		ConditionName: "always-true-0",
	}
	trStatus := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	trStatus[trName] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "mytask-1",
		ConditionChecks:  ccStatus,
	}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: trStatus,
			},
		},
	}

	pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, pts, providedResources)
	if err != nil {
		t.Fatalf("Did not expect error when resolving PipelineRun without Conditions: %v", err)
	}
	expectedConditionChecks := TaskConditionCheckState{{
		ConditionRegisterName: "always-true-0",
		ConditionCheckName:    ccName,
		Condition:             &condition,
		ConditionCheck:        v1beta1.NewConditionCheck(cc),
		PipelineTaskCondition: &ptc,
		ResolvedResources:     providedResources,
	}}

	if d := cmp.Diff(expectedConditionChecks, pipelineState[0].ResolvedConditionChecks, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{}, ResolvedConditionCheck{})); d != "" {
		t.Fatalf("ConditionChecks did not resolve as expected %s", diff.PrintWantGot(d))
	}
}

func TestResolvedConditionCheck_WithResources(t *testing.T) {
	names.TestingSeed()

	condition := tbv1alpha1.Condition("always-true", tbv1alpha1.ConditionSpec(
		tbv1alpha1.ConditionResource("workspace", resourcev1alpha1.PipelineResourceTypeGit),
	))

	gitResource := tb.PipelineResource("some-repo", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
		resourcev1alpha1.PipelineResourceTypeGit))

	ptc := v1beta1.PipelineTaskCondition{
		ConditionRef: "always-true",
		Resources: []v1beta1.PipelineTaskInputResource{{
			Name:     "workspace",
			Resource: "blah", // The name used in the pipeline
		}},
	}

	pts := []v1beta1.PipelineTask{{
		Name:       "mytask1",
		TaskRef:    &v1beta1.TaskRef{Name: "task"},
		Conditions: []v1beta1.PipelineTaskCondition{ptc},
	}}

	getTask := func(name string) (v1beta1.TaskInterface, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, errors.New("should not get called") }

	// This err result is required to satisfy the type alias on this function, but it triggers
	// a false positive in the linter: https://github.com/mvdan/unparam/issues/40
	// nolint: unparam
	getCondition := func(_ string) (*v1alpha1.Condition, error) {
		return condition, nil
	}

	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	tcs := []struct {
		name              string
		providedResources map[string]*resourcev1alpha1.PipelineResource
		wantErr           bool
		expected          map[string]*resourcev1alpha1.PipelineResource
	}{{
		name:              "resource exists",
		providedResources: map[string]*resourcev1alpha1.PipelineResource{"blah": gitResource},
		expected:          map[string]*resourcev1alpha1.PipelineResource{"workspace": gitResource},
	}, {
		name:              "resource does not exist",
		providedResources: map[string]*resourcev1alpha1.PipelineResource{},
		wantErr:           true,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pipelineState, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, pts, tc.providedResources)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Did not get error when it was expected for test %s", tc.name)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error when no error expected: %v", err)
				}
				expectedConditionChecks := TaskConditionCheckState{{
					ConditionRegisterName: "always-true-0",
					ConditionCheckName:    "pipelinerun-mytask1-9l9zj-always-true-0-mz4c7",
					Condition:             condition,
					PipelineTaskCondition: &ptc,
					ResolvedResources:     tc.expected,
				}}
				if d := cmp.Diff(expectedConditionChecks, pipelineState[0].ResolvedConditionChecks, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{}, ResolvedConditionCheck{})); d != "" {
					t.Fatalf("ConditionChecks did not resolve as expected %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestValidateResourceBindings(t *testing.T) {
	p := tb.Pipeline("pipelines", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
	))
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
	))
	err := ValidateResourceBindings(&p.Spec, pr)
	if err != nil {
		t.Fatalf("didn't expect error getting resources from bindings but got: %v", err)
	}
}

func TestValidateResourceBindings_Missing(t *testing.T) {
	p := tb.Pipeline("pipelines", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
		tb.PipelineDeclaredResource("image-resource", "image"),
	))
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
	))
	err := ValidateResourceBindings(&p.Spec, pr)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was missing but got no error")
	}
}

func TestGetResourcesFromBindings_Extra(t *testing.T) {
	p := tb.Pipeline("pipelines", tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-resource", "git"),
	))
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunResourceBinding("git-resource", tb.PipelineResourceBindingRef("sweet-resource")),
		tb.PipelineRunResourceBinding("image-resource", tb.PipelineResourceBindingRef("sweet-resource2")),
	))
	err := ValidateResourceBindings(&p.Spec, pr)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was extra but got no error")
	}
}

func TestValidateWorkspaceBindings(t *testing.T) {
	p := tb.Pipeline("pipelines", tb.PipelineSpec(
		tb.PipelineWorkspaceDeclaration("foo"),
	))
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunWorkspaceBindingEmptyDir("bar"),
	))
	if err := ValidateWorkspaceBindings(&p.Spec, pr); err == nil {
		t.Fatalf("Expected error indicating `foo` workspace was not provided but got no error")
	}
}

func TestValidateServiceaccountMapping(t *testing.T) {
	p := tb.Pipeline("pipelines", tb.PipelineSpec(
		tb.PipelineTask("mytask1", "task",
			tb.PipelineTaskInputResource("input1", "git-resource")),
	))
	pr := tb.PipelineRun("pipelinerun", tb.PipelineRunSpec("pipeline",
		tb.PipelineRunServiceAccountNameTask("mytaskwrong", "default"),
	))
	if err := ValidateServiceaccountMapping(&p.Spec, pr); err == nil {
		t.Fatalf("Expected error indicating `mytaskwrong` was not defined as `task` in Pipeline but got no error")
	}
}

func TestIsBeforeFirstTaskRun_WithNotStartedTask(t *testing.T) {
	if !noneStartedState.IsBeforeFirstTaskRun() {
		t.Fatalf("Expected state to be before first taskrun")
	}
}

func TestIsBeforeFirstTaskRun_WithStartedTask(t *testing.T) {
	if oneStartedState.IsBeforeFirstTaskRun() {
		t.Fatalf("Expected state to be after first taskrun")
	}
}

func TestPipelineRunState_GetFinalTasks(t *testing.T) {
	tcs := []struct {
		name               string
		desc               string
		state              PipelineRunState
		DAGTasks           []v1beta1.PipelineTask
		finalTasks         []v1beta1.PipelineTask
		expectedFinalTasks []*ResolvedPipelineRunTask
	}{{
		// tasks: [ mytask1, mytask2]
		// none finally
		name: "01 - DAG tasks done, no final tasks",
		desc: "DAG tasks (mytask1 and mytask2) finished successfully -" +
			" do not schedule final tasks since pipeline didnt have any",
		state:              oneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[1]},
		finalTasks:         []v1beta1.PipelineTask{},
		expectedFinalTasks: []*ResolvedPipelineRunTask{},
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "02 - DAG task not started, no final tasks",
		desc:               "DAG tasks (mytask1) not started yet - do not schedule final tasks (mytask2)",
		state:              noneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{},
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "03 - DAG task not finished, no final tasks",
		desc:               "DAG tasks (mytask1) started but not finished - do not schedule final tasks (mytask2)",
		state:              oneStartedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{},
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "04 - DAG task done, return final tasks",
		desc:               "DAG tasks (mytask1) done - schedule final tasks (mytask2)",
		state:              oneFinishedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{oneFinishedState[1]},
	}, {
		// tasks: [ mytask1]
		// finally: [mytask2]
		name:               "05 - DAG task failed, return final tasks",
		desc:               "DAG task (mytask1) failed - schedule final tasks (mytask2)",
		state:              oneFailedState,
		DAGTasks:           []v1beta1.PipelineTask{pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{oneFinishedState[1]},
	}, {
		// tasks: [ mytask6 with condition]
		// finally: [mytask2]
		name:               "06 - DAG task condition started, no final tasks",
		desc:               "DAG task (mytask6) condition started - do not schedule final tasks (mytask1)",
		state:              append(conditionCheckStartedState, noneStartedState[0]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5]},
		finalTasks:         []v1beta1.PipelineTask{pts[0]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{},
	}, {
		// tasks: [ mytask6 with condition]
		// finally: [mytask2]
		name:               "07 - DAG task condition done, no final tasks",
		desc:               "DAG task (mytask6) condition finished, mytask6 not started - do not schedule final tasks (mytask2)",
		state:              append(conditionCheckSuccessNoTaskStartedState, noneStartedState[0]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5]},
		finalTasks:         []v1beta1.PipelineTask{pts[0]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{},
	}, {
		// tasks: [ mytask6 with condition]
		// finally: [mytask2]
		name:               "08 - DAG task skipped, return final tasks",
		desc:               "DAG task (mytask6) condition failed - schedule final tasks (mytask2) ",
		state:              append(conditionCheckFailedWithNoOtherTasksState, noneStartedState[0]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5]},
		finalTasks:         []v1beta1.PipelineTask{pts[0]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{noneStartedState[0]},
	}, {
		// tasks: [ mytask1, mytask6 with condition]
		// finally: [mytask2]
		name:               "09 - DAG task succeeded/skipped, return final tasks ",
		desc:               "DAG task (mytask1) finished, mytask6 condition failed - schedule final tasks (mytask2)",
		state:              append(conditionCheckFailedWithOthersPassedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5], pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{noneStartedState[1]},
	}, {
		// tasks: [ mytask1, mytask6 with condition]
		// finally: [mytask2]
		name:               "10 - DAG task failed/skipped, return final tasks",
		desc:               "DAG task (mytask1) failed, mytask6 condition failed - schedule final tasks (mytask2)",
		state:              append(conditionCheckFailedWithOthersFailedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5], pts[0]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{noneStartedState[1]},
	}, {
		// tasks: [ mytask6 with condition, mytask7 runAfter mytask6]
		// finally: [mytask2]
		name:               "11 - DAG task skipped, return final tasks",
		desc:               "DAG task (mytask6) condition failed, mytask6 and mytask7 skipped - schedule final tasks (mytask2)",
		state:              append(taskWithParentSkippedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[5], pts[6]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{noneStartedState[1]},
	}, {
		// tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6]
		// finally: [mytask2]
		name:               "12 - DAG task succeeded/skipped, return final tasks",
		desc:               "DAG task (mytask1) finished - DAG task (mytask6) condition failed, mytask6 and mytask8 skipped - schedule final tasks (mytask2)",
		state:              append(taskWithMultipleParentsSkippedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{noneStartedState[1]},
	}, {
		// tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6, mytask9 runAfter mytask1 and mytask6]
		// finally: [mytask2]
		name: "13 - DAG task succeeded/skipped - return final tasks",
		desc: "DAG task (mytask1) finished - DAG task (mytask6) condition failed, mytask6, mytask8, and mytask9 skipped" +
			"- schedule final tasks (mytask2)",
		state:              append(taskWithGrandParentSkippedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7], pts[8]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{noneStartedState[1]},
	}, {
		//tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6, mytask9 runAfter mytask1 and mytask6]
		//finally: [mytask2]
		name: "14 - DAG task succeeded, skipped - return final tasks",
		desc: "DAG task (mytask1) finished - DAG task (mytask6) failed - mytask8 and mytask9 skipped" +
			"- schedule final tasks (mytask2)",
		state:              append(taskWithGrandParentsOneFailedState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7], pts[8]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{noneStartedState[1]},
	}, {
		//tasks: [ mytask1, mytask6 with condition, mytask8 runAfter mytask6, mytask9 runAfter mytask1 and mytask6]
		//finally: [mytask2]
		name:               "15 - DAG task succeeded/started - no final tasks",
		desc:               "DAG task (mytask1) finished - DAG task (mytask6) started - do no schedule final tasks",
		state:              append(taskWithGrandParentsOneNotRunState, noneStartedState[1]),
		DAGTasks:           []v1beta1.PipelineTask{pts[0], pts[5], pts[7], pts[8]},
		finalTasks:         []v1beta1.PipelineTask{pts[1]},
		expectedFinalTasks: []*ResolvedPipelineRunTask{},
	}}
	for _, tc := range tcs {
		dagGraph, err := dag.Build(v1beta1.PipelineTaskList(tc.DAGTasks))
		if err != nil {
			t.Fatalf("Unexpected error while buildig DAG for pipelineTasks %v: %v", tc.DAGTasks, err)
		}
		finalGraph, err := dag.Build(v1beta1.PipelineTaskList(tc.finalTasks))
		if err != nil {
			t.Fatalf("Unexpected error while buildig DAG for final pipelineTasks %v: %v", tc.finalTasks, err)
		}
		t.Run(tc.name, func(t *testing.T) {
			next := tc.state.GetFinalTasks(dagGraph, finalGraph)
			if d := cmp.Diff(tc.expectedFinalTasks, next); d != "" {
				t.Errorf("Didn't get expected final Tasks for %s (%s): %s", tc.name, tc.desc, diff.PrintWantGot(d))
			}
		})
	}
}
