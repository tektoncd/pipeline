package main

import (
	"os"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realStepAgent is the real agent
type realStepAgent struct{}

var _ entrypoint.StepAgent = (*realStepAgent)(nil)

func (*realStepAgent) Init(stepFs, stepID string) {
	stepLocation := filepath.Join(stepFs, stepID)
	if _, err := os.Stat(stepLocation); os.IsNotExist(err) {
		_ = os.Mkdir(stepLocation, 0755)
	}
}
