/*
Copyright 2019 The Knative Authors.

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

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	cm "knative.dev/pkg/configmap"
)

const (
	// ConfigName is the name of the configmap
	ConfigName = "config-tracing"

	enableKey         = "enable"
	backendKey        = "backend"
	zipkinEndpointKey = "zipkin-endpoint"
	debugKey          = "debug"
	sampleRateKey     = "sample-rate"
)

// BackendType specifies the backend to use for tracing
type BackendType string

const (
	// None is used for no backend.
	None BackendType = "none"

	// Zipkin is used for Zipkin backend.
	Zipkin BackendType = "zipkin"
)

// Config holds the configuration for tracers
type Config struct {
	Backend        BackendType
	ZipkinEndpoint string

	Debug      bool
	SampleRate float64
}

// Equals returns true if two Configs are identical
func (cfg *Config) Equals(other *Config) bool {
	return reflect.DeepEqual(other, cfg)
}

// NoopConfig returns a new noop config
func NoopConfig() *Config {
	return &Config{
		Backend:    None,
		Debug:      false,
		SampleRate: 0.1,
	}
}

// NewTracingConfigFromMap returns a Config given a map corresponding to a ConfigMap
func NewTracingConfigFromMap(cfgMap map[string]string) (*Config, error) {
	tc := NoopConfig()

	if backend, ok := cfgMap[backendKey]; ok {
		switch bt := BackendType(backend); bt {
		case Zipkin, None:
			tc.Backend = bt
		default:
			return nil, fmt.Errorf("unsupported tracing backend value %q", backend)
		}
	} else if enable, ok := cfgMap[enableKey]; ok {
		// For backwards compatibility, parse the enabled flag as Zipkin.
		enableBool, err := strconv.ParseBool(enable)
		if err != nil {
			return nil, fmt.Errorf("failed parsing tracing config %q: %w", enableKey, err)
		}
		if enableBool {
			tc.Backend = Zipkin
		}
	}

	if err := cm.Parse(cfgMap,
		cm.AsString(zipkinEndpointKey, &tc.ZipkinEndpoint),
		cm.AsBool(debugKey, &tc.Debug),
		cm.AsFloat64(sampleRateKey, &tc.SampleRate),
	); err != nil {
		return nil, err
	}

	if tc.Backend == Zipkin && tc.ZipkinEndpoint == "" {
		return nil, errors.New("zipkin tracing enabled without a zipkin endpoint specified")
	}

	if tc.SampleRate < 0 || tc.SampleRate > 1 {
		return nil, fmt.Errorf("sample-rate = %v must be in [0, 1] range", tc.SampleRate)
	}

	return tc, nil
}

// NewTracingConfigFromConfigMap returns a Config for the given configmap
func NewTracingConfigFromConfigMap(config *corev1.ConfigMap) (*Config, error) {
	if config == nil {
		return NewTracingConfigFromMap(nil)
	}
	return NewTracingConfigFromMap(config.Data)
}

// JSONToTracingConfig converts a json string of a Config.
// Returns a non-nil Config always and an eventual error.
func JSONToTracingConfig(jsonCfg string) (*Config, error) {
	if jsonCfg == "" {
		return NoopConfig(), errors.New("empty json tracing config")
	}

	var configMap map[string]string
	if err := json.Unmarshal([]byte(jsonCfg), &configMap); err != nil {
		return NoopConfig(), err
	}

	cfg, err := NewTracingConfigFromMap(configMap)
	if err != nil {
		return NoopConfig(), nil
	}
	return cfg, nil
}

func TracingConfigToJSON(cfg *Config) (string, error) { //nolint // for backcompat.
	if cfg == nil {
		return "", nil
	}

	out := make(map[string]string, 5)
	out[backendKey] = string(cfg.Backend)
	if cfg.ZipkinEndpoint != "" {
		out[zipkinEndpointKey] = cfg.ZipkinEndpoint
	}
	out[debugKey] = fmt.Sprint(cfg.Debug)
	out[sampleRateKey] = fmt.Sprint(cfg.SampleRate)

	jsonCfg, err := json.Marshal(out)
	return string(jsonCfg), err
}
