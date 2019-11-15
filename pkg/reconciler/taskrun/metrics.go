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
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/metrics"
)

var (
	trDuration = stats.Float64(
		"taskrun_duration_seconds",
		"The taskrun's execution time in seconds",
		stats.UnitDimensionless)
	trDistribution = view.Distribution(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400)

	prTRDuration = stats.Float64(
		"pipelinerun_taskrun_duration_seconds",
		"The pipelinerun's taskrun execution time in seconds",
		stats.UnitDimensionless)
	prTRLatencyDistribution = view.Distribution(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400)

	trCount = stats.Float64("taskrun_count",
		"number of taskruns",
		stats.UnitDimensionless)

	runningTRsCount = stats.Float64("running_taskruns_count",
		"Number of taskruns executing currently",
		stats.UnitDimensionless)

	podLatency = stats.Float64("taskruns_pod_latency",
		"scheduling latency for the taskruns pods",
		stats.UnitMilliseconds)
)

type Recorder struct {
	initialized bool

	task        tag.Key
	taskRun     tag.Key
	namespace   tag.Key
	status      tag.Key
	pipeline    tag.Key
	pipelineRun tag.Key
	pod         tag.Key
}

// NewRecorder creates a new metrics recorder instance
// to log the TaskRun related metrics
func NewRecorder() (*Recorder, error) {
	r := &Recorder{
		initialized: true,
	}

	task, err := tag.NewKey("task")
	if err != nil {
		return nil, err
	}
	r.task = task

	taskRun, err := tag.NewKey("taskrun")
	if err != nil {
		return nil, err
	}
	r.taskRun = taskRun

	namespace, err := tag.NewKey("namespace")
	if err != nil {
		return nil, err
	}
	r.namespace = namespace

	status, err := tag.NewKey("status")
	if err != nil {
		return nil, err
	}
	r.status = status

	pipeline, err := tag.NewKey("pipeline")
	if err != nil {
		return nil, err
	}
	r.pipeline = pipeline

	pipelineRun, err := tag.NewKey("pipelinerun")
	if err != nil {
		return nil, err
	}
	r.pipelineRun = pipelineRun

	pod, err := tag.NewKey("pod")
	if err != nil {
		return nil, err
	}
	r.pod = pod

	err = view.Register(
		&view.View{
			Description: trDuration.Description(),
			Measure:     trDuration,
			Aggregation: trDistribution,
			TagKeys:     []tag.Key{r.task, r.taskRun, r.namespace, r.status},
		},
		&view.View{
			Description: prTRDuration.Description(),
			Measure:     prTRDuration,
			Aggregation: prTRLatencyDistribution,
			TagKeys:     []tag.Key{r.task, r.taskRun, r.namespace, r.status, r.pipeline, r.pipelineRun},
		},
		&view.View{
			Description: trCount.Description(),
			Measure:     trCount,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.status},
		},
		&view.View{
			Description: runningTRsCount.Description(),
			Measure:     runningTRsCount,
			Aggregation: view.LastValue(),
		},
		&view.View{
			Description: podLatency.Description(),
			Measure:     podLatency,
			Aggregation: view.LastValue(),
			TagKeys:     []tag.Key{r.task, r.taskRun, r.namespace, r.pod},
		},
	)

	if err != nil {
		r.initialized = false
		return r, err
	}

	return r, nil
}

// DurationAndCount logs the duration of TaskRun execution and
// count for number of TaskRuns succeed or failed
// returns an error if its failed to log the metrics
func (r *Recorder) DurationAndCount(tr *v1alpha1.TaskRun) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording for %s , failed to initialize the metrics recorder", tr.Name)
	}

	duration := time.Since(tr.Status.StartTime.Time)
	if tr.Status.CompletionTime != nil {
		duration = tr.Status.CompletionTime.Sub(tr.Status.StartTime.Time)
	}

	taskName := "anonymous"
	if tr.Spec.TaskRef != nil {
		taskName = tr.Spec.TaskRef.Name
	}

	status := "success"
	if tr.Status.Conditions[0].Status == corev1.ConditionFalse {
		status = "failed"
	}

	if ok, pipeline, pipelinerun := tr.IsPartOfPipeline(); ok {
		ctx, err := tag.New(
			context.Background(),
			tag.Insert(r.task, taskName),
			tag.Insert(r.taskRun, tr.Name),
			tag.Insert(r.namespace, tr.Namespace),
			tag.Insert(r.status, status),
			tag.Insert(r.pipeline, pipeline),
			tag.Insert(r.pipelineRun, pipelinerun),
		)

		if err != nil {
			return err
		}

		stats.Record(ctx, prTRDuration.M(float64(duration/time.Second)))
		metrics.Record(ctx, trCount.M(1))
		return nil
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.task, taskName),
		tag.Insert(r.taskRun, tr.Name),
		tag.Insert(r.namespace, tr.Namespace),
		tag.Insert(r.status, status),
	)
	if err != nil {
		return err
	}

	metrics.Record(ctx, trDuration.M(float64(duration/time.Second)))
	metrics.Record(ctx, trCount.M(1))

	return nil
}

// RunningTaskRuns logs the number of TaskRuns running right now
// returns an error if its failed to log the metrics
func (r *Recorder) RunningTaskRuns(lister listers.TaskRunLister) error {
	if !r.initialized {
		return errors.New("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	trs, err := lister.List(labels.Everything())
	if err != nil {
		return err
	}

	var runningTrs int
	for _, pr := range trs {
		if !pr.IsDone() {
			runningTrs++
		}
	}

	ctx, err := tag.New(
		context.Background(),
	)
	if err != nil {
		return err
	}
	metrics.Record(ctx, runningTRsCount.M(float64(runningTrs)))

	return nil
}

// RecordPodLatency logs the duration required to schedule the pod for TaskRun
// returns an error if its failed to log the metrics
func (r *Recorder) RecordPodLatency(pod *corev1.Pod, tr *v1alpha1.TaskRun) error {
	if !r.initialized {
		return errors.New("ignoring the metrics recording for pod , failed to initialize the metrics recorder")
	}

	scheduledTime := getScheduledTime(pod)
	if scheduledTime.IsZero() {
		return errors.New("pod has never got scheduled")
	}

	latency := scheduledTime.Sub(pod.CreationTimestamp.Time)
	taskName := "anonymous"
	if tr.Spec.TaskRef != nil {
		taskName = tr.Spec.TaskRef.Name
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.task, taskName),
		tag.Insert(r.taskRun, tr.Name),
		tag.Insert(r.namespace, tr.Namespace),
		tag.Insert(r.pod, pod.Name),
	)
	if err != nil {
		return err
	}

	metrics.Record(ctx, podLatency.M(float64(latency)))

	return nil
}

func getScheduledTime(pod *corev1.Pod) metav1.Time {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled {
			return c.LastTransitionTime
		}
	}

	return metav1.Time{}
}
