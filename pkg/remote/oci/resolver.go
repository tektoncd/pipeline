/*
Copyright 2020 The Tekton Authors

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
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	imgname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	ociremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"github.com/tektoncd/pipeline/pkg/remote"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// KindAnnotation is an OCI annotation for the bundle kind
	KindAnnotation = "dev.tekton.image.kind"
	// APIVersionAnnotation is an OCI annotation for the bundle version
	APIVersionAnnotation = "dev.tekton.image.apiVersion"
	// TitleAnnotation is an OCI annotation for the bundle title
	TitleAnnotation = "dev.tekton.image.name"
	// MaximumBundleObjects defines the maximum number of objects in a bundle
	MaximumBundleObjects = 10
)

// Resolver implements the Resolver interface using OCI images.
type Resolver struct {
	imageReference string
	keychain       authn.Keychain
	timeout        time.Duration
}

// NewResolver is a convenience function to return a new OCI resolver instance as a remote.Resolver with a short, 1m
// timeout for resolving an individual image.
func NewResolver(ref string, keychain authn.Keychain) remote.Resolver {
	return &Resolver{imageReference: ref, keychain: keychain, timeout: time.Second * 60}
}

// List retrieves a flat set of Tekton objects
func (o *Resolver) List() ([]remote.ResolvedObject, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	img, err := o.retrieveImage(timeoutCtx)
	if err != nil {
		return nil, err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("Could not parse image manifest: %w", err)
	}

	if err := o.checkImageCompliance(manifest); err != nil {
		return nil, err
	}

	contents := make([]remote.ResolvedObject, 0, len(manifest.Layers))
	for _, l := range manifest.Layers {
		contents = append(contents, remote.ResolvedObject{
			Kind:       l.Annotations[KindAnnotation],
			APIVersion: l.Annotations[APIVersionAnnotation],
			Name:       l.Annotations[TitleAnnotation],
		})
	}

	return contents, nil
}

// Get retrieves a specific object with the given Kind and name
func (o *Resolver) Get(kind, name string) (runtime.Object, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	img, err := o.retrieveImage(timeoutCtx)
	if err != nil {
		return nil, err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("could not parse image manifest: %w", err)
	}

	if err := o.checkImageCompliance(manifest); err != nil {
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
		lKind := l.Annotations[KindAnnotation]
		lName := l.Annotations[TitleAnnotation]

		if kind == lKind && name == lName {
			obj, err := readTarLayer(layerMap[l.Digest.String()])
			if err != nil {
				// This could still be a raw layer so try to read it as that instead.
				return readRawLayer(layers[idx])
			}
			return obj, nil
		}
	}
	return nil, fmt.Errorf("could not find object in image with kind: %s and name: %s", kind, name)
}

// retrieveImage will fetch the image's contents and manifest.
func (o *Resolver) retrieveImage(ctx context.Context) (v1.Image, error) {
	imgRef, err := imgname.ParseReference(o.imageReference)
	if err != nil {
		return nil, fmt.Errorf("%s is an unparseable image reference: %w", o.imageReference, err)
	}
	return ociremote.Image(imgRef, ociremote.WithAuthFromKeychain(o.keychain), ociremote.WithContext(ctx))
}

// checkImageCompliance will perform common checks to ensure the Tekton Bundle is compliant to our spec.
func (o *Resolver) checkImageCompliance(manifest *v1.Manifest) error {
	// Check the manifest's layers to ensure there are a maximum of 10.
	if len(manifest.Layers) > MaximumBundleObjects {
		return fmt.Errorf("bundle %s contained more than the maximum %d allow objects", o.imageReference, MaximumBundleObjects)
	}

	// Ensure each layer complies to the spec.
	for _, l := range manifest.Layers {
		refDigest := fmt.Sprintf("%s:%s", o.imageReference, l.Digest.String())
		if _, ok := l.Annotations[APIVersionAnnotation]; !ok {
			return fmt.Errorf("invalid tekton bundle: %s does not contain a %s annotation", refDigest, APIVersionAnnotation)
		}

		if _, ok := l.Annotations[TitleAnnotation]; !ok {
			return fmt.Errorf("invalid tekton bundle: %s does not contain a %s annotation", refDigest, TitleAnnotation)
		}

		kind, ok := l.Annotations[KindAnnotation]
		if !ok {
			return fmt.Errorf("invalid tekton bundle: %s does not contain a %s annotation", refDigest, KindAnnotation)
		}
		if strings.TrimSuffix(strings.ToLower(kind), "s") != kind {
			return fmt.Errorf("invalid tekton bundle: %s annotation for %s must be lowercased and singular, found %s", KindAnnotation, refDigest, kind)
		}
	}

	return nil
}

// Utility function to read out the contents of an image layer, assumed to be a tarball, as a parsed Tekton resource.
func readTarLayer(layer v1.Layer) (runtime.Object, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("Failed to read image layer: %w", err)
	}
	defer rc.Close()

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

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(contents, nil, nil)
	return obj, err
}

// Utility function to read out the contents of an image layer, assumed to be raw bytes, as a parsed Tekton resource.
func readRawLayer(layer v1.Layer) (runtime.Object, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to read image layer: %w", err)
	}
	defer rc.Close()

	contents, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("could not read contents of image layer: %w", err)
	}

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(contents, nil, nil)
	return obj, err
}
