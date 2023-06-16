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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	faketaskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/taskrun/fake"
	"github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/pkg/pod"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/metrics/metricstest"
	_ "knative.dev/pkg/metrics/testing"
)

var (
	startTime      = metav1.Now()
	completionTime = metav1.NewTime(startTime.Time.Add(time.Minute))
)

func getConfigContext() context.Context {
	ctx := context.Background()
	cfg := &config.Config{
		Metrics: &config.Metrics{
			TaskrunLevel:            config.TaskrunLevelAtTaskrun,
			PipelinerunLevel:        config.PipelinerunLevelAtPipelinerun,
			DurationTaskrunType:     config.DefaultDurationTaskrunType,
			DurationPipelinerunType: config.DefaultDurationPipelinerunType,
		},
	}
	return config.ToContext(ctx, cfg)
}

func TestUninitializedMetrics(t *testing.T) {
	metrics := Recorder{}

	beforeCondition := &apis.Condition{
		Type:   apis.ConditionReady,
		Status: corev1.ConditionUnknown,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := metrics.DurationAndCount(ctx, &v1.TaskRun{}, beforeCondition); err == nil {
		t.Error("DurationCount recording expected to return error but got nil")
	}
	if err := metrics.RunningTaskRuns(ctx, nil); err == nil {
		t.Error("Current TaskRunsCount recording expected to return error but got nil")
	}
	if err := metrics.RecordPodLatency(ctx, nil, nil); err == nil {
		t.Error("Pod Latency recording expected to return error but got nil")
	}
}

func TestMetricsOnStore(t *testing.T) {
	log := zap.NewExample()
	defer log.Sync()
	logger := log.Sugar()

	ctx := getConfigContext()
	metrics, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// We check that there's no change when incorrect config is passed
	MetricsOnStore(logger)(config.GetMetricsConfigName(), &config.Store{})
	// Comparing function assign to struct with the one which should yield same value
	if reflect.ValueOf(metrics.insertTaskTag).Pointer() != reflect.ValueOf(taskrunInsertTag).Pointer() {
		t.Fatalf("metrics recorder shouldn't change during this OnStore call")
	}

	// Config shouldn't change when incorrect config map is pass
	cfg := &config.Metrics{
		TaskrunLevel:            "foo",
		PipelinerunLevel:        "bar",
		DurationTaskrunType:     config.DurationTaskrunTypeHistogram,
		DurationPipelinerunType: config.DurationPipelinerunTypeLastValue,
	}

	// We test that there's no change when incorrect values in configmap is passed
	MetricsOnStore(logger)(config.GetMetricsConfigName(), cfg)
	// Comparing function assign to struct with the one which should yield same value
	if reflect.ValueOf(metrics.insertTaskTag).Pointer() != reflect.ValueOf(taskrunInsertTag).Pointer() {
		t.Fatalf("metrics recorder shouldn't change during this OnStore call")
	}

	// We test when we pass correct config
	cfg = &config.Metrics{
		TaskrunLevel:            config.TaskrunLevelAtNS,
		PipelinerunLevel:        config.PipelinerunLevelAtNS,
		DurationTaskrunType:     config.DurationTaskrunTypeHistogram,
		DurationPipelinerunType: config.DurationPipelinerunTypeLastValue,
	}

	MetricsOnStore(logger)(config.GetMetricsConfigName(), cfg)
	if reflect.ValueOf(metrics.insertTaskTag).Pointer() != reflect.ValueOf(nilInsertTag).Pointer() {
		t.Fatalf("metrics recorder didn't change during OnStore call")
	}
}

func TestRecordTaskRunDurationCount(t *testing.T) {
	for _, c := range []struct {
		name                 string
		taskRun              *v1.TaskRun
		metricName           string // "taskrun_duration_seconds" or "pipelinerun_taskrun_duration_seconds"
		expectedDurationTags map[string]string
		expectedCountTags    map[string]string
		expectedDuration     float64
		expectedCount        int64
		beforeCondition      *apis.Condition
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
		metricName: "taskrun_duration_seconds",
		expectedDurationTags: map[string]string{
			"task":      "task-1",
			"taskrun":   "taskrun-1",
			"namespace": "ns",
			"status":    "success",
		},
		expectedCountTags: map[string]string{
			"status": "success",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
	}, {
		name: "for succeeded taskrun with before condition",
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
		metricName: "taskrun_duration_seconds",
		expectedDurationTags: map[string]string{
			"task":      "task-1",
			"taskrun":   "taskrun-1",
			"namespace": "ns",
			"status":    "success",
		},
		expectedCountTags: map[string]string{
			"status": "success",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition: &apis.Condition{
			Type:   apis.ConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "for succeeded taskrun recount",
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
		metricName:           "taskrun_duration_seconds",
		expectedDurationTags: nil,
		expectedCountTags:    nil,
		expectedDuration:     0,
		expectedCount:        0,
		beforeCondition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "for failed taskrun",
		taskRun: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "taskrun-1", Namespace: "ns"},
			Spec: v1.TaskRunSpec{
				TaskRef: &v1.TaskRef{Name: "task-1"},
			},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		metricName: "taskrun_duration_seconds",
		expectedDurationTags: map[string]string{
			"task":      "task-1",
			"taskrun":   "taskrun-1",
			"namespace": "ns",
			"status":    "failed",
		},
		expectedCountTags: map[string]string{
			"status": "failed",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
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
		metricName: "pipelinerun_taskrun_duration_seconds",
		expectedDurationTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"task":        "task-1",
			"taskrun":     "taskrun-1",
			"namespace":   "ns",
			"status":      "success",
		},
		expectedCountTags: map[string]string{
			"status": "success",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
	}, {
		name: "for failed taskrun in pipelinerun",
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
						Status: corev1.ConditionFalse,
					}},
				},
				TaskRunStatusFields: v1.TaskRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		metricName: "pipelinerun_taskrun_duration_seconds",
		expectedDurationTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"task":        "task-1",
			"taskrun":     "taskrun-1",
			"namespace":   "ns",
			"status":      "failed",
		},
		expectedCountTags: map[string]string{
			"status": "failed",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
	}} {
		t.Run(c.name, func(t *testing.T) {
			unregisterMetrics()

			ctx := getConfigContext()
			metrics, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			if err := metrics.DurationAndCount(ctx, c.taskRun, c.beforeCondition); err != nil {
				t.Errorf("DurationAndCount: %v", err)
			}
			if c.expectedCountTags != nil {
				metricstest.CheckCountData(t, "taskrun_count", c.expectedCountTags, c.expectedCount)
			} else {
				metricstest.CheckStatsNotReported(t, "taskrun_count")
			}
			if c.expectedDurationTags != nil {
				metricstest.CheckLastValueData(t, c.metricName, c.expectedDurationTags, c.expectedDuration)
			} else {
				metricstest.CheckStatsNotReported(t, c.metricName)
			}
		})
	}
}

func TestRecordRunningTaskRunsCount(t *testing.T) {
	unregisterMetrics()
	newTaskRun := func(status corev1.ConditionStatus) *v1.TaskRun {
		return &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("taskrun-")},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: status,
					}},
				},
			},
		}
	}

	ctx, _ := ttesting.SetupFakeContext(t)
	informer := faketaskruninformer.Get(ctx)
	// Add N randomly-named TaskRuns with differently-succeeded statuses.
	for _, tr := range []*v1.TaskRun{
		newTaskRun(corev1.ConditionTrue),
		newTaskRun(corev1.ConditionUnknown),
		newTaskRun(corev1.ConditionFalse),
	} {
		if err := informer.Informer().GetIndexer().Add(tr); err != nil {
			t.Fatalf("Adding TaskRun to informer: %v", err)
		}
	}

	ctx = getConfigContext()
	metrics, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	if err := metrics.RunningTaskRuns(ctx, informer.Lister()); err != nil {
		t.Errorf("RunningTaskRuns: %v", err)
	}
	metricstest.CheckLastValueData(t, "running_taskruns_count", map[string]string{}, 1)
}

func TestRecordRunningTaskRunsThrottledCounts(t *testing.T) {
	for _, tc := range []struct {
		status     corev1.ConditionStatus
		reason     string
		nodeCount  float64
		quotaCount float64
	}{
		{
			status: corev1.ConditionTrue,
			reason: "",
		},
		{
			status: corev1.ConditionTrue,
			reason: pod.ReasonExceededResourceQuota,
		},
		{
			status: corev1.ConditionTrue,
			reason: pod.ReasonExceededNodeResources,
		},
		{
			status: corev1.ConditionFalse,
			reason: "",
		},
		{
			status: corev1.ConditionFalse,
			reason: pod.ReasonExceededResourceQuota,
		},
		{
			status: corev1.ConditionFalse,
			reason: pod.ReasonExceededNodeResources,
		},
		{
			status: corev1.ConditionUnknown,
			reason: "",
		},
		{
			status:     corev1.ConditionUnknown,
			reason:     pod.ReasonExceededResourceQuota,
			quotaCount: 1,
		},
		{
			status:    corev1.ConditionUnknown,
			reason:    pod.ReasonExceededNodeResources,
			nodeCount: 1,
		},
	} {
		unregisterMetrics()
		tr := &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("taskrun-")},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: tc.status,
						Reason: tc.reason,
					}},
				},
			},
		}
		ctx, _ := ttesting.SetupFakeContext(t)
		informer := faketaskruninformer.Get(ctx)
		if err := informer.Informer().GetIndexer().Add(tr); err != nil {
			t.Fatalf("Adding TaskRun to informer: %v", err)
		}

		ctx = getConfigContext()
		metrics, err := NewRecorder(ctx)
		if err != nil {
			t.Fatalf("NewRecorder: %v", err)
		}

		if err := metrics.RunningTaskRuns(ctx, informer.Lister()); err != nil {
			t.Errorf("RunningTaskRuns: %v", err)
		}
		metricstest.CheckLastValueData(t, "running_taskruns_throttled_by_quota_count", map[string]string{}, tc.quotaCount)
		metricstest.CheckLastValueData(t, "running_taskruns_throttled_by_node_count", map[string]string{}, tc.nodeCount)
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
		expectedTags   map[string]string
		expectedValue  float64
		expectingError bool
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
		expectedTags: map[string]string{
			"pod":       "test-taskrun-pod-123456",
			"task":      "task-1",
			"taskrun":   "test-taskrun",
			"namespace": "foo",
		},
		expectedValue: 4e+09,
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
	}} {
		t.Run(td.name, func(t *testing.T) {
			unregisterMetrics()

			ctx := getConfigContext()
			metrics, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			if err := metrics.RecordPodLatency(ctx, td.pod, taskRun); td.expectingError && err == nil {
				t.Error("RecordPodLatency wanted error, got nil")
			} else if !td.expectingError {
				if err != nil {
					t.Errorf("RecordPodLatency: %v", err)
				}
				metricstest.CheckLastValueData(t, "taskruns_pod_latency", td.expectedTags, td.expectedValue)
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

func unregisterMetrics() {
	metricstest.Unregister("taskrun_duration_seconds", "pipelinerun_taskrun_duration_seconds", "taskrun_count", "running_taskruns_count", "running_taskruns_throttled_by_quota_count", "running_taskruns_throttled_by_node_count", "taskruns_pod_latency")

	// Allow the recorder singleton to be recreated.
	once = sync.Once{}
	r = nil
	errRegistering = nil
}
