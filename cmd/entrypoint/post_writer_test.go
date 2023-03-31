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
	"os"
	"path/filepath"
	"testing"
)

func TestRealPostWriter_WriteFileContent(t *testing.T) {
	testdir := t.TempDir()
	tests := []struct {
		name, file, content string
	}{{
		name:    "write a file content",
		file:    "sample.txt",
		content: "this is a sample file",
	}, {
		name: "write a file without specifying any path",
	}, {
		name: "create an empty file",
		file: "sample.txt",
	}, {
		name: "create an empty file in new subdirectory",
		file: filepath.Join(testdir, "dir", "sample.txt"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := realPostWriter{}
			rw.Write(tt.file, tt.content)
			if tt.file != "" {
				defer os.Remove(tt.file)
				if _, err := os.Stat(tt.file); err != nil {
					t.Fatalf("Failed to create a file %q", tt.file)
				}
				b, err := os.ReadFile(tt.file)
				if err != nil {
					t.Fatalf("Failed to read the file %q", tt.file)
				}
				if tt.content != string(b) {
					t.Fatalf("Failed to write the desired content %q to the file %q", tt.content, tt.file)
				}
			}
		})
	}
}
