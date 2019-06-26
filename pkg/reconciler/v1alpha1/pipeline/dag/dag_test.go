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

package dag

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

func testGraph(t *testing.T) *v1alpha1.DAG {
	//  b     a
	//  |    / \
	//  |   |   x
	//  |   | / |
	//  |   y   |
	//   \ /    z
	//    w
	t.Helper()
	g, err := v1alpha1.BuildDAG([]v1alpha1.PipelineTask{
		{
			Name: "a",
		},
		{
			Name: "b",
		},
		{
			Name:     "w",
			RunAfter: []string{"b", "y"},
		},
		{
			Name:     "x",
			RunAfter: []string{"a"},
		},
		{
			Name:     "y",
			RunAfter: []string{"a", "x"},
		},
		{
			Name:     "z",
			RunAfter: []string{"x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestGetSchedulable(t *testing.T) {
	g := testGraph(t)
	tcs := []struct {
		name          string
		finished      []string
		expectedTasks map[string]v1alpha1.PipelineTask
	}{{
		name:     "nothing-done",
		finished: []string{},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"a": {Name: "a"},
			"b": {Name: "b"},
		},
	}, {
		name:     "a-done",
		finished: []string{"a"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"b": {Name: "b"},
			"x": {Name: "x"},
		},
	}, {
		name:     "b-done",
		finished: []string{"b"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"a": {Name: "a"},
		},
	}, {
		name:     "a-and-b-done",
		finished: []string{"a", "b"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"x": {Name: "x"},
		},
	}, {
		name:     "a-x-done",
		finished: []string{"a", "x"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"b": {Name: "b"},
			"y": {Name: "y"},
			"z": {Name: "z"},
		},
	}, {
		name:     "a-x-b-done",
		finished: []string{"a", "x", "b"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"y": {Name: "y"},
			"z": {Name: "z"},
		},
	}, {
		name:     "a-x-y-done",
		finished: []string{"a", "x", "y"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"b": {Name: "b"},
			"z": {Name: "z"},
		},
	}, {
		name:     "a-x-y-done",
		finished: []string{"a", "x", "y"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"b": {Name: "b"},
			"z": {Name: "z"},
		},
	}, {
		name:     "a-x-y-b-done",
		finished: []string{"a", "x", "y", "b"},
		expectedTasks: map[string]v1alpha1.PipelineTask{
			"w": {Name: "w"},
			"z": {Name: "z"},
		},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := GetSchedulable(g, tc.finished...)
			if err != nil {
				t.Fatalf("Didn't expect error when getting next tasks for %v but got %v", tc.finished, err)
			}
			if d := cmp.Diff(tasks, tc.expectedTasks, cmpopts.IgnoreFields(v1alpha1.PipelineTask{}, "RunAfter")); d != "" {
				t.Errorf("expected that with %v done, %v would be ready to schedule but was different: %s", tc.finished, tc.expectedTasks, d)
			}
		})
	}
}

func TestGetSchedulable_Invalid(t *testing.T) {
	g := testGraph(t)
	tcs := []struct {
		name     string
		finished []string
	}{{
		// x can't be completed on its own b/c it depends on a
		name:     "only-x",
		finished: []string{"x"},
	}, {
		// y can't be completed on its own b/c it depends on a and x
		name:     "only-y",
		finished: []string{"y"},
	}, {
		// w can't be completed on its own b/c it depends on y and b
		name:     "only-w",
		finished: []string{"w"},
	}, {
		name:     "only-y-and-x",
		finished: []string{"y", "x"},
	}, {
		name:     "only-y-and-w",
		finished: []string{"y", "w"},
	}, {
		name:     "only-x-and-w",
		finished: []string{"x", "w"},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GetSchedulable(g, tc.finished...)
			if err == nil {
				t.Fatalf("Expected error for invalid done tasks %v but got none", tc.finished)
			}
		})
	}
}
