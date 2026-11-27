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

	args := artifactEntrypointArgs(taskSpec, "oci", repository, insecure, 0, ArtifactProvenance{})

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
	args := artifactEntrypointArgs(taskSpec, "oci", "registry:5000/artifacts", false, 0, ArtifactProvenance{})
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

	args := artifactEntrypointArgs(taskSpec, "oci", "registry:5000/artifacts", false, 512, ArtifactProvenance{})
	if !contains(args, "-artifact_inline_threshold") {
		t.Error("expected -artifact_inline_threshold flag")
	}
	if !contains(args, "512") {
		t.Error("expected threshold value 512 in args")
	}

	argsNoThreshold := artifactEntrypointArgs(taskSpec, "oci", "registry:5000/artifacts", false, 0, ArtifactProvenance{})
	if contains(argsNoThreshold, "-artifact_inline_threshold") {
		t.Error("should not emit -artifact_inline_threshold when threshold is 0")
	}
}

func TestStripArtifactInputArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "removes artifact_inputs and its value",
			args: []string{"-artifact_outputs", `[{"name":"out"}]`, "-artifact_inputs", `[{"name":"in"}]`, "-artifact_backend", "oci"},
			want: []string{"-artifact_outputs", `[{"name":"out"}]`, "-artifact_backend", "oci"},
		},
		{
			name: "no artifact_inputs present",
			args: []string{"-artifact_outputs", `[{"name":"out"}]`, "-artifact_backend", "oci"},
			want: []string{"-artifact_outputs", `[{"name":"out"}]`, "-artifact_backend", "oci"},
		},
		{
			name: "empty args",
			args: []string{},
			want: nil,
		},
		{
			name: "only artifact_inputs",
			args: []string{"-artifact_inputs", `[{"name":"in"}]`},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripArtifactInputArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Fatalf("stripArtifactInputArgs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("stripArtifactInputArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestArtifactFetchInit(t *testing.T) {
	image := "gcr.io/tekton/entrypoint:latest"
	inputsJSON := `[{"name":"data","uri":"registry:5000/artifacts/data@sha256:abc","path":"/tekton/artifacts/inputs/data"}]`

	t.Run("basic init container", func(t *testing.T) {
		c := artifactFetchInit(image, inputsJSON, "oci", false, "", SecurityContextConfig{}, false)
		if c.Name != ContainerNameArtifactFetch {
			t.Errorf("expected name %q, got %q", ContainerNameArtifactFetch, c.Name)
		}
		if c.Image != image {
			t.Errorf("expected image %q, got %q", image, c.Image)
		}
		if c.Command[0] != entrypointBinary {
			t.Errorf("expected command[0] %q, got %q", entrypointBinary, c.Command[0])
		}
		if c.Command[1] != "artifact-fetch" {
			t.Errorf("expected command[1] 'artifact-fetch', got %q", c.Command[1])
		}
		if c.Command[2] != inputsJSON {
			t.Errorf("expected command[2] to be inputs JSON")
		}
		// Should NOT have --insecure
		for _, arg := range c.Command {
			if arg == "--insecure" {
				t.Error("should not have --insecure flag")
			}
		}
	})

	t.Run("with insecure flag", func(t *testing.T) {
		c := artifactFetchInit(image, inputsJSON, "oci", true, "", SecurityContextConfig{}, false)
		found := false
		for _, arg := range c.Command {
			if arg == "--insecure" {
				found = true
			}
		}
		if !found {
			t.Error("expected --insecure flag")
		}
	})

	t.Run("with docker config", func(t *testing.T) {
		dockerCfg := `{"auths":{"registry:5000":{}}}`
		c := artifactFetchInit(image, inputsJSON, "oci", false, dockerCfg, SecurityContextConfig{}, false)
		found := false
		for _, arg := range c.Command {
			if strings.HasPrefix(arg, "--docker-config=") {
				found = true
				if !strings.Contains(arg, "registry:5000") {
					t.Errorf("expected docker config in arg, got %q", arg)
				}
			}
		}
		if !found {
			t.Error("expected --docker-config flag")
		}
	})

	t.Run("with security context", func(t *testing.T) {
		c := artifactFetchInit(image, inputsJSON, "oci", false, "", SecurityContextConfig{SetSecurityContext: true}, false)
		if c.SecurityContext == nil {
			t.Error("expected security context to be set")
		}
	})

	t.Run("volume mounts include artifacts dir", func(t *testing.T) {
		c := artifactFetchInit(image, inputsJSON, "oci", false, "", SecurityContextConfig{}, false)
		foundBin := false
		foundArtifacts := false
		for _, vm := range c.VolumeMounts {
			if vm.Name == binVolumeName {
				foundBin = true
			}
			if vm.Name == "tekton-internal-artifacts" {
				foundArtifacts = true
			}
		}
		if !foundBin {
			t.Error("expected bin volume mount")
		}
		if !foundArtifacts {
			t.Error("expected artifacts volume mount")
		}
	})
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if strings.Contains(v, s) {
			return true
		}
	}
	return false
}
