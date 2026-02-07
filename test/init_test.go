//go:build conformance || e2e || examples

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

// This file contains initialization logic for the tests, such as special magical global state that needs to be initialized.

package test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tektoncd/pipeline/internal/test/annotation"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // Mysteriously by k8s libs, or they fail to create `KubeClient`s when using oidc authentication. Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	knativetest "knative.dev/pkg/test"
)

var (
	skipRootUserTests = false
	testCategory      = flag.String("category", "all", "Test category to run: parallel, serial, or all")
	showTests         = flag.Bool("show-tests", false, "Show test categorization and exit")
)

func init() {
	flag.BoolVar(&skipRootUserTests, "skipRootUserTests", false, "Skip tests that require root user")
}

// TestMain initializes anything global needed by the tests. Right now this is just log and metric
// setup since the log and metric libs we're using use global state :(
func TestMain(m *testing.M) {
	flag.Parse()

	// Parse test annotations from source files with strict requirements
	opts := annotation.DefaultScanOptions()
	manifest, err := annotation.ScanTestAnnotations(".", opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Show categorization if requested
	if *showTests {
		showCategorization(manifest)
		os.Exit(0)
	}

	// Check if user provided explicit -run flag
	// If so, skip category filtering and run normally to respect user's choice
	runFlag := flag.Lookup("test.run")
	if runFlag != nil && runFlag.Value.String() != "" {
		exitCode := m.Run()
		fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`\n", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
		os.Exit(exitCode)
	}

	// Filter tests based on category
	var exitCode int
	switch *testCategory {
	case "parallel":
		filterTests(manifest.Parallel)
		fmt.Fprintf(os.Stderr, "Running %d parallel tests\n", len(manifest.Parallel))
		exitCode = m.Run()

	case "serial":
		filterTests(manifest.Serial)
		fmt.Fprintf(os.Stderr, "Running %d serial tests\n", len(manifest.Serial))
		exitCode = m.Run()

	case "all":
		// Run serial tests first, then parallel tests
		fmt.Fprintf(os.Stderr, "Running all tests in order: serial first (%d), then parallel (%d)\n",
			len(manifest.Serial), len(manifest.Parallel))

		// First run serial tests
		if len(manifest.Serial) > 0 {
			fmt.Fprintf(os.Stderr, "\n=== Running Serial Tests ===\n")
			filterTests(manifest.Serial)
			exitCode = m.Run()
			if exitCode != 0 {
				fmt.Fprintf(os.Stderr, "Serial tests failed with exit code %d\n", exitCode)
				fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`\n", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
				os.Exit(exitCode)
			}
		}

		// Then run parallel tests
		if len(manifest.Parallel) > 0 {
			fmt.Fprintf(os.Stderr, "\n=== Running Parallel Tests ===\n")
			filterTests(manifest.Parallel)
			exitCode = m.Run()
			if exitCode != 0 {
				fmt.Fprintf(os.Stderr, "Parallel tests failed with exit code %d\n", exitCode)
				fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`\n", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
				os.Exit(exitCode)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown category: %s (use: parallel, serial, or all)\n", *testCategory)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`\n", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	os.Exit(exitCode)
}

// filterTests sets the test.run flag to only run specified tests
func filterTests(tests []annotation.TestInfo) {
	pattern := annotation.FilterPattern(tests)
	flag.Set("test.run", pattern)
}

// showCategorization prints test categorization and exits
func showCategorization(manifest *annotation.TestManifest) {
	fmt.Fprintf(os.Stdout, "Test Categorization\n")
	fmt.Fprintf(os.Stdout, "===================\n")
	fmt.Fprintf(os.Stdout, "\n")

	// Show serial tests first since they run first
	fmt.Fprintf(os.Stdout, "Serial Tests (%d):\n", len(manifest.Serial))
	for _, test := range manifest.Serial {
		fmt.Fprintf(os.Stdout, "  - %s (%s:%d)\n", test.Name, filepath.Base(test.File), test.Line)
		if test.Reason != "" {
			fmt.Fprintf(os.Stdout, "    Reason: %s\n", test.Reason)
		}
	}
	fmt.Fprintf(os.Stdout, "\n")

	fmt.Fprintf(os.Stdout, "Parallel Tests (%d):\n", len(manifest.Parallel))
	for _, test := range manifest.Parallel {
		fmt.Fprintf(os.Stdout, "  - %s (%s:%d)\n", test.Name, filepath.Base(test.File), test.Line)
	}
	fmt.Fprintf(os.Stdout, "\n")

	fmt.Fprintf(os.Stdout, "Usage:\n")
	fmt.Fprintf(os.Stdout, "  go test -tags=e2e -category=parallel ./test\n")
	fmt.Fprintf(os.Stdout, "  go test -tags=e2e -category=serial ./test\n")
	fmt.Fprintf(os.Stdout, "  go test -tags=e2e -category=all ./test\n")
}
