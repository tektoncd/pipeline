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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// TestInfo represents metadata about a test function
type TestInfo struct {
	Name      string
	File      string
	Line      int
	Execution string // "parallel" or "serial"
	Reason    string
}

// TestManifest holds categorized tests
type TestManifest struct {
	Parallel []TestInfo
	Serial   []TestInfo
	Unknown  []TestInfo
}

// TestMain initializes anything global needed by the tests. Right now this is just log and metric
// setup since the log and metric libs we're using use global state :(
func TestMain(m *testing.M) {
	flag.Parse()

	// Parse test annotations from source files
	manifest, err := scanTestAnnotations(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing test annotations: %v\n", err)
		os.Exit(1)
	}

	// Show categorization if requested
	if *showTests {
		showCategorization(manifest)
		os.Exit(0)
	}

	// Filter tests based on category
	var exitCode int
	switch *testCategory {
	case "parallel":
		filterTests(manifest.Parallel)
		fmt.Fprintf(os.Stderr, "Running %d parallel tests\n", len(manifest.Parallel))
		if len(manifest.Unknown) > 0 {
			fmt.Fprintf(os.Stderr, "WARNING: %d test(s) without @test:execution annotation\n", len(manifest.Unknown))
		}
		exitCode = m.Run()

	case "serial":
		filterTests(manifest.Serial)
		fmt.Fprintf(os.Stderr, "Running %d serial tests\n", len(manifest.Serial))
		if len(manifest.Unknown) > 0 {
			fmt.Fprintf(os.Stderr, "WARNING: %d test(s) without @test:execution annotation\n", len(manifest.Unknown))
		}
		exitCode = m.Run()

	case "all":
		// Run serial tests first, then parallel tests
		fmt.Fprintf(os.Stderr, "Running all tests in order: serial first (%d), then parallel (%d), then unknown (%d)\n",
			len(manifest.Serial), len(manifest.Parallel), len(manifest.Unknown))

		if len(manifest.Unknown) > 0 {
			fmt.Fprintf(os.Stderr, "WARNING: %d test(s) without @test:execution annotation will run last\n", len(manifest.Unknown))
		}

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

		// Finally run unknown/unannotated tests
		if len(manifest.Unknown) > 0 {
			fmt.Fprintf(os.Stderr, "\n=== Running Unannotated Tests ===\n")
			filterTests(manifest.Unknown)
			exitCode = m.Run()
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown category: %s (use: parallel, serial, or all)\n", *testCategory)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`\n", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	os.Exit(exitCode)
}

// scanTestAnnotations parses all *_test.go files and extracts test metadata
func scanTestAnnotations(dir string) (*TestManifest, error) {
	manifest := &TestManifest{
		Parallel: []TestInfo{},
		Serial:   []TestInfo{},
		Unknown:  []TestInfo{},
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-test files
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		tests, err := parseTestFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		for _, test := range tests {
			switch test.Execution {
			case "parallel":
				manifest.Parallel = append(manifest.Parallel, test)
			case "serial":
				manifest.Serial = append(manifest.Serial, test)
			default:
				manifest.Unknown = append(manifest.Unknown, test)
			}
		}

		return nil
	})

	return manifest, err
}

// parseTestFile extracts test metadata from a single file
func parseTestFile(filename string) ([]TestInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var tests []TestInfo

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Only process Test functions (skip TestMain)
		if !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
			continue
		}

		test := TestInfo{
			Name: fn.Name.Name,
			File: filename,
			Line: fset.Position(fn.Pos()).Line,
		}

		if fn.Doc != nil {
			test.Execution, test.Reason = parseAnnotations(fn.Doc)
		}

		tests = append(tests, test)
	}

	return tests, nil
}

// parseAnnotations extracts execution mode and reason from function doc comments
func parseAnnotations(doc *ast.CommentGroup) (execution, reason string) {
	for _, comment := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		text = strings.TrimSpace(strings.TrimPrefix(text, "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))

		if !strings.HasPrefix(text, "@test:") {
			continue
		}

		parts := strings.SplitN(strings.TrimPrefix(text, "@test:"), "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "execution":
			execution = value
		case "reason":
			reason = value
		}
	}

	return
}

// filterTests sets the test.run flag to only run specified tests
func filterTests(tests []TestInfo) {
	if len(tests) == 0 {
		// No tests to run - set impossible pattern
		flag.Set("test.run", "^$")
		return
	}

	names := make([]string, len(tests))
	for i, t := range tests {
		names[i] = t.Name
	}

	// Create regex pattern: ^(Test1|Test2|Test3)$
	pattern := "^(" + strings.Join(names, "|") + ")$"
	flag.Set("test.run", pattern)
}

// showCategorization prints test categorization and exits
func showCategorization(manifest *TestManifest) {
	fmt.Fprintf(os.Stdout, "Test Categorization\n")
	fmt.Fprintf(os.Stdout, "===================\n")
	fmt.Fprintf(os.Stdout, "\n")

	fmt.Fprintf(os.Stdout, "Parallel Tests (%d):\n", len(manifest.Parallel))
	for _, test := range manifest.Parallel {
		fmt.Fprintf(os.Stdout, "  - %s (%s:%d)\n", test.Name, filepath.Base(test.File), test.Line)
	}
	fmt.Fprintf(os.Stdout, "\n")

	fmt.Fprintf(os.Stdout, "Serial Tests (%d):\n", len(manifest.Serial))
	for _, test := range manifest.Serial {
		fmt.Fprintf(os.Stdout, "  - %s (%s:%d)\n", test.Name, filepath.Base(test.File), test.Line)
		if test.Reason != "" {
			fmt.Fprintf(os.Stdout, "    Reason: %s\n", test.Reason)
		}
	}
	fmt.Fprintf(os.Stdout, "\n")

	if len(manifest.Unknown) > 0 {
		fmt.Fprintf(os.Stdout, "Tests Without Annotation (%d):\n", len(manifest.Unknown))
		for _, test := range manifest.Unknown {
			fmt.Fprintf(os.Stdout, "  - %s (%s:%d)\n", test.Name, filepath.Base(test.File), test.Line)
		}
		fmt.Fprintf(os.Stdout, "\n")
	}

	fmt.Fprintf(os.Stdout, "Usage:\n")
	fmt.Fprintf(os.Stdout, "  go test -tags=e2e -category=parallel ./test\n")
	fmt.Fprintf(os.Stdout, "  go test -tags=e2e -category=serial ./test\n")
	fmt.Fprintf(os.Stdout, "  go test -tags=e2e -category=all ./test\n")
}
