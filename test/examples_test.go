//go:build examples
// +build examples

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

package test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

var (
	defaultKoDockerRepoRE = regexp.MustCompile("gcr.io/christiewilson-catfactory")
	imagesMappingRE       = getImagesMappingRE()
)

// getCreatedTektonCRD parses output of an external ko invocation provided as
// input, as is the kind of Tekton CRD to search for (ie. taskrun)
func getCreatedTektonCRD(input []byte, kind string) (string, error) {
	re := regexp.MustCompile(kind + `.tekton.dev\/(.+) created`)
	submatch := re.FindSubmatch(input)
	if submatch == nil || len(submatch) < 2 {
		return "", nil
	}
	return string(submatch[1]), nil
}

func waitValidatePipelineRunDone(ctx context.Context, t *testing.T, c *clients, pipelineRunName string) {
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout, Succeed(pipelineRunName), pipelineRunName, v1beta1Version); err != nil {
		t.Fatalf("Failed waiting for pipeline run done: %v", err)
	}
}

func waitValidateV1PipelineRunDone(ctx context.Context, t *testing.T, c *clients, pipelineRunName string) {
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout, Succeed(pipelineRunName), pipelineRunName, v1Version); err != nil {
		t.Fatalf("Failed waiting for V1 pipeline run done: %v", err)
	}
}

func waitValidateTaskRunDone(ctx context.Context, t *testing.T, c *clients, taskRunName string) {
	// Per test basis
	if err := WaitForTaskRunState(ctx, c, taskRunName, Succeed(taskRunName), taskRunName, v1beta1Version); err != nil {
		t.Fatalf("Failed waiting for task run done: %v", err)
	}
}

func waitValidateV1TaskRunDone(ctx context.Context, t *testing.T, c *clients, taskRunName string) {
	// Per test basis
	if err := WaitForTaskRunState(ctx, c, taskRunName, Succeed(taskRunName), taskRunName, v1Version); err != nil {
		t.Fatalf("Failed waiting for V1 task run done: %v", err)
	}
}

// substituteEnv substitutes docker repos and bucket paths from the system
// environment for input to allow tests on local clusters. It unsets the
// namespace for ServiceAccounts so that they work under test. It also
// replaces image names to arch specific ones, based on provided mapping.
func substituteEnv(input []byte, namespace string) ([]byte, error) {
	// Replace the placeholder image repo with the value of the
	// KO_DOCKER_REPO env var.
	val, ok := os.LookupEnv("KO_DOCKER_REPO")
	if !ok {
		return nil, errors.New("KO_DOCKER_REPO is not set")
	}
	output := defaultKoDockerRepoRE.ReplaceAll(input, []byte(val))

	// Replace any "namespace: default"s with the test namespace.
	output = defaultNamespaceRE.ReplaceAll(output, []byte("namespace: "+namespace))

	// Replace image names to arch specific ones, where it's necessary
	for existingImage, archSpecificImage := range imagesMappingRE {
		output = existingImage.ReplaceAll(output, archSpecificImage)
	}
	return output, nil
}

// koCreate wraps the ko binary and invokes `ko create` for input within
// namespace
func koCreate(input []byte, namespace string) ([]byte, error) {
	cmd := exec.Command("ko", "create", "--platform", "linux/"+getTestArch(), "-f", "-", "--", "--namespace", namespace)
	cmd.Stdin = bytes.NewReader(input)
	return cmd.CombinedOutput()
}

// deleteClusterTask removes a single clustertask by name using provided
// clientset. Test state is used for logging. deleteClusterTask does not wait
// for the clustertask to be deleted, so it is still possible to have name
// conflicts during test
func deleteClusterTask(ctx context.Context, t *testing.T, c *clients, name string) {
	t.Logf("Deleting clustertask %s", name)
	if err := c.V1beta1ClusterTaskClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete clustertask: %v", err)
	}
}

type createFunc func(input []byte, namespace string) ([]byte, error)
type waitFunc func(ctx context.Context, t *testing.T, c *clients, name string)

func exampleTest(path string, waitValidateFunc waitFunc, createFunc createFunc, kind string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Setup unique namespaces for each test so they can run in complete
		// isolation
		c, namespace := setup(ctx, t)

		knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
		defer tearDown(ctx, t, c, namespace)

		inputExample, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Error reading file: %v", err)
		}

		subbedInput, err := substituteEnv(inputExample, namespace)
		if err != nil {
			t.Skipf("Couldn't substitute environment: %v", err)
		}

		out, err := createFunc(subbedInput, namespace)
		if err != nil {
			t.Fatalf("%s Output: %s", err, out)
		}

		// Parse from koCreate for now
		name, err := getCreatedTektonCRD(out, kind)
		if name == "" {
			// Nothing to check from ko create, this is not a taskrun or pipeline
			// run. Some examples in the directory do not directly output a TaskRun
			// or PipelineRun (ie. task-result.yaml).
			t.Skipf("pipelinerun or taskrun not created for %s", path)
		} else if err != nil {
			t.Fatalf("Failed to get created Tekton CRD of kind %s: %v", kind, err)
		}

		// NOTE: If an example creates more than one clustertask, they will not all
		// be cleaned up
		clustertask, err := getCreatedTektonCRD(out, "clustertask")
		if clustertask != "" {
			knativetest.CleanupOnInterrupt(func() { deleteClusterTask(ctx, t, c, clustertask) }, t.Logf)
			defer deleteClusterTask(ctx, t, c, clustertask)
		} else if err != nil {
			t.Fatalf("Failed to get created clustertask: %v", err)
		}

		waitValidateFunc(ctx, t, c, name)
	}
}

func getExamplePaths(t *testing.T, dir string, filter pathFilter) []string {
	var examplePaths []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatalf("couldn't walk path %s: %v", path, err)
		}
		// Do not append root and any other folders named "examples"
		if info.Name() == "examples" && info.IsDir() {
			return nil
		}
		if info.Name() == "no-ci" && info.IsDir() {
			return filepath.SkipDir
		}
		if !filter(path) {
			return nil
		}
		if info.IsDir() == false && filepath.Ext(info.Name()) == ".yaml" {
			// Ignore test matching the regexp in the TEST_EXAMPLES_IGNORES
			// environement variable.
			val, ok := os.LookupEnv("TEST_EXAMPLES_IGNORES")
			if ok {
				re := regexp.MustCompile(val)
				submatch := re.FindSubmatch([]byte(path))
				if submatch != nil {
					t.Logf("Skipping test %s", path)
					return nil
				}
			}
			t.Logf("Adding test %s", path)
			examplePaths = append(examplePaths, path)
			return nil
		}
		return nil
	})
	if err != nil {
		t.Fatalf("couldn't walk example directory %s: %v", dir, err)
	}

	return examplePaths
}

func extractTestName(baseDir string, path string) string {
	re := regexp.MustCompile(baseDir + "/(.+).yaml")
	submatch := re.FindSubmatch([]byte(path))
	if submatch == nil {
		return path
	}
	return string(submatch[1])
}

func TestExamples(t *testing.T) {
	pf, err := getPathFilter(t)
	if err != nil {
		t.Fatal(err.Error())
		return
	}
	testYamls(t, "../examples", kubectlCreate, pf)
}

func TestYamls(t *testing.T) {
	pf, err := getPathFilter(t)
	if err != nil {
		t.Fatal(err.Error())
		return
	}
	testYamls(t, "./yamls", koCreate, pf)
}

func testYamls(t *testing.T, baseDir string, createFunc createFunc, filter pathFilter) {
	t.Parallel()
	for _, path := range getExamplePaths(t, baseDir, filter) {
		path := path // capture range variable
		testName := extractTestName(baseDir, path)
		waitValidateFunc := waitValidatePipelineRunDone
		if strings.Contains(path, "/v1/") {
			waitValidateFunc = waitValidateV1PipelineRunDone
		}
		kind := "pipelinerun"

		if strings.Contains(path, "/taskruns/") {
			waitValidateFunc = waitValidateTaskRunDone
			if strings.Contains(path, "/v1/") {
				waitValidateFunc = waitValidateV1TaskRunDone
			}
			kind = "taskrun"
		}

		t.Run(testName, exampleTest(path, waitValidateFunc, createFunc, kind))
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
			"docker:dind":                           "ibmcom/docker-s390x:20.10",
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
