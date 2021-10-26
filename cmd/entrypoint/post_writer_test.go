package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRealPostWriter_WriteFileContent(t *testing.T) {
	testdir, err := ioutil.TempDir("", "post-writer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testdir)
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
				b, err := ioutil.ReadFile(tt.file)
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
