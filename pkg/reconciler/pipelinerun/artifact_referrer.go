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

package pipelinerun

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1oci "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// TaskArtifactResult holds an artifact output from a completed task along with metadata.
type TaskArtifactResult struct {
	TaskName  string
	Artifact  v1.Artifact
	MediaType string // e.g., "application/vnd.cyclonedx+json"
}

// AttachReferrers creates OCI referrer manifests linking non-subject artifacts
// to the subject artifact. This implements the OCI referrers API
// for supply chain metadata (SBOMs, test results, etc.) attached to a build image.
func AttachReferrers(ctx context.Context, artifacts []TaskArtifactResult, insecure bool, opts ...remote.Option) error {
	// Find the subject artifact
	var subject *TaskArtifactResult
	var referrers []TaskArtifactResult

	for i := range artifacts {
		if artifacts[i].Artifact.Subject {
			subject = &artifacts[i]
		} else {
			referrers = append(referrers, artifacts[i])
		}
	}

	if subject == nil {
		// No build output — nothing to attach referrers to
		return nil
	}

	if len(subject.Artifact.Values) == 0 {
		return fmt.Errorf("build output artifact %q has no values", subject.Artifact.Name)
	}

	// Parse the subject reference
	nameOpts := []name.Option{}
	if insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}

	subjectRef, err := name.ParseReference(subject.Artifact.Values[0].Uri, nameOpts...)
	if err != nil {
		return fmt.Errorf("parsing subject reference %q: %w", subject.Artifact.Values[0].Uri, err)
	}

	// Get the subject descriptor
	remoteOpts := append([]remote.Option{remote.WithContext(ctx)}, opts...)
	subjectDesc, err := remote.Get(subjectRef, remoteOpts...)
	if err != nil {
		return fmt.Errorf("getting subject descriptor: %w", err)
	}

	// For each referrer artifact, push a referrer manifest
	for _, ref := range referrers {
		if len(ref.Artifact.Values) == 0 {
			continue
		}

		artifactType := ref.MediaType
		if artifactType == "" {
			artifactType = "application/vnd.tekton.artifact.v1"
		}

		// Create an empty OCI manifest with annotations and subject.
		// Subject must be applied last because each mutate wrapper
		// overwrites manifest.Subject with its own field in compute().
		img := empty.Image
		img = mutate.MediaType(img, types.OCIManifestSchema1)
		img = mutate.Annotations(img, map[string]string{
			"org.opencontainers.image.artifact.type": artifactType,
			"tekton.dev/artifact.name":               ref.Artifact.Name,
			"tekton.dev/artifact.task":               ref.TaskName,
			"tekton.dev/artifact.uri":                ref.Artifact.Values[0].Uri,
		}).(v1oci.Image)
		img = mutate.Subject(img, v1oci.Descriptor{
			MediaType: subjectDesc.MediaType,
			Size:      subjectDesc.Size,
			Digest:    subjectDesc.Digest,
		}).(v1oci.Image)

		// Push the referrer manifest to the subject's repository
		digest, err := img.Digest()
		if err != nil {
			return fmt.Errorf("computing referrer digest: %w", err)
		}

		repo := subjectRef.Context()
		referrerRef := repo.Digest(digest.String())

		if err := remote.Write(referrerRef, img, remoteOpts...); err != nil {
			return fmt.Errorf("pushing referrer for artifact %q: %w", ref.Artifact.Name, err)
		}
	}

	return nil
}
