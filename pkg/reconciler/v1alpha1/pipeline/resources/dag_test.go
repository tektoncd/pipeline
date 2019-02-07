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
			name: "linear-pipeline",
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
	a := v1alpha1.PipelineTask{Name: "a"}
	b := v1alpha1.PipelineTask{Name: "b"}
	w := v1alpha1.PipelineTask{
		Name: "w",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"b", "y"}}},
		},
	}
	x := v1alpha1.PipelineTask{
		Name: "x",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	y := v1alpha1.PipelineTask{
		Name: "y",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"x", "a"}}},
		},
	}
	z := v1alpha1.PipelineTask{
		Name: "z",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"x"}}},
		},
	}
	p := v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "test",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{a, b, w, x, y, z},
		},
	}
	g, err := Build(&p)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable()), []v1alpha1.PipelineTask{a, b}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable("a")), []v1alpha1.PipelineTask{x}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable("b")), []v1alpha1.PipelineTask{}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable("a", "b")), []v1alpha1.PipelineTask{x}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable("x")), []v1alpha1.PipelineTask{z}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable("a", "x")), []v1alpha1.PipelineTask{y, z}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable("a", "x", "b")), []v1alpha1.PipelineTask{y, z}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
	}
	if d := cmp.Diff(sortPipelineTask(g.GetSchedulable("a", "x", "y")), []v1alpha1.PipelineTask{z}); d != "" {
		t.Errorf("incorrect dependencees for no task. diff %s", d)
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
