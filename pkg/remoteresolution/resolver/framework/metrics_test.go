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

// This file lives in package framework (rather than framework_test, like
// reconciler_test.go) specifically so it can set the unexported metrics
// field directly on a Reconciler, without needing to grow the package's
// exported surface just for tests.
package framework

import (
	"errors"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	fakeresolutionclientset "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned/fake"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolutionrequestmetrics"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func newTestResolutionRequest(createdAgo time.Duration) *v1beta1.ResolutionRequest {
	return &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-createdAgo)),
			Labels: map[string]string{
				resolutioncommon.LabelKeyResolverType: LabelValueFakeResolverType,
			},
		},
	}
}

func TestWriteResolvedDataRecordsResolutionMetric(t *testing.T) {
	rr := newTestResolutionRequest(2 * time.Second)
	fakeClientset := fakeresolutionclientset.NewSimpleClientset(rr)

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	metricsRecorder, err := resolutionrequestmetrics.NewRecorderForTesting(t.Context(), provider)
	if err != nil {
		t.Fatalf("NewRecorderForTesting: %v", err)
	}

	r := &Reconciler{
		resolutionRequestClientSet: fakeClientset,
		metrics:                    metricsRecorder,
	}

	resource := &FakeResolvedResource{
		Content:       `{"apiVersion": "tekton.dev/v1", "kind": "Pipeline"}`,
		AnnotationMap: map[string]string{resolutionrequestmetrics.CachedAnnotationKey: "true"},
	}

	if err := r.writeResolvedData(t.Context(), rr, resource); err != nil {
		t.Fatalf("writeResolvedData: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	hist, found := findHistogramMetric(t, rm, "tekton_pipelines_controller_resolution_request_resolution_seconds")
	if !found {
		t.Fatal("resolution_request_resolution_seconds metric not found")
	}
	if len(hist.DataPoints) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(hist.DataPoints))
	}
	dp := hist.DataPoints[0]
	if dp.Sum <= 0 {
		t.Errorf("expected a positive resolution duration, got %v", dp.Sum)
	}

	gotCached := false
	for _, kv := range dp.Attributes.ToSlice() {
		if string(kv.Key) == "cached" {
			gotCached = kv.Value.AsBool()
		}
	}
	if !gotCached {
		t.Error("expected cached=true attribute, since the resolved resource carried the cache annotation")
	}
}

func TestWriteResolvedDataDoesNotRecordOnError(t *testing.T) {
	rr := newTestResolutionRequest(time.Second)
	// No fake clientset seeded with rr, so the Patch call in writeResolvedData
	// will fail and OnError/MarkFailed will run instead of recording a
	// resolution metric.
	fakeClientset := fakeresolutionclientset.NewSimpleClientset()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	metricsRecorder, err := resolutionrequestmetrics.NewRecorderForTesting(t.Context(), provider)
	if err != nil {
		t.Fatalf("NewRecorderForTesting: %v", err)
	}

	r := &Reconciler{
		resolutionRequestClientSet: fakeClientset,
		metrics:                    metricsRecorder,
	}

	resource := &FakeResolvedResource{
		Content: `{"apiVersion": "tekton.dev/v1", "kind": "Pipeline"}`,
	}

	if err := r.writeResolvedData(t.Context(), rr, resource); err == nil {
		t.Fatal("expected writeResolvedData to return an error when the patch fails")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	if hist, found := findHistogramMetric(t, rm, "tekton_pipelines_controller_resolution_request_resolution_seconds"); found && len(hist.DataPoints) > 0 {
		t.Errorf("expected no resolution_request_resolution_seconds data points on patch failure, got %d", len(hist.DataPoints))
	}
}

func TestMarkFailedRecordsCompletionMetric(t *testing.T) {
	rr := newTestResolutionRequest(2 * time.Second)
	fakeClientset := fakeresolutionclientset.NewSimpleClientset(rr)

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	metricsRecorder, err := resolutionrequestmetrics.NewRecorderForTesting(t.Context(), provider)
	if err != nil {
		t.Fatalf("NewRecorderForTesting: %v", err)
	}

	r := &Reconciler{
		resolutionRequestClientSet: fakeClientset,
		metrics:                    metricsRecorder,
	}

	if err := r.MarkFailed(t.Context(), rr, errors.New("boom")); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	hist, found := findHistogramMetric(t, rm, "tekton_pipelines_controller_resolution_request_completion_seconds")
	if !found {
		t.Fatal("resolution_request_completion_seconds metric not found")
	}
	if len(hist.DataPoints) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(hist.DataPoints))
	}
	if hist.DataPoints[0].Sum <= 0 {
		t.Errorf("expected a positive completion duration, got %v", hist.DataPoints[0].Sum)
	}
}

func TestMarkFailedSkipsAlreadyDoneRequest(t *testing.T) {
	rr := newTestResolutionRequest(2 * time.Second)
	rr.Status.MarkSucceeded() // IsDone() becomes true
	fakeClientset := fakeresolutionclientset.NewSimpleClientset(rr)

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	metricsRecorder, err := resolutionrequestmetrics.NewRecorderForTesting(t.Context(), provider)
	if err != nil {
		t.Fatalf("NewRecorderForTesting: %v", err)
	}

	r := &Reconciler{
		resolutionRequestClientSet: fakeClientset,
		metrics:                    metricsRecorder,
	}

	// MarkFailed should no-op (and not record a metric) for a request that's
	// already reached a terminal state, since some other actor (e.g. the
	// pipelines controller's own reconciler) already recorded its completion.
	if err := r.MarkFailed(t.Context(), rr, errors.New("too late")); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	if hist, found := findHistogramMetric(t, rm, "tekton_pipelines_controller_resolution_request_completion_seconds"); found && len(hist.DataPoints) > 0 {
		t.Errorf("expected no completion data points for an already-Done request, got %d", len(hist.DataPoints))
	}
}

func TestMarkFailedNilMetricsRecorder(t *testing.T) {
	rr := newTestResolutionRequest(time.Second)
	fakeClientset := fakeresolutionclientset.NewSimpleClientset(rr)

	// A Reconciler with no metrics wired up (e.g. an older test helper, or a
	// future caller that forgets to wire one in) must not panic.
	r := &Reconciler{resolutionRequestClientSet: fakeClientset}

	if err := r.MarkFailed(t.Context(), rr, errors.New("boom")); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
}
