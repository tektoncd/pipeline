/*
Copyright 2022 The Tekton Authors
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

package bundle

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// MaximumBundleObjects defines the maximum number of objects in a bundle
	MaximumBundleObjects = 20
)

// RequestOptions are the options used to request a resource from
// a remote bundle.
type RequestOptions struct {
	ServiceAccount string
	Bundle         string
	EntryName      string
	Kind           string
}

// ResolvedResource wraps the content of a matched entry in a bundle.
type ResolvedResource struct {
	data        []byte
	annotations map[string]string
	source      *pipelinev1.RefSource
}

var _ framework.ResolvedResource = &ResolvedResource{}

// Data returns the bytes of the resource fetched from the bundle.
func (br *ResolvedResource) Data() []byte {
	return br.data
}

// Annotations returns the annotations from the bundle that are relevant
// to resolution.
func (br *ResolvedResource) Annotations() map[string]string {
	return br.annotations
}

// RefSource is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint.
func (br *ResolvedResource) RefSource() *pipelinev1.RefSource {
	return br.source
}

// GetEntry accepts a keychain and options for the request and returns
// either a successfully resolved bundle entry or an error.
func GetEntry(ctx context.Context, keychain authn.Keychain, opts RequestOptions) (*ResolvedResource, error) {
	uri, img, err := retrieveImage(ctx, keychain, opts.Bundle)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve the oci image: %w", err)
	}

	h, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("cannot get the oci digest: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("could not parse image manifest: %w", err)
	}

	if err := checkImageCompliance(manifest); err != nil {
		return nil, fmt.Errorf("invalid tekton bundle %s, error: %w", opts.Bundle, err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("could not read image layers: %w", err)
	}

	layerMap := map[string]v1.Layer{}
	for _, l := range layers {
		digest, err := l.Digest()
		if err != nil {
			return nil, fmt.Errorf("failed to find digest for layer: %w", err)
		}
		layerMap[digest.String()] = l
	}

	for idx, l := range manifest.Layers {
		lKind := l.Annotations[BundleAnnotationKind]
		lName := l.Annotations[BundleAnnotationName]

		if strings.ToLower(opts.Kind) == strings.ToLower(lKind) && opts.EntryName == lName {
			obj, err := readTarLayer(layerMap[l.Digest.String()])
			if err != nil {
				// This could still be a raw layer so try to read it as that instead.
				obj, _ = readRawLayer(layers[idx])
			}
			return &ResolvedResource{
				data: obj,
				annotations: map[string]string{
					ResolverAnnotationKind:       lKind,
					ResolverAnnotationName:       lName,
					ResolverAnnotationAPIVersion: l.Annotations[BundleAnnotationAPIVersion],
				},
				source: &pipelinev1.RefSource{
					URI: uri,
					Digest: map[string]string{
						h.Algorithm: h.Hex,
					},
					EntryPoint: opts.EntryName,
				},
			}, nil
		}
	}
	return nil, fmt.Errorf("could not find object in image with kind: %s and name: %s", opts.Kind, opts.EntryName)
}

// retrieveImage will fetch the image's url, contents and manifest.
func retrieveImage(ctx context.Context, keychain authn.Keychain, ref string) (string, v1.Image, error) {
	imgRef, err := name.ParseReference(ref)
	if err != nil {
		return "", nil, fmt.Errorf("%s is an unparseable image reference: %w", ref, err)
	}

	img, err := remote.Image(imgRef, remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx))
	return imgRef.Context().Name(), img, err
}

// checkImageCompliance will perform common checks to ensure the Tekton Bundle is compliant to our spec.
func checkImageCompliance(manifest *v1.Manifest) error {
	// Check the manifest's layers to ensure there are a maximum of 10.
	if len(manifest.Layers) > MaximumBundleObjects {
		return fmt.Errorf("contained more than the maximum %d allow objects", MaximumBundleObjects)
	}

	// Ensure each layer complies to the spec.
	for i, l := range manifest.Layers {
		if _, ok := l.Annotations[BundleAnnotationAPIVersion]; !ok {
			return fmt.Errorf("the layer %v does not contain a %s annotation", i, BundleAnnotationAPIVersion)
		}

		if _, ok := l.Annotations[BundleAnnotationName]; !ok {
			return fmt.Errorf("the layer %v does not contain a %s annotation", i, BundleAnnotationName)
		}

		kind, ok := l.Annotations[BundleAnnotationKind]
		if !ok {
			return fmt.Errorf("the layer %v does not contain a %s annotation", i, BundleAnnotationKind)
		}
		if strings.TrimSuffix(strings.ToLower(kind), "s") != kind {
			return fmt.Errorf("the layer %v the annotation %s must be lowercased and singular, found %s", i, BundleAnnotationKind, kind)
		}
	}

	return nil
}

// Utility function to read out the contents of an image layer, assumed to be a tarball, as bytes.
func readTarLayer(layer v1.Layer) ([]byte, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to read image layer: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	// If the user bundled this up as a tar file then we need to untar it.
	treader := tar.NewReader(rc)
	header, err := treader.Next()
	if err != nil {
		return nil, fmt.Errorf("layer is not a tarball")
	}

	contents := make([]byte, header.Size)
	if _, err := treader.Read(contents); err != nil && !errors.Is(err, io.EOF) {
		// We only allow 1 resource per layer so this tar bundle should have one and only one file.
		return nil, fmt.Errorf("failed to read tar bundle: %w", err)
	}

	return contents, nil
}

// Utility function to read out the contents of an image layer, assumed to be raw bytes, as bytes.
func readRawLayer(layer v1.Layer) ([]byte, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to read image layer: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	contents, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("could not read contents of image layer: %w", err)
	}

	return contents, nil
}
