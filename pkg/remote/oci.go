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
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	imgname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
)

// KeychainProvider is an input to the OCIResolver which returns a keychain for fetching remote images with
// authentication.
type KeychainProvider func() (authn.Keychain, error)

// OCIResolver will attempt to fetch Tekton resources from an OCI compliant image repository.
type OCIResolver struct {
	imageReference   string
	keychainProvider KeychainProvider
}

// GetTask will retrieve the specified task from the resolver's defined image and return its spec. If it cannot be
// retrieved for any reason, an error is returned.
func (o OCIResolver) GetTask(taskName string) (*v1beta1.TaskSpec, error) {
	taskContents, err := o.readImageLayer("task", taskName)
	if err != nil {
		return nil, err
	}

	// Deserialize the task into a valid task spec.
	var task v1beta1.Task
	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(taskContents, nil, &task)
	if err != nil {
		return nil, fmt.Errorf("Invalid remote task %s: %w", taskName, err)
	}

	return &task.Spec, nil
}

func (o OCIResolver) readImageLayer(kind string, name string) ([]byte, error) {
	imgRef, err := imgname.ParseReference(o.imageReference)
	if err != nil {
		return nil, fmt.Errorf("%s is an unparseable task image reference: %w", o.imageReference, err)
	}

	// Create a keychain for use in authenticating against the remote repository.
	keychain, err := o.keychainProvider()
	if err != nil {
		return nil, err
	}

	img, err := remote.Image(imgRef, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, fmt.Errorf("Error pulling image %q: %w", o.imageReference, err)
	}
	// Ensure the media type is exclusively the Tekton catalog media type.
	if mt, err := img.MediaType(); err != nil || string(mt) != "application/vnd.cdf.tekton.catalog.v1beta1+yaml" {
		return nil, fmt.Errorf("cannot parse reference from image type %s: %w", string(mt), err)
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
