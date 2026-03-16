/*
Copyright 2019 The Tekton Authors

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

package pipelinerunmetrics

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	startTime      = metav1.Now()
	completionTime = metav1.NewTime(startTime.Time.Add(time.Minute))
)

func resetMetrics() {
	once = sync.Once{}
	r = nil
	errRegistering = nil
}

func getConfigContext(countWithReason bool) context.Context {
	ctx := context.Background()
	cfg := &config.Config{
		Metrics: &config.Metrics{
			TaskrunLevel:            config.TaskrunLevelAtTaskrun,
			PipelinerunLevel:        config.PipelinerunLevelAtPipelinerun,
			RunningPipelinerunLevel: config.DefaultRunningPipelinerunLevel,
			DurationTaskrunType:     config.DefaultDurationTaskrunType,
			DurationPipelinerunType: config.DefaultDurationPipelinerunType,
			CountWithReason:         countWithReason,
		},
	}
	return config.ToContext(ctx, cfg)
}

func getConfigContextRunningPRLevel(runningPipelinerunLevel string) context.Context {
	ctx := context.Background()
	cfg := &config.Config{
		Metrics: &config.Metrics{
			TaskrunLevel:            config.TaskrunLevelAtTaskrun,
			PipelinerunLevel:        config.PipelinerunLevelAtPipelinerun,
			DurationTaskrunType:     config.DefaultDurationTaskrunType,
			DurationPipelinerunType: config.DefaultDurationPipelinerunType,
			CountWithReason:         false,
			RunningPipelinerunLevel: runningPipelinerunLevel,
		},
	}
	return config.ToContext(ctx, cfg)
}

func TestUninitializedMetrics(t *testing.T) {
	resetMetrics()
	metrics := Recorder{}

	if err := metrics.DurationAndCount(t.Context(), &v1.PipelineRun{}, nil); err == nil {
		t.Error("DurationAndCount recording expected to return error but got nil")
	}
	if err := metrics.observeRunningPipelineRuns(t.Context(), nil, nil); err == nil {
		t.Error("Current PR count recording expected to return error but got nil")
	}
}

func TestDurationAndCountNilStartTime(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			// StartTime deliberately nil
		},
	}

	if err := r.DurationAndCount(ctx, pr, nil); err != nil {
		t.Errorf("DurationAndCount with nil StartTime returned error: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	// Duration should be recorded as 0
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_pipelinerun_duration_seconds" {
				if hist, ok := m.Data.(metricdata.Histogram[float64]); ok && len(hist.DataPoints) > 0 {
					if hist.DataPoints[0].Sum != 0 {
						t.Errorf("Expected duration 0 for nil StartTime, got %v", hist.DataPoints[0].Sum)
					}
				}
				return
			}
		}
	}
	t.Error("duration metric not found")
}

func TestDurationAndCountGaugeDurationType(t *testing.T) {
	resetMetrics()
	ctx := context.Background()
	cfg := &config.Config{
		Metrics: &config.Metrics{
			TaskrunLevel:            config.TaskrunLevelAtTaskrun,
			PipelinerunLevel:        config.PipelinerunLevelAtPipelinerun,
			RunningPipelinerunLevel: config.DefaultRunningPipelinerunLevel,
			DurationTaskrunType:     config.DefaultDurationTaskrunType,
			DurationPipelinerunType: config.DurationPipelinerunTypeLastValue,
		},
	}
	ctx = config.ToContext(ctx, cfg)

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
	}

	if err := r.DurationAndCount(ctx, pr, nil); err != nil {
		t.Fatalf("DurationAndCount: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_pipelinerun_duration_seconds" {
				if _, ok := m.Data.(metricdata.Gauge[float64]); !ok {
					t.Errorf("Expected Gauge[float64] for LastValue duration type, got %T", m.Data)
				}
				return
			}
		}
	}
	t.Error("duration metric not found")
}

func TestOnStoreInvalidConfig(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	logger := zap.NewExample().Sugar()

	// An invalid PipelinerunLevel causes configure to return an error.
	// OnStore should log it but not corrupt the recorder state.
	invalidCfg := &config.Metrics{
		PipelinerunLevel: "invalid-level",
	}
	OnStore(logger, r)(config.GetMetricsConfigName(), invalidCfg)

	// Recorder should still be initialized and functional.
	if !r.initialized {
		t.Error("recorder should remain initialized after a failed configure")
	}
}

func TestOnStore(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// create a logger
	logger := zap.NewExample().Sugar()

	t.Run("wrong name", func(t *testing.T) {
		OnStore(logger, r)("wrong-name", &config.Metrics{PipelinerunLevel: config.PipelinerunLevelAtNS})
		if r.cfg.PipelinerunLevel != config.PipelinerunLevelAtPipelinerun {
			t.Error("OnStore should not have updated config")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		OnStore(logger, r)(config.GetMetricsConfigName(), &config.Store{})
		if r.cfg.PipelinerunLevel != config.PipelinerunLevelAtPipelinerun {
			t.Error("OnStore should not have updated config")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		newConfig := &config.Metrics{
			PipelinerunLevel: config.PipelinerunLevelAtPipeline,
		}
		OnStore(logger, r)(config.GetMetricsConfigName(), newConfig)
		if r.cfg.PipelinerunLevel != config.PipelinerunLevelAtPipeline {
			t.Error("OnStore should have updated config")
		}
	})
}

func TestUpdateConfig(t *testing.T) {
	resetMetrics()
	// Test that the config is updated when it changes, and not when it doesn't.
	ctx := getConfigContext(false)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// First, update with a new config.
	newConfig := &config.Metrics{
		PipelinerunLevel: config.PipelinerunLevelAtPipeline,
	}
	if r.updateConfig(newConfig) == nil {
		t.Error("updateConfig should have returned the new config, but returned nil")
	}

	// Then, update with the same config.
	if r.updateConfig(newConfig) != nil {
		t.Error("updateConfig should have returned nil for unchanged config, but returned non-nil")
	}

	// Finally, update with a different config.
	differentConfig := &config.Metrics{
		PipelinerunLevel: config.PipelinerunLevelAtNS,
	}
	if r.updateConfig(differentConfig) == nil {
		t.Error("updateConfig should have returned the new config, but returned nil")
	}
}

func TestRecordPipelineRunDurationCount(t *testing.T) {
	for _, test := range []struct {
		name             string
		pipelineRun      *v1.PipelineRun
		beforeCondition  *apis.Condition
		countWithReason  bool
		pipelinerunLevel string
		expectedTags     map[string]string
		expectedCount    int64
		expectedDuration float64
	}{{
		name: "for succeeded pipeline",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition:  nil,
		countWithReason:  false,
		pipelinerunLevel: config.PipelinerunLevelAtPipelinerun,
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for succeeded pipeline at pipeline level",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition:  nil,
		countWithReason:  false,
		pipelinerunLevel: config.PipelinerunLevelAtPipeline,
		expectedTags: map[string]string{
			"pipeline":  "pipeline-1",
			"namespace": "ns",
			"status":    "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for succeeded pipeline at namespace level",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition:  nil,
		countWithReason:  false,
		pipelinerunLevel: config.PipelinerunLevelAtNS,
		expectedTags: map[string]string{
			"namespace": "ns",
			"status":    "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for succeeded pipeline different condition",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: &apis.Condition{
			Type:   apis.ConditionReady,
			Status: corev1.ConditionUnknown,
		},
		countWithReason: false,
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for failed pipeline",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: nil,
		countWithReason: false,
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "failed",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for failed pipeline with reason",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "TaskRunImagePullFailed",
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: nil,
		countWithReason: true,
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "failed",
			"reason":      "TaskRunImagePullFailed",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for cancelled pipeline",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: v1.PipelineRunReasonCancelled.String(),
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: nil,
		countWithReason: false,
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "cancelled",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "no record when condition unchanged",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		// beforeCondition matches afterCondition — DurationAndCount should be a no-op
		beforeCondition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		countWithReason:  false,
		expectedTags:     nil, // no metric expected
		expectedCount:    0,
		expectedDuration: 0,
	}} {
		t.Run(test.name, func(t *testing.T) {
			resetMetrics()
			ctx := getConfigContext(test.countWithReason)
			if test.pipelinerunLevel != "" {
				cfg := config.FromContextOrDefaults(ctx)
				cfg.Metrics.PipelinerunLevel = test.pipelinerunLevel
				ctx = config.ToContext(ctx, cfg)
			}
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			otel.SetMeterProvider(provider)

			r, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			// We don't need to manually reset meter anymore if we use resetMetrics()
			// and set global provider before NewRecorder

			if err := r.DurationAndCount(ctx, test.pipelineRun, test.beforeCondition); err != nil {
				t.Errorf("DurationAndCount recording failed: %v", err)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &rm); err != nil {
				t.Fatalf("Collect error: %v", err)
			}

			// When condition is unchanged DurationAndCount is a no-op; no data recorded.
			if test.expectedTags == nil {
				if len(rm.ScopeMetrics) != 0 {
					t.Errorf("Expected no metrics, got %d scope metrics", len(rm.ScopeMetrics))
				}
				return
			}

			if len(rm.ScopeMetrics) != 1 {
				t.Fatalf("Expected 1 scope metric, got %d", len(rm.ScopeMetrics))
			}

			// Check duration
			var durationMetric metricdata.Metrics
			var totalMetric metricdata.Metrics

			for _, m := range rm.ScopeMetrics[0].Metrics {
				if m.Name == "tekton_pipelines_controller_pipelinerun_duration_seconds" {
					durationMetric = m
				}
				if m.Name == "tekton_pipelines_controller_pipelinerun_total" {
					totalMetric = m
				}
			}

			if durationMetric.Name == "" {
				t.Error("duration metric not found")
			} else {
				hist, ok := durationMetric.Data.(metricdata.Histogram[float64])
				if !ok {
					t.Errorf("duration metric data is not a Histogram[float64]: %T", durationMetric.Data)
				} else {
					if len(hist.DataPoints) != 1 {
						t.Errorf("Expected 1 duration data point, got %d", len(hist.DataPoints))
					} else {
						dp := hist.DataPoints[0]
						if dp.Sum != test.expectedDuration {
							t.Errorf("Expected duration sum %v, got %v", test.expectedDuration, dp.Sum)
						}

						// Verify attributes
						gotAttrs := make(map[string]string)
						for _, kv := range dp.Attributes.ToSlice() {
							gotAttrs[string(kv.Key)] = kv.Value.AsString()
						}
						if d := cmp.Diff(test.expectedTags, gotAttrs); d != "" {
							t.Errorf("Duration attributes diff (-want, +got): %s", d)
						}
					}
				}
			}

			if totalMetric.Name == "" {
				t.Error("total metric not found")
			} else {
				sum, ok := totalMetric.Data.(metricdata.Sum[int64])
				if !ok {
					t.Errorf("total metric data is not a Sum[int64]: %T", totalMetric.Data)
				} else {
					if len(sum.DataPoints) != 1 {
						t.Errorf("Expected 1 total data point, got %d", len(sum.DataPoints))
					} else {
						dp := sum.DataPoints[0]
						if dp.Value != test.expectedCount {
							t.Errorf("Expected total count %v, got %v", test.expectedCount, dp.Value)
						}

						// Verify attributes for count (status only)
						expectedCountTags := map[string]string{"status": test.expectedTags["status"]}
						gotAttrs := make(map[string]string)
						for _, kv := range dp.Attributes.ToSlice() {
							gotAttrs[string(kv.Key)] = kv.Value.AsString()
						}
						if d := cmp.Diff(expectedCountTags, gotAttrs); d != "" {
							t.Errorf("Count attributes diff (-want, +got): %s", d)
						}
					}
				}
			}
		})
	}
}

func TestRecordRunningPipelineRunsCount(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Create a mock lister that returns an empty list
	mockLister := &mockPipelineRunLister{}

	// Register callback manually
	_, err = r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		return r.observeRunningPipelineRuns(ctx, o, mockLister)
	}, r.runningPRsGauge, r.runningPRsWaitingOnPipelineResolutionGauge, r.runningPRsWaitingOnTaskResolutionGauge)
	if err != nil {
		t.Fatalf("Failed to register callback: %v", err)
	}

	// Verify the recorder is properly initialized
	if !r.initialized {
		t.Error("Recorder should be initialized")
	}
}

// mockPipelineRunLister implements the listers.PipelineRunLister interface.
type mockPipelineRunLister struct {
	prs []*v1.PipelineRun
	err error
}

func (m *mockPipelineRunLister) List(selector labels.Selector) ([]*v1.PipelineRun, error) {
	return m.prs, m.err
}

func (m *mockPipelineRunLister) PipelineRuns(namespace string) listers.PipelineRunNamespaceLister {
	// This function is not needed for these tests.
	return nil
}

func newPipelineRun(status corev1.ConditionStatus, namespace, pipelineName, prName string) *v1.PipelineRun {
	if prName == "" {
		prName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pipelinerun")
	}
	return &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: prName, Namespace: namespace},
		Spec: v1.PipelineRunSpec{
			PipelineRef: &v1.PipelineRef{
				Name: pipelineName,
			},
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: status,
				}},
			},
		},
	}
}

func TestRecordRunningPipelineRunsCountAtAllLevels(t *testing.T) {
	pipelineRuns := []*v1.PipelineRun{
		newPipelineRun(corev1.ConditionUnknown, "testns1", "pipeline1", "pr1"),
		newPipelineRun(corev1.ConditionUnknown, "testns1", "pipeline2", "pr2"),
		newPipelineRun(corev1.ConditionUnknown, "testns1", "pipeline2", "pr3"),
		newPipelineRun(corev1.ConditionFalse, "testns1", "pipeline2", "pr4"), // Not running
		newPipelineRun(corev1.ConditionTrue, "testns1", "pipeline1", "pr5"),  // Not running
		newPipelineRun(corev1.ConditionUnknown, "testns2", "pipeline1", "pr6"),
		newPipelineRun(corev1.ConditionUnknown, "testns2", "pipeline1", "pr7"),
		newPipelineRun(corev1.ConditionUnknown, "testns2", "pipeline3", "pr8"),
	}

	for _, test := range []struct {
		name                    string
		runningPipelinerunLevel string
		expected                map[attribute.Set]int64
	}{{
		name:                    "at pipelinerun level",
		runningPipelinerunLevel: config.PipelinerunLevelAtPipelinerun,
		expected: map[attribute.Set]int64{
			attribute.NewSet(attribute.String("namespace", "testns1"), attribute.String("pipeline", "pipeline1"), attribute.String("pipelinerun", "pr1")): 1,
			attribute.NewSet(attribute.String("namespace", "testns1"), attribute.String("pipeline", "pipeline2"), attribute.String("pipelinerun", "pr2")): 1,
			attribute.NewSet(attribute.String("namespace", "testns1"), attribute.String("pipeline", "pipeline2"), attribute.String("pipelinerun", "pr3")): 1,
			attribute.NewSet(attribute.String("namespace", "testns2"), attribute.String("pipeline", "pipeline1"), attribute.String("pipelinerun", "pr6")): 1,
			attribute.NewSet(attribute.String("namespace", "testns2"), attribute.String("pipeline", "pipeline1"), attribute.String("pipelinerun", "pr7")): 1,
			attribute.NewSet(attribute.String("namespace", "testns2"), attribute.String("pipeline", "pipeline3"), attribute.String("pipelinerun", "pr8")): 1,
			attribute.NewSet(): 6,
		},
	}, {
		name:                    "at pipeline level",
		runningPipelinerunLevel: config.PipelinerunLevelAtPipeline,
		expected: map[attribute.Set]int64{
			attribute.NewSet(attribute.String("namespace", "testns1"), attribute.String("pipeline", "pipeline1")): 1,
			attribute.NewSet(attribute.String("namespace", "testns1"), attribute.String("pipeline", "pipeline2")): 2,
			attribute.NewSet(attribute.String("namespace", "testns2"), attribute.String("pipeline", "pipeline1")): 2,
			attribute.NewSet(attribute.String("namespace", "testns2"), attribute.String("pipeline", "pipeline3")): 1,
			attribute.NewSet(): 6,
		},
	}, {
		name:                    "at namespace level",
		runningPipelinerunLevel: config.PipelinerunLevelAtNS,
		expected: map[attribute.Set]int64{
			attribute.NewSet(attribute.String("namespace", "testns1")): 3,
			attribute.NewSet(attribute.String("namespace", "testns2")): 3,
			attribute.NewSet(): 6,
		},
	}, {
		name:                    "at cluster level",
		runningPipelinerunLevel: "", // cluster level
		expected: map[attribute.Set]int64{
			attribute.NewSet(): 6,
		},
	}} {
		t.Run(test.name, func(t *testing.T) {
			ctx := getConfigContextRunningPRLevel(test.runningPipelinerunLevel)
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			otel.SetMeterProvider(provider)

			// Create a new recorder with the test meter provider.
			r = &Recorder{
				initialized: true,

				meter: provider.Meter("tekton_pipelines_controller"),
			}
			cfg := config.FromContextOrDefaults(ctx)
			r.cfg = cfg.Metrics
			if err := r.configure(cfg.Metrics); err != nil {
				t.Fatalf("initializeInstruments: %v", err)
			}

			mockLister := &mockPipelineRunLister{prs: pipelineRuns}

			_, err := r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
				return r.observeRunningPipelineRuns(ctx, o, mockLister)
			}, r.runningPRsGauge, r.runningPRsWaitingOnPipelineResolutionGauge, r.runningPRsWaitingOnTaskResolutionGauge)
			if err != nil {
				t.Fatalf("Failed to register callback: %v", err)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &rm); err != nil {
				t.Fatalf("Collect error: %v", err)
			}

			if len(rm.ScopeMetrics) != 1 {
				t.Fatalf("Expected 1 scope metric, got %d", len(rm.ScopeMetrics))
			}
			if len(rm.ScopeMetrics[0].Metrics) < 1 {
				t.Fatalf("Expected at least 1 metric, got %d", len(rm.ScopeMetrics[0].Metrics))
			}

			// Find the running_pipelineruns metric
			var runningPRsMetric metricdata.Metrics
			for _, m := range rm.ScopeMetrics[0].Metrics {
				if m.Name == "tekton_pipelines_controller_running_pipelineruns" {
					runningPRsMetric = m
					break
				}
			}
			if runningPRsMetric.Name == "" {
				t.Fatal("running_pipelineruns metric not found")
			}

			got := make(map[attribute.Set]int64)
			gauge, ok := runningPRsMetric.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("metric data is not a Gauge[int64]: %T", runningPRsMetric.Data)
			}
			for _, dp := range gauge.DataPoints {
				got[dp.Attributes] = dp.Value
			}

			if d := cmp.Diff(test.expected, got); d != "" {
				t.Errorf("Metric data diff (-want, +got): %s", d)
			}
		})
	}
}

func TestRecordRunningPipelineRunsResolutionWaitCounts(t *testing.T) {
	multiplier := 3
	for _, tc := range []struct {
		status      corev1.ConditionStatus
		reason      string
		prWaitCount int64
		trWaitCount int64
	}{
		{
			status: corev1.ConditionTrue,
			reason: "",
		},
		{
			status: corev1.ConditionTrue,
			reason: v1.PipelineRunReasonResolvingPipelineRef.String(),
		},
		{
			status: corev1.ConditionTrue,
			reason: v1.TaskRunReasonResolvingTaskRef,
		},
		{
			status: corev1.ConditionFalse,
			reason: "",
		},
		{
			status: corev1.ConditionFalse,
			reason: v1.PipelineRunReasonResolvingPipelineRef.String(),
		},
		{
			status: corev1.ConditionFalse,
			reason: v1.TaskRunReasonResolvingTaskRef,
		},
		{
			status: corev1.ConditionUnknown,
			reason: "",
		},
		{
			status:      corev1.ConditionUnknown,
			reason:      v1.PipelineRunReasonResolvingPipelineRef.String(),
			prWaitCount: 3,
		},
		{
			status:      corev1.ConditionUnknown,
			reason:      v1.TaskRunReasonResolvingTaskRef,
			trWaitCount: 3,
		},
	} {
		ctx := getConfigContext(false)
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		otel.SetMeterProvider(provider)

		r = &Recorder{
			initialized: true,

			meter: provider.Meter("tekton_pipelines_controller"),
		}
		cfg := config.FromContextOrDefaults(ctx)
		r.cfg = cfg.Metrics
		if err := r.configure(cfg.Metrics); err != nil {
			t.Fatalf("initializeInstruments: %v", err)
		}

		var prs []*v1.PipelineRun
		for range multiplier {
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pipelinerun-")},
				Status: v1.PipelineRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{{
							Type:   apis.ConditionSucceeded,
							Status: tc.status,
							Reason: tc.reason,
						}},
					},
				},
			}
			prs = append(prs, pr)
		}

		mockLister := &mockPipelineRunLister{prs: prs}

		_, err := r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
			return r.observeRunningPipelineRuns(ctx, o, mockLister)
		}, r.runningPRsGauge, r.runningPRsWaitingOnPipelineResolutionGauge, r.runningPRsWaitingOnTaskResolutionGauge)
		if err != nil {
			t.Fatalf("Failed to register callback: %v", err)
		}

		var rm metricdata.ResourceMetrics
		if err := reader.Collect(ctx, &rm); err != nil {
			t.Fatalf("Collect error: %v", err)
		}

		// Check pipeline resolution wait count
		if tc.prWaitCount > 0 {
			var m metricdata.Metrics
			for _, metric := range rm.ScopeMetrics[0].Metrics {
				if metric.Name == "tekton_pipelines_controller_running_pipelineruns_waiting_on_pipeline_resolution" {
					m = metric
					break
				}
			}
			if m.Name == "" {
				t.Error("pipeline resolution wait metric not found")
			} else if gauge, ok := m.Data.(metricdata.Gauge[int64]); !ok {
				t.Errorf("metric data is not a Gauge[int64]: %T", m.Data)
			} else if len(gauge.DataPoints) > 0 && gauge.DataPoints[0].Value != tc.prWaitCount {
				t.Errorf("Expected pipeline resolution wait count %v, got %v", tc.prWaitCount, gauge.DataPoints[0].Value)
			}
		}

		// Check task resolution wait count
		if tc.trWaitCount > 0 {
			var m metricdata.Metrics
			for _, metric := range rm.ScopeMetrics[0].Metrics {
				if metric.Name == "tekton_pipelines_controller_running_pipelineruns_waiting_on_task_resolution" {
					m = metric
					break
				}
			}
			if m.Name == "" {
				t.Error("task resolution wait metric not found")
			} else if gauge, ok := m.Data.(metricdata.Gauge[int64]); !ok {
				t.Errorf("metric data is not a Gauge[int64]: %T", m.Data)
			} else if len(gauge.DataPoints) > 0 && gauge.DataPoints[0].Value != tc.trWaitCount {
				t.Errorf("Expected task resolution wait count %v, got %v", tc.trWaitCount, gauge.DataPoints[0].Value)
			}
		}
	}
}

func TestRecordRunningPipelineRunsCountZeroing(t *testing.T) {
	ctx := getConfigContextRunningPRLevel(config.PipelinerunLevelAtPipelinerun)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r = &Recorder{
		initialized: true,
		meter:       provider.Meter("tekton_pipelines_controller"),
	}
	cfg := config.FromContextOrDefaults(ctx)
	r.cfg = cfg.Metrics
	if err := r.configure(cfg.Metrics); err != nil {
		t.Fatalf("initializeInstruments: %v", err)
	}

	// 1. Start with one running PipelineRun
	pr := newPipelineRun(corev1.ConditionUnknown, "testns", "pipeline1", "pr1")
	mockLister := &mockPipelineRunLister{prs: []*v1.PipelineRun{pr}}

	_, err := r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		return r.observeRunningPipelineRuns(ctx, o, mockLister)
	}, r.runningPRsGauge, r.runningPRsWaitingOnPipelineResolutionGauge, r.runningPRsWaitingOnTaskResolutionGauge)
	if err != nil {
		t.Fatalf("Failed to register callback: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	runningPRsMetric := getRunningPRsMetric(t, rm)
	gauge, ok := runningPRsMetric.Data.(metricdata.Gauge[int64])
	if !ok {
		t.Fatalf("metric data is not a Gauge[int64]: %T", runningPRsMetric.Data)
	}

	// Verify we have 2 data points: 1 with labels, 1 without (global)
	if len(gauge.DataPoints) != 2 {
		t.Fatalf("Expected 2 data points, got %d", len(gauge.DataPoints))
	}
	foundGlobal := false
	for _, dp := range gauge.DataPoints {
		if dp.Attributes.Len() == 0 {
			foundGlobal = true
			if dp.Value != 1 {
				t.Errorf("Expected global value 1, got %d", dp.Value)
			}
		} else if dp.Value != 1 {
			t.Errorf("Expected labeled value 1, got %d", dp.Value)
		}
	}
	if !foundGlobal {
		t.Error("Global data point not found")
	}

	// 2. Clear the list (PipelineRun finished)
	mockLister.prs = []*v1.PipelineRun{}

	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	// Verify the global metric is reported as 0.
	if len(rm.ScopeMetrics) > 0 {
		for _, m := range rm.ScopeMetrics[0].Metrics {
			if m.Name == "tekton_pipelines_controller_running_pipelineruns" {
				gauge, ok := m.Data.(metricdata.Gauge[int64])
				if !ok {
					t.Errorf("metric data is not a Gauge[int64]: %T", m.Data)
					return
				}
				if len(gauge.DataPoints) != 1 {
					t.Errorf("Expected 1 data point for finished PipelineRun (global 0), got %d", len(gauge.DataPoints))
					return
				}
				dp := gauge.DataPoints[0]
				if dp.Attributes.Len() != 0 {
					t.Errorf("Expected global data point with no attributes, got %v", dp.Attributes)
				}
				if dp.Value != 0 {
					t.Errorf("Expected global value 0, got %d", dp.Value)
				}
				return
			}
		}
	}
	t.Error("running_pipelineruns metric not found")
}

func TestObserveRunningPipelineRunsListerError(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	listerErr := errors.New("lister failed")
	mockLister := &mockPipelineRunLister{err: listerErr}

	observeErr := r.observeRunningPipelineRuns(ctx, nil, mockLister)
	if observeErr == nil {
		t.Fatal("Expected error from observeRunningPipelineRuns when lister fails, got nil")
	}
	if !errors.Is(observeErr, listerErr) {
		t.Errorf("Expected lister error to be wrapped, got: %v", observeErr)
	}
}

func TestReportRunningPipelineRuns(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	mockLister := &mockPipelineRunLister{}

	// Run ReportRunningPipelineRuns in a goroutine
	done := make(chan struct{})
	go func() {
		r.ReportRunningPipelineRuns(ctx, mockLister)
		close(done)
	}()

	// Cancel context to stop the reporting loop
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("ReportRunningPipelineRuns did not exit after context cancellation")
	}
}

func TestInsertTag(t *testing.T) {
	resetMetrics()
	pipeline := "test-pipeline"
	pipelinerun := "test-pipelinerun"

	t.Run("pipelinerunInsertTag", func(t *testing.T) {
		tags := pipelinerunInsertTag(pipeline, pipelinerun)
		if len(tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(tags))
		}
	})

	t.Run("pipelineInsertTag", func(t *testing.T) {
		tags := pipelineInsertTag(pipeline, pipelinerun)
		if len(tags) != 1 {
			t.Errorf("Expected 1 tag, got %d", len(tags))
		}
	})

	t.Run("nilInsertTag", func(t *testing.T) {
		tags := nilInsertTag(pipeline, pipelinerun)
		if len(tags) != 0 {
			t.Errorf("Expected 0 tags, got %d", len(tags))
		}
	})
}

func TestGetPipelineTagName(t *testing.T) {
	tests := []struct {
		name     string
		pr       *v1.PipelineRun
		expected string
	}{
		{
			name: "with pipeline ref",
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					PipelineRef: &v1.PipelineRef{Name: "test-pipeline"},
				},
			},
			expected: "test-pipeline",
		},
		{
			name: "with pipeline spec",
			pr: &v1.PipelineRun{
				Spec: v1.PipelineRunSpec{
					PipelineSpec: &v1.PipelineSpec{},
				},
			},
			expected: "anonymous",
		},
		{
			name: "with pipeline label",
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pipeline.PipelineLabelKey: "pipeline-label",
					},
				},
			},
			expected: "pipeline-label",
		},
		{
			name:     "empty",
			pr:       &v1.PipelineRun{},
			expected: "anonymous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPipelineTagName(tt.pr); got != tt.expected {
				t.Errorf("getPipelineTagName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func getRunningPRsMetric(t *testing.T, rm metricdata.ResourceMetrics) metricdata.Metrics {
	t.Helper()
	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("Expected scope metrics, got 0")
	}
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "tekton_pipelines_controller_running_pipelineruns" {
			return m
		}
	}
	t.Fatal("running_pipelineruns metric not found")
	return metricdata.Metrics{}
}
