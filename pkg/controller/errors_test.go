/*
Copyright 2025 The Tekton Authors

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

package controller

import (
	"errors"
	"net/http"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsWebhookTimeout(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "non-status error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name: "webhook timeout error - exact match",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: "timeout",
				},
			},
			expected: true,
		},
		{
			name: "webhook timeout error - contains timeout in message",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: "admission webhook timeout occurred",
				},
			},
			expected: true,
		},
		{
			name: "webhook timeout error - case insensitive",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: "TIMEOUT",
				},
			},
			expected: false, // Case-sensitive matching, so "TIMEOUT" should not match "timeout"
		},
		{
			name: "non-webhook error - different status code",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusBadRequest,
					Message: "timeout",
				},
			},
			expected: false,
		},
		{
			name: "non-webhook error - different message",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: "server error",
				},
			},
			expected: false,
		},
		{
			name: "non-webhook error - forbidden",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusForbidden,
					Message: "forbidden",
				},
			},
			expected: false,
		},
		{
			name: "non-webhook error - conflict",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusConflict,
					Message: "conflict",
				},
			},
			expected: false,
		},
		{
			name: "non-webhook error - not found",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusNotFound,
					Message: "not found",
				},
			},
			expected: false,
		},
		{
			name: "webhook timeout error - with additional context",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: "failed calling webhook: timeout",
				},
			},
			expected: true,
		},
		{
			name: "webhook timeout error - with error details",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: "admission webhook \"example.com\" failed: timeout",
				},
			},
			expected: true,
		},
		{
			name: "non-webhook error - timeout in different context",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:    http.StatusInternalServerError,
					Message: "operation completed successfully, no timeout",
				},
			},
			expected: true, // Contains "timeout" substring, so it should match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWebhookTimeout(tt.err)
			if result != tt.expected {
				t.Errorf("IsWebhookTimeout() = %v, want %v", result, tt.expected)
			}
		})
	}
}
