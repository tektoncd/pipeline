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

package metrics

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type shutdownFunc func(ctx context.Context) error

func noopFunc(context.Context) error { return nil }

type MeterProvider struct {
	metric.MeterProvider
	shutdown []shutdownFunc
}

func (m *MeterProvider) Shutdown(ctx context.Context) error {
	var errs []error
	for _, shutdown := range m.shutdown {
		if err := shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func NewMeterProvider(
	ctx context.Context,
	cfg Config,
	opts ...sdkmetric.Option,
) (*MeterProvider, error) {
	if cfg.Protocol == ProtocolNone {
		return &MeterProvider{MeterProvider: noop.NewMeterProvider()}, nil
	}

	reader, rShutdown, err := readerFor(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating reader: %w", err)
	}

	opts = append(opts, sdkmetric.WithReader(reader))
	provider := sdkmetric.NewMeterProvider(opts...)

	return &MeterProvider{
		MeterProvider: provider,
		shutdown:      []shutdownFunc{provider.Shutdown, rShutdown},
	}, nil
}

func readerFor(ctx context.Context, cfg Config) (sdkmetric.Reader, shutdownFunc, error) {
	switch cfg.Protocol {
	case ProtocolGRPC:
		return buildGRPC(ctx, cfg)
	case ProtocolHTTPProtobuf:
		return buildHTTP(ctx, cfg)
	case ProtocolPrometheus:
		return buildPrometheus(ctx, cfg)
	default:
		return nil, noopFunc, fmt.Errorf("unsupported metric exporter: %q", cfg.Protocol)
	}
}

func buildGRPC(ctx context.Context, cfg Config) (sdkmetric.Reader, shutdownFunc, error) {
	var grpcOpts []otlpmetricgrpc.Option

	opt, err := endpointFor(cfg, otlpmetricgrpc.WithEndpointURL)
	if err != nil {
		return nil, noopFunc, fmt.Errorf("unable to process metrics endpoint: %w", err)
	} else if opt != nil {
		grpcOpts = append(grpcOpts, opt)
	}

	exporter, err := otlpmetricgrpc.New(ctx, grpcOpts...)
	if err != nil {
		return nil, noopFunc, fmt.Errorf("failed to build exporter: %w", err)
	}

	var opts []sdkmetric.PeriodicReaderOption

	if interval := intervalFor(cfg); interval != nil {
		opts = append(opts, interval)
	}

	return sdkmetric.NewPeriodicReader(exporter, opts...), noopFunc, nil
}

func buildHTTP(ctx context.Context, cfg Config) (sdkmetric.Reader, shutdownFunc, error) {
	var httpOpts []otlpmetrichttp.Option

	opt, err := endpointFor(cfg, otlpmetrichttp.WithEndpointURL)
	if err != nil {
		return nil, noopFunc, fmt.Errorf("unable to process metrics endpoint: %w", err)
	} else if opt != nil {
		httpOpts = append(httpOpts, opt)
	}

	exporter, err := otlpmetrichttp.New(ctx, httpOpts...)
	if err != nil {
		return nil, noopFunc, fmt.Errorf("failed to build exporter: %w", err)
	}

	var opts []sdkmetric.PeriodicReaderOption
	if opt := intervalFor(cfg); opt != nil {
		opts = append(opts, opt)
	}

	return sdkmetric.NewPeriodicReader(exporter, opts...), noopFunc, nil
}

func intervalFor(cfg Config) sdkmetric.PeriodicReaderOption {
	var interval sdkmetric.PeriodicReaderOption

	// Use the configuration's export interval if it's non-zero and there isn't
	// an environment override
	if os.Getenv("OTEL_METRIC_EXPORT_INTERVAL") == "" && cfg.ExportInterval != 0 {
		interval = sdkmetric.WithInterval(cfg.ExportInterval)
	}

	return interval
}

// If the OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
func endpointFor[T any](cfg Config, opt func(string) T) (T, error) {
	var epOption T

	override := cmp.Or(
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"),
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
