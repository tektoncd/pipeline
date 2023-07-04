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

package tracing

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type tracerProvider struct {
	service  string
	provider trace.TracerProvider
	cfg      *config.Tracing
}

func init() {
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

// New returns a new instance of tracerProvider for the given service
func New(service string) *tracerProvider {
	return &tracerProvider{
		service:  service,
		provider: trace.NewNoopTracerProvider(),
	}
}

// OnStore configures tracerProvider dynamically
func (t *tracerProvider) OnStore(logger *zap.SugaredLogger) func(name string, value interface{}) {
	return func(name string, value interface{}) {
		if name == config.GetTracingConfigName() {
			cfg, ok := value.(*config.Tracing)
			if !ok {
				logger.Error("Failed to do type assertion for extracting TRACING config")
				return
			}

			if cfg.Equals(t.cfg) {
				logger.Info("Tracing config unchanged", cfg, t.cfg)
				return
			}
			t.cfg = cfg

			tp, err := createTracerProvider(t.service, cfg)
			if err != nil {
				logger.Errorf("Unable to initialize tracing with error : %v", err.Error())
				return
			}
			logger.Info("Initialized Tracer Provider")
			if p, ok := t.provider.(*tracesdk.TracerProvider); ok {
				if err := p.Shutdown(context.Background()); err != nil {
					logger.Errorf("Unable to shutdown tracingprovider with error : %v", err.Error())
				}
			}
			t.provider = tp
		}
	}
}

func (t *tracerProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	return t.provider.Tracer(name, options...)
}

func createTracerProvider(service string, cfg *config.Tracing) (trace.TracerProvider, error) {
	if !cfg.Enabled {
		return trace.NewNoopTracerProvider(), nil
	}

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint(cfg.Endpoint),
	))
	if err != nil {
		return nil, err
	}
	// Initialize tracerProvider with the jaeger exporter
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		// Record information about the service in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
		)),
	)
	return tp, nil
}
