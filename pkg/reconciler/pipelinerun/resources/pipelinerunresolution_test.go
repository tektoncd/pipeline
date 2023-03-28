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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
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

func nopGetRun(string) (v1beta1.RunObject, error) {
	return nil, errors.New("GetRun should not be called")
}
func nopGetTask(context.Context, string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
	return nil, nil, errors.New("GetTask should not be called")
}
func nopGetTaskRun(string) (*v1beta1.TaskRun, error) {
	return nil, errors.New("GetTaskRun should not be called")
}

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
	TaskRef: &v1beta1.TaskRef{Name: "task"},
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
}, {
	Name:    "mytask10",
	TaskRef: &v1beta1.TaskRef{Name: "taskWithWhenExpressions"},
	WhenExpressions: []v1beta1.WhenExpression{{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo", "bar"},
	}},
}, {
	Name:    "mytask11",
	TaskRef: &v1beta1.TaskRef{Name: "taskWithWhenExpressions"},
	WhenExpressions: []v1beta1.WhenExpression{{
		Input:    "foo",
		Operator: selection.NotIn,
		Values:   []string{"foo", "bar"},
	}},
}, {
	Name:    "mytask12",
	TaskRef: &v1beta1.TaskRef{Name: "taskWithWhenExpressions"},
	WhenExpressions: []v1beta1.WhenExpression{{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo", "bar"},
	}},
	RunAfter: []string{"mytask1"},
}, {
	Name:    "mytask13",
	TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
}, {
	Name:    "mytask14",
	TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
}, {
	Name:    "mytask15",
	TaskRef: &v1beta1.TaskRef{Name: "taskWithReferenceToTaskResult"},
	Params:  v1beta1.Params{{Name: "param1", Value: *v1beta1.NewStructuredValues("$(tasks.mytask1.results.result1)")}},
}, {
	Name: "mytask16",
	Matrix: &v1beta1.Matrix{
		Params: v1beta1.Params{{
			Name:  "browser",
			Value: v1beta1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name: "mytask17",
	Matrix: &v1beta1.Matrix{
		Params: v1beta1.Params{{
			Name:  "browser",
			Value: v1beta1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask18",
	TaskRef: &v1beta1.TaskRef{Name: "task"},
	Retries: 1,
	Matrix: &v1beta1.Matrix{
		Params: v1beta1.Params{{
			Name:  "browser",
			Value: v1beta1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask19",
	TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
	Matrix: &v1beta1.Matrix{
		Params: v1beta1.Params{{
			Name:  "browser",
			Value: v1beta1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask20",
	TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: "customtask"},
	Matrix: &v1beta1.Matrix{
		Params: v1beta1.Params{{
			Name:  "browser",
			Value: v1beta1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}, {
	Name:    "mytask21",
	TaskRef: &v1beta1.TaskRef{Name: "task"},
	Retries: 2,
	Matrix: &v1beta1.Matrix{
		Params: v1beta1.Params{{
			Name:  "browser",
			Value: v1beta1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
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
		Steps: []v1beta1.Step{{
			Name: "step1",
		}},
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
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask4",
	},
	Spec: v1beta1.TaskRunSpec{},
}}

var runs = []v1beta1.CustomRun{{
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

var v1alpha1Runs = []v1alpha1.Run{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask15",
	},
	Spec: v1alpha1.RunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask16",
	},
	Spec: v1alpha1.RunSpec{},
}}

var matrixedPipelineTask = &v1beta1.PipelineTask{
	Name: "task",
	Matrix: &v1beta1.Matrix{
		Params: v1beta1.Params{{
			Name:  "browser",
			Value: v1beta1.ParamValue{ArrayVal: []string{"safari", "chrome"}},
		}}},
}

func makeScheduled(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status = v1beta1.TaskRunStatus{ /* explicitly empty */ }
	return newTr
}

func makeStarted(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newTr
}

func makeRunStarted(run v1beta1.CustomRun) *v1beta1.CustomRun {
	newRun := newRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newRun
}

func makeSucceeded(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionTrue
	return newTr
}

func makeRunSucceeded(run v1beta1.CustomRun) *v1beta1.CustomRun {
	newRun := newRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionTrue
	return newRun
}

func makeFailed(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionFalse
	return newTr
}

func makeToBeRetried(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionUnknown
	newTr.Status.Conditions[0].Reason = v1beta1.TaskRunReasonToBeRetried.String()
	return newTr
}

func makeV1alpha1RunFailed(run v1alpha1.Run) *v1alpha1.Run {
	newRun := newV1alpha1Run(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionFalse
	return newRun
}

func makeRunFailed(run v1beta1.CustomRun) *v1beta1.CustomRun {
	newRun := newRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionFalse
	return newRun
}

func withCancelled(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Status.Conditions[0].Reason = v1beta1.TaskRunSpecStatusCancelled
	return tr
}

func withCancelledForTimeout(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Spec.StatusMessage = v1beta1.TaskRunCancelledByPipelineTimeoutMsg
	tr.Status.Conditions[0].Reason = v1beta1.TaskRunSpecStatusCancelled
	return tr
}

func withV1alpha1RunCancelled(run *v1alpha1.Run) *v1alpha1.Run {
	run.Status.Conditions[0].Reason = v1alpha1.RunReasonCancelled.String()
	return run
}

func withRunCancelled(run *v1beta1.CustomRun) *v1beta1.CustomRun {
	run.Status.Conditions[0].Reason = v1beta1.CustomRunReasonCancelled.String()
	return run
}

func withRunCancelledForTimeout(run *v1beta1.CustomRun) *v1beta1.CustomRun {
	run.Spec.StatusMessage = v1beta1.CustomRunCancelledByPipelineTimeoutMsg
	run.Status.Conditions[0].Reason = v1beta1.CustomRunReasonCancelled.String()
	return run
}

func withCancelledBySpec(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Spec.Status = v1beta1.TaskRunSpecStatusCancelled
	return tr
}

func withRunCancelledBySpec(run *v1beta1.CustomRun) *v1beta1.CustomRun {
	run.Spec.Status = v1beta1.CustomRunSpecStatusCancelled
	return run
}

func makeRetried(tr v1beta1.TaskRun) (newTr *v1beta1.TaskRun) {
	newTr = newTaskRun(tr)
	newTr = withRetries(newTr)
	return
}

func withRetries(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Status.RetriesStatus = []v1beta1.TaskRunStatus{{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return tr
}

func withRunRetries(r *v1beta1.CustomRun) *v1beta1.CustomRun {
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

func withV1alpha1RunRetries(r *v1alpha1.Run) *v1alpha1.Run {
	r.Status.RetriesStatus = []v1alpha1.RunStatus{{
		Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		},
	}}
	return r
}

func newTaskRun(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			Name:      tr.Name,
		},
		Spec: tr.Spec,
		Status: v1beta1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}
}

func withPipelineTaskRetries(pt v1beta1.PipelineTask, retries int) *v1beta1.PipelineTask {
	pt.Retries = retries
	return &pt
}

func newV1alpha1Run(run v1alpha1.Run) *v1alpha1.Run {
	return &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: run.Namespace,
			Name:      run.Name,
		},
		Spec: run.Spec,
		Status: v1alpha1.RunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}
}

func newRun(run v1beta1.CustomRun) *v1beta1.CustomRun {
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
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneStartedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeStarted(trs[0]),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneFinishedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var oneFailedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeFailed(trs[0]),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var finalScheduledState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      makeScheduled(trs[1]),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var allFinishedState = PipelineRunState{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var noRunStartedState = PipelineRunState{{
	PipelineTask:  &pts[12],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask13",
	RunObject:     nil,
}, {
	PipelineTask:  &pts[13],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask14",
	RunObject:     nil,
}}

var oneRunStartedState = PipelineRunState{{
	PipelineTask:  &pts[12],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask13",
	RunObject:     makeRunStarted(runs[0]),
}, {
	PipelineTask:  &pts[13],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask14",
	RunObject:     nil,
}}

var oneRunFinishedState = PipelineRunState{{
	PipelineTask:  &pts[12],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask13",
	RunObject:     makeRunSucceeded(runs[0]),
}, {
	PipelineTask:  &pts[13],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask14",
	RunObject:     nil,
}}

var oneRunFailedState = PipelineRunState{{
	PipelineTask:  &pts[12],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask13",
	RunObject:     makeRunFailed(runs[0]),
}, {
	PipelineTask:  &pts[13],
	CustomTask:    true,
	RunObjectName: "pipelinerun-mytask14",
	RunObject:     nil,
}}

var taskCancelled = PipelineRunState{{
	PipelineTask: &pts[4],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      withCancelled(makeRetried(trs[0])),
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var runCancelled = PipelineRunState{{
	PipelineTask:  &pts[12],
	RunObjectName: "pipelinerun-mytask13",
	RunObject:     withRunCancelled(newRun(runs[0])),
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
	TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0])},
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
	TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0])},
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
	TaskRuns:     []*v1beta1.TaskRun{makeFailed(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRun:      nil,
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var finalScheduledStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     []*v1beta1.TaskRun{makeScheduled(trs[1])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var allFinishedStateMatrix = PipelineRunState{{
	PipelineTask: &pts[15],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[16],
	TaskRunNames: []string{"pipelinerun-mytask2"},
	TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0])},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

var noRunStartedStateMatrix = PipelineRunState{{
	PipelineTask:   &pts[18],
	CustomTask:     true,
	RunObjectNames: []string{"pipelinerun-mytask13"},
	RunObjects:     nil,
}, {
	PipelineTask:   &pts[19],
	CustomTask:     true,
	RunObjectNames: []string{"pipelinerun-mytask13"},
	RunObjects:     nil,
}}

var oneRunStartedStateMatrix = PipelineRunState{{
	PipelineTask:   &pts[18],
	CustomTask:     true,
	RunObjectNames: []string{"pipelinerun-mytask13"},
	RunObjects:     []v1beta1.RunObject{makeRunStarted(runs[0])},
}, {
	PipelineTask:   &pts[19],
	CustomTask:     true,
	RunObjectNames: []string{"pipelinerun-mytask14"},
	RunObjects:     nil,
}}

var oneRunFailedStateMatrix = PipelineRunState{{
	PipelineTask:   &pts[18],
	CustomTask:     true,
	RunObjectNames: []string{"pipelinerun-mytask13"},
	RunObjects:     []v1beta1.RunObject{makeRunFailed(runs[0])},
}, {
	PipelineTask:   &pts[19],
	CustomTask:     true,
	RunObjectNames: []string{"pipelinerun-mytask14"},
	RunObjects:     nil,
}}

var taskCancelledMatrix = PipelineRunState{{
	PipelineTask: &pts[20],
	TaskRunNames: []string{"pipelinerun-mytask1"},
	TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeRetried(trs[0]))},
	ResolvedTask: &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	},
}}

func dagFromState(state PipelineRunState) (*dag.Graph, error) {
	pts := []v1beta1.PipelineTask{}
	for _, rpt := range state {
		pts = append(pts, *rpt.PipelineTask)
	}
	return dag.Build(v1beta1.PipelineTaskList(pts), v1beta1.PipelineTaskList(pts).Deps())
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
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      nil,
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
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      withCancelled(makeFailed(trs[0])),
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      nil,
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
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
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
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeSucceeded(trs[0]),
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[0],
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[7], // mytask8 runAfter mytask1, mytask6
			TaskRunName:  "pipelinerun-mytask3",
			TaskRun:      nil,
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
			TaskRunName:  "pipelinerun-mytask1",
			TaskRun:      makeFailed(trs[0]),
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[5],
			TaskRunName:  "pipelinerun-mytask2",
			TaskRun:      makeStarted(trs[1]),
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[6], // mytask7 runAfter mytask6
			TaskRunName:  "pipelinerun-mytask3",
			TaskRun:      nil,
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
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
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
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
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
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			PipelineTask: &pts[11],
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}},
		expected: map[string]bool{
			"mytask12": false,
		},
	}, {
		name:  "run-started",
		state: oneRunStartedState,
		expected: map[string]bool{
			"mytask13": false,
		},
	}, {
		name:  "run-cancelled",
		state: runCancelled,
		expected: map[string]bool{
			"mytask13": false,
		},
	}, {
		name: "tasks-when-expressions",
		state: PipelineRunState{{
			// skipped because when expressions evaluate to false
			PipelineTask: &pts[10],
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its parent task being skipped because when expressions are scoped to task
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-1",
			TaskRun:     nil,
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
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its parent task being skipped because when expressions are scoped to task
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-1",
			TaskRun:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its grandparent task being skipped because when expressions are scoped to task
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask19",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask18"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-2",
			TaskRun:     nil,
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
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its parent task mytask11 being skipped because when expressions are scoped to task
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-1",
			TaskRun:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// not skipped regardless of its grandparent task mytask11 being skipped because when expressions are scoped to task
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask19",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask18"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-2",
			TaskRun:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// attempted but skipped because of missing result in params from parent task mytask11 which was skipped
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "mytask20",
				TaskRef: &v1beta1.TaskRef{Name: "task"},
				Params: v1beta1.Params{{
					Name:  "commit",
					Value: *v1beta1.NewStructuredValues("$(tasks.mytask11.results.missingResult)"),
				}},
			},
			TaskRunName: "pipelinerun-resource-dependent-task-1",
			TaskRun:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// skipped because of parent task mytask20 was skipped because of missing result from grandparent task
			// mytask11 which was skipped
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask21",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask20"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-3",
			TaskRun:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// attempted but skipped because of missing result from parent task mytask11 which was skipped in when expressions
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "mytask22",
				TaskRef: &v1beta1.TaskRef{Name: "task"},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(tasks.mytask11.results.missingResult)",
					Operator: selection.In,
					Values:   []string{"expectedResult"},
				}},
			},
			TaskRunName: "pipelinerun-resource-dependent-task-2",
			TaskRun:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// skipped because of parent task mytask22 was skipping because of missing result from grandparent task
			// mytask11 which was skipped
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask23",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask22"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-4",
			TaskRun:     nil,
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
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "run not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeStarted(trs[0]),
		},
		want: false,
	}, {
		name: "run running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunStarted(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeSucceeded(trs[0]),
		},
		want: false,
	}, {
		name: "run succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunSucceeded(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeFailed(trs[0]),
		},
		want: true,
	}, {
		name: "run failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunFailed(runs[0]),
		},
		want: true,
	}, {
		name: "run failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    makeRunFailed(runs[0]),
		},
		want: true,
	}, {
		name: "v1alpha1 run failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    makeV1alpha1RunFailed(v1alpha1Runs[0]),
		},
		want: false,
	}, {
		name: "taskrun failed - Retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withRetries(makeFailed(trs[0])),
		},
		want: true,
	}, {
		name: "v1alpha1 run failed - Retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    withV1alpha1RunRetries(makeV1alpha1RunFailed(v1alpha1Runs[0])),
		},
		want: true,
	}, {
		name: "run failed - Retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    withRunRetries(makeRunFailed(runs[0])),
		},
		want: true,
	}, {
		name: "taskrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: true,
	}, {
		name: "taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(newTaskRun(trs[0])),
		},
		want: false,
	}, {
		name: "taskrun cancelled for timeout",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelledForTimeout(makeFailed(trs[0])),
		},
		want: true,
	}, {
		name: "run cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			RunObject:    withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "run cancelled for timeout",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			RunObject:    withRunCancelledForTimeout(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			RunObject:    withRunCancelled(newRun(runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: true,
	}, {
		name: "v1alpha1 run cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			RunObject:    withV1alpha1RunCancelled(makeV1alpha1RunFailed(v1alpha1Runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "run cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			RunObject:    withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "taskrun cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(withRetries(makeFailed(trs[0]))),
		},
		want: true,
	}, {
		name: "run cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			RunObject:    withRunCancelled(withRunRetries(makeRunFailed(runs[0]))),
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
			TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunStarted(runs[0]), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunStarted(runs[0]), makeRunSucceeded(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunSucceeded(runs[0]), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeFailed(trs[0]), makeFailed(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed runs failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunFailed(runs[1])},
		},
		want: true,
	}, {
		name: "one matrixed taskrun failed, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeFailed(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run failed, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withRetries(makeFailed(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed v1alpha1 runs failed: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{makeV1alpha1RunFailed(v1alpha1Runs[0]), makeV1alpha1RunFailed(v1alpha1Runs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs failed: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunFailed(runs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns failed: one taskrun with retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: one run with retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), withRunRetries(makeRunFailed(runs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs failed: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunRetries(makeRunFailed(runs[0])), withRunRetries(makeRunFailed(runs[1]))},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), withRunCancelled(makeRunFailed(runs[1]))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(newTaskRun(trs[0])), withCancelled(newTaskRun(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(newRun(runs[0])), withRunCancelled(newRun(runs[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(newTaskRun(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(newRun(runs[0])), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed v1alpha1 runs cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withV1alpha1RunCancelled(makeV1alpha1RunFailed(v1alpha1Runs[0])), withV1alpha1RunCancelled(makeV1alpha1RunFailed(v1alpha1Runs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), withRunCancelled(makeRunFailed(runs[1]))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled: retries remaining, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retries remaining, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), withCancelled(withRetries(makeFailed(trs[1])))},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(withRunRetries(makeRunFailed(runs[0]))), withRunCancelled(withRunRetries(makeRunFailed(runs[1])))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled: retried, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retried, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(withRunRetries(makeRunFailed(runs[0]))), makeRunStarted(runs[1])},
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
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task not skipped because parent is not yet done
			PipelineTask: &pts[11],
			TaskRun:      nil,
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
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task is not skipped regardless of its parent task being skipped due to when expressions evaluating
			// to false, because when expressions are scoped to task only
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-1",
			TaskRun:     nil,
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
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task is not skipped regardless of its parent task being skipped due to when expressions evaluating
			// to false, because when expressions are scoped to task only
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask18",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask11"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-1",
			TaskRun:     nil,
			ResolvedTask: &resources.ResolvedTask{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task is not skipped regardless of its parent task being skipped due to when expressions evaluating
			// to false, because when expressions are scoped to task only
			PipelineTask: &v1beta1.PipelineTask{
				Name:     "mytask19",
				TaskRef:  &v1beta1.TaskRef{Name: "task"},
				RunAfter: []string{"mytask18"},
			},
			TaskRunName: "pipelinerun-ordering-dependent-task-2",
			TaskRun:     nil,
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

func getExpectedMessage(runName string, specStatus v1beta1.PipelineRunSpecStatus, status corev1.ConditionStatus,
	successful, incomplete, skipped, failed, cancelled int) string {
	if status == corev1.ConditionFalse &&
		(specStatus == v1beta1.PipelineRunSpecStatusCancelledRunFinally ||
			specStatus == v1beta1.PipelineRunSpecStatusStoppedRunFinally) {
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
	pts := []v1beta1.PipelineTask{{
		Name:    "customtask",
		TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
	}, {
		Name: "customtask-spec",
		TaskSpec: &v1beta1.EmbeddedTask{
			TypeMeta: runtime.TypeMeta{
				APIVersion: "example.dev/v0",
				Kind:       "Example",
			},
		},
	}, {
		Name:    "run-exists",
		TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
	}}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
	}
	run := &v1beta1.CustomRun{ObjectMeta: metav1.ObjectMeta{Name: "run-exists-abcde"}}
	getRun := func(name string) (v1beta1.RunObject, error) {
		if name == "pipelinerun-run-exists" {
			return run, nil
		}
		return nil, kerrors.NewNotFound(v1beta1.Resource("run"), name)
	}
	pipelineState := PipelineRunState{}
	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	ctx = cfg.ToContext(ctx)
	for _, task := range pts {
		ps, err := ResolvePipelineTask(ctx, pr, nopGetTask, nopGetTaskRun, getRun, task)
		if err != nil {
			t.Fatalf("ResolvePipelineTask: %v", err)
		}
		pipelineState = append(pipelineState, ps)
	}

	expectedState := PipelineRunState{{
		PipelineTask:  &pts[0],
		CustomTask:    true,
		RunObjectName: "pipelinerun-customtask",
		RunObject:     nil,
	}, {
		PipelineTask:  &pts[1],
		CustomTask:    true,
		RunObjectName: "pipelinerun-customtask-spec",
		RunObject:     nil,
	}, {
		PipelineTask:  &pts[2],
		CustomTask:    true,
		RunObjectName: "pipelinerun-run-exists",
		RunObject:     run,
	}}
	if d := cmp.Diff(expectedState, pipelineState); d != "" {
		t.Errorf("Unexpected pipeline state: %s", diff.PrintWantGot(d))
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return task, nil, nil
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	pipelineState := PipelineRunState{}
	for _, task := range pts {
		ps, err := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, task)
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
	if d := cmp.Diff(pipelineState[0].ResolvedTask, expectedTask, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but actual differed %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(pipelineState[1].ResolvedTask, expectedTask, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected resources where only Tasks were resolved but actual differed %s", diff.PrintWantGot(d))
	}
}

func TestResolvePipelineRun_TaskDoesntExist(t *testing.T) {
	pts := []v1beta1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask2",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
		Matrix: &v1beta1.Matrix{
			Params: v1beta1.Params{{
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("f", "o", "o"),
			}, {
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("b", "a", "r"),
			}},
		}}}

	// Return an error when the Task is retrieved, as if it didn't exist
	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return nil, nil, kerrors.NewNotFound(v1beta1.Resource("task"), name)
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) {
		return nil, kerrors.NewNotFound(v1beta1.Resource("taskrun"), name)
	}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	for _, pt := range pts {
		_, err := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, pt)
		switch err := err.(type) {
		case nil:
			t.Fatalf("Expected error getting non-existent Tasks for Pipeline %s but got none", p.Name)
		case *TaskNotFoundError:
			// expected error
		default:
			t.Fatalf("Expected specific error type returned by func for non-existent Task for Pipeline %s but got %s", p.Name, err)
		}
	}
}

func TestResolvePipelineRun_VerificationFailed(t *testing.T) {
	pts := []v1beta1.PipelineTask{{
		Name:    "mytask1",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
	}, {
		Name:    "mytask2",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
		Matrix: &v1beta1.Matrix{
			Params: v1beta1.Params{{
				Name:  "foo",
				Value: *v1beta1.NewStructuredValues("f", "o", "o"),
			}, {
				Name:  "bar",
				Value: *v1beta1.NewStructuredValues("b", "a", "r"),
			}},
		}}}

	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return nil, nil, trustedresources.ErrResourceVerificationFailed
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	for _, pt := range pts {
		_, err := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, pt)
		if err == nil {
			t.Errorf("expected to get err but got nil")
		}
		if err != trustedresources.ErrResourceVerificationFailed {
			t.Errorf("expected to get %v but got %v", trustedresources.ErrResourceVerificationFailed, err)
		}
	}
}

func TestValidateWorkspaceBindingsWithValidWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *v1beta1.PipelineSpec
		pr   *v1beta1.PipelineRun
		err  string
	}{{
		name: "include required workspace",
		spec: &v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
		},
		pr: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "foo",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
	}, {
		name: "omit optional workspace",
		spec: &v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name:     "foo",
				Optional: true,
			}},
		},
		pr: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{},
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
		spec *v1beta1.PipelineSpec
		pr   *v1beta1.PipelineRun
		err  string
	}{{
		name: "missing required workspace",
		spec: &v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
		},
		pr: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{},
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
		p       *v1beta1.Pipeline
		pr      *v1beta1.PipelineRun
		wantErr bool
	}{{
		name: "valid task mapping",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelines",
			},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
				}},
			},
		},
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "pipeline",
				},
				TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{{
					PipelineTaskName:       "mytask1",
					TaskServiceAccountName: "default",
				}},
			},
		},
		wantErr: false,
	}, {
		name: "valid finally task mapping",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelines",
			},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name: "myfinaltask1",
					TaskRef: &v1beta1.TaskRef{
						Name: "finaltask",
					},
				}},
			},
		},
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "pipeline",
				},
				TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{{
					PipelineTaskName:       "myfinaltask1",
					TaskServiceAccountName: "default",
				}},
			},
		},
		wantErr: false,
	}, {
		name: "invalid task mapping",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelines",
			},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name: "myfinaltask1",
					TaskRef: &v1beta1.TaskRef{
						Name: "finaltask",
					},
				}},
			},
		},
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun",
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{
					Name: "pipeline",
				},
				TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{{
					PipelineTaskName:       "wrongtask",
					TaskServiceAccountName: "default",
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

	t1 := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: tName1,
		},
	}

	ptwe1 := v1beta1.WhenExpression{
		Input:    "foo",
		Operator: selection.In,
		Values:   []string{"foo"},
	}

	pt := v1beta1.PipelineTask{
		Name:            "mytask1",
		TaskRef:         &v1beta1.TaskRef{Name: "task"},
		WhenExpressions: []v1beta1.WhenExpression{ptwe1},
	}

	getTask := func(_ context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return task, nil, nil
	}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	getTaskRun := func(name string) (*v1beta1.TaskRun, error) {
		switch name {
		case "pipelinerun-mytask1-always-true-0":
			return t1, nil
		case "pipelinerun-mytask1":
			return &trs[0], nil
		}
		return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
	}

	t.Run("When Expressions exist", func(t *testing.T) {
		_, err := ResolvePipelineTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, pt)
		if err != nil {
			t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
		}
	})
}

func TestIsCustomTask(t *testing.T) {
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return task, nil, nil
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	getRun := func(name string) (v1beta1.RunObject, error) { return nil, nil }

	for _, tc := range []struct {
		name string
		pt   v1beta1.PipelineTask
		want bool
	}{{
		name: "custom taskSpec",
		pt: v1beta1.PipelineTask{
			TaskSpec: &v1beta1.EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "example.dev/v0",
					Kind:       "Sample",
				},
			},
		},
		want: true,
	}, {
		name: "custom taskSpec missing kind",
		pt: v1beta1.PipelineTask{
			TaskSpec: &v1beta1.EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "example.dev/v0",
					Kind:       "",
				},
			},
		},
		want: false,
	}, {
		name: "custom taskSpec missing apiVersion",
		pt: v1beta1.PipelineTask{
			TaskSpec: &v1beta1.EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "",
					Kind:       "Sample",
				},
			},
		},
		want: false,
	}, {
		name: "custom taskRef",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Sample",
			},
		},
		want: true,
	}, {
		name: "custom taskRef missing kind",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "",
			},
		},
		want: false,
	}, {
		name: "custom taskRef missing apiVersion",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				APIVersion: "",
				Kind:       "Sample",
			},
		},
		want: false,
	}, {
		name: "non-custom taskref",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				Name: "task",
			},
		},
		want: false,
	}, {
		name: "non-custom taskspec",
		pt: v1beta1.PipelineTask{
			TaskSpec: &v1beta1.EmbeddedTask{},
		},
		want: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.NewStore(logtesting.TestLogger(t))
			ctx = cfg.ToContext(ctx)
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt)
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
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dag-task",
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "commit",
					Value: *v1beta1.NewStructuredValues("SHA2"),
				}},
			},
		},
	}

	state := PipelineRunState{{
		TaskRunName: "dag-task",
		TaskRun:     tr,
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "dag-task",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task-1",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			Params: v1beta1.Params{{
				Name:  "commit",
				Value: *v1beta1.NewStructuredValues("$(tasks.dag-task.results.commit)"),
			}},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task-2",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			Params: v1beta1.Params{{
				Name:  "commit",
				Value: *v1beta1.NewStructuredValues("$(tasks.dag-task.results.missingResult)"),
			}},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task-3",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			WhenExpressions: v1beta1.WhenExpressions{{
				Input:    "foo",
				Operator: selection.NotIn,
				Values:   []string{"bar"},
			}},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task-4",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			WhenExpressions: v1beta1.WhenExpressions{{
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"bar"},
			}},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task-5",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			WhenExpressions: v1beta1.WhenExpressions{{
				Input:    "$(tasks.dag-task.results.commit)",
				Operator: selection.In,
				Values:   []string{"SHA2"},
			}},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task-6",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			WhenExpressions: v1beta1.WhenExpressions{{
				Input:    "$(tasks.dag-task.results.missing)",
				Operator: selection.In,
				Values:   []string{"none"},
			}},
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
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tasks := v1beta1.PipelineTaskList([]v1beta1.PipelineTask{*state[0].PipelineTask})
			d, err := dag.Build(tasks, tasks.Deps())
			if err != nil {
				t.Fatalf("Could not get a dag from the dag tasks %#v: %v", state[0], err)
			}

			// build graph with finally tasks
			var pts []v1beta1.PipelineTask
			for i := range state {
				if i > 0 { // first one is a dag task that produces a result
					pts = append(pts, *state[i].PipelineTask)
				}
			}
			dfinally, err := dag.Build(v1beta1.PipelineTaskList(pts), map[string][]string{})
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
		TaskRunName: "dag-task",
		TaskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dag-task",
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "dag-task",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
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
				TaskRun: &v1beta1.TaskRun{
					ObjectMeta: metav1.ObjectMeta{
						Name: "final-task",
					},
					Status: v1beta1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: []apis.Condition{{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionUnknown,
							}},
						},
					},
				},
				PipelineTask: &v1beta1.PipelineTask{
					Name:    "final-task",
					TaskRef: &v1beta1.TaskRef{Name: "task"},
					WhenExpressions: []v1beta1.WhenExpression{
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
			SkippingReason: v1beta1.None,
		},
	}, {
		name: "task scheduled",
		state: PipelineRunState{
			task,
			{
				TaskRunName: "final-task",
				TaskRun: &v1beta1.TaskRun{
					ObjectMeta: metav1.ObjectMeta{
						Name: "final-task",
					},
					Status: v1beta1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: []apis.Condition{ /* explicitly empty */ },
						},
					},
				},
				PipelineTask: &v1beta1.PipelineTask{
					Name:    "final-task",
					TaskRef: &v1beta1.TaskRef{Name: "task"},
					WhenExpressions: []v1beta1.WhenExpression{
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
			SkippingReason: v1beta1.None,
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			tasks := v1beta1.PipelineTaskList([]v1beta1.PipelineTask{*tc.state[0].PipelineTask})
			d, err := dag.Build(tasks, tasks.Deps())
			if err != nil {
				t.Fatalf("Could not get a dag from the dag tasks %#v: %v", tc.state[0], err)
			}

			// build graph with finally tasks
			var pts v1beta1.PipelineTaskList
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
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dag-task",
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "commit",
					Value: *v1beta1.NewStructuredValues("SHA2"),
				}},
			},
		},
	}

	state := PipelineRunState{{
		TaskRunName: "dag-task",
		TaskRun:     tr,
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "dag-task",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			Params: v1beta1.Params{{
				Name:  "commit",
				Value: *v1beta1.NewStructuredValues("$(tasks.dag-task.results.commit)"),
			}},
		},
	},
	}

	tasks := v1beta1.PipelineTaskList([]v1beta1.PipelineTask{*state[0].PipelineTask})
	d, err := dag.Build(tasks, tasks.Deps())
	if err != nil {
		t.Fatalf("Could not get a dag from the dag tasks %#v: %v", state[0], err)
	}

	// build graph with finally tasks
	pts := []v1beta1.PipelineTask{*state[1].PipelineTask}

	dfinally, err := dag.Build(v1beta1.PipelineTaskList(pts), map[string][]string{})
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

	childRefs := []v1beta1.ChildStatusReference{{
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
	childRefs := []v1beta1.ChildStatusReference{{
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
	childRefs := []v1beta1.ChildStatusReference{{
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
			namesOfRunsFromChildRefs := getNamesOfRuns(childRefs, tc.ptName, testPrName, 2)
			sort.Strings(namesOfRunsFromChildRefs)
			if d := cmp.Diff(tc.wantRunNames, namesOfRunsFromChildRefs); d != "" {
				t.Errorf("getRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetRunName(t *testing.T) {
	prName := "pipeline-run"
	childRefs := []v1beta1.ChildStatusReference{{
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
			rnFromChildRefs := getRunName(childRefs, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, rnFromChildRefs); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
			rnFromBoth := getRunName(childRefs, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, rnFromBoth); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestIsMatrixed(t *testing.T) {
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return task, nil, nil
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }
	getRun := func(name string) (v1beta1.RunObject, error) { return &runs[0], nil }

	for _, tc := range []struct {
		name string
		pt   v1beta1.PipelineTask
		want bool
	}{{
		name: "custom task with matrix",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Sample",
			},
			Matrix: &v1beta1.Matrix{
				Params: v1beta1.Params{{
					Name:  "platform",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
				}}},
		},
		want: true,
	}, {
		name: "custom task without matrix",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Sample",
			},
		},
		want: false,
	}, {
		name: "task with matrix",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				Name: "my-task",
			},
			Matrix: &v1beta1.Matrix{
				Params: v1beta1.Params{{
					Name:  "platform",
					Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
				}}},
		},
		want: true,
	}, {
		name: "task without matrix",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
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
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt)
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

	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineRunName,
		},
	}

	var taskRuns []*v1beta1.TaskRun
	var taskRunsNames []string
	taskRunsMap := map[string]*v1beta1.TaskRun{}
	for i := 0; i < 9; i++ {
		trName := fmt.Sprintf("%s-%s-%d", pipelineRunName, pipelineTaskName, i)
		tr := &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: trName,
			},
		}
		taskRuns = append(taskRuns, tr)
		taskRunsNames = append(taskRunsNames, trName)
		taskRunsMap[trName] = tr
	}

	pts := []v1beta1.PipelineTask{{
		Name: "pipelinetask",
		TaskRef: &v1beta1.TaskRef{
			Name: "my-task",
		},
		Matrix: &v1beta1.Matrix{
			Params: v1beta1.Params{{
				Name:  "platform",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}}},
	}, {
		Name: "pipelinetask",
		TaskRef: &v1beta1.TaskRef{
			Name: "my-task",
		},
		Matrix: &v1beta1.Matrix{
			Params: v1beta1.Params{{
				Name:  "platform",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}, {
				Name:  "browsers",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"chrome", "safari", "firefox"}},
			}},
		}}}

	rtr := &resources.ResolvedTask{
		TaskName: "task",
		TaskSpec: &v1beta1.TaskSpec{Steps: []v1beta1.Step{{
			Name: "step1",
		}}},
	}

	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return task, nil, nil
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return taskRunsMap[name], nil }
	getRun := func(name string) (v1beta1.RunObject, error) { return &runs[0], nil }

	for _, tc := range []struct {
		name string
		pt   v1beta1.PipelineTask
		want *ResolvedPipelineTask
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
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt)
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

	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineRunName,
		},
	}

	var runs []v1beta1.RunObject
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

	pts := []v1beta1.PipelineTask{{
		Name: "pipelinetask",
		TaskRef: &v1beta1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
			Name:       "my-task",
		},
		Matrix: &v1beta1.Matrix{
			Params: v1beta1.Params{{
				Name:  "platform",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}}},
	}, {
		Name: "pipelinetask",
		TaskRef: &v1beta1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
			Name:       "my-task",
		},
		Matrix: &v1beta1.Matrix{
			Params: v1beta1.Params{{
				Name:  "platform",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}, {
				Name:  "browsers",
				Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"chrome", "safari", "firefox"}},
			}}},
	}}

	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, *v1beta1.ConfigSource, error) {
		return task, nil, nil
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }
	getRun := func(name string) (v1beta1.RunObject, error) { return runsMap[name], nil }

	for _, tc := range []struct {
		name   string
		pt     v1beta1.PipelineTask
		getRun GetRun
		want   *ResolvedPipelineTask
	}{{
		name: "custom task with matrix - single parameter",
		pt:   pts[0],
		want: &ResolvedPipelineTask{
			CustomTask:     true,
			RunObjectNames: runNames[:3],
			RunObjects:     runs[:3],
			PipelineTask:   &pts[0],
		},
	}, {
		name: "custom task with matrix - multiple parameters",
		pt:   pts[1],
		want: &ResolvedPipelineTask{
			CustomTask:     true,
			RunObjectNames: runNames,
			RunObjects:     runs,
			PipelineTask:   &pts[1],
		},
	}, {
		name: "custom task with matrix - nil run",
		pt:   pts[1],
		getRun: func(name string) (v1beta1.RunObject, error) {
			return nil, kerrors.NewNotFound(v1beta1.Resource("run"), name)
		},
		want: &ResolvedPipelineTask{
			CustomTask:     true,
			RunObjectNames: runNames,
			RunObjects:     nil,
			PipelineTask:   &pts[1],
		},
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
			rpt, err := ResolvePipelineTask(ctx, pr, getTask, getTaskRun, tc.getRun, tc.pt)
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
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "run not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeStarted(trs[0]),
		},
		want: false,
	}, {
		name: "run running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunStarted(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeSucceeded(trs[0]),
		},
		want: true,
	}, {
		name: "run succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunSucceeded(runs[0]),
		},
		want: true,
	}, {
		name: "taskrun failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeFailed(trs[0]),
		},
		want: false,
	}, {
		name: "run failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunFailed(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withRetries(makeToBeRetried(trs[0])),
		},
		want: false,
	}, {
		name: "run failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    makeRunFailed(runs[0]),
		},
		want: false,
	}, {
		name: "run failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    withRunRetries(makeRunFailed(runs[0])),
		},
		want: false,
	}, {
		name: "taskrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: false,
	}, {
		name: "taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(newTaskRun(trs[0])),
		},
		want: false,
	}, {
		name: "run cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			RunObject:    withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			RunObject:    withRunCancelled(newRun(runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: false,
	}, {
		name: "run cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			RunObject:    withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(withRetries(makeFailed(trs[0]))),
		},
		want: false,
	}, {
		name: "run cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			RunObject:    withRunCancelled(withRunRetries(makeRunFailed(runs[0]))),
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
			TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunStarted(runs[0]), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunStarted(runs[0]), makeRunSucceeded(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0]), makeSucceeded(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed runs succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunSucceeded(runs[0]), makeRunSucceeded(runs[1])},
		},
		want: true,
	}, {
		name: "one matrixed taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunSucceeded(runs[0]), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeFailed(trs[0]), makeFailed(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunFailed(runs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun failed, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeFailed(trs[0]), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run failed, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeToBeRetried(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunFailed(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: one taskrun with retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: one run with retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), withRunRetries(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunRetries(makeRunFailed(runs[0])), withRunRetries(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), withRunCancelled(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(newTaskRun(trs[0])), withCancelled(newTaskRun(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(newRun(runs[0])), withRunCancelled(newRun(runs[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(newTaskRun(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(newRun(runs[0])), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), withRunCancelled(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retries remaining, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retries remaining, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), makeRunStarted(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), withCancelled(withRetries(makeFailed(trs[1])))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(withRunRetries(makeRunFailed(runs[0]))), withRunCancelled(withRunRetries(makeRunFailed(runs[1])))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retried, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), makeStarted(trs[1])},
		},
		want: false,
	}, {
		name: "one matrixed run cancelled: retried, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(withRunRetries(makeRunFailed(runs[0]))), makeRunStarted(runs[1])},
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
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "run not started",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeStarted(trs[0]),
		},
		want: true,
	}, {
		name: "run running",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunStarted(runs[0]),
		},
		want: true,
	}, {
		name: "taskrun succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeSucceeded(trs[0]),
		},
		want: false,
	}, {
		name: "run succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunSucceeded(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeFailed(trs[0]),
		},
		want: false,
	}, {
		name: "run failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			RunObject:    makeRunFailed(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withRetries(makeFailed(trs[0])),
		},
		want: false,
	}, {
		name: "run failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    makeRunFailed(runs[0]),
		},
		want: false,
	}, {
		name: "run failed: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			RunObject:    withRunRetries(makeRunFailed(runs[0])),
		},
		want: false,
	}, {
		name: "taskrun cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: false,
	}, {
		name: "taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(newTaskRun(trs[0])),
		},
		want: true,
	}, {
		name: "run cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			RunObject:    withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			RunObject:    withRunCancelled(newRun(runs[0])),
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "taskrun cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: false,
	}, {
		name: "run cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			RunObject:    withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(withRetries(makeFailed(trs[0]))),
		},
		want: false,
	}, {
		name: "run cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			RunObject:    withRunCancelled(withRunRetries(makeRunFailed(runs[0]))),
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
			TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0]), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "matrixed runs running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunStarted(runs[0]), makeRunStarted(runs[1])},
		},
		want: true,
	}, {
		name: "one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeStarted(trs[0]), makeSucceeded(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunStarted(runs[0]), makeRunSucceeded(runs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns succeeded",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeSucceeded(trs[0]), makeSucceeded(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs succeeded",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunSucceeded(runs[0]), makeRunSucceeded(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeFailed(trs[0]), makeFailed(trs[1])},
		},
		want: false,
	}, {
		name: "matrixed runs failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunFailed(runs[1])},
		},
		want: false,
	}, {
		name: "one matrixed taskrun failed, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{makeFailed(trs[0]), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run failed, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunStarted(runs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns failed: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeToBeRetried(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs failed: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), makeRunFailed(runs[1])},
		},
		want: false,
	}, {
		name: "matrixed taskruns failed: one taskrun with retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withRetries(makeToBeRetried(trs[0])), withRetries(makeFailed(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs failed: one run with retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{makeRunFailed(runs[0]), withRunRetries(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs failed: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunRetries(makeRunFailed(runs[0])), withRunRetries(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "matrixed taskruns cancelled",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), withRunCancelled(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), makeRunStarted(runs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(newTaskRun(trs[0])), withCancelled(newTaskRun(trs[1]))},
		},
		want: true,
	}, {
		name: "matrixed runs cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(newRun(runs[0])), withRunCancelled(newRun(runs[1]))},
		},
		want: true,
	}, {
		name: "one matrixed taskrun cancelled but not failed",
		rpt: ResolvedPipelineTask{
			PipelineTask: matrixedPipelineTask,
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(newTaskRun(trs[0])), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled but not failed",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: matrixedPipelineTask,
			RunObjects:   []v1beta1.RunObject{withRunCancelled(newRun(runs[0])), makeRunStarted(runs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), withCancelled(makeFailed(trs[1]))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retries remaining",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), withRunCancelled(makeRunFailed(runs[1]))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retries remaining, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(makeFailed(trs[0])), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled: retries remaining, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(makeRunFailed(runs[0])), makeRunStarted(runs[1])},
		},
		want: true,
	}, {
		name: "matrixed taskruns cancelled: retried",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), withCancelled(withRetries(makeFailed(trs[1])))},
		},
		want: false,
	}, {
		name: "matrixed runs cancelled: retried",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(withRunRetries(makeRunFailed(runs[0]))), withRunCancelled(withRunRetries(makeRunFailed(runs[1])))},
		},
		want: false,
	}, {
		name: "one matrixed taskrun cancelled: retried, one matrixed taskrun running",
		rpt: ResolvedPipelineTask{
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			TaskRuns:     []*v1beta1.TaskRun{withCancelled(withRetries(makeFailed(trs[0]))), makeStarted(trs[1])},
		},
		want: true,
	}, {
		name: "one matrixed run cancelled: retried, one matrixed run running",
		rpt: ResolvedPipelineTask{
			CustomTask:   true,
			PipelineTask: withPipelineTaskRetries(*matrixedPipelineTask, 1),
			RunObjects:   []v1beta1.RunObject{withRunCancelled(withRunRetries(makeRunFailed(runs[0]))), makeRunStarted(runs[1])},
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
