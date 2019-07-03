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

package v1alpha1_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/list"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sameNodes(l, r []*v1alpha1.Node) error {
	lNames, rNames := []string{}, []string{}
	for _, n := range l {
		lNames = append(lNames, n.Task.Name)
	}
	for _, n := range r {
		rNames = append(rNames, n.Task.Name)
	}

	return list.IsSame(lNames, rNames)
}

func assertSameDAG(t *testing.T, l, r *v1alpha1.DAG) {
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
	expectedDAG := &v1alpha1.DAG{
		Nodes: map[string]*v1alpha1.Node{
			"a": {Task: a},
			"b": {Task: b},
			"c": {Task: c},
		},
	}
	g, err := v1alpha1.BuildDAG(p.Spec.Tasks)
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

	nodeA := &v1alpha1.Node{Task: a}
	nodeB := &v1alpha1.Node{Task: b}
	nodeC := &v1alpha1.Node{Task: c}
	nodeX := &v1alpha1.Node{Task: xDependsOnA}
	nodeY := &v1alpha1.Node{Task: yDependsOnARunsAfterB}
	nodeZ := &v1alpha1.Node{Task: zDependsOnX}

	nodeA.Next = []*v1alpha1.Node{nodeX, nodeY}
	nodeB.Next = []*v1alpha1.Node{nodeY}
	nodeX.Prev = []*v1alpha1.Node{nodeA}
	nodeX.Next = []*v1alpha1.Node{nodeZ}
	nodeY.Prev = []*v1alpha1.Node{nodeA, nodeB}
	nodeZ.Prev = []*v1alpha1.Node{nodeX}

	expectedDAG := &v1alpha1.DAG{
		Nodes: map[string]*v1alpha1.Node{
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
	g, err := v1alpha1.BuildDAG(p.Spec.Tasks)
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
	nodeA := &v1alpha1.Node{Task: a}
	nodeD := &v1alpha1.Node{Task: dDependsOnA}
	nodeE := &v1alpha1.Node{Task: eRunsAfterA}
	nodeF := &v1alpha1.Node{Task: fDependsOnDAndE}
	nodeG := &v1alpha1.Node{Task: gRunsAfterF}

	nodeA.Next = []*v1alpha1.Node{nodeD, nodeE}
	nodeD.Prev = []*v1alpha1.Node{nodeA}
	nodeD.Next = []*v1alpha1.Node{nodeF}
	nodeE.Prev = []*v1alpha1.Node{nodeA}
	nodeE.Next = []*v1alpha1.Node{nodeF}
	nodeF.Prev = []*v1alpha1.Node{nodeD, nodeE}
	nodeF.Next = []*v1alpha1.Node{nodeG}
	nodeG.Prev = []*v1alpha1.Node{nodeF}

	expectedDAG := &v1alpha1.DAG{
		Nodes: map[string]*v1alpha1.Node{
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
	g, err := v1alpha1.BuildDAG(p.Spec.Tasks)
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
			if _, err := v1alpha1.BuildDAG(p.Spec.Tasks); err == nil {
				t.Errorf("expected to see an error for invalid DAG in pipeline %v but had none", tc.spec)
			}
		})
	}
}
