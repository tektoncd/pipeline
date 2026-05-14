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
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func setupTestMeter(t *testing.T) (*sdkmetric.ManualReader, func()) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	oldProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	return reader, func() {
		otel.SetMeterProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	}
}

func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	return rm
}

func findMetric(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

func TestNewRecorder(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}
	if rec == nil {
		t.Fatal("NewRecorder() returned nil")
	}
	if !rec.initialized {
		t.Fatal("Recorder not initialized")
	}

	rec.RecordResolution(context.Background(), "git", StatusSuccess, 100*time.Millisecond)
	rm := collectMetrics(t, reader)

	duration := findMetric(rm, "tekton_pipelines_resolver_resolution_duration_seconds")
	if duration == nil {
		t.Fatal("resolution_duration_seconds metric not found")
	}
	total := findMetric(rm, "tekton_pipelines_resolver_resolution_total")
	if total == nil {
		t.Fatal("resolution_total metric not found")
	}
}

func TestRecordResolutionSuccess(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordResolution(context.Background(), "git", StatusSuccess, 500*time.Millisecond)

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "tekton_pipelines_resolver_resolution_total")
	if total == nil {
		t.Fatal("resolution_total not found")
	}

	sum, ok := total.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64], got %T", total.Data)
	}
	if len(sum.DataPoints) == 0 {
		t.Fatal("no data points for resolution_total")
	}
	if sum.DataPoints[0].Value != 1 {
		t.Errorf("expected total count 1, got %d", sum.DataPoints[0].Value)
	}
	assertAttribute(t, sum.DataPoints[0].Attributes, "resolver_type", "git")
	assertAttribute(t, sum.DataPoints[0].Attributes, "status", "success")
}

func TestRecordResolutionError(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordResolution(context.Background(), "hub", StatusError, 200*time.Millisecond)

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "tekton_pipelines_resolver_resolution_total")
	if total == nil {
		t.Fatal("resolution_total not found")
	}
	sum := total.Data.(metricdata.Sum[int64])
	assertAttribute(t, sum.DataPoints[0].Attributes, "resolver_type", "hub")
	assertAttribute(t, sum.DataPoints[0].Attributes, "status", "error")
}

func TestRecordResolutionTimeout(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordResolution(context.Background(), "bundles", StatusTimeout, 60*time.Second)

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "tekton_pipelines_resolver_resolution_total")
	if total == nil {
		t.Fatal("resolution_total not found")
	}
	sum := total.Data.(metricdata.Sum[int64])
	assertAttribute(t, sum.DataPoints[0].Attributes, "resolver_type", "bundles")
	assertAttribute(t, sum.DataPoints[0].Attributes, "status", "timeout")
}

func TestRecordResolutionDuration(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordResolution(context.Background(), "cluster", StatusSuccess, 250*time.Millisecond)

	rm := collectMetrics(t, reader)

	duration := findMetric(rm, "tekton_pipelines_resolver_resolution_duration_seconds")
	if duration == nil {
		t.Fatal("resolution_duration_seconds not found")
	}
	hist, ok := duration.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("expected Histogram[float64], got %T", duration.Data)
	}
	if len(hist.DataPoints) == 0 {
		t.Fatal("no data points for duration histogram")
	}
	if hist.DataPoints[0].Count != 1 {
		t.Errorf("expected histogram count 1, got %d", hist.DataPoints[0].Count)
	}
	if hist.DataPoints[0].Sum < 0.2 || hist.DataPoints[0].Sum > 0.3 {
		t.Errorf("expected histogram sum ~0.25, got %f", hist.DataPoints[0].Sum)
	}
}

func TestRecordCacheHit(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordCacheHit(context.Background(), "git")
	rec.RecordCacheHit(context.Background(), "git")
	rec.RecordCacheHit(context.Background(), "bundles")

	rm := collectMetrics(t, reader)

	cacheHit := findMetric(rm, "tekton_pipelines_resolver_cache_hit_total")
	if cacheHit == nil {
		t.Fatal("cache_hit_total not found")
	}
	sum, ok := cacheHit.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("expected Sum[int64], got %T", cacheHit.Data)
	}

	gitCount := int64(0)
	bundlesCount := int64(0)
	for _, dp := range sum.DataPoints {
		val, _ := dp.Attributes.Value("resolver_type")
		switch val.AsString() {
		case "git":
			gitCount = dp.Value
		case "bundles":
			bundlesCount = dp.Value
		}
	}
	if gitCount != 2 {
		t.Errorf("expected git cache hits 2, got %d", gitCount)
	}
	if bundlesCount != 1 {
		t.Errorf("expected bundles cache hits 1, got %d", bundlesCount)
	}
}

func TestRecordCacheMiss(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordCacheMiss(context.Background(), "git")

	rm := collectMetrics(t, reader)
	cacheMiss := findMetric(rm, "tekton_pipelines_resolver_cache_miss_total")
	if cacheMiss == nil {
		t.Fatal("cache_miss_total not found")
	}
	sum := cacheMiss.Data.(metricdata.Sum[int64])
	if len(sum.DataPoints) == 0 || sum.DataPoints[0].Value != 1 {
		t.Errorf("expected cache miss count 1, got %d", sum.DataPoints[0].Value)
	}
}

func TestRecordSingleflightDedup(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordSingleflightDedup(context.Background(), "bundles")
	rec.RecordSingleflightDedup(context.Background(), "bundles")

	rm := collectMetrics(t, reader)
	dedup := findMetric(rm, "tekton_pipelines_resolver_singleflight_dedup_total")
	if dedup == nil {
		t.Fatal("singleflight_dedup_total not found")
	}
	sum := dedup.Data.(metricdata.Sum[int64])
	if len(sum.DataPoints) == 0 || sum.DataPoints[0].Value != 2 {
		t.Errorf("expected singleflight dedup count 2, got %d", sum.DataPoints[0].Value)
	}
}

func TestRecordResolutionInvalidRequest(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	rec.RecordResolution(context.Background(), "http", StatusInvalidRequest, 5*time.Millisecond)

	rm := collectMetrics(t, reader)
	total := findMetric(rm, "tekton_pipelines_resolver_resolution_total")
	if total == nil {
		t.Fatal("resolution_total not found")
	}
	sum := total.Data.(metricdata.Sum[int64])
	assertAttribute(t, sum.DataPoints[0].Attributes, "status", "invalid_request")
	assertAttribute(t, sum.DataPoints[0].Attributes, "resolver_type", "http")
}

func TestRecordResolutionNilRecorder(t *testing.T) {
	var rec *Recorder
	rec.RecordResolution(context.Background(), "git", StatusSuccess, time.Second)
	rec.RecordCacheHit(context.Background(), "git")
	rec.RecordCacheMiss(context.Background(), "git")
	rec.RecordSingleflightDedup(context.Background(), "git")
}

func TestRecordResolutionConcurrency(t *testing.T) {
	reader, cleanup := setupTestMeter(t)
	defer cleanup()

	rec, err := NewRecorder()
	if err != nil {
		t.Fatalf("NewRecorder() error: %v", err)
	}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec.RecordResolution(context.Background(), "git", StatusSuccess, time.Millisecond)
		}()
	}
	wg.Wait()

	rm := collectMetrics(t, reader)
	total := findMetric(rm, "tekton_pipelines_resolver_resolution_total")
	if total == nil {
		t.Fatal("resolution_total not found")
	}
	sum := total.Data.(metricdata.Sum[int64])
	if len(sum.DataPoints) == 0 || sum.DataPoints[0].Value != 100 {
		t.Errorf("expected 100 concurrent recordings, got %d", sum.DataPoints[0].Value)
	}
}

func assertAttribute(t *testing.T, attrs attribute.Set, key, expected string) {
	t.Helper()
	val, exists := attrs.Value(attribute.Key(key))
	if !exists {
		t.Errorf("attribute %q not found", key)
		return
	}
	if val.AsString() != expected {
		t.Errorf("attribute %q: expected %q, got %q", key, expected, val.AsString())
	}
}
