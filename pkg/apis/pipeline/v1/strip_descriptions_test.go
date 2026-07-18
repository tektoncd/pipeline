/*
Copyright 2026 The Tekton Authors

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

package v1_test

import (
	"slices"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// fullEmbeddedTask exercises every recursive branch of TaskSpec.StripDescriptions.
func fullEmbeddedTask() *v1.EmbeddedTask {
	return &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
		Description: "embedded task desc",
		Params:      v1.ParamSpecs{{Name: "ep", Description: "embedded param desc"}},
		Results:     []v1.TaskResult{{Name: "er", Description: "embedded result desc"}},
		Workspaces:  []v1.WorkspaceDeclaration{{Name: "ew", Description: "embedded ws desc"}},
		Steps: []v1.Step{{
			Name:    "es",
			Results: []v1.StepResult{{Name: "esr", Description: "embedded step result desc"}},
		}},
	}}
}

func TestTaskSpecStripDescriptions(t *testing.T) {
	tests := []struct {
		name string
		spec *v1.TaskSpec
	}{{
		name: "all fields populated",
		spec: &v1.TaskSpec{
			Description: "task desc",
			Params:      v1.ParamSpecs{{Name: "p", Description: "param desc"}},
			Results:     []v1.TaskResult{{Name: "r", Description: "result desc"}},
			Workspaces:  []v1.WorkspaceDeclaration{{Name: "w", Description: "ws desc"}},
			Steps: []v1.Step{{
				Name:    "s",
				Results: []v1.StepResult{{Name: "sr", Description: "step result desc"}},
			}},
		},
	}, {
		name: "multiple items per slice",
		spec: &v1.TaskSpec{
			Description: "task desc",
			Params:      v1.ParamSpecs{{Name: "p1", Description: "d1"}, {Name: "p2", Description: "d2"}},
			Results:     []v1.TaskResult{{Name: "r1", Description: "d1"}, {Name: "r2", Description: "d2"}},
			Steps: []v1.Step{
				{Name: "s1", Results: []v1.StepResult{{Name: "sr1", Description: "d1"}}},
				{Name: "s2", Results: []v1.StepResult{{Name: "sr2", Description: "d2"}}},
			},
		},
	}, {
		name: "empty slices",
		spec: &v1.TaskSpec{Description: "task desc"},
	}, {
		name: "descriptions already empty",
		spec: &v1.TaskSpec{
			Params:  v1.ParamSpecs{{Name: "p"}},
			Results: []v1.TaskResult{{Name: "r"}},
			Steps:   []v1.Step{{Name: "s", Results: []v1.StepResult{{Name: "sr"}}}},
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := taskSpecNames(tc.spec)

			tc.spec.StripDescriptions()

			if d := nonEmptyDescriptions(tc.spec); len(d) > 0 {
				t.Errorf("expected all TaskSpec descriptions cleared, found: %v", d)
			}
			if got := taskSpecNames(tc.spec); !slices.Equal(got, names) {
				t.Errorf("StripDescriptions altered non-description fields: got %v, want %v", got, names)
			}
		})
	}
}

func TestTaskSpecStripDescriptionsNil(t *testing.T) {
	var ts *v1.TaskSpec
	ts.StripDescriptions()
}

func TestPipelineSpecStripDescriptions(t *testing.T) {
	tests := []struct {
		name string
		spec *v1.PipelineSpec
	}{{
		name: "all fields populated",
		spec: &v1.PipelineSpec{
			Description: "pipeline desc",
			Params:      v1.ParamSpecs{{Name: "p", Description: "param desc"}},
			Results:     []v1.PipelineResult{{Name: "r", Description: "result desc"}},
			Workspaces:  []v1.PipelineWorkspaceDeclaration{{Name: "w", Description: "ws desc"}},
			Tasks:       []v1.PipelineTask{{Name: "t", Description: "task desc", TaskSpec: fullEmbeddedTask()}},
			Finally:     []v1.PipelineTask{{Name: "f", Description: "finally desc"}},
		},
	}, {
		name: "finally with embedded TaskSpec",
		spec: &v1.PipelineSpec{
			Finally: []v1.PipelineTask{{Name: "f", Description: "finally desc", TaskSpec: fullEmbeddedTask()}},
		},
	}, {
		name: "task with TaskRef only",
		spec: &v1.PipelineSpec{
			Tasks: []v1.PipelineTask{{Name: "t", Description: "task desc", TaskRef: &v1.TaskRef{Name: "my-task"}}},
		},
	}, {
		name: "empty pipeline spec",
		spec: &v1.PipelineSpec{Description: "pipeline desc"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := pipelineSpecNames(tc.spec)

			tc.spec.StripDescriptions()

			if d := nonEmptyPipelineDescriptions(tc.spec); len(d) > 0 {
				t.Errorf("expected all PipelineSpec descriptions cleared, found: %v", d)
			}
			if got := pipelineSpecNames(tc.spec); !slices.Equal(got, names) {
				t.Errorf("StripDescriptions altered non-description fields: got %v, want %v", got, names)
			}
		})
	}
}

func TestPipelineSpecStripDescriptionsNil(t *testing.T) {
	var ps *v1.PipelineSpec
	ps.StripDescriptions()
}

// taskSpecNames collects identity fields to prove stripping touches only descriptions.
func taskSpecNames(ts *v1.TaskSpec) []string {
	var names []string
	for _, p := range ts.Params {
		names = append(names, "param:"+p.Name)
	}
	for _, r := range ts.Results {
		names = append(names, "result:"+r.Name)
	}
	for _, w := range ts.Workspaces {
		names = append(names, "workspace:"+w.Name)
	}
	for _, s := range ts.Steps {
		names = append(names, "step:"+s.Name)
		for _, r := range s.Results {
			names = append(names, "stepresult:"+r.Name)
		}
	}
	return names
}

func pipelineSpecNames(ps *v1.PipelineSpec) []string {
	var names []string
	for _, p := range ps.Params {
		names = append(names, "param:"+p.Name)
	}
	for _, r := range ps.Results {
		names = append(names, "result:"+r.Name)
	}
	for _, w := range ps.Workspaces {
		names = append(names, "workspace:"+w.Name)
	}
	for _, pt := range append(slices.Clone(ps.Tasks), ps.Finally...) {
		names = append(names, "task:"+pt.Name)
		if pt.TaskRef != nil {
			names = append(names, "taskref:"+pt.TaskRef.Name)
		}
		if pt.TaskSpec != nil {
			names = append(names, taskSpecNames(&pt.TaskSpec.TaskSpec)...)
		}
	}
	return names
}

func nonEmptyPipelineDescriptions(ps *v1.PipelineSpec) []string {
	var found []string
	if ps.Description != "" {
		found = append(found, "spec")
	}
	for _, p := range ps.Params {
		if p.Description != "" {
			found = append(found, "param")
		}
	}
	for _, r := range ps.Results {
		if r.Description != "" {
			found = append(found, "result")
		}
	}
	for _, w := range ps.Workspaces {
		if w.Description != "" {
			found = append(found, "workspace")
		}
	}
	for _, pt := range append(slices.Clone(ps.Tasks), ps.Finally...) {
		if pt.Description != "" {
			found = append(found, "pipelineTask")
		}
		if pt.TaskSpec != nil {
			for _, d := range nonEmptyDescriptions(&pt.TaskSpec.TaskSpec) {
				found = append(found, "embedded:"+d)
			}
		}
	}
	return found
}

func nonEmptyDescriptions(ts *v1.TaskSpec) []string {
	var found []string
	if ts.Description != "" {
		found = append(found, "spec")
	}
	for _, p := range ts.Params {
		if p.Description != "" {
			found = append(found, "param")
		}
	}
	for _, r := range ts.Results {
		if r.Description != "" {
			found = append(found, "result")
		}
	}
	for _, w := range ts.Workspaces {
		if w.Description != "" {
			found = append(found, "workspace")
		}
	}
	for _, s := range ts.Steps {
		for _, r := range s.Results {
			if r.Description != "" {
				found = append(found, "stepresult")
			}
		}
	}
	return found
}
