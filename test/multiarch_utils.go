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
	"runtime"
	"testing"
)

var (
	imageNames    = initImageNames()
	excludedTests = initExcludedTests()
)

const (
	// Busybox image with specific sha
	BusyboxSha = iota
	// Registry image
	Registry
)

// return architecture of the cluster where test suites will be executed.
// default value is similar to build architecture, TEST_RUNTIME_ARCH is used when test target cluster has another architecture
func getTestArch() string {
	val, ok := os.LookupEnv("TEST_RUNTIME_ARCH")
	if ok {
		return val
	}
	return runtime.GOARCH
}

func initImageNames() map[int]string {
	mapping := make(map[int]string)

	switch getTestArch() {
	case "s390x":
		mapping[BusyboxSha] = "busybox@sha256:4f47c01fa91355af2865ac10fef5bf6ec9c7f42ad2321377c21e844427972977"
		mapping[Registry] = "ibmcom/registry:2.6.2.5"

	default:
		mapping[BusyboxSha] = "busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649"
		mapping[Registry] = "registry"
	}
	return mapping
}

func initExcludedTests() map[string]bool {
	mapping := make(map[string]bool)
	tests := []string{}
	switch getTestArch() {
	case "s390x":
		//examples
		tests = []string{
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
			"TestExamples/v1alpha1/taskruns/git-volume",
			"TestExamples/v1beta1/taskruns/git-volume",
			"TestExamples/v1beta1/taskruns/docker-creds",
			"TestExamples/v1alpha1/taskruns/docker-creds",
			"TestExamples/v1beta1/taskruns/steps-run-in-order",
			"TestExamples/v1alpha1/taskruns/steps-run-in-order",
			"TestExamples/v1beta1/taskruns/step-by-digest",
			"TestExamples/v1alpha1/taskruns/step-by-digest",
			"TestExamples/v1alpha1/taskruns/gcs-resource",
			"TestExamples/v1beta1/taskruns/gcs-resource",
			"TestExamples/v1beta1/taskruns/authenticating-git-commands",
			"TestExamples/v1beta1/pipelineruns/pipelinerun-with-final-tasks",
			"TestExamples/v1beta1/taskruns/pullrequest_input_copystep_output",
			"TestExamples/v1alpha1/taskruns/pullrequest_input_copystep_output",
			"TestExamples/v1beta1/taskruns/pullrequest",
			"TestExamples/v1alpha1/taskruns/pullrequest",
			"TestExamples/v1beta1/taskruns/step-script",
			"TestExamples/v1alpha1/taskruns/step-script",
			"TestExamples/v1beta1/pipelineruns/conditional-pipelinerun",
			"TestExamples/v1alpha1/pipelineruns/pipelinerun-with-resourcespec",
			"TestExamples/v1beta1/pipelineruns/pipelinerun-with-resourcespec",
			"TestExamples/v1beta1/taskruns/git-ssh-creds-without-known_hosts",
			"TestExamples/v1alpha1/taskruns/optional-resources",
			"TestExamples/v1beta1/taskruns/optional-resources",
			"TestExamples/v1beta1/taskruns/task-output-image",
			//e2e
			"TestEntrypointRunningStepsInOrder",
			"TestWorkingDirIgnoredNonSlashWorkspace",
			"TestTaskRun_EmbeddedResource",
			"TestTaskRunPipelineRunCancel",
			"TestEntrypointRunningStepsInOrder",
			"TestGitPipelineRun",
			"TestGitPipelineRunFail",
			"TestGitPipelineRunWithRefspec",
			"TestGitPipelineRun_Disable_SSLVerify",
			"TestGitPipelineRunWithNonMasterBranch",
			"TestHelmDeployPipelineRun",
			"TestKanikoTaskRun",
			"TestPipelineRun",
			"TestSidecarTaskSupport",
			"TestWorstkingDirCreated",
			"TestWorkingDirIgnoredNonSlashWorkspace",
			"TestWorkingDirCreated",
			"TestPipelineRun/service_account_propagation_and_pipeline_param",
			"TestPipelineRun/pipelinerun_succeeds_with_LimitRange_minimum_in_namespace",
		}
	default:
		// do nothing
	}

	for _, test := range tests {
		mapping[test] = true
	}

	return mapping
}

// get test image based on unique id
func GetTestImage(image int) string {
	return imageNames[image]
}

// check if test name is in the excluded list and skip it
func SkipIfExcluded(t *testing.T) {
	if excludedTests[t.Name()] {
		t.Skipf("skip for %s architecture", getTestArch())
	}
}
