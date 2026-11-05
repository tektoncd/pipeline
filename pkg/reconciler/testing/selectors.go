package testing

import (
	"errors"

	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// NopGetCustomRun returns an error for CustomRun lookup (used in tests where CustomRun should not be called)
func NopGetCustomRun(string) (*v1beta1.CustomRun, error) {
	return nil, errors.New("GetCustomRun should not be called")
}
