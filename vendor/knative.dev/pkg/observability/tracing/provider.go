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

package tracing

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func noopFunc(context.Context) error { return nil }

type TracerProvider struct {
	trace.TracerProvider
	shutdown func(context.Context) error
}

func (m *TracerProvider) Shutdown(ctx context.Context) error {
	return m.shutdown(ctx)
}

func DefaultTextMapPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func NewTracerProvider(
	ctx context.Context,
	cfg Config,
	opts ...sdktrace.TracerProviderOption,
) (*TracerProvider, error) {
	if cfg.Protocol == ProtocolNone {
		return &TracerProvider{
			TracerProvider: noop.NewTracerProvider(),
			shutdown:       noopFunc,
		}, nil
	}

	exp, err := exporterFor(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating tracer exporter: %w", err)
	}

	sampler, err := sampleFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating tracer sampler: %w", err)
	}

	opts = append(opts,
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sampler),
	)
	provider := sdktrace.NewTracerProvider(opts...)
	return &TracerProvider{
		TracerProvider: provider,
		shutdown:       provider.Shutdown,
	}, nil
}

func exporterFor(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	switch cfg.Protocol {
	case ProtocolGRPC:
		return buildGRPC(ctx, cfg)
	case ProtocolHTTPProtobuf:
		return buildHTTP(ctx, cfg)
	case ProtocolStdout:
		return buildStdout()
	default:
		return nil, fmt.Errorf("unsupported metric exporter: %q", cfg.Protocol)
	}
}

func buildStdout() (sdktrace.SpanExporter, error) {
	return stdouttrace.New()
}

func buildGRPC(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	var grpcOpts []otlptracegrpc.Option

	opt, err := endpointFor(cfg, otlptracegrpc.WithEndpointURL)
	if err != nil {
		return nil, fmt.Errorf("unable to process traces endpoint: %w", err)
	} else if opt != nil {
		grpcOpts = append(grpcOpts, opt)
	}

	exporter, err := otlptracegrpc.New(ctx, grpcOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build exporter: %w", err)
	}
	return exporter, nil
}

func buildHTTP(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	var httpOpts []otlptracehttp.Option

	opt, err := endpointFor(cfg, otlptracehttp.WithEndpointURL)
	if err != nil {
		return nil, fmt.Errorf("unable to process traces endpoint: %w", err)
	} else if opt != nil {
		httpOpts = append(httpOpts, opt)
	}

	exporter, err := otlptracehttp.New(ctx, httpOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build exporter: %w", err)
	}

	return exporter, nil
}

// If the OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT is
// set then we will prefer that over what's in the Config
func endpointFor[T any](cfg Config, opt func(string) T) (T, error) {
	var epOption T

	override := cmp.Or(
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
	)

	if override != "" {
		return epOption, nil
	}

	ep := cfg.Endpoint

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return epOption, err
	}
	if u.Opaque != "" {
		ep = "https://" + ep
	}

	epOption = opt(ep)
	return epOption, nil
}

func sampleFor(cfg Config) (sdktrace.Sampler, error) {
	// Don't override env arg
	if os.Getenv("OTEL_TRACES_SAMPLER") != "" {
		return nil, nil
	}

	if cfg.Protocol == ProtocolStdout {
		return sdktrace.AlwaysSample(), nil
	}

	rate := cfg.SamplingRate

	if val := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); val != "" {
		override, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse sample rate override: %w", err)
		}

		rate = override
	}

	if rate >= 1.0 {
		return sdktrace.AlwaysSample(), nil
	}

	if cfg.SamplingRate <= 0.0 {
		return sdktrace.NeverSample(), nil
	}

	root := sdktrace.TraceIDRatioBased(cfg.SamplingRate)
	return sdktrace.ParentBased(root), nil
}
