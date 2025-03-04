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
	ConfigBackoffDuration = "backoff-duration"
	DefaultBackoffDuration = 1.0 * time.Second
	// ConfigBackoffFactor is the configuration field name for controlling
	// the factor by which successive backoffs will increase when a bundle
	// resolution fails
	ConfigBackoffFactor = "backoff-factor"
	DefaultBackoffFactor = 3.0
	// ConfigBackoffJitter is the configuration field name for controlling
	// the randomness applied to backoff durations when a bundle resolution fails
	ConfigBackoffJitter = "backoff-jitter"
	DefaultBackoffJitter = 0.1
	// ConfigBackoffSteps is the configuration field name for controlling
	// the number of attempted backoffs to retry when a bundle resolution fails
	ConfigBackoffSteps = "backoff-steps"
	DefaultBackoffSteps = 3
)

type BundleResolverBackoffConfig struct {
	Duration: time.Duration
	Factor:	  float64
	Jitter:   float64
	Steps:	  int

}

func  {
	conf := framework.GetResolverConfigFromContext(ctx)
}

// GetResolutionTimeout returns a time.Duration for the amount of time a
// single bundle fetch may take. This can be configured with the
// fetch-timeout field in the bundle-resolver-config ConfigMap.
func GetBundleResolverBackoff(ctx context.Context) (BundleResolverBackoffConfig, error) {
	conf := framework.GetResolverConfigFromContext(ctx)

	customRetryBackoff := BundleResolverBackoffConfig{
		Duration: DefaultBackoffDuration,
		Factor:   DefaultBackoffFactor,
		Jitter:   DefaultBackoffJitter,
		Steps:    DefaultBackoffSteps,
	}
	if v, ok := conf[ConfigBackoffDuration]; ok {
		var err error
		duration, err = time.ParseDuration(v)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff duration value %s: %w", v, err)
		}
	}
	if v, ok := conf[ConfigBackoffFactor]; ok {
		var err error
		factor, err = strconv.ParseFloat(string(v, 64)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff factor value %s: %w", v, err)
		}
	}
	if v, ok := conf[ConfigBackoffJitter]; ok {
		var err error
		factor, err = strconv.ParseFloat(string(v, 64)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff jitter value %s: %w", v, err)
		}
	}
	if v, ok := conf[ConfigBackoffSteps]; ok {
		var err error
		factor, err = strconv.ParseInt(string(v)
		if err != nil {
			return customRetryBackoff, fmt.Errorf("error parsing backoff steps value %s: %w", v, err)
		}
	}

	return customRetryBackoff, nil
}