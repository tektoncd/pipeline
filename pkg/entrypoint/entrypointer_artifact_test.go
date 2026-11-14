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
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocibackend "github.com/tektoncd/pipeline/pkg/artifactstorage/oci"
)

func TestEntrypointer_ArtifactOutputUpload(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	registryHost := strings.TrimPrefix(srv.URL, "http://")

	provider := ocibackend.NewOCIProvider("",
		ocibackend.WithInsecure(true),
		ocibackend.WithRemoteOptions(remote.WithTransport(srv.Client().Transport)),
	)

	tmpDir := t.TempDir()
	stepMetadataDir := filepath.Join(tmpDir, "step-metadata")
	if err := os.MkdirAll(filepath.Join(stepMetadataDir, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output-artifacts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	e := Entrypointer{
		Command:  []string{"echo", "hello"},
		PostFile: filepath.Join(tmpDir, "postfile"),
		Waiter:   &fakeWaiter{},
		Runner: &writeFileRunner{
			path:    filepath.Join(outputDir, "sbom.json"),
			content: []byte(`{"components":[]}`),
		},
		PostWriter:      &fakePostWriter{},
		TerminationPath: filepath.Join(tmpDir, "termination"),
		StepMetadataDir: stepMetadataDir,
		ArtifactOutputs: []ArtifactOutput{
			{
				Name:       "sbom",
				Path:       outputDir,
				Repository: registryHost + "/artifacts/sbom",
				MediaType:  "application/vnd.cyclonedx+json",
			},
		},
		ArtifactProvider: provider,
	}

	if err := e.Go(); err != nil {
		t.Fatalf("Entrypointer.Go() error = %v", err)
	}

	termContent, err := os.ReadFile(filepath.Join(tmpDir, "termination"))
	if err != nil {
		t.Fatalf("failed to read termination file: %v", err)
	}
	termStr := string(termContent)
	if !strings.Contains(termStr, "artifact-sbom") {
		t.Errorf("termination message should contain artifact-sbom, got: %s", termStr)
	}
	if !strings.Contains(termStr, "artifacts/sbom@sha256:") {
		t.Errorf("termination message should contain artifact URI, got: %s", termStr)
	}
}

// writeFileRunner writes path during Run to simulate a step producing an artifact.
type writeFileRunner struct {
	path    string
	content []byte
}

func (r *writeFileRunner) Run(ctx context.Context, args ...string) error {
	return os.WriteFile(r.path, r.content, 0o644)
}
