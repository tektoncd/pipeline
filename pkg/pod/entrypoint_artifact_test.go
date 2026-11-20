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

package pod

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func TestArtifactEntrypointArgs(t *testing.T) {
	taskSpec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Outputs: []v1.ArtifactDeclaration{
				{Name: "sbom", MediaType: "application/vnd.cyclonedx+json"},
			},
		},
	}

	repository := "registry:5000/artifacts"
	insecure := true

	args := artifactEntrypointArgs(taskSpec, "oci", repository, insecure, 0)

	// Should contain -artifact_outputs flag with correct JSON
	found := false
	for i, arg := range args {
		if arg == "-artifact_outputs" {
			found = true
			if i+1 >= len(args) {
				t.Fatal("-artifact_outputs flag has no value")
			}
			val := args[i+1]
			// Verify it's valid JSON containing the expected values
			var parsed []map[string]interface{}
			if err := json.Unmarshal([]byte(val), &parsed); err != nil {
				t.Fatalf("failed to parse artifact outputs JSON: %v", err)
			}
			if len(parsed) != 1 {
				t.Fatalf("expected 1 output, got %d", len(parsed))
			}
			if parsed[0]["name"] != "sbom" {
				t.Errorf("expected name 'sbom', got %v", parsed[0]["name"])
			}
			if parsed[0]["mediaType"] != "application/vnd.cyclonedx+json" {
				t.Errorf("expected mediaType, got %v", parsed[0]["mediaType"])
			}
			if parsed[0]["repository"] != "registry:5000/artifacts/sbom" {
				t.Errorf("expected repository, got %v", parsed[0]["repository"])
			}
		}
	}
	if !found {
		t.Error("expected -artifact_outputs flag in args")
	}

	// Should contain -artifact_insecure flag
	if !contains(args, "-artifact_insecure") {
		t.Error("expected -artifact_insecure flag in args")
	}
}

func TestArtifactEntrypointArgs_NoArtifacts(t *testing.T) {
	taskSpec := &v1.TaskSpec{}
	args := artifactEntrypointArgs(taskSpec, "oci", "registry:5000/artifacts", false, 0)
	if len(args) != 0 {
		t.Errorf("expected empty args for task without artifacts, got %v", args)
	}
}

func TestArtifactEntrypointArgs_InlineThreshold(t *testing.T) {
	taskSpec := &v1.TaskSpec{
		Artifacts: &v1.ArtifactDeclarations{
			Outputs: []v1.ArtifactDeclaration{
				{Name: "data", Type: v1.ArtifactTypeContent},
			},
		},
	}

	args := artifactEntrypointArgs(taskSpec, "oci", "registry:5000/artifacts", false, 512)
	if !contains(args, "-artifact_inline_threshold") {
		t.Error("expected -artifact_inline_threshold flag")
	}
	if !contains(args, "512") {
		t.Error("expected threshold value 512 in args")
	}

	argsNoThreshold := artifactEntrypointArgs(taskSpec, "oci", "registry:5000/artifacts", false, 0)
	if contains(argsNoThreshold, "-artifact_inline_threshold") {
		t.Error("should not emit -artifact_inline_threshold when threshold is 0")
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if strings.Contains(v, s) {
			return true
		}
	}
	return false
}
