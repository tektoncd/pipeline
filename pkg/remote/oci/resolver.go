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
	"fmt"
	"io/ioutil"

	"github.com/google/go-containerregistry/pkg/authn"
	imgname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	ociremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"github.com/tektoncd/pipeline/pkg/remote"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ remote.Resolver = (*Resolver)(nil)

// Resolver implements the Resolver interface using OCI images.
type Resolver struct {
	imageReference string
	keychain       authn.Keychain
}

func (o *Resolver) List() ([]remote.ResolvedObject, error) {
	img, err := o.retrieveImage()
	if err != nil {
		return nil, err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("Could not parse image manifest: %w", err)
	}
	contents := make([]remote.ResolvedObject, 0, len(manifest.Layers))
	for _, l := range manifest.Layers {
		contents = append(contents, remote.ResolvedObject{
			Kind:       l.Annotations["cdf.tekton.image.kind"],
			APIVersion: l.Annotations["cdf.tekton.image.apiVersion"],
			Name:       l.Annotations["org.opencontainers.image.title"],
		})
	}

	return contents, nil
}

func (o *Resolver) Get(kind, name string) (runtime.Object, error) {
	img, err := o.retrieveImage()
	if err != nil {
		return nil, err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("Could not parse image manifest: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("Could not read image layers: %w", err)
	}

	for idx, l := range manifest.Layers {
		lKind := l.Annotations["cdf.tekton.image.kind"]
		lName := l.Annotations["org.opencontainers.image.title"]

		if kind == lKind && name == lName {
			return readLayer(layers[idx])
		}
	}
	return nil, fmt.Errorf("Could not find object in image with kind: %s and name: %s", kind, name)
}

// retrieveImage will fetch the image's contents and manifest.
func (o *Resolver) retrieveImage() (v1.Image, error) {
	imgRef, err := imgname.ParseReference(o.imageReference)
	if err != nil {
		return nil, fmt.Errorf("%s is an unparseable image reference: %w", o.imageReference, err)
	}
	return ociremote.Image(imgRef, ociremote.WithAuthFromKeychain(o.keychain))
}

// Utility function to read out the contents of an image layer as a parsed Tekton resource.
func readLayer(layer v1.Layer) (runtime.Object, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("Failed to read image layer: %w", err)
	}
	defer rc.Close()

	contents, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("Could not read contents of image layer: %w", err)
	}

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(contents, nil, nil)
	return obj, err
}
