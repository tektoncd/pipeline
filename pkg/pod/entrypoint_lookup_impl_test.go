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

package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func TestImageData(t *testing.T) {
	// Generate an Image with an Entrypoint configured.
	img, err := random.Image(1, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	img, err = mutate.Config(img, v1.Config{
		Entrypoint: []string{"my", "entrypoint"},
	})
	if err != nil {
		t.Fatalf("mutate.Config: %v", err)
	}
	// Get the generated image's digest.
	dig, err := img.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	ref, err := name.ParseReference("ubuntu@"+dig.String(), name.WeakValidation)
	if err != nil {
		t.Fatalf("ParseReference(%q): %v", dig, err)
	}

	// Get the image data (entrypoint and digest) for the image.
	gotEP, gotDig, err := imageData(ref, img)
	if err != nil {
		t.Fatalf("imageData: %v", err)
	}
	if d := cmp.Diff([]string{"my", "entrypoint"}, gotEP); d != "" {
		t.Errorf("Diff(-want, +got): %s", d)
	}
	if d := cmp.Diff("index.docker.io/library/ubuntu@"+dig.String(), gotDig.String()); d != "" {
		t.Errorf("Diff(-want, +got): %s", d)
	}
}
