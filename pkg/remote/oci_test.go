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
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	imgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pushImage(imgRef name.Reference, task *v1beta1.Task) (imgv1.Image, error) {
	taskRaw, err := yaml.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("invalid sample task def %s", err.Error())
	}

	img := mutate.MediaType(empty.Image, types.MediaType("application/vnd.cdf.tekton.catalog.v1beta1+yaml"))
	layer, err := tarball.LayerFromReader(strings.NewReader(string(taskRaw)))
	if err != nil {
		return nil, fmt.Errorf("unexpected error adding task layer to image %s", err.Error())
	}

	img, err = mutate.Append(img, mutate.Addendum{
		Layer: layer,
		Annotations: map[string]string{
			"org.opencontainers.image.title": fmt.Sprintf("task/%s", task.GetName()),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not add layer to image %s", err.Error())
	}

	if err := remoteimg.Write(imgRef, img); err != nil {
		return nil, fmt.Errorf("could not push example image to registry")
	}

	return img, nil
}

func TestOCIResolver(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	imgRef, err := name.ParseReference(fmt.Sprintf("%s/test/ociresolver", u.Host))
	if err != nil {
		t.Errorf("undexpected error producing image reference %s", err.Error())
	}

	// Create the image using an example task.
	task := v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "hello-world",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{
				{
					Container: v1.Container{
						Image: "ubuntu",
					},
					Script: "echo \"Hello World!\"",
				},
			},
		},
	}
	img, err := pushImage(imgRef, &task)
	if err != nil {
		t.Error(err)
	}

	// Now we can call our resolver and see if the spec returned is the same.
	digest, err := img.Digest()
	if err != nil {
		t.Errorf("unexpected error getting digest of image: %s", err.Error())
	}
	resolver := OCIResolver{
		imageReference:   imgRef.Context().Digest(digest.String()).String(),
		keychainProvider: func() (authn.Keychain, error) { return authn.DefaultKeychain, nil },
	}

	actual, err := resolver.GetTask("hello-world")
	if err != nil {
		t.Errorf("failed to fetch task hello-world: %s", err.Error())
	}

	if diff := cmp.Diff(actual, &task.Spec); diff != "" {
		t.Error(diff)
	}
}
