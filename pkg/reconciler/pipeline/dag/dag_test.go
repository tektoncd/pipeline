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

package dag_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testGraph(t *testing.T) *dag.Graph {
	//  b     a
	//  |    / \
	//  |   |   x
	//  |   | / |
	//  |   y   |
	//   \ /    z
	//    w
	t.Helper()
	tasks := []v1beta1.PipelineTask{{
		Name: "a",
	}, {
		Name: "b",
	}, {
		Name: "w",
		Params: []v1beta1.Param{{
			Name: "foo",
			Value: v1beta1.ArrayOrString{
				Type:      v1beta1.ParamTypeString,
				StringVal: "$(tasks.y.results.bar)",
			},
		}},
		RunAfter: []string{"b"},
	}, {
		Name:     "x",
		RunAfter: []string{"a"},
	}, {
		Name:     "y",
		RunAfter: []string{"a", "x"},
	}, {
		Name:     "z",
		RunAfter: []string{"x"},
	}}
	g, err := dag.Build(v1beta1.PipelineTaskList(tasks))
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
		expectedTasks sets.String
	}{{
		name:          "nothing-done",
		finished:      []string{},
		expectedTasks: sets.NewString("a", "b"),
	}, {
		name:          "a-done",
		finished:      []string{"a"},
		expectedTasks: sets.NewString("b", "x"),
	}, {
		name:          "b-done",
		finished:      []string{"b"},
		expectedTasks: sets.NewString("a"),
	}, {
		name:          "a-and-b-done",
		finished:      []string{"a", "b"},
		expectedTasks: sets.NewString("x"),
	}, {
		name:          "a-x-done",
		finished:      []string{"a", "x"},
		expectedTasks: sets.NewString("b", "y", "z"),
	}, {
		name:          "a-x-b-done",
		finished:      []string{"a", "x", "b"},
		expectedTasks: sets.NewString("y", "z"),
	}, {
		name:          "a-x-y-done",
		finished:      []string{"a", "x", "y"},
		expectedTasks: sets.NewString("b", "z"),
	}, {
		name:          "a-x-y-done",
		finished:      []string{"a", "x", "y"},
		expectedTasks: sets.NewString("b", "z"),
	}, {
		name:          "a-x-y-b-done",
		finished:      []string{"a", "x", "y", "b"},
		expectedTasks: sets.NewString("w", "z"),
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := dag.GetSchedulable(g, tc.finished...)
			if err != nil {
				t.Fatalf("Didn't expect error when getting next tasks for %v but got %v", tc.finished, err)
			}
			if d := cmp.Diff(tasks, tc.expectedTasks, cmpopts.IgnoreFields(v1beta1.PipelineTask{}, "RunAfter")); d != "" {
				t.Errorf("expected that with %v done, %v would be ready to schedule but was different: %s", tc.finished, tc.expectedTasks, diff.PrintWantGot(d))
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
			_, err := dag.GetSchedulable(g, tc.finished...)
			if err == nil {
				t.Fatalf("Expected error for invalid done tasks %v but got none", tc.finished)
			}
		})
	}
}

func sameNodes(l, r []*dag.Node) error {
	lNames, rNames := []string{}, []string{}
	for _, n := range l {
		lNames = append(lNames, n.Task.HashKey())
	}
	for _, n := range r {
		rNames = append(rNames, n.Task.HashKey())
	}

	return list.IsSame(lNames, rNames)
}

func assertSameDAG(t *testing.T, l, r *dag.Graph) {
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
	a := v1beta1.PipelineTask{Name: "a"}
	b := v1beta1.PipelineTask{Name: "b"}
	c := v1beta1.PipelineTask{Name: "c"}

	// This test make sure we can create a Pipeline with no links between any Tasks
	// (all tasks run in parallel)
	//    a   b   c
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{a, b, c},
		},
	}
	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": {Task: a},
			"b": {Task: b},
			"c": {Task: c},
		},
	}
	g, err := dag.Build(v1beta1.PipelineTaskList(p.Spec.Tasks))
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_JoinMultipleRoots(t *testing.T) {
	a := v1beta1.PipelineTask{Name: "a"}
	b := v1beta1.PipelineTask{Name: "b"}
	c := v1beta1.PipelineTask{Name: "c"}
	xDependsOnA := v1beta1.PipelineTask{
		Name: "x",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	yDependsOnARunsAfterB := v1beta1.PipelineTask{
		Name:     "y",
		RunAfter: []string{"b"},
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	zDependsOnX := v1beta1.PipelineTask{
		Name: "z",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"x"}}},
		},
	}

	//   a    b   c
	//   | \ /
	//   x  y
	//   |
	//   z

	nodeA := &dag.Node{Task: a}
	nodeB := &dag.Node{Task: b}
	nodeC := &dag.Node{Task: c}
	nodeX := &dag.Node{Task: xDependsOnA}
	nodeY := &dag.Node{Task: yDependsOnARunsAfterB}
	nodeZ := &dag.Node{Task: zDependsOnX}

	nodeA.Next = []*dag.Node{nodeX, nodeY}
	nodeB.Next = []*dag.Node{nodeY}
	nodeX.Prev = []*dag.Node{nodeA}
	nodeX.Next = []*dag.Node{nodeZ}
	nodeY.Prev = []*dag.Node{nodeA, nodeB}
	nodeZ.Prev = []*dag.Node{nodeX}

	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": nodeA,
			"b": nodeB,
			"c": nodeC,
			"x": nodeX,
			"y": nodeY,
			"z": nodeZ},
	}
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{a, xDependsOnA, yDependsOnARunsAfterB, zDependsOnX, b, c},
		},
	}
	g, err := dag.Build(v1beta1.PipelineTaskList(p.Spec.Tasks))
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_FanInFanOut(t *testing.T) {
	a := v1beta1.PipelineTask{Name: "a"}
	dDependsOnA := v1beta1.PipelineTask{
		Name: "d",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	eRunsAfterA := v1beta1.PipelineTask{
		Name:     "e",
		RunAfter: []string{"a"},
	}
	fDependsOnDAndE := v1beta1.PipelineTask{
		Name: "f",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"d", "e"}}},
		},
	}
	gRunsAfterF := v1beta1.PipelineTask{
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
	nodeA := &dag.Node{Task: a}
	nodeD := &dag.Node{Task: dDependsOnA}
	nodeE := &dag.Node{Task: eRunsAfterA}
	nodeF := &dag.Node{Task: fDependsOnDAndE}
	nodeG := &dag.Node{Task: gRunsAfterF}

	nodeA.Next = []*dag.Node{nodeD, nodeE}
	nodeD.Prev = []*dag.Node{nodeA}
	nodeD.Next = []*dag.Node{nodeF}
	nodeE.Prev = []*dag.Node{nodeA}
	nodeE.Next = []*dag.Node{nodeF}
	nodeF.Prev = []*dag.Node{nodeD, nodeE}
	nodeF.Next = []*dag.Node{nodeG}
	nodeG.Prev = []*dag.Node{nodeF}

	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": nodeA,
			"d": nodeD,
			"e": nodeE,
			"f": nodeF,
			"g": nodeG,
		},
	}
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{a, dDependsOnA, eRunsAfterA, fDependsOnDAndE, gRunsAfterF},
		},
	}
	g, err := dag.Build(v1beta1.PipelineTaskList(p.Spec.Tasks))
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_Invalid(t *testing.T) {
	a := v1beta1.PipelineTask{Name: "a"}
	xDependsOnA := v1beta1.PipelineTask{
		Name: "x",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	zDependsOnX := v1beta1.PipelineTask{
		Name: "z",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"x"}}},
		},
	}
	aDependsOnZ := v1beta1.PipelineTask{
		Name: "a",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"z"}}},
		},
	}
	xAfterA := v1beta1.PipelineTask{
		Name:     "x",
		RunAfter: []string{"a"},
	}
	zAfterX := v1beta1.PipelineTask{
		Name:     "z",
		RunAfter: []string{"x"},
	}
	aAfterZ := v1beta1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"z"},
	}
	selfLinkFrom := v1beta1.PipelineTask{
		Name: "a",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
	}
	selfLinkAfter := v1beta1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"a"},
	}
	invalidTaskFrom := v1beta1.PipelineTask{
		Name: "a",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"none"}}},
		},
	}
	invalidTaskAfter := v1beta1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"none"},
	}

	invalidConditionalTask := v1beta1.PipelineTask{
		Name: "b",
		Conditions: []v1beta1.PipelineTaskCondition{{
			ConditionRef: "some-condition",
			Resources:    []v1beta1.PipelineTaskInputResource{{From: []string{"none"}}},
		}},
	}

	tcs := []struct {
		name string
		spec v1beta1.PipelineSpec
	}{{
		name: "self-link-from",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{selfLinkFrom}},
	}, {
		name: "self-link-after",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{selfLinkAfter}},
	}, {
		name: "cycle-from",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{xDependsOnA, zDependsOnX, aDependsOnZ}},
	}, {
		name: "cycle-runAfter",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{xAfterA, zAfterX, aAfterZ}},
	}, {
		name: "cycle-both",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{xDependsOnA, zAfterX, aDependsOnZ}},
	}, {
		name: "duplicate-tasks",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{a, a}},
	}, {
		name: "invalid-task-name-from",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{invalidTaskFrom}},
	}, {
		name: "invalid-task-name-after",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{invalidTaskAfter}},
	}, {
		name: "invalid-task-name-from-conditional",
		spec: v1beta1.PipelineSpec{Tasks: []v1beta1.PipelineTask{invalidConditionalTask}},
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			p := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name},
				Spec:       tc.spec,
			}
			if _, err := dag.Build(v1beta1.PipelineTaskList(p.Spec.Tasks)); err == nil {
				t.Errorf("expected to see an error for invalid DAG in pipeline %v but had none", tc.spec)
			}
		})
	}
}

func TestBuild_ConditionResources(t *testing.T) {
	// a,b, c are regular tasks
	a := v1beta1.PipelineTask{Name: "a"}
	b := v1beta1.PipelineTask{Name: "b"}
	c := v1beta1.PipelineTask{Name: "c"}

	// Condition that depends on Task a output
	cond1DependsOnA := v1beta1.PipelineTaskCondition{
		Resources: []v1beta1.PipelineTaskInputResource{{From: []string{"a"}}},
	}
	// Condition that depends on Task b output
	cond2DependsOnB := v1beta1.PipelineTaskCondition{
		Resources: []v1beta1.PipelineTaskInputResource{{From: []string{"b"}}},
	}

	// x indirectly depends on A,B via its conditions
	xDependsOnAAndB := v1beta1.PipelineTask{
		Name:       "x",
		Conditions: []v1beta1.PipelineTaskCondition{cond1DependsOnA, cond2DependsOnB},
	}

	// y depends on a both directly + via its conditional
	yDependsOnA := v1beta1.PipelineTask{
		Name: "y",
		Resources: &v1beta1.PipelineTaskResources{
			Inputs: []v1beta1.PipelineTaskInputResource{{From: []string{"a"}}},
		},
		Conditions: []v1beta1.PipelineTaskCondition{cond1DependsOnA},
	}

	// y depends on b both directly + via its conditional
	zDependsOnBRunsAfterC := v1beta1.PipelineTask{
		Name:       "z",
		RunAfter:   []string{"c"},
		Conditions: []v1beta1.PipelineTaskCondition{cond2DependsOnB},
	}

	//  a   b   c
	// / \ / \ /
	// y  x   z
	nodeA := &dag.Node{Task: a}
	nodeB := &dag.Node{Task: b}
	nodeC := &dag.Node{Task: c}
	nodeX := &dag.Node{Task: xDependsOnAAndB}
	nodeY := &dag.Node{Task: yDependsOnA}
	nodeZ := &dag.Node{Task: zDependsOnBRunsAfterC}

	nodeA.Next = []*dag.Node{nodeX, nodeY}
	nodeB.Next = []*dag.Node{nodeX, nodeZ}
	nodeC.Next = []*dag.Node{nodeZ}
	nodeX.Prev = []*dag.Node{nodeA, nodeB}
	nodeY.Prev = []*dag.Node{nodeA}
	nodeZ.Prev = []*dag.Node{nodeB, nodeC}

	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": nodeA,
			"b": nodeB,
			"c": nodeC,
			"x": nodeX,
			"y": nodeY,
			"z": nodeZ,
		},
	}

	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{a, b, c, xDependsOnAAndB, yDependsOnA, zDependsOnBRunsAfterC},
		},
	}

	g, err := dag.Build(v1beta1.PipelineTaskList(p.Spec.Tasks))
	if err != nil {
		t.Errorf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_TaskParamsFromTaskResults(t *testing.T) {
	a := v1beta1.PipelineTask{Name: "a"}
	b := v1beta1.PipelineTask{Name: "b"}
	c := v1beta1.PipelineTask{Name: "c"}
	d := v1beta1.PipelineTask{Name: "d"}
	e := v1beta1.PipelineTask{Name: "e"}
	xDependsOnA := v1beta1.PipelineTask{
		Name: "x",
		Params: []v1beta1.Param{{
			Name:  "paramX",
			Value: *v1beta1.NewArrayOrString("$(tasks.a.results.resultA)"),
		}},
	}
	yDependsOnBRunsAfterC := v1beta1.PipelineTask{
		Name:     "y",
		RunAfter: []string{"c"},
		Params: []v1beta1.Param{{
			Name:  "paramB",
			Value: *v1beta1.NewArrayOrString("$(tasks.b.results.resultB)"),
		}},
	}
	zDependsOnDAndE := v1beta1.PipelineTask{
		Name: "z",
		Params: []v1beta1.Param{{
			Name:  "paramZ",
			Value: *v1beta1.NewArrayOrString("$(tasks.d.results.resultD) $(tasks.e.results.resultE)"),
		}},
	}

	//   a  b   c  d   e
	//   |   \ /    \ /
	//   x    y      z
	nodeA := &dag.Node{Task: a}
	nodeB := &dag.Node{Task: b}
	nodeC := &dag.Node{Task: c}
	nodeD := &dag.Node{Task: d}
	nodeE := &dag.Node{Task: e}
	nodeX := &dag.Node{Task: xDependsOnA}
	nodeY := &dag.Node{Task: yDependsOnBRunsAfterC}
	nodeZ := &dag.Node{Task: zDependsOnDAndE}

	nodeA.Next = []*dag.Node{nodeX}
	nodeB.Next = []*dag.Node{nodeY}
	nodeC.Next = []*dag.Node{nodeY}
	nodeD.Next = []*dag.Node{nodeZ}
	nodeE.Next = []*dag.Node{nodeZ}
	nodeX.Prev = []*dag.Node{nodeA}
	nodeY.Prev = []*dag.Node{nodeB, nodeC}
	nodeZ.Prev = []*dag.Node{nodeD, nodeE}

	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": nodeA,
			"b": nodeB,
			"c": nodeC,
			"d": nodeD,
			"e": nodeE,
			"x": nodeX,
			"y": nodeY,
			"z": nodeZ,
		},
	}
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{a, b, c, d, e, xDependsOnA, yDependsOnBRunsAfterC, zDependsOnDAndE},
		},
	}
	g, err := dag.Build(v1beta1.PipelineTaskList(p.Spec.Tasks))
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_ConditionsParamsFromTaskResults(t *testing.T) {
	a := v1beta1.PipelineTask{Name: "a"}
	xDependsOnA := v1beta1.PipelineTask{
		Name: "x",
		Conditions: []v1beta1.PipelineTaskCondition{{
			ConditionRef: "cond",
			Params: []v1beta1.Param{{
				Name:  "paramX",
				Value: *v1beta1.NewArrayOrString("$(tasks.a.results.resultA)"),
			}},
		}},
	}

	//   a
	//   |
	//   x
	nodeA := &dag.Node{Task: a}
	nodeX := &dag.Node{Task: xDependsOnA}

	nodeA.Next = []*dag.Node{nodeX}
	nodeX.Prev = []*dag.Node{nodeA}
	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": nodeA,
			"x": nodeX,
		},
	}
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{a, xDependsOnA},
		},
	}
	g, err := dag.Build(v1beta1.PipelineTaskList(p.Spec.Tasks))
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}
