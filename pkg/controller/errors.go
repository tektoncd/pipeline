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

// Package controller provides helper methods for external controllers for
// Custom Task types.
package controller

import (
	"errors"
	"net/http"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// IsWebhookTimeout checks if the error is due to a mutating admission webhook timeout.
// This function is used to determine if an error should trigger exponential backoff retry logic.
func IsWebhookTimeout(err error) bool {
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		return statusErr.ErrStatus.Code == http.StatusInternalServerError &&
			strings.Contains(statusErr.ErrStatus.Message, "timeout")
	}
	return false
}
