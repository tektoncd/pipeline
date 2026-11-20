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

package oci

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"

	v1types "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
	"github.com/tektoncd/pipeline/pkg/artifactstorage"
)

// OCIProvider implements the Provider and ContentAddressedProvider interfaces
// using OCI registries via go-containerregistry.
type OCIProvider struct {
	repository string
	insecure   bool
	remoteOpts []remote.Option
}

var _ artifactstorage.Provider = (*OCIProvider)(nil)
var _ artifactstorage.ContentAddressedProvider = (*OCIProvider)(nil)

// Option configures the OCI provider.
type Option func(*OCIProvider)

// WithInsecure allows plain HTTP transport.
func WithInsecure(insecure bool) Option {
	return func(p *OCIProvider) {
		p.insecure = insecure
	}
}

// WithRemoteOptions appends remote.Option values used for all registry calls.
func WithRemoteOptions(opts ...remote.Option) Option {
	return func(p *OCIProvider) {
		p.remoteOpts = append(p.remoteOpts, opts...)
	}
}

// NewOCIProvider creates an OCI-backed artifact storage provider.
func NewOCIProvider(repository string, opts ...Option) *OCIProvider {
	p := &OCIProvider{
		repository: repository,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *OCIProvider) Name() string { return "oci" }

// ContentAddressed reports that OCI pulls are digest-pinned and therefore
// self-verifying, so core skips wrapping Fetch in a verifying reader.
func (p *OCIProvider) ContentAddressed() bool { return true }

func (p *OCIProvider) nameOpts() []name.Option {
	if p.insecure {
		return []name.Option{name.Insecure}
	}
	return nil
}

func (p *OCIProvider) buildRemoteOpts(ctx context.Context) []remote.Option {
	opts := make([]remote.Option, 0, len(p.remoteOpts)+2)
	if ctx != nil {
		opts = append(opts, remote.WithContext(ctx))
	}
	opts = append(opts, p.remoteOpts...)
	if p.insecure {
		opts = append(opts, remote.WithTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user opted in
		}))
	}
	return opts
}

func (p *OCIProvider) Store(ctx context.Context, opts artifactstorage.StoreOptions, content io.Reader) (*v1types.ArtifactStorageRef, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("reading content: %w", err)
	}

	h := sha256.New()
	h.Write(data)
	contentDigest := hex.EncodeToString(h.Sum(nil))

	layerMediaType := types.MediaType(opts.MediaType)
	if layerMediaType == "" {
		layerMediaType = types.OCILayer
	}

	layer, err := tarball.LayerFromReader(bytes.NewReader(data), tarball.WithMediaType(layerMediaType))
	if err != nil {
		return nil, fmt.Errorf("creating layer: %w", err)
	}

	img, err := mutate.AppendLayers(mutate.MediaType(empty.Image, types.OCIManifestSchema1), layer)
	if err != nil {
		return nil, fmt.Errorf("creating image with layer: %w", err)
	}

	repo := opts.Repository
	if repo == "" {
		repo = p.repository
	}

	parsedRepo, err := name.NewRepository(repo, p.nameOpts()...)
	if err != nil {
		return nil, fmt.Errorf("parsing repository %q: %w", repo, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("computing digest: %w", err)
	}

	ref := parsedRepo.Digest(digest.String())

	if err := remote.Write(ref, img, p.buildRemoteOpts(ctx)...); err != nil {
		return nil, fmt.Errorf("pushing artifact to %s: %w", ref.String(), err)
	}

	return &v1types.ArtifactStorageRef{
		Backend:     "oci",
		Location:    ref.String(),
		Digest:      "sha256:" + contentDigest,
		ContentType: string(layerMediaType),
	}, nil
}

func (p *OCIProvider) Fetch(ctx context.Context, ref *v1types.ArtifactStorageRef) (io.ReadCloser, error) {
	parsed, err := name.ParseReference(ref.Location, p.nameOpts()...)
	if err != nil {
		return nil, fmt.Errorf("parsing artifact reference %q: %w", ref.Location, err)
	}

	img, err := remote.Image(parsed, p.buildRemoteOpts(ctx)...)
	if err != nil {
		return nil, fmt.Errorf("pulling artifact from %s: %w", ref.Location, err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("getting layers: %w", err)
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("artifact at %s has no layers", ref.Location)
	}

	return layerReadCloser(layers[0])
}

func (p *OCIProvider) Delete(ctx context.Context, ref *v1types.ArtifactStorageRef) error {
	parsed, err := name.ParseReference(ref.Location, p.nameOpts()...)
	if err != nil {
		return fmt.Errorf("parsing artifact reference %q: %w", ref.Location, err)
	}
	return remote.Delete(parsed, p.buildRemoteOpts(ctx)...)
}

func (p *OCIProvider) Exists(ctx context.Context, ref *v1types.ArtifactStorageRef) (bool, error) {
	parsed, err := name.ParseReference(ref.Location, p.nameOpts()...)
	if err != nil {
		return false, fmt.Errorf("parsing artifact reference %q: %w", ref.Location, err)
	}

	_, err = remote.Head(parsed, p.buildRemoteOpts(ctx)...)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("checking artifact at %s: %w", ref.Location, err)
	}
	return true, nil
}

// layerReadCloser returns a ReadCloser for the compressed layer content.
func layerReadCloser(layer v1.Layer) (io.ReadCloser, error) {
	return layer.Compressed()
}

// isNotFound checks if the error indicates a 404 / not-found response.
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "NOT_FOUND") ||
		strings.Contains(err.Error(), "MANIFEST_UNKNOWN") ||
		strings.Contains(err.Error(), "404")
}
