package testing

import (
	"errors"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// NopGetCustomRun returns an error for CustomRun lookup (used in tests where CustomRun should not be called)
func NopGetCustomRun(string) (*v1beta1.CustomRun, error) {
	return nil, errors.New("GetCustomRun should not be called")
}

// NopGetTask returns an error for Task lookup (used in tests where Task should not be called)
func NopGetTask(string) (*v1.Task, error) {
	return nil, errors.New("GetTask should not be called")
}

// NopGetPipeline returns an error for Pipeline lookup (used in tests where Pipeline should not be called)
func NopGetPipeline(string) (*v1.Pipeline, error) {
	return nil, errors.New("GetPipeline should not be called")
}
