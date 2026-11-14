/*
Copyright 2024 The Tekton Authors

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

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPipelineTaskArtifactBinding_JSONRoundTrip(t *testing.T) {
	binding := PipelineTaskArtifactBinding{
		Name: "source",
		From: "tasks.build.outputs.image",
	}

	data, err := json.Marshal(binding)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var got PipelineTaskArtifactBinding
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if d := cmp.Diff(binding, got); d != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", d)
	}
}

func TestPipelineTask_WithArtifactBindings(t *testing.T) {
	pt := PipelineTask{
		Name:    "deploy",
		TaskRef: &TaskRef{Name: "deploy-task"},
		Artifacts: &PipelineTaskArtifacts{
			Inputs: []PipelineTaskArtifactBinding{
				{Name: "image", From: "tasks.build.outputs.image"},
				{Name: "sbom", From: "tasks.build.outputs.sbom"},
			},
		},
	}

	data, err := json.Marshal(pt)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var got PipelineTask
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if got.Artifacts == nil {
		t.Fatal("expected Artifacts to be non-nil after round-trip")
	}
	if d := cmp.Diff(pt.Artifacts, got.Artifacts); d != "" {
		t.Errorf("PipelineTask.Artifacts round-trip mismatch (-want +got):\n%s", d)
	}
}

func TestPipelineTaskArtifacts_NilWhenOmitted(t *testing.T) {
	pt := PipelineTask{
		Name:    "simple",
		TaskRef: &TaskRef{Name: "simple-task"},
	}

	data, err := json.Marshal(pt)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var got PipelineTask
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if got.Artifacts != nil {
		t.Errorf("expected Artifacts to be nil when omitted, got %+v", got.Artifacts)
	}
}
