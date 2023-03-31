/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common_test

import (
	"errors"
	"testing"

	common "github.com/tektoncd/pipeline/pkg/resolution/common"
)

type TestError struct{}

var _ error = &TestError{}

func (*TestError) Error() string {
	return "test error"
}

func TestResolutionErrorUnwrap(t *testing.T) {
	originalError := &TestError{}
	resolutionError := common.NewError("", originalError)
	if !errors.Is(resolutionError, &TestError{}) {
		t.Errorf("resolution error expected to unwrap successfully")
	}
}

func TestResolutionErrorMessage(t *testing.T) {
	originalError := errors.New("this is just a test message")
	resolutionError := common.NewError("", originalError)
	if resolutionError.Error() != originalError.Error() {
		t.Errorf("resolution error message expected to equal that of original error")
	}
}
