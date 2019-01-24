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
		{"linear-pipeline",
			v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, b, c}},
			&DAG{
				Nodes: map[string]*Node{
					"a": {Task: a},
					"b": {Task: b},
					"c": {Task: c},
				},
			},
			false,
			""},
		{"complex-pipeline",
			v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, xDependsOnA, yDependsOnAB, zDependsOnX, b, c}},
			&DAG{
				Nodes: map[string]*Node{
					"a": {Task: a},
					"b": {Task: b},
					"c": {Task: c},
					"x": {Task: xDependsOnA, Prev: []*Node{{Task: a}}},
					"y": {Task: yDependsOnAB, Prev: []*Node{{Task: b}, {Task: a}}},
					"z": {Task: zDependsOnX, Prev: []*Node{nodeX}},
				},
			},
			false,
			""},
		{"self-link",
			v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{selfLink}},
			nil,
			true,
			` "self-link" is invalid: : Internal error: cycle detected; task "a" depends on itself`},
		{"cycle-2",
			v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{xDependsOnA, zDependsOnX, aDependsOnZ}},
			nil,
			true,
			` "cycle-2" is invalid: : Internal error: cycle detected; a -> x -> z -> a `},
		{"duplicate-tasks",
			v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{a, a}},
			nil,
			true,
			` "duplicate-tasks" is invalid: spec.tasks.name: Duplicate value: "a"`},
		{"invalid-task-name",
			v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{invalidTask}},
			nil,
			true,
			` "invalid-task-name" is invalid: spec.tasks.name: Not found: "none"`},
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

func TestGetPrevTasks(t *testing.T) {
	a := v1alpha1.PipelineTask{Name: "a"}
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
	p := v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "test",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{a, x, y},
		},
	}
	g, err := Build(&p)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if d := cmp.Diff(g.GetPreviousTasks("a"), []v1alpha1.PipelineTask{}); d != "" {
		t.Errorf("incorrect prev tasks for PipelineTask a. diff %s", d)
	}
	if d := cmp.Diff(g.GetPreviousTasks("x"), []v1alpha1.PipelineTask{a}); d != "" {
		t.Errorf("incorrect prev tasks for PipelineTask x. diff %s", d)
	}
	if d := cmp.Diff(g.GetPreviousTasks("y"), []v1alpha1.PipelineTask{x, a}); d != "" {
		t.Errorf("incorrect prev tasks for PipelineTask y. diff %s", d)
	}
}

// hasErr returns true if err is not nil
func hasErr(err error) bool {
	return err != nil
}
