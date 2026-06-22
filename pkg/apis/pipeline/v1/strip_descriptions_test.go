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
	"strings"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func TestTaskSpecStripDescriptions(t *testing.T) {
	ts := &v1.TaskSpec{
		Description: "task desc",
		Params:      v1.ParamSpecs{{Name: "p", Description: "param desc"}},
		Results:     []v1.TaskResult{{Name: "r", Description: "result desc"}},
		Workspaces:  []v1.WorkspaceDeclaration{{Name: "w", Description: "ws desc"}},
		Steps: []v1.Step{{
			Name:    "s",
			Results: []v1.StepResult{{Name: "sr", Description: "step result desc"}},
		}},
	}

	ts.StripDescriptions()

	if d := nonEmptyDescriptions(ts); len(d) > 0 {
		t.Errorf("expected all TaskSpec descriptions cleared, found: %v", d)
	}
	if ts.Params[0].Name != "p" || ts.Results[0].Name != "r" || ts.Steps[0].Results[0].Name != "sr" {
		t.Error("StripDescriptions must not alter non-description fields")
	}
}

func TestTaskSpecStripDescriptionsNil(t *testing.T) {
	var ts *v1.TaskSpec
	ts.StripDescriptions()
}

func TestPipelineSpecStripDescriptions(t *testing.T) {
	embedded := &v1.EmbeddedTask{TaskSpec: v1.TaskSpec{
		Description: "embedded task desc",
		Params:      v1.ParamSpecs{{Name: "ep", Description: "embedded param desc"}},
		Results:     []v1.TaskResult{{Name: "er", Description: "embedded result desc"}},
	}}
	ps := &v1.PipelineSpec{
		Description: "pipeline desc",
		Params:      v1.ParamSpecs{{Name: "p", Description: "param desc"}},
		Results:     []v1.PipelineResult{{Name: "r", Description: "result desc"}},
		Workspaces:  []v1.PipelineWorkspaceDeclaration{{Name: "w", Description: "ws desc"}},
		Tasks:       []v1.PipelineTask{{Name: "t", Description: "task desc", TaskSpec: embedded}},
		Finally:     []v1.PipelineTask{{Name: "f", Description: "finally desc"}},
	}

	ps.StripDescriptions()

	var leftover []string
	if ps.Description != "" {
		leftover = append(leftover, "spec")
	}
	if ps.Params[0].Description != "" {
		leftover = append(leftover, "param")
	}
	if ps.Results[0].Description != "" {
		leftover = append(leftover, "result")
	}
	if ps.Workspaces[0].Description != "" {
		leftover = append(leftover, "workspace")
	}
	if ps.Tasks[0].Description != "" {
		leftover = append(leftover, "task")
	}
	if ps.Finally[0].Description != "" {
		leftover = append(leftover, "finally")
	}
	if d := nonEmptyDescriptions(&ps.Tasks[0].TaskSpec.TaskSpec); len(d) > 0 {
		leftover = append(leftover, "embedded:"+strings.Join(d, ","))
	}
	if len(leftover) > 0 {
		t.Errorf("expected all PipelineSpec descriptions cleared, found: %v", leftover)
	}
	if ps.Tasks[0].Name != "t" || ps.Finally[0].Name != "f" {
		t.Error("StripDescriptions must not alter non-description fields")
	}
}

func TestPipelineSpecStripDescriptionsNil(t *testing.T) {
	var ps *v1.PipelineSpec
	ps.StripDescriptions()
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
