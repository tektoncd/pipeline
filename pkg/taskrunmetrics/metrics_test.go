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
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	_ "knative.dev/pkg/metrics/testing"
)

var (
	startTime      = metav1.Now()
	completionTime = metav1.NewTime(startTime.Time.Add(time.Minute))
)

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
	if err := r.RunningTaskRuns(ctx, nil); err == nil {
		t.Error("Current TaskRunsCount recording expected to return error but got nil")
	}
	if err := r.RecordPodLatency(ctx, &corev1.Pod{}, &v1.TaskRun{}); err == nil {
		t.Error("Pod Latency recording expected to return error but got nil")
	}
}

func TestUpdateConfig(t *testing.T) {
	// Test that the config is updated when it changes, and not when it doesn't.
	ctx := getConfigContext(false, false)
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// First, update with a new config.
	newConfig := &config.Metrics{
		TaskrunLevel: config.TaskrunLevelAtTask,
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
		TaskrunLevel: config.TaskrunLevelAtNS,
	}
	if !r.updateConfig(differentConfig) {
		t.Error("updateConfig should have returned true, but returned false")
	}
}

func TestRecordTaskRunDurationCount(t *testing.T) {
	for _, c := range []struct {
		name            string
		taskRun         *v1.TaskRun
		beforeCondition *apis.Condition
		countWithReason bool
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
	}} {
		t.Run(c.name, func(t *testing.T) {
			ctx := getConfigContext(c.countWithReason, false)
			metrics, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			if err := metrics.DurationAndCount(ctx, c.taskRun, c.beforeCondition); err != nil {
				t.Errorf("DurationAndCount: %v", err)
			}
		})
	}
}

// mockTaskRunLister implements the listers.TaskRunLister interface.
type mockTaskRunLister struct {
	trs []*v1.TaskRun
}

func (m *mockTaskRunLister) List(selector labels.Selector) ([]*v1.TaskRun, error) {
	return m.trs, nil
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

	once = sync.Once{}
	ctx := getConfigContext(false, false)
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	mockLister := &mockTaskRunLister{trs: taskRuns}

	if err := r.RunningTaskRuns(ctx, mockLister); err != nil {
		t.Errorf("RunningTaskRuns recording failed: %v", err)
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

	sum, ok := runningTRsMetric.Data.(metricdata.Sum[float64])
	if !ok {
		t.Fatalf("metric data is not a Sum[float64]: %T", runningTRsMetric.Data)
	}
	if len(sum.DataPoints) != 1 {
		t.Fatalf("Expected 1 data point, got %d", len(sum.DataPoints))
	}
	if sum.DataPoints[0].Value != 1 {
		t.Errorf("Expected 1 running taskrun, got %f", sum.DataPoints[0].Value)
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
		expectedQuota         map[attribute.Set]float64
		expectedNode          map[attribute.Set]float64
	}{{
		name:                  "without namespace",
		throttleWithNamespace: false,
		expectedQuota: map[attribute.Set]float64{
			attribute.NewSet(): 3,
		},
		expectedNode: map[attribute.Set]float64{
			attribute.NewSet(): 2,
		},
	}, {
		name:                  "with namespace",
		throttleWithNamespace: true,
		expectedQuota: map[attribute.Set]float64{
			attribute.NewSet(attribute.String("namespace", "testns1")): 2,
			attribute.NewSet(attribute.String("namespace", "testns2")): 1,
		},
		expectedNode: map[attribute.Set]float64{
			attribute.NewSet(attribute.String("namespace", "testns2")): 1,
			attribute.NewSet(attribute.String("namespace", "testns3")): 1,
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			once = sync.Once{}
			ctx := getConfigContext(false, tc.throttleWithNamespace)
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))
			otel.SetMeterProvider(provider)

			r, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			mockLister := &mockTaskRunLister{trs: taskRuns}

			if err := r.RunningTaskRuns(ctx, mockLister); err != nil {
				t.Errorf("RunningTaskRuns recording failed: %v", err)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(ctx, &rm); err != nil {
				t.Fatalf("Collect error: %v", err)
			}

			gotQuota := make(map[attribute.Set]float64)
			gotNode := make(map[attribute.Set]float64)

			for _, m := range rm.ScopeMetrics[0].Metrics {
				sum, ok := m.Data.(metricdata.Sum[float64])
				if !ok {
					continue
				}
				switch m.Name {
				case "tekton_pipelines_controller_running_taskruns_throttled_by_quota":
					for _, dp := range sum.DataPoints {
						gotQuota[dp.Attributes] = dp.Value
					}
				case "tekton_pipelines_controller_running_taskruns_throttled_by_node":
					for _, dp := range sum.DataPoints {
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
