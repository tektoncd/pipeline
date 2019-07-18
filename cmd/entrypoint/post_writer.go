package main

import (
	"log"
	"os"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// RealPostWriter actually writes files.
type RealPostWriter struct{}

var _ entrypoint.PostWriter = (*RealPostWriter)(nil)

func (*RealPostWriter) Write(file string) {
	if file == "" {
		return
	}
	if _, err := os.Create(file); err != nil {
		log.Fatalf("Creating %q: %v", file, err)
	}
}
