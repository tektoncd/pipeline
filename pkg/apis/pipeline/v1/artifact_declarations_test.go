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

func TestArtifactDeclaration_JSONRoundTrip(t *testing.T) {
	decl := ArtifactDeclaration{
		Name:      "source-code",
		MediaType: "application/vnd.tekton.artifact.source.v1+tar.gz",
		Subject:   true,
	}

	data, err := json.Marshal(decl)
	if err != nil {
		t.Fatalf("failed to marshal ArtifactDeclaration: %v", err)
	}

	var got ArtifactDeclaration
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal ArtifactDeclaration: %v", err)
	}

	if d := cmp.Diff(decl, got); d != "" {
		t.Errorf("ArtifactDeclaration round-trip mismatch (-want +got):\n%s", d)
	}
}

func TestArtifactDeclarations_JSONRoundTrip(t *testing.T) {
	decls := ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: "source", MediaType: "application/vnd.tekton.artifact.source.v1+tar.gz"},
		},
		Outputs: []ArtifactDeclaration{
			{Name: "image", MediaType: "application/vnd.oci.image.manifest.v1+json", Subject: true},
			{Name: "sbom", MediaType: "application/vnd.cyclonedx+json"},
		},
	}

	data, err := json.Marshal(decls)
	if err != nil {
		t.Fatalf("failed to marshal ArtifactDeclarations: %v", err)
	}

	var got ArtifactDeclarations
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal ArtifactDeclarations: %v", err)
	}

	if d := cmp.Diff(decls, got); d != "" {
		t.Errorf("ArtifactDeclarations round-trip mismatch (-want +got):\n%s", d)
	}
}

func TestTaskSpec_WithArtifactDeclarations(t *testing.T) {
	spec := TaskSpec{
		Artifacts: &ArtifactDeclarations{
			Inputs: []ArtifactDeclaration{
				{Name: "source"},
			},
			Outputs: []ArtifactDeclaration{
				{Name: "image", Subject: true},
			},
		},
		Steps: []Step{{
			Name:  "build",
			Image: "golang:1.22",
		}},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("failed to marshal TaskSpec: %v", err)
	}

	var got TaskSpec
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal TaskSpec: %v", err)
	}

	if got.Artifacts == nil {
		t.Fatal("expected Artifacts to be non-nil after round-trip")
	}
	if d := cmp.Diff(spec.Artifacts, got.Artifacts); d != "" {
		t.Errorf("TaskSpec.Artifacts round-trip mismatch (-want +got):\n%s", d)
	}
}
