package main

import (
	"log"
	"os"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realPostWriter actually writes files.
type realPostWriter struct {
	discardError bool
}

var _ entrypoint.PostWriter = (*realPostWriter)(nil)

func (rpw *realPostWriter) Write(file string, err error) {
	if err != nil && !rpw.discardError {
		file += ".err"
	}
	if _, err := os.Create(file); err != nil {
		log.Fatalf("Creating %q: %v", file, err)
	}
}
