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

package v1beta1

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func TestToStep_WithArtifacts(t *testing.T) {
	spec := &StepActionSpec{
		Image:  "buildah:latest",
		Script: "buildah push $(params.IMAGE)",
		Results: []v1.StepResult{
			{Name: "IMAGE_URL"},
		},
		Artifacts: &v1.StepArtifacts{
			Outputs: []v1.ArtifactDeclaration{
				{Name: "image", Type: v1.ArtifactTypeReference, Description: "The built image"},
				{Name: "sbom", Type: v1.ArtifactTypeContent},
			},
		},
	}

	step := spec.ToStep()

	if step.Artifacts == nil {
		t.Fatal("expected Artifacts to be set on Step, got nil")
	}
	if got := len(step.Artifacts.Outputs); got != 2 {
		t.Fatalf("expected 2 artifact outputs, got %d", got)
	}
	if step.Artifacts.Outputs[0].Name != "image" {
		t.Errorf("expected first output name 'image', got %q", step.Artifacts.Outputs[0].Name)
	}
	if step.Artifacts.Outputs[0].Type != v1.ArtifactTypeReference {
		t.Errorf("expected first output type 'reference', got %q", step.Artifacts.Outputs[0].Type)
	}
	if step.Artifacts.Outputs[1].Name != "sbom" {
		t.Errorf("expected second output name 'sbom', got %q", step.Artifacts.Outputs[1].Name)
	}
}

func TestToStep_WithoutArtifacts(t *testing.T) {
	spec := &StepActionSpec{
		Image:  "alpine",
		Script: "echo hello",
	}

	step := spec.ToStep()

	if step.Artifacts != nil {
		t.Errorf("expected Artifacts to be nil, got %v", step.Artifacts)
	}
}
