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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	knativetls "knative.dev/pkg/network/tls"
)

const (
	defaultPrometheusPort            = "9090"
	maxPrometheusPort                = 65535
	minPrometheusPort                = 1024
	defaultPrometheusHost            = "" // IPv4 and IPv6
	prometheusPortEnvName            = "METRICS_PROMETHEUS_PORT"
	prometheusHostEnvName            = "METRICS_PROMETHEUS_HOST"
	prometheusTLSCertEnvName         = "METRICS_PROMETHEUS_TLS_CERT"
	prometheusTLSKeyEnvName          = "METRICS_PROMETHEUS_TLS_KEY"
	prometheusTLSClientAuthEnvName   = "METRICS_PROMETHEUS_TLS_CLIENT_AUTH"
	prometheusTLSClientCAFileEnvName = "METRICS_PROMETHEUS_TLS_CLIENT_CA_FILE"
	// used with network/tls.DefaultConfigFromEnv. E.g. METRICS_PROMETHEUS_TLS_MIN_VERSION.
	prometheusTLSEnvPrefix = "METRICS_PROMETHEUS_"
)

type ServerOption func(*options)

type Server struct {
	http     *http.Server
	certFile string
	keyFile  string
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
	envOverride(&o.certFile, prometheusTLSCertEnvName)
	envOverride(&o.keyFile, prometheusTLSKeyEnvName)
	envOverride(&o.clientAuth, prometheusTLSClientAuthEnvName)
	envOverride(&o.clientCAFile, prometheusTLSClientCAFileEnvName)

	if err := validate(&o); err != nil {
		return nil, err
	}

	var tlsConfig *tls.Config
	if o.certFile != "" && o.keyFile != "" {
		cfg, err := knativetls.DefaultConfigFromEnv(prometheusTLSEnvPrefix)
		if err != nil {
			return nil, err
		}
		if err := applyPrometheusClientAuth(cfg, &o); err != nil {
			return nil, err
		}
		tlsConfig = cfg
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	addr := net.JoinHostPort(o.host, o.port)

	return &Server{
		http: &http.Server{
			Addr:      addr,
			Handler:   mux,
			TLSConfig: tlsConfig,
			// https://medium.com/a-journey-with-go/go-understand-and-mitigate-slowloris-attack-711c1b1403f6
			ReadHeaderTimeout: 5 * time.Second,
		},
		certFile: o.certFile,
		keyFile:  o.keyFile,
	}, nil
}

// ListenAndServe starts the metrics server on plain HTTP.
func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

// ListenAndServeTLS starts the metrics server on TLS (HTTPS) using the given certificate and key files.
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	return s.http.ListenAndServeTLS(certFile, keyFile)
}

// Serve starts the metrics server, choosing TLS or plain HTTP based on the server configuration.
// If both METRICS_PROMETHEUS_TLS_CERT and METRICS_PROMETHEUS_TLS_KEY are set, it calls ListenAndServeTLS
func (s *Server) Serve() error {
	if s.certFile != "" && s.keyFile != "" {
		return s.http.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

type options struct {
	host         string
	port         string
	certFile     string
	keyFile      string
	clientAuth   string
	clientCAFile string
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

	if (o.certFile != "" && o.keyFile == "") || (o.certFile == "" && o.keyFile != "") {
		return fmt.Errorf("both %s and %s must be set or neither", prometheusTLSCertEnvName, prometheusTLSKeyEnvName)
	}

	tlsEnabled := o.certFile != "" && o.keyFile != ""
	auth := strings.TrimSpace(strings.ToLower(o.clientAuth))

	if auth != "" && auth != "none" && auth != "optional" && auth != "require" {
		return fmt.Errorf("invalid %s %q: must be %q, %q, or %q",
			prometheusTLSClientAuthEnvName, o.clientAuth, "none", "optional", "require")
	}

	if !tlsEnabled && ((auth != "" && auth != "none") || o.clientCAFile != "") {
		return fmt.Errorf("%s and %s require TLS to be enabled (%s and %s must be set)",
			prometheusTLSClientAuthEnvName, prometheusTLSClientCAFileEnvName, prometheusTLSCertEnvName, prometheusTLSKeyEnvName)
	}

	if tlsEnabled && (auth == "optional" || auth == "require") && strings.TrimSpace(o.clientCAFile) == "" {
		return fmt.Errorf("%s must be set when %s is %q (client certs cannot be validated without a CA)",
			prometheusTLSClientCAFileEnvName, prometheusTLSClientAuthEnvName, auth)
	}

	if tlsEnabled && (auth == "" || auth == "none") && strings.TrimSpace(o.clientCAFile) != "" {
		return fmt.Errorf("%s is set but %s is %q; set %s to %q or %q to use client certificate verification",
			prometheusTLSClientCAFileEnvName, prometheusTLSClientAuthEnvName, auth, prometheusTLSClientAuthEnvName, "optional", "require")
	}

	return nil
}

func envOverride(target *string, envName string) {
	val := os.Getenv(envName)
	if val != "" {
		*target = val
	}
}

// applyPrometheusClientAuth configures mTLS (client certificate verification) on cfg.
// o.clientAuth and o.clientCAFile are populated from env vars; validate() has already checked them.
func applyPrometheusClientAuth(cfg *tls.Config, o *options) error {
	v := strings.TrimSpace(strings.ToLower(o.clientAuth))
	if v == "" || v == "none" {
		return nil
	}

	var clientAuth tls.ClientAuthType
	switch v {
	case "optional":
		clientAuth = tls.VerifyClientCertIfGiven
	case "require":
		clientAuth = tls.RequireAndVerifyClientCert
	}

	caFile := strings.TrimSpace(o.clientCAFile)
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return fmt.Errorf("reading %s: %w", prometheusTLSClientCAFileEnvName, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("no valid CA certificates found in %s", prometheusTLSClientCAFileEnvName)
		}
		cfg.ClientCAs = pool
	}

	cfg.ClientAuth = clientAuth
	return nil
}
