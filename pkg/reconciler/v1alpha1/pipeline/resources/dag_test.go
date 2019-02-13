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

package pipeline

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuild(t *testing.T) {
	a := v1alpha1.PipelineTask{Name: "a"}
	b := v1alpha1.PipelineTask{Name: "b"}
	c := v1alpha1.PipelineTask{Name: "c"}
	xDependsOnA := v1alpha1.PipelineTask{
		Name: "x",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	yDependsOnAB := v1alpha1.PipelineTask{
		Name: "y",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"b", "a"}}},
		},
	}
	zDependsOnX := v1alpha1.PipelineTask{
		Name: "z",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"x"}}},
		},
	}
	aDependsOnZ := v1alpha1.PipelineTask{
		Name: "a",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"z"}}},
		},
	}
	dDependsOnA := v1alpha1.PipelineTask{
		Name: "d",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	eDependsOnA := v1alpha1.PipelineTask{
		Name: "e",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	fDependsOnDAndE := v1alpha1.PipelineTask{
		Name: "f",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"d", "e"}}},
		},
	}
	gDependOnF := v1alpha1.PipelineTask{
		Name: "g",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"f"}}},
		},
	}
	selfLink := v1alpha1.PipelineTask{
		Name: "a",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	invalidTask := v1alpha1.PipelineTask{
		Name: "a",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"none"}}},
		},
	}
	nodeX := &Node{Task: xDependsOnA, Prev: []*Node{{Task: a}}}

	tcs := []struct {
		name        string
		spec        v1alpha1.PipelineSpec
		expectedDAG *DAG
		shdErr      bool
		expectedErr string
	}{
		{
			// This test make sure we can create a Pipeline with no links between any Tasks
			// (all tasks run in parallel)
			//    a   b   c
			name: "parallel-pipeline",
			spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, b, c}},
			expectedDAG: &DAG{
				Nodes: map[string]*Node{
					"a": {Task: a},
					"b": {Task: b},
					"c": {Task: c},
				},
			},
			shdErr:      false,
			expectedErr: "",
		}, {
			// This test make sure we can create a more complex Pipeline:
			//      a  b   c
			//      |  |
			//      x  |
			//     /\ /
			//    z  y
			name: "complex-pipeline",
			spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, xDependsOnA, yDependsOnAB, zDependsOnX, b, c}},
			expectedDAG: &DAG{
				Nodes: map[string]*Node{
					"a": {Task: a},
					"b": {Task: b},
					"c": {Task: c},
					"x": {Task: xDependsOnA, Prev: []*Node{{Task: a}}},
					"y": {Task: yDependsOnAB, Prev: []*Node{{Task: b}, {Task: a}}},
					"z": {Task: zDependsOnX, Prev: []*Node{nodeX}},
				},
			},
			shdErr:      false,
			expectedErr: "",
		}, {
			name:        "self-link",
			spec:        v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{selfLink}},
			expectedDAG: nil,
			shdErr:      true,
			expectedErr: ` "self-link" is invalid: : Internal error: cycle detected; task "a" depends on itself`,
		}, {
			name:        "cycle-2",
			spec:        v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{xDependsOnA, zDependsOnX, aDependsOnZ}},
			expectedDAG: nil,
			shdErr:      true,
			expectedErr: ` "cycle-2" is invalid: : Internal error: cycle detected; a -> x -> z -> a `,
		}, {
			name:        "duplicate-tasks",
			spec:        v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, a}},
			expectedDAG: nil,
			shdErr:      true,
			expectedErr: ` "duplicate-tasks" is invalid: spec.tasks.name: Duplicate value: "a"`,
		}, {
			name:        "invalid-task-name",
			spec:        v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{invalidTask}},
			expectedDAG: nil,
			shdErr:      true,
			expectedErr: ` "invalid-task-name" is invalid: spec.tasks.name: Not found: "none"`,
		}, {
			// This test make sure we don't detect cycle (A -> B -> B -> …) when there is not
			// The graph looks like the following.
			//   a
			//  / \
			// d   e
			//  \ /
			//   f
			//   |
			//   g
			// This means we "visit" a twice, from two different path ; but there is no cycle.
			name: "no-cycle",
			spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, dDependsOnA, eDependsOnA, fDependsOnDAndE, gDependOnF}},
			expectedDAG: &DAG{
				Nodes: map[string]*Node{
					"a": {Task: a},
					"d": {Task: dDependsOnA, Prev: []*Node{{Task: a}}},
					"e": {Task: eDependsOnA, Prev: []*Node{{Task: a}}},
					"f": {Task: fDependsOnDAndE, Prev: []*Node{{Task: dDependsOnA, Prev: []*Node{{Task: a}}}, {Task: eDependsOnA, Prev: []*Node{{Task: a}}}}},
					"g": {Task: gDependOnF, Prev: []*Node{{Task: fDependsOnDAndE, Prev: []*Node{{Task: dDependsOnA, Prev: []*Node{{Task: a}}}, {Task: eDependsOnA, Prev: []*Node{{Task: a}}}}}}},
				},
			},
			shdErr:      false,
			expectedErr: "",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			p := &v1alpha1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      tc.name,
				},
				Spec: tc.spec,
			}
			g, err := Build(p)
			if hasErr(err) != tc.shdErr {
				t.Errorf("expected to see an err %t found %s", tc.shdErr, err)
			}
			if d := cmp.Diff(tc.expectedDAG, g); d != "" {
				t.Errorf("expected to see no diff in DAG but saw diff %s", cmp.Diff(tc.expectedDAG, g))
			}
			if tc.expectedErr != "" {
				if tc.expectedErr != err.Error() {
					t.Errorf("expected to see error message \n%s \ngot\n%s", tc.expectedErr, err.Error())
				}
			}
		})
	}
}

func TestGetSchedulable(t *testing.T) {
	//  b     a
	//  |    / \
	//  |   |   x
	//  |   | / |
	//  |   y   |
	//   \ /    z
	//    w
	g := new()
	g.Nodes["a"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "a"},
	}
	g.Nodes["b"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "b"},
	}
	g.Nodes["x"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "x"},
		Prev: []*Node{g.Nodes["a"]},
	}
	g.Nodes["y"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "y"},
		Prev: []*Node{g.Nodes["a"], g.Nodes["x"]},
	}
	g.Nodes["z"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "z"},
		Prev: []*Node{g.Nodes["x"]},
	}
	g.Nodes["w"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "w"},
		Prev: []*Node{g.Nodes["y"], g.Nodes["b"]},
	}
	tcs := []struct {
		name          string
		finished      []string
		expectedTasks []v1alpha1.PipelineTask
	}{{
		name:          "nothing-done",
		finished:      []string{},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "a"}, {Name: "b"}},
	}, {
		name:          "a-done",
		finished:      []string{"a"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "x"}},
	}, {
		name:          "b-done",
		finished:      []string{"b"},
		expectedTasks: []v1alpha1.PipelineTask{},
	}, {
		name:          "a-and-b-done",
		finished:      []string{"a", "b"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "x"}},
	}, {
		// TODO: should this return b also?
		// try adding something between b and w maybe to be sure
		name: "x-done",
		// TODO: if ix is done, shouldnt a be done also?
		finished:      []string{"x"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "z"}},
	}, {
		// TODO: should this return b also?
		name:          "a-x-done",
		finished:      []string{"a", "x"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "y"}, {Name: "z"}},
	}, {
		name:          "a-x-b-done",
		finished:      []string{"a", "x", "b"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "y"}, {Name: "z"}},
	}, {
		name:          "a-x-y-done",
		finished:      []string{"a", "x", "y"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "z"}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(sortPipelineTask(g.GetSchedulable(tc.finished...)), tc.expectedTasks); d != "" {
				t.Errorf("incorrect dependences for no task. diff %s", d)
			}
		})
	}
}

func sortPipelineTask(tasks []v1alpha1.PipelineTask) []v1alpha1.PipelineTask {
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Name < tasks[j].Name
	})
	return tasks
}

// hasErr returns true if err is not nil
func hasErr(err error) bool {
	return err != nil
}
