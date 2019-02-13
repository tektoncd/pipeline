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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	tb "github.com/knative/build-pipeline/test/builder"
	"github.com/knative/build-pipeline/test/names"
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
var oneStartedState = []*ResolvedPipelineRunTask{{
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
var oneFinishedState = []*ResolvedPipelineRunTask{{
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
var oneFailedState = []*ResolvedPipelineRunTask{{
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
var firstFinishedState = []*ResolvedPipelineRunTask{{
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
		TaskSpec: &v1alpha1.TaskSpec{},
	},
}}
var allFinishedState = []*ResolvedPipelineRunTask{{
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

func TestGetNextTask(t *testing.T) {
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
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return nil, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return r, nil }

	pipelineState, err := ResolvePipelineRun(pr, getTask, getClusterTask, getResource, p.Spec.Tasks, providedResources)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedState := []*ResolvedPipelineRunTask{{
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
		t.Fatalf("Expected to get current pipeline state %v, but actual differed: %s", expectedState, d)
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
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return clustertask, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return nil, fmt.Errorf("should not get called") }
	pr := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	pipelineState, err := ResolvePipelineRun(pr, getTask, getClusterTask, getResource, pts, providedResources)
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
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return nil, fmt.Errorf("should not get called") }
	pr := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun",
		},
	}
	_, err := ResolvePipelineRun(pr, getTask, getClusterTask, getResource, pts, providedResources)
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
	getClusterTask := func(name string) (v1alpha1.TaskInterface, error) { return clustertask, nil }
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return nil, fmt.Errorf("shouldnt be called") }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun",
				},
			}
			_, err := ResolvePipelineRun(pr, getTask, getClusterTask, getResource, tt.p.Spec.Tasks, providedResources)
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
			_, err := ResolvePipelineRun(pr, getTask, getClusterTask, getResource, tt.p.Spec.Tasks, providedResources)
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
			c := GetPipelineConditionStatus("somepipelinerun", tc.state, zap.NewNop().Sugar(), &metav1.Time{time.Now()},
				nil)
			if c.Status != tc.expectedStatus {
				t.Fatalf("Expected to get status %s but got %s for state %v", tc.expectedStatus, c.Status, tc.state)
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
		tb.PipelineTask("mytask1", "task",
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
	taskrunStatus := map[string]v1alpha1.TaskRunStatus{}
	taskrunStatus["pipelinerun-mytask1-9l9zj"] = v1alpha1.TaskRunStatus{}

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
	getResource := func(name string) (*v1alpha1.PipelineResource, error) { return r, nil }

	pipelineState, err := ResolvePipelineRun(pr, getTask, getClusterTask, getResource, p.Spec.Tasks, providedResources)
	if err != nil {
		t.Fatalf("Error getting tasks for fake pipeline %s: %s", p.ObjectMeta.Name, err)
	}
	expectedState := []*ResolvedPipelineRunTask{{
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
	}}

	if d := cmp.Diff(pipelineState, expectedState, cmpopts.IgnoreUnexported(v1alpha1.TaskRunSpec{})); d != "" {
		t.Fatalf("Expected to get current pipeline state %v, but actual differed: %s", expectedState, d)
	}
}
