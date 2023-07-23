/*
Copyright 2023 The Tekton Authors

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
	"fmt"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	// tracingEnabledKey is the configmap key which determines if tracing is enabled
	tracingEnabledKey = "enabled"
	// tracingEndpintKey is the configmap key for tracing api endpoint
	tracingEndpointKey = "endpoint"

	// tracingCredentialsSecretKey is the name of the secret which contains credentials for tracing endpoint
	tracingCredentialsSecretKey = "credentialsSecret"

	// DefaultEndpoint is the default destination for sending traces
	DefaultEndpoint = "http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces"
)

// DefaultTracing holds all the default configurations for tracing
var DefaultTracing, _ = newTracingFromMap(map[string]string{})

// Tracing holds the configurations for tracing
// +k8s:deepcopy-gen=true
type Tracing struct {
	Enabled           bool
	Endpoint          string
	CredentialsSecret string
}

// Equals returns true if two Configs are identical
func (cfg *Tracing) Equals(other *Tracing) bool {
	if cfg == nil && other == nil {
		return true
	}

	if cfg == nil || other == nil {
		return false
	}

	return other.Enabled == cfg.Enabled &&
		other.Endpoint == cfg.Endpoint &&
		other.CredentialsSecret == cfg.CredentialsSecret
}

// GetTracingConfigName returns the name of the configmap containing all
// customizations for tracing
func GetTracingConfigName() string {
	if e := os.Getenv("CONFIG_TRACING_NAME"); e != "" {
		return e
	}
	return "config-tracing"
}

// newTracingFromMap returns a Config given a map from ConfigMap
func newTracingFromMap(config map[string]string) (*Tracing, error) {
	t := Tracing{
		Enabled:  false,
		Endpoint: DefaultEndpoint,
	}

	if endpoint, ok := config[tracingEndpointKey]; ok {
		t.Endpoint = endpoint
	}

	if secret, ok := config[tracingCredentialsSecretKey]; ok {
		t.CredentialsSecret = secret
	}

	if enabled, ok := config[tracingEnabledKey]; ok {
		e, err := strconv.ParseBool(enabled)
		if err != nil {
			return nil, fmt.Errorf("failed parsing tracing config %q: %w", enabled, err)
		}
		t.Enabled = e
	}
	return &t, nil
}

// NewTracingFromConfigMap returns a Config given a ConfigMap
func NewTracingFromConfigMap(config *corev1.ConfigMap) (*Tracing, error) {
	return newTracingFromMap(config.Data)
}
