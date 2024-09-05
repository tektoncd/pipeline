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

	"go.opencensus.io/metric/metricproducer"
	"go.opencensus.io/stats/view"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"

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

	if err := metrics.DurationAndCount(&v1.PipelineRun{}, nil); err == nil {
		t.Error("DurationAndCount recording expected to return error but got nil")
	}
	if err := metrics.RunningPipelineRuns(nil); err == nil {
		t.Error("Current PR count recording expected to return error but got nil")
	}
}

func TestOnStore(t *testing.T) {
	unregisterMetrics()
	log := zap.NewExample().Sugar()

	// 1. Initial state
	initialCfg := &config.Config{Metrics: &config.Metrics{
		PipelinerunLevel:        config.PipelinerunLevelAtPipelinerun,
		DurationPipelinerunType: config.DurationPipelinerunTypeLastValue,
	}}
	ctx := config.ToContext(t.Context(), initialCfg)
	r, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder failed: %v", err)
	}
	onStoreCallback := OnStore(log, r)

	// Check initial state
	if reflect.ValueOf(r.insertTag).Pointer() != reflect.ValueOf(pipelinerunInsertTag).Pointer() {
		t.Fatalf("Initial insertTag function is incorrect")
	}
	initialHash := r.hash

	// 2. Call with wrong name - should not change anything
	onStoreCallback("wrong-name", &config.Metrics{PipelinerunLevel: config.PipelinerunLevelAtNS})
	if r.hash != initialHash {
		t.Errorf("Hash changed after call with wrong name")
	}
	if reflect.ValueOf(r.insertTag).Pointer() != reflect.ValueOf(pipelinerunInsertTag).Pointer() {
		t.Errorf("insertTag changed after call with wrong name")
	}

	// 3. Call with wrong type - should log an error and not change anything
	onStoreCallback(config.GetMetricsConfigName(), &config.Store{})
	if r.hash != initialHash {
		t.Errorf("Hash changed after call with wrong type")
	}
	if reflect.ValueOf(r.insertTag).Pointer() != reflect.ValueOf(pipelinerunInsertTag).Pointer() {
		t.Errorf("insertTag changed after call with wrong type")
	}

	// 4. Call with a valid new config - should change
	newCfg := &config.Metrics{
		PipelinerunLevel:        config.PipelinerunLevelAtNS,
		DurationPipelinerunType: config.DurationPipelinerunTypeLastValue,
	}
	onStoreCallback(config.GetMetricsConfigName(), newCfg)
	if r.hash == initialHash {
		t.Errorf("Hash did not change after valid config update")
	}
	if reflect.ValueOf(r.insertTag).Pointer() != reflect.ValueOf(nilInsertTag).Pointer() {
		t.Errorf("insertTag did not change after valid config update")
	}
	newHash := r.hash

	// 5. Call with the same config again - should not change
	onStoreCallback(config.GetMetricsConfigName(), newCfg)
	if r.hash != newHash {
		t.Errorf("Hash changed after second call with same config")
	}
	if reflect.ValueOf(r.insertTag).Pointer() != reflect.ValueOf(nilInsertTag).Pointer() {
		t.Errorf("insertTag changed after second call with same config")
	}

	// 6. Call with an invalid config - should update hash but not insertTag
	invalidCfg := &config.Metrics{PipelinerunLevel: "invalid-level"}
	onStoreCallback(config.GetMetricsConfigName(), invalidCfg)
	if r.hash == newHash {
		t.Errorf("Hash did not change after invalid config update")
	}
	// Because viewRegister fails, the insertTag function should not be updated and should remain `nilInsertTag` from the previous step.
	if reflect.ValueOf(r.insertTag).Pointer() != reflect.ValueOf(nilInsertTag).Pointer() {
		t.Errorf("insertTag changed after invalid config update")
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
		name                 string
		pipelineRun          *v1.PipelineRun
		expectedDurationTags map[string]string
		expectedCountTags    map[string]string
		expectedDuration     float64
		expectedCount        int64
		beforeCondition      *apis.Condition
		countWithReason      bool
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
		countWithReason:  false,
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
		countWithReason: false,
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
		countWithReason: false,
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
		countWithReason:  false,
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
		countWithReason:  false,
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
		countWithReason:  false,
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
						Reason: "Failed",
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
			"reason":      "Failed",
			"status":      "failed",
		},
		expectedCountTags: map[string]string{
			"status": "failed",
			"reason": "Failed",
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
		countWithReason:  true,
	}, {
		name: "for cancelled pipeline with reason",
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
						Reason: ReasonCancelled.String(),
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
			"reason":      ReasonCancelled.String(),
		},
		expectedCountTags: map[string]string{
			"status": "cancelled",
			"reason": ReasonCancelled.String(),
		},
		expectedDuration: 60,
		expectedCount:    1,
		beforeCondition:  nil,
		countWithReason:  true,
	}, {
		name: "for failed pipeline with reference remote pipeline",
		pipelineRun: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pipelinerun-1",
				Namespace: "ns",
				Labels: map[string]string{
					pipeline.PipelineLabelKey: "pipeline-remote",
				},
			},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					ResolverRef: v1.ResolverRef{
						Resolver: "git",
					},
				},
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
			"pipeline":    "pipeline-remote",
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
		countWithReason:  false,
	}} {
		t.Run(test.name, func(t *testing.T) {
			unregisterMetrics()

			ctx := getConfigContext(test.countWithReason)
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
				delete(test.expectedCountTags, "reason")
				metricstest.CheckCountData(t, "pipelinerun_total", test.expectedCountTags, test.expectedCount)
			} else {
				metricstest.CheckStatsNotReported(t, "pipelinerun_count")
				metricstest.CheckStatsNotReported(t, "pipelinerun_total")
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

	ctx = getConfigContext(false)
	metrics, err := NewRecorder(ctx)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	if err := metrics.RunningPipelineRuns(informer.Lister()); err != nil {
		t.Errorf("RunningPipelineRuns: %v", err)
	}
	metricstest.CheckLastValueData(t, "running_pipelineruns_count", map[string]string{}, 1)
	metricstest.CheckLastValueData(t, "running_pipelineruns", map[string]string{}, 1)
}

func TestRecordRunningPipelineRunsCountAtAllLevels(t *testing.T) {
	newPipelineRun := func(status corev1.ConditionStatus, namespace string, name string) *v1.PipelineRun {
		if name == "" {
			name = "anonymous"
		}
		return &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pipelinerun"), Namespace: namespace},
			Spec: v1.PipelineRunSpec{
				PipelineRef: &v1.PipelineRef{
					Name: name,
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

	pipelineRuns := []*v1.PipelineRun{
		newPipelineRun(corev1.ConditionUnknown, "testns1", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns1", "another"),
		newPipelineRun(corev1.ConditionUnknown, "testns1", "another"),
		newPipelineRun(corev1.ConditionFalse, "testns1", "another"),
		newPipelineRun(corev1.ConditionTrue, "testns1", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns2", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns2", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns2", "another"),
		newPipelineRun(corev1.ConditionUnknown, "testns3", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns3", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns3", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns3", ""),
		newPipelineRun(corev1.ConditionUnknown, "testns3", "another"),
		newPipelineRun(corev1.ConditionFalse, "testns3", ""),
	}

	pipelineRunMeasureQueries := []map[string]string{}
	pipelineRunExpectedResults := []int64{}
	for _, pipelineRun := range pipelineRuns {
		if pipelineRun.Status.Conditions[0].Status == corev1.ConditionUnknown {
			pipelineRunMeasureQueries = append(pipelineRunMeasureQueries, map[string]string{
				"namespace":   pipelineRun.Namespace,
				"pipeline":    pipelineRun.Spec.PipelineRef.Name,
				"pipelinerun": pipelineRun.Name,
			})
			pipelineRunExpectedResults = append(pipelineRunExpectedResults, 1)
		}
	}

	for _, test := range []struct {
		name            string
		metricLevel     string
		pipelineRuns    []*v1.PipelineRun
		measureQueries  []map[string]string
		expectedResults []int64
	}{{
		name:         "pipelinerun at pipeline level",
		metricLevel:  "pipeline",
		pipelineRuns: pipelineRuns,
		measureQueries: []map[string]string{
			{"namespace": "testns1", "pipeline": "anonymous"},
			{"namespace": "testns2", "pipeline": "anonymous"},
			{"namespace": "testns3", "pipeline": "anonymous"},
			{"namespace": "testns1", "pipeline": "another"},
		},
		expectedResults: []int64{1, 2, 4, 2},
	}, {
		name:         "pipelinerun at namespace level",
		metricLevel:  "namespace",
		pipelineRuns: pipelineRuns,
		measureQueries: []map[string]string{
			{"namespace": "testns1"},
			{"namespace": "testns2"},
			{"namespace": "testns3"},
		},
		expectedResults: []int64{3, 3, 5},
	}, {
		name:         "pipelinerun at cluster level",
		metricLevel:  "",
		pipelineRuns: pipelineRuns,
		measureQueries: []map[string]string{
			{},
		},
		expectedResults: []int64{11},
	}, {
		name:            "pipelinerun at pipelinerun level",
		metricLevel:     "pipelinerun",
		pipelineRuns:    pipelineRuns,
		measureQueries:  pipelineRunMeasureQueries,
		expectedResults: pipelineRunExpectedResults,
	}} {
		t.Run(test.name, func(t *testing.T) {
			unregisterMetrics()

			ctx, _ := ttesting.SetupFakeContext(t)
			informer := fakepipelineruninformer.Get(ctx)
			// Add N randomly-named PipelineRuns with differently-succeeded statuses.
			for _, pipelineRun := range test.pipelineRuns {
				if err := informer.Informer().GetIndexer().Add(pipelineRun); err != nil {
					t.Fatalf("Adding TaskRun to informer: %v", err)
				}
			}

			ctx = getConfigContextRunningPRLevel(test.metricLevel)
			recorder, err := NewRecorder(ctx)
			if err != nil {
				t.Fatalf("NewRecorder: %v", err)
			}

			if err := recorder.RunningPipelineRuns(informer.Lister()); err != nil {
				t.Errorf("RunningPipelineRuns: %v", err)
			}

			for idx, query := range test.measureQueries {
				checkLastValueDataForTags(t, "running_pipelineruns", query, float64(test.expectedResults[idx]))
			}
		})
	}
}

func TestRecordRunningPipelineRunsResolutionWaitCounts(t *testing.T) {
	multiplier := 3
	for _, tc := range []struct {
		status      corev1.ConditionStatus
		reason      string
		prWaitCount float64
		trWaitCount float64
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
		unregisterMetrics()
		ctx, _ := ttesting.SetupFakeContext(t)
		informer := fakepipelineruninformer.Get(ctx)
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
			if err := informer.Informer().GetIndexer().Add(pr); err != nil {
				t.Fatalf("Adding TaskRun to informer: %v", err)
			}
		}

		ctx = getConfigContext(false)
		metrics, err := NewRecorder(ctx)
		if err != nil {
			t.Fatalf("NewRecorder: %v", err)
		}

		if err := metrics.RunningPipelineRuns(informer.Lister()); err != nil {
			t.Errorf("RunningTaskRuns: %v", err)
		}
		metricstest.CheckLastValueData(t, "running_pipelineruns_waiting_on_pipeline_resolution_count", map[string]string{}, tc.prWaitCount)
		metricstest.CheckLastValueData(t, "running_pipelineruns_waiting_on_task_resolution_count", map[string]string{}, tc.trWaitCount)
		metricstest.CheckLastValueData(t, "running_pipelineruns_waiting_on_pipeline_resolution", map[string]string{}, tc.prWaitCount)
		metricstest.CheckLastValueData(t, "running_pipelineruns_waiting_on_task_resolution", map[string]string{}, tc.trWaitCount)
	}
}

func unregisterMetrics() {
	metricstest.Unregister("pipelinerun_duration_seconds", "pipelinerun_count", "pipelinerun_total", "running_pipelineruns_waiting_on_pipeline_resolution_count", "running_pipelineruns_waiting_on_pipeline_resolution", "running_pipelineruns_waiting_on_task_resolution_count", "running_pipelineruns_waiting_on_task_resolution", "running_pipelineruns_count", "running_pipelineruns")

	// Allow the recorder singleton to be recreated.
	once = sync.Once{}
	r = nil
	errRegistering = nil
}

// We have to write this function as knative package does not provide the feature to validate multiple records for same metric.
func checkLastValueDataForTags(t *testing.T, name string, wantTags map[string]string, expected float64) {
	t.Helper()
	for _, producer := range metricproducer.GlobalManager().GetAll() {
		meter := producer.(view.Meter)
		data, err := meter.RetrieveData(name)
		if err != nil || len(data) == 0 {
			continue
		}
		val := getLastValueData(data, wantTags)
		if val == nil {
			t.Error("Found no data for ", name, wantTags)
		} else if expected != val.Value {
			t.Error("Value did not match for ", name, wantTags, ", expected", expected, "got", val.Value)
		}
	}
}

// Returns the LastValueData from the matching row. If no row is matched then returns nil
func getLastValueData(rows []*view.Row, wantTags map[string]string) *view.LastValueData {
	for _, row := range rows {
		if len(wantTags) != len(row.Tags) {
			continue
		}
		matched := true
		for _, got := range row.Tags {
			n := got.Key.Name()
			if wantTags[n] != got.Value {
				matched = false
				break
			}
		}
		if matched {
			return row.Data.(*view.LastValueData)
		}
	}
	return nil
}
