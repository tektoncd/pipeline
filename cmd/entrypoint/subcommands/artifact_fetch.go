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

package subcommands

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/artifactstorage"
	ocibackend "github.com/tektoncd/pipeline/pkg/artifactstorage/oci"
	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// ArtifactFetchCommand is the name of the artifact-fetch subcommand.
const ArtifactFetchCommand = "artifact-fetch"

// ArtifactsFetchedMarker is the marker file written by the artifact-fetch
// init container after all content input artifacts have been downloaded.
// Steps check for this file to skip redundant downloads.
var ArtifactsFetchedMarker = "/tekton/artifacts/.fetched"

// ArtifactsFetchedMarker is the file the init container writes after
// successfully downloading all content input artifacts. Steps check for
// this file to avoid redundant downloads.

// artifactFetch downloads all content input artifacts into the shared
// /tekton/artifacts volume. It is run as an init container so that all
// artifacts are available before any Step executes.
//
// Usage: entrypoint artifact-fetch <inputs-json> [flags...]
//
// Supported flags (positional after <inputs-json>):
//
//	--insecure            allow plain HTTP for artifact registry
//	--backend=<name>      storage backend (default: oci)
//	--docker-config=<json> Docker config JSON for registry auth
func artifactFetch(args []string) error {
	if len(args) < 1 {
		return errors.New("artifact-fetch requires at least 1 argument: <inputs-json>")
	}

	inputsJSON := args[0]
	insecure := false
	backend := "oci"
	dockerConfig := ""

	for _, arg := range args[1:] {
		switch {
		case arg == "--insecure":
			insecure = true
		case len(arg) > len("--backend=") && arg[:len("--backend=")] == "--backend=":
			backend = arg[len("--backend="):]
		case len(arg) > len("--docker-config=") && arg[:len("--docker-config=")] == "--docker-config=":
			dockerConfig = arg[len("--docker-config="):]
		default:
			return fmt.Errorf("artifact-fetch: unknown flag %q", arg)
		}
	}

	inputs, err := entrypoint.ParseArtifactInputs(inputsJSON)
	if err != nil {
		return fmt.Errorf("parsing artifact inputs: %w", err)
	}

	if dockerConfig != "" {
		cfgDir := filepath.Join(os.TempDir(), "tekton-artifact-docker")
		if err := os.MkdirAll(cfgDir, 0o700); err == nil {
			if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(dockerConfig), 0o600); err == nil {
				os.Setenv("DOCKER_CONFIG", cfgDir)
			}
		}
	}

	registry := artifactstorage.NewRegistry()
	ociProvider := ocibackend.NewOCIProvider("",
		ocibackend.WithInsecure(insecure),
		ocibackend.WithRemoteOptions(remote.WithAuthFromKeychain(authn.DefaultKeychain)),
	)
	if err := registry.Register(ociProvider); err != nil {
		return fmt.Errorf("registering OCI artifact provider: %w", err)
	}

	provider, err := registry.Get(backend)
	if err != nil {
		return fmt.Errorf("resolving artifact backend %q: %w", backend, err)
	}

	ctx := context.Background()
	downloaded := 0
	for _, input := range inputs {
		if input.Reference {
			continue
		}
		if err := os.MkdirAll(input.Path, 0o755); err != nil {
			return fmt.Errorf("creating directory for artifact %q: %w", input.Name, err)
		}
		if err := entrypoint.DownloadArtifact(ctx, input, provider); err != nil {
			return fmt.Errorf("downloading artifact %q: %w", input.Name, err)
		}
		downloaded++
		log.Printf("artifact-fetch: downloaded %q to %s", input.Name, input.Path)
	}

	// Write marker file so Steps know artifacts are already fetched.
	if err := os.MkdirAll(filepath.Dir(ArtifactsFetchedMarker), 0o755); err != nil {
		return fmt.Errorf("creating marker directory: %w", err)
	}
	if err := os.WriteFile(ArtifactsFetchedMarker, []byte(fmt.Sprintf("fetched %d artifacts\n", downloaded)), 0o600); err != nil {
		return fmt.Errorf("writing fetched marker: %w", err)
	}

	log.Printf("artifact-fetch: completed, %d content artifacts downloaded", downloaded)
	return nil
}
