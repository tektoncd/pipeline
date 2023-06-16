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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	fakepipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/pipelinerun/fake"
	"github.com/tektoncd/pipeline/pkg/names"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/metrics/metricstest" // Required to setup metrics env for testing
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

	if err := metrics.DurationAndCount(&v1.PipelineRun{}, nil); err == nil {
		t.Error("DurationAndCount recording expected to return error but got nil")
	}
	if err := metrics.RunningPipelineRuns(nil); err == nil {
		t.Error("Current PR count recording expected to return error but got nil")
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
	if reflect.ValueOf(metrics.insertTag).Pointer() != reflect.ValueOf(pipelinerunInsertTag).Pointer() {
		t.Fatal("metrics recorder shouldn't change during this OnStore call")
	}

	// Test when incorrect value in configmap is pass
	cfg := &config.Metrics{
		TaskrunLevel:            "foo",
		PipelinerunLevel:        "bar",
		DurationTaskrunType:     config.DurationTaskrunTypeHistogram,
		DurationPipelinerunType: config.DurationPipelinerunTypeLastValue,
	}
	MetricsOnStore(logger)(config.GetMetricsConfigName(), cfg)
	if reflect.ValueOf(metrics.insertTag).Pointer() != reflect.ValueOf(pipelinerunInsertTag).Pointer() {
		t.Fatal("metrics recorder shouldn't change during this OnStore call")
	}

	cfg = &config.Metrics{
		TaskrunLevel:            config.TaskrunLevelAtNS,
		PipelinerunLevel:        config.PipelinerunLevelAtNS,
		DurationTaskrunType:     config.DurationTaskrunTypeHistogram,
		DurationPipelinerunType: config.DurationPipelinerunTypeLastValue,
	}
	MetricsOnStore(logger)(config.GetMetricsConfigName(), cfg)
	if reflect.ValueOf(metrics.insertTag).Pointer() != reflect.ValueOf(nilInsertTag).Pointer() {
		t.Fatal("metrics recorder didn't change during OnStore call")
	}
}

func TestRecordPipelineRunDurationCount(t *testing.T) {
	for _, test := range []struct {
		name                 string
		pipelineRun          *v1.PipelineRun
		expectedDurationTags map[string]string
		expectedCountTags    map[string]string
		expectedDuration     float64
		expectedCount        int64
		beforeCondition      *apis.Condition
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
		expectedDurationTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
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
		expectedDurationTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "success",
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
		name: "for succeeded pipeline recount",
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
		expectedDurationTags: nil,
		expectedCountTags:    nil,
		expectedDuration:     0,
		expectedCount:        0,
		beforeCondition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
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
						Reason: ReasonCancelled,
					}},
				},
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					StartTime:      &startTime,
					CompletionTime: &completionTime,
				},
			},
		},
		expectedDurationTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "cancelled",
		},
		expectedCountTags: map[string]string{
			"status": "cancelled",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
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
		expectedDurationTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "failed",
		},
		expectedCountTags: map[string]string{
			"status": "failed",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
	}, {
		name: "for pipeline without start or completion time",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-1", Namespace: "ns"},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{Name: "pipeline-1"},
				Status:      v1.PipelineRunSpecStatusPending,
			},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
		expectedDurationTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "failed",
		},
		expectedCountTags: map[string]string{
			"status": "failed",
		},
		expectedDuration: 0,
		expectedCount:    1,
		beforeCondition:  nil,
	}} {
		t.Run(test.name, func(t *testing.T) {
			unregisterMetrics()

			ctx := getConfigContext()
			metrics, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			if err := metrics.DurationAndCount(test.pipelineRun, test.beforeCondition); err != nil {
				t.Errorf("DurationAndCount: %v", err)
			}
			if test.expectedDurationTags != nil {
				metricstest.CheckLastValueData(t, "pipelinerun_duration_seconds", test.expectedDurationTags, test.expectedDuration)
			} else {
				metricstest.CheckStatsNotReported(t, "pipelinerun_duration_seconds")
			}
			if test.expectedCountTags != nil {
				metricstest.CheckCountData(t, "pipelinerun_count", test.expectedCountTags, test.expectedCount)
			} else {
				metricstest.CheckStatsNotReported(t, "pipelinerun_count")
			}
		})
	}
}

func TestRecordRunningPipelineRunsCount(t *testing.T) {
	unregisterMetrics()

	newPipelineRun := func(status corev1.ConditionStatus) *v1.PipelineRun {
		return &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pipelinerun-")},
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

	ctx, _ := ttesting.SetupFakeContext(t)
	informer := fakepipelineruninformer.Get(ctx)
	// Add N randomly-named PipelineRuns with differently-succeeded statuses.
	for _, tr := range []*v1.PipelineRun{
		newPipelineRun(corev1.ConditionTrue),
		newPipelineRun(corev1.ConditionUnknown),
		newPipelineRun(corev1.ConditionFalse),
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

	if err := metrics.RunningPipelineRuns(informer.Lister()); err != nil {
		t.Errorf("RunningPipelineRuns: %v", err)
	}
	metricstest.CheckLastValueData(t, "running_pipelineruns_count", map[string]string{}, 1)
}

func unregisterMetrics() {
	metricstest.Unregister("pipelinerun_duration_seconds", "pipelinerun_count", "running_pipelineruns_count")

	// Allow the recorder singleton to be recreated.
	once = sync.Once{}
	r = nil
	errRegistering = nil
}
