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

package test

import (
	"os"
	"regexp"
	"runtime"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	imageNames      = initImageNames()
	excludedTests   = initExcludedTests()
	imagesMappingRE map[*regexp.Regexp][]byte
)

const (
	// Busybox image with specific sha
	busyboxImage = iota
	// Registry image
	registryImage
	// kubectl image
	kanikoImage
	// dockerize image
	dockerizeImage
)

func init() {
	imagesMappingRE = getImagesMappingRE()
}

// getTestArch returns architecture of the cluster where test suites will be executed.
// default value is similar to build architecture, TEST_RUNTIME_ARCH is used when test target cluster has another architecture
func getTestArch() string {
	val, ok := os.LookupEnv("TEST_RUNTIME_ARCH")
	if ok {
		return val
	}
	return runtime.GOARCH
}

// initImageNames returns the map with arch dependent image names for e2e tests
func initImageNames() map[int]string {
	switch getTestArch() {
	case "s390x":
		return map[int]string{
			busyboxImage:   "busybox@sha256:4f47c01fa91355af2865ac10fef5bf6ec9c7f42ad2321377c21e844427972977",
			registryImage:  "ibmcom/registry:2.6.2.5",
			kanikoImage:    "gcr.io/kaniko-project/executor:s390x-9ed158c1f63a059cde4fd5f8b95af51d452d9aa7",
			dockerizeImage: "ibmcom/dockerize-s390x",
		}
	case "ppc64le":
		return map[int]string{
			busyboxImage:   "busybox@sha256:4f47c01fa91355af2865ac10fef5bf6ec9c7f42ad2321377c21e844427972977",
			registryImage:  "ppc64le/registry:2",
			kanikoImage:    "ibmcom/kaniko-project-executor-ppc64le:v0.17.1",
			dockerizeImage: "ibmcom/dockerize-ppc64le",
		}
	default:
		return map[int]string{
			busyboxImage:   "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649",
			registryImage:  "registry",
			kanikoImage:    "gcr.io/kaniko-project/executor:v1.3.0",
			dockerizeImage: "jwilder/dockerize",
		}
	}
}

// getImagesMappingRE generates the map ready to search and replace image names with regexp for examples files.
// search is done using "image: <name>" pattern.
func getImagesMappingRE() map[*regexp.Regexp][]byte {
	imageNamesMapping := imageNamesMapping()
	imageMappingRE := make(map[*regexp.Regexp][]byte, len(imageNamesMapping))

	for existingImage, archSpecificImage := range imageNamesMapping {
		imageMappingRE[regexp.MustCompile("(?im)image: "+existingImage+"$")] = []byte("image: " + archSpecificImage)
		imageMappingRE[regexp.MustCompile("(?im)default: "+existingImage+"$")] = []byte("default: " + archSpecificImage)
	}

	return imageMappingRE
}

// imageNamesMapping provides mapping between image name in the examples yaml files and desired image name for specific arch.
// by default empty map is returned.
func imageNamesMapping() map[string]string {

	switch getTestArch() {
	case "s390x":
		return map[string]string{
			"registry":                              getTestImage(registryImage),
			"node":                                  "node:alpine3.11",
			"gcr.io/cloud-builders/git":             "alpine/git:latest",
			"docker:dind":                           "ibmcom/docker-s390x:dind",
			"docker":                                "docker:18.06.3",
			"mikefarah/yq:3":                        "danielxlee/yq:2.4.0",
			"stedolan/jq":                           "ibmcom/jq-s390x:latest",
			"amd64/ubuntu":                          "s390x/ubuntu",
			"gcr.io/kaniko-project/executor:v1.3.0": getTestImage(kanikoImage),
		}
	case "ppc64le":
		return map[string]string{
			"registry":                              getTestImage(registryImage),
			"node":                                  "node:alpine3.11",
			"gcr.io/cloud-builders/git":             "alpine/git:latest",
			"docker:dind":                           "ibmcom/docker-ppc64le:19.03-dind",
			"docker":                                "docker:18.06.3",
			"mikefarah/yq:3":                        "danielxlee/yq:2.4.0",
			"stedolan/jq":                           "ibmcom/jq-ppc64le:latest",
			"gcr.io/kaniko-project/executor:v1.3.0": getTestImage(kanikoImage),
		}

	}

	return make(map[string]string)
}

// initExcludedTests provides list of excluded tests for e2e and exanples tests
func initExcludedTests() sets.String {

	switch getTestArch() {
	case "s390x":
		return sets.NewString(
			// examples
			"TestExamples/v1alpha1/taskruns/gcs-resource",
			"TestExamples/v1beta1/taskruns/gcs-resource",
			"TestExamples/v1beta1/taskruns/creds-init-only-mounts-provided-credentials",
		)
	case "ppc64le":
		return sets.NewString(
			// examples
			"TestExamples/v1alpha1/taskruns/gcs-resource",
			"TestExamples/v1beta1/taskruns/gcs-resource",
		)
	}

	return sets.NewString()
}

// getTestImage gets test image based on unique id
func getTestImage(image int) string {
	return imageNames[image]
}

// skipIfExcluded checks if test name is in the excluded list and skip it
func skipIfExcluded(t *testing.T) {
	if excludedTests.Has(t.Name()) {
		t.Skipf("skip for %s architecture", getTestArch())
	}
}
