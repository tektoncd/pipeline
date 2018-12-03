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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
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

func newTaskRun(tr v1alpha1.TaskRun) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			Name:      tr.Name,
		},
		Spec: tr.Spec,
		Status: v1alpha1.TaskRunStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type: duckv1alpha1.ConditionSucceeded,
			}},
		},
	}
}

var noneStartedState = []*ResolvedPipelineRunTask{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      nil,
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}}
var oneStartedState = []*ResolvedPipelineRunTask{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeStarted(trs[0]),
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}}
var oneFinishedState = []*ResolvedPipelineRunTask{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}}
var oneFailedState = []*ResolvedPipelineRunTask{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeFailed(trs[0]),
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}}
var firstFinishedState = []*ResolvedPipelineRunTask{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      nil,
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &v1alpha1.TaskSpec{},
	},
}}
var allFinishedState = []*ResolvedPipelineRunTask{{
	PipelineTask: &pts[0],
	TaskRunName:  "pipelinerun-mytask1",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}, {
	PipelineTask: &pts[1],
	TaskRunName:  "pipelinerun-mytask2",
	TaskRun:      makeSucceeded(trs[0]),
	ResolvedTaskRun: &resources.ResolvedTaskRun{
		TaskSpec: &task.Spec,
	},
}}

func TestGetNextTask_NoneStarted(t *testing.T) {
	tcs := []struct {
		name         string
		state        []*ResolvedPipelineRunTask
		expectedTask *ResolvedPipelineRunTask
	}{
		{
			name:         "no-tasks-started",
			state:        noneStartedState,
			expectedTask: noneStartedState[0],
		},
		{
			name:         "one-task-started",
			state:        oneStartedState,
			expectedTask: nil,
		},
		{
			name:         "one-task-finished",
			state:        oneFinishedState,
			expectedTask: oneFinishedState[1],
		},
		{
			name:         "one-task-failed",
			state:        oneFailedState,
			expectedTask: nil,
		},
		{
			name:         "first-task-finished",
			state:        firstFinishedState,
			expectedTask: firstFinishedState[1],
		},
		{
			name:         "all-finished",
			state:        allFinishedState,
			expectedTask: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			nextTask := GetNextTask("somepipelinerun", tc.state, zap.NewNop().Sugar())
			if d := cmp.Diff(nextTask, tc.expectedTask); d != "" {
				t.Fatalf("Expected to indicate first task should be run, but different state returned: %s", d)
			}
		})
	}
}

func TestResolvePipelineRun(t *testing.T) {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "pipeline",
			},
			// The Task "task" doesn't actually take any inputs or outputs, but validating
			// that is not done as part of Run resolution
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "mytask1",
				Inputs: []v1alpha1.TaskResourceBinding{{
					Name: "input1",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "someresource",
					},
				}},
			}, {
				Name: "mytask2",
				Outputs: []v1alpha1.TaskResourceBinding{{
					Name: "output1",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "someresource",
					},
				}},
			}},
		},
	}

	r := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "someresource",
		},
	}

	getTask := func(name string) (*v1alpha1.Task, error) { return task, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return r, nil }

	pipelineState, err := ResolvePipelineRun(getTask, getResource, p, pr)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedState := []*ResolvedPipelineRunTask{{
		PipelineTask: &pts[0],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      nil,
		ResolvedTaskRun: &resources.ResolvedTaskRun{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs: map[string]*v1alpha1.PipelineResource{
				"input1": r,
			},
			Outputs: map[string]*v1alpha1.PipelineResource{},
		},
	}, {
		PipelineTask: &pts[1],
		TaskRunName:  "pipelinerun-mytask2",
		TaskRun:      nil,
		ResolvedTaskRun: &resources.ResolvedTaskRun{
			TaskName: task.Name,
			TaskSpec: &task.Spec,
			Inputs:   map[string]*v1alpha1.PipelineResource{},
			Outputs: map[string]*v1alpha1.PipelineResource{
				"output1": r,
			},
		},
	}}
	if d := cmp.Diff(pipelineState, expectedState, cmpopts.IgnoreUnexported(v1alpha1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected to get current pipeline state %v, but actual differed: %s", expectedState, d)
	}
}

func TestResolvePipelineRun_TaskDoesntExist(t *testing.T) {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "pipeline",
			},
		},
	}
	getTask := func(name string) (*v1alpha1.Task, error) {
		return nil, errors.NewNotFound(v1alpha1.Resource("task"), name)
	}
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return nil, fmt.Errorf("should not get called") }

	_, err := ResolvePipelineRun(getTask, getResource, p, pr)
	if err == nil {
		t.Fatalf("Expected error getting non-existent Tasks for Pipeline %s but got none", p.Name)
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("Expected same error type returned by func for non-existent Task for Pipeline %s but got %s", p.Name, err)
	}
}

func TestResolveTaskRuns_AllStarted(t *testing.T) {
	state := []*ResolvedPipelineRunTask{{
		TaskRunName: "pipelinerun-mytask1",
	}, {
		TaskRunName: "pipelinerun-mytask2",
	}}
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) {
		if name == "pipelinerun-mytask1" {
			return &trs[0], nil
		}
		if name == "pipelinerun-mytask2" {
			return &trs[1], nil
		}
		return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
	}
	err := ResolveTaskRuns(getTaskRun, state)
	if err != nil {
		t.Fatalf("Didn't expect error resolving taskruns but got %v", err)
	}
	if state[0].TaskRun != &trs[0] {
		t.Errorf("Expected first task to resolve to first taskrun but was %v", state[0].TaskRun)
	}
	if state[1].TaskRun != &trs[1] {
		t.Errorf("Expected second task to resolve to second taskrun but was %v", state[1].TaskRun)
	}
}

func TestResolveTaskRuns_SomeStarted(t *testing.T) {
	state := []*ResolvedPipelineRunTask{{
		TaskRunName: "pipelinerun-mytask1",
	}, {
		TaskRunName: "pipelinerun-mytask2",
	}}
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) {
		// only the first has started
		if name == "pipelinerun-mytask1" {
			return &trs[0], nil
		}
		return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
	}
	err := ResolveTaskRuns(getTaskRun, state)
	if err != nil {
		t.Fatalf("Didn't expect error resolving taskruns but got %v", err)
	}
	if state[0].TaskRun != &trs[0] {
		t.Errorf("Expected first task to resolve to first taskrun but was %v", state[0].TaskRun)
	}
	if state[1].TaskRun != nil {
		t.Errorf("Expected second task to not resolve but was %v", state[1].TaskRun)
	}
}

func TestResolveTaskRuns_NoneStarted(t *testing.T) {
	state := []*ResolvedPipelineRunTask{{
		TaskRunName: "pipelinerun-mytask1",
	}, {
		TaskRunName: "pipelinerun-mytask2",
	}}
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) {
		return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
	}
	err := ResolveTaskRuns(getTaskRun, state)
	if err != nil {
		t.Fatalf("Didn't expect error resolving taskruns but got %v", err)
	}
	if state[0].TaskRun != nil {
		t.Errorf("Expected first task to not resolve but was %v", state[0].TaskRun)
	}
	if state[1].TaskRun != nil {
		t.Errorf("Expected second task to not resolve but was %v", state[1].TaskRun)
	}
}

func TestResolveTaskRuns_Error(t *testing.T) {
	state := []*ResolvedPipelineRunTask{{
		TaskRunName: "pipelinerun-mytask1",
	}}
	getTaskRun := func(name string) (*v1alpha1.TaskRun, error) {
		return nil, fmt.Errorf("something has gone wrong")
	}
	err := ResolveTaskRuns(getTaskRun, state)
	if err == nil {
		t.Fatalf("Expected to get an error when unable to resolve task runs")
	}
}

func TestGetPipelineConditionStatus(t *testing.T) {
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
			name:           "first-task-finished",
			state:          firstFinishedState,
			expectedStatus: corev1.ConditionUnknown,
		},
		{
			name:           "all-finished",
			state:          allFinishedState,
			expectedStatus: corev1.ConditionTrue,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := GetPipelineConditionStatus("somepipelinerun", tc.state, zap.NewNop().Sugar())
			if c.Status != tc.expectedStatus {
				t.Fatalf("Expected to get status %s but got %s for state %v", tc.expectedStatus, c.Status, tc.state)
			}
		})
	}
}
