package main

import (
	"log"
	"os"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realPostWriter actually writes files.
type realPostWriter struct{}

var _ entrypoint.PostWriter = (*realPostWriter)(nil)

// Write creates a file and writes content to that file if content is specified
// assumption here is the underlying directory structure already exists
func (*realPostWriter) Write(file string, content string) {
	if file == "" {
		return
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

// CreateDirWithSymlink creates the specified directory and a symbolic link to that directory
func (*realPostWriter) CreateDirWithSymlink(source, link string) {
	if source == "" {
		return
	}
	if err := os.MkdirAll(source, 0770); err != nil {
		log.Fatalf("Creating directory %q: %v", source, err)
	}

	if link == "" {
		return
	}
	// create a symlink if it does not exist
	if _, err := os.Stat(link); os.IsNotExist(err) {
		// check if a source exist before creating a symbolic link
		if _, err := os.Stat(source); err == nil {
			if err := os.Symlink(source, link); err != nil {
				log.Fatalf("Creating a symlink %q: %v", link, err)
			}
		}
	}
}
