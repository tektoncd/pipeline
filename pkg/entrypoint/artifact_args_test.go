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

package entrypoint

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseArtifactInputs(t *testing.T) {
	json := `[{"name":"source","uri":"registry:5000/artifacts/src@sha256:abc","path":"/workspace/source"}]`

	got, err := ParseArtifactInputs(json)
	if err != nil {
		t.Fatalf("ParseArtifactInputs() error = %v", err)
	}

	want := []ArtifactInput{
		{Name: "source", URI: "registry:5000/artifacts/src@sha256:abc", Path: "/workspace/source"},
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestParseArtifactOutputs(t *testing.T) {
	json := `[{"name":"sbom","path":"/workspace/sbom","repository":"registry:5000/artifacts/sbom","mediaType":"application/vnd.cyclonedx+json","buildOutput":false}]`

	got, err := ParseArtifactOutputs(json)
	if err != nil {
		t.Fatalf("ParseArtifactOutputs() error = %v", err)
	}

	want := []ArtifactOutput{
		{Name: "sbom", Path: "/workspace/sbom", Repository: "registry:5000/artifacts/sbom", MediaType: "application/vnd.cyclonedx+json"},
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestParseArtifactInputs_Empty(t *testing.T) {
	got, err := ParseArtifactInputs("")
	if err != nil {
		t.Fatalf("ParseArtifactInputs() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}
