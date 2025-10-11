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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"k8s.io/client-go/util/workqueue"

	"knative.dev/pkg/observability/attributekey"
)

var nameKey = attributekey.String("name")

type WorkqueueMetricsProvider struct {
	depth          metric.Int64UpDownCounter
	adds           metric.Int64Counter
	latency        metric.Float64Histogram
	workDuration   metric.Float64Histogram
	unfinishedWork metric.Float64Gauge
	longestRunning metric.Float64Gauge
	retries        metric.Int64Counter
}

type options struct {
	meterProvider metric.MeterProvider
}

type Option func(*options)

func WithMeterProvider(p metric.MeterProvider) Option {
	return func(o *options) {
		if p != nil {
			o.meterProvider = p
		}
	}
}

func NewWorkqueueMetricsProvider(opts ...Option) (*WorkqueueMetricsProvider, error) {
	options := options{
		meterProvider: otel.GetMeterProvider(),
	}
	var err error

	for _, opt := range opts {
		opt(&options)
	}

	meter := options.meterProvider.Meter(scopeName)

	w := &WorkqueueMetricsProvider{}

	w.depth, err = meter.Int64UpDownCounter(
		"kn.workqueue.depth",
		metric.WithDescription("Number of current items in the queue"),
		metric.WithUnit("{item}"),
	)
	if err != nil {
		return nil, err
	}

	w.adds, err = meter.Int64Counter(
		"kn.workqueue.adds",
		metric.WithDescription("Number of items added to the queue"),
		metric.WithUnit("{item}"),
	)
	if err != nil {
		return nil, err
	}

	w.latency, err = meter.Float64Histogram(
		"kn.workqueue.queue.duration",
		metric.WithDescription("How long an item stays in workqueue"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	if err != nil {
		return nil, err
	}

	w.workDuration, err = meter.Float64Histogram(
		"kn.workqueue.process.duration",
		metric.WithUnit("How long in seconds processing an item from workqueue takes"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	if err != nil {
		return nil, err
	}

	w.unfinishedWork, err = meter.Float64Gauge(
		"kn.workqueue.unfinished_work",
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	w.longestRunning, err = meter.Float64Gauge(
		"kn.workqueue.longest_running_processor",
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	w.retries, err = meter.Int64Counter(
		"kn.workqueue.retries",
		metric.WithDescription("Number of items re-added to the queue"),
		metric.WithUnit("{item}"),
	)
	if err != nil {
		return nil, err
	}

	return w, nil
}

func (p *WorkqueueMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return &gauge{m: p.depth, attrs: attrs(name)}
}

func (p *WorkqueueMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return &counter{m: p.adds, attrs: attrs(name)}
}

func (p *WorkqueueMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return &histogram{m: p.latency, attrs: attrs(name)}
}

func (p *WorkqueueMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return &histogram{m: p.workDuration, attrs: attrs(name)}
}

func (p *WorkqueueMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return &settableGauge{m: p.unfinishedWork, attrs: attrs(name)}
}

func (p *WorkqueueMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return &settableGauge{m: p.longestRunning, attrs: attrs(name)}
}

func (p *WorkqueueMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return &counter{m: p.retries, attrs: attrs(name)}
}

var _ workqueue.MetricsProvider = (*WorkqueueMetricsProvider)(nil)

func attrs(name string) attribute.Set {
	return attribute.NewSet(nameKey.With(name))
}
