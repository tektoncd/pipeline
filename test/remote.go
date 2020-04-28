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

package test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"k8s.io/apimachinery/pkg/runtime"
)

// CreateImage will push a new OCI image artifact with the provided raw data object as a layer and return the full image
// reference with a digest to fetch the image. Key must be specified as [lowercase kind]/[object name]. The image ref
// with a digest is returned.
func CreateImage(ref string, objs ...runtime.Object) (string, error) {
	imgRef, err := name.ParseReference(ref)
	if err != nil {
		return "", fmt.Errorf("undexpected error producing image reference %w", err)
	}

	img := empty.Image

	for _, obj := range objs {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return "", fmt.Errorf("error serializing object: %w", err)
		}

		layer, err := tarball.LayerFromReader(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("unexpected error adding layer to image %w", err)
		}

		img, err = mutate.Append(img, mutate.Addendum{
			Layer: layer,
			Annotations: map[string]string{
				"org.opencontainers.image.title": getObjectName(obj),
				"cdf.tekton.image.kind":          strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind),
				"cdf.tekton.image.apiVersion":    strings.ToLower(obj.GetObjectKind().GroupVersionKind().Version),
			},
		})
		if err != nil {
			return "", fmt.Errorf("could not add layer to image %w", err)
		}
	}

	if err := remoteimg.Write(imgRef, img); err != nil {
		return "", fmt.Errorf("could not push example image to registry")
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("could not read image digest: %w", err)
	}

	return imgRef.Context().Digest(digest.String()).String(), nil
}

// Return the ObjectMetadata.Name field which every resource should have.
func getObjectName(obj runtime.Object) string {
	return reflect.Indirect(reflect.ValueOf(obj)).FieldByName("ObjectMeta").FieldByName("Name").String()
}
