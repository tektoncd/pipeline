package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realPostWriter actually writes files.
type realPostWriter struct{}

var _ entrypoint.PostWriter = (*realPostWriter)(nil)

func (*realPostWriter) Write(stepFs, stepID string, isErr bool) {
	stepReadyLocation := filepath.Join(stepFs, stepID, "ready")
	if stepID == "" {
		return
	}
	if isErr {
		stepReadyLocation = fmt.Sprintf("%s.err", stepReadyLocation)
	}
	if _, err := os.Create(stepReadyLocation); err != nil {
		log.Fatalf("Creating %q: %v", stepReadyLocation, err)
	}
}
