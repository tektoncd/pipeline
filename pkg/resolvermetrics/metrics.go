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

package resolvermetrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	meterName = "tekton_pipelines_resolver"

	StatusSuccess        = "success"
	StatusError          = "error"
	StatusTimeout        = "timeout"
	StatusInvalidRequest = "invalid_request"
)

var (
	resolverTypeKey = attribute.Key("resolver_type")
	statusKey       = attribute.Key("status")
)

// Recorder holds OpenTelemetry instruments for resolver metrics.
type Recorder struct {
	initialized bool
	meter       metric.Meter

	resolutionDuration     metric.Float64Histogram
	resolutionTotal        metric.Int64Counter
	cacheHitTotal          metric.Int64Counter
	cacheMissTotal         metric.Int64Counter
	singleflightDedupTotal metric.Int64Counter
}

// NewRecorder creates a new metrics Recorder for the resolver framework.
func NewRecorder() (*Recorder, error) {
	r := &Recorder{
		meter: otel.GetMeterProvider().Meter(meterName),
	}

	duration, err := r.meter.Float64Histogram(
		"tekton_pipelines_resolver_resolution_duration_seconds",
		metric.WithDescription("Duration of resolution requests in seconds."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolution duration histogram: %w", err)
	}
	r.resolutionDuration = duration

	total, err := r.meter.Int64Counter(
		"tekton_pipelines_resolver_resolution_total",
		metric.WithDescription("Total number of resolution requests."),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolution total counter: %w", err)
	}
	r.resolutionTotal = total

	cacheHit, err := r.meter.Int64Counter(
		"tekton_pipelines_resolver_cache_hit_total",
		metric.WithDescription("Total number of resolver cache hits."),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache hit counter: %w", err)
	}
	r.cacheHitTotal = cacheHit

	cacheMiss, err := r.meter.Int64Counter(
		"tekton_pipelines_resolver_cache_miss_total",
		metric.WithDescription("Total number of resolver cache misses."),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache miss counter: %w", err)
	}
	r.cacheMissTotal = cacheMiss

	singleflightDedup, err := r.meter.Int64Counter(
		"tekton_pipelines_resolver_singleflight_dedup_total",
		metric.WithDescription("Total number of resolution requests deduplicated by singleflight."),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create singleflight dedup counter: %w", err)
	}
	r.singleflightDedupTotal = singleflightDedup

	r.initialized = true
	return r, nil
}

// RecordResolution records the duration and count of a resolution request.
func (r *Recorder) RecordResolution(ctx context.Context, resolverType, status string, duration time.Duration) {
	if r == nil || !r.initialized {
		return
	}

	attrs := metric.WithAttributes(
		resolverTypeKey.String(resolverType),
		statusKey.String(status),
	)

	r.resolutionDuration.Record(ctx, duration.Seconds(), attrs)
	r.resolutionTotal.Add(ctx, 1, attrs)
}

// RecordCacheHit records a resolver cache hit.
func (r *Recorder) RecordCacheHit(ctx context.Context, resolverType string) {
	if r == nil || !r.initialized {
		return
	}

	r.cacheHitTotal.Add(ctx, 1, metric.WithAttributes(
		resolverTypeKey.String(resolverType),
	))
}

// RecordCacheMiss records a resolver cache miss.
func (r *Recorder) RecordCacheMiss(ctx context.Context, resolverType string) {
	if r == nil || !r.initialized {
		return
	}

	r.cacheMissTotal.Add(ctx, 1, metric.WithAttributes(
		resolverTypeKey.String(resolverType),
	))
}

// RecordSingleflightDedup records a singleflight deduplication event.
func (r *Recorder) RecordSingleflightDedup(ctx context.Context, resolverType string) {
	if r == nil || !r.initialized {
		return
	}

	r.singleflightDedupTotal.Add(ctx, 1, metric.WithAttributes(
		resolverTypeKey.String(resolverType),
	))
}
