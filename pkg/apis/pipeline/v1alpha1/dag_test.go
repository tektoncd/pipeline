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

package v1alpha1

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/list"
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
	a := PipelineTask{Name: "a"}
	b := PipelineTask{Name: "b"}
	c := PipelineTask{Name: "c"}

	// This test make sure we can create a Pipeline with no links between any Tasks
	// (all tasks run in parallel)
	//    a   b   c
	p := &Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: PipelineSpec{
			Tasks: []PipelineTask{a, b, c},
		},
	}
	expectedDAG := &DAG{
		Nodes: map[string]*Node{
			"a": {Task: a},
			"b": {Task: b},
			"c": {Task: c},
		},
	}
	g, err := BuildDAG(p.Spec.Tasks)
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_JoinMultipleRoots(t *testing.T) {
	a := PipelineTask{Name: "a"}
	b := PipelineTask{Name: "b"}
	c := PipelineTask{Name: "c"}
	xDependsOnA := PipelineTask{
		Name: "x",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	yDependsOnARunsAfterB := PipelineTask{
		Name:     "y",
		RunAfter: []string{"b"},
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	zDependsOnX := PipelineTask{
		Name: "z",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"x"}}},
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
	p := &Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: PipelineSpec{
			Tasks: []PipelineTask{a, xDependsOnA, yDependsOnARunsAfterB, zDependsOnX, b, c},
		},
	}
	g, err := BuildDAG(p.Spec.Tasks)
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_FanInFanOut(t *testing.T) {
	a := PipelineTask{Name: "a"}
	dDependsOnA := PipelineTask{
		Name: "d",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	eRunsAfterA := PipelineTask{
		Name:     "e",
		RunAfter: []string{"a"},
	}
	fDependsOnDAndE := PipelineTask{
		Name: "f",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"d", "e"}}},
		},
	}
	gRunsAfterF := PipelineTask{
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
	p := &Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: PipelineSpec{
			Tasks: []PipelineTask{a, dDependsOnA, eRunsAfterA, fDependsOnDAndE, gRunsAfterF},
		},
	}
	g, err := BuildDAG(p.Spec.Tasks)
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_Invalid(t *testing.T) {
	a := PipelineTask{Name: "a"}
	xDependsOnA := PipelineTask{
		Name: "x",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	zDependsOnX := PipelineTask{
		Name: "z",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"x"}}},
		},
	}
	aDependsOnZ := PipelineTask{
		Name: "a",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"z"}}},
		},
	}
	xAfterA := PipelineTask{
		Name:     "x",
		RunAfter: []string{"a"},
	}
	zAfterX := PipelineTask{
		Name:     "z",
		RunAfter: []string{"x"},
	}
	aAfterZ := PipelineTask{
		Name:     "a",
		RunAfter: []string{"z"},
	}
	selfLinkFrom := PipelineTask{
		Name: "a",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	selfLinkAfter := PipelineTask{
		Name:     "a",
		RunAfter: []string{"a"},
	}
	invalidTaskFrom := PipelineTask{
		Name: "a",
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{From: []string{"none"}}},
		},
	}
	invalidTaskAfter := PipelineTask{
		Name:     "a",
		RunAfter: []string{"none"},
	}

	tcs := []struct {
		name string
		spec PipelineSpec
	}{{
		name: "self-link-from",
		spec: PipelineSpec{Tasks: []PipelineTask{selfLinkFrom}},
	}, {
		name: "self-link-after",
		spec: PipelineSpec{Tasks: []PipelineTask{selfLinkAfter}},
	}, {
		name: "cycle-from",
		spec: PipelineSpec{Tasks: []PipelineTask{xDependsOnA, zDependsOnX, aDependsOnZ}},
	}, {
		name: "cycle-runAfter",
		spec: PipelineSpec{Tasks: []PipelineTask{xAfterA, zAfterX, aAfterZ}},
	}, {
		name: "cycle-both",
		spec: PipelineSpec{Tasks: []PipelineTask{xDependsOnA, zAfterX, aDependsOnZ}},
	}, {
		name: "duplicate-tasks",
		spec: PipelineSpec{Tasks: []PipelineTask{a, a}},
	}, {
		name: "invalid-task-name-from",
		spec: PipelineSpec{Tasks: []PipelineTask{invalidTaskFrom}},
	}, {
		name: "invalid-task-name-after",
		spec: PipelineSpec{Tasks: []PipelineTask{invalidTaskAfter}},
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			p := &Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name},
				Spec:       tc.spec,
			}
			if _, err := BuildDAG(p.Spec.Tasks); err == nil {
				t.Errorf("expected to see an error for invalid DAG in pipeline %v but had none", tc.spec)
			}
		})
	}
}
