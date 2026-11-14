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

package taskrunmetrics

import (
	"bytes"
	"context"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	exemplarTraceID = trace.TraceID{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19}
	exemplarSpanID  = trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
)

// withSpan returns ctx carrying a span context like the one the reconciler
// starts before it records metrics.
func withSpan(ctx context.Context, sampled bool) context.Context {
	cfg := trace.SpanContextConfig{TraceID: exemplarTraceID, SpanID: exemplarSpanID}
	if sampled {
		cfg.TraceFlags = trace.FlagsSampled
	}
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(cfg))
}

// exemplarIDs returns the trace and span IDs of the exemplars recorded on the
// first data point of the named histogram or counter.
func exemplarIDs(t *testing.T, rm metricdata.ResourceMetrics, name string) (traceIDs, spanIDs [][]byte) {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			switch data := m.Data.(type) {
			case metricdata.Histogram[float64]:
				if len(data.DataPoints) > 0 {
					for _, e := range data.DataPoints[0].Exemplars {
						traceIDs, spanIDs = append(traceIDs, e.TraceID), append(spanIDs, e.SpanID)
					}
					return traceIDs, spanIDs
				}
			case metricdata.Sum[int64]:
				if len(data.DataPoints) > 0 {
					for _, e := range data.DataPoints[0].Exemplars {
						traceIDs, spanIDs = append(traceIDs, e.TraceID), append(spanIDs, e.SpanID)
					}
					return traceIDs, spanIDs
				}
			}
			t.Fatalf("metric %s has no data points", name)
		}
	}
	t.Fatalf("metric %s not found", name)
	return nil, nil
}

// TestDurationAndCountExemplars checks that the duration and count metrics
// carry the trace of the reconcile that recorded them, so that a metric data
// point can be linked to its trace.
func TestDurationAndCountExemplars(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sampled bool
	}{
		{name: "sampled span", sampled: true},
		{name: "unsampled span", sampled: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetMetrics()
			ctx := getConfigContext(false, false)
			reader := sdkmetric.NewManualReader()
			otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

			rec, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}
			tr := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
				Spec:       v1.TaskRunSpec{TaskRef: &v1.TaskRef{Name: "task-1"}},
				Status: v1.TaskRunStatus{
					Status: duckv1.Status{Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}}},
					TaskRunStatusFields: v1.TaskRunStatusFields{
						StartTime:      &startTime,
						CompletionTime: &completionTime,
					},
				},
			}
			if err := rec.DurationAndCount(withSpan(ctx, tc.sampled), tr, nil); err != nil {
				t.Fatalf("DurationAndCount: %v", err)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &rm); err != nil {
				t.Fatalf("Collect: %v", err)
			}
			for _, name := range []string{
				"tekton_pipelines_controller_taskrun_duration_seconds",
				"tekton_pipelines_controller_taskrun_total",
			} {
				traceIDs, spanIDs := exemplarIDs(t, rm, name)
				if !tc.sampled {
					if len(traceIDs) != 0 {
						t.Errorf("%s: got %d exemplars without a sampled span, want none", name, len(traceIDs))
					}
					continue
				}
				if len(traceIDs) == 0 {
					t.Fatalf("%s: no exemplar recorded with a sampled span", name)
				}
				if !bytes.Equal(traceIDs[0], exemplarTraceID[:]) || !bytes.Equal(spanIDs[0], exemplarSpanID[:]) {
					t.Errorf("%s: exemplar trace/span = %x/%x, want %x/%x", name, traceIDs[0], spanIDs[0], exemplarTraceID[:], exemplarSpanID[:])
				}
			}
		})
	}
}
