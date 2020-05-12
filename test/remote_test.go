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
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
)

func TestCreateImage(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, _ := url.Parse(s.URL)

	task := tb.Task("test-create-image", tb.TaskType())

	ref, err := CreateImage(u.Host+"/task/test-create-image", task)
	if err != nil {
		t.Errorf("uploading image failed unexpectedly with an error: %w", err)
	}

	// Pull the image and ensure the layers are composed correctly.
	imgRef, err := name.ParseReference(ref)
	if err != nil {
		t.Errorf("digest %s is not a valid reference: %w", ref, err)
	}

	img, err := remote.Image(imgRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		t.Errorf("could not fetch created image: %w", err)
	}

	m, err := img.Manifest()
	if err != nil {
		t.Errorf("failed to fetch img manifest: %w", err)
	}

	layers, err := img.Layers()
	if err != nil || len(layers) != 1 {
		t.Errorf("img layers were incorrect. Num Layers: %d. Err: %w", len(layers), err)
	}

	if diff := cmp.Diff(m.Layers[0].Annotations["org.opencontainers.image.title"], "test-create-image"); diff != "" {
		t.Error(diff)
	}
	if diff := cmp.Diff(m.Layers[0].Annotations["cdf.tekton.image.kind"], "task"); diff != "" {
		t.Error(diff)
	}
	if diff := cmp.Diff(m.Layers[0].Annotations["cdf.tekton.image.apiVersion"], "v1beta1"); diff != "" {
		t.Error(diff)
	}

	// Read the layer's contents and ensure it matches the task we uploaded.
	rc, err := layers[0].Uncompressed()
	if err != nil {
		t.Errorf("layer contents were corrupted: %w", err)
	}
	defer rc.Close()
	actual, err := ioutil.ReadAll(rc)
	if err != nil {
		t.Errorf("layer contents weren't readable: %w", err)
	}

	raw, err := yaml.Marshal(task)
	if err != nil {
		t.Fatalf("Could not marshal task to bytes: %#v", err)
	}
	if diff := cmp.Diff(string(raw), string(actual)); diff != "" {
		t.Error(diff)
	}
}
