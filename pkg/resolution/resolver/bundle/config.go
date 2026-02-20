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

package bundle

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// ConfigServiceAccount is the configuration field name for controlling
	// the Service Account name to use for bundle requests.
	ConfigServiceAccount = "default-service-account"
	// ConfigKind is the configuration field name for controlling
	// what the layer name in the bundle image is.
	ConfigKind = "default-kind"
	// ConfigTimeoutKey is the configuration field name for controlling
	// the maximum duration of a resolution request for a file from registry.
	ConfigTimeoutKey = "fetch-timeout"
	// ConfigBackoffDuration is the configuration field name for controlling
	// the initial duration of a backoff when a bundle resolution fails
	ConfigBackoffDuration  = "backoff-duration"
	DefaultBackoffDuration = 2.0 * time.Second
	// ConfigBackoffFactor is the configuration field name for controlling
	// the factor by which successive backoffs will increase when a bundle
	// resolution fails
	ConfigBackoffFactor  = "backoff-factor"
	DefaultBackoffFactor = 2.0
	// ConfigBackoffJitter is the configuration field name for controlling
	// the randomness applied to backoff durations when a bundle resolution fails
	ConfigBackoffJitter  = "backoff-jitter"
	DefaultBackoffJitter = 0.1
	// ConfigBackoffSteps is the configuration field name for controlling
	// the number of attempted backoffs to retry when a bundle resolution fails
	ConfigBackoffSteps  = "backoff-steps"
	DefaultBackoffSteps = 2
	// ConfigBackoffCap is the configuration field name for controlling
	// the maximum duration to try when backing off
	ConfigBackoffCap  = "backoff-cap"
	DefaultBackoffCap = 10 * time.Second
)

// GetBundleResolverBackoff returns a remote.Backoff to
// be passed when resolving remote images. This can be configured with the
// backoff-duration, backoff-factor, backoff-jitter, backoff-steps, and backoff-cap
// fields in the bundle-resolver-config ConfigMap.
func GetBundleResolverBackoff(ctx context.Context) (remote.Backoff, error) {
	conf := framework.GetResolverConfigFromContext(ctx)

	customRetryBackoff := remote.Backoff{
		Duration: DefaultBackoffDuration,
		Factor:   DefaultBackoffFactor,
		Jitter:   DefaultBackoffJitter,
		Steps:    DefaultBackoffSteps,
		Cap:      DefaultBackoffCap,
	}
	if v, ok := conf[ConfigBackoffDuration]; ok {
		var err error
		duration, err := time.ParseDuration(v)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff duration value %s: %w", v, err)
		}
		customRetryBackoff.Duration = duration
	}
	if v, ok := conf[ConfigBackoffFactor]; ok {
		var err error
		factor, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff factor value %s: %w", v, err)
		}
		customRetryBackoff.Factor = factor
	}
	if v, ok := conf[ConfigBackoffJitter]; ok {
		var err error
		jitter, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff jitter value %s: %w", v, err)
		}
		customRetryBackoff.Jitter = jitter
	}
	if v, ok := conf[ConfigBackoffSteps]; ok {
		var err error
		steps, err := strconv.Atoi(v)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff steps value %s: %w", v, err)
		}
		customRetryBackoff.Steps = steps
	}
	if v, ok := conf[ConfigBackoffCap]; ok {
		var err error
		cap, err := time.ParseDuration(v)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff steps value %s: %w", v, err)
		}
		customRetryBackoff.Cap = cap
	}

	return customRetryBackoff, nil
}
