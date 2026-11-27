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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/static"
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

// ContentAddressed returns true because Fetch already verifies ref.Digest
// internally. This tells VerifiedFetch to skip its own redundant check.
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

const artifactConfigMediaType = "application/vnd.tekton.artifact.config.v1+json"

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
		layerMediaType = types.MediaType(detectContentType(data))
	}

	layer := static.NewLayer(data, layerMediaType)

	img, err := mutate.AppendLayers(mutate.MediaType(empty.Image, types.OCIManifestSchema1), layer)
	if err != nil {
		return nil, fmt.Errorf("creating image with layer: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	cfgBlob := struct {
		Created  string `json:"created"`
		Artifact struct {
			Name        string `json:"name"`
			TaskRun     string `json:"taskrun,omitempty"`
			PipelineRun string `json:"pipelinerun,omitempty"`
			Namespace   string `json:"namespace,omitempty"`
			ContentType string `json:"contentType,omitempty"`
		} `json:"artifact"`
	}{
		Created: now,
	}
	cfgBlob.Artifact.Name = opts.ArtifactName
	cfgBlob.Artifact.ContentType = string(layerMediaType)

	annotations := map[string]string{
		"dev.tekton.artifact/name":    opts.ArtifactName,
		"dev.tekton.artifact/created": now,
	}

	if opts.TaskRun != "" {
		cfgBlob.Artifact.TaskRun = opts.TaskRun
		annotations["dev.tekton.artifact/taskrun"] = opts.TaskRun
	}
	if opts.PipelineRun != "" {
		cfgBlob.Artifact.PipelineRun = opts.PipelineRun
		annotations["dev.tekton.artifact/pipelinerun"] = opts.PipelineRun
	}
	if opts.Namespace != "" {
		cfgBlob.Artifact.Namespace = opts.Namespace
		annotations["dev.tekton.artifact/namespace"] = opts.Namespace
	}

	cfgJSON, err := json.Marshal(cfgBlob)
	if err != nil {
		return nil, fmt.Errorf("marshaling config blob: %w", err)
	}

	img, err = mutate.AppendLayers(img, static.NewLayer(cfgJSON, artifactConfigMediaType))
	if err != nil {
		return nil, fmt.Errorf("appending config layer: %w", err)
	}
	img = mutate.ConfigMediaType(img, artifactConfigMediaType)

	img = mutate.Annotations(img, annotations).(v1.Image)

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

	rc, err := layerReadCloser(layers[0])
	if err != nil {
		return nil, err
	}

	if digest := strings.TrimPrefix(ref.Digest, "sha256:"); digest != "" {
		return artifactstorage.NewVerifyingReader(rc, digest), nil
	}
	return rc, nil
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

func layerReadCloser(layer v1.Layer) (io.ReadCloser, error) {
	return layer.Compressed()
}

// detectContentType sniffs the content bytes and returns a MIME type.
// It improves on http.DetectContentType by recognising JSON and XML,
// which the standard library reports as "text/plain".
func detectContentType(data []byte) string {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) > 0 {
		switch trimmed[0] {
		case '{', '[':
			return "application/json"
		case '<':
			return "application/xml"
		}
	}
	ct := http.DetectContentType(data)
	if strings.HasPrefix(ct, "text/") {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

// isNotFound checks if the error indicates a 404 / not-found response
// by inspecting the structured transport.Error from go-containerregistry.
func isNotFound(err error) bool {
	var terr *transport.Error
	if !errors.As(err, &terr) {
		return false
	}
	if terr.StatusCode == http.StatusNotFound {
		return true
	}
	for _, d := range terr.Errors {
		if d.Code == transport.ManifestUnknownErrorCode || d.Code == transport.NameUnknownErrorCode {
			return true
		}
	}
	return false
}
