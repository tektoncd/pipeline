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

package v1

// StripDescriptions clears documentation-only description fields from the TaskSpec snapshot. See #10321.
func (ts *TaskSpec) StripDescriptions() {
	if ts == nil {
		return
	}
	ts.Description = ""
	for i := range ts.Params {
		ts.Params[i].Description = ""
	}
	for i := range ts.Results {
		ts.Results[i].Description = ""
	}
	for i := range ts.Workspaces {
		ts.Workspaces[i].Description = ""
	}
	for i := range ts.Steps {
		for j := range ts.Steps[i].Results {
			ts.Steps[i].Results[j].Description = ""
		}
	}
}

// StripDescriptions clears documentation-only description fields from the PipelineSpec snapshot. See #10321.
func (ps *PipelineSpec) StripDescriptions() {
	if ps == nil {
		return
	}
	ps.Description = ""
	for i := range ps.Params {
		ps.Params[i].Description = ""
	}
	for i := range ps.Results {
		ps.Results[i].Description = ""
	}
	for i := range ps.Workspaces {
		ps.Workspaces[i].Description = ""
	}
	for i := range ps.Tasks {
		ps.Tasks[i].stripDescriptions()
	}
	for i := range ps.Finally {
		ps.Finally[i].stripDescriptions()
	}
}

// stripDescriptions clears the PipelineTask description and any inline embedded TaskSpec descriptions.
func (pt *PipelineTask) stripDescriptions() {
	pt.Description = ""
	if pt.TaskSpec != nil {
		pt.TaskSpec.TaskSpec.StripDescriptions()
	}
}
