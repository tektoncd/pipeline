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
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
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
	source      *v1alpha1.ConfigSource
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

// Source is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint.
func (br *ResolvedResource) Source() *v1alpha1.ConfigSource {
	return br.source
}

// GetEntry accepts a keychain and options for the request and returns
// either a successfully resolved bundle entry or an error.
func GetEntry(ctx context.Context, keychain authn.Keychain, opts RequestOptions) (*ResolvedResource, error) {
	img, err := retrieveImage(ctx, keychain, opts.Bundle)
	if err != nil {
		return nil, err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("could not parse image manifest: %w", err)
	}

	if err := checkImageCompliance(opts.Bundle, manifest); err != nil {
		return nil, err
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

		if opts.Kind == lKind && opts.EntryName == lName {
			obj, err := readTarLayer(layerMap[l.Digest.String()])
			if err != nil {
				// This could still be a raw layer so try to read it as that instead.
				obj, err = readRawLayer(layers[idx])
			}
			return &ResolvedResource{
				data: obj,
				annotations: map[string]string{
					ResolverAnnotationKind:       lKind,
					ResolverAnnotationName:       lName,
					ResolverAnnotationAPIVersion: l.Annotations[BundleAnnotationAPIVersion],
				},
			}, nil
		}
	}
	return nil, fmt.Errorf("could not find object in image with kind: %s and name: %s", opts.Kind, opts.EntryName)
}

// retrieveImage will fetch the image's contents and manifest.
func retrieveImage(ctx context.Context, keychain authn.Keychain, ref string) (v1.Image, error) {
	imgRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("%s is an unparseable image reference: %w", ref, err)
	}
	return remote.Image(imgRef, remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx))
}

// checkImageCompliance will perform common checks to ensure the Tekton Bundle is compliant to our spec.
func checkImageCompliance(ref string, manifest *v1.Manifest) error {
	// Check the manifest's layers to ensure there are a maximum of 10.
	if len(manifest.Layers) > MaximumBundleObjects {
		return fmt.Errorf("bundle %s contained more than the maximum %d allow objects", ref, MaximumBundleObjects)
	}

	// Ensure each layer complies to the spec.
	for _, l := range manifest.Layers {
		refDigest := fmt.Sprintf("%s:%s", ref, l.Digest.String())
		if _, ok := l.Annotations[BundleAnnotationAPIVersion]; !ok {
			return fmt.Errorf("invalid tekton bundle: %s does not contain a %s annotation", refDigest, BundleAnnotationKind)
		}

		if _, ok := l.Annotations[BundleAnnotationName]; !ok {
			return fmt.Errorf("invalid tekton bundle: %s does not contain a %s annotation", refDigest, BundleAnnotationName)
		}

		kind, ok := l.Annotations[BundleAnnotationKind]
		if !ok {
			return fmt.Errorf("invalid tekton bundle: %s does not contain a %s annotation", refDigest, BundleAnnotationKind)
		}
		if strings.TrimSuffix(strings.ToLower(kind), "s") != kind {
			return fmt.Errorf("invalid tekton bundle: %s annotation for %s must be lowercased and singular, found %s", BundleAnnotationKind, refDigest, kind)
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
	if _, err := treader.Read(contents); err != nil && err != io.EOF {
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

	contents, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("could not read contents of image layer: %w", err)
	}

	return contents, nil
}
