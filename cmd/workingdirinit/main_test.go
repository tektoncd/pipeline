//go:build linux

/*
Copyright 2023 The Tekton Authors

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
package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testCase struct {
	value    string
	expected string
}

var testCases []testCase = []testCase{
	{
		// Test trailing path separator
		value:    "/workspace/",
		expected: "/workspace",
	},
	{
		// Test workspace subdirectory
		value:    "/workspace/foobar",
		expected: "/workspace/foobar",
	},
	{
		// Test double path separator
		value:    "/foo//bar",
		expected: "/foo/bar",
	},
	{
		// Test relative path with './' prefix
		value:    "./foo/bar",
		expected: "foo/bar",
	},
	{
		// Test relative path with no prefix
		value:    "foo/bar",
		expected: "foo/bar",
	},
	{
		// Test empty string
		value:    "",
		expected: ".",
	},
}

func TestCleanPath(t *testing.T) {
	for _, tc := range testCases {
		diff := cmp.Diff(tc.expected, cleanPath(tc.value))

		if diff != "" {
			t.Errorf("diff(-want, +got): %s", diff)
		}
	}
}
