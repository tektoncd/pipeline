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

package resolutionrequestmetrics

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	resolutionSecondsMetricName = "tekton_pipelines_controller_resolution_request_resolution_seconds"
	completionSecondsMetricName = "tekton_pipelines_controller_resolution_request_completion_seconds"
)

func resetMetrics() {
	once = sync.Once{}
	r = nil
	errRegistering = nil
}

func newResolutionRequest(namespace, resolverType string, createdAgo time.Duration, annotations map[string]string) *v1beta1.ResolutionRequest {
	rr := &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-createdAgo)),
		},
	}
	if resolverType != "" {
		rr.Labels = map[string]string{resolutioncommon.LabelKeyResolverType: resolverType}
	}
	rr.Status.Annotations = annotations
	return rr
}

func findHistogramMetric(t *testing.T, rm metricdata.ResourceMetrics, name string) (metricdata.Histogram[float64], bool) {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				hist, ok := m.Data.(metricdata.Histogram[float64])
				if !ok {
					t.Fatalf("metric %s data is not Histogram[float64]: %T", name, m.Data)
				}
				return hist, true
			}
		}
	}
	return metricdata.Histogram[float64]{}, false
}

func TestNewRecorder(t *testing.T) {
	resetMetrics()
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	rec, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	if !rec.initialized {
		t.Error("expected recorder to be initialized")
	}

	// A second call should return the same, already-initialized instance
	// (sync.Once), mirroring how both cmd/controller and cmd/resolvers may
	// independently call NewRecorder in the same process.
	rec2, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder (second call): %v", err)
	}
	if rec != rec2 {
		t.Error("expected NewRecorder to return the same singleton instance")
	}
}

func TestCached(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{name: "nil annotations", annotations: nil, expected: false},
		{name: "no cache annotation", annotations: map[string]string{"foo": "bar"}, expected: false},
		{name: "cached false", annotations: map[string]string{CachedAnnotationKey: "false"}, expected: false},
		{name: "cached true", annotations: map[string]string{CachedAnnotationKey: "true"}, expected: true},
		{name: "cached garbage value", annotations: map[string]string{CachedAnnotationKey: "yes"}, expected: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Cached(test.annotations); got != test.expected {
				t.Errorf("Cached(%v) = %v, want %v", test.annotations, got, test.expected)
			}
		})
	}
}

func TestRecordResolutionAndCompletion(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		cached       bool
		expectedTags map[string]string
	}{
		{
			name:         "uncached git resolver",
			resolverType: "git",
			cached:       false,
			expectedTags: map[string]string{"resolver": "git", "cached": "false", "namespace": "ns"},
		},
		{
			name:         "cached hub resolver",
			resolverType: "hub",
			cached:       true,
			expectedTags: map[string]string{"resolver": "hub", "cached": "true", "namespace": "ns"},
		},
		{
			name:         "missing resolver type label falls back to unknown",
			resolverType: "",
			cached:       false,
			expectedTags: map[string]string{"resolver": unknownResolverType, "cached": "false", "namespace": "ns"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetMetrics()
			ctx := context.Background()
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			otel.SetMeterProvider(provider)

			rec, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			rr := newResolutionRequest("ns", test.resolverType, 2*time.Second, nil)
			rec.RecordResolution(ctx, rr, test.cached)
			rec.RecordCompletion(ctx, rr, test.cached)

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &rm); err != nil {
				t.Fatalf("Collect error: %v", err)
			}

			for _, metricName := range []string{resolutionSecondsMetricName, completionSecondsMetricName} {
				hist, found := findHistogramMetric(t, rm, metricName)
				if !found {
					t.Fatalf("%s metric not found", metricName)
				}
				if len(hist.DataPoints) != 1 {
					t.Fatalf("expected 1 data point for %s, got %d", metricName, len(hist.DataPoints))
				}
				dp := hist.DataPoints[0]
				if dp.Sum <= 0 {
					t.Errorf("%s: expected positive elapsed seconds, got %v", metricName, dp.Sum)
				}

				gotAttrs := make(map[string]string)
				for _, kv := range dp.Attributes.ToSlice() {
					gotAttrs[string(kv.Key)] = fmt.Sprintf("%v", kv.Value.AsInterface())
				}
				if d := cmp.Diff(test.expectedTags, gotAttrs); d != "" {
					t.Errorf("%s attributes diff (-want, +got): %s", metricName, d)
				}
			}
		})
	}
}

func TestRecordResolutionNilSafety(t *testing.T) {
	resetMetrics()
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	// A nil *Recorder (the zero value for an unwired caller) must be a no-op,
	// not a panic.
	var nilRec *Recorder
	nilRec.RecordResolution(ctx, newResolutionRequest("ns", "git", time.Second, nil), false)
	nilRec.RecordCompletion(ctx, newResolutionRequest("ns", "git", time.Second, nil), false)

	// An uninitialized (but non-nil) Recorder, and a nil ResolutionRequest,
	// must also be no-ops.
	uninitRec := &Recorder{}
	uninitRec.RecordResolution(ctx, newResolutionRequest("ns", "git", time.Second, nil), false)

	rec, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	rec.RecordResolution(ctx, nil, false)
	rec.RecordCompletion(ctx, nil, false)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == resolutionSecondsMetricName || m.Name == completionSecondsMetricName {
				if hist, ok := m.Data.(metricdata.Histogram[float64]); ok && len(hist.DataPoints) > 0 {
					t.Errorf("expected no data points recorded for nil-safety cases, found %d for %s", len(hist.DataPoints), m.Name)
				}
			}
		}
	}
}
