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
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
)

// EntrypointCache looks up an image's entrypoint (command) in a container
// image registry, possibly using the given service account's credentials.
type EntrypointCache interface {
	Get(imageName, namespace, serviceAccountName string) (cmd []string, d name.Digest, err error)
}

// resolveEntrypoints looks up container image ENTRYPOINTs for all steps that
// don't specify a Command.
//
// Images that are not specified by digest will be specified by digest after
// lookup in the resulting list of containers.
func resolveEntrypoints(cache EntrypointCache, namespace, serviceAccountName string, steps []corev1.Container) ([]corev1.Container, error) {
	// Keep a local cache of image->digest lookups, just for the scope of
	// resolving this set of steps. If the image is pushed to, we need to
	// resolve its digest and entrypoint again, but we can skip lookups
	// while resolving the same TaskRun.
	type result struct {
		digest name.Digest
		ep     []string
	}
	localCache := map[string]result{}
	for i, s := range steps {
		if len(s.Command) != 0 {
			// Nothing to resolve.
			continue
		}

		var digest name.Digest
		var ep []string
		var err error
		if r, found := localCache[s.Image]; found {
			digest = r.digest
			ep = r.ep
		} else {
			// Look it up in the cache, which will also resolve the digest.
			ep, digest, err = cache.Get(s.Image, namespace, serviceAccountName)
			if err != nil {
				return nil, err
			}
			localCache[s.Image] = result{digest, ep} // Cache it locally in case another step specifies the same image.
		}

		steps[i].Image = digest.String() // Specify image by digest, since we know it now.
		steps[i].Command = ep            // Specify the command explicitly.
	}
	return steps, nil
}
