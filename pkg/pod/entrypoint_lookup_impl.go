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
	"errors"
	"fmt"

	"github.com/containerd/containerd/platforms"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	lru "github.com/hashicorp/golang-lru"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const cacheSize = 1024

type entrypointCache struct {
	kubeclient kubernetes.Interface
	lru        *lru.Cache // cache of digest->map[string][]string commands
}

// NewEntrypointCache returns a new entrypoint cache implementation that uses
// K8s credentials to pull image metadata from a container image registry.
func NewEntrypointCache(kubeclient kubernetes.Interface) (EntrypointCache, error) {
	lru, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}
	return &entrypointCache{
		kubeclient: kubeclient,
		lru:        lru,
	}, nil
}

// Get gets the image from the cache for the given ref, namespace, and SA.
//
// It also returns the digest associated with the given reference. If the
// reference referred to an index, the returned digest will be the index's
// digest, not any platform-specific image contained by the index.
func (e *entrypointCache) get(ctx context.Context, ref name.Reference, namespace, serviceAccountName string, imagePullSecrets []corev1.LocalObjectReference) (*imageData, error) {
	// If image is specified by digest, check the local cache.
	if digest, ok := ref.(name.Digest); ok {
		if id, ok := e.lru.Get(digest.String()); ok {
			return id.(*imageData), nil
		}
	}

	pullSecretsNames := make([]string, 0, len(imagePullSecrets))
	for _, ps := range imagePullSecrets {
		pullSecretsNames = append(pullSecretsNames, ps.Name)
	}
	// Consult the remote registry, using imagePullSecrets.
	kc, err := k8schain.New(ctx, e.kubeclient, k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccountName,
		ImagePullSecrets:   pullSecretsNames,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating k8schain: %v", err)
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(kc))
	if err != nil {
		return nil, err
	}

	// Check the cache for this ref@digest, in case we've seen it before.
	// This saves looking up each constinuent image's commands if we've seen
	// the multi-platform image before.
	refByDigest := ref.Context().Digest(desc.Digest.String()).String()
	if id, ok := e.lru.Get(refByDigest); ok {
		return id.(*imageData), nil
	}

	id := &imageData{
		digest:   desc.Digest,
		commands: map[string][]string{},
	}
	switch {
	case desc.MediaType.IsImage():
		img, err := desc.Image()
		if err != nil {
			return nil, err
		}
		ep, plat, err := imageInfo(img)
		if err != nil {
			return nil, err
		}
		id.commands[plat] = ep
	case desc.MediaType.IsIndex():
		idx, err := desc.ImageIndex()
		if err != nil {
			return nil, err
		}
		id.commands, err = buildCommandMap(idx)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported media type for image reference")
	}

	// Cache the digest->commands for future lookup.
	e.lru.Add(refByDigest, id)

	return id, nil
}

func buildCommandMap(idx v1.ImageIndex) (map[string][]string, error) {
	// Map platform strings to digest, to handle some ~malformed images
	// that specify the same manifest multiple times.
	platToDigest := map[string]v1.Hash{}

	cmds := map[string][]string{}

	mf, err := idx.IndexManifest()
	if err != nil {
		return nil, err
	}
	for _, desc := range mf.Manifests {
		plat := desc.Platform.String()
		if got, found := platToDigest[plat]; found && got != desc.Digest {
			return nil, fmt.Errorf("duplicate unique image found for platform: %s: found %s and %s", plat, got, desc.Digest)
		}
		platToDigest[plat] = desc.Digest
		img, err := idx.Image(desc.Digest)
		if err != nil {
			return nil, err
		}
		cmds[plat], _, err = imageInfo(img)
		if err != nil {
			return nil, err
		}
	}
	return cmds, nil
}

func imageInfo(img v1.Image) (cmd []string, platform string, err error) {
	cf, err := img.ConfigFile()
	if err != nil {
		return nil, "", err
	}
	ep := cf.Config.Entrypoint
	if len(ep) == 0 {
		ep = cf.Config.Cmd
	}

	plat := platforms.Format(specs.Platform{
		OS:           cf.OS,
		Architecture: cf.Architecture,
		// A single image's config metadata doesn't include the CPU
		// architecture variant, but we'll assume this is okay since
		// the runtime node's image selection will also select the same
		// image. This will only be a problem if the image is a
		// single-platform image that happens to specify a variant, and
		// the runtime node it gets assigned to has a value for
		// runtime.GOARM.
	})
	return ep, plat, nil
}
