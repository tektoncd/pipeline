/*
Copyright 2025 The Tekton Authors

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

// Package annotation provides utilities for parsing and managing test annotations
// that categorize e2e tests as parallel or serial execution.
package annotation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// TestInfo represents metadata about a test function
type TestInfo struct {
	Name      string
	File      string
	Line      int
	Execution string // "parallel" or "serial"
	Reason    string // Required for serial tests
}

// TestManifest holds categorized tests
type TestManifest struct {
	Parallel []TestInfo
	Serial   []TestInfo
}

// ScanOptions configures the behavior of test annotation scanning
type ScanOptions struct {
	RequireAnnotations bool // If true, fail on unannotated tests
	RequireReason      bool // If true, fail on serial tests without reason
}

// DefaultScanOptions returns the default scanning options
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		RequireAnnotations: true,
		RequireReason:      true,
	}
}

// ScanTestAnnotations parses all *_test.go files in a directory and extracts test metadata.
// It enforces annotation requirements based on the provided options.
func ScanTestAnnotations(dir string, opts ScanOptions) (*TestManifest, error) {
	manifest := &TestManifest{
		Parallel: []TestInfo{},
		Serial:   []TestInfo{},
	}

	var unannotated []TestInfo
	var missingReason []TestInfo

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
				if opts.RequireReason && test.Reason == "" {
					missingReason = append(missingReason, test)
				}
				manifest.Serial = append(manifest.Serial, test)
			default:
				unannotated = append(unannotated, test)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Check for violations of requirements
	if opts.RequireAnnotations && len(unannotated) > 0 {
		return nil, fmt.Errorf("found %d test(s) without @test:execution annotation:\n%s",
			len(unannotated), formatTestList(unannotated))
	}

	if opts.RequireReason && len(missingReason) > 0 {
		return nil, fmt.Errorf("found %d serial test(s) without @test:reason annotation:\n%s",
			len(missingReason), formatTestList(missingReason))
	}

	return manifest, nil
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

// formatTestList formats a slice of TestInfo for error messages
func formatTestList(tests []TestInfo) string {
	var sb strings.Builder
	for _, test := range tests {
		sb.WriteString(fmt.Sprintf("  - %s (%s:%d)\n", test.Name, test.File, test.Line))
	}
	return sb.String()
}

// FilterPattern creates a regex pattern to match only the specified tests
func FilterPattern(tests []TestInfo) string {
	if len(tests) == 0 {
		// No tests to run - return impossible pattern
		return "^$"
	}

	names := make([]string, len(tests))
	for i, t := range tests {
		names[i] = t.Name
	}

	// Create regex pattern: ^(Test1|Test2|Test3)$
	return "^(" + strings.Join(names, "|") + ")$"
}
