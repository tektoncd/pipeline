package resolution

import (
	"errors"
	"testing"
)

type TestError struct{}

var _ error = &TestError{}

func (*TestError) Error() string {
	return "test error"
}

func TestResolutionErrorUnwrap(t *testing.T) {
	originalError := &TestError{}
	resolutionError := NewError("", originalError)
	if !errors.Is(resolutionError, &TestError{}) {
		t.Errorf("resolution error expected to unwrap successfully")
	}
}

func TestResolutionErrorMessage(t *testing.T) {
	originalError := errors.New("this is just a test message")
	resolutionError := NewError("", originalError)
	if resolutionError.Error() != originalError.Error() {
		t.Errorf("resolution error message expected to equal that of original error")
	}
}
