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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	logtesting "knative.dev/pkg/logging/testing"
)

func nopGetRun(string) (*v1alpha1.Run, error) {
	return nil, errors.New("GetRun should not be called")
}
func nopGetTask(context.Context, string) (v1beta1.TaskObject, error) {
	return nil, errors.New("GetTask should not be called")
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
	Params:  []v1beta1.Param{{Name: "param1", Value: *v1beta1.NewArrayOrString("$(tasks.mytask1.results.result1)")}},
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
}}

var runs = []v1alpha1.Run{{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask13",
	},
	Spec: v1alpha1.RunSpec{},
}, {
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "namespace",
		Name:      "pipelinerun-mytask14",
	},
	Spec: v1alpha1.RunSpec{},
}}

var gitDeclaredResource = v1beta1.PipelineDeclaredResource{
	Name: "git-resource",
	Type: resourcev1alpha1.PipelineResourceTypeGit,
}

var gitSweetResourceBinding = v1beta1.PipelineResourceBinding{
	Name:        "git-resource",
	ResourceRef: &v1beta1.PipelineResourceRef{Name: "sweet-resource"},
}

func makeStarted(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newTr
}

func makeRunStarted(run v1alpha1.Run) *v1alpha1.Run {
	newRun := newRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionUnknown
	return newRun
}

func makeSucceeded(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionTrue
	return newTr
}

func makeRunSucceeded(run v1alpha1.Run) *v1alpha1.Run {
	newRun := newRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionTrue
	return newRun
}

func makeFailed(tr v1beta1.TaskRun) *v1beta1.TaskRun {
	newTr := newTaskRun(tr)
	newTr.Status.Conditions[0].Status = corev1.ConditionFalse
	return newTr
}

func makeRunFailed(run v1alpha1.Run) *v1alpha1.Run {
	newRun := newRun(run)
	newRun.Status.Conditions[0].Status = corev1.ConditionFalse
	return newRun
}

func withCancelled(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Status.Conditions[0].Reason = v1beta1.TaskRunSpecStatusCancelled
	return tr
}

func withRunCancelled(run *v1alpha1.Run) *v1alpha1.Run {
	run.Status.Conditions[0].Reason = v1alpha1.RunReasonCancelled
	return run
}

func withCancelledBySpec(tr *v1beta1.TaskRun) *v1beta1.TaskRun {
	tr.Spec.Status = v1beta1.TaskRunSpecStatusCancelled
	return tr
}

func withRunCancelledBySpec(run *v1alpha1.Run) *v1alpha1.Run {
	run.Spec.Status = v1alpha1.RunSpecStatusCancelled
	return run
}

func makeRetried(tr v1beta1.TaskRun) (newTr *v1beta1.TaskRun) {
	newTr = newTaskRun(tr)
	newTr = withRetries(newTr)
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

func withRunRetries(r *v1alpha1.Run) *v1alpha1.Run {
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}
}

func newRun(run v1alpha1.Run) *v1alpha1.Run {
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

var noRunStartedState = PipelineRunState{{
	PipelineTask: &pts[12],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask13",
	Run:          nil,
}, {
	PipelineTask: &pts[13],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask14",
	Run:          nil,
}}

var oneRunStartedState = PipelineRunState{{
	PipelineTask: &pts[12],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask13",
	Run:          makeRunStarted(runs[0]),
}, {
	PipelineTask: &pts[13],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask14",
	Run:          nil,
}}

var oneRunFinishedState = PipelineRunState{{
	PipelineTask: &pts[12],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask13",
	Run:          makeRunSucceeded(runs[0]),
}, {
	PipelineTask: &pts[13],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask14",
	Run:          nil,
}}

var oneRunFailedState = PipelineRunState{{
	PipelineTask: &pts[12],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask13",
	Run:          makeRunFailed(runs[0]),
}, {
	PipelineTask: &pts[13],
	CustomTask:   true,
	RunName:      "pipelinerun-mytask14",
	Run:          nil,
}}

var taskCancelled = PipelineRunState{{
	PipelineTask: &pts[4],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      withCancelled(makeRetried(trs[0])),
	ResolvedTaskResources: &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	},
}}

var runCancelled = PipelineRunState{{
	PipelineTask: &pts[12],
	RunName:      "pipelinerun-mytask13",
	Run:          withRunCancelled(newRun(runs[0])),
}}

var taskWithOptionalResourcesDeprecated = &v1beta1.Task{
	ObjectMeta: metav1.ObjectMeta{
		Name: "task",
	},
	Spec: v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Name: "step1",
		}},
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
		Steps: []v1beta1.Step{{
			Name: "step1",
		}},
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

func dagFromState(state PipelineRunState) (*dag.Graph, error) {
	pts := []v1beta1.PipelineTask{}
	for _, rprt := range state {
		pts = append(pts, *rprt.PipelineTask)
	}
	return dag.Build(v1beta1.PipelineTaskList(pts), v1beta1.PipelineTaskList(pts).Deps())
}

func TestIsSkipped(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    PipelineRunState
		expected map[string]bool
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
		name: "tasks-parent-failed",
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
		expected: map[string]bool{
			"mytask7": true,
		},
	}, {
		name: "tasks-parent-cancelled",
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
		expected: map[string]bool{
			"mytask7": true,
		},
	}, {
		name: "tasks-grandparent-failed",
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
		expected: map[string]bool{
			"mytask10": true,
		},
	}, {
		name: "tasks-parents-failed-passed",
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
		expected: map[string]bool{
			"mytask8": true,
		},
	}, {
		name: "task-failed-pipeline-stopping",
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
		expected: map[string]bool{
			"mytask7": true,
		},
	}, {
		name: "tasks-when-expressions-passed",
		state: PipelineRunState{{
			PipelineTask: &pts[9],
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			// attempted but skipped because of missing result in params from parent task mytask11 which was skipped
			PipelineTask: &v1beta1.PipelineTask{
				Name:    "mytask20",
				TaskRef: &v1beta1.TaskRef{Name: "task"},
				Params: []v1beta1.Param{{
					Name:  "commit",
					Value: *v1beta1.NewArrayOrString("$(tasks.mytask11.results.missingResult)"),
				}},
			},
			TaskRunName: "pipelinerun-resource-dependent-task-1",
			TaskRun:     nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			}
			for taskName, isSkipped := range tc.expected {
				rprt := stateMap[taskName]
				if rprt == nil {
					t.Fatalf("Could not get task %s from the state: %v", taskName, tc.state)
				}
				if d := cmp.Diff(isSkipped, rprt.Skip(&facts).IsSkipped); d != "" {
					t.Errorf("Didn't get expected isSkipped from task %s: %s", taskName, diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestIsFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		rprt ResolvedPipelineRunTask
		want bool
	}{{
		name: "taskrun not started",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
		},
		want: false,
	}, {
		name: "run not started",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun running",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeStarted(trs[0]),
		},
		want: false,
	}, {

		name: "run running",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			Run:          makeRunStarted(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun succeeded",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeSucceeded(trs[0]),
		},
		want: false,
	}, {

		name: "run succeeded",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			Run:          makeRunSucceeded(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun failed",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      makeFailed(trs[0]),
		},
		want: true,
	}, {

		name: "run failed",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			CustomTask:   true,
			Run:          makeRunFailed(runs[0]),
		},
		want: true,
	}, {
		name: "taskrun failed: retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      makeFailed(trs[0]),
		},
		want: false,
	}, {

		name: "run failed: retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			Run:          makeRunFailed(runs[0]),
		},
		want: false,
	}, {
		name: "taskrun failed: no retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withRetries(makeFailed(trs[0])),
		},
		want: true,
	}, {

		name: "run failed: no retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			CustomTask:   true,
			Run:          withRunRetries(makeRunFailed(runs[0])),
		},
		want: true,
	}, {
		name: "taskrun cancelled",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: true,
	}, {
		name: "taskrun cancelled but not failed",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			TaskRun:      withCancelled(newTaskRun(trs[0])),
		},
		want: false,
	}, {
		name: "run cancelled",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			Run:          withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "run cancelled but not failed",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task"},
			Run:          withRunCancelled(newRun(runs[0])),
			CustomTask:   true,
		},
		want: false,
	}, {
		name: "taskrun cancelled: retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(makeFailed(trs[0])),
		},
		want: true,
	}, {
		name: "run cancelled: retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			Run:          withRunCancelled(makeRunFailed(runs[0])),
			CustomTask:   true,
		},
		want: true,
	}, {
		name: "taskrun cancelled: no retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			TaskRun:      withCancelled(withRetries(makeFailed(trs[0]))),
		},
		want: true,
	}, {
		name: "run cancelled: no retries remaining",
		rprt: ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{Name: "task", Retries: 1},
			Run:          withRunCancelled(withRunRetries(makeRunFailed(runs[0]))),
			CustomTask:   true,
		},
		want: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rprt.isFailure(); got != tc.want {
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}, {
			// child task not skipped because parent is not yet done
			PipelineTask: &pts[11],
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			ResolvedTaskResources: &resources.ResolvedTaskResources{
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
			}
			for taskName, isSkipped := range tc.expected {
				rprt := stateMap[taskName]
				if rprt == nil {
					t.Fatalf("Could not get task %s from the state: %v", taskName, tc.state)
				}
				if d := cmp.Diff(isSkipped, rprt.skipBecauseParentTaskWasSkipped(&facts)); d != "" {
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

func TestGetResourcesFromBindings(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipeline",
			},
			Resources: []v1beta1.PipelineResourceBinding{
				gitSweetResourceBinding,
				{
					Name: "image-resource",
					ResourceSpec: &resourcev1alpha1.PipelineResourceSpec{
						Type: resourcev1alpha1.PipelineResourceTypeImage,
						Params: []v1beta1.ResourceParam{{
							Name:  "url",
							Value: "gcr.io/sven",
						}},
					},
				},
			},
		},
	}
	r := &resourcev1alpha1.PipelineResource{ObjectMeta: metav1.ObjectMeta{Name: "sweet-resource"}}
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
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipeline",
			},
			Resources: []v1beta1.PipelineResourceBinding{gitSweetResourceBinding},
		},
	}
	getResource := func(name string) (*resourcev1alpha1.PipelineResource, error) {
		return nil, kerrors.NewNotFound(resourcev1alpha1.Resource("pipelineresources"), name)
	}
	_, err := GetResourcesFromBindings(pr, getResource)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was missing but got no error")
	} else if !kerrors.IsNotFound(err) {
		t.Fatalf("GetResourcesFromBindings() = %v, wanted IsNotFound", err)
	}
}

func TestGetResourcesFromBindings_ErrorGettingResource(t *testing.T) {
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipeline",
			},
			Resources: []v1beta1.PipelineResourceBinding{gitSweetResourceBinding},
		},
	}
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

	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{gitDeclaredResource},
			Tasks: []v1beta1.PipelineTask{
				{
					Name:    "mytask1",
					TaskRef: &v1beta1.TaskRef{Name: "task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
					},
				},
				{
					Name:    "mytask2",
					TaskRef: &v1beta1.TaskRef{Name: "task"},
					Resources: &v1beta1.PipelineTaskResources{
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name:     "output1",
							Resource: "git-resource",
						}},
					},
				},
				{
					Name:    "mytask3",
					TaskRef: &v1beta1.TaskRef{Name: "task"},
					Resources: &v1beta1.PipelineTaskResources{
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name:     "output1",
							Resource: "git-resource",
						}},
					},
				},
				{
					Name: "mytask4",
					TaskSpec: &v1beta1.EmbeddedTask{
						TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{Name: "step1"}},
						},
					},
				},
			},
		},
	}

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
	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }

	pipelineState := PipelineRunState{}
	for _, task := range p.Spec.Tasks {
		ps, err := ResolvePipelineRunTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, task, providedResources)
		if err != nil {
			t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
		}
		pipelineState = append(pipelineState, ps)
	}
	expectedState := PipelineRunState{{
		PipelineTask: &p.Spec.Tasks[0],
		TaskRunName:  "pipelinerun-mytask1",
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
		TaskRunName:  "pipelinerun-mytask2",
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
		TaskRunName:  "pipelinerun-mytask3",
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
		TaskRunName:  "pipelinerun-mytask4",
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
	run := &v1alpha1.Run{ObjectMeta: metav1.ObjectMeta{Name: "run-exists-abcde"}}
	getRun := func(name string) (*v1alpha1.Run, error) {
		if name == "pipelinerun-run-exists" {
			return run, nil
		}
		return nil, kerrors.NewNotFound(v1beta1.Resource("run"), name)
	}
	pipelineState := PipelineRunState{}
	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	cfg.OnConfigChanged(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
		Data: map[string]string{
			"enable-custom-tasks": "true",
		},
	})
	ctx = cfg.ToContext(ctx)
	for _, task := range pts {
		ps, err := ResolvePipelineRunTask(ctx, pr, nopGetTask, nopGetTaskRun, getRun, task, nil)
		if err != nil {
			t.Fatalf("ResolvePipelineRunTask: %v", err)
		}
		pipelineState = append(pipelineState, ps)
	}

	expectedState := PipelineRunState{{
		PipelineTask: &pts[0],
		CustomTask:   true,
		RunName:      "pipelinerun-customtask",
		Run:          nil,
	}, {
		PipelineTask: &pts[1],
		CustomTask:   true,
		RunName:      "pipelinerun-customtask-spec",
		Run:          nil,
	}, {
		PipelineTask: &pts[2],
		CustomTask:   true,
		RunName:      "pipelinerun-run-exists",
		Run:          run,
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
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	pipelineState := PipelineRunState{}
	for _, task := range pts {
		ps, err := ResolvePipelineRunTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, task, providedResources)
		if err != nil {
			t.Errorf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
		}
		pipelineState = append(pipelineState, ps)
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
	pt := v1beta1.PipelineTask{
		Name:    "mytask1",
		TaskRef: &v1beta1.TaskRef{Name: "task"},
	}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	// Return an error when the Task is retrieved, as if it didn't exist
	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, error) {
		return nil, kerrors.NewNotFound(v1beta1.Resource("task"), name)
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) {
		return nil, kerrors.NewNotFound(v1beta1.Resource("taskrun"), name)
	}
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	_, err := ResolvePipelineRunTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, pt, providedResources)
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
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
					},
				}},
			},
		},
	}, {
		name: "output doesnt exist",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "mytask1",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
					},
				}},
			},
		},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun",
				},
			}
			pipelineState := PipelineRunState{}
			ps, err := ResolvePipelineRunTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, tt.p.Spec.Tasks[0], providedResources)
			if err == nil {
				t.Fatalf("Expected error when bindings are in incorrect state for Pipeline %s but got none: %s", p.ObjectMeta.Name, err)
			}
			pipelineState = append(pipelineState, ps)
		})
	}
}

func TestResolvePipelineRun_withExistingTaskRuns(t *testing.T) {
	names.TestingSeed()

	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{gitDeclaredResource},
			Tasks: []v1beta1.PipelineTask{{
				Name: "mytask-with-a-really-long-name-to-trigger-truncation",
				TaskRef: &v1beta1.TaskRef{
					Name: "task",
				},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name:     "input1",
						Resource: "git-resource",
					}},
				},
			}},
		},
	}

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
	taskrunStatus["pipelinerun-mytask-with-a-really-long-name-to-trigger-tru"] = &v1beta1.PipelineRunTaskRunStatus{
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
	getTask := func(_ context.Context, name string) (v1beta1.TaskObject, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	resolvedTask, err := ResolvePipelineRunTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, p.Spec.Tasks[0], providedResources)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedTask := &ResolvedPipelineRunTask{
		PipelineTask: &p.Spec.Tasks[0],
		TaskRunName:  "pipelinerun-mytask-with-a-really-long-name-to-trigger-tru",
		TaskRun:      nil,
		ResolvedTaskResources: &resources.ResolvedTaskResources{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs: map[string]*resourcev1alpha1.PipelineResource{
				"input1": r,
			},
			Outputs: map[string]*resourcev1alpha1.PipelineResource{},
		},
	}

	if d := cmp.Diff(resolvedTask, expectedTask, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected to get current pipeline state %v, but actual differed %s", expectedTask, diff.PrintWantGot(d))
	}
}

func TestResolvedPipelineRun_PipelineTaskHasOptionalResources(t *testing.T) {
	names.TestingSeed()
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "mytask1",
				TaskRef: &v1beta1.TaskRef{
					Name: "task",
				},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name:     "required-input",
						Resource: "git-resource",
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{{
						Name:     "required-output",
						Resource: "git-resource",
					}},
				},
			}},
		},
	}

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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, error) {
		return taskWithOptionalResourcesDeprecated, nil
	}
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }

	actualTask, err := ResolvePipelineRunTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, p.Spec.Tasks[0], providedResources)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedTask := &ResolvedPipelineRunTask{
		PipelineTask: &p.Spec.Tasks[0],
		TaskRunName:  "pipelinerun-mytask1",
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
	}

	if d := cmp.Diff(expectedTask, actualTask, cmpopts.IgnoreUnexported(v1beta1.TaskRunSpec{})); d != "" {
		t.Errorf("Expected to get current pipeline state %v, but actual differed %s", expectedTask, diff.PrintWantGot(d))
	}
}

func TestValidateResourceBindings(t *testing.T) {
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{gitDeclaredResource},
		},
	}
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "pipeline"},
			Resources:   []v1beta1.PipelineResourceBinding{gitSweetResourceBinding},
		},
	}
	err := ValidateResourceBindings(&p.Spec, pr)
	if err != nil {
		t.Fatalf("didn't expect error getting resources from bindings but got: %v", err)
	}
}

func TestValidateResourceBindings_Missing(t *testing.T) {
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{
				gitDeclaredResource,
				{
					Name: "image-resource",
					Type: resourcev1alpha1.PipelineResourceTypeImage,
				},
			},
		},
	}
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "pipeline"},
			Resources:   []v1beta1.PipelineResourceBinding{gitSweetResourceBinding},
		},
	}
	err := ValidateResourceBindings(&p.Spec, pr)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was missing but got no error")
	}
}

func TestGetResourcesFromBindings_Extra(t *testing.T) {
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines"},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{gitDeclaredResource},
		},
	}
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "pipeline"},
			Resources: []v1beta1.PipelineResourceBinding{
				gitSweetResourceBinding,
				{
					Name:        "image-resource",
					ResourceRef: &v1beta1.PipelineResourceRef{Name: "sweet-resource2"},
				},
			},
		},
	}
	err := ValidateResourceBindings(&p.Spec, pr)
	if err == nil {
		t.Fatalf("Expected error indicating `image-resource` was extra but got no error")
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
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
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
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
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
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
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

func TestValidateServiceaccountMapping(t *testing.T) {
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
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
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
				ServiceAccountNames: []v1beta1.PipelineRunSpecServiceAccountName{{
					TaskName:           "mytask1",
					ServiceAccountName: "default",
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
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
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
				ServiceAccountNames: []v1beta1.PipelineRunSpecServiceAccountName{{
					TaskName:           "myfinaltask1",
					ServiceAccountName: "default",
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
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "git-resource",
						}},
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
				ServiceAccountNames: []v1beta1.PipelineRunSpecServiceAccountName{{
					TaskName:           "wrongtask",
					ServiceAccountName: "default",
				}},
			},
		},
		wantErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.p.Spec
			err := ValidateServiceaccountMapping(&spec, tc.pr)
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

	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(_ context.Context, name string) (v1beta1.TaskObject, error) { return task, nil }
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
		_, err := ResolvePipelineRunTask(context.Background(), pr, getTask, getTaskRun, nopGetRun, pt, providedResources)
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
	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return nil, nil }
	getRun := func(name string) (*v1alpha1.Run, error) { return nil, nil }

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
		name: "custom both taskRef and taskSpec",
		pt: v1beta1.PipelineTask{
			TaskRef: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Sample",
			},
			TaskSpec: &v1beta1.EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "example.dev/v0",
					Kind:       "Sample",
				},
			},
		},
		want: false,
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
			cfg.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
				Data: map[string]string{
					"enable-custom-tasks": "true",
				},
			})
			ctx = cfg.ToContext(ctx)
			rprt, err := ResolvePipelineRunTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt, nil)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
			}
			got := rprt.IsCustomTask()
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
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "commit",
					Value: *v1beta1.NewArrayOrString("SHA2"),
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
			Params: []v1beta1.Param{{
				Name:  "commit",
				Value: *v1beta1.NewArrayOrString("$(tasks.dag-task.results.commit)"),
			}},
		},
	}, {
		PipelineTask: &v1beta1.PipelineTask{
			Name:    "final-task-2",
			TaskRef: &v1beta1.TaskRef{Name: "task"},
			Params: []v1beta1.Param{{
				Name:  "commit",
				Value: *v1beta1.NewArrayOrString("$(tasks.dag-task.results.missingResult)"),
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

	expected := map[string]bool{
		"final-task-1": false,
		"final-task-2": true,
		"final-task-3": false,
		"final-task-4": true,
		"final-task-5": false,
		"final-task-6": true,
	}

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

	facts := &PipelineRunFacts{
		State:           state,
		TasksGraph:      d,
		FinalTasksGraph: dfinally,
	}

	for i := range state {
		if i > 0 { // first one is a dag task that produces a result
			finallyTaskName := state[i].PipelineTask.Name
			if d := cmp.Diff(expected[finallyTaskName], state[i].IsFinallySkipped(facts).IsSkipped); d != "" {
				t.Fatalf("Didn't get expected isFinallySkipped from finally task %s: %s", finallyTaskName, diff.PrintWantGot(d))
			}
		}
	}
}

func TestResolvedPipelineRunTask_IsFinalTask(t *testing.T) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dag-task",
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "commit",
					Value: *v1beta1.NewArrayOrString("SHA2"),
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
			Params: []v1beta1.Param{{
				Name:  "commit",
				Value: *v1beta1.NewArrayOrString("$(tasks.dag-task.results.commit)"),
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
	}

	finallyTaskName := state[1].PipelineTask.Name
	if d := cmp.Diff(true, state[1].IsFinalTask(facts)); d != "" {
		t.Fatalf("Didn't get expected isFinallySkipped from finally task %s: %s", finallyTaskName, diff.PrintWantGot(d))
	}

}

func TestGetTaskRunName(t *testing.T) {
	prName := "pipeline-run"
	taskRunsStatus := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"taskrun-for-task1": {
			PipelineTaskName: "task1",
		},
	}
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
			trNameFromTRStatus := GetTaskRunName(taskRunsStatus, nil, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, trNameFromTRStatus); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
			trNameFromChildRefs := GetTaskRunName(nil, childRefs, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, trNameFromChildRefs); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetTaskRunNames(t *testing.T) {
	prName := "mypipelinerun"
	taskRunsStatus := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"mypipelinerun-mytask-0": {
			PipelineTaskName: "mytask",
		},
		"mypipelinerun-mytask-1": {
			PipelineTaskName: "mytask",
		},
	}

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
			"pipeline-run-0123456789-01234569d54677e88e96776942290e00b578ca5",
			"pipeline-run-0123456789-01234563c0313c59d28c85a2c2b3fd3b17a9514",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testPrName := prName
			if tc.prName != "" {
				testPrName = tc.prName
			}
			trNameFromTRStatus := GetNamesofTaskRuns(taskRunsStatus, nil, tc.ptName, testPrName, 2)
			if d := cmp.Diff(tc.wantTrNames, trNameFromTRStatus); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
			trNameFromChildRefs := GetNamesofTaskRuns(nil, childRefs, tc.ptName, testPrName, 2)
			if d := cmp.Diff(tc.wantTrNames, trNameFromChildRefs); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetRunName(t *testing.T) {
	prName := "pipeline-run"
	runsStatus := map[string]*v1beta1.PipelineRunRunStatus{
		"run-for-task1": {
			PipelineTaskName: "task1",
		},
	}
	childRefs := []v1beta1.ChildStatusReference{{
		TypeMeta:         runtime.TypeMeta{Kind: "Run"},
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
			rnFromRunsStatus := getRunName(runsStatus, nil, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, rnFromRunsStatus); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
			rnFromChildRefs := getRunName(nil, childRefs, tc.ptName, testPrName)
			if d := cmp.Diff(tc.wantTrName, rnFromChildRefs); d != "" {
				t.Errorf("GetTaskRunName: %s", diff.PrintWantGot(d))
			}
			rnFromBoth := getRunName(runsStatus, childRefs, tc.ptName, testPrName)
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
	getTask := func(ctx context.Context, name string) (v1beta1.TaskObject, error) { return task, nil }
	getTaskRun := func(name string) (*v1beta1.TaskRun, error) { return &trs[0], nil }
	getRun := func(name string) (*v1alpha1.Run, error) { return &runs[0], nil }

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
			Matrix: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}},
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
			Matrix: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}},
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
			rprt, err := ResolvePipelineRunTask(ctx, pr, getTask, getTaskRun, getRun, tc.pt, nil)
			if err != nil {
				t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
			}
			got := rprt.IsMatrixed()
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("IsMatrixed: %s", diff.PrintWantGot(d))
			}
		})
	}
}
