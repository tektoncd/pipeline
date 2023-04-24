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
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

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
			tasks, err := dag.GetCandidateTasks(g, tc.finished...)
			if err != nil {
				t.Fatalf("Didn't expect error when getting next tasks for %v but got %v", tc.finished, err)
			}
			if d := cmp.Diff(tasks, tc.expectedTasks, cmpopts.IgnoreFields(v1.PipelineTask{}, "RunAfter")); d != "" {
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
			_, err := dag.GetCandidateTasks(g, tc.finished...)
			if err == nil {
				t.Fatalf("Expected error for invalid done tasks %v but got none", tc.finished)
			}
		})
	}
}

func TestBuild_Parallel(t *testing.T) {
	a := v1.PipelineTask{Name: "a"}
	b := v1.PipelineTask{Name: "b"}
	c := v1.PipelineTask{Name: "c"}

	// This test make sure we can create a Pipeline with no links between any Tasks
	// (all tasks run in parallel)
	//    a   b   c
	p := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{a, b, c},
		},
	}
	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": {Key: "a"},
			"b": {Key: "b"},
			"c": {Key: "c"},
		},
	}
	g, err := dag.Build(v1.PipelineTaskList(p.Spec.Tasks), v1.PipelineTaskList(p.Spec.Tasks).Deps())
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_JoinMultipleRoots(t *testing.T) {
	a := v1.PipelineTask{
		Name: "a",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "result",
			}},
		}},
	}
	b := v1.PipelineTask{Name: "b"}
	c := v1.PipelineTask{
		Name: "c",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "result",
			}},
		}},
	}
	xDependsOnA := v1.PipelineTask{
		Name: "x",
		Params: []v1.Param{
			{
				Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
			},
		},
	}
	yDependsOnARunsAfterB := v1.PipelineTask{
		Name:     "y",
		RunAfter: []string{"b"},
		Params: []v1.Param{
			{
				Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
			},
		},
	}
	zRunsAfterx := v1.PipelineTask{
		Name:     "z",
		RunAfter: []string{"x"},
	}

	//   a    b   c
	//   | \ /
	//   x  y
	//   |
	//   z

	nodeA := &dag.Node{Key: "a"}
	nodeB := &dag.Node{Key: "b"}
	nodeC := &dag.Node{Key: "c"}
	nodeX := &dag.Node{Key: xDependsOnA.Name}
	nodeY := &dag.Node{Key: yDependsOnARunsAfterB.Name}
	nodeZ := &dag.Node{Key: zRunsAfterx.Name}

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
	p := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{a, xDependsOnA, yDependsOnARunsAfterB, zRunsAfterx, b, c},
		},
	}
	g, err := dag.Build(v1.PipelineTaskList(p.Spec.Tasks), v1.PipelineTaskList(p.Spec.Tasks).Deps())
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_FanInFanOut(t *testing.T) {
	a := v1.PipelineTask{
		Name: "a",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "result",
			}},
		}},
	}
	dDependsOnA := v1.PipelineTask{
		Name: "d",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "resultFromD",
			}},
		}},
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
		}},
	}
	eDependsOnA := v1.PipelineTask{
		Name: "e",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "resultFromE",
			}},
		}},
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
		}},
	}
	fDependsOnDAndE := v1.PipelineTask{
		Name: "f",
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.d.results.resultFromD)"),
		}, {
			Value: *v1.NewStructuredValues("$(tasks.e.results.resultFromE)"),
		}},
	}
	gRunsAfterF := v1.PipelineTask{
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
	nodeA := &dag.Node{Key: "a"}
	nodeD := &dag.Node{Key: dDependsOnA.Name}
	nodeE := &dag.Node{Key: eDependsOnA.Name}
	nodeF := &dag.Node{Key: fDependsOnDAndE.Name}
	nodeG := &dag.Node{Key: gRunsAfterF.Name}

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
	p := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{a, dDependsOnA, eDependsOnA, fDependsOnDAndE, gRunsAfterF},
		},
	}
	g, err := dag.Build(v1.PipelineTaskList(p.Spec.Tasks), v1.PipelineTaskList(p.Spec.Tasks).Deps())
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_TaskParamsFromTaskResults(t *testing.T) {
	a := v1.PipelineTask{Name: "a"}
	b := v1.PipelineTask{Name: "b"}
	c := v1.PipelineTask{Name: "c"}
	d := v1.PipelineTask{Name: "d"}
	e := v1.PipelineTask{Name: "e"}
	f := v1.PipelineTask{Name: "f"}
	xDependsOnA := v1.PipelineTask{
		Name: "x",
		Params: []v1.Param{{
			Name:  "paramX",
			Value: *v1.NewStructuredValues("$(tasks.a.results.resultA)"),
		}},
	}
	yDependsOnBRunsAfterC := v1.PipelineTask{
		Name:     "y",
		RunAfter: []string{"c"},
		Params: []v1.Param{{
			Name:  "paramB",
			Value: *v1.NewStructuredValues("$(tasks.b.results.resultB)"),
		}},
	}
	zDependsOnDAndE := v1.PipelineTask{
		Name: "z",
		Params: []v1.Param{{
			Name:  "paramZ",
			Value: *v1.NewStructuredValues("$(tasks.d.results.resultD) $(tasks.e.results.resultE)"),
		}},
	}
	wDependsOnF := v1.PipelineTask{
		Name: "w",
		Params: []v1.Param{{
			Name:  "paramw",
			Value: *v1.NewStructuredValues("$(tasks.f.results.resultF[*])"),
		}},
	}

	//   a  b   c  d   e  f
	//   |   \ /    \ /   |
	//   x    y      z    w
	nodeA := &dag.Node{Key: "a"}
	nodeB := &dag.Node{Key: "b"}
	nodeC := &dag.Node{Key: "c"}
	nodeD := &dag.Node{Key: "d"}
	nodeE := &dag.Node{Key: "e"}
	nodeF := &dag.Node{Key: "f"}
	nodeX := &dag.Node{Key: xDependsOnA.Name}
	nodeY := &dag.Node{Key: yDependsOnBRunsAfterC.Name}
	nodeZ := &dag.Node{Key: zDependsOnDAndE.Name}
	nodeW := &dag.Node{Key: wDependsOnF.Name}

	nodeA.Next = []*dag.Node{nodeX}
	nodeB.Next = []*dag.Node{nodeY}
	nodeC.Next = []*dag.Node{nodeY}
	nodeD.Next = []*dag.Node{nodeZ}
	nodeE.Next = []*dag.Node{nodeZ}
	nodeF.Next = []*dag.Node{nodeW}
	nodeX.Prev = []*dag.Node{nodeA}
	nodeY.Prev = []*dag.Node{nodeB, nodeC}
	nodeZ.Prev = []*dag.Node{nodeD, nodeE}
	nodeW.Prev = []*dag.Node{nodeF}

	expectedDAG := &dag.Graph{
		Nodes: map[string]*dag.Node{
			"a": nodeA,
			"b": nodeB,
			"c": nodeC,
			"d": nodeD,
			"e": nodeE,
			"f": nodeF,
			"x": nodeX,
			"y": nodeY,
			"z": nodeZ,
			"w": nodeW,
		},
	}
	p := &v1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{a, b, c, d, e, f, xDependsOnA, yDependsOnBRunsAfterC, zDependsOnDAndE, wDependsOnF},
		},
	}
	tasks := v1.PipelineTaskList(p.Spec.Tasks)
	g, err := dag.Build(tasks, tasks.Deps())
	if err != nil {
		t.Fatalf("didn't expect error creating valid Pipeline %v but got %v", p, err)
	}
	assertSameDAG(t, expectedDAG, g)
}

func TestBuild_InvalidDAG(t *testing.T) {
	a := v1.PipelineTask{
		Name: "a",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "result",
			}},
		}},
	}
	xDependsOnA := v1.PipelineTask{
		Name: "x",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "resultX",
			}},
		}},
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
		}},
	}
	zDependsOnX := v1.PipelineTask{
		Name: "z",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "resultZ",
			}},
		}},
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.x.results.resultX)"),
		}},
	}
	aDependsOnZ := v1.PipelineTask{
		Name: "a",
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.z.results.resultZ)"),
		}},
	}
	xAfterA := v1.PipelineTask{
		Name:     "x",
		RunAfter: []string{"a"},
	}
	zAfterX := v1.PipelineTask{
		Name:     "z",
		RunAfter: []string{"x"},
	}
	aAfterZ := v1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"z"},
	}
	selfLinkResult := v1.PipelineTask{
		Name: "a",
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "result",
			}},
		}},
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
		}},
	}
	invalidTaskResult := v1.PipelineTask{
		Name: "a",
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.invalid.results.none)"),
		}},
	}
	selfLinkAfter := v1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"a"},
	}
	invalidTaskAfter := v1.PipelineTask{
		Name:     "a",
		RunAfter: []string{"none"},
	}

	aRunsAfterE := v1.PipelineTask{
		Name: "a", RunAfter: []string{"e"},
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "result",
			}},
		}},
	}
	bDependsOnA := v1.PipelineTask{
		Name: "b",
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
		}},
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "resultb",
			}},
		}},
	}
	cRunsAfterA := v1.PipelineTask{
		Name: "c",
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.a.results.result)"),
		}},
		TaskSpec: &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
			Results: []v1.TaskResult{{
				Name: "resultc",
			}},
		}},
	}
	dDependsOnBAndC := v1.PipelineTask{
		Name: "d",
		Params: []v1.Param{{
			Value: *v1.NewStructuredValues("$(tasks.b.results.resultb)"),
		}, {
			Value: *v1.NewStructuredValues("$(tasks.c.results.resultc)"),
		}},
	}
	eRunsAfterD := v1.PipelineTask{
		Name:     "e",
		RunAfter: []string{"d"},
	}
	fRunsAfterD := v1.PipelineTask{
		Name:     "f",
		RunAfter: []string{"d"},
	}
	gDependsOnF := v1.PipelineTask{
		Name:     "g",
		RunAfter: []string{"f"},
	}

	tcs := []struct {
		name string
		spec v1.PipelineSpec
		err  string
	}{{
		// a
		// |
		// a ("a" uses result of "a" as params)
		name: "self-link-result",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{selfLinkResult}},
		err:  "cycle detected",
	}, {
		// a
		// |
		// a ("a" runAfter "a")
		name: "self-link-after",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{selfLinkAfter}},
		err:  "cycle detected",
	}, {
		// a (also "a" depends on resource from "z")
		// |
		// x ("x" depends on resource from "a")
		// |
		// z ("z" depends on resource from "x")
		name: "cycle-from",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{xDependsOnA, zDependsOnX, aDependsOnZ}},
		err:  "cycle detected",
	}, {
		// a (also "a" runAfter "z")
		// |
		// x ("x" runAfter "a")
		// |
		// z ("z" runAfter "x")
		name: "cycle-runAfter",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{xAfterA, zAfterX, aAfterZ}},
		err:  "cycle detected",
	}, {
		// a (also "a" depends on resource from "z")
		// |
		// x ("x" depends on resource from "a")
		// |
		// z ("z" runAfter "x")
		name: "cycle-both",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{xDependsOnA, zAfterX, aDependsOnZ}},
		err:  "cycle detected",
	}, {
		// This test make sure we detect a cyclic branch in a DAG with multiple branches.
		// The following DAG is having a cyclic branch with an additional dependency (a runAfter e)
		//   a
		//  / \
		// b   c
		//  \ /
		//   d
		//  / \
		// e   f
		//     |
		//     g
		name: "multiple-branches-with-one-cyclic-branch",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{aRunsAfterE, bDependsOnA, cRunsAfterA, dDependsOnBAndC, eRunsAfterD, fRunsAfterD, gDependsOnF}},
		err:  "cycle detected",
	}, {
		name: "duplicate-tasks",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{a, a}},
		err:  "duplicate pipeline task",
	}, {
		name: "invalid-task-result",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{invalidTaskResult}},
		err:  "wasn't present in Pipeline",
	}, {
		name: "invalid-task-name-after",
		spec: v1.PipelineSpec{Tasks: []v1.PipelineTask{invalidTaskAfter}},
		err:  "wasn't present in Pipeline",
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			p := &v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name},
				Spec:       tc.spec,
			}
			_, err := dag.Build(v1.PipelineTaskList(p.Spec.Tasks), v1.PipelineTaskList(p.Spec.Tasks).Deps())
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("expected to see an error for invalid DAG in pipeline %v but had none", tc.spec)
			}
		})
	}
}

func TestBuildGraphWithHundredsOfTasks_Success(t *testing.T) {
	var tasks []v1.PipelineTask
	// separate branches with sequential tasks and redundant links (each task explicitly depends on all predecessors)
	// b00 - 000 - 001 - ... - 100
	// b01 - 000 - 001 - ... - 100
	// ..
	// b04 - 000 - 001 - ... - 100
	nBranches, nTasks := 5, 100
	for branchIdx := 0; branchIdx < nBranches; branchIdx++ {
		var taskDeps []string
		firstTaskName := fmt.Sprintf("b%02d", branchIdx)
		firstTask := v1.PipelineTask{
			Name:     firstTaskName,
			TaskRef:  &v1.TaskRef{Name: firstTaskName + "-task"},
			RunAfter: taskDeps,
		}
		tasks = append(tasks, firstTask)
		taskDeps = append(taskDeps, firstTaskName)
		for taskIdx := 0; taskIdx < nTasks; taskIdx++ {
			taskName := fmt.Sprintf("%s-%03d", firstTaskName, taskIdx)
			task := v1.PipelineTask{
				Name:     taskName,
				TaskRef:  &v1.TaskRef{Name: taskName + "-task"},
				RunAfter: taskDeps,
			}
			tasks = append(tasks, task)
			taskDeps = append(taskDeps, taskName)
		}
	}

	_, err := dag.Build(v1.PipelineTaskList(tasks), v1.PipelineTaskList(tasks).Deps())
	if err != nil {
		t.Error(err)
	}
}

func TestBuildGraphWithHundredsOfTasks_InvalidDAG(t *testing.T) {
	var tasks []v1.PipelineTask
	// branches with circular interdependencies
	nBranches, nTasks := 5, 100
	for branchIdx := 0; branchIdx < nBranches; branchIdx++ {
		depBranchIdx := branchIdx + 1
		if depBranchIdx == nBranches {
			depBranchIdx = 0
		}
		taskDeps := []string{fmt.Sprintf("b%02d", depBranchIdx)}
		firstTaskName := fmt.Sprintf("b%02d", branchIdx)
		firstTask := v1.PipelineTask{
			Name:     firstTaskName,
			TaskRef:  &v1.TaskRef{Name: firstTaskName + "-task"},
			RunAfter: taskDeps,
		}
		tasks = append(tasks, firstTask)
		taskDeps = append(taskDeps, firstTaskName)
		for taskIdx := 0; taskIdx < nTasks; taskIdx++ {
			taskName := fmt.Sprintf("%s-%03d", firstTaskName, taskIdx)
			task := v1.PipelineTask{
				Name:     taskName,
				TaskRef:  &v1.TaskRef{Name: taskName + "-task"},
				RunAfter: taskDeps,
			}
			tasks = append(tasks, task)
			taskDeps = append(taskDeps, taskName)
		}
	}

	_, err := dag.Build(v1.PipelineTaskList(tasks), v1.PipelineTaskList(tasks).Deps())
	if err == nil {
		t.Errorf("Pipeline.Validate() did not return error for invalid pipeline with cycles")
	}
}

func testGraph(t *testing.T) *dag.Graph {
	//  b     a
	//  |    / \
	//  |   |   x
	//  |   | / |
	//  |   y   |
	//   \ /    z
	//    w
	t.Helper()
	tasks := []v1.PipelineTask{{
		Name: "a",
	}, {
		Name: "b",
	}, {
		Name: "w",
		Params: []v1.Param{{
			Name:  "foo",
			Value: *v1.NewStructuredValues("$(tasks.y.results.bar)"),
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
	g, err := dag.Build(v1.PipelineTaskList(tasks), v1.PipelineTaskList(tasks).Deps())
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func sameNodes(l, r []*dag.Node) error {
	lNames, rNames := []string{}, []string{}
	for _, n := range l {
		lNames = append(lNames, n.Key)
	}
	for _, n := range r {
		rNames = append(rNames, n.Key)
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

func TestFindCyclesInDependencies(t *testing.T) {
	deps := map[string][]string{
		"a": {},
		"b": {"c", "d"},
		"c": {},
		"d": {},
	}

	err := dag.FindCyclesInDependencies(deps)
	if err != nil {
		t.Error(err)
	}

	tcs := []struct {
		name string
		deps map[string][]string
		err  string
	}{{
		name: "valid-empty-deps",
		deps: map[string][]string{
			"a": {},
			"b": {"c", "d"},
			"c": {},
			"d": {},
		},
	}, {
		name: "self-link",
		deps: map[string][]string{
			"a": {"a"},
		},
		err: `task "a" depends on "a"`,
	}, {
		name: "interdependent-tasks",
		deps: map[string][]string{
			"a": {"b"},
			"b": {"a"},
		},
		err: `task "a" depends on "b"`,
	}, {
		name: "multiple-cycles",
		deps: map[string][]string{
			"a": {"b", "c"},
			"b": {"a"},
			"c": {"d"},
			"d": {"a", "b"},
		},
		err: `task "a" depends on "b", "c"`,
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := dag.FindCyclesInDependencies(tc.deps)
			if tc.err == "" {
				if err != nil {
					t.Errorf("expected to see no error for valid DAG but had: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Errorf("expected to see an error: %q for invalid DAG but had: %v", tc.err, err)
				}
			}
		})
	}
}
