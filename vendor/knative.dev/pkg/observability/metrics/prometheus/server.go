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

package prometheus

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultPrometheusPort            = "9090"
	defaultPrometheusReportingPeriod = 5
	maxPrometheusPort                = 65535
	minPrometheusPort                = 1024
	defaultPrometheusHost            = "" // IPv4 and IPv6
	prometheusPortEnvName            = "METRICS_PROMETHEUS_PORT"
	prometheusHostEnvName            = "METRICS_PROMETHEUS_HOST"
)

type ServerOption func(*options)

type Server struct {
	http *http.Server
}

func NewServer(opts ...ServerOption) (*Server, error) {
	o := options{
		host: defaultPrometheusHost,
		port: defaultPrometheusPort,
	}

	for _, opt := range opts {
		opt(&o)
	}

	envOverride(&o.host, prometheusHostEnvName)
	envOverride(&o.port, prometheusPortEnvName)

	if err := validate(&o); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	addr := net.JoinHostPort(o.host, o.port)

	return &Server{
		http: &http.Server{
			Addr:    addr,
			Handler: mux,
			// https://medium.com/a-journey-with-go/go-understand-and-mitigate-slowloris-attack-711c1b1403f6
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

func (s *Server) ListenAndServe() {
	s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

type options struct {
	host string
	port string
}

func WithHost(host string) ServerOption {
	return func(o *options) {
		o.host = host
	}
}

func WithPort(port string) ServerOption {
	return func(o *options) {
		o.port = port
	}
}

func validate(o *options) error {
	port, err := strconv.ParseUint(o.port, 10, 16)
	if err != nil {
		return fmt.Errorf("prometheus port %q could not be parsed as a port number: %w",
			o.port, err)
	}

	if port < minPrometheusPort || port > maxPrometheusPort {
		return fmt.Errorf("prometheus port %d, should be between %d and %d",
			port, minPrometheusPort, maxPrometheusPort)
	}

	return nil
}

func envOverride(target *string, envName string) {
	val := os.Getenv(envName)
	if val != "" {
		*target = val
	}
}
