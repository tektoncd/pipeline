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
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	corev1 "k8s.io/api/core/v1"
)

// EntrypointCache looks up an image's entrypoint (command) in a container
// image registry, possibly using the given service account's credentials.
type EntrypointCache interface {
	// get the Image data for the given image reference. If the value is
	// not found in the cache, it will be fetched from the image registry,
	// possibly using K8s service account imagePullSecrets.
	//
	// It also returns the digest associated with the given reference. If
	// the reference referred to an index, the returned digest will be the
	// index's digest, not any platform-specific image contained by the
	// index.
	get(ctx context.Context, ref name.Reference, namespace, serviceAccountName string, imagePullSecrets []corev1.LocalObjectReference, hasArgs bool) (*imageData, error)
}

// imageData contains information looked up about an image or multi-platform image index.
type imageData struct {
	digest   v1.Hash
	commands map[string][]string // map of platform -> []command
}

// resolveEntrypoints looks up container image ENTRYPOINTs for all steps that
// don't specify a Command.
//
// Images that are not specified by digest will be specified by digest after
// lookup in the resulting list of containers.
func resolveEntrypoints(ctx context.Context, cache EntrypointCache, namespace, serviceAccountName string, imagePullSecrets []corev1.LocalObjectReference, steps []corev1.Container) ([]corev1.Container, error) {
	// Keep a local cache of name->imageData lookups, just for the scope of
	// resolving this set of steps. If the image is pushed to before the
	// next run, we need to resolve its digest and commands again, but we
	// can skip lookups while resolving the same TaskRun.
	localCache := map[name.Reference]imageData{}
	for i, s := range steps {
		// If the command is already specified, there's nothing to resolve.
		if len(s.Command) > 0 {
			continue
		}
		hasArgs := len(s.Args) > 0

		ref, err := name.ParseReference(s.Image, name.WeakValidation)
		if err != nil {
			return nil, err
		}
		var id imageData
		if cid, found := localCache[ref]; found {
			id = cid
		} else {
			// Look it up for real.
			lid, err := cache.get(ctx, ref, namespace, serviceAccountName, imagePullSecrets, hasArgs)
			if err != nil {
				return nil, err
			}
			id = *lid

			// Cache it locally in case another step in this task specifies the same image.
			localCache[ref] = *lid
		}

		// Resolve the original reference to a reference by digest.
		steps[i].Image = ref.Context().Digest(id.digest.String()).String()

		// Encode the map of platform->command to JSON and pass it via env var.
		b, err := json.Marshal(id.commands)
		if err != nil {
			return nil, err
		}
		steps[i].Env = append(steps[i].Env, corev1.EnvVar{
			Name:  "TEKTON_PLATFORM_COMMANDS",
			Value: string(b),
		})
	}
	return steps, nil
}
