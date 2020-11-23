/*
Copyright 2020 The Tekton Authors

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
package environment

import (
	"errors"
	"os"
	"regexp"
)

const (
	// Busybox image with specific sha
	BusyboxImage = iota
	// Registry image
	RegistryImage
	// kubectl image
	KubectlImage
	// helm image
	HelmImage
	// kaniko executor image
	KanikoImage
)

var (
	defaultKoDockerRepoRE = regexp.MustCompile("gcr.io/christiewilson-catfactory")
	defaultNamespaceRE    = regexp.MustCompile("namespace: default")

	amd64Images = map[int]string{
		BusyboxImage:  "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649",
		RegistryImage: "registry",
		KubectlImage:  "lachlanevenson/k8s-kubectl",
		HelmImage:     "alpine/helm:3.1.2",
		KanikoImage:   "gcr.io/kaniko-project/executor:v1.3.0",
	}
	s390xImages = map[int]string{
		BusyboxImage:  "busybox@sha256:4f47c01fa91355af2865ac10fef5bf6ec9c7f42ad2321377c21e844427972977",
		RegistryImage: "ibmcom/registry:2.6.2.5",
		KubectlImage:  "ibmcom/kubectl:v1.13.9",
		HelmImage:     "ibmcom/alpine-helm-s390x:latest",
		KanikoImage:   "gcr.io/kaniko-project/executor:s390x-9ed158c1f63a059cde4fd5f8b95af51d452d9aa7",
	}
)

func (e *Execution) GetImage(image int) string {
	return getImage(e.Platform, image)
}

func getImage(platform string, image int) string {
	switch platform {
	case "linux/s390x":
		return s390xImages[image]
	default:
		return amd64Images[image]
	}
}

// SubstituteEnv substitutes docker repos and bucket paths from the system
// environment for input to allow tests on local clusters. It unsets the
// namespace for ServiceAccounts so that they work under test. It also
// replaces image names to arch specific ones, based on provided mapping.
func (e *Execution) SubstituteEnv(input []byte, namespace string) ([]byte, error) {
	// Replace the placeholder image repo with the value of the
	// KO_DOCKER_REPO env var.
	val, ok := os.LookupEnv("KO_DOCKER_REPO")
	if !ok {
		return nil, errors.New("KO_DOCKER_REPO is not set")
	}
	output := defaultKoDockerRepoRE.ReplaceAll(input, []byte(val))

	// Strip any "namespace: default"s, all resources will be created in
	// the test namespace using `ko create -n`
	output = defaultNamespaceRE.ReplaceAll(output, []byte("namespace: "+namespace))

	// Replace image names to arch specific ones, where it's necessary
	for existingImage, archSpecificImage := range getImagesMappingRE(e.Platform) {
		output = existingImage.ReplaceAll(output, archSpecificImage)
	}
	return output, nil
}

// getImagesMappingRE generates the map ready to search and replace image names with regexp for examples files.
// search is done using "image: <name>" pattern.
func getImagesMappingRE(platform string) map[*regexp.Regexp][]byte {
	imageNamesMapping := imageNamesMapping(platform)
	imageMappingRE := make(map[*regexp.Regexp][]byte, len(imageNamesMapping))

	for existingImage, archSpecificImage := range imageNamesMapping {
		imageMappingRE[regexp.MustCompile("(?im)image: "+existingImage+"$")] = []byte("image: " + archSpecificImage)
	}

	return imageMappingRE
}

// imageNamesMapping provides mapping between image name in the examples yaml files and desired image name for specific arch.
// by default empty map is returned.
func imageNamesMapping(platform string) map[string]string {
	m := make(map[string]string)
	m["registry"] = getImage(platform, RegistryImage)
	m["lachlanevenson/k8s-kubectl"] = getImage(platform, KubectlImage)
	m["gcr.io/kaniko-project/executor:v1.3.0"] = getImage(platform, KanikoImage)
	switch platform { // nolint: gocritic
	case "linux/s390x":
		m["node"] = "node:alpine3.11"
		m["gcr.io/cloud-builders/git"] = "alpine/git:latest"
		m["docker:dind"] = "ibmcom/docker-s390x:dind"
		m["docker"] = "docker:18.06.3"
		m["mikefarah/yq"] = "danielxlee/yq:2.4.0"
		m["stedolan/jq"] = "ibmcom/jq-s390x:latest"
	}
	return m
}
