package resolver

import "fmt"

type ErrorInvalidResourceKey struct {
	key      string
	original error
}

var _ error = &ErrorInvalidResourceKey{}

func (e *ErrorInvalidResourceKey) Error() string {
	return fmt.Sprintf("invalid resource key %q: %v", e.key, e.original)
}

func (e *ErrorInvalidResourceKey) Unwrap() error {
	return e.original
}

type ErrorGettingResource struct {
	kind     string
	key      string
	original error
}

var _ error = &ErrorGettingResource{}

func (e *ErrorGettingResource) Error() string {
	return fmt.Sprintf("error getting %s %q: %v", e.kind, e.key, e.original)
}

func (e *ErrorGettingResource) Unwrap() error {
	return e.original
}
