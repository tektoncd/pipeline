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
	//kubectl image
	kubectlImage
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
	if getTestArch() == "s390x" {
		return map[int]string{
			busyboxImage:  "busybox@sha256:4f47c01fa91355af2865ac10fef5bf6ec9c7f42ad2321377c21e844427972977",
			registryImage: "ibmcom/registry:2.6.2.5",
			kubectlImage:  "ibmcom/kubectl:v1.13.9",
		}
	}
	return map[int]string{
		busyboxImage:  "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649",
		registryImage: "registry",
		kubectlImage:  "lachlanevenson/k8s-kubectl",
	}
}

// getImagesMappingRE generates the map ready to search and replace image names with regexp for examples files.
// search is done using "image: <name>" pattern.
func getImagesMappingRE() map[*regexp.Regexp][]byte {
	imageNamesMapping := imageNamesMapping()
	imageMappingRE := make(map[*regexp.Regexp][]byte, len(imageNamesMapping))

	for existingImage, archSpecificImage := range imageNamesMapping {
		imageMappingRE[regexp.MustCompile("(?im)image: "+existingImage+"$")] = []byte("image: " + archSpecificImage)
	}

	return imageMappingRE
}

// imageNamesMapping provides mapping between image name in the examples yaml files and desired image name for specific arch.
// by default empty map is returned.
func imageNamesMapping() map[string]string {
	if getTestArch() == "s390x" {
		return map[string]string{
			"registry":                   getTestImage(registryImage),
			"node":                       "node:alpine3.11",
			"lachlanevenson/k8s-kubectl": getTestImage(kubectlImage),
			"gcr.io/cloud-builders/git":  "alpine/git:latest",
		}
	}

	return make(map[string]string)
}

// initExcludedTests provides list of excluded tests for e2e and exanples tests
func initExcludedTests() sets.String {
	if getTestArch() == "s390x" {
		return sets.NewString(
			//examples
			"TestExamples/v1alpha1/taskruns/dind-sidecar",
			"TestExamples/v1beta1/taskruns/dind-sidecar",
			"TestExamples/v1alpha1/taskruns/build-gcs-targz",
			"TestExamples/v1beta1/taskruns/build-gcs-targz",
			"TestExamples/v1alpha1/taskruns/build-push-kaniko",
			"TestExamples/v1alpha1/taskruns/pull-private-image",
			"TestExamples/v1beta1/taskruns/pull-private-image",
			"TestExamples/v1alpha1/pipelineruns/pipelinerun",
			"TestExamples/v1beta1/pipelineruns/pipelinerun",
			"TestExamples/v1beta1/taskruns/build-gcs-zip",
			"TestExamples/v1alpha1/taskruns/build-gcs-zip",
			"TestExamples/v1beta1/taskruns/docker-creds",
			"TestExamples/v1alpha1/taskruns/docker-creds",
			"TestExamples/v1alpha1/taskruns/gcs-resource",
			"TestExamples/v1beta1/taskruns/gcs-resource",
			"TestExamples/v1beta1/taskruns/authenticating-git-commands",
			"TestExamples/v1beta1/pipelineruns/pipelinerun-with-final-tasks",
			"TestExamples/v1beta1/taskruns/workspace-in-sidecar",
			//e2e
			"TestHelmDeployPipelineRun",
			"TestKanikoTaskRun",
			"TestPipelineRun/service_account_propagation_and_pipeline_param",
			"TestPipelineRun/pipelinerun_succeeds_with_LimitRange_minimum_in_namespace",
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
