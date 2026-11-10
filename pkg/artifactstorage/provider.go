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

package artifactstorage

import (
	"context"
	"io"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
)

// Provider is the contract that every artifact storage backend must implement.
// Verification is performed by Tekton core, not by the provider — core wraps
// every Fetch in a digest-verifying reader by default.
type Provider interface {
	Name() string
	Store(ctx context.Context, opts StoreOptions, content io.Reader) (*v1.ArtifactStorageRef, error)
	Fetch(ctx context.Context, ref *v1.ArtifactStorageRef) (io.ReadCloser, error)
	Delete(ctx context.Context, ref *v1.ArtifactStorageRef) error
	Exists(ctx context.Context, ref *v1.ArtifactStorageRef) (bool, error)
}

// ContentAddressedProvider is optional. Providers whose transport is
// inherently content-addressed (e.g. OCI pulls pinned by digest) implement
// it so that core skips wrapping Fetch in a verifying reader, avoiding
// hashing content twice.
type ContentAddressedProvider interface {
	Provider
	ContentAddressed() bool
}

// StoreOptions carries metadata needed when storing an artifact.
type StoreOptions struct {
	// ArtifactName is the logical name of the artifact being stored.
	ArtifactName string
	// MediaType is the MIME type of the content (e.g. "application/vnd.oci.image.layer.v1.tar+gzip").
	MediaType string
	// Repository overrides the default repository for this store call.
	Repository string
	// Tag is the tag to apply to the stored artifact (optional; backends may ignore it).
	Tag string
	// Insecure allows plain HTTP transport when true.
	Insecure bool
}
