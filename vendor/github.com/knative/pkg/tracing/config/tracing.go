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
	"errors"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	// ConfigName is the name of the configmap
	ConfigName = "config-tracing"

	enableKey         = "enable"
	zipkinEndpointKey = "zipkin-endpoint"
	debugKey          = "debug"
	sampleRateKey     = "sample-rate"
)

// Config holds the configuration for tracers
type Config struct {
	Enable         bool
	ZipkinEndpoint string
	Debug          bool
	SampleRate     float64
}

// Equals returns true if two Configs are identical
func (cfg *Config) Equals(other *Config) bool {
	return other.Enable == cfg.Enable && other.ZipkinEndpoint == cfg.ZipkinEndpoint && other.Debug == cfg.Debug && other.SampleRate == cfg.SampleRate
}

// NewTracingConfigFromMap returns a Config given a map corresponding to a ConfigMap
func NewTracingConfigFromMap(cfgMap map[string]string) (*Config, error) {
	tc := Config{
		Enable:     false,
		Debug:      false,
		SampleRate: 0.1,
	}
	if enable, ok := cfgMap[enableKey]; ok {
		enableBool, err := strconv.ParseBool(enable)
		if err != nil {
			return nil, fmt.Errorf("Failed parsing tracing config %q: %v", enableKey, err)
		}
		tc.Enable = enableBool
	}

	if endpoint, ok := cfgMap[zipkinEndpointKey]; !ok {
		if tc.Enable {
			return nil, errors.New("Tracing enabled but no zipkin endpoint specified")
		}
	} else {
		tc.ZipkinEndpoint = endpoint
	}

	if debug, ok := cfgMap[debugKey]; ok {
		debugBool, err := strconv.ParseBool(debug)
		if err != nil {
			return nil, fmt.Errorf("Failed parsing tracing config %q", debugKey)
		}
		tc.Debug = debugBool
	}

	if sampleRate, ok := cfgMap[sampleRateKey]; ok {
		sampleRateFloat, err := strconv.ParseFloat(sampleRate, 64)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse sampleRate in tracing config: %v", err)
		}
		tc.SampleRate = sampleRateFloat
	}

	return &tc, nil
}

// NewTracingConfigFromConfigMap returns a Config for the given configmap
func NewTracingConfigFromConfigMap(config *corev1.ConfigMap) (*Config, error) {
	return NewTracingConfigFromMap(config.Data)
}
