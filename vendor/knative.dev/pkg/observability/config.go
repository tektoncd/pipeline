/*
Copyright 2025 The Knative Authors

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

package observability

import (
	"context"

	"knative.dev/pkg/observability/metrics"
	"knative.dev/pkg/observability/runtime"
	"knative.dev/pkg/observability/tracing"
)

type (
	TracingConfig = tracing.Config
	MetricsConfig = metrics.Config
	RuntimeConfig = runtime.Config
)

type Config struct {
	Tracing TracingConfig `json:"tracing"`
	Metrics MetricsConfig `json:"metrics"`
	Runtime RuntimeConfig `json:"runtime"`
}

func DefaultConfig() *Config {
	return &Config{
		Tracing: tracing.DefaultConfig(),
		Metrics: metrics.DefaultConfig(),
		Runtime: runtime.DefaultConfig(),
	}
}

func NewFromMap(m map[string]string) (*Config, error) {
	var err error
	c := DefaultConfig()

	if c.Tracing, err = tracing.NewFromMap(m); err != nil {
		return nil, err
	}

	if c.Metrics, err = metrics.NewFromMap(m); err != nil {
		return nil, err
	}
	if c.Runtime, err = runtime.NewFromMap(m); err != nil {
		return nil, err
	}

	return c, nil
}

type cfgKey struct{}

// WithConfig associates a observability configuration with the context.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, cfgKey{}, cfg)
}

// GetConfig gets the observability config from the provided context.
func GetConfig(ctx context.Context) *Config {
	untyped := ctx.Value(cfgKey{})
	if untyped == nil {
		return nil
	}
	return untyped.(*Config)
}
