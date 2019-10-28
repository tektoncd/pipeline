package main

import (
	"log"
	"os"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realPostWriter actually writes files.
type realPostWriter struct{}

var _ entrypoint.PostWriter = (*realPostWriter)(nil)

func (*realPostWriter) Write(file string) {
	if file == "" {
		return
	}
	if _, err := os.Create(file); err != nil {
		log.Fatalf("Creating %q: %v", file, err)
	}
}
