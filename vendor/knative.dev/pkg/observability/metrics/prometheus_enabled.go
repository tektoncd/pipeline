//go:build !noprometheus

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
	"context"
	"fmt"
	"net"

	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"knative.dev/pkg/observability/metrics/prometheus"
)

func buildPrometheus(_ context.Context, cfg Config) (sdkmetric.Reader, shutdownFunc, error) {
	r, err := otelprom.New()
	if err != nil {
		return nil, noopFunc, fmt.Errorf("unable to create otel prometheus exporter: %w", err)
	}

	var opts []prometheus.ServerOption

	if cfg.Endpoint != "" {
		host, port, err := net.SplitHostPort(cfg.Endpoint)
		if err != nil {
			return nil, noopFunc, fmt.Errorf(
				`metrics-endpoint %q for prometheus exporter should be "host:port", "host%%zone:port", "[host]:port" or "[host%%zone]:port"`,
				cfg.Endpoint,
			)
		}

		opts = append(opts,
			prometheus.WithHost(host),
			prometheus.WithPort(port),
		)
	}

	server, err := prometheus.NewServer(opts...)

	go func() {
		server.ListenAndServe()
	}()

	return r, server.Shutdown, err
}
