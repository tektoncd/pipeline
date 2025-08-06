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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTimeout(t *testing.T) {
	tests := []struct {
		name                  string
		timeout               *metav1.Duration
		defaultTimeoutMinutes int
		expectError           bool
		expectedDuration      time.Duration
	}{
		{
			name:                  "nil timeout returns default",
			timeout:               nil,
			defaultTimeoutMinutes: 60,
			expectError:           false,
			expectedDuration:      60 * time.Minute,
		},
		{
			name:                  "positive timeout is valid",
			timeout:               &metav1.Duration{Duration: 30 * time.Minute},
			defaultTimeoutMinutes: 60,
			expectError:           false,
			expectedDuration:      30 * time.Minute,
		},
		{
			name:                  "zero timeout is valid",
			timeout:               &metav1.Duration{Duration: 0},
			defaultTimeoutMinutes: 60,
			expectError:           false,
			expectedDuration:      0,
		},
		{
			name:                  "negative timeout returns error",
			timeout:               &metav1.Duration{Duration: -10 * time.Minute},
			defaultTimeoutMinutes: 60,
			expectError:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Timeout(tt.timeout, tt.defaultTimeoutMinutes)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && result.Duration != tt.expectedDuration {
				t.Errorf("expected duration %v, got %v", tt.expectedDuration, result.Duration)
			}
		})
	}
}
