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
	"log"
	"os"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realPostWriter actually writes files.
type realPostWriter struct{}

var _ entrypoint.PostWriter = (*realPostWriter)(nil)

// Write creates a file and writes content to that file if content is specified
func (*realPostWriter) Write(file string, content string) {
	if file == "" {
		return
	}

	// Create directory if it doesn't already exist
	if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
		log.Fatalf("Error creating parent directory of %q: %v", file, err)
	}

	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("Creating %q: %v", file, err)
	}

	if content != "" {
		if _, err := f.WriteString(content); err != nil {
			log.Fatalf("Writing %q: %v", file, err)
		}
	}
}
