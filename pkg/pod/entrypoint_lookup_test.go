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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	corev1 "k8s.io/api/core/v1"
)

func TestResolveEntrypoints(t *testing.T) {
	// Generate a random image with entrypoint configured.
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
	dig, err := img.Digest()
	if err != nil {
		t.Fatalf("image.Digest: %v", err)
	}
	t.Logf("Random image digest is %s", dig.String())

	// Populate an EntrypointCache backed by a map.
	cache := fakeCache{
		"gcr.io/my/image@" + dig.String(): &data{img: img},
		"gcr.io/my/image:latest":          &data{img: img},
	}

	got, err := resolveEntrypoints(cache, "namespace", "serviceAccountName", []corev1.Container{{
		// This step specifies its command, so there's nothing to
		// resolve.
		Image:   "fully-specified",
		Command: []string{"specified", "command"},
	}, {
		// This step doesn't specify a command, so we'll look it up in
		// the registry and, while we're at it, resolve the image
		// digest.
		Image: "gcr.io/my/image",
	}, {
		// This step doesn't specify a command, so we'll look it up in
		// the registry (by explicit digest).
		Image: "gcr.io/my/image@" + dig.String(),
	}, {
		// This step doesn't specify a command, but we already looked
		// it up, so it's already in the local cache -- we don't need
		// to look it up in the remote registry again.
		Image: "gcr.io/my/image",
	}})
	if err != nil {
		t.Fatalf("resolveEntrypoints: %v", err)
	}

	want := []corev1.Container{{
		// The step that explicitly specified its command wasn't
		// resolved at all.
		Image:   "fully-specified",
		Command: []string{"specified", "command"},
	}, {
		// The step that didn't specify a command had its digest and
		// entrypoint looked up in the registry, and cached.
		Image:   "gcr.io/my/image@" + dig.String(),
		Command: []string{"my", "entrypoint"},
	}, {
		// The step that didn't specify command was looked up in the
		// registry (by digest), and cached.
		Image:   "gcr.io/my/image@" + dig.String(),
		Command: []string{"my", "entrypoint"},
	}, {
		// The other step that didn't specify command or digest was
		// resolved from the *local* cache, without hitting the remote
		// registry again.
		Image:   "gcr.io/my/image@" + dig.String(),
		Command: []string{"my", "entrypoint"},
	}}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Diff (-want, +got): %s", d)
	}
}

type fakeCache map[string]*data
type data struct {
	img  v1.Image
	seen bool // Whether the image has been looked up before.
}

func (f fakeCache) Get(ref name.Reference, _, _ string) (v1.Image, error) {
	if d, ok := ref.(name.Digest); ok {
		if data, found := f[d.String()]; found {
			return data.img, nil
		}
	}

	d, found := f[ref.Name()]
	if !found {
		return nil, fmt.Errorf("image %q not found", ref)
	}
	if d.seen {
		return nil, fmt.Errorf("image %q was already looked up", ref)
	}
	return d.img, nil
}

func (f fakeCache) Set(d name.Digest, img v1.Image) {
	f[d.String()] = &data{img: img}
}
