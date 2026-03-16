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

package taskrunmetrics

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
	"github.com/tektoncd/pipeline/pkg/pod"
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

func getConfigContext(countWithReason, throttleWithNamespace bool) context.Context {
	ctx := context.Background()
	cfg := &config.Config{
		Metrics: &config.Metrics{
			TaskrunLevel:            config.TaskrunLevelAtTaskrun,
			PipelinerunLevel:        config.PipelinerunLevelAtPipelinerun,
			DurationTaskrunType:     config.DefaultDurationTaskrunType,
			DurationPipelinerunType: config.DefaultDurationPipelinerunType,
			CountWithReason:         countWithReason,
			ThrottleWithNamespace:   throttleWithNamespace,
		},
	}
	return config.ToContext(ctx, cfg)
}

func TestUninitializedMetrics(t *testing.T) {
	resetMetrics()
	r := &Recorder{}

	beforeCondition := &apis.Condition{
		Type:   apis.ConditionReady,
		Status: corev1.ConditionUnknown,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := r.DurationAndCount(ctx, &v1.TaskRun{}, beforeCondition); err == nil {
		t.Error("DurationCount recording expected to return error but got nil")
	}
	if err := r.observeRunningTaskRuns(ctx, nil, nil); err == nil {
		t.Error("Current TaskRunsCount recording expected to return error but got nil")
	}
	if err := r.RecordPodLatency(ctx, &corev1.Pod{}, &v1.TaskRun{}); err == nil {
		t.Error("Pod Latency recording expected to return error but got nil")
	}
}

func TestDurationAndCountNilStartTime(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task-1"},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			// StartTime deliberately nil
		},
	}

	if err := r.DurationAndCount(ctx, tr, nil); err != nil {
		t.Errorf("DurationAndCount with nil StartTime returned error: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	// Duration should be recorded as 0
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_taskrun_duration_seconds" {
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

func TestDurationAndCountCancelledTaskRun(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task-1"},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
					Reason: v1.TaskRunReasonCancelled.String(),
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
	}

	if err := r.DurationAndCount(ctx, tr, nil); err != nil {
		t.Fatalf("DurationAndCount: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_taskrun_total" {
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok {
					t.Fatalf("total metric data is not Sum[int64]: %T", m.Data)
				}
				if len(sum.DataPoints) != 1 {
					t.Fatalf("Expected 1 total data point, got %d", len(sum.DataPoints))
				}
				gotStatus := ""
				for _, kv := range sum.DataPoints[0].Attributes.ToSlice() {
					if kv.Key == "status" {
						gotStatus = kv.Value.AsString()
					}
				}
				if gotStatus != "cancelled" {
					t.Errorf("Expected status=cancelled, got %q", gotStatus)
				}
				return
			}
		}
	}
	t.Error("taskrun_total metric not found")
}

func TestDurationAndCountNoopWhenConditionUnchanged(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	condition := &apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	}
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
		Spec:       v1.TaskRunSpec{TaskRef: &v1.TaskRef{Name: "task-1"}},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{*condition},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
	}

	// beforeCondition == afterCondition — should be a no-op
	if err := r.DurationAndCount(ctx, tr, condition); err != nil {
		t.Errorf("DurationAndCount returned unexpected error: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	if len(rm.ScopeMetrics) != 0 {
		t.Errorf("Expected no metrics recorded for unchanged condition, got %d scope metrics", len(rm.ScopeMetrics))
	}
}

func TestDurationAndCountGaugeDurationTypeInPipelineRun(t *testing.T) {
	resetMetrics()
	ctx := context.Background()
	cfg := &config.Config{
		Metrics: &config.Metrics{
			TaskrunLevel:        config.TaskrunLevelAtTaskrun,
			PipelinerunLevel:    config.PipelinerunLevelAtPipelinerun,
			DurationTaskrunType: config.DurationTaskrunTypeLastValue,
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

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrun-1",
			Namespace: "ns",
			Labels: map[string]string{
				pipeline.PipelineLabelKey:    "pipeline-1",
				pipeline.PipelineRunLabelKey: "pipelinerun-1",
			},
		},
		Spec: v1.TaskRunSpec{TaskRef: &v1.TaskRef{Name: "task-1"}},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
	}

	if err := r.DurationAndCount(ctx, tr, nil); err != nil {
		t.Fatalf("DurationAndCount: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds" {
				if _, ok := m.Data.(metricdata.Gauge[float64]); !ok {
					t.Errorf("Expected Gauge[float64] for pipelinerun taskrun LastValue duration type, got %T", m.Data)
				}
				return
			}
		}
	}
	t.Error("pipelinerun_taskrun duration metric not found")
}

func TestDurationAndCountGaugeDurationType(t *testing.T) {
	resetMetrics()
	ctx := context.Background()
	cfg := &config.Config{
		Metrics: &config.Metrics{
			TaskrunLevel:        config.TaskrunLevelAtTaskrun,
			PipelinerunLevel:    config.PipelinerunLevelAtPipelinerun,
			DurationTaskrunType: config.DurationTaskrunTypeLastValue,
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

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
		Spec:       v1.TaskRunSpec{TaskRef: &v1.TaskRef{Name: "task-1"}},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
	}

	if err := r.DurationAndCount(ctx, tr, nil); err != nil {
		t.Fatalf("DurationAndCount: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_taskrun_duration_seconds" {
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
	ctx := getConfigContext(false, false)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	logger := zap.NewExample().Sugar()

	// An invalid TaskrunLevel causes configure to return an error.
	// OnStore should log it but not corrupt the recorder state.
	invalidCfg := &config.Metrics{
		TaskrunLevel: "invalid-level",
	}
	OnStore(logger, r)(config.GetMetricsConfigName(), invalidCfg)

	if !r.initialized {
		t.Error("recorder should remain initialized after a failed configure")
	}
}

func TestOnStore(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false, false)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// create a logger
	logger := zap.NewExample().Sugar()

	t.Run("wrong name", func(t *testing.T) {
		OnStore(logger, r)("wrong-name", &config.Metrics{TaskrunLevel: config.TaskrunLevelAtNS})
		if r.cfg.TaskrunLevel != config.TaskrunLevelAtTaskrun {
			t.Error("OnStore should not have updated config")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		OnStore(logger, r)(config.GetMetricsConfigName(), &config.Store{})
		if r.cfg.TaskrunLevel != config.TaskrunLevelAtTaskrun {
			t.Error("OnStore should not have updated config")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		newConfig := &config.Metrics{
			TaskrunLevel: config.TaskrunLevelAtTask,
		}
		OnStore(logger, r)(config.GetMetricsConfigName(), newConfig)
		if r.cfg.TaskrunLevel != config.TaskrunLevelAtTask {
			t.Error("OnStore should have updated config")
		}
	})
}

func TestUpdateConfig(t *testing.T) {
	resetMetrics()
	// Test that the config is updated when it changes, and not when it doesn't.
	ctx := getConfigContext(false, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// First, update with a new config.
	newConfig := &config.Metrics{
		TaskrunLevel: config.TaskrunLevelAtTask,
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
		TaskrunLevel: config.TaskrunLevelAtNS,
	}
	if r.updateConfig(differentConfig) == nil {
		t.Error("updateConfig should have returned the new config, but returned nil")
	}
}

func TestRecordTaskRunDurationCount(t *testing.T) {
	for _, c := range []struct {
		name             string
		taskRun          *v1.TaskRun
		beforeCondition  *apis.Condition
		countWithReason  bool
		taskrunLevel     string
		pipelinerunLevel string
		expectedTags     map[string]string
		expectedCount    int64
		expectedDuration float64
	}{{
		name: "for succeeded taskrun",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task-1"},
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: nil,
		countWithReason: false,
		taskrunLevel:    config.TaskrunLevelAtTaskrun,
		expectedTags: map[string]string{
			"task":      "task-1",
			"taskrun":   "taskrun-1",
			"namespace": "ns",
			"status":    "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for succeeded taskrun at task level",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task-1"},
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: nil,
		countWithReason: false,
		taskrunLevel:    config.TaskrunLevelAtTask,
		expectedTags: map[string]string{
			"task":      "task-1",
			"namespace": "ns",
			"status":    "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for succeeded taskrun at namespace level",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task-1"},
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: nil,
		countWithReason: false,
		taskrunLevel:    config.TaskrunLevelAtNS,
		expectedTags: map[string]string{
			"namespace": "ns",
			"status":    "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for succeeded taskrun in pipelinerun",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrun-1", Namespace: "ns",
				Labels: map[string]string{
					pipeline.PipelineLabelKey:    "pipeline-1",
					pipeline.PipelineRunLabelKey: "pipelinerun-1",
				},
			},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task-1"},
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition:  nil,
		countWithReason:  false,
		taskrunLevel:     config.TaskrunLevelAtTaskrun,
		pipelinerunLevel: config.PipelinerunLevelAtPipelinerun,
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"task":        "task-1",
			"taskrun":     "taskrun-1",
			"namespace":   "ns",
			"status":      "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}, {
		name: "for succeeded taskrun ref cluster task",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns", Labels: map[string]string{
				pipeline.PipelineTaskLabelKey: "task-1",
			}},
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{},
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		beforeCondition: nil,
		countWithReason: false,
		expectedTags: map[string]string{
			"task":      "task-1",
			"taskrun":   "taskrun-1",
			"namespace": "ns",
			"status":    "success",
		},
		expectedCount:    1,
		expectedDuration: 60,
	}} {
		t.Run(c.name, func(t *testing.T) {
			resetMetrics()
			ctx := getConfigContext(c.countWithReason, false)
			if c.taskrunLevel != "" || c.pipelinerunLevel != "" {
				cfg := config.FromContextOrDefaults(ctx)
				if c.taskrunLevel != "" {
					cfg.Metrics.TaskrunLevel = c.taskrunLevel
				}
				if c.pipelinerunLevel != "" {
					cfg.Metrics.PipelinerunLevel = c.pipelinerunLevel
				}
				ctx = config.ToContext(ctx, cfg)
			}
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			otel.SetMeterProvider(provider)

			metrics, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			if err := metrics.DurationAndCount(ctx, c.taskRun, c.beforeCondition); err != nil {
				t.Errorf("DurationAndCount: %v", err)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &rm); err != nil {
				t.Fatalf("Collect error: %v", err)
			}

			if len(rm.ScopeMetrics) != 1 {
				t.Fatalf("Expected 1 scope metric, got %d", len(rm.ScopeMetrics))
			}

			// Check duration
			var durationMetric metricdata.Metrics
			var totalMetric metricdata.Metrics

			for _, m := range rm.ScopeMetrics[0].Metrics {
				if m.Name == "tekton_pipelines_controller_taskrun_duration_seconds" || m.Name == "tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds" {
					durationMetric = m
				}
				if m.Name == "tekton_pipelines_controller_taskrun_total" {
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
						if dp.Sum != c.expectedDuration {
							t.Errorf("Expected duration sum %v, got %v", c.expectedDuration, dp.Sum)
						}

						// Verify attributes
						gotAttrs := make(map[string]string)
						for _, kv := range dp.Attributes.ToSlice() {
							gotAttrs[string(kv.Key)] = kv.Value.AsString()
						}
						if d := cmp.Diff(c.expectedTags, gotAttrs); d != "" {
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
						if dp.Value != c.expectedCount {
							t.Errorf("Expected total count %v, got %v", c.expectedCount, dp.Value)
						}

						// Verify attributes for count (should only have status)
						expectedCountTags := map[string]string{"status": c.expectedTags["status"]}
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

// mockTaskRunLister implements the listers.TaskRunLister interface.
type mockTaskRunLister struct {
	trs []*v1.TaskRun
	err error
}

func (m *mockTaskRunLister) List(selector labels.Selector) ([]*v1.TaskRun, error) {
	return m.trs, m.err
}

func (m *mockTaskRunLister) TaskRuns(namespace string) listers.TaskRunNamespaceLister {
	// This function is not needed for these tests.
	return nil
}

func newTaskRun(status corev1.ConditionStatus, reason, namespace string) *v1.TaskRun {
	return &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("taskrun-"), Namespace: namespace},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: status,
					Reason: reason,
				}},
			},
		},
	}
}

func TestRecordRunningTaskRunsCount(t *testing.T) {
	taskRuns := []*v1.TaskRun{
		newTaskRun(corev1.ConditionTrue, "", "testns1"),
		newTaskRun(corev1.ConditionUnknown, "", "testns1"),
		newTaskRun(corev1.ConditionFalse, "", "testns2"),
	}

	once = sync.Once{} // Use resetMetrics() ideally but let's keep it consistent
	resetMetrics()
	ctx := getConfigContext(false, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	mockLister := &mockTaskRunLister{trs: taskRuns}

	_, err = r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		return r.observeRunningTaskRuns(ctx, o, mockLister)
	}, r.runningTRsGauge, r.runningTRsWaitingOnTaskResolutionGauge, r.runningTRsThrottledByQuotaGauge, r.runningTRsThrottledByNodeGauge)
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

	// Find the running_taskruns metric
	var runningTRsMetric metricdata.Metrics
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if m.Name == "tekton_pipelines_controller_running_taskruns" {
			runningTRsMetric = m
			break
		}
	}
	if runningTRsMetric.Name == "" {
		t.Fatal("running_taskruns metric not found")
	}

	gauge, ok := runningTRsMetric.Data.(metricdata.Gauge[int64])
	if !ok {
		t.Fatalf("metric data is not a Gauge[int64]: %T", runningTRsMetric.Data)
	}
	if len(gauge.DataPoints) != 1 {
		t.Fatalf("Expected 1 data point, got %d", len(gauge.DataPoints))
	}
	if gauge.DataPoints[0].Value != 1 {
		t.Errorf("Expected 1 running taskrun, got %d", gauge.DataPoints[0].Value)
	}
}

func TestRecordRunningTaskRunsThrottledCounts(t *testing.T) {
	taskRuns := []*v1.TaskRun{
		// Throttled by quota
		newTaskRun(corev1.ConditionUnknown, pod.ReasonExceededResourceQuota, "testns1"),
		newTaskRun(corev1.ConditionUnknown, pod.ReasonExceededResourceQuota, "testns1"),
		newTaskRun(corev1.ConditionUnknown, pod.ReasonExceededResourceQuota, "testns2"),
		// Throttled by node
		newTaskRun(corev1.ConditionUnknown, pod.ReasonExceededNodeResources, "testns2"),
		newTaskRun(corev1.ConditionUnknown, pod.ReasonExceededNodeResources, "testns3"),
		// Not running
		newTaskRun(corev1.ConditionTrue, "", "testns1"),
		newTaskRun(corev1.ConditionFalse, pod.ReasonExceededResourceQuota, "testns2"),
	}

	for _, tc := range []struct {
		name                  string
		throttleWithNamespace bool
		expectedQuota         map[attribute.Set]int64
		expectedNode          map[attribute.Set]int64
	}{{
		name:                  "without namespace",
		throttleWithNamespace: false,
		expectedQuota: map[attribute.Set]int64{
			attribute.NewSet(): 3,
		},
		expectedNode: map[attribute.Set]int64{
			attribute.NewSet(): 2,
		},
	}, {
		name:                  "with namespace",
		throttleWithNamespace: true,
		expectedQuota: map[attribute.Set]int64{
			attribute.NewSet(attribute.String("namespace", "testns1")): 2,
			attribute.NewSet(attribute.String("namespace", "testns2")): 1,
		},
		expectedNode: map[attribute.Set]int64{
			attribute.NewSet(attribute.String("namespace", "testns2")): 1,
			attribute.NewSet(attribute.String("namespace", "testns3")): 1,
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			resetMetrics()
			ctx := getConfigContext(false, tc.throttleWithNamespace)
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			otel.SetMeterProvider(provider)

			r, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			mockLister := &mockTaskRunLister{trs: taskRuns}

			_, err = r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
				return r.observeRunningTaskRuns(ctx, o, mockLister)
			}, r.runningTRsGauge, r.runningTRsWaitingOnTaskResolutionGauge, r.runningTRsThrottledByQuotaGauge, r.runningTRsThrottledByNodeGauge)
			if err != nil {
				t.Fatalf("Failed to register callback: %v", err)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &rm); err != nil {
				t.Fatalf("Collect error: %v", err)
			}

			gotQuota := make(map[attribute.Set]int64)
			gotNode := make(map[attribute.Set]int64)

			for _, m := range rm.ScopeMetrics[0].Metrics {
				gauge, ok := m.Data.(metricdata.Gauge[int64])
				if !ok {
					continue
				}
				switch m.Name {
				case "tekton_pipelines_controller_running_taskruns_throttled_by_quota":
					for _, dp := range gauge.DataPoints {
						gotQuota[dp.Attributes] = dp.Value
					}
				case "tekton_pipelines_controller_running_taskruns_throttled_by_node":
					for _, dp := range gauge.DataPoints {
						gotNode[dp.Attributes] = dp.Value
					}
				}
			}

			if d := cmp.Diff(tc.expectedQuota, gotQuota); d != "" {
				t.Errorf("Quota metrics diff (-want, +got): %s", d)
			}
			if d := cmp.Diff(tc.expectedNode, gotNode); d != "" {
				t.Errorf("Node metrics diff (-want, +got): %s", d)
			}
		})
	}
}

func TestConfigurePipelinerunLevels(t *testing.T) {
	for _, tc := range []struct {
		name             string
		pipelinerunLevel string
	}{{
		name:             "at pipeline level",
		pipelinerunLevel: config.PipelinerunLevelAtPipeline,
	}, {
		name:             "at namespace level",
		pipelinerunLevel: config.PipelinerunLevelAtNS,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			resetMetrics()
			ctx := getConfigContext(false, false)
			r, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}
			cfg := &config.Metrics{
				TaskrunLevel:     config.TaskrunLevelAtTask,
				PipelinerunLevel: tc.pipelinerunLevel,
			}
			if err := r.configure(cfg); err != nil {
				t.Errorf("configure with PipelinerunLevel=%s failed: %v", tc.pipelinerunLevel, err)
			}
		})
	}
}

func TestDurationAndCountWithReason(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(true, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
		Spec:       v1.TaskRunSpec{TaskRef: &v1.TaskRef{Name: "task-1"}},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
					Reason: "TaskRunImagePullFailed",
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
			},
		},
	}

	if err := r.DurationAndCount(ctx, tr, nil); err != nil {
		t.Fatalf("DurationAndCount: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_taskrun_duration_seconds" {
				hist, ok := m.Data.(metricdata.Histogram[float64])
				if !ok {
					t.Fatalf("Expected Histogram[float64], got %T", m.Data)
				}
				if len(hist.DataPoints) == 0 {
					t.Fatal("No data points in taskrun_duration_seconds")
				}
				for _, kv := range hist.DataPoints[0].Attributes.ToSlice() {
					if string(kv.Key) == "reason" {
						return // reason attribute present in duration metric
					}
				}
				t.Error("Expected 'reason' attribute in taskrun_duration_seconds when CountWithReason=true")
				return
			}
		}
	}
	t.Error("taskrun_duration_seconds metric not found")
}

func TestObserveRunningTaskRunsResolvingTaskRef(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: v1.TaskRunReasonResolvingTaskRef,
				}},
			},
		},
	}
	mockLister := &mockTaskRunLister{trs: []*v1.TaskRun{tr}}

	_, err = r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		return r.observeRunningTaskRuns(ctx, o, mockLister)
	}, r.runningTRsGauge, r.runningTRsWaitingOnTaskResolutionGauge, r.runningTRsThrottledByQuotaGauge, r.runningTRsThrottledByNodeGauge)
	if err != nil {
		t.Fatalf("RegisterCallback: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_pipelines_controller_running_taskruns_waiting_on_task_resolution_count" {
				gauge, ok := m.Data.(metricdata.Gauge[int64])
				if !ok {
					t.Fatalf("Expected Gauge[int64], got %T", m.Data)
				}
				if len(gauge.DataPoints) == 0 || gauge.DataPoints[0].Value != 1 {
					t.Errorf("Expected wait count=1, got %v", gauge.DataPoints)
				}
				return
			}
		}
	}
	t.Error("waiting_on_task_resolution metric not found")
}

func TestObserveRunningTaskRunsListerError(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false, false)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	listerErr := errors.New("lister failed")
	mockLister := &mockTaskRunLister{err: listerErr}

	observeErr := r.observeRunningTaskRuns(ctx, nil, mockLister)
	if observeErr == nil {
		t.Fatal("Expected error from observeRunningTaskRuns when lister fails, got nil")
	}
	if !errors.Is(observeErr, listerErr) {
		t.Errorf("Expected lister error to be wrapped, got: %v", observeErr)
	}
}

func TestReportRunningTaskRuns(t *testing.T) {
	resetMetrics()
	ctx := getConfigContext(false, false)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	mockLister := &mockTaskRunLister{}

	// Run ReportRunningTaskRuns in a goroutine
	done := make(chan struct{})
	go func() {
		r.ReportRunningTaskRuns(ctx, mockLister)
		close(done)
	}()

	// Cancel context to stop the reporting loop
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("ReportRunningTaskRuns did not exit after context cancellation")
	}
}

func TestRecordPodLatency(t *testing.T) {
	creationTime := metav1.Now()

	taskRun := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "test-taskrun", Namespace: "foo"},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{Name: "task-1"},
		},
	}
	for _, td := range []struct {
		name           string
		pod            *corev1.Pod
		expectingError bool
		taskRun        *v1.TaskRun
	}{{
		name: "for scheduled pod",
		pod: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-taskrun-pod-123456",
				Namespace:         "foo",
				CreationTimestamp: creationTime,
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{{
					Type:               corev1.PodScheduled,
					LastTransitionTime: metav1.Time{Time: creationTime.Add(4 * time.Second)},
				}},
			},
		},
		taskRun: taskRun,
	}, {
		name: "for non scheduled pod",
		pod: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-taskrun-pod-123456",
				Namespace:         "foo",
				CreationTimestamp: creationTime,
			},
			Status: corev1.PodStatus{},
		},
		expectingError: true,
		taskRun:        taskRun,
	}} {
		t.Run(td.name, func(t *testing.T) {
			resetMetrics()
			ctx := getConfigContext(false, false)
			metrics, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			if err := metrics.RecordPodLatency(ctx, td.pod, td.taskRun); td.expectingError && err == nil {
				t.Error("RecordPodLatency wanted error, got nil")
			} else if !td.expectingError {
				if err != nil {
					t.Errorf("RecordPodLatency: %v", err)
				}
			}
		})
	}
}

func TestTaskRunIsOfPipelinerun(t *testing.T) {
	tests := []struct {
		name                  string
		tr                    *v1.TaskRun
		expectedValue         bool
		expetectedPipeline    string
		expetectedPipelineRun string
	}{{
		name: "yes",
		tr: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					pipeline.PipelineLabelKey:    "pipeline",
					pipeline.PipelineRunLabelKey: "pipelinerun",
				},
			},
		},
		expectedValue:         true,
		expetectedPipeline:    "pipeline",
		expetectedPipelineRun: "pipelinerun",
	}, {
		name:          "no",
		tr:            &v1.TaskRun{},
		expectedValue: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, pipeline, pipelineRun := IsPartOfPipeline(test.tr)
			if value != test.expectedValue {
				t.Fatalf("Expecting %v got %v", test.expectedValue, value)
			}

			if pipeline != test.expetectedPipeline {
				t.Fatalf("Mismatch in pipeline: got %s expected %s", pipeline, test.expetectedPipeline)
			}

			if pipelineRun != test.expetectedPipelineRun {
				t.Fatalf("Mismatch in pipelinerun: got %s expected %s", pipelineRun, test.expetectedPipelineRun)
			}
		})
	}
}

func TestGetTaskTagName(t *testing.T) {
	tests := []struct {
		name     string
		tr       *v1.TaskRun
		expected string
	}{
		{
			name: "with task ref",
			tr: &v1.TaskRun{
				Spec: v1.TaskRunSpec{
					TaskRef: &v1.TaskRef{Name: "test-task"},
				},
			},
			expected: "test-task",
		},
		{
			name: "with task spec and pipeline task label",
			tr: &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pipeline.PipelineTaskLabelKey: "pipeline-task",
					},
				},
				Spec: v1.TaskRunSpec{
					TaskSpec: &v1.TaskSpec{},
				},
			},
			expected: "pipeline-task",
		},
		{
			name: "with task label",
			tr: &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pipeline.TaskLabelKey: "task-label",
					},
				},
			},
			expected: "task-label",
		},
		{
			name:     "empty",
			tr:       &v1.TaskRun{},
			expected: "anonymous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTaskTagName(tt.tr); got != tt.expected {
				t.Errorf("getTaskTagName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInsertTags(t *testing.T) {
	resetMetrics()
	pipelineName := "test-pipeline"
	pipelinerunName := "test-pipelinerun"
	taskName := "test-task"
	taskrunName := "test-taskrun"

	t.Run("pipelinerunInsertTag", func(t *testing.T) {
		tags := pipelinerunInsertTag(pipelineName, pipelinerunName)
		if len(tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(tags))
		}
	})

	t.Run("pipelineInsertTag", func(t *testing.T) {
		tags := pipelineInsertTag(pipelineName, pipelinerunName)
		if len(tags) != 1 {
			t.Errorf("Expected 1 tag, got %d", len(tags))
		}
	})

	t.Run("taskrunInsertTag", func(t *testing.T) {
		tags := taskrunInsertTag(taskName, taskrunName)
		if len(tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(tags))
		}
	})

	t.Run("taskInsertTag", func(t *testing.T) {
		tags := taskInsertTag(taskName, taskrunName)
		if len(tags) != 1 {
			t.Errorf("Expected 1 tag, got %d", len(tags))
		}
	})

	t.Run("nilInsertTag", func(t *testing.T) {
		tags := nilInsertTag(taskName, taskrunName)
		if len(tags) != 0 {
			t.Errorf("Expected 0 tags, got %d", len(tags))
		}
	})
}
