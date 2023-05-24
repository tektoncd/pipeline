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
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func nopGetCustomRun(string) (*v1beta1.CustomRun, error) {
	return nil, errors.New("GetRun should not be called")
}
func nopGetTask(context.Context, string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
	return nil, nil, nil, errors.New("GetTask should not be called")
}
func nopGetTaskRun(string) (*v1.TaskRun, error) {
	return nil, errors.New("GetTaskRun should not be called")
}

var pts = []v1.PipelineTask{{
	Name:    "mytask1",
	TaskRef: &v1.TaskRef{Name: "task"},
}, {
	Name:    "mytask2",
	TaskRef: &v1.TaskRef{Name: "task"},
}, {
	Name:    "mytask3",
	TaskRef: &v1.TaskRef{Name: "clustertask"},
}, {
	Name:    "mytask4",
	TaskRef: &v1.TaskRef{Name: "task"},
	Retries: 1,
}, {
	Name:    "mytask5",
	TaskRef: &v1.TaskRef{Name: "cancelledTask"},
	Retries: 2,
}, {
	Name:    "mytask6",
	TaskRef: &v1.TaskRef{Name: "task"},
}, {
	Name:     "mytask7",
	TaskRef:  &v1.TaskRef{Name: "taskWithOneParent"},
	RunAfter: []string{"mytask6"},
}, {
	Name:     "mytask8",
	TaskRef:  &v1.TaskRef{Name: "taskWithTwoParents"},
	RunAfter: []string{"mytask1", "mytask6"},
}, {
	Name:     "mytask9",
	TaskRef:  &v1.TaskRef{Name: "taskHasParentWithRunAfter"},
	RunAfter: []string{"mytask8"},
}, {
	Name:    "mytask10",
	TaskRef: &v1.TaskRef{Name: "taskWithWhenExpressions"},
	When: []v1.WhenExpression{{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo", "bar"},
	}},
}, {
	Name:    "mytask11",
	TaskRef: &v1.TaskRef{Name: "taskWithWhenExpressions"},
	When: []v1.WhenExpression{{
		Input:    "foo",
		Operator: selection.NotIn,
		Values:   []string{"foo", "bar"},
	}},
}, {
	Name:    "mytask12",
	TaskRef: &v1.TaskRef{Name: "taskWithWhenExpressions"},
	When: []v1.WhenExpression{{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo", "bar"},
	}},
	RunAfter: []string{"mytask1"},
}, {
	Name:    "mytask13",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
}, {
	Name:    "mytask14",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
}, {
	Name:    "mytask15",
	TaskRef: &v1.TaskRef{Name: "taskWithReferenceToTaskResult"},
	Params:  v1.Params{{Name: "param1", Value: *v1.NewStructuredValues("$(tasks.mytask1.results.result1)")}},
}, {
	Name: "mytask16",
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name: "mytask17",
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask18",
	TaskRef: &v1.TaskRef{Name: "task"},
	Retries: 1,
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask19",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask20",
	TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask21",
	TaskRef: &v1.TaskRef{Name: "task"},
	Retries: 2,
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}}

var p = &v1.Pipeline{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipeline",
	},
	Spec: v1.PipelineSpec{
		Tasks: pts,
	},
}

var task = &v1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name: "task",
	},
	Spec: v1.TaskSpec{
		Steps: []v1.Step{{
			Name: "step1",
		}},
	},
}

var trs = []v1.TaskRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask1",
	},
	Spec: v1.TaskRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask2",
	},
	Spec: v1.TaskRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask4",
	},
	Spec: v1.TaskRunSpec{},
}}

var customRuns = []v1beta1.CustomRun{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask13",
	},
	Spec: v1beta1.CustomRunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask14",
	},
	Spec: v1beta1.CustomRunSpec{},
}}

var matrixedPipelineTask = &v1.PipelineTask{
	Name: "task",
	Matrix: &v1.Matrix{
		Params: v1.Params{{
			Name:  "browser",
			Value: v1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}

func makeScheduled(tr v1.TaskRun) *v1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status = v1.TaskRunStatus{ /* explicitly empty */ }
	return newTr
}

func makeStarted(tr v1.TaskRun) *v1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newTr
}

func makeCustomRunStarted(run v1beta1.CustomRun) *v1beta1.CustomRun {
	newRun := newCustomRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newRun
}

func makeSucceeded(tr v1.TaskRun) *v1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionTrue
	return newTr
}

func makeCustomRunSucceeded(run v1beta1.CustomRun) *v1beta1.CustomRun {
	newRun := newCustomRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionTrue
	return newRun
}

func makeFailed(tr v1.TaskRun) *v1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionFalse
	return newTr
}

func makeToBeRetried(tr v1.TaskRun) *v1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionUnknown
	newTr.Status.Conditions[0].Reason = v1.TaskRunReasonToBeRetried.String()
	return newTr
}

func makeCustomRunFailed(run v1beta1.CustomRun) *v1beta1.CustomRun {
	newRun := newCustomRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionFalse
	return newRun
}

func withCancelled(tr *v1.TaskRun) *v1.TaskRun {
	tr.Status.Conditions[0].Reason = v1.TaskRunSpecStatusCancelled
	return tr
}

func withCancelledForTimeout(tr *v1.TaskRun) *v1.TaskRun {
	tr.Spec.StatusMessage = v1.TaskRunCancelledByPipelineTimeoutMsg
	tr.Status.Conditions[0].Reason = v1.TaskRunSpecStatusCancelled
	return tr
}

func withCustomRunCancelled(run *v1beta1.CustomRun) *v1beta1.CustomRun {
	run.Status.Conditions[0].Reason = v1beta1.CustomRunReasonCancelled.String()
	return run
}

func withCustomRunCancelledForTimeout(run *v1beta1.CustomRun) *v1beta1.CustomRun {
	run.Spec.StatusMessage = v1beta1.CustomRunCancelledByPipelineTimeoutMsg
	run.Status.Conditions[0].Reason = v1beta1.CustomRunReasonCancelled.String()
	return run
}

func withCancelledBySpec(tr *v1.TaskRun) *v1.TaskRun {
	tr.Spec.Status = v1.TaskRunSpecStatusCancelled
	return tr
}

func withCustomRunCancelledBySpec(run *v1beta1.CustomRun) *v1beta1.CustomRun {
	run.Spec.Status = v1beta1.CustomRunSpecStatusCancelled
	return run
}

func makeRetried(tr v1.TaskRun) (newTr *v1.TaskRun) {
	newTr = newTaskRun(tr)
	newTr = withRetries(newTr)
	return
}

func withRetries(tr *v1.TaskRun) *v1.TaskRun {
	tr.Status.RetriesStatus = []v1.TaskRunStatus{{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return tr
}

func withCustomRunRetries(r *v1beta1.CustomRun) *v1beta1.CustomRun {
	r.Status.RetriesStatus = []v1beta1.CustomRunStatus{{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return r
}

func newTaskRun(tr v1.TaskRun) *v1.TaskRun {
	return &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			Name:      tr.Name,
		},
		Spec: tr.Spec,
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}
}

func withPipelineTaskRetries(pt v1.PipelineTask, retries int) *v1.PipelineTask {
	pt.Retries = retries
	return &pt
}

func newCustomRun(run v1beta1.CustomRun) *v1beta1.CustomRun {
	return &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: run.Namespace,
			Name:      run.Name,
		},
		Spec: run.Spec,
		Status: v1beta1.CustomRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}
}

var noneStartedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneStartedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeStarted(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneFinishedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneFailedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var finalScheduledState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     []*v1.TaskRun{makeScheduled(trs[1])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var allFinishedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var noCustomRunStartedState = PipelineRunState{{
	PipelineTask:   &pts[12],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     nil,
}, {
	PipelineTask:   &pts[13],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask14"},
	CustomRuns:     nil,
}}

var oneCustomRunStartedState = PipelineRunState{{
	PipelineTask:   &pts[12],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
}, {
	PipelineTask:   &pts[13],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask14"},
	CustomRuns:     nil,
}}

var oneCustomRunFinishedState = PipelineRunState{{
	PipelineTask:   &pts[12],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0])},
}, {
	PipelineTask:   &pts[13],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask14"},
	CustomRuns:     nil,
}}

var oneCustomRunFailedState = PipelineRunState{{
	PipelineTask:   &pts[12],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
}, {
	PipelineTask:   &pts[13],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask14"},
	CustomRuns:     nil,
}}

var taskCancelled = PipelineRunState{{
	PipelineTask: &pts[4],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{withCancelled(makeRetried(trs[0]))},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var customRunCancelled = PipelineRunState{{
	PipelineTask:   &pts[12],
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0]))},
}}

var noneStartedStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask3"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneStartedStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeStarted(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneFinishedStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneFailedStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var finalScheduledStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     []*v1.TaskRun{makeScheduled(trs[1])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var allFinishedStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var noCustomRunStartedStateMatrix = PipelineRunState{{
	PipelineTask:   &pts[18],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     nil,
}, {
	PipelineTask:   &pts[19],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     nil,
}}

var oneCustomRunStartedStateMatrix = PipelineRunState{{
	PipelineTask:   &pts[18],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
}, {
	PipelineTask:   &pts[19],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask14"},
	CustomRuns:     nil,
}}

var oneCustomRunFailedStateMatrix = PipelineRunState{{
	PipelineTask:   &pts[18],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask13"},
	CustomRuns:     []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
}, {
	PipelineTask:   &pts[19],
	CustomTask:     true,
	CustomRunNames: []string{"pipelinerun-mytask14"},
	CustomRuns:     nil,
}}

var taskCancelledMatrix = PipelineRunState{{
	PipelineTask: &pts[20],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1.TaskRun{withCancelled(makeRetried(trs[0]))},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

func dagFromState(state PipelineRunState) (*dag.Graph, error) {
	pts := []v1.PipelineTask{}
	for _, rpt := range state {
		pts = append(pts, *rpt.PipelineTask)
	}
	return dag.Build(v1.PipelineTaskList(pts), v1.PipelineTaskList(pts).Deps())
}

func TestIsSkipped(t *testing.T) {
	for _, tc := range []struct {
		name            string
		state           PipelineRunState
		startTime       time.Time
		tasksTimeout    time.Duration
		pipelineTimeout time.Duration
		expected        map[string]bool
	}{{
		name:  "tasks-failed",
		state: oneFailedState,
		expected: map[string]bool{
			"mytask1": false,
		},
	}, {
		name:  "tasks-passed",
		state: oneFinishedState,
		expected: map[string]bool{
			"mytask1": false,
		},
	}, {
		name:  "tasks-cancelled",
		state: taskCancelled,
		expected: map[string]bool{
			"mytask5": false,
		},
	}, {
		name:  "tasks-scheduled",
		state: finalScheduledState,
		expected: map[string]bool{
			"mytask1": false,
			"mytask2": false,
		},
	}, {
		name: "tasks-parent-failed",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunNames: []string{"pipelinerun-mytask1"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunNames: []string{"pipelinerun-mytask2"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask7": true,
		},
	}, {
		name: "tasks-parent-cancelled",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunNames: []string{"pipelinerun-mytask1"},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunNames: []string{"pipelinerun-mytask2"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask7": true,
		},
	}, {
		name: "tasks-grandparent-failed",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunNames: []string{"pipelinerun-mytask1"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunNames: []string{"pipelinerun-mytask2"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask10",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask7"},
			}, // mytask10 runAfter mytask7 runAfter mytask6
			TaskRunNames: []string{"pipelinerun-mytask3"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask10": true,
		},
	}, {
		name: "tasks-parents-failed-passed",
		state: PipelineRunState{{
			PipelineTask: &pts[5],
			TaskRunNames: []string{"pipelinerun-mytask1"},
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[0],
			TaskRunNames: []string{"pipelinerun-mytask2"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[7], // mytask8 runAfter mytask1, mytask6
			TaskRunNames: []string{"pipelinerun-mytask3"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask8": true,
		},
	}, {
		name: "task-failed-pipeline-stopping",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRunNames: []string{"pipelinerun-mytask1"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[5],
			TaskRunNames: []string{"pipelinerun-mytask2"},
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[1])},
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunNames: []string{"pipelinerun-mytask3"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask7": true,
		},
	}, {
		name: "tasks-when-expressions-passed",
		state: PipelineRunState{{
			PipelineTask: &pts[9],
			TaskRunNames: []string{"pipelinerun-guardedtask"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask10": false,
		},
	}, {
		name: "tasks-when-expression-failed",
		state: PipelineRunState{{
			PipelineTask: &pts[10],
			TaskRunNames: []string{"pipelinerun-guardedtask"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask11": true,
		},
	}, {
		name: "when-expression-task-but-without-parent-done",
		state: PipelineRunState{{
			PipelineTask: &pts[0],
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[11],
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask12": false,
		},
	}, {
		name:  "customrun-started",
		state: oneCustomRunStartedState,
		expected: map[string]bool{
			"mytask13": false,
		},
	}, {
		name:  "customrun-cancelled",
		state: customRunCancelled,
		expected: map[string]bool{
			"mytask13": false,
		},
	}, {
		name: "tasks-when-expressions",
		state: PipelineRunState{{
			// skipped because when expressions evaluate to false
			PipelineTask: &pts[10],
			TaskRunNames: []string{"pipelinerun-guardedtask"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its parent task being skipped because when expressions are scoped to task
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-1"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask11": true,
			"mytask18": false,
		},
	}, {
		name: "tasks-when-expressions-run-multiple-dependent-tasks",
		state: PipelineRunState{{
			// skipped because when expressions evaluate to false
			PipelineTask: &pts[10],
			TaskRunNames: []string{"pipelinerun-guardedtask"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its parent task being skipped because when expressions are scoped to task
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-1"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its grandparent task being skipped because when expressions are scoped to task
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask19",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask18"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-2"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask11": true,
			"mytask18": false,
			"mytask19": false,
		},
	}, {
		name: "tasks-when-expressions-run-multiple-ordering-and-resource-dependent-tasks",
		state: PipelineRunState{{
			// skipped because when expressions evaluate to false
			PipelineTask: &pts[10],
			TaskRunNames: []string{"pipelinerun-guardedtask"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its parent task mytask11 being skipped because when expressions are scoped to task
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-1"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its grandparent task mytask11 being skipped because when expressions are scoped to task
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask19",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask18"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-2"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// attempted but skipped because of missing result in params from parent task mytask11 which was skipped
			PipelineTask: &v1.PipelineTask{
				Name:    "mytask20",
				TaskRef: &v1.TaskRef{Name: "task"},
				Params: v1.Params{{
					Name:  "commit",
					Value: *v1.NewStructuredValues("$(tasks.mytask11.results.missingResult)"),
				}},
			},
			TaskRunNames: []string{"pipelinerun-resource-dependent-task-1"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// skipped because of parent task mytask20 was skipped because of missing result from grandparent task
			// mytask11 which was skipped
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask21",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask20"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-3"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// attempted but skipped because of missing result from parent task mytask11 which was skipped in when expressions
			PipelineTask: &v1.PipelineTask{
				Name:    "mytask22",
				TaskRef: &v1.TaskRef{Name: "task"},
				When: v1.WhenExpressions{{
					Input:    "$(tasks.mytask11.results.missingResult)",
					Operator: selection.In,
					Values:   []string{"expectedResult"},
				}},
			},
			TaskRunNames: []string{"pipelinerun-resource-dependent-task-2"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// skipped because of parent task mytask22 was skipping because of missing result from grandparent task
			// mytask11 which was skipped
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask23",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask22"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-4"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask11": true,
			"mytask18": false,
			"mytask19": false,
			"mytask20": true,
			"mytask21": true,
			"mytask22": true,
			"mytask23": true,
		},
	}, {
		name:         "pipeline-tasks-timeout-not-reached",
		state:        oneStartedState,
		startTime:    now.Add(-5 * time.Minute),
		tasksTimeout: 10 * time.Minute,
		expected: map[string]bool{
			"mytask1": false,
			"mytask2": false,
		},
	}, {
		name:            "pipeline-timeout-not--reached",
		state:           oneStartedState,
		startTime:       now.Add(-5 * time.Minute),
		pipelineTimeout: 10 * time.Minute,
		expected: map[string]bool{
			"mytask1": false,
			"mytask2": false,
		},
	}, {
		name:         "pipeline-tasks-timeout-reached",
		state:        oneStartedState,
		startTime:    now.Add(-5 * time.Minute),
		tasksTimeout: 4 * time.Minute,
		expected: map[string]bool{
			"mytask1": false,
			"mytask2": true,
		},
	}, {
		name:            "pipeline-timeout-reached",
		state:           oneStartedState,
		startTime:       now.Add(-5 * time.Minute),
		pipelineTimeout: 4 * time.Minute,
		expected: map[string]bool{
			"mytask1": false,
			"mytask2": true,
		},
	}, {
		name: "matrix-params-contain-empty-arr",
		state: PipelineRunState{{
			// not skipped no empty arrs
			PipelineTask: &v1.PipelineTask{
				Name:    "mytask1",
				TaskRef: &v1.TaskRef{Name: "matrix-1"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name: "a-param",
						Value: v1.ParamValue{
							Type:     v1.ParamTypeArray,
							ArrayVal: []string{"foo", "bar"},
						},
					}}},
			},
			TaskRunNames: []string{"pipelinerun-matrix-empty-params"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// skipped empty ArrayVal exist in matrix param
			PipelineTask: &v1.PipelineTask{
				Name:    "mytask2",
				TaskRef: &v1.TaskRef{Name: "matrix-2"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name: "a-param",
						Value: v1.ParamValue{
							Type:     v1.ParamTypeArray,
							ArrayVal: []string{},
						},
					}}},
			},
			TaskRunNames: []string{"pipelinerun-matrix-empty-params"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// skipped empty ArrayVal exist in matrix param
			PipelineTask: &v1.PipelineTask{
				Name:    "mytask3",
				TaskRef: &v1.TaskRef{Name: "matrix-2"},
				Matrix: &v1.Matrix{
					Params: v1.Params{{
						Name: "a-param",
						Value: v1.ParamValue{
							Type:     v1.ParamTypeArray,
							ArrayVal: []string{"foo", "bar"},
						},
					}, {
						Name: "b-param",
						Value: v1.ParamValue{
							Type:     v1.ParamTypeArray,
							ArrayVal: []string{},
						},
					}}},
			},
			TaskRunNames: []string{"pipelinerun-matrix-empty-params"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask1": false,
			"mytask2": true,
			"mytask3": true,
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Could not get a dag from the TC state %#v: %v", tc.state, err)
			}
			stateMap := tc.state.ToMap()
			startTime := tc.startTime
			if startTime.IsZero() {
				startTime = now
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					StartTime: &startTime,
					Clock:     testClock,
				},
			}
			if tc.tasksTimeout != 0 {
				facts.TimeoutsState.TasksTimeout = &tc.tasksTimeout
			}
			if tc.pipelineTimeout != 0 {
				facts.TimeoutsState.PipelineTimeout = &tc.pipelineTimeout
			}
			for taskName, isSkipped := range tc.expected {
				rpt := stateMap[taskName]
				if rpt == nil {
					t.Fatalf("Could not get task %s from the state: %v", taskName, tc.state)
				}
				if d := cmp.Diff(isSkipped, rpt.Skip(&facts).IsSkipped); d != "" {
					t.Errorf("Didn't get expected isSkipped from task %s: %s", taskName, diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestIsFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		rpt  ResolvedPipelineTask
		want bool
	}{{
		name: "taskrun not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "run not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0])},
		},
		want: false,
	}, {
		name: "run running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
		},
		want: false,
	}, {
		name: "taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
		},
		want: false,
	}, {
		name: "run succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0])},
		},
		want: false,
	}, {
		name: "taskrun failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
		},
		want: true,
	}, {
		name: "run failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
		},
		want: true,
	}, {
		name: "run failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
		},
		want: true,
	}, {
		name: "taskrun failed - Retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withRetries(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "customrun failed - Retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunRetries(makeCustomRunFailed(customRuns[0]))},
		},
		want: true,
	}, {
		name: "taskrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled for timeout",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelledForTimeout(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "customrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "customrun cancelled for timeout",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelledForTimeout(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "customrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0]))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "customrun cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "taskrun cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0])))},
		},
		want: true,
	}, {
		name: "custom run cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0])))},
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "matrixed taskruns not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed runs not started",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed taskruns running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunSucceeded(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeFailed(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed runs failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunFailed(customRuns[1])},
		},
		want: true,
	}, {
		name: "one matrixed taskrun failed, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run failed, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withRetries(makeFailed(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs failed: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunFailed(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns failed: one taskrun with retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: one run with retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), withCustomRunRetries(makeCustomRunFailed(customRuns[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs failed: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunRetries(makeCustomRunFailed(customRuns[0])), withCustomRunRetries(makeCustomRunFailed(customRuns[1]))},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), withCustomRunCancelled(makeCustomRunFailed(customRuns[1]))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), withCancelled(newTaskRun(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), withCustomRunCancelled(newCustomRun(customRuns[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), withCustomRunCancelled(makeCustomRunFailed(customRuns[1]))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled: retries remaining, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retries remaining, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), withCancelled(withRetries(makeFailed(trs[1])))},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0]))), withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[1])))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled: retried, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retried, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0]))), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rpt.isFailure(); got != tc.want {
				t.Errorf("expected isFailure: %t but got %t", tc.want, got)
			}
		})
	}
}

func TestIsCancelled(t *testing.T) {
	tcs := []struct {
		name string
		rpt  ResolvedPipelineTask
		want bool
	}{{
		name: "taskruns not started",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{},
		},
		want: false,
	}, {
		name: "taskrun not done",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{makeStarted(trs[0])},
		},
		want: false,
	}, {
		name: "taskrun succeeded but not cancelled",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{(makeSucceeded(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun failed but not cancelled",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled and failed",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "taskrun cancelled but still running",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelled(makeStarted(trs[0]))},
		},
		want: false,
	}, {
		name: "one taskrun cancelled, one not done",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one taskrun cancelled, one done but not cancelled",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeSucceeded(trs[1])},
		},
		want: true,
	}, {
		name: "customruns not started",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{},
		},
		want: false,
	}, {
		name: "customrun not done",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
		},
		want: false,
	}, {
		name: "customrun succeeded but not cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{(makeCustomRunSucceeded(customRuns[0]))},
		},
		want: false,
	}, {
		name: "customrun failed but not cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{(makeCustomRunFailed(customRuns[0]))},
		},
		want: false,
	}, {
		name: "customrun cancelled and failed",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
		},
		want: true,
	}, {
		name: "customrun cancelled but still running",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunStarted(customRuns[0]))},
		},
		want: false,
	}, {
		name: "one customrun cancelled, one not done",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "one customrun cancelled, one done but not cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunSucceeded(customRuns[1])},
		},
		want: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rpt.isCancelled(); got != tc.want {
				t.Errorf("expected isCancelled: %t but got %t", tc.want, got)
			}
		})
	}
}

func TestIsCancelledForTimeout(t *testing.T) {
	tcs := []struct {
		name string
		rpt  ResolvedPipelineTask
		want bool
	}{{
		name: "taskruns not started",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{},
		},
		want: false,
	}, {
		name: "taskrun not done",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{makeStarted(trs[0])},
		},
		want: false,
	}, {
		name: "taskrun succeeded but not cancelled",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{(makeSucceeded(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun failed but not cancelled",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled by spec and failed",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelledBySpec(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled for timeout and failed",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelledForTimeout(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "taskrun cancelled for timeout but still running",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelledForTimeout(makeStarted(trs[0]))},
		},
		want: false,
	}, {
		name: "one taskrun cancelled for timeout, one not done",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelledForTimeout(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one taskrun cancelled for timeout, one done but not cancelled",
		rpt: ResolvedPipelineTask{
			TaskRuns: []*v1.TaskRun{withCancelledForTimeout(makeFailed(trs[0])), makeSucceeded(trs[1])},
		},
		want: true,
	}, {
		name: "customruns not started",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{},
		},
		want: false,
	}, {
		name: "customrun not done",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
		},
		want: false,
	}, {
		name: "customrun succeeded but not cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{(makeCustomRunSucceeded(customRuns[0]))},
		},
		want: false,
	}, {
		name: "customrun failed but not cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{(makeCustomRunFailed(customRuns[0]))},
		},
		want: false,
	}, {
		name: "customrun cancelled by spec and failed",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelledBySpec(makeCustomRunFailed(customRuns[0]))},
		},
		want: false,
	}, {
		name: "customrun cancelled for timeout and failed",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelledForTimeout(makeCustomRunFailed(customRuns[0]))},
		},
		want: true,
	}, {
		name: "customrun cancelled for timeout but still running",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelledForTimeout(makeCustomRunStarted(customRuns[0]))},
		},
		want: false,
	}, {
		name: "one customrun cancelled for timeout, one not done",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelledForTimeout(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "one customrun cancelled for timeout, one done but not cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask: true,
			CustomRuns: []*v1beta1.CustomRun{withCustomRunCancelledForTimeout(makeCustomRunFailed(customRuns[0])), makeCustomRunSucceeded(customRuns[1])},
		},
		want: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rpt.isCancelledForTimeOut(); got != tc.want {
				t.Errorf("expected isCancelledForTimeOut: %t but got %t", tc.want, got)
			}
		})
	}
}

func TestHaveAnyCustomRunsFailed(t *testing.T) {
	for _, tc := range []struct {
		name string
		rpt  ResolvedPipelineTask
		want bool
	}{{
		name: "run not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "run running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
		},
		want: false,
	}, {
		name: "run succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0])},
		},
		want: false,
	}, {
		name: "run failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
		},
		want: true,
	}, {
		name: "customrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "customrun cancelled for timeout",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelledForTimeout(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "customrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0]))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "matrixed runs not started",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed runs running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunSucceeded(customRuns[1])},
		},
		want: false,
	}, {
		name: "one matrixed run succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed runs failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunFailed(customRuns[1])},
		},
		want: true,
	}, {
		name: "one matrixed run failed, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), withCustomRunCancelled(makeCustomRunFailed(customRuns[1]))},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), withCustomRunCancelled(newCustomRun(customRuns[1]))},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rpt.haveAnyCustomRunsFailed(); got != tc.want {
				t.Errorf("expected haveAnyCustomRunsFailed: %t but got %t", tc.want, got)
			}
		})
	}
}

func TestHaveAnyTaskRunsFailed(t *testing.T) {
	for _, tc := range []struct {
		name string
		rpt  ResolvedPipelineTask
		want bool
	}{{
		name: "taskrun not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0])},
		},
		want: false,
	}, {
		name: "taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
		},
		want: false,
	}, {
		name: "taskrun failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
		},
		want: true,
	}, {
		name: "taskrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled for timeout",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelledForTimeout(makeFailed(trs[0]))},
		},
		want: true,
	}, {
		name: "matrixed taskruns not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed taskruns running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeFailed(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed taskrun failed, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), withCancelled(newTaskRun(trs[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rpt.haveAnyTaskRunsFailed(); got != tc.want {
				t.Errorf("expected haveAnyTaskRunsFailed: %t but got %t", tc.want, got)
			}
		})
	}
}

func TestSkipBecauseParentTaskWasSkipped(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    PipelineRunState
		expected map[string]bool
	}{{
		name: "when-expression-task-but-without-parent-done",
		state: PipelineRunState{{
			// parent task has when expressions but is not yet done
			PipelineTask: &pts[0],
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task not skipped because parent is not yet done
			PipelineTask: &pts[11],
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask12": false,
		},
	}, {
		name: "tasks-when-expressions",
		state: PipelineRunState{{
			// parent task is skipped because when expressions evaluate to false, not because of its parent tasks
			PipelineTask: &pts[10],
			TaskRunNames: []string{"pipelinerun-guardedtask"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task is not skipped regardless of its parent task being skipped due to when expressions evaluating
			// to false, because when expressions are scoped to task only
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-1"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask11": false,
			"mytask18": false,
		},
	}, {
		name: "tasks-when-expressions-run-multiple-dependent-tasks",
		state: PipelineRunState{{
			// parent task is skipped because when expressions evaluate to false, not because of its parent tasks
			PipelineTask: &pts[10],
			TaskRunNames: []string{"pipelinerun-guardedtask"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task is not skipped regardless of its parent task being skipped due to when expressions evaluating
			// to false, because when expressions are scoped to task only
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-1"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task is not skipped regardless of its parent task being skipped due to when expressions evaluating
			// to false, because when expressions are scoped to task only
			PipelineTask: &v1.PipelineTask{
				Name:     "mytask19",
				TaskRef:  &v1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask18"},
			},
			TaskRunNames: []string{"pipelinerun-ordering-dependent-task-2"},
			TaskRuns:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask11": false,
			"mytask18": false,
			"mytask19": false,
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := dagFromState(tc.state)
			if err != nil {
				t.Fatalf("Could not get a dag from the TC state %#v: %v", tc.state, err)
			}
			stateMap := tc.state.ToMap()
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}
			for taskName, isSkipped := range tc.expected {
				rpt := stateMap[taskName]
				if rpt == nil {
					t.Fatalf("Could not get task %s from the state: %v", taskName, tc.state)
				}
				if d := cmp.Diff(isSkipped, rpt.skipBecauseParentTaskWasSkipped(&facts)); d != "" {
					t.Errorf("Didn't get expected isSkipped from task %s: %s", taskName, diff.PrintWantGot(d))
				}
			}
		})
	}
}

func getExpectedMessage(runName string, specStatus v1.PipelineRunSpecStatus, status corev1.ConditionStatus,
	successful, incomplete, skipped, failed, cancelled int) string {
	if status == corev1.ConditionFalse &&
		(specStatus == v1.PipelineRunSpecStatusCancelledRunFinally ||
			specStatus == v1.PipelineRunSpecStatusStoppedRunFinally) {
		return fmt.Sprintf("PipelineRun %q was cancelled", runName)
	}
	if status == corev1.ConditionFalse || status == corev1.ConditionTrue {
		return fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Skipped: %d",
			successful+failed+cancelled, failed, cancelled, skipped)
	}
	return fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Incomplete: %d, Skipped: %d",
		successful+failed+cancelled, failed, cancelled, incomplete, skipped)
}

func TestResolvePipelineRun_CustomTask(t *testing.T) {
	names.TestingSeed()
	pts := []v1.PipelineTask{{
		Name:    "customtask",
		TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
	}, {
		Name: "customtask-spec",
		TaskSpec: &v1.EmbeddedTask{
			TypeMeta: runtime.TypeMeta{
				APIVersion: "example.dev/v0",
				Kind:       "Example",
			},
		},
	}, {
		Name:    "run-exists",
		TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
	}}
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
	}
	run := &v1beta1.CustomRun{ObjectMeta: metav1.ObjectMeta{Name: "run-exists-abcde"}}
	getRun := func(name string) (*v1beta1.CustomRun, error) {
		if name == "pipelinerun-run-exists" {
			return run, nil
		}
		return nil, kerrors.NewNotFound(v1.Resource("run"), name)
	}
	pipelineState := PipelineRunState{}
	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	ctx = cfg.ToContext(ctx)
	for _, task := range pts {
		ps, err := ResolvePipelineTask(ctx, pr, nopGetTask, nopGetTaskRun, getRun, task, nil)
		if err != nil {
			t.Fatalf("ResolvePipelineTask: %v", err)
		}
		pipelineState = append(pipelineState, ps)
	}

	expectedState := PipelineRunState{{
		PipelineTask:   &pts[0],
		CustomTask:     true,
		CustomRunNames: []string{"pipelinerun-customtask"},
		CustomRuns:     nil,
	}, {
		PipelineTask:   &pts[1],
		CustomTask:     true,
		CustomRunNames: []string{"pipelinerun-customtask-spec"},
		CustomRuns:     nil,
	}, {
		PipelineTask:   &pts[2],
		CustomTask:     true,
		CustomRunNames: []string{"pipelinerun-run-exists"},
		CustomRuns:     []*v1beta1.CustomRun{run},
	}}
	if d := cmp.Diff(expectedState, pipelineState); d != "" {
		t.Errorf("Unexpected pipeline state: %s", diff.PrintWantGot(d))
	}
}

func TestResolvePipelineRun_PipelineTaskHasNoResources(t *testing.T) {
	pts := []v1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask2",
		TaskRef: &v1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask3",
		TaskRef: &v1.TaskRef{Name: "task"},
	}}

	getTask := func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, nil, nil, nil
	}
	getTaskRun := func(name string) (*v1.TaskRun, error) { return &trs[0], nil }
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	pipelineState := PipelineRunState{}
	for _, task := range pts {
		ps, err := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetCustomRun, task, nil)
		if err != nil {
			t.Errorf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
		}
		pipelineState = append(pipelineState, ps)
	}
	if len(pipelineState) != 3 {
		t.Fatalf("Expected only 2 resolved PipelineTasks but got %d", len(pipelineState))
	}
	expectedTask := &resources.ResolvedTask{
		TaskName: task.Name,
		TaskSpec: &task.Spec,
	}
	if d := cmp.Diff(pipelineState[0].ResolvedTask, expectedTask, cmpopts.IgnoreUnexported(v1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but actual differed %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(pipelineState[1].ResolvedTask, expectedTask, cmpopts.IgnoreUnexported(v1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but actual differed %s", diff.PrintWantGot(d))
	}
}

func TestResolvePipelineRun_TaskDoesntExist(t *testing.T) {
	pts := []v1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask2",
		TaskRef: &v1.TaskRef{Name: "task"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "foo",
				Value: *v1.NewStructuredValues("f", "o", "o"),
			}, {
				Name:  "bar",
				Value: *v1.NewStructuredValues("b", "a", "r"),
			}},
		}}}

	// Return an error when the Task is retrieved, as if it didn't exist
	getTask := func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return nil, nil, nil, kerrors.NewNotFound(v1.Resource("task"), name)
	}
	getTaskRun := func(name string) (*v1.TaskRun, error) {
		return nil, kerrors.NewNotFound(v1.Resource("taskrun"), name)
	}
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	for _, pt := range pts {
		_, err := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetCustomRun, pt, nil)
		var tnf *TaskNotFoundError
		switch {
		case err == nil:
			t.Fatalf("Pipeline %s: want error, got nil", p.Name)
		case errors.As(err, &tnf):
			// expected error
		default:
			t.Fatalf("Pipeline %s: Want %T, got %s of type %T", p.Name, tnf, err, err)
		}
	}
}

func TestResolvePipelineRun_VerificationFailed(t *testing.T) {
	pts := []v1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask2",
		TaskRef: &v1.TaskRef{Name: "task"},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "foo",
				Value: *v1.NewStructuredValues("f", "o", "o"),
			}, {
				Name:  "bar",
				Value: *v1.NewStructuredValues("b", "a", "r"),
			}},
		}}}
	verificationResult := &trustedresources.VerificationResult{VerificationResultType: trustedresources.VerificationError, Err: trustedresources.ErrResourceVerificationFailed}
	getTask := func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, nil, verificationResult, nil
	}
	getTaskRun := func(name string) (*v1.TaskRun, error) { return nil, nil } //nolint:nilnil
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	for _, pt := range pts {
		rt, _ := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetCustomRun, pt, nil)
		if d := cmp.Diff(verificationResult, rt.ResolvedTask.VerificationResult, cmpopts.EquateErrors()); d != "" {
			t.Errorf(diff.PrintWantGot(d))
		}
	}
}

func TestValidateWorkspaceBindingsWithValidWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *v1.PipelineSpec
		pr   *v1.PipelineRun
		err  string
	}{{
		name: "include required workspace",
		spec: &v1.PipelineSpec{
			Workspaces: []v1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
		},
		pr: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				Workspaces: []v1.WorkspaceBinding{{
					Name:     "foo",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
	}, {
		name: "omit optional workspace",
		spec: &v1.PipelineSpec{
			Workspaces: []v1.PipelineWorkspaceDeclaration{{
				Name:     "foo",
				Optional: true,
			}},
		},
		pr: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				Workspaces: []v1.WorkspaceBinding{},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateWorkspaceBindings(tc.spec, tc.pr); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidateWorkspaceBindingsWithInvalidWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *v1.PipelineSpec
		pr   *v1.PipelineRun
		err  string
	}{{
		name: "missing required workspace",
		spec: &v1.PipelineSpec{
			Workspaces: []v1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
		},
		pr: &v1.PipelineRun{
			Spec: v1.PipelineRunSpec{
				Workspaces: []v1.WorkspaceBinding{},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateWorkspaceBindings(tc.spec, tc.pr); err == nil {
				t.Fatalf("Expected error indicating `foo` workspace was not provided but got no error")
			}
		})
	}
}

func TestValidateTaskRunSpecs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		p       *v1.Pipeline
		pr      *v1.PipelineRun
		wantErr bool
	}{{
		name: "valid task mapping",
		p: &v1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelines",
			},
			Spec: v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1.TaskRef{
						Name: "task",
					},
				}},
			},
		},
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun",
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "pipeline",
				},
				TaskRunSpecs: []v1.PipelineTaskRunSpec{{
					PipelineTaskName:   "mytask1",
					ServiceAccountName: "default",
				}},
			},
		},
		wantErr: false,
	}, {
		name: "valid finally task mapping",
		p: &v1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelines",
			},
			Spec: v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1.TaskRef{
						Name: "task",
					},
				}},
				Finally: []v1.PipelineTask{{
					Name: "myfinaltask1",
					TaskRef: &v1.TaskRef{
						Name: "finaltask",
					},
				}},
			},
		},
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun",
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "pipeline",
				},
				TaskRunSpecs: []v1.PipelineTaskRunSpec{{
					PipelineTaskName:   "myfinaltask1",
					ServiceAccountName: "default",
				}},
			},
		},
		wantErr: false,
	}, {
		name: "invalid task mapping",
		p: &v1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelines",
			},
			Spec: v1.PipelineSpec{
				Tasks: []v1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1.TaskRef{
						Name: "task",
					},
				}},
				Finally: []v1.PipelineTask{{
					Name: "myfinaltask1",
					TaskRef: &v1.TaskRef{
						Name: "finaltask",
					},
				}},
			},
		},
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun",
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: "pipeline",
				},
				TaskRunSpecs: []v1.PipelineTaskRunSpec{{
					PipelineTaskName:   "wrongtask",
					ServiceAccountName: "default",
				}},
			},
		},
		wantErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.p.Spec
			err := ValidateTaskRunSpecs(&spec, tc.pr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Did not get error when it was expected for test: %s", tc.name)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error when no error expected: %v", err)
				}
			}
		})
	}
}

func TestResolvePipeline_WhenExpressions(t *testing.T) {
	names.TestingSeed()
	tName1 := "pipelinerun-mytask1-always-true"

	t1 := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: tName1,
		},
	}

	ptwe1 := v1.WhenExpression{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo"},
	}

	pt := v1.PipelineTask{
		Name:    "mytask1",
		TaskRef: &v1.TaskRef{Name: "task"},
		When:    []v1.WhenExpression{ptwe1},
	}

	getTask := func(_ context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, nil, nil, nil
	}
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	getTaskRun := func(name string) (*v1.TaskRun, error) {
		switch name {
		case "pipelinerun-mytask1-always-true-0":
			return t1, nil
		case "pipelinerun-mytask1":
			return &trs[0], nil
		}
		return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
	}

	t.Run("When Expressions exist", func(t *testing.T) {
		_, err := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetCustomRun, pt, nil)
		if err != nil {
			t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
		}
	})
}

func TestIsCustomTask(t *testing.T) {
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	getTask := func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, nil, nil, nil
	}
	getTaskRun := func(name string) (*v1.TaskRun, error) { return nil, nil }    //nolint:nilnil
	getRun := func(name string) (*v1beta1.CustomRun, error) { return nil, nil } //nolint:nilnil

	for _, tc := range []struct {
		name string
		pt   v1.PipelineTask
		want bool
	}{{
		name: "custom taskSpec",
		pt: v1.PipelineTask{
			TaskSpec: &v1.EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "example.dev/v0",
					Kind:       "Sample",
				},
			},
		},
		want: true,
	}, {
		name: "custom taskSpec missing kind",
		pt: v1.PipelineTask{
			TaskSpec: &v1.EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "example.dev/v0",
					Kind:       "",
				},
			},
		},
		want: false,
	}, {
		name: "custom taskSpec missing apiVersion",
		pt: v1.PipelineTask{
			TaskSpec: &v1.EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "",
					Kind:       "Sample",
				},
			},
		},
		want: false,
	}, {
		name: "custom taskRef",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Sample",
			},
		},
		want: true,
	}, {
		name: "custom taskRef missing kind",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "",
			},
		},
		want: false,
	}, {
		name: "custom taskRef missing apiVersion",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				APIVersion: "",
				Kind:       "Sample",
			},
		},
		want: false,
	}, {
		name: "non-custom taskref",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				Name: "task",
			},
		},
		want: false,
	}, {
		name: "non-custom taskspec",
		pt: v1.PipelineTask{
			TaskSpec: &v1.EmbeddedTask{},
		},
		want: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.NewStore(logtesting.TestLogger(t))
			ctx = cfg.ToContext(ctx)
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt, nil)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
			}
			got := rpt.IsCustomTask()
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("IsCustomTask: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolvedPipelineRunTask_IsFinallySkipped(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dag-task",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "commit",
					Value: *v1.NewStructuredValues("SHA2"),
				}},
			},
		},
	}

	state := PipelineRunState{{
		TaskRunNames: []string{"dag-task"},
		TaskRuns:     []*v1.TaskRun{tr},
		PipelineTask: &v1.PipelineTask{
			Name:    "dag-task",
			TaskRef: &v1.TaskRef{Name: "task"},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task-1",
			TaskRef: &v1.TaskRef{Name: "task"},
			Params: v1.Params{{
				Name:  "commit",
				Value: *v1.NewStructuredValues("$(tasks.dag-task.results.commit)"),
			}},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task-2",
			TaskRef: &v1.TaskRef{Name: "task"},
			Params: v1.Params{{
				Name:  "commit",
				Value: *v1.NewStructuredValues("$(tasks.dag-task.results.missingResult)"),
			}},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task-3",
			TaskRef: &v1.TaskRef{Name: "task"},
			When: v1.WhenExpressions{{
				Input:    "foo",
				Operator: selection.NotIn,
				Values:   []string{"bar"},
			}},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task-4",
			TaskRef: &v1.TaskRef{Name: "task"},
			When: v1.WhenExpressions{{
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"bar"},
			}},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task-5",
			TaskRef: &v1.TaskRef{Name: "task"},
			When: v1.WhenExpressions{{
				Input:    "$(tasks.dag-task.results.commit)",
				Operator: selection.In,
				Values:   []string{"SHA2"},
			}},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task-6",
			TaskRef: &v1.TaskRef{Name: "task"},
			When: v1.WhenExpressions{{
				Input:    "$(tasks.dag-task.results.missing)",
				Operator: selection.In,
				Values:   []string{"none"},
			}},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task-7",
			TaskRef: &v1.TaskRef{Name: "task"},
			Matrix: &v1.Matrix{
				Params: v1.Params{{
					Name:  "platform",
					Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{}},
				}}},
		},
	}}

	testCases := []struct {
		name             string
		startTime        time.Time
		finallyStartTime time.Time
		finallyTimeout   time.Duration
		pipelineTimeout  time.Duration
		expected         map[string]bool
	}{
		{
			name: "no finally timeout",
			expected: map[string]bool{
				"final-task-1": false,
				"final-task-2": true,
				"final-task-3": false,
				"final-task-4": true,
				"final-task-5": false,
				"final-task-6": true,
				"final-task-7": true,
			},
		}, {
			name:             "finally timeout not yet reached",
			finallyStartTime: now.Add(-5 * time.Minute),
			finallyTimeout:   10 * time.Minute,
			expected: map[string]bool{
				"final-task-1": false,
				"final-task-2": true,
				"final-task-3": false,
				"final-task-4": true,
				"final-task-5": false,
				"final-task-6": true,
				"final-task-7": true,
			},
		}, {
			name:            "pipeline timeout not yet reached",
			startTime:       now.Add(-5 * time.Minute),
			pipelineTimeout: 10 * time.Minute,
			expected: map[string]bool{
				"final-task-1": false,
				"final-task-2": true,
				"final-task-3": false,
				"final-task-4": true,
				"final-task-5": false,
				"final-task-6": true,
				"final-task-7": true,
			},
		}, {
			name:             "finally timeout passed",
			finallyStartTime: now.Add(-5 * time.Minute),
			finallyTimeout:   4 * time.Minute,
			expected: map[string]bool{
				"final-task-1": true,
				"final-task-2": true,
				"final-task-3": true,
				"final-task-4": true,
				"final-task-5": true,
				"final-task-6": true,
				"final-task-7": true,
			},
		}, {
			name:            "pipeline timeout passed",
			startTime:       now.Add(-5 * time.Minute),
			pipelineTimeout: 4 * time.Minute,
			expected: map[string]bool{
				"final-task-1": true,
				"final-task-2": true,
				"final-task-3": true,
				"final-task-4": true,
				"final-task-5": true,
				"final-task-6": true,
				"final-task-7": true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tasks := v1.PipelineTaskList([]v1.PipelineTask{*state[0].PipelineTask})
			d, err := dag.Build(tasks, tasks.Deps())
			if err != nil {
				t.Fatalf("Could not get a dag from the dag tasks %#v: %v", state[0], err)
			}

			// build graph with finally tasks
			var pts []v1.PipelineTask
			for i := range state {
				if i > 0 { // first one is a dag task that produces a result
					pts = append(pts, *state[i].PipelineTask)
				}
			}
			dfinally, err := dag.Build(v1.PipelineTaskList(pts), map[string][]string{})
			if err != nil {
				t.Fatalf("Could not get a dag from the finally tasks %#v: %v", pts, err)
			}

			finallyStartTime := tc.finallyStartTime
			if finallyStartTime.IsZero() {
				finallyStartTime = now
			}
			prStartTime := tc.startTime
			if prStartTime.IsZero() {
				prStartTime = now
			}
			facts := &PipelineRunFacts{
				State:           state,
				TasksGraph:      d,
				FinalTasksGraph: dfinally,
				TimeoutsState: PipelineRunTimeoutsState{
					StartTime:        &prStartTime,
					FinallyStartTime: &finallyStartTime,
					Clock:            testClock,
				},
			}
			if tc.finallyTimeout != 0 {
				facts.TimeoutsState.FinallyTimeout = &tc.finallyTimeout
			}
			if tc.pipelineTimeout != 0 {
				facts.TimeoutsState.PipelineTimeout = &tc.pipelineTimeout
			}

			for i := range state {
				if i > 0 { // first one is a dag task that produces a result
					finallyTaskName := state[i].PipelineTask.Name
					if d := cmp.Diff(tc.expected[finallyTaskName], state[i].IsFinallySkipped(facts).IsSkipped); d != "" {
						t.Fatalf("Didn't get expected isFinallySkipped from finally task %s: %s", finallyTaskName, diff.PrintWantGot(d))
					}
				}
			}
		})
	}
}

func TestResolvedPipelineRunTask_IsFinallySkippedByCondition(t *testing.T) {
	task := &ResolvedPipelineTask{
		TaskRunNames: []string{"dag-task"},
		TaskRuns: []*v1.TaskRun{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dag-task",
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		}},
		PipelineTask: &v1.PipelineTask{
			Name:    "dag-task",
			TaskRef: &v1.TaskRef{Name: "task"},
		},
	}
	for _, tc := range []struct {
		name  string
		state PipelineRunState
		want  TaskSkipStatus
	}{{
		name: "task started",
		state: PipelineRunState{
			task,
			{
				TaskRuns: []*v1.TaskRun{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "final-task",
					},
					Status: v1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: []apis.Condition{{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionUnknown,
							}},
						},
					},
				}},
				PipelineTask: &v1.PipelineTask{
					Name:    "final-task",
					TaskRef: &v1.TaskRef{Name: "task"},
					When: []v1.WhenExpression{
						{
							Input:    "$(tasks.dag-task.status)",
							Operator: selection.In,
							Values:   []string{"Succeeded"},
						},
					},
				},
			},
		},
		want: TaskSkipStatus{
			IsSkipped:      false,
			SkippingReason: v1.None,
		},
	}, {
		name: "task scheduled",
		state: PipelineRunState{
			task,
			{
				TaskRunNames: []string{"final-task"},
				TaskRuns: []*v1.TaskRun{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "final-task",
					},
					Status: v1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: []apis.Condition{ /* explicitly empty */ },
						},
					},
				}},
				PipelineTask: &v1.PipelineTask{
					Name:    "final-task",
					TaskRef: &v1.TaskRef{Name: "task"},
					When: []v1.WhenExpression{
						{
							Input:    "$(tasks.dag-task.status)",
							Operator: selection.In,
							Values:   []string{"Succeeded"},
						},
					},
				},
			}},
		want: TaskSkipStatus{
			IsSkipped:      false,
			SkippingReason: v1.None,
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			tasks := v1.PipelineTaskList([]v1.PipelineTask{*tc.state[0].PipelineTask})
			d, err := dag.Build(tasks, tasks.Deps())
			if err != nil {
				t.Fatalf("Could not get a dag from the dag tasks %#v: %v", tc.state[0], err)
			}

			// build graph with finally tasks
			var pts v1.PipelineTaskList
			for _, state := range tc.state[1:] {
				pts = append(pts, *state.PipelineTask)
			}
			dfinally, err := dag.Build(pts, map[string][]string{})
			if err != nil {
				t.Fatalf("Could not get a dag from the finally tasks %#v: %v", pts, err)
			}

			facts := &PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: dfinally,
				TimeoutsState: PipelineRunTimeoutsState{
					Clock: testClock,
				},
			}

			for _, state := range tc.state[1:] {
				got := state.IsFinallySkipped(facts)
				if d := cmp.Diff(tc.want, got); d != "" {
					t.Errorf("IsFinallySkipped: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestResolvedPipelineRunTask_IsFinalTask(t *testing.T) {
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dag-task",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "commit",
					Value: *v1.NewStructuredValues("SHA2"),
				}},
			},
		},
	}

	state := PipelineRunState{{
		TaskRunNames: []string{"dag-task"},
		TaskRuns:     []*v1.TaskRun{tr},
		PipelineTask: &v1.PipelineTask{
			Name:    "dag-task",
			TaskRef: &v1.TaskRef{Name: "task"},
		},
	}, {
		PipelineTask: &v1.PipelineTask{
			Name:    "final-task",
			TaskRef: &v1.TaskRef{Name: "task"},
			Params: v1.Params{{
				Name:  "commit",
				Value: *v1.NewStructuredValues("$(tasks.dag-task.results.commit)"),
			}},
		},
	},
	}

	tasks := v1.PipelineTaskList([]v1.PipelineTask{*state[0].PipelineTask})
	d, err := dag.Build(tasks, tasks.Deps())
	if err != nil {
		t.Fatalf("Could not get a dag from the dag tasks %#v: %v", state[0], err)
	}

	// build graph with finally tasks
	pts := []v1.PipelineTask{*state[1].PipelineTask}

	dfinally, err := dag.Build(v1.PipelineTaskList(pts), map[string][]string{})
	if err != nil {
		t.Fatalf("Could not get a dag from the finally tasks %#v: %v", pts, err)
	}

	facts := &PipelineRunFacts{
		State:           state,
		TasksGraph:      d,
		FinalTasksGraph: dfinally,
		TimeoutsState: PipelineRunTimeoutsState{
			Clock: testClock,
		},
	}

	finallyTaskName := state[1].PipelineTask.Name
	if d := cmp.Diff(true, state[1].IsFinalTask(facts)); d != "" {
		t.Fatalf("Didn't get expected isFinallySkipped from finally task %s: %s", finallyTaskName, diff.PrintWantGot(d))
	}
}

func TestGetTaskRunName(t *testing.T) {
	prName := "pipeline-run"

	childRefs := []v1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
		Name:             "taskrun-for-task1",
		PipelineTaskName: "task1",
	}}

	for _, tc := range []struct {
		name       string
		ptName     string
		prName     string
		wantTrName string
	}{{
		name:       "existing taskrun",
		ptName:     "task1",
		wantTrName: "taskrun-for-task1",
	}, {
		name:       "new taskrun",
		ptName:     "task2",
		wantTrName: "pipeline-run-task2",
	}, {
		name:       "new taskrun with long name",
		ptName:     "task2-0123456789-0123456789-0123456789-0123456789-0123456789",
		wantTrName: "pipeline-runee4a397d6eab67777d4e6f9991cd19e6-task2-0123456789-0",
	}, {
		name:       "new taskrun, pr with long name",
		ptName:     "task3",
		prName:     "pipeline-run-0123456789-0123456789-0123456789-0123456789",
		wantTrName: "pipeline-run-0123456789-0123456789-0123456789-0123456789-task3",
	}, {
		name:       "new taskrun, taskrun and pr with long name",
		ptName:     "task2-0123456789-0123456789-0123456789-0123456789-0123456789",
		prName:     "pipeline-run-0123456789-0123456789-0123456789-0123456789",
		wantTrName: "pipeline-run-0123456789-012345607ad8c7aac5873cdfabe472a68996b5c",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testPrName := prName
			if tc.prName != "" {
				testPrName = tc.prName
			}
			trNameFromChildRefs := GetTaskRunName(childRefs, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, trNameFromChildRefs); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetNamesOfTaskRuns(t *testing.T) {
	prName := "mypipelinerun"
	childRefs := []v1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
		Name:             "mypipelinerun-mytask-0",
		PipelineTaskName: "mytask",
	}, {
		TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
		Name:             "mypipelinerun-mytask-1",
		PipelineTaskName: "mytask",
	}}

	for _, tc := range []struct {
		name        string
		ptName      string
		prName      string
		wantTrNames []string
	}{{
		name:        "existing taskruns",
		ptName:      "mytask",
		wantTrNames: []string{"mypipelinerun-mytask-0", "mypipelinerun-mytask-1"},
	}, {
		name:        "new taskruns",
		ptName:      "mynewtask",
		wantTrNames: []string{"mypipelinerun-mynewtask-0", "mypipelinerun-mynewtask-1"},
	}, {
		name:   "new pipelinetask with long names",
		ptName: "longtask-0123456789-0123456789-0123456789-0123456789-0123456789",
		wantTrNames: []string{
			"mypipelinerun09c563f6b29a3a2c16b98e6dc95979c5-longtask-01234567",
			"mypipelinerunab643c1924b632f050e5a07fe482fc25-longtask-01234567",
		},
	}, {
		name:   "new taskruns, pipelinerun with long name",
		ptName: "task3",
		prName: "pipeline-run-0123456789-0123456789-0123456789-0123456789",
		wantTrNames: []string{
			"pipeline-run-01234567891276ed292277c9bebded38d907a517fe-task3-0",
			"pipeline-run-01234567891276ed292277c9bebded38d907a517fe-task3-1",
		},
	}, {
		name:   "new taskruns, pipelinetask and pipelinerun with long name",
		ptName: "task2-0123456789-0123456789-0123456789-0123456789-0123456789",
		prName: "pipeline-run-0123456789-0123456789-0123456789-0123456789",
		wantTrNames: []string{
			"pipeline-run-0123456789-01234563c0313c59d28c85a2c2b3fd3b17a9514",
			"pipeline-run-0123456789-01234569d54677e88e96776942290e00b578ca5",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testPrName := prName
			if tc.prName != "" {
				testPrName = tc.prName
			}
			namesOfTaskRunsFromChildRefs := GetNamesOfTaskRuns(childRefs, tc.ptName, testPrName, 2)
			sort.Strings(namesOfTaskRunsFromChildRefs)
			if d := cmp.Diff(tc.wantTrNames, namesOfTaskRunsFromChildRefs); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetNamesOfRuns(t *testing.T) {
	prName := "mypipelinerun"
	childRefs := []v1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "Run"},
		Name:             "mypipelinerun-mytask-0",
		PipelineTaskName: "mytask",
	}, {
		TypeMeta:         runtime.TypeMeta{Kind: "Run"},
		Name:             "mypipelinerun-mytask-1",
		PipelineTaskName: "mytask",
	}}

	for _, tc := range []struct {
		name         string
		ptName       string
		prName       string
		wantRunNames []string
	}{{
		name:         "existing runs",
		ptName:       "mytask",
		wantRunNames: []string{"mypipelinerun-mytask-0", "mypipelinerun-mytask-1"},
	}, {
		name:         "new runs",
		ptName:       "mynewtask",
		wantRunNames: []string{"mypipelinerun-mynewtask-0", "mypipelinerun-mynewtask-1"},
	}, {
		name:   "new pipelinetask with long names",
		ptName: "longtask-0123456789-0123456789-0123456789-0123456789-0123456789",
		wantRunNames: []string{
			"mypipelinerun09c563f6b29a3a2c16b98e6dc95979c5-longtask-01234567",
			"mypipelinerunab643c1924b632f050e5a07fe482fc25-longtask-01234567",
		},
	}, {
		name:   "new runs, pipelinerun with long name",
		ptName: "task3",
		prName: "pipeline-run-0123456789-0123456789-0123456789-0123456789",
		wantRunNames: []string{
			"pipeline-run-01234567891276ed292277c9bebded38d907a517fe-task3-0",
			"pipeline-run-01234567891276ed292277c9bebded38d907a517fe-task3-1",
		},
	}, {
		name:   "new runs, pipelinetask and pipelinerun with long name",
		ptName: "task2-0123456789-0123456789-0123456789-0123456789-0123456789",
		prName: "pipeline-run-0123456789-0123456789-0123456789-0123456789",
		wantRunNames: []string{
			"pipeline-run-0123456789-01234563c0313c59d28c85a2c2b3fd3b17a9514",
			"pipeline-run-0123456789-01234569d54677e88e96776942290e00b578ca5",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testPrName := prName
			if tc.prName != "" {
				testPrName = tc.prName
			}
			namesOfRunsFromChildRefs := getNamesOfCustomRuns(childRefs, tc.ptName, testPrName, 2)
			sort.Strings(namesOfRunsFromChildRefs)
			if d := cmp.Diff(tc.wantRunNames, namesOfRunsFromChildRefs); d != "" {
				t.Errorf("getRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetRunName(t *testing.T) {
	prName := "pipeline-run"
	childRefs := []v1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "CustomRun"},
		Name:             "run-for-task1",
		PipelineTaskName: "task1",
	}}

	for _, tc := range []struct {
		name       string
		ptName     string
		prName     string
		wantTrName string
	}{{
		name:       "existing run",
		ptName:     "task1",
		wantTrName: "run-for-task1",
	}, {
		name:       "new run",
		ptName:     "task2",
		wantTrName: "pipeline-run-task2",
	}, {
		name:       "new run with long name",
		ptName:     "task2-12345678901234567890123456789012345678901234567890",
		wantTrName: "pipeline-run155b61773fd25b1fbd46dba34cd7cbeb-task2-123456789012",
	}, {
		name:       "new run, pr with long name",
		ptName:     "task2",
		prName:     "pipeline-run-12345678901234567890123456789012345678901234567890",
		wantTrName: "pipeline-run-123456789012f022e3ec9d06c5795de718d2f11bdd71-task2",
	}, {
		name:       "new run, run and pr with long name",
		ptName:     "task2-12345678901234567890123456789012345678901234567890",
		prName:     "pipeline-run-1234567890123456789012345678901234567890",
		wantTrName: "pipeline-run-1234567890123456782725b0120d3afbb59a451b67da5eb51c",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testPrName := prName
			if tc.prName != "" {
				testPrName = tc.prName
			}
			rnFromChildRefs := getCustomRunName(childRefs, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, rnFromChildRefs); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
			rnFromBoth := getCustomRunName(childRefs, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, rnFromBoth); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestIsMatrixed(t *testing.T) {
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	getTask := func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, nil, nil, nil
	}
	getTaskRun := func(name string) (*v1.TaskRun, error) { return &trs[0], nil }
	getRun := func(name string) (*v1beta1.CustomRun, error) { return &customRuns[0], nil }

	for _, tc := range []struct {
		name string
		pt   v1.PipelineTask
		want bool
	}{{
		name: "custom task with matrix",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Sample",
			},
			Matrix: &v1.Matrix{
				Params: v1.Params{{
					Name:  "platform",
					Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
				}}},
		},
		want: true,
	}, {
		name: "custom task without matrix",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Sample",
			},
		},
		want: false,
	}, {
		name: "task with matrix",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				Name: "my-task",
			},
			Matrix: &v1.Matrix{
				Params: v1.Params{{
					Name:  "platform",
					Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
				}}},
		},
		want: true,
	}, {
		name: "task without matrix",
		pt: v1.PipelineTask{
			TaskRef: &v1.TaskRef{
				Name: "my-task",
			},
		},
		want: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.NewStore(logtesting.TestLogger(t))
			cfg.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
				Data: map[string]string{
					"enable-api-fields": "alpha",
				},
			})
			ctx = cfg.ToContext(ctx)
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt, nil)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
			}
			got := rpt.PipelineTask.IsMatrixed()
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("IsMatrixed: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolvePipelineRunTask_WithMatrix(t *testing.T) {
	pipelineRunName := "pipelinerun"
	pipelineTaskName := "pipelinetask"

	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineRunName,
		},
	}

	var taskRuns []*v1.TaskRun
	var taskRunsNames []string
	taskRunsMap := map[string]*v1.TaskRun{}
	for i := 0; i < 9; i++ {
		trName := fmt.Sprintf("%s-%s-%d", pipelineRunName, pipelineTaskName, i)
		tr := &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: trName,
			},
		}
		taskRuns = append(taskRuns, tr)
		taskRunsNames = append(taskRunsNames, trName)
		taskRunsMap[trName] = tr
	}

	pts := []v1.PipelineTask{{
		Name: "pipelinetask",
		TaskRef: &v1.TaskRef{
			Name: "my-task",
		},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}}},
	}, {
		Name: "pipelinetask",
		TaskRef: &v1.TaskRef{
			Name: "my-task",
		},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}, {
				Name:  "browsers",
				Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"chrome", "safari", "firefox"}},
			}},
		},
	}, {
		Name: "pipelinetask-with-whole-array-results",
		TaskRef: &v1.TaskRef{
			Name: "pipelinetask-with-whole-array-results",
		},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "platforms", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "$(tasks.get-platforms.results.platforms[*])"},
			}},
		},
	}}

	rtr := &resources.ResolvedTask{
		TaskName: "task",
		TaskSpec: &v1.TaskSpec{Steps: []v1.Step{{
			Name: "step1",
		}}},
	}

	var pipelineRunState = PipelineRunState{{
		TaskRunNames: []string{"get-platforms"},
		TaskRuns: []*v1.TaskRun{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "get-platforms",
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{successCondition},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{{
						Name:  "platforms",
						Value: *v1.NewStructuredValues("linux", "mac", "windows"),
					}},
				},
			},
		}},
		PipelineTask: &v1.PipelineTask{
			Name:    "pipelinetask-with-whole-array-results",
			TaskRef: &v1.TaskRef{Name: "pipelinetask-with-whole-array-results"},
			Params: v1.Params{{
				Name:  "platforms",
				Value: *v1.NewStructuredValues("$(tasks.get-platforms.results.platforms[*])"),
			}},
		},
	}}

	getTask := func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, nil, nil, nil
	}
	getTaskRun := func(name string) (*v1.TaskRun, error) { return taskRunsMap[name], nil }
	getRun := func(name string) (*v1beta1.CustomRun, error) { return &customRuns[0], nil }

	for _, tc := range []struct {
		name string
		pt   v1.PipelineTask
		want *ResolvedPipelineTask
		pst  PipelineRunState
	}{{
		name: "task with matrix - single parameter",
		pt:   pts[0],
		want: &ResolvedPipelineTask{
			TaskRunNames: taskRunsNames[:3],
			TaskRuns:     taskRuns[:3],
			PipelineTask: &pts[0],
			ResolvedTask: rtr,
		},
	}, {
		name: "task with matrix - multiple parameters",
		pt:   pts[1],
		want: &ResolvedPipelineTask{
			TaskRunNames: taskRunsNames,
			TaskRuns:     taskRuns,
			PipelineTask: &pts[1],
			ResolvedTask: rtr,
		},
	}, {
		name: "task with matrix - whole array results",
		pt:   pts[2],
		want: &ResolvedPipelineTask{
			TaskRunNames: nil,
			TaskRuns:     nil,
			PipelineTask: &pts[2],
			ResolvedTask: nil,
		},
		pst: pipelineRunState,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.NewStore(logtesting.TestLogger(t))
			cfg.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
				Data: map[string]string{
					"enable-api-fields": "alpha",
				},
			})
			ctx = cfg.ToContext(ctx)
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt, tc.pst)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
			}
			if d := cmp.Diff(tc.want, rpt); d != "" {
				t.Errorf("Did not get expected ResolvePipelineTask with Matrix: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolvePipelineRunTask_WithMatrixedCustomTask(t *testing.T) {
	pipelineRunName := "pipelinerun"
	pipelineTaskName := "pipelinetask"

	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineRunName,
		},
	}

	var runs []*v1beta1.CustomRun
	var runNames []string
	runsMap := map[string]*v1beta1.CustomRun{}
	for i := 0; i < 9; i++ {
		runName := fmt.Sprintf("%s-%s-%d", pipelineRunName, pipelineTaskName, i)
		run := &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: runName,
			},
		}
		runs = append(runs, run)
		runNames = append(runNames, runName)
		runsMap[runName] = run
	}

	pts := []v1.PipelineTask{{
		Name: "pipelinetask",
		TaskRef: &v1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
			Name:       "my-task",
		},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}}},
	}, {
		Name: "pipelinetask",
		TaskRef: &v1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
			Name:       "my-task",
		},
		Matrix: &v1.Matrix{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}, {
				Name:  "browsers",
				Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"chrome", "safari", "firefox"}},
			}}},
	}, {
		Name:    "customTask-with-whole-array-results",
		TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "aTask"},
		Params: v1.Params{{
			Name:  "platforms",
			Value: *v1.NewStructuredValues("$(tasks.get-platforms.results.platforms[*])"),
		}},
	}}

	var pipelineRunState = PipelineRunState{{
		TaskRunNames: []string{"get-platforms"},
		TaskRuns: []*v1.TaskRun{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "get-platforms",
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{successCondition},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{{
						Name:  "platforms",
						Value: *v1.NewStructuredValues("linux", "mac", "windows"),
					}},
				},
			},
		}},
		PipelineTask: &v1.PipelineTask{
			Name:    "customTask-with-whole-array-results",
			TaskRef: &v1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "aTask"},
			Params: v1.Params{{
				Name:  "platforms",
				Value: *v1.NewStructuredValues("$(tasks.get-platforms.results.platforms[*])"),
			}},
		},
	}}

	getTask := func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
		return task, nil, nil, nil
	}
	getTaskRun := func(name string) (*v1.TaskRun, error) { return &trs[0], nil }
	getRun := func(name string) (*v1beta1.CustomRun, error) { return runsMap[name], nil }

	for _, tc := range []struct {
		name   string
		pt     v1.PipelineTask
		getRun GetRun
		want   *ResolvedPipelineTask
		pst    PipelineRunState
	}{{
		name: "custom task with matrix - single parameter",
		pt:   pts[0],
		want: &ResolvedPipelineTask{
			CustomTask:     true,
			CustomRunNames: runNames[:3],
			CustomRuns:     runs[:3],
			PipelineTask:   &pts[0],
		},
	}, {
		name: "custom task with matrix - multiple parameters",
		pt:   pts[1],
		want: &ResolvedPipelineTask{
			CustomTask:     true,
			CustomRunNames: runNames,
			CustomRuns:     runs,
			PipelineTask:   &pts[1],
		},
	}, {
		name: "custom task with matrix - nil run",
		pt:   pts[1],
		getRun: func(name string) (*v1beta1.CustomRun, error) {
			return nil, kerrors.NewNotFound(v1.Resource("run"), name)
		},
		want: &ResolvedPipelineTask{
			CustomTask:     true,
			CustomRunNames: runNames,
			CustomRuns:     nil,
			PipelineTask:   &pts[1],
		},
	}, {
		name: "custom task with matrix - whole array results",
		pt:   pts[2],
		want: &ResolvedPipelineTask{
			CustomTask:     true,
			CustomRunNames: []string{"pipelinerun-customTask-with-whole-array-results"},
			CustomRuns:     nil,
			PipelineTask:   &pts[2],
		},
		pst: pipelineRunState,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.NewStore(logtesting.TestLogger(t))
			cfg.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
				Data: map[string]string{
					"enable-api-fields": "alpha",
				},
			})
			ctx = cfg.ToContext(ctx)
			if tc.getRun == nil {
				tc.getRun = getRun
			}
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, tc.getRun, tc.pt, tc.pst)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
			}
			if d := cmp.Diff(tc.want, rpt); d != "" {
				t.Errorf("Did not get expected ResolvedPipelineTask with Matrix and Custom Task: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestIsSuccessful(t *testing.T) {
	for _, tc := range []struct {
		name string
		rpt  ResolvedPipelineTask
		want bool
	}{{
		name: "taskrun not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "run not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0])},
		},
		want: false,
	}, {
		name: "run running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
		},
		want: false,
	}, {
		name: "taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
		},
		want: true,
	}, {
		name: "run succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0])},
		},
		want: true,
	}, {
		name: "taskrun failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
		},
		want: false,
	}, {
		name: "run failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
		},
		want: false,
	}, {
		name: "taskrun failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withRetries(makeToBeRetried(trs[0]))},
		},
		want: false,
	}, {
		name: "run failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
		},
		want: false,
	}, {
		name: "run failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunRetries(makeCustomRunFailed(customRuns[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0]))},
		},
		want: false,
	}, {
		name: "run cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0]))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "run cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0])))},
		},
		want: false,
	}, {
		name: "run cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0])))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "matrixed taskruns not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed runs not started",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed taskruns running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunSucceeded(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0]), makeSucceeded(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed runs succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0]), makeCustomRunSucceeded(customRuns[1])},
		},
		want: true,
	}, {
		name: "one matrixed taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeFailed(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunFailed(customRuns[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun failed, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run failed, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeToBeRetried(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunFailed(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: one taskrun with retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: one run with retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), withCustomRunRetries(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunRetries(makeCustomRunFailed(customRuns[0])), withCustomRunRetries(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), withCustomRunCancelled(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), withCancelled(newTaskRun(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), withCustomRunCancelled(newCustomRun(customRuns[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), withCustomRunCancelled(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retries remaining, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retries remaining, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), withCancelled(withRetries(makeFailed(trs[1])))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0]))), withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[1])))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retried, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retried, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0]))), makeCustomRunStarted(customRuns[1])},
		},
		want: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rpt.isSuccessful(); got != tc.want {
				t.Errorf("expected isSuccessful: %t but got %t", tc.want, got)
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	for _, tc := range []struct {
		name string
		rpt  ResolvedPipelineTask
		want bool
	}{{
		name: "taskrun not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "run not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0])},
		},
		want: true,
	}, {
		name: "run running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0])},
		},
		want: true,
	}, {
		name: "taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0])},
		},
		want: false,
	}, {
		name: "run succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0])},
		},
		want: false,
	}, {
		name: "taskrun failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0])},
		},
		want: false,
	}, {
		name: "run failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
		},
		want: false,
	}, {
		name: "taskrun failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withRetries(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "run failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0])},
		},
		want: false,
	}, {
		name: "run failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunRetries(makeCustomRunFailed(customRuns[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0]))},
		},
		want: true,
	}, {
		name: "run cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task"},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0]))},
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "taskrun cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0]))},
		},
		want: false,
	}, {
		name: "run cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0]))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0])))},
		},
		want: false,
	}, {
		name: "run cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1.PipelineTask{Name: "task", Retries: 1},
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0])))},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "matrixed taskruns not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed runs not started",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
		},
		want: false,
	}, {
		name: "matrixed taskruns running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed runs running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}, {
		name: "one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeStarted(trs[0]), makeSucceeded(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunStarted(customRuns[0]), makeCustomRunSucceeded(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeSucceeded(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunSucceeded(customRuns[0]), makeCustomRunSucceeded(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeFailed(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunFailed(customRuns[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun failed, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{makeFailed(trs[0]), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run failed, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeToBeRetried(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs failed: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), makeCustomRunFailed(customRuns[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: one taskrun with retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs failed: one run with retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{makeCustomRunFailed(customRuns[0]), withCustomRunRetries(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunRetries(makeCustomRunFailed(customRuns[0])), withCustomRunRetries(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), withCustomRunCancelled(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), withCancelled(newTaskRun(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), withCustomRunCancelled(newCustomRun(customRuns[1]))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1.TaskRun{withCancelled(newTaskRun(trs[0])), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(newCustomRun(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), withCustomRunCancelled(makeCustomRunFailed(customRuns[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retries remaining, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled: retries remaining, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(makeCustomRunFailed(customRuns[0])), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), withCancelled(withRetries(makeFailed(trs[1])))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0]))), withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[1])))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retried, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled: retried, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			CustomRuns:   []*v1beta1.CustomRun{withCustomRunCancelled(withCustomRunRetries(makeCustomRunFailed(customRuns[0]))), makeCustomRunStarted(customRuns[1])},
		},
		want: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rpt.IsRunning(); got != tc.want {
				t.Errorf("expected IsRunning: %t but got %t", tc.want, got)
			}
		})
	}
}
