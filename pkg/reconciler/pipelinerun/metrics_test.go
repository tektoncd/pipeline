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

package pipelinerun

import (
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	alpha1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	fakepipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/pipelinerun/fake"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/metrics/metricstest"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestUninitializedMetrics(t *testing.T) {
	metrics := Recorder{}

	durationCountError := metrics.DurationAndCount(&v1alpha1.PipelineRun{})
	prCountError := metrics.RunningPipelineRuns(nil)

	assertErrNotNil(durationCountError, "DurationAndCount recording expected to return error but got nil", t)
	assertErrNotNil(prCountError, "Current PR count recording expected to return error but got nil", t)
}

func TestRecordPipelineRunDurationCount(t *testing.T) {
	startTime := time.Now()

	testData := []struct {
		name             string
		taskRun          *v1alpha1.PipelineRun
		expectedTags     map[string]string
		expectedDuration float64
		expectedCount    int64
	}{{
		name: "for_succeeded_pipeline",
		taskRun: tb.PipelineRun("pipelinerun-1", "ns",
			tb.PipelineRunSpec("pipeline-1"),
			tb.PipelineRunStatus(
				tb.PipelineRunStartTime(startTime),
				tb.PipelineRunCompletionTime(startTime.Add(1*time.Minute)),
				tb.PipelineRunStatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			)),
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "success",
		},
		expectedDuration: 60,
		expectedCount:    1,
	}, {
		name: "for_failed_pipeline",
		taskRun: tb.PipelineRun("pipelinerun-1", "ns",
			tb.PipelineRunSpec("pipeline-1"),
			tb.PipelineRunStatus(
				tb.PipelineRunStartTime(startTime),
				tb.PipelineRunCompletionTime(startTime.Add(1*time.Minute)),
				tb.PipelineRunStatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}),
			)),
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"namespace":   "ns",
			"status":      "failed",
		},
		expectedDuration: 60,
		expectedCount:    1,
	}}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			unregisterMetrics()

			metrics, err := NewRecorder()
			assertErrIsNil(err, "Recorder initialization failed", t)

			err = metrics.DurationAndCount(test.taskRun)
			assertErrIsNil(err, "DurationAndCount recording recording got an error", t)
			metricstest.CheckDistributionData(t, "pipelinerun_duration_seconds", test.expectedTags, 1, test.expectedDuration, test.expectedDuration)
			metricstest.CheckCountData(t, "pipelinerun_count", test.expectedTags, test.expectedCount)

		})
	}
}

func TestRecordRunningPipelineRunsCount(t *testing.T) {
	unregisterMetrics()

	ctx, _ := rtesting.SetupFakeContext(t)
	informer := fakepipelineruninformer.Get(ctx)
	addPipelineRun(informer, "pipelinerun-1", "pipeline-1", "ns", corev1.ConditionTrue, t)
	addPipelineRun(informer, "pipelinerun-2", "pipeline-2", "ns", corev1.ConditionFalse, t)
	addPipelineRun(informer, "pipelinerun-3", "pipeline-3", "ns", corev1.ConditionUnknown, t)

	metrics, err := NewRecorder()
	assertErrIsNil(err, "Recorder initialization failed", t)

	err = metrics.RunningPipelineRuns(informer.Lister())
	assertErrIsNil(err, "RunningPrsCount recording expected to return nil but got error", t)
	metricstest.CheckLastValueData(t, "running_pipelineruns_count", map[string]string{}, 1)

}

func addPipelineRun(informer alpha1.PipelineRunInformer, run, pipeline, ns string, status corev1.ConditionStatus, t *testing.T) {
	t.Helper()

	err := informer.Informer().GetIndexer().Add(tb.PipelineRun(run, ns,
		tb.PipelineRunSpec(pipeline),
		tb.PipelineRunStatus(
			tb.PipelineRunStatusCondition(apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: status,
			}),
		)))

	if err != nil {
		t.Errorf("Failed to add the pipelinerun")
	}
}

func assertErrNotNil(err error, message string, t *testing.T) {
	t.Helper()
	if err == nil {
		t.Errorf(message)
	}
}

func assertErrIsNil(err error, message string, t *testing.T) {
	t.Helper()
	if err != nil {
		t.Errorf(message)
	}
}

func unregisterMetrics() {
	metricstest.Unregister("pipelinerun_duration_seconds", "pipelinerun_count", "running_pipelineruns_count")

}
