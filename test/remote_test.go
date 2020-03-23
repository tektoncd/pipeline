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
	remote "github.com/google/go-containerregistry/pkg/v1/remote"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestCreateImage(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, _ := url.Parse(s.URL)

	task := tb.Task("test-create-image", "no-ns")
	raw, err := yaml.Marshal(task)
	if err != nil {
		t.Errorf("failed to marshal task to bytes: %w", err)
	}

	ref, err := CreateImage(u.Host, "task/test-create-image", string(raw))
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

	if diff := cmp.Diff(m.Layers[0].Annotations["org.opencontainers.image.title"], "task/test-create-image"); diff != "" {
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

	if diff := cmp.Diff(string(raw), string(actual)); diff != "" {
		t.Error(diff)
	}
}
