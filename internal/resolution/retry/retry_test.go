/*
Copyright 2026 The Tekton Authors

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

package retry

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/util/wait"
)

var defaultBackoff = wait.Backoff{
	Duration: DefaultBackoffDuration,
	Factor:   DefaultBackoffFactor,
	Jitter:   DefaultBackoffJitter,
	Steps:    DefaultBackoffSteps,
	Cap:      DefaultBackoffCap,
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]string
		want   wait.Backoff
	}{{
		name:   "defaults",
		config: map[string]string{},
		want:   defaultBackoff,
	}, {
		name: "custom values",
		config: map[string]string{
			ConfigBackoffDuration: "7s",
			ConfigBackoffFactor:   "7",
			ConfigBackoffJitter:   "0.5",
			ConfigBackoffSteps:    "3",
			ConfigBackoffCap:      "20s",
		},
		want: wait.Backoff{Duration: 7 * time.Second, Factor: 7, Jitter: 0.5, Steps: 3, Cap: 20 * time.Second},
	}, {
		name: "unparseable values fall back to defaults",
		config: map[string]string{
			ConfigBackoffDuration: "not-a-duration",
			ConfigBackoffFactor:   "not-a-float",
			ConfigBackoffJitter:   "not-a-float",
			ConfigBackoffSteps:    "not-an-int",
			ConfigBackoffCap:      "not-a-duration",
		},
		want: defaultBackoff,
	}, {
		name: "negative values fall back to defaults",
		config: map[string]string{
			ConfigBackoffDuration: "-1s",
			ConfigBackoffFactor:   "-1.0",
			ConfigBackoffJitter:   "-0.5",
			ConfigBackoffSteps:    "-1",
			ConfigBackoffCap:      "-5s",
		},
		want: defaultBackoff,
	}, {
		name:   "zero steps falls back to default",
		config: map[string]string{ConfigBackoffSteps: "0"},
		want:   defaultBackoff,
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := framework.InjectResolverConfigToContext(t.Context(), tc.config)
			if d := cmp.Diff(tc.want, Backoff(ctx)); d != "" {
				t.Errorf("unexpected backoff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolve(t *testing.T) {
	fastBackoff := map[string]string{
		ConfigBackoffDuration: "1ms",
		ConfigBackoffFactor:   "1.0",
		ConfigBackoffJitter:   "0",
		ConfigBackoffCap:      "10ms",
	}
	tests := []struct {
		name      string
		steps     string
		failures  int
		wantCalls int
		wantErr   bool
	}{{
		name:      "first attempt succeeds",
		steps:     "3",
		wantCalls: 1,
	}, {
		name:      "succeeds after failed attempts",
		steps:     "3",
		failures:  2,
		wantCalls: 3,
	}, {
		name:      "returns the last error when all attempts fail",
		steps:     "2",
		failures:  5,
		wantCalls: 2,
		wantErr:   true,
	}, {
		name:      "one step disables retries",
		steps:     "1",
		failures:  1,
		wantCalls: 1,
		wantErr:   true,
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]string{ConfigBackoffSteps: tc.steps}
			maps.Copy(config, fastBackoff)
			ctx := framework.InjectResolverConfigToContext(t.Context(), config)

			calls := 0
			got, err := Resolve(ctx, func() (string, error) {
				calls++
				if calls <= tc.failures {
					return "", fmt.Errorf("attempt %d failed", calls)
				}
				return "resolved", nil
			})
			if calls != tc.wantCalls {
				t.Errorf("expected %d calls, got %d", tc.wantCalls, calls)
			}
			if tc.wantErr {
				wantErr := fmt.Sprintf("attempt %d failed", tc.wantCalls)
				if err == nil || err.Error() != wantErr {
					t.Fatalf("expected error %q, got %v", wantErr, err)
				}
				if got != "" {
					t.Errorf("expected zero value on error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != "resolved" {
				t.Errorf("expected %q, got %q", "resolved", got)
			}
		})
	}
}

func TestResolveContextCanceled(t *testing.T) {
	ctx := framework.InjectResolverConfigToContext(t.Context(), map[string]string{ConfigBackoffSteps: "3"})
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := Resolve(ctx, func() (string, error) {
		t.Fatal("fn should not be called with a canceled context")
		return "", nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestResolveContextCanceledDuringBackoff(t *testing.T) {
	ctx := framework.InjectResolverConfigToContext(t.Context(), map[string]string{
		ConfigBackoffDuration: "50ms",
		ConfigBackoffFactor:   "1.0",
		ConfigBackoffJitter:   "0",
		ConfigBackoffSteps:    "5",
		ConfigBackoffCap:      "1s",
	})
	ctx, cancel := context.WithCancel(ctx)

	calls := 0
	_, err := Resolve(ctx, func() (string, error) {
		calls++
		cancel()
		return "", errors.New("transient error")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}
