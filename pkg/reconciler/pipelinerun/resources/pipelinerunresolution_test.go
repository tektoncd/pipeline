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
	tbv1alpha1 "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
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
	"k8s.io/apimachinery/pkg/selection"
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
	}, {
		name:     "tasks-when-expressions-passed",
		taskName: "mytask10",
		state: PipelineRunState{{
			PipelineTask: &pts[9],
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		expected: false,
	}, {
		name:     "tasks-when-expression-failed",
		taskName: "mytask11",
		state: PipelineRunState{{
			PipelineTask: &pts[10],
			TaskRunName:  "pipelinerun-guardedtask",
			TaskRun:      nil,
			ResolvedTaskResources: &resources.ResolvedTaskResources{
				TaskSpec: &task.Spec,
			},
		}},
		expected: true,
	}, {
		name:     "when-expression-task-but-without-parent-done",
		taskName: "mytask12",
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
		expected: false,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d, err := DagFromState(tc.state)
			if err != nil {
				t.Fatalf("Could not get a dag from the TC state %#v: %v", tc.state, err)
			}
			stateMap := tc.state.ToMap()
			rprt := stateMap[tc.taskName]
			if rprt == nil {
				t.Fatalf("Could not get task %s from the state: %v", tc.taskName, tc.state)
			}
			facts := PipelineRunFacts{
				State:           tc.state,
				TasksGraph:      d,
				FinalTasksGraph: &dag.Graph{},
			}
			isSkipped := rprt.Skip(&facts)
			if d := cmp.Diff(isSkipped, tc.expected); d != "" {
				t.Errorf("Didn't get expected isSkipped %s", diff.PrintWantGot(d))
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
			tb.PipelineTaskSpec(v1beta1.TaskSpec{
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
	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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
	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) {
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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
	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) {
		return taskWithOptionalResourcesDeprecated, nil
	}
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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
		Params: []v1beta1.Param{{
			Name: "path", Value: *v1beta1.NewArrayOrString("$(params.path)"),
		}, {
			Name: "image", Value: *v1beta1.NewArrayOrString("$(params.image)"),
		}},
	}

	ptc2 := v1beta1.PipelineTaskCondition{
		ConditionRef: "always-true",
		Params: []v1beta1.Param{{
			Name: "path", Value: *v1beta1.NewArrayOrString("$(params.path-test)"),
		}, {
			Name: "image", Value: *v1beta1.NewArrayOrString("$(params.image-test)"),
		}},
	}

	pts := []v1beta1.PipelineTask{{
		Name:       "mytask1",
		TaskRef:    &v1beta1.TaskRef{Name: "task"},
		Conditions: []v1beta1.PipelineTaskCondition{ptc1, ptc2},
	}}
	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
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

func TestValidateWorkspaceBindingsWithValidWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *v1beta1.PipelineSpec
		run  *v1beta1.PipelineRun
		err  string
	}{{
		name: "include required workspace",
		spec: &v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
		},
		run: &v1beta1.PipelineRun{
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
		run: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateWorkspaceBindings(tc.spec, tc.run); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidateWorkspaceBindingsWithInvalidWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *v1beta1.PipelineSpec
		run  *v1beta1.PipelineRun
		err  string
	}{{
		name: "missing required workspace",
		spec: &v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name: "foo",
			}},
		},
		run: &v1beta1.PipelineRun{
			Spec: v1beta1.PipelineRunSpec{
				Workspaces: []v1beta1.WorkspaceBinding{},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateWorkspaceBindings(tc.spec, tc.run); err == nil {
				t.Fatalf("Expected error indicating `foo` workspace was not provided but got no error")
			}
		})
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

func TestResolvePipeline_WhenExpressions(t *testing.T) {
	names.TestingSeed()
	tName1 := "pipelinerun-mytask1-9l9zj-always-true-mz4c7"

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

	pts := []v1beta1.PipelineTask{{
		Name:            "mytask1",
		TaskRef:         &v1beta1.TaskRef{Name: "task"},
		WhenExpressions: []v1beta1.WhenExpression{ptwe1},
	}}

	providedResources := map[string]*resourcev1alpha1.PipelineResource{}

	getTask := func(ctx context.Context, name string) (v1beta1.TaskInterface, error) { return task, nil }
	getClusterTask := func(name string) (v1beta1.TaskInterface, error) { return nil, errors.New("should not get called") }
	getCondition := func(name string) (*v1alpha1.Condition, error) { return &condition, nil }
	pr := v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}

	getTaskRun := func(name string) (*v1beta1.TaskRun, error) {
		switch name {
		case "pipelinerun-mytask1-9l9zj-always-true-0-mz4c7":
			return t1, nil
		case "pipelinerun-mytask1-9l9zj":
			return &trs[0], nil
		}
		return nil, fmt.Errorf("getTaskRun called with unexpected name %s", name)
	}

	t.Run("When Expressions exist", func(t *testing.T) {
		_, err := ResolvePipelineRun(context.Background(), pr, getTask, getTaskRun, getClusterTask, getCondition, pts, providedResources)
		if err != nil {
			t.Fatalf("Did not expect error when resolving PipelineRun: %v", err)
		}
	})
}
