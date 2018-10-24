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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/knative/build/pkg/apis/build/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

const (
	// MountName is the name of the pvc being mounted (which
	// will contain the entrypoint binary and eventually the logs)
	MountName         = "tools"
	MountPoint        = "/tools"
	BinaryLocation    = MountPoint + "/entrypoint"
	JSONConfigEnvVar  = "ENTRYPOINT_OPTIONS"
	Image             = "gcr.io/k8s-prow/entrypoint@sha256:7c7cd8906ce4982ffee326218e9fc75da2d4896d53cabc9833b9cc8d2d6b2b8f"
	InitContainerName = "place-tools"
	ProcessLogFile    = "/tools/process-log.txt"
	MarkerFile        = "/tools/marker-file.txt"
)

var toolsMount = corev1.VolumeMount{
	Name:      MountName,
	MountPath: MountPoint,
}

// Cache is a simple caching mechanism allowing for caching the results of
// getting the Entrypoint of a container image from a remote registry. It
// is synchronized via a mutex so that we can share a single Cache across
// each worker thread that the reconciler is running. The mutex is necessary
// due to the possibility of a panic if two workers were to attempt to read and
// write to the internal map at the same time.
type Cache struct {
	mtx   sync.RWMutex
	cache map[string][]string
}

// NewCache is a simple helper function that returns a pointer to a Cache that
// has had the internal cache map initialized.
func NewCache() *Cache {
	return &Cache{
		cache: make(map[string][]string),
	}
}

func (c *Cache) get(sha string) ([]string, bool) {
	c.mtx.RLock()
	ep, ok := c.cache[sha]
	c.mtx.RUnlock()
	return ep, ok
}

func (c *Cache) set(sha string, ep []string) {
	c.mtx.Lock()
	c.cache[sha] = ep
	c.mtx.Unlock()
}

// AddCopyStep will prepend a BuildStep (Container) that will
// copy the entrypoint binary from the entrypoint image into the
// volume mounted at MountPoint, so that it can be mounted by
// subsequent steps and used to capture logs.
func AddCopyStep(b *v1alpha1.BuildSpec) {
	cp := corev1.Container{
		Name:         InitContainerName,
		Image:        Image,
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

// GetRemoteEntrypoint accepts a cache of image lookups, as well as the image
// to look for. If the cache does not contain the image, it will lookup the
// metadata from the images registry, and then commit that to the cache
func GetRemoteEntrypoint(cache *Cache, image string) ([]string, error) {
	if ep, ok := cache.get(image); ok {
		return ep, nil
	}
	// verify the image name, then download the remote config file
	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse image %s: %v", image, err)
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("couldn't get container image info from registry %s: %v", image, err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("couldn't get config for image %s: %v", image, err)
	}
	cache.set(image, cfg.ContainerConfig.Entrypoint)
	return cfg.ContainerConfig.Entrypoint, nil
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
