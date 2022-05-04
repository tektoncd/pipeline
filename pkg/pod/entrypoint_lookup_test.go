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
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
)

func TestResolveEntrypoints(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Generate a random image with entrypoint configured.
	img, err := random.Image(1, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	dig, err := img.Digest()
	if err != nil {
		t.Fatalf("image.Digest: %v", err)
	}
	t.Logf("Random image digest is %s", dig.String())

	id := &imageData{
		digest: dig,
		commands: map[string][]string{
			"only-plat": {"my", "entrypoint"},
		},
	}

	multi := &imageData{
		digest: dig,
		commands: map[string][]string{
			"plat-1": {"plat", "one"},
			"plat-2": {"plat", "two"},
		},
	}

	// Populate an EntrypointCache backed by a map.
	cache := fakeCache{
		"gcr.io/my/image@" + dig.String(): &data{id: id},
		"gcr.io/my/image:latest":          &data{id: id},
		"reg.io/multi/arch:latest":        &data{id: multi},
	}

	got, err := resolveEntrypoints(ctx, cache, "namespace", "serviceAccountName", []corev1.LocalObjectReference{{Name: "imageSecret"}}, []corev1.Container{{
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
	}, {
		// This is a multi-arch image, so we'll pass each platform's
		// commands to the Pod in an env var, to be interpreted by the
		// entrypoint binary.
		Image: "reg.io/multi/arch",
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
		Image: "gcr.io/my/image@" + dig.String(),
		Env: []corev1.EnvVar{{
			Name:  "TEKTON_PLATFORM_COMMANDS",
			Value: `{"only-plat":["my","entrypoint"]}`,
		}},
	}, {
		// The step that didn't specify command was looked up in the
		// registry (by digest), and cached.
		Image: "gcr.io/my/image@" + dig.String(),
		Env: []corev1.EnvVar{{
			Name:  "TEKTON_PLATFORM_COMMANDS",
			Value: `{"only-plat":["my","entrypoint"]}`,
		}},
	}, {
		// The other step that didn't specify command or digest was
		// resolved from the *local* cache, without hitting the remote
		// registry again.
		Image: "gcr.io/my/image@" + dig.String(),
		Env: []corev1.EnvVar{{
			Name:  "TEKTON_PLATFORM_COMMANDS",
			Value: `{"only-plat":["my","entrypoint"]}`,
		}},
	}, {
		Image: "reg.io/multi/arch@" + dig.String(),
		Env: []corev1.EnvVar{{
			Name:  "TEKTON_PLATFORM_COMMANDS",
			Value: `{"plat-1":["plat","one"],"plat-2":["plat","two"]}`,
		}},
	}}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Diff %s", diff.PrintWantGot(d))
	}
}

type fakeCache map[string]*data
type data struct {
	id   *imageData
	seen bool // Whether the image has been looked up before.
}

func (f fakeCache) get(ctx context.Context, ref name.Reference, _, _ string, _ []corev1.LocalObjectReference, _ bool) (*imageData, error) {
	if d, ok := ref.(name.Digest); ok {
		if data, found := f[d.String()]; found {
			return data.id, nil
		}
	}

	d, found := f[ref.Name()]
	if !found {
		return nil, fmt.Errorf("image %q not found", ref)
	}
	if d.seen {
		return nil, fmt.Errorf("image %q was already looked up", ref)
	}
	return d.id, nil
}
