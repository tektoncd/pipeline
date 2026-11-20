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

// Package retry retries failed resolution attempts with an exponential
// backoff that is configured in the resolver's ConfigMap.
package retry

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/logging"
)

const (
	// ConfigBackoffDuration is the configuration field name for controlling
	// the initial duration of a backoff when a resolution attempt fails.
	ConfigBackoffDuration = "backoff-duration"
	// DefaultBackoffDuration is used when ConfigBackoffDuration is not set.
	DefaultBackoffDuration = 2 * time.Second
	// ConfigBackoffFactor is the configuration field name for controlling
	// the factor by which successive backoffs increase when a resolution
	// attempt fails.
	ConfigBackoffFactor = "backoff-factor"
	// DefaultBackoffFactor is used when ConfigBackoffFactor is not set.
	DefaultBackoffFactor = 2.0
	// ConfigBackoffJitter is the configuration field name for controlling
	// the randomness applied to backoff durations when a resolution attempt
	// fails.
	ConfigBackoffJitter = "backoff-jitter"
	// DefaultBackoffJitter is used when ConfigBackoffJitter is not set.
	DefaultBackoffJitter = 0.1
	// ConfigBackoffSteps is the configuration field name for controlling
	// the total number of resolution attempts.
	ConfigBackoffSteps = "backoff-steps"
	// DefaultBackoffSteps is used when ConfigBackoffSteps is not set.
	DefaultBackoffSteps = 2
	// ConfigBackoffCap is the configuration field name for controlling
	// the maximum duration to try when backing off.
	ConfigBackoffCap = "backoff-cap"
	// DefaultBackoffCap is used when ConfigBackoffCap is not set.
	DefaultBackoffCap = 10 * time.Second
)

// Backoff returns the wait.Backoff to use when retrying a failed resolution
// attempt. It is configured with the backoff-duration, backoff-factor,
// backoff-jitter, backoff-steps and backoff-cap fields of the resolver's
// ConfigMap. Invalid values are logged as warnings and replaced with their
// defaults so that retry behavior is preserved.
func Backoff(ctx context.Context) wait.Backoff {
	logger := logging.FromContext(ctx)
	conf := framework.GetResolverConfigFromContext(ctx)

	backoff := wait.Backoff{
		Duration: DefaultBackoffDuration,
		Factor:   DefaultBackoffFactor,
		Jitter:   DefaultBackoffJitter,
		Steps:    DefaultBackoffSteps,
		Cap:      DefaultBackoffCap,
	}
	if v, ok := conf[ConfigBackoffDuration]; ok {
		duration, err := time.ParseDuration(v)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffDuration, v, DefaultBackoffDuration, err)
		case duration < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffDuration, v, DefaultBackoffDuration)
		default:
			backoff.Duration = duration
		}
	}
	if v, ok := conf[ConfigBackoffFactor]; ok {
		factor, err := strconv.ParseFloat(v, 64)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffFactor, v, DefaultBackoffFactor, err)
		case factor < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffFactor, v, DefaultBackoffFactor)
		default:
			backoff.Factor = factor
		}
	}
	if v, ok := conf[ConfigBackoffJitter]; ok {
		jitter, err := strconv.ParseFloat(v, 64)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffJitter, v, DefaultBackoffJitter, err)
		case jitter < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffJitter, v, DefaultBackoffJitter)
		default:
			backoff.Jitter = jitter
		}
	}
	if v, ok := conf[ConfigBackoffSteps]; ok {
		steps, err := strconv.Atoi(v)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffSteps, v, DefaultBackoffSteps, err)
		case steps < 1:
			logger.Warnf("invalid %s value %q: must be at least 1, using default %v", ConfigBackoffSteps, v, DefaultBackoffSteps)
		default:
			backoff.Steps = steps
		}
	}
	if v, ok := conf[ConfigBackoffCap]; ok {
		backoffCap, err := time.ParseDuration(v)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffCap, v, DefaultBackoffCap, err)
		case backoffCap < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffCap, v, DefaultBackoffCap)
		default:
			backoff.Cap = backoffCap
		}
	}

	return backoff
}

// Resolve calls fn until it succeeds or the Backoff from the resolver's
// ConfigMap runs out of steps. If every attempt fails, the error from the
// last attempt is returned. If the context is canceled or its deadline
// passes, the context error is returned.
func Resolve[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero, result T
	var lastErr error
	retryErr := wait.ExponentialBackoffWithContext(ctx, Backoff(ctx), func(context.Context) (bool, error) {
		result, lastErr = fn()
		return lastErr == nil, nil
	})
	if retryErr != nil {
		if errors.Is(retryErr, context.Canceled) || errors.Is(retryErr, context.DeadlineExceeded) {
			return zero, retryErr
		}
		if lastErr != nil {
			return zero, lastErr
		}
		return zero, retryErr
	}
	return result, nil
}
