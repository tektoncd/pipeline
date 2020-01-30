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

package taskrun

import (
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	alpha1 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	faketaskruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/taskrun/fake"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/metrics/metricstest"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestUninitializedMetrics(t *testing.T) {
	metrics := Recorder{}

	durationCountError := metrics.DurationAndCount(&v1alpha1.TaskRun{})
	taskrunsCountError := metrics.RunningTaskRuns(nil)
	podLatencyError := metrics.RecordPodLatency(nil, nil)

	assertErrNotNil(durationCountError, "DurationCount recording expected to return error but got nil", t)
	assertErrNotNil(taskrunsCountError, "Current TaskrunsCount recording expected to return error but got nil", t)
	assertErrNotNil(podLatencyError, "Pod Latency recording expected to return error but got nil", t)
}

func TestRecordTaskrunDurationCount(t *testing.T) {
	startTime := time.Now()

	testData := []struct {
		name             string
		taskRun          *v1alpha1.TaskRun
		expectedTags     map[string]string
		expectedDuration float64
		expectedCount    int64
	}{{
		name: "for_succeeded_task",
		taskRun: tb.TaskRun("taskrun-1", "ns",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("task-1"),
			),
			tb.TaskRunStatus(
				tb.TaskRunStartTime(startTime),
				tb.TaskRunCompletionTime(startTime.Add(1*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			)),
		expectedTags: map[string]string{
			"task":      "task-1",
			"taskrun":   "taskrun-1",
			"namespace": "ns",
			"status":    "success",
		},
		expectedDuration: 60,
		expectedCount:    1,
	}, {
		name: "for_failed_task",
		taskRun: tb.TaskRun("taskrun-1", "ns",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("task-1"),
			),
			tb.TaskRunStatus(
				tb.TaskRunStartTime(startTime),
				tb.TaskRunCompletionTime(startTime.Add(1*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}),
			)),
		expectedTags: map[string]string{
			"task":      "task-1",
			"taskrun":   "taskrun-1",
			"namespace": "ns",
			"status":    "failed",
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
			assertErrIsNil(err, "DurationAndCount recording got an error", t)
			metricstest.CheckDistributionData(t, "taskrun_duration_seconds", test.expectedTags, 1, test.expectedDuration, test.expectedDuration)
			metricstest.CheckCountData(t, "taskrun_count", test.expectedTags, test.expectedCount)

		})
	}
}

func TestRecordPipelinerunTaskrunDurationCount(t *testing.T) {
	startTime := time.Now()

	testData := []struct {
		name             string
		taskRun          *v1alpha1.TaskRun
		expectedTags     map[string]string
		expectedDuration float64
		expectedCount    int64
	}{{
		name: "for_succeeded_task",
		taskRun: tb.TaskRun("taskrun-1", "ns",
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineLabelKey, "pipeline-1"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineRunLabelKey, "pipelinerun-1"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("task-1"),
			),
			tb.TaskRunStatus(
				tb.TaskRunStartTime(startTime),
				tb.TaskRunCompletionTime(startTime.Add(1*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
			)),
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"task":        "task-1",
			"taskrun":     "taskrun-1",
			"namespace":   "ns",
			"status":      "success",
		},
		expectedDuration: 60,
		expectedCount:    1,
	}, {
		name: "for_failed_task",
		taskRun: tb.TaskRun("taskrun-1", "ns",
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineLabelKey, "pipeline-1"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineRunLabelKey, "pipelinerun-1"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("task-1"),
			),
			tb.TaskRunStatus(
				tb.TaskRunStartTime(startTime),
				tb.TaskRunCompletionTime(startTime.Add(1*time.Minute)),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}),
			)),
		expectedTags: map[string]string{
			"pipeline":    "pipeline-1",
			"pipelinerun": "pipelinerun-1",
			"task":        "task-1",
			"taskrun":     "taskrun-1",
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
			assertErrIsNil(err, "DurationAndCount recording got an error", t)
			metricstest.CheckDistributionData(t, "pipelinerun_taskrun_duration_seconds", test.expectedTags, 1, test.expectedDuration, test.expectedDuration)
			metricstest.CheckCountData(t, "taskrun_count", test.expectedTags, test.expectedCount)

		})
	}
}

func TestRecordRunningTaskrunsCount(t *testing.T) {
	unregisterMetrics()

	ctx, _ := rtesting.SetupFakeContext(t)
	informer := faketaskruninformer.Get(ctx)
	addTaskruns(informer, "taskrun-1", "task-1", "ns", corev1.ConditionTrue, t)
	addTaskruns(informer, "taskrun-2", "task-3", "ns", corev1.ConditionUnknown, t)
	addTaskruns(informer, "taskrun-3", "task-3", "ns", corev1.ConditionFalse, t)

	metrics, err := NewRecorder()
	assertErrIsNil(err, "Recorder initialization failed", t)

	err = metrics.RunningTaskRuns(informer.Lister())
	assertErrIsNil(err, "RunningTaskRuns recording expected to return nil but got error", t)
	metricstest.CheckLastValueData(t, "running_taskruns_count", map[string]string{}, 1)
}

func TestRecordPodLatency(t *testing.T) {
	creationTime := time.Now()
	testData := []struct {
		name           string
		pod            *corev1.Pod
		taskRun        *v1alpha1.TaskRun
		expectedTags   map[string]string
		expectedValue  float64
		expectingError bool
	}{{
		name: "for_scheduled_pod",
		pod: tb.Pod("test-taskrun-pod-123456", "foo",
			tb.PodCreationTimestamp(creationTime),
			tb.PodStatus(
				tb.PodStatusConditions(corev1.PodCondition{
					Type:               corev1.PodScheduled,
					LastTransitionTime: metav1.Time{Time: creationTime.Add(4 * time.Second)},
				}),
			)),
		taskRun: tb.TaskRun("test-taskrun", "foo",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("task-1"),
			),
		),
		expectedTags: map[string]string{
			"pod":       "test-taskrun-pod-123456",
			"task":      "task-1",
			"taskrun":   "test-taskrun",
			"namespace": "foo",
		},
		expectedValue: 4e+09,
	}, {
		name: "for_non_scheduled_pod",
		pod: tb.Pod("test-taskrun-pod-123456", "foo",
			tb.PodCreationTimestamp(creationTime),
		),
		taskRun: tb.TaskRun("test-taskrun", "foo",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("task-1"),
			),
		),
		expectingError: true,
	}}

	for _, td := range testData {
		t.Run(td.name, func(t *testing.T) {
			unregisterMetrics()

			metrics, err := NewRecorder()
			assertErrIsNil(err, "Recorder initialization failed", t)

			err = metrics.RecordPodLatency(td.pod, td.taskRun)
			if td.expectingError {
				assertErrNotNil(err, "Pod Latency recording expected to return error but got nil", t)
				return
			}
			assertErrIsNil(err, "RecordPodLatency recording expected to return nil but got error", t)
			metricstest.CheckLastValueData(t, "taskruns_pod_latency", td.expectedTags, td.expectedValue)

		})
	}

}

func addTaskruns(informer alpha1.TaskRunInformer, taskrun, task, ns string, status corev1.ConditionStatus, t *testing.T) {
	err := informer.Informer().GetIndexer().Add(tb.TaskRun(taskrun, ns,
		tb.TaskRunSpec(
			tb.TaskRunTaskRef(task),
		),
		tb.TaskRunStatus(
			tb.StatusCondition(apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: status,
			}),
		)))

	if err != nil {
		t.Error("Failed to add the taskrun")
	}
}

func assertErrIsNil(err error, message string, t *testing.T) {
	t.Helper()
	if err != nil {
		t.Errorf(message)
	}
}

func assertErrNotNil(err error, message string, t *testing.T) {
	t.Helper()
	if err == nil {
		t.Errorf(message)
	}
}

func unregisterMetrics() {
	metricstest.Unregister("taskrun_duration_seconds", "pipelinerun_taskrun_duration_seconds", "taskrun_count", "running_taskruns_count", "taskruns_pod_latency")
}
