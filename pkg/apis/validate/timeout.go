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

package validate

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// Timeout validates a timeout field and returns the validated timeout with defaults applied.
// If timeout is nil, returns default timeout. If timeout is negative, returns an error.
func Timeout(timeout *metav1.Duration, defaultTimeoutMinutes int) (*metav1.Duration, *apis.FieldError) {
	if timeout == nil {
		return &metav1.Duration{Duration: time.Duration(defaultTimeoutMinutes) * time.Minute}, nil
	}
	if timeout.Duration < 0 {
		return nil, apis.ErrInvalidValue(timeout.Duration.String()+" should be >= 0", "timeout")
	}
	return timeout, nil
}
