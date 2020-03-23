/*
Copyright 2019 The Tekton Authors

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

package remote

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	imgname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
)

// OCIResolver will attempt to fetch Tekton resources from an OCI compliant image repository.
type OCIResolver struct {
	imageReference string
	keychain       authn.Keychain
}

func NewOCIResolver(image string, keychain authn.Keychain) OCIResolver {
	return OCIResolver{
		imageReference: image,
		keychain:       keychain,
	}
}

// GetTask will retrieve the specified task from the resolver's defined image and return its spec. If it cannot be
// retrieved for any reason, an error is returned.
func (o OCIResolver) GetTask(taskName string) (v1alpha1.TaskInterface, error) {
	taskContents, err := o.readImageLayer("task", taskName)
	if err != nil {
		return nil, err
	}

	// Deserialize the contents into a valid task spec.
	obj, kind, err := scheme.Codecs.UniversalDeserializer().Decode(taskContents, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("Invalid remote task %s: %w", taskName, err)
	}

	if t, ok := obj.(*v1alpha1.Task); ok {
		return t, nil
	}

	if t, ok := obj.(*v1beta1.Task); ok {
		var tt v1alpha1.Task
		err = tt.ConvertFrom(context.Background(), t)
		return &tt, err
	}

	if t, ok := obj.(*v1alpha1.ClusterTask); ok {
		return t, nil
	}

	if t, ok := obj.(*v1beta1.ClusterTask); ok {
		var ct v1alpha1.Task
		err = ct.ConvertFrom(context.Background(), t)
		return &ct, err
	}

	return nil, fmt.Errorf("unknown task kind %s", kind.String())
}

func (o OCIResolver) readImageLayer(kind string, name string) ([]byte, error) {
	imgRef, err := imgname.ParseReference(o.imageReference)
	if err != nil {
		return nil, fmt.Errorf("%s is an unparseable task image reference: %w", o.imageReference, err)
	}

	img, err := remote.Image(imgRef, remote.WithAuthFromKeychain(o.keychain))
	if err != nil {
		return nil, fmt.Errorf("Error pulling image %q: %w", o.imageReference, err)
	}

	m, err := img.Manifest()
	if err != nil {
		return nil, err
	}

	ls, err := img.Layers()
	if err != nil {
		return nil, err
	}
	var layer v1.Layer
	for idx, l := range m.Layers {
		if l.Annotations["org.opencontainers.image.title"] == o.getLayerName(kind, name) {
			layer = ls[idx]
		}
	}
	if layer == nil {
		return nil, fmt.Errorf("Resource %s/%s not found", kind, name)
	}
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}

func (o OCIResolver) getLayerName(kind string, name string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(kind), name)
}
