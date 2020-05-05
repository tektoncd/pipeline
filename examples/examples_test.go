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
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"os"
	"os/exec"

	"github.com/tektoncd/pipeline/pkg/names"
	knativetest "knative.dev/pkg/test"
)

const (
	timeoutSeconds = 600
	sleepBetween   = 10

	// we may want to consider either not running examples that require registry access
	// or doing something more sophisticated to inject the right registry in when folks
	// are executing the examples
	horribleHardCodedRegistry = "gcr.io/christiewilson-catfactory"
)

// formatLogger is a printf style function for logging in tests.
type formatLogger func(template string, args ...interface{})

// cmd will run the command c with args and if input is provided, that will be piped
// into the process as input
func cmd(logf formatLogger, c string, args []string, input string) (string, error) {
	// Remove braces from args when logging so users can see the complete call
	logf("Executing %s %v", c, strings.Trim(fmt.Sprint(args), "[]"))

	cmd := exec.Command(c, args...)
	cmd.Env = os.Environ()

	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		logf("couldn't run command %s %v: %v, %s", c, args, err, stderr.String())
		return "", err
	}

	return stdout.String(), nil
}

// eraseClusterTasks will erase all cluster tasks which do not get cleaned up by namespace.
func eraseClusterTasks(logf formatLogger) {
	if _, err := cmd(logf, "kubectl", []string{"delete", "--ignore-not-found=true", "clustertasks.tekton.dev", "--all"}, ""); err != nil {
		logf("couldn't delete cluster tasks: %v", err)
	}
}

// getYamls will look in the directory in examples indicated by version and run for yaml files
func getYamls(t *testing.T, version, run string) []string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)

	// Don't read recursively; the only dir within these dirs is no-ci which doesn't
	// want any tests run against it
	files, err := ioutil.ReadDir(path.Join(path.Dir(filename), version, run))
	if err != nil {
		t.Fatalf("Couldn't read yaml files from %s/%s/%s: %v", path.Dir(filename), version, run, err)
	}
	yamls := []string{}
	for _, f := range files {
		if matches, _ := filepath.Match("*.yaml", f.Name()); matches {
			yamls = append(yamls, f.Name())
		}
	}
	return yamls
}

// replaceDockerRepo will look in the content f and replace the hard coded docker
// repo with the on provided via the KO_DOCKER_REPO environment variable
func replaceDockerRepo(t *testing.T, f string) string {
	t.Helper()
	r := os.Getenv("KO_DOCKER_REPO")
	if r == "" {
		t.Fatalf("KO_DOCKER_REPO must be set")
	}
	read, err := ioutil.ReadFile(f)
	if err != nil {
		t.Fatalf("couldnt read contents of %s: %v", f, err)
	}
	return strings.Replace(string(read), horribleHardCodedRegistry, r, -1)
}

// logRun will retrieve the entire yaml of run in namespace n and log it
func logRun(t *testing.T, n, run string) {
	t.Helper()
	yaml, err := cmd(t.Logf, "kubectl", []string{"--namespace", n, "get", run, "-o", "yaml"}, "")
	if err == nil {
		t.Logf(yaml)
	}
}

// pollRun will use kubectl to query the specified run in namespace n
// to see if it has completed. It will timeout after timeoutSeconds.
func pollRun(t *testing.T, n, run string, wg *sync.WaitGroup) {
	t.Helper()
	// instead of polling it might be faster to explore using --watch and parsing
	// the output as it returns
	for i := 0; i < (timeoutSeconds / sleepBetween); i++ {
		status, err := cmd(t.Logf, "kubectl", []string{"--namespace", n, "get", run, "--output=jsonpath={.status.conditions[*].status}"}, "")
		if err != nil {
			t.Fatalf("couldnt get status of %s: %v", run, err)
			wg.Done()
			return
		}

		switch status {
		case "", "Unknown":
			// Not finished running yet
			time.Sleep(sleepBetween * time.Second)
		case "True":
			t.Logf("%s completed successfully", run)
			wg.Done()
			return
		default:
			t.Errorf("%s has failed with status %s", run, status)
			logRun(t, n, run)

			wg.Done()
			return
		}
	}
	t.Errorf("%s did not complete within %d seconds", run, timeoutSeconds)
	logRun(t, n, run)
	wg.Done()
}

// waitForAllRuns will use kubectl to poll all runs in runs in namespace n
// until completed, failed, or timed out
func waitForAllRuns(t *testing.T, n string, runs []string) {
	t.Helper()

	var wg sync.WaitGroup
	for _, run := range runs {
		wg.Add(1)
		go pollRun(t, n, run, &wg)
	}
	wg.Wait()
}

// getRuns will look for "run" in the provided ko output to determine the names
// of any runs created.
func getRuns(k string) []string {
	runs := []string{}
	// this a pretty naive way of looking for these run names, it would be an
	// improvement to be more aware of the output format or to parse the yamls and
	// use a client to apply them so we can get the name of the created runs via
	// the client
	for _, s := range strings.Split(k, "\n") {
		name := strings.TrimSuffix(s, " created")
		if strings.Contains(name, "run") {
			runs = append(runs, name)
		}
	}
	return runs
}

// createRandomNamespace will create a namespace with a randomized name so that anything
// happening within this namespace will not conflict with other namespaces. It will return
// the name of the namespace.
func createRandomNamespace(t *testing.T) string {
	t.Helper()

	n := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("irkalla")
	_, err := cmd(t.Logf, "kubectl", []string{"create", "namespace", n}, "")
	if err != nil {
		t.Fatalf("could not create namespace %s for example: %v", n, err)
	}
	return n
}

// deleteNamespace will delete the namespace with kubectl.
func deleteNamespace(logf formatLogger, n string) {
	_, err := cmd(logf, "kubectl", []string{"delete", "namespace", n}, "")
	if err != nil {
		logf("could not delete namespace %s for example: %v", n, err)
	}
}

// runTests will use ko to create all yamls in the directory indicated by version
// and run, and wait for all runs (PipelineRuns and TaskRuns) created
func runTests(t *testing.T, version, run string) {
	yamls := getYamls(t, version, run)
	for _, yaml := range yamls {
		y := yaml

		t.Run(fmt.Sprintf("%s/%s", run, y), func(t *testing.T) {
			t.Parallel()

			// apply the yaml in its own namespace so it can run in parallel
			// with similar tests (e.g. v1alpha1 + v1beta1) without conflicting with each other
			// and we can easily cleanup.
			n := createRandomNamespace(t)
			knativetest.CleanupOnInterrupt(func() { deleteNamespace(t.Logf, n) }, t.Logf)
			defer deleteNamespace(t.Logf, n)

			t.Logf("Applying %s to namespace %s", y, n)
			content := replaceDockerRepo(t, fmt.Sprintf("%s/%s/%s", version, run, y))
			output, err := cmd(t.Logf, "ko", []string{"create", "--namespace", n, "-f", "-"}, content)
			if err == nil {
				runs := getRuns(output)

				if len(runs) == 0 {
					t.Fatalf("no runs were created for %s, output %s", y, output)
				}

				t.Logf("Waiting for created runs %v", runs)
				waitForAllRuns(t, n, runs)
			}
		})
	}
}

func TestYaml(t *testing.T) {
	versions := []string{"v1alpha1", "v1beta1"}
	runs := []string{"taskruns", "pipelineruns"}

	knativetest.CleanupOnInterrupt(func() { eraseClusterTasks(t.Logf) }, t.Logf)
	defer eraseClusterTasks(t.Logf)

	for _, version := range versions {
		for _, run := range runs {
			runTests(t, version, run)
		}
	}
}
