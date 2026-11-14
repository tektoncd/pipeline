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

package resources_test

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

func TestApplyArtifactDeclarationPaths(t *testing.T) {
	spec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Inputs: []v1.ArtifactDeclaration{
				{Name: "source"},
			},
			Outputs: []v1.ArtifactDeclaration{
				{Name: "results"},
			},
		},
		Steps: []v1.Step{{
			Name:  "test",
			Image: "busybox",
			Script: `
cd $(inputs.source.path)
mkdir -p $(outputs.results.path)
echo done > $(outputs.results.path)/data.json
`,
		}},
	}

	got := resources.ApplyArtifactDeclarationPaths(spec, "")

	expectedScript := `
cd /tekton/artifacts/inputs/source
mkdir -p /tekton/artifacts/outputs/results
echo done > /tekton/artifacts/outputs/results/data.json
`
	if got.Steps[0].Script != expectedScript {
		t.Errorf("script mismatch:\ngot:  %q\nwant: %q", got.Steps[0].Script, expectedScript)
	}
}

func TestApplyArtifactDeclarationPaths_WithResolvedInputs(t *testing.T) {
	spec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Inputs: []v1.ArtifactDeclaration{
				{Name: "image"},
			},
		},
		Steps: []v1.Step{{
			Name:  "show",
			Image: "alpine",
			Script: `
echo "URI: $(inputs.image.uri)"
echo "Digest: $(inputs.image.digest)"
echo "Path: $(inputs.image.path)"
`,
		}},
	}

	resolvedJSON := `[{"name":"image","uri":"registry.example.com/app@sha256:abc123","path":"/tekton/artifacts/inputs/image","digest":{"sha256":"abc123"}}]`
	got := resources.ApplyArtifactDeclarationPaths(spec, resolvedJSON)

	expectedScript := `
echo "URI: registry.example.com/app@sha256:abc123"
echo "Digest: sha256:abc123"
echo "Path: /tekton/artifacts/inputs/image"
`
	if got.Steps[0].Script != expectedScript {
		t.Errorf("script mismatch:\ngot:  %q\nwant: %q", got.Steps[0].Script, expectedScript)
	}
}

func TestApplyArtifactDeclarationPaths_EmptyResolvedInputs(t *testing.T) {
	spec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Inputs: []v1.ArtifactDeclaration{
				{Name: "image"},
			},
		},
		Steps: []v1.Step{{
			Name:   "show",
			Image:  "alpine",
			Script: `echo "$(inputs.image.uri)"`,
		}},
	}

	got := resources.ApplyArtifactDeclarationPaths(spec, "")
	if got.Steps[0].Script != `echo "$(inputs.image.uri)"` {
		t.Errorf("script should not substitute uri with empty resolved inputs, got: %q", got.Steps[0].Script)
	}
}

func TestApplyArtifactDeclarationPaths_NoArtifacts(t *testing.T) {
	spec := &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:   "test",
			Image:  "busybox",
			Script: "echo hello",
		}},
	}

	got := resources.ApplyArtifactDeclarationPaths(spec, "")
	if got.Steps[0].Script != "echo hello" {
		t.Errorf("script should not change without artifacts, got: %q", got.Steps[0].Script)
	}
}

func TestApplyArtifactDeclarationPaths_ValueInheritsTypeFromStep(t *testing.T) {
	spec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Outputs: []v1.ArtifactDeclaration{
				{
					Name:    "image",
					Subject: true,
					Value:   "$(steps.build.artifacts.image)",
				},
			},
		},
		Steps: []v1.Step{{
			Name:  "build",
			Image: "buildah",
			Artifacts: &v1.StepArtifacts{
				Outputs: []v1.ArtifactDeclaration{
					{Name: "image", Type: v1.ArtifactTypeReference},
				},
			},
			Script: `echo "$(outputs.image.uri)"`,
		}},
	}

	got := resources.ApplyArtifactDeclarationPaths(spec, "")
	expectedScript := `echo "/tekton/artifacts/outputs/image.uri"`
	if got.Steps[0].Script != expectedScript {
		t.Errorf("expected reference path for value-inherited type:\ngot:  %q\nwant: %q", got.Steps[0].Script, expectedScript)
	}
}

func TestApplyArtifactDeclarationPaths_ValueInheritsContentType(t *testing.T) {
	spec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Outputs: []v1.ArtifactDeclaration{
				{
					Name:  "logs",
					Value: "$(steps.test.artifacts.logs)",
				},
			},
		},
		Steps: []v1.Step{{
			Name:  "test",
			Image: "golang",
			Artifacts: &v1.StepArtifacts{
				Outputs: []v1.ArtifactDeclaration{
					{Name: "logs", Type: v1.ArtifactTypeContent},
				},
			},
			Script: `echo "$(outputs.logs.path)"`,
		}},
	}

	got := resources.ApplyArtifactDeclarationPaths(spec, "")
	expectedScript := `echo "/tekton/artifacts/outputs/logs"`
	if got.Steps[0].Script != expectedScript {
		t.Errorf("expected content path for value-inherited type:\ngot:  %q\nwant: %q", got.Steps[0].Script, expectedScript)
	}
}

func TestApplyArtifactDeclarationPaths_ExplicitTypeOverridesStepType(t *testing.T) {
	spec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Outputs: []v1.ArtifactDeclaration{
				{
					Name:  "image",
					Type:  v1.ArtifactTypeReference,
					Value: "$(steps.build.artifacts.image)",
				},
			},
		},
		Steps: []v1.Step{{
			Name:  "build",
			Image: "buildah",
			Artifacts: &v1.StepArtifacts{
				Outputs: []v1.ArtifactDeclaration{
					{Name: "image", Type: v1.ArtifactTypeContent},
				},
			},
			Script: `echo "$(outputs.image.uri)"`,
		}},
	}

	got := resources.ApplyArtifactDeclarationPaths(spec, "")
	expectedScript := `echo "/tekton/artifacts/outputs/image.uri"`
	if got.Steps[0].Script != expectedScript {
		t.Errorf("explicit type should override step type:\ngot:  %q\nwant: %q", got.Steps[0].Script, expectedScript)
	}
}

func TestApplyArtifacts_StepScopedArtifactPaths(t *testing.T) {
	spec := &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:  "build",
			Image: "buildah",
			Artifacts: &v1.StepArtifacts{
				Outputs: []v1.ArtifactDeclaration{
					{Name: "image", Type: v1.ArtifactTypeReference},
					{Name: "logs", Type: v1.ArtifactTypeContent},
				},
			},
			Script: `
buildah push myimage
echo "myimage@sha256:abc" > $(step.artifacts.outputs.image.uri)
cp /tmp/build.log $(step.artifacts.outputs.logs.path)/
`,
		}},
	}

	got := resources.ApplyArtifacts(spec)
	expectedScript := `
buildah push myimage
echo "myimage@sha256:abc" > /tekton/artifacts/outputs/image.uri
cp /tmp/build.log /tekton/artifacts/outputs/logs/
`
	if got.Steps[0].Script != expectedScript {
		t.Errorf("step-scoped artifact path mismatch:\ngot:  %q\nwant: %q", got.Steps[0].Script, expectedScript)
	}
}
