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

package entrypoint

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	lru "github.com/hashicorp/golang-lru"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// MountName is the name of the pvc being mounted (which
	// will contain the entrypoint binary and eventually the logs)
	MountName              = "tools"
	MountPoint             = "/builder/tools"
	DownwardMountName      = "downward"
	DownwardMountPoint     = "/builder/downward"
	DownwardMountReadyFile = "ready"
	BinaryLocation         = MountPoint + "/entrypoint"
	JSONConfigEnvVar       = "ENTRYPOINT_OPTIONS"
	InitContainerName      = "place-tools"
	cacheSize              = 1024
)

var toolsMount = corev1.VolumeMount{
	Name:      MountName,
	MountPath: MountPoint,
}
var downwardMount = corev1.VolumeMount{
	Name:      DownwardMountName,
	MountPath: DownwardMountPoint,
}
var (
	entrypointImage = flag.String("entrypoint-image", "override-with-entrypoint:latest",
		"The container image containing our entrypoint binary.")
)

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

// AddToEntrypointCache adds an image digest and its entrypoint
// to the cache
func AddToEntrypointCache(c *Cache, sha string, ep []string) {
	c.set(sha, ep)
}

// AddCopyStep will prepend a Step (Container) that will
// copy the entrypoint binary from the entrypoint image into the
// volume mounted at MountPoint, so that it can be mounted by
// subsequent steps and used to capture logs.
func AddCopyStep(spec *v1alpha1.TaskSpec) {
	cp := corev1.Container{
		Name:    InitContainerName,
		Image:   *entrypointImage,
		Command: []string{"/bin/sh"},
		// based on the ko version, the binary could be in one of two different locations
		Args:         []string{"-c", fmt.Sprintf("cp /ko-app/entrypoint %s", BinaryLocation)},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}
	spec.Steps = append([]corev1.Container{cp}, spec.Steps...)

}

// RedirectSteps will modify each of the steps/containers such that
// the binary being run is no longer the one specified by the Command
// and the Args, but is instead the entrypoint binary, which will
// itself invoke the Command and Args, but also capture logs.
func RedirectSteps(cache *Cache, steps []corev1.Container, kubeclient kubernetes.Interface, taskRun *v1alpha1.TaskRun, logger *zap.SugaredLogger) error {
	for i := range steps {
		step := &steps[i]
		if err := RedirectStep(cache, i, step, kubeclient, taskRun, logger); err != nil {
			return err
		}
	}

	return nil
}

// RedirectStep will modify a step/container such that
// the binary being run is no longer the one specified by the Command
// and the Args, but is instead the entrypoint binary, which will
// itself invoke the Command and Args, but also capture logs.
func RedirectStep(cache *Cache, stepNum int, step *corev1.Container, kubeclient kubernetes.Interface, taskRun *v1alpha1.TaskRun, logger *zap.SugaredLogger) error {
	if len(step.Command) == 0 {
		logger.Infof("Getting Cmd from remote entrypoint for step: %s", step.Name)
		var err error
		step.Command, err = GetRemoteEntrypoint(cache, step.Image, kubeclient, taskRun)
		if err != nil {
			logger.Errorf("Error getting entry point image", err.Error())
			return err
		}
	}

	step.Args = GetArgs(stepNum, step.Command, step.Args)
	step.Command = []string{BinaryLocation}
	step.VolumeMounts = append(step.VolumeMounts, toolsMount)
	// The first step in a Task waits for the existence of a file projected into the Pod
	// from the Downward API. That file will be populated only when all sidecars are ready,
	// thereby ensuring that steps don't start executing while helper sidecars are still pending.
	if stepNum == 0 {
		step.VolumeMounts = append(step.VolumeMounts, downwardMount)
	}
	return nil
}

// GetArgs returns the arguments that should be specified for the step which has been wrapped
// such that it will execute our custom entrypoint instead of the user provided Command and Args.
func GetArgs(stepNum int, commands, args []string) []string {
	waitFile := getWaitFile(stepNum)
	// The binary we want to run must be separated from its arguments by --
	// so if commands has more than one value, we'll move the other values
	// into the arg list so we can separate them
	if len(commands) > 1 {
		args = append(commands[1:], args...)
		commands = commands[:1]
	}
	argsForEntrypoint := []string{
		"-wait_file", waitFile,
		"-post_file", getWaitFile(stepNum + 1),
	}
	if stepNum == 0 {
		argsForEntrypoint = append(argsForEntrypoint, "-wait_file_content")
	}
	argsForEntrypoint = append(argsForEntrypoint, "-entrypoint")
	argsForEntrypoint = append(argsForEntrypoint, commands...)
	// TODO: what if Command has multiple elements, do we need "--" between command and args?
	argsForEntrypoint = append(argsForEntrypoint, "--")
	return append(argsForEntrypoint, args...)
}

func getWaitFile(stepNum int) string {
	if stepNum == 0 {
		return fmt.Sprintf("%s/%s", DownwardMountPoint, DownwardMountReadyFile)
	}

	return fmt.Sprintf("%s/%s", MountPoint, strconv.Itoa(stepNum-1))
}

// GetRemoteEntrypoint accepts a cache of digest lookups, as well as the digest
// to look for. If the cache does not contain the digest, it will lookup the
// metadata from the images registry, and then commit that to the cache
func GetRemoteEntrypoint(cache *Cache, digest string, kubeclient kubernetes.Interface, taskRun *v1alpha1.TaskRun) ([]string, error) {
	if ep, ok := cache.get(digest); ok {
		return ep, nil
	}
	img, err := getRemoteImage(digest, kubeclient, taskRun)
	if err != nil {
		return nil, xerrors.Errorf("Failed to fetch remote image %s: %w", digest, err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, xerrors.Errorf("Failed to get config for image %s: %w", digest, err)
	}
	command := cfg.Config.Entrypoint
	if len(command) == 0 {
		command = cfg.Config.Cmd
	}
	cache.set(digest, command)
	return command, nil
}

func getRemoteImage(image string, kubeclient kubernetes.Interface, taskRun *v1alpha1.TaskRun) (v1.Image, error) {
	// verify the image name, then download the remote config file
	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return nil, xerrors.Errorf("Failed to parse image %s: %w", image, err)
	}

	kc, err := k8schain.New(kubeclient, k8schain.Options{
		Namespace:          taskRun.Namespace,
		ServiceAccountName: taskRun.Spec.ServiceAccount,
	})
	if err != nil {
		return nil, xerrors.Errorf("Failed to create k8schain: %w", err)
	}

	// this will first try to authenticate using the k8schain,
	// then fall back to the google keychain,
	// then fall back to anonymous
	mkc := authn.NewMultiKeychain(kc)
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(mkc))
	if err != nil {
		return nil, xerrors.Errorf("Failed to get container image info from registry %s: %w", image, err)
	}

	return img, nil
}
