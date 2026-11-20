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

package resources

import (
	"encoding/json"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

func TestResolveArtifactInputsForTask(t *testing.T) {
	pt := &v1.PipelineTask{
		Name: "consume",
		Artifacts: &v1.PipelineTaskArtifacts{
			Inputs: []v1.PipelineTaskArtifactBinding{
				{Name: "data", From: "tasks.produce.outputs.results"},
			},
		},
	}

	taskArtifacts := map[string]*v1.Artifacts{
		"produce": {
			Outputs: []v1.Artifact{
				{
					Name: "results",
					Values: []v1.ArtifactValue{
						{Uri: "registry:5000/artifacts/results@sha256:abc123", Digest: map[v1.Algorithm]string{"sha256": "abc123"}},
					},
				},
			},
		},
	}

	resolved, err := ResolveArtifactInputsForTask(pt, taskArtifacts)
	if err != nil {
		t.Fatalf("ResolveArtifactInputsForTask() error = %v", err)
	}

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved input, got %d", len(resolved))
	}
	if resolved[0].Name != "data" {
		t.Errorf("expected name 'data', got %q", resolved[0].Name)
	}
	if resolved[0].URI != "registry:5000/artifacts/results@sha256:abc123" {
		t.Errorf("expected URI from upstream, got %q", resolved[0].URI)
	}
	if resolved[0].Path != "/tekton/artifacts/inputs/data" {
		t.Errorf("expected path, got %q", resolved[0].Path)
	}
}

func TestResolveArtifactInputsForTask_NoBindings(t *testing.T) {
	pt := &v1.PipelineTask{Name: "simple"}

	resolved, err := ResolveArtifactInputsForTask(pt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 0 {
		t.Errorf("expected empty, got %v", resolved)
	}
}

func TestResolveArtifactInputsForTask_MissingUpstream(t *testing.T) {
	pt := &v1.PipelineTask{
		Name: "consume",
		Artifacts: &v1.PipelineTaskArtifacts{
			Inputs: []v1.PipelineTaskArtifactBinding{
				{Name: "data", From: "tasks.missing.outputs.results"},
			},
		},
	}

	_, err := ResolveArtifactInputsForTask(pt, map[string]*v1.Artifacts{})
	if err == nil {
		t.Error("expected error for missing upstream task")
	}
}

func TestResolvedArtifactInputsJSON(t *testing.T) {
	inputs := []entrypoint.ArtifactInput{
		{Name: "data", URI: "registry:5000/x@sha256:abc", Path: "/workspace/artifacts/inputs/data"},
	}

	data, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}

	var got []entrypoint.ArtifactInput
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got[0].URI != inputs[0].URI {
		t.Errorf("round-trip mismatch: %q != %q", got[0].URI, inputs[0].URI)
	}
}
