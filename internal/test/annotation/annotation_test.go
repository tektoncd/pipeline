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

package annotation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAnnotations(t *testing.T) {
	tests := []struct {
		name              string
		testFileContent   string
		wantParallel      int
		wantSerial        int
		requireAnnotation bool
		requireReason     bool
		wantError         bool
	}{
		{
			name: "parallel test with annotation",
			testFileContent: `package test
// TestParallelTest tests something
// @test:execution=parallel
func TestParallelTest(t *testing.T) {}
`,
			wantParallel:      1,
			wantSerial:        0,
			requireAnnotation: true,
			requireReason:     true,
			wantError:         false,
		},
		{
			name: "serial test with reason",
			testFileContent: `package test
// TestSerialTest tests something
// @test:execution=serial
// @test:reason=modifies shared ConfigMap state
func TestSerialTest(t *testing.T) {}
`,
			wantParallel:      0,
			wantSerial:        1,
			requireAnnotation: true,
			requireReason:     true,
			wantError:         false,
		},
		{
			name: "serial test without reason fails when required",
			testFileContent: `package test
// TestSerialTest tests something
// @test:execution=serial
func TestSerialTest(t *testing.T) {}
`,
			wantParallel:      0,
			wantSerial:        1,
			requireAnnotation: true,
			requireReason:     true,
			wantError:         true,
		},
		{
			name: "serial test without reason allowed when not required",
			testFileContent: `package test
// TestSerialTest tests something
// @test:execution=serial
func TestSerialTest(t *testing.T) {}
`,
			wantParallel:      0,
			wantSerial:        1,
			requireAnnotation: true,
			requireReason:     false,
			wantError:         false,
		},
		{
			name: "unannotated test fails when annotations required",
			testFileContent: `package test
// TestUnannotated tests something
func TestUnannotated(t *testing.T) {}
`,
			wantParallel:      0,
			wantSerial:        0,
			requireAnnotation: true,
			requireReason:     true,
			wantError:         true,
		},
		{
			name: "unannotated test allowed when not required",
			testFileContent: `package test
// TestUnannotated tests something
func TestUnannotated(t *testing.T) {}
`,
			wantParallel:      0,
			wantSerial:        0,
			requireAnnotation: false,
			requireReason:     false,
			wantError:         false,
		},
		{
			name: "multiple tests mixed",
			testFileContent: `package test
// TestParallel1 tests something
// @test:execution=parallel
func TestParallel1(t *testing.T) {}

// TestSerial1 tests something
// @test:execution=serial
// @test:reason=modifies ConfigMap
func TestSerial1(t *testing.T) {}

// TestParallel2 tests something
// @test:execution=parallel
func TestParallel2(t *testing.T) {}
`,
			wantParallel:      2,
			wantSerial:        1,
			requireAnnotation: true,
			requireReason:     true,
			wantError:         false,
		},
		{
			name: "TestMain is skipped",
			testFileContent: `package test
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// TestParallel tests something
// @test:execution=parallel
func TestParallel(t *testing.T) {}
`,
			wantParallel:      1,
			wantSerial:        0,
			requireAnnotation: true,
			requireReason:     true,
			wantError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test_test.go")

			// Write test file
			if err := os.WriteFile(testFile, []byte(tt.testFileContent), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Scan annotations
			opts := ScanOptions{
				RequireAnnotations: tt.requireAnnotation,
				RequireReason:      tt.requireReason,
			}
			manifest, err := ScanTestAnnotations(tmpDir, opts)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(manifest.Parallel) != tt.wantParallel {
				t.Errorf("Expected %d parallel tests, got %d", tt.wantParallel, len(manifest.Parallel))
			}

			if len(manifest.Serial) != tt.wantSerial {
				t.Errorf("Expected %d serial tests, got %d", tt.wantSerial, len(manifest.Serial))
			}
		})
	}
}

func TestFilterPattern(t *testing.T) {
	tests := []struct {
		name  string
		tests []TestInfo
		want  string
	}{
		{
			name:  "empty list returns impossible pattern",
			tests: []TestInfo{},
			want:  "^$",
		},
		{
			name: "single test",
			tests: []TestInfo{
				{Name: "TestFoo"},
			},
			want: "^(TestFoo)$",
		},
		{
			name: "multiple tests",
			tests: []TestInfo{
				{Name: "TestFoo"},
				{Name: "TestBar"},
				{Name: "TestBaz"},
			},
			want: "^(TestFoo|TestBar|TestBaz)$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterPattern(tt.tests)
			if got != tt.want {
				t.Errorf("FilterPattern() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTestFile(t *testing.T) {
	testContent := `package test

import "testing"

// TestWithComment tests something
// @test:execution=parallel
func TestWithComment(t *testing.T) {}

// TestWithMultipleComments tests something
// This is a description
// @test:execution=serial
// @test:reason=modifies state
func TestWithMultipleComments(t *testing.T) {}

func TestWithoutComment(t *testing.T) {}

// Not a test function
func Helper() {}

// TestMain is special
func TestMain(m *testing.M) {}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_test.go")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tests, err := parseTestFile(testFile)
	if err != nil {
		t.Fatalf("parseTestFile() error = %v", err)
	}

	// Should find 3 tests (TestWithComment, TestWithMultipleComments, TestWithoutComment)
	// TestMain and Helper should be excluded
	if len(tests) != 3 {
		t.Errorf("Expected 3 tests, got %d", len(tests))
	}

	// Verify first test
	if tests[0].Name != "TestWithComment" {
		t.Errorf("Expected first test name TestWithComment, got %s", tests[0].Name)
	}
	if tests[0].Execution != "parallel" {
		t.Errorf("Expected execution=parallel, got %s", tests[0].Execution)
	}

	// Verify second test
	if tests[1].Name != "TestWithMultipleComments" {
		t.Errorf("Expected second test name TestWithMultipleComments, got %s", tests[1].Name)
	}
	if tests[1].Execution != "serial" {
		t.Errorf("Expected execution=serial, got %s", tests[1].Execution)
	}
	if tests[1].Reason != "modifies state" {
		t.Errorf("Expected reason='modifies state', got %s", tests[1].Reason)
	}

	// Verify third test (unannotated)
	if tests[2].Name != "TestWithoutComment" {
		t.Errorf("Expected third test name TestWithoutComment, got %s", tests[2].Name)
	}
	if tests[2].Execution != "" {
		t.Errorf("Expected empty execution for unannotated test, got %s", tests[2].Execution)
	}
}

func TestFormatTestList(t *testing.T) {
	tests := []TestInfo{
		{Name: "TestFoo", File: "/path/to/foo_test.go", Line: 10},
		{Name: "TestBar", File: "/path/to/bar_test.go", Line: 20},
	}

	result := formatTestList(tests)

	if !strings.Contains(result, "TestFoo") {
		t.Error("Expected result to contain TestFoo")
	}
	if !strings.Contains(result, "TestBar") {
		t.Error("Expected result to contain TestBar")
	}
	if !strings.Contains(result, "foo_test.go:10") {
		t.Error("Expected result to contain file:line for TestFoo")
	}
	if !strings.Contains(result, "bar_test.go:20") {
		t.Error("Expected result to contain file:line for TestBar")
	}
}
