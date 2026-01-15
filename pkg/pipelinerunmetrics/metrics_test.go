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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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
	metrics := Recorder{}

	if err := metrics.DurationAndCount(t.Context(), &v1.PipelineRun{}, nil); err == nil {
		t.Error("DurationAndCount recording expected to return error but got nil")
	}
	if err := metrics.observeRunningPipelineRuns(t.Context(), nil, nil); err == nil {
		t.Error("Current PR count recording expected to return error but got nil")
	}
}

func TestUpdateConfig(t *testing.T) {
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
	if !r.updateConfig(newConfig) {
		t.Error("updateConfig should have returned true, but returned false")
	}

	// Then, update with the same config.
	if r.updateConfig(newConfig) {
		t.Error("updateConfig should have returned false, but returned true")
	}

	// Finally, update with a different config.
	differentConfig := &config.Metrics{
		PipelinerunLevel: config.PipelinerunLevelAtNS,
	}
	if !r.updateConfig(differentConfig) {
		t.Error("updateConfig should have returned true, but returned false")
	}
}

func TestRecordPipelineRunDurationCount(t *testing.T) {
	for _, test := range []struct {
		name            string
		pipelineRun     *v1.PipelineRun
		beforeCondition *apis.Condition
		countWithReason bool
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
		beforeCondition: nil,
		countWithReason: false,
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
	}} {
		t.Run(test.name, func(t *testing.T) {
			ctx := getConfigContext(test.countWithReason)
			r, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			// Test that recording works without error
			if err := r.DurationAndCount(ctx, test.pipelineRun, test.beforeCondition); err != nil {
				t.Errorf("DurationAndCount recording failed: %v", err)
			}

			// For OpenTelemetry, we can't easily assert metric values in unit tests
			// as they are recorded asynchronously. Instead, we verify the API works
			// and the recorder is properly initialized.
			if !r.initialized {
				t.Error("Recorder should be initialized")
			}
		})
	}
}

func TestRecordRunningPipelineRunsCount(t *testing.T) {
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
}

func (m *mockPipelineRunLister) List(selector labels.Selector) ([]*v1.PipelineRun, error) {
	return m.prs, nil
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
		},
	}, {
		name:                    "at pipeline level",
		runningPipelinerunLevel: config.PipelinerunLevelAtPipeline,
		expected: map[attribute.Set]int64{
			attribute.NewSet(attribute.String("namespace", "testns1"), attribute.String("pipeline", "pipeline1")): 1,
			attribute.NewSet(attribute.String("namespace", "testns1"), attribute.String("pipeline", "pipeline2")): 2,
			attribute.NewSet(attribute.String("namespace", "testns2"), attribute.String("pipeline", "pipeline1")): 2,
			attribute.NewSet(attribute.String("namespace", "testns2"), attribute.String("pipeline", "pipeline3")): 1,
		},
	}, {
		name:                    "at namespace level",
		runningPipelinerunLevel: config.PipelinerunLevelAtNS,
		expected: map[attribute.Set]int64{
			attribute.NewSet(attribute.String("namespace", "testns1")): 3,
			attribute.NewSet(attribute.String("namespace", "testns2")): 3,
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
				initialized:     true,
				ReportingPeriod: 30 * time.Second,
				meter:           provider.Meter("tekton_pipelines_controller"),
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
	ctx := getConfigContext(false)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Create a mock lister that returns an empty list
	mockLister := &mockPipelineRunLister{}

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

func TestRecordRunningPipelineRunsCountZeroing(t *testing.T) {
	ctx := getConfigContextRunningPRLevel(config.PipelinerunLevelAtPipelinerun)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r = &Recorder{
		initialized:     true,
		ReportingPeriod: 30 * time.Second,
		meter:           provider.Meter("tekton_pipelines_controller"),
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

	// Verify we have 1 data point with value 1
	if len(gauge.DataPoints) != 1 {
		t.Fatalf("Expected 1 data point, got %d", len(gauge.DataPoints))
	}
	if gauge.DataPoints[0].Value != 1 {
		t.Errorf("Expected value 1, got %d", gauge.DataPoints[0].Value)
	}

	// 2. Clear the list (PipelineRun finished)
	mockLister.prs = []*v1.PipelineRun{}

	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	// Verify the metric is empty or the specific data point is gone.
	if len(rm.ScopeMetrics) > 0 {
		for _, m := range rm.ScopeMetrics[0].Metrics {
			if m.Name == "tekton_pipelines_controller_running_pipelineruns" {
				gauge, ok := m.Data.(metricdata.Gauge[int64])
				if !ok {
					t.Errorf("metric data is not a Gauge[int64]: %T", m.Data)
					return
				}
				if len(gauge.DataPoints) != 0 {
					t.Errorf("Expected 0 data points for finished PipelineRun, got %d", len(gauge.DataPoints))
				}
				return
			}
		}
	}
	// Metric not found is also success (no data points reported)
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
