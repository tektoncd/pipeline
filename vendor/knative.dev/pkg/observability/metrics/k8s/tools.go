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

package k8s

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/semconv/v1.34.0/httpconv"
	"k8s.io/client-go/tools/metrics"
)

const (
	scopeName = "knative.dev/pkg/observability/metrics/k8s"

	resultMetricName        = "kn.k8s.client.http.response.status_code"
	resultMetricDescription = "Count of response codes partitioned by method and host"
)

var latencyBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

type ClientProvider struct {
	latencyMetric httpconv.ClientRequestDuration
	resultMetric  metric.Int64Counter
}

func NewClientMetricProvider(opts ...Option) (*ClientProvider, error) {
	options := options{
		meterProvider: otel.GetMeterProvider(),
	}

	cp := &ClientProvider{}

	for _, opt := range opts {
		opt(&options)
	}

	meter := options.meterProvider.Meter(scopeName)

	var err error
	cp.latencyMetric, err = httpconv.NewClientRequestDuration(
		meter,
		metric.WithExplicitBucketBoundaries(latencyBounds...),
	)
	if err != nil {
		return nil, err
	}

	cp.resultMetric, err = meter.Int64Counter(
		resultMetricName,
		metric.WithDescription(resultMetricDescription),
		metric.WithUnit("{item}"),
	)
	if err != nil {
		return nil, err
	}

	return cp, nil
}

func (cp *ClientProvider) RequestLatencyMetric() metrics.LatencyMetric {
	return &latency{cp}
}

func (cp *ClientProvider) RequestResultMetric() metrics.ResultMetric {
	return &result{cp}
}

type latency struct {
	cp *ClientProvider
}

func (l *latency) Observe(ctx context.Context, verb string, u url.URL, latency time.Duration) {
	serverAddress := u.Hostname()
	serverPort := 80

	if u.Scheme == "https" {
		serverPort = 443
	}

	if portStr := u.Port(); portStr != "" {
		if port, err := strconv.Atoi(portStr); err != nil {
			serverPort = port
		}
	}

	elapsedTime := float64(latency) / float64(time.Second)

	l.cp.latencyMetric.Record(ctx, elapsedTime,
		httpconv.RequestMethodAttr(strings.ToUpper(verb)),
		serverAddress,
		serverPort,
		l.cp.latencyMetric.AttrURLTemplate(u.Path),
		l.cp.latencyMetric.AttrURLScheme(u.Scheme),
	)
}

type result struct {
	cp *ClientProvider
}

func (r *result) Increment(ctx context.Context, code string, method string, host string) {
	var attrs attribute.Set

	// assume we hit the happy path most frequently then in
	// that case we want to avoid the strconv.Atoi parsing
	if code == "200" {
		attrs = attribute.NewSet(
			semconv.ServerAddressKey.String(host),
			semconv.HTTPRequestMethodKey.String(strings.ToUpper(method)),
			semconv.HTTPResponseStatusCodeKey.Int(200),
		)
	} else if c, err := strconv.Atoi(code); err != nil {
		attrs = attribute.NewSet(
			semconv.ServerAddressKey.String(host),
			semconv.HTTPRequestMethodKey.String(strings.ToUpper(method)),
			semconv.HTTPResponseStatusCodeKey.Int(c),
		)
	} else {
		attrs = attribute.NewSet(
			semconv.ServerAddressKey.String(host),
			semconv.HTTPRequestMethodKey.String(strings.ToUpper(method)),
		)
	}

	r.cp.resultMetric.Add(ctx, 1, metric.WithAttributeSet(attrs))
}

var (
	_ metrics.LatencyMetric = (*latency)(nil)
	_ metrics.ResultMetric  = (*result)(nil)
)
