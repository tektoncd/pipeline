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
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/list"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sameNodes(l, r []*Node) error {
	lNames, rNames := []string{}, []string{}
	for _, n := range l {
		lNames = append(lNames, n.Task.Name)
	}
	for _, n := range r {
		rNames = append(rNames, n.Task.Name)
	}

	return list.IsSame(lNames, rNames)
}

func assertSameDAG(t *testing.T, l, r *DAG) {
	t.Helper()
	lKeys, rKeys := []string{}, []string{}

	for k := range l.Nodes {
		lKeys = append(lKeys, k)
	}
	for k := range r.Nodes {
		rKeys = append(rKeys, k)
	}

	// For the DAGs to be the same, they must contain the same nodes
	err := list.IsSame(lKeys, rKeys)
	if err != nil {
		t.Fatalf("DAGS contain different nodes: %v", err)
	}

	// If they contain the same nodes, the DAGs will be the same if all
	// of the nodes have the same linkages
	for k, rn := range r.Nodes {
		ln := l.Nodes[k]

		err := sameNodes(rn.Prev, ln.Prev)
		if err != nil {
			t.Errorf("The %s nodes in the DAG have different previous nodes: %v", k, err)
		}
		err = sameNodes(rn.Next, ln.Next)
		if err != nil {
			t.Errorf("The %s nodes in the DAG have different next nodes: %v", k, err)
		}
	}
}

func TestBuild_Parallel(t *testing.T) {
	a := v1alpha1.PipelineTask{Name: "a"}
	b := v1alpha1.PipelineTask{Name: "b"}
	c := v1alpha1.PipelineTask{Name: "c"}

	// This test make sure we can create a Pipeline with no links between any Tasks
	// (all tasks run in parallel)
	//    a   b   c
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{a, b, c},
		},
	}
	expectedDAG := &DAG{
		Nodes: map[string]*Node{
			"a": {Task: a},
			"b": {Task: b},
			"c": {Task: c},
		},
	}
	g, err := Build(p)
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_JoinMultipleRoots(t *testing.T) {
	a := v1alpha1.PipelineTask{Name: "a"}
	b := v1alpha1.PipelineTask{Name: "b"}
	c := v1alpha1.PipelineTask{Name: "c"}
	xDependsOnA := v1alpha1.PipelineTask{
		Name: "x",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	yDependsOnARunsAfterB := v1alpha1.PipelineTask{
		Name:     "y",
		RunAfter: []string{"b"},
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	zDependsOnX := v1alpha1.PipelineTask{
		Name: "z",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"x"}}},
		},
	}

	//   a    b   c
	//   | \ /
	//   x  y
	//   |
	//   z

	nodeA := &Node{Task: a}
	nodeB := &Node{Task: b}
	nodeC := &Node{Task: c}
	nodeX := &Node{Task: xDependsOnA}
	nodeY := &Node{Task: yDependsOnARunsAfterB}
	nodeZ := &Node{Task: zDependsOnX}

	nodeA.Next = []*Node{nodeX, nodeY}
	nodeB.Next = []*Node{nodeY}
	nodeX.Prev = []*Node{nodeA}
	nodeX.Next = []*Node{nodeZ}
	nodeY.Prev = []*Node{nodeA, nodeB}
	nodeZ.Prev = []*Node{nodeX}

	expectedDAG := &DAG{
		Nodes: map[string]*Node{
			"a": nodeA,
			"b": nodeB,
			"c": nodeC,
			"x": nodeX,
			"y": nodeY,
			"z": nodeZ},
	}
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{a, xDependsOnA, yDependsOnARunsAfterB, zDependsOnX, b, c},
		},
	}
	g, err := Build(p)
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_FanInFanOut(t *testing.T) {
	a := v1alpha1.PipelineTask{Name: "a"}
	dDependsOnA := v1alpha1.PipelineTask{
		Name: "d",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	eRunsAfterA := v1alpha1.PipelineTask{
		Name:     "e",
		RunAfter: []string{"a"},
	}
	fDependsOnDAndE := v1alpha1.PipelineTask{
		Name: "f",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"d", "e"}}},
		},
	}
	gRunsAfterF := v1alpha1.PipelineTask{
		Name:     "g",
		RunAfter: []string{"f"},
	}

	// This test make sure we don't detect cycle (A -> B -> B -> …) when there is not.
	// This means we "visit" a twice, from two different path ; but there is no cycle.
	//   a
	//  / \
	// d   e
	//  \ /
	//   f
	//   |
	//   g
	nodeA := &Node{Task: a}
	nodeD := &Node{Task: dDependsOnA}
	nodeE := &Node{Task: eRunsAfterA}
	nodeF := &Node{Task: fDependsOnDAndE}
	nodeG := &Node{Task: gRunsAfterF}

	nodeA.Next = []*Node{nodeD, nodeE}
	nodeD.Prev = []*Node{nodeA}
	nodeD.Next = []*Node{nodeF}
	nodeE.Prev = []*Node{nodeA}
	nodeE.Next = []*Node{nodeF}
	nodeF.Prev = []*Node{nodeD, nodeE}
	nodeF.Next = []*Node{nodeG}
	nodeG.Prev = []*Node{nodeF}

	expectedDAG := &DAG{
		Nodes: map[string]*Node{
			"a": nodeA,
			"d": nodeD,
			"e": nodeE,
			"f": nodeF,
			"g": nodeG,
		},
	}
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{a, dDependsOnA, eRunsAfterA, fDependsOnDAndE, gRunsAfterF},
		},
	}
	g, err := Build(p)
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_Invalid(t *testing.T) {
	a := v1alpha1.PipelineTask{Name: "a"}
	xDependsOnA := v1alpha1.PipelineTask{
		Name: "x",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
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
	xAfterA := v1alpha1.PipelineTask{
		Name:     "x",
		RunAfter: []string{"a"},
	}
	zAfterX := v1alpha1.PipelineTask{
		Name:     "z",
		RunAfter: []string{"x"},
	}
	aAfterZ := v1alpha1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"z"},
	}
	selfLinkFrom := v1alpha1.PipelineTask{
		Name: "a",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	selfLinkAfter := v1alpha1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"a"},
	}
	invalidTaskFrom := v1alpha1.PipelineTask{
		Name: "a",
		Resources: &v1alpha1.PipelineTaskResources{
			Inputs: []v1alpha1.PipelineTaskInputResource{{From: []string{"none"}}},
		},
	}
	invalidTaskAfter := v1alpha1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"none"},
	}

	tcs := []struct {
		name string
		spec v1alpha1.PipelineSpec
	}{{
		name: "self-link-from",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{selfLinkFrom}},
	}, {
		name: "self-link-after",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{selfLinkAfter}},
	}, {
		name: "cycle-from",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{xDependsOnA, zDependsOnX, aDependsOnZ}},
	}, {
		name: "cycle-runAfter",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{xAfterA, zAfterX, aAfterZ}},
	}, {
		name: "cycle-both",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{xDependsOnA, zAfterX, aDependsOnZ}},
	}, {
		name: "duplicate-tasks",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, a}},
	}, {
		name: "invalid-task-name-from",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{invalidTaskFrom}},
	}, {
		name: "invalid-task-name-after",
		spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{invalidTaskAfter}},
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			p := &v1alpha1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name},
				Spec:       tc.spec,
			}
			if _, err := Build(p); err == nil {
				t.Errorf("expected to see an error for invalid DAG in pipeline %v but had none", tc.spec)
			}
		})
	}
}

func testGraph() *DAG {
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
	}
	g.linkPipelineTasks(g.Nodes["a"], g.Nodes["x"])

	g.Nodes["y"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "y"},
	}
	g.linkPipelineTasks(g.Nodes["a"], g.Nodes["y"])
	g.linkPipelineTasks(g.Nodes["x"], g.Nodes["y"])

	g.Nodes["z"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "z"},
	}
	g.linkPipelineTasks(g.Nodes["x"], g.Nodes["z"])

	g.Nodes["w"] = &Node{
		Task: v1alpha1.PipelineTask{Name: "w"},
	}
	g.linkPipelineTasks(g.Nodes["y"], g.Nodes["w"])
	g.linkPipelineTasks(g.Nodes["b"], g.Nodes["w"])
	return g
}

func TestGetSchedulable(t *testing.T) {
	g := testGraph()
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
		expectedTasks: []v1alpha1.PipelineTask{{Name: "b"}, {Name: "x"}},
	}, {
		name:          "b-done",
		finished:      []string{"b"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "a"}},
	}, {
		name:          "a-and-b-done",
		finished:      []string{"a", "b"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "x"}},
	}, {
		name:          "a-x-done",
		finished:      []string{"a", "x"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "b"}, {Name: "y"}, {Name: "z"}},
	}, {
		name:          "a-x-b-done",
		finished:      []string{"a", "x", "b"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "y"}, {Name: "z"}},
	}, {
		name:          "a-x-y-done",
		finished:      []string{"a", "x", "y"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "b"}, {Name: "z"}},
	}, {
		name:          "a-x-y-done",
		finished:      []string{"a", "x", "y"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "b"}, {Name: "z"}},
	}, {
		name:          "a-x-y-b-done",
		finished:      []string{"a", "x", "y", "b"},
		expectedTasks: []v1alpha1.PipelineTask{{Name: "w"}, {Name: "z"}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := g.GetSchedulable(tc.finished...)
			if err != nil {
				t.Fatalf("Didn't expect error when getting next tasks for %v but got %v", tc.finished, err)
			}
			if d := cmp.Diff(sortPipelineTask(tasks), tc.expectedTasks); d != "" {
				t.Errorf("expected that with %v done, %v would be ready to schedule but was different: %s", tc.finished, tc.expectedTasks, d)
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

func TestGetSchedulable_Invalid(t *testing.T) {
	g := testGraph()
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
			_, err := g.GetSchedulable(tc.finished...)
			if err == nil {
				t.Fatalf("Expected error for invalid done tasks %v but got none", tc.finished)
			}
		})
	}
}
