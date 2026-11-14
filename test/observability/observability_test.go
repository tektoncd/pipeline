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

package observability_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"knative.dev/pkg/observability/metrics"
	"knative.dev/pkg/observability/resource"
	"knative.dev/pkg/observability/tracing"
)

// @test:execution=parallel
func TestDefaultResource(t *testing.T) {
	// Given the environment used by the controller processes.
	t.Setenv("SYSTEM_NAMESPACE", "tekton-pipelines")
	t.Setenv("POD_NAME", "test-controller")
	t.Setenv("OTEL_SERVICE_NAME", "")

	// When Knative combines its attributes with the SDK's default resource.
	r := resource.Default("test-controller")

	// Then startup succeeds and retains the configured service identity.
	value, ok := r.Set().Value(attribute.Key("service.name"))
	if !ok || value.AsString() != "test-controller" {
		t.Errorf("service.name = %v, want test-controller", value)
	}
}

// @test:execution=parallel
func TestTracingHTTPEndpointPaths(t *testing.T) {
	testHTTPEndpointPaths(t, "traces", func(ctx context.Context, endpoint string) error {
		provider, err := tracing.NewTracerProvider(ctx, tracing.Config{
			Protocol:     tracing.ProtocolHTTPProtobuf,
			Endpoint:     endpoint,
			SamplingRate: 1,
		})
		if err != nil {
			return err
		}
		_, span := provider.Tracer("endpoint-test").Start(ctx, "test-span")
		span.End()
		return provider.Shutdown(ctx)
	})
}

// @test:execution=parallel
func TestMetricsHTTPEndpointPaths(t *testing.T) {
	testHTTPEndpointPaths(t, "metrics", func(ctx context.Context, endpoint string) error {
		provider, err := metrics.NewMeterProvider(ctx, metrics.Config{
			Protocol:       metrics.ProtocolHTTPProtobuf,
			Endpoint:       endpoint,
			ExportInterval: time.Hour,
		})
		if err != nil {
			return err
		}
		counter, err := provider.Meter("endpoint-test").Int64Counter("test.counter")
		if err != nil {
			return errors.Join(err, provider.Shutdown(ctx))
		}
		counter.Add(ctx, 1)
		return provider.Shutdown(ctx)
	})
}

func testHTTPEndpointPaths(t *testing.T, signal string, export func(context.Context, string) error) {
	t.Helper()
	for _, tc := range []struct {
		name       string
		suffix     string
		envName    string
		envSuffix  string
		wantSuffix string
	}{
		{name: "pathless", wantSuffix: "/v1/" + signal},
		{name: "explicit path", suffix: "/custom", wantSuffix: "/custom"},
		{name: "explicit root", suffix: "/", wantSuffix: "/"},
		{name: "generic environment override", suffix: "/configured", envName: "OTEL_EXPORTER_OTLP_ENDPOINT", envSuffix: "/base", wantSuffix: "/base/v1/" + signal},
		{name: "signal environment override", suffix: "/configured", envName: "OTEL_EXPORTER_OTLP_" + strings.ToUpper(signal) + "_ENDPOINT", wantSuffix: "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given a collector subscribed before any telemetry is exported.
			requests := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.URL.RequestURI()
				w.Header().Set("Content-Type", "application/x-protobuf")
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)
			for _, name := range []string{
				"OTEL_EXPORTER_OTLP_ENDPOINT",
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
				"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
				"OTEL_TRACES_SAMPLER",
				"OTEL_TRACES_SAMPLER_ARG",
				"OTEL_METRIC_EXPORT_INTERVAL",
			} {
				t.Setenv(name, "")
			}
			if tc.envName != "" {
				t.Setenv(tc.envName, server.URL+tc.envSuffix)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			// When shutdown flushes a real span or metric through Knative.
			if err := export(ctx, server.URL+tc.suffix); err != nil {
				t.Fatal(err)
			}

			// Then the request preserves configured paths and environment precedence.
			select {
			case got := <-requests:
				if got != tc.wantSuffix {
					t.Errorf("request URI = %q, want %q", got, tc.wantSuffix)
				}
			case <-ctx.Done():
				t.Fatalf("collector received no request: %v", ctx.Err())
			}
		})
	}
}
