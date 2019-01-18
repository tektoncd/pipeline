/*
Copyright 2018 The Knative Authors

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

package entrypoint

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	lru "github.com/hashicorp/golang-lru"
	corev1 "k8s.io/api/core/v1"

	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/config"
	"github.com/knative/build/pkg/apis/build/v1alpha1"
)

const (
	// MountName is the name of the pvc being mounted (which
	// will contain the entrypoint binary and eventually the logs)
	MountName         = "tools"
	MountPoint        = "/tools"
	BinaryLocation    = MountPoint + "/entrypoint"
	JSONConfigEnvVar  = "ENTRYPOINT_OPTIONS"
	InitContainerName = "place-tools"
	ProcessLogFile    = "/tools/process-log.txt"
	MarkerFile        = "/tools/marker-file.txt"
	digestSeparator   = "@"
	cacheSize         = 1024
)

var toolsMount = corev1.VolumeMount{
	Name:      MountName,
	MountPath: MountPoint,
}

// Cache is a simple caching mechanism allowing for caching the results of
// getting the Entrypoint of a container image from a remote registry. The
// internal lru cache is thread-safe.
type Cache struct {
	lru *lru.Cache
}

// NewCache is a simple helper function that returns a pointer to a Cache that
// has had the internal fixed-sized lru cache initialized.
func NewCache() (*Cache, error) {
	lru, err := lru.New(cacheSize)
	return &Cache{lru}, err
}

func (c *Cache) get(sha string) ([]string, bool) {
	if ep, ok := c.lru.Get(sha); ok {
		return ep.([]string), true
	}
	return nil, false
}

func (c *Cache) set(sha string, ep []string) {
	c.lru.Add(sha, ep)
}

// AddCopyStep will prepend a BuildStep (Container) that will
// copy the entrypoint binary from the entrypoint image into the
// volume mounted at MountPoint, so that it can be mounted by
// subsequent steps and used to capture logs.
func AddCopyStep(ctx context.Context, b *v1alpha1.BuildSpec) {
	cfg := config.FromContext(ctx).Entrypoint

	cp := corev1.Container{
		Name:         InitContainerName,
		Image:        cfg.Image,
		Command:      []string{"/bin/cp"},
		Args:         []string{"/entrypoint", BinaryLocation},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}
	b.Steps = append([]corev1.Container{cp}, b.Steps...)

}

type entrypointArgs struct {
	Args       []string `json:"args"`
	ProcessLog string   `json:"process_log"`
	MarkerFile string   `json:"marker_file"`
}

func getEnvVar(cmd, args []string) (string, error) {
	entrypointArgs := entrypointArgs{
		Args:       append(cmd, args...),
		ProcessLog: ProcessLogFile,
		MarkerFile: MarkerFile,
	}
	j, err := json.Marshal(entrypointArgs)
	if err != nil {
		return "", fmt.Errorf("couldn't marshal arguments %q for entrypoint env var: %s", entrypointArgs, err)
	}
	return string(j), nil
}

// GetRemoteEntrypoint accepts a cache of digest lookups, as well as the digest
// to look for. If the cache does not contain the digest, it will lookup the
// metadata from the images registry, and then commit that to the cache
func GetRemoteEntrypoint(cache *Cache, digest string) ([]string, error) {
	if ep, ok := cache.get(digest); ok {
		return ep, nil
	}
	img, err := getRemoteImage(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote image %s: %v", digest, err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("couldn't get config for image %s: %v", digest, err)
	}
	cache.set(digest, cfg.ContainerConfig.Entrypoint)
	return cfg.ContainerConfig.Entrypoint, nil
}

// GetImageDigest tries to find and return image digest in cache, if
// cache doesn't exists it will lookup the digest in remote image manifest
// and then cache it
func GetImageDigest(cache *Cache, image string) (string, error) {
	if digestList, ok := cache.get(image); ok && (len(digestList) > 0) {
		return digestList[0], nil
	}
	img, err := getRemoteImage(image)
	if err != nil {
		return "", fmt.Errorf("failed to fetch remote image %s: %v", image, err)
	}
	digestHash, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("couldn't get digest hash for image %s: %v", image, err)
	}
	// Parse Digest Hash struct into sha string
	digest := fmt.Sprintf("%s%s%s", image, digestSeparator, digestHash.String())
	cache.set(image, []string{digest})

	return digest, nil
}

// RedirectSteps will modify each of the steps/containers such that
// the binary being run is no longer the one specified by the Command
// and the Args, but is instead the entrypoint binary, which will
// itself invoke the Command and Args, but also capture logs.
func RedirectSteps(steps []corev1.Container) error {
	for i := range steps {
		step := &steps[i]
		e, err := getEnvVar(step.Command, step.Args)
		if err != nil {
			return fmt.Errorf("couldn't get env var for entrypoint: %s", err)
		}
		step.Command = []string{BinaryLocation}
		step.Args = []string{}

		step.Env = append(step.Env, corev1.EnvVar{
			Name:  JSONConfigEnvVar,
			Value: e,
		})
		step.VolumeMounts = append(step.VolumeMounts, toolsMount)
	}
	return nil
}

func getRemoteImage(image string) (v1.Image, error) {
	// verify the image name, then download the remote config file
	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse image %s: %v", image, err)
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("couldn't get container image info from registry %s: %v", image, err)
	}

	return img, nil
}
