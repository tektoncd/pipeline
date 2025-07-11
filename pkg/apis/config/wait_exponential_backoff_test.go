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

package config_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewWaitExponentialBackoffFromConfigMap(t *testing.T) {
	for _, tc := range []struct {
		name     string
		want     *config.WaitExponentialBackoff
		fileName string
	}{
		{
			name: "empty",
			want: &config.WaitExponentialBackoff{
				Duration: mustParseDuration(t, config.DefaultWaitExponentialBackoffDuration),
				Factor:   config.DefaultWaitExponentialBackoffFactor,
				Jitter:   config.DefaultWaitExponentialBackoffJitter,
				Steps:    config.DefaultWaitExponentialBackoffSteps,
				Cap:      mustParseDuration(t, config.DefaultWaitExponentialBackoffCap),
			},
			fileName: "config-wait-exponential-backoff-empty",
		},
		{
			name: "custom values",
			want: &config.WaitExponentialBackoff{
				Duration: 5 * time.Second,
				Factor:   3.5,
				Jitter:   0.2,
				Steps:    7,
				Cap:      120 * time.Second,
			},
			fileName: "config-wait-exponential-backoff-custom",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm := test.ConfigMapFromTestFile(t, tc.fileName)
			if got, err := config.NewWaitExponentialBackoffFromConfigMap(cm); err == nil {
				if d := cmp.Diff(tc.want, got); d != "" {
					t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
				}
			} else {
				t.Errorf("NewWaitExponentialBackoffFromConfigMap(actual) = %v", err)
			}
		})
	}
}

func TestWaitExponentialBackoffEquals(t *testing.T) {
	testCases := []struct {
		name     string
		left     *config.WaitExponentialBackoff
		right    *config.WaitExponentialBackoff
		expected bool
	}{
		{
			name:     "left and right nil",
			left:     nil,
			right:    nil,
			expected: true,
		},
		{
			name:     "left nil",
			left:     nil,
			right:    &config.WaitExponentialBackoff{},
			expected: false,
		},
		{
			name:     "right nil",
			left:     &config.WaitExponentialBackoff{},
			right:    nil,
			expected: false,
		},
		{
			name:     "right and right default",
			left:     &config.WaitExponentialBackoff{},
			right:    &config.WaitExponentialBackoff{},
			expected: true,
		},
		{
			name: "different duration",
			left: &config.WaitExponentialBackoff{
				Duration: 1 * time.Second,
			},
			right: &config.WaitExponentialBackoff{
				Duration: 2 * time.Second,
			},
			expected: false,
		},
		{
			name: "different factor",
			left: &config.WaitExponentialBackoff{
				Factor: 2.0,
			},
			right: &config.WaitExponentialBackoff{
				Factor: 3.0,
			},
			expected: false,
		},
		{
			name: "different jitter",
			left: &config.WaitExponentialBackoff{
				Jitter: 0.1,
			},
			right: &config.WaitExponentialBackoff{
				Jitter: 0.2,
			},
			expected: false,
		},
		{
			name: "different steps",
			left: &config.WaitExponentialBackoff{
				Steps: 5,
			},
			right: &config.WaitExponentialBackoff{
				Steps: 6,
			},
			expected: false,
		},
		{
			name: "different cap",
			left: &config.WaitExponentialBackoff{
				Cap: 10 * time.Second,
			},
			right: &config.WaitExponentialBackoff{
				Cap: 20 * time.Second,
			},
			expected: false,
		},
		{
			name: "same all fields",
			left: &config.WaitExponentialBackoff{
				Duration: 5 * time.Second,
				Factor:   3.5,
				Jitter:   0.2,
				Steps:    7,
				Cap:      120 * time.Second,
			},
			right: &config.WaitExponentialBackoff{
				Duration: 5 * time.Second,
				Factor:   3.5,
				Jitter:   0.2,
				Steps:    7,
				Cap:      120 * time.Second,
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.left.Equals(tc.right)
			if actual != tc.expected {
				t.Errorf("Comparison failed expected: %t, actual: %t", tc.expected, actual)
			}
		})
	}
}

func mustParseDuration(t *testing.T, s string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatalf("failed to parse duration %q: %v", s, err)
	}
	return d
}
