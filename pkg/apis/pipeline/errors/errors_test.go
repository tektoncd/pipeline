/*
Copyright 2023 The Tekton Authors

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

package errors_test

import (
	"errors"
	"testing"

	pipelineErrors "github.com/tektoncd/pipeline/pkg/apis/pipeline/errors"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

type TestError struct{}

var _ error = &TestError{}

func (*TestError) Error() string {
	return "test error"
}

func TestUserErrorUnwrap(t *testing.T) {
	originalError := &TestError{}
	userError := pipelineErrors.WrapUserError(originalError)

	if !errors.Is(userError, &TestError{}) {
		t.Errorf("user error  expected to unwrap successfully")
	}
}

func TestResolutionErrorMessage(t *testing.T) {
	originalError := &TestError{}
	expectedErrorMessage := originalError.Error()

	userError := pipelineErrors.WrapUserError(originalError)

	if userError.Error() != expectedErrorMessage {
		t.Errorf("user error message expected to equal to %s, got: %s", expectedErrorMessage, userError.Error())
	}
}

func TestLabelsUserError(t *testing.T) {
	const hasUserError = true
	tcs := []struct {
		description string
		reason      string
		messages    []interface{}
		expected    string
	}{{
		description: "error messags with user error",
		reason:      v1.PipelineRunReasonInvalidGraph.String(),
		messages:    makeMessages(hasUserError),
		expected:    "[User error] " + v1.PipelineRunReasonInvalidGraph.String(),
	}, {
		description: "error messags without user error",
		messages:    makeMessages(!hasUserError),
		reason:      v1.PipelineRunReasonInvalidGraph.String(),
		expected:    v1.PipelineRunReasonInvalidGraph.String(),
	}}
	for _, tc := range tcs {
		{
			reason := pipelineErrors.LabelsUserErrorReason(tc.reason, tc.messages)

			if reason != tc.expected {
				t.Errorf("failure reason expected: %s; but got %s", tc.expected, reason)
			}
		}
	}
}

func makeMessages(hasUserError bool) []interface{} {
	msgs := []string{"foo error message", "bar error format"}
	original := errors.New("orignal error")

	messages := make([]interface{}, 0)
	for _, msg := range msgs {
		messages = append(messages, msg)
	}

	if hasUserError {
		messages = append(messages, pipelineErrors.WrapUserError(original))
	}
	return messages
}
