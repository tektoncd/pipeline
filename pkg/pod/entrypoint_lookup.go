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

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	corev1 "k8s.io/api/core/v1"
)

// EntrypointCache looks up an image's entrypoint (command) in a container
// image registry, possibly using the given service account's credentials.
type EntrypointCache interface {
	// Get the Image data for the given image reference. If the value is
	// not found in the cache, it will be fetched from the image registry,
	// possibly using K8s service account imagePullSecrets.
	Get(ref name.Reference, namespace, serviceAccountName string) (v1.Image, error)
	// Update the cache with a new digest->Image mapping. This will avoid a
	// remote registry lookup next time Get is called.
	Set(digest name.Digest, img v1.Image)
}

// resolveEntrypoints looks up container image ENTRYPOINTs for all steps that
// don't specify a Command.
//
// Images that are not specified by digest will be specified by digest after
// lookup in the resulting list of containers.
func resolveEntrypoints(cache EntrypointCache, namespace, serviceAccountName string, steps []corev1.Container) ([]corev1.Container, error) {
	// Keep a local cache of name->image lookups, just for the scope of
	// resolving this set of steps. If the image is pushed to before the
	// next run, we need to resolve its digest and entrypoint again, but we
	// can skip lookups while resolving the same TaskRun.
	localCache := map[name.Reference]v1.Image{}
	for i, s := range steps {
		if len(s.Command) != 0 {
			// Nothing to resolve.
			continue
		}

		origRef, err := name.ParseReference(s.Image, name.WeakValidation)
		if err != nil {
			return nil, err
		}
		var img v1.Image
		if cimg, found := localCache[origRef]; found {
			img = cimg
		} else {
			// Look it up in the cache. If it's not found in the
			// cache, it will be resolved from the registry.
			img, err = cache.Get(origRef, namespace, serviceAccountName)
			if err != nil {
				return nil, err
			}
			// Cache it locally in case another step specifies the same image.
			localCache[origRef] = img
		}

		ep, digest, err := imageData(origRef, img)
		if err != nil {
			return nil, err
		}

		cache.Set(digest, img) // Cache the lookup for next time this image is looked up by digest.

		steps[i].Image = digest.String() // Specify image by digest, since we know it now.
		steps[i].Command = ep            // Specify the command explicitly.
	}
	return steps, nil
}

// imageData pulls the entrypoint from the image, and returns the given
// original reference, with image digest resolved.
func imageData(ref name.Reference, img v1.Image) ([]string, name.Digest, error) {
	digest, err := img.Digest()
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("error getting image digest: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("error getting image config: %v", err)
	}

	// Entrypoint can be specified in either .Config.Entrypoint or
	// .Config.Cmd.
	ep := cfg.Config.Entrypoint
	if len(ep) == 0 {
		ep = cfg.Config.Cmd
	}

	d, err := name.NewDigest(ref.Context().String()+"@"+digest.String(), name.WeakValidation)
	if err != nil {
		return nil, name.Digest{}, fmt.Errorf("error constructing resulting digest: %v", err)
	}
	return ep, d, nil
}
