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
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
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
		Namespace: "namespace",
		Name:      "task",
	},
	Spec: v1alpha1.TaskSpec{},
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

func TestGetNextTask_NoneStarted(t *testing.T) {
	noneStartedState := []*PipelineRunTaskRun{{
		Task:         task,
		PipelineTask: &pts[0],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      nil,
	}, {
		Task:         task,
		PipelineTask: &pts[1],
		TaskRunName:  "pipelinerun-mytask2",
		TaskRun:      nil,
	}}
	// TODO: one started
	firstFinishedState := []*PipelineRunTaskRun{{
		Task:         task,
		PipelineTask: &pts[0],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      &trs[0],
	}, {
		Task:         task,
		PipelineTask: &pts[1],
		TaskRunName:  "pipelinerun-mytask2",
		TaskRun:      nil,
	}}
	// TODO: all finished
	tcs := []struct {
		name         string
		state        []*PipelineRunTaskRun
		expectedTask *PipelineRunTaskRun
	}{
		{
			name:         "no-tasks-started",
			state:        noneStartedState,
			expectedTask: noneStartedState[0],
		},
		{
			name:         "first-task-finished",
			state:        firstFinishedState,
			expectedTask: firstFinishedState[1],
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			nextTask := GetNextTask(tc.state)
			if d := cmp.Diff(nextTask, tc.expectedTask); d != "" {
				t.Fatalf("Expected to indicate first task should be run, but different state returned: %s", d)
			}
		})
	}
}

func TestGetPipelineState(t *testing.T) {
	getTask := func(namespace, name string) (*v1alpha1.Task, error) {
		return task, nil
	}
	getTaskRun := func(namespace, name string) (*v1alpha1.TaskRun, error) {
		// We'll make it so that only the first Task has started running
		if name == "pipelinerun-mytask1" {
			return &trs[0], nil
		}
		return nil, errors.NewNotFound(v1alpha1.Resource("taskrun"), name)
	}
	pipelineState, err := GetPipelineState(getTask, getTaskRun, p, "pipelinerun")
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedState := []*PipelineRunTaskRun{{
		Task:         task,
		PipelineTask: &pts[0],
		TaskRunName:  "pipelinerun-mytask1",
		TaskRun:      &trs[0],
	}, {
		Task:         task,
		PipelineTask: &pts[1],
		TaskRunName:  "pipelinerun-mytask2",
		TaskRun:      nil,
	}}
	if d := cmp.Diff(pipelineState, expectedState); d != "" {
		t.Fatalf("Expected to get current pipeline state %v, but actual differed: %s", expectedState, d)
	}
}

func TestGetPipelineState_TaskDoesntExist(t *testing.T) {
	getTask := func(namespace, name string) (*v1alpha1.Task, error) {
		return nil, fmt.Errorf("Task %s doesn't exist", name)
	}
	getTaskRun := func(namespace, name string) (*v1alpha1.TaskRun, error) {
		return nil, nil
	}
	_, err := GetPipelineState(getTask, getTaskRun, p, "pipelinerun")
	if err == nil {
		t.Fatalf("Expected error getting non-existent Tasks for Pipeline %s but got none", p.Name)
	}
}
