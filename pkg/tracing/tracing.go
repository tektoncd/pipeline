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
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/system"
)

type tracerProvider struct {
	embedded.TracerProvider

	service  string
	provider trace.TracerProvider
	cfg      *config.Tracing
	username string
	password string
	logger   *zap.SugaredLogger
}

func init() {
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

// New returns a new instance of tracerProvider for the given service
func New(service string, logger *zap.SugaredLogger) *tracerProvider {
	return &tracerProvider{
		service:  service,
		provider: noop.NewTracerProvider(),
		logger:   logger,
	}
}

// OnStore configures tracerProvider dynamically
func (t *tracerProvider) OnStore(lister listerv1.SecretLister) func(name string, value interface{}) {
	return func(name string, value interface{}) {
		if name != config.GetTracingConfigName() {
			return
		}

		cfg, ok := value.(*config.Tracing)
		if !ok {
			t.logger.Error("tracing configmap is in invalid format. value: %v", value)
			return
		}

		if cfg.Equals(t.cfg) {
			t.logger.Info("tracing config unchanged", cfg, t.cfg)
			return
		}
		t.cfg = cfg

		if lister != nil && cfg.CredentialsSecret != "" {
			sec, err := lister.Secrets(system.Namespace()).Get(cfg.CredentialsSecret)
			if err != nil {
				t.logger.Errorf("unable to initialize tracing with error : %v", err.Error())
				return
			}
			creds := sec.Data
			t.username = string(creds["username"])
			t.password = string(creds["password"])
		} else {
			t.username = ""
			t.password = ""
		}

		t.reinitialize()
	}
}

func (t *tracerProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	return t.provider.Tracer(name, options...)
}

// Handler is called by the informer when the secret is updated
func (t *tracerProvider) Handler(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		t.logger.Error("Failed to do type assertion for Secret")
		return
	}
	t.OnSecret(secret)
}

func (t *tracerProvider) OnSecret(secret *corev1.Secret) {
	if secret.Name != t.cfg.CredentialsSecret {
		return
	}

	creds := secret.Data
	username := string(creds["username"])
	password := string(creds["password"])

	if t.username == username && t.password == password {
		// No change in credentials, no need to reinitialize
		return
	}
	t.username = username
	t.password = password

	t.logger.Debugf("tracing credentials updated, reinitializing tracingprovider with secret: %v", secret.Name)

	t.reinitialize()
}

func (t *tracerProvider) reinitialize() {
	tp, err := createTracerProvider(t.service, t.cfg, t.username, t.password)
	if err != nil {
		t.logger.Errorf("unable to initialize tracing with error : %v", err.Error())
		return
	}
	t.logger.Info("initialized Tracer Provider")
	if p, ok := t.provider.(*tracesdk.TracerProvider); ok {
		if err := p.Shutdown(context.Background()); err != nil {
			t.logger.Errorf("unable to shutdown tracingprovider with error : %v", err.Error())
		}
	}
	t.provider = tp
}

func createTracerProvider(service string, cfg *config.Tracing, user, pass string) (trace.TracerProvider, error) {
	if !cfg.Enabled {
		return noop.NewTracerProvider(), nil
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(u.Host),
		otlptracehttp.WithURLPath(u.Path),
	}

	if u.Scheme == "http" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	if user != "" && pass != "" {
		creds := fmt.Sprintf("%s:%s", user, pass)
		enc := base64.StdEncoding.EncodeToString([]byte(creds))
		o := otlptracehttp.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Basic %s", enc),
		})
		opts = append(opts, o)
	}

	ctx := context.Background()
	exp, err := otlptracehttp.New(ctx, opts...)

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
