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
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
)

var (
	pipelinerunTag = tag.MustNewKey("pipelinerun")
	pipelineTag    = tag.MustNewKey("pipeline")
	taskrunTag     = tag.MustNewKey("taskrun")
	taskTag        = tag.MustNewKey("task")
	namespaceTag   = tag.MustNewKey("namespace")
	statusTag      = tag.MustNewKey("status")
	podTag         = tag.MustNewKey("pod")

	trDurationView      *view.View
	prTRDurationView    *view.View
	trCountView         *view.View
	runningTRsCountView *view.View
	podLatencyView      *view.View
	cloudEventsView     *view.View

	trDuration = stats.Float64(
		"taskrun_duration_seconds",
		"The taskrun's execution time in seconds",
		stats.UnitDimensionless)

	prTRDuration = stats.Float64(
		"pipelinerun_taskrun_duration_seconds",
		"The pipelinerun's taskrun execution time in seconds",
		stats.UnitDimensionless)

	trCount = stats.Float64("taskrun_count",
		"number of taskruns",
		stats.UnitDimensionless)

	runningTRsCount = stats.Float64("running_taskruns_count",
		"Number of taskruns executing currently",
		stats.UnitDimensionless)

	podLatency = stats.Float64("taskruns_pod_latency",
		"scheduling latency for the taskruns pods",
		stats.UnitMilliseconds)

	cloudEvents = stats.Int64("cloudevent_count",
		"number of cloud events sent including retries",
		stats.UnitDimensionless)
)

type Recorder struct {
	mutex       sync.Mutex
	initialized bool

	ReportingPeriod time.Duration

	insertTaskTag func(task,
		taskrun string) []tag.Mutator

	insertPipelineTag func(pipeline,
		pipelinerun string) []tag.Mutator
}

// We cannot register the view multiple times, so NewRecorder lazily
// initializes this singleton and returns the same recorder across any
// subsequent invocations.
var (
	once        sync.Once
	r           *Recorder
	recorderErr error
)

// NewRecorder creates a new metrics recorder instance
// to log the TaskRun related metrics
func NewRecorder(ctx context.Context) (*Recorder, error) {
	once.Do(func() {

		r = &Recorder{
			initialized: true,

			// Default to reporting metrics every 30s.
			ReportingPeriod: 30 * time.Second,
		}

		cfg := config.FromContextOrDefaults(ctx)

		recorderErr = viewRegister(cfg.Metrics)
		if recorderErr != nil {
			r.initialized = false
			return
		}
	})

	return r, recorderErr
}

func viewRegister(cfg *config.Metrics) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	prunTag := []tag.Key{}
	trunTag := []tag.Key{}
	switch cfg.PipelinerunLevel {
	case config.PipelinerunLevelAtPipelinerun:
		prunTag = []tag.Key{pipelineTag, pipelinerunTag}
		r.insertPipelineTag = pipelinerunInsertTag
	case config.PipelinerunLevelAtPipeline:
		prunTag = []tag.Key{pipelineTag}
		r.insertPipelineTag = pipelineInsertTag
	case config.PipelinerunLevelAtNS:
		prunTag = []tag.Key{}
		r.insertPipelineTag = nilInsertTag
	default:
		return errors.New("invalid config for PipelinerunLevel: " + cfg.PipelinerunLevel)
	}

	switch cfg.TaskrunLevel {
	case config.TaskrunLevelAtTaskrun:
		trunTag = []tag.Key{taskTag, taskrunTag}
		r.insertTaskTag = taskrunInsertTag
	case config.TaskrunLevelAtTask:
		trunTag = []tag.Key{taskTag}
		r.insertTaskTag = taskInsertTag
	case config.PipelinerunLevelAtNS:
		trunTag = []tag.Key{}
		r.insertTaskTag = nilInsertTag
	default:
		return errors.New("invalid config for TaskrunLevel: " + cfg.TaskrunLevel)
	}

	distribution := view.Distribution(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400)

	if cfg.TaskrunLevel == config.TaskrunLevelAtTaskrun ||
		cfg.PipelinerunLevel == config.PipelinerunLevelAtPipelinerun {
		distribution = view.LastValue()
	} else {
		switch cfg.DurationTaskrunType {
		case config.DurationTaskrunTypeHistogram:
		case config.DurationTaskrunTypeLastValue:
			distribution = view.LastValue()
		default:
			return errors.New("invalid config for DurationTaskrunType: " + cfg.DurationTaskrunType)
		}
	}

	trDurationView = &view.View{
		Description: trDuration.Description(),
		Measure:     trDuration,
		Aggregation: distribution,
		TagKeys:     append([]tag.Key{statusTag, namespaceTag}, trunTag...),
	}
	prTRDurationView = &view.View{
		Description: prTRDuration.Description(),
		Measure:     prTRDuration,
		Aggregation: distribution,
		TagKeys:     append([]tag.Key{statusTag, namespaceTag}, append(trunTag, prunTag...)...),
	}
	trCountView = &view.View{
		Description: trCount.Description(),
		Measure:     trCount,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{statusTag},
	}
	runningTRsCountView = &view.View{
		Description: runningTRsCount.Description(),
		Measure:     runningTRsCount,
		Aggregation: view.LastValue(),
	}
	podLatencyView = &view.View{
		Description: podLatency.Description(),
		Measure:     podLatency,
		Aggregation: view.LastValue(),
		TagKeys:     append([]tag.Key{namespaceTag, podTag}, trunTag...),
	}
	cloudEventsView = &view.View{
		Description: cloudEvents.Description(),
		Measure:     cloudEvents,
		Aggregation: view.Sum(),
		TagKeys:     append([]tag.Key{statusTag, namespaceTag}, append(trunTag, prunTag...)...),
	}
	return view.Register(
		trDurationView,
		prTRDurationView,
		trCountView,
		runningTRsCountView,
		podLatencyView,
		cloudEventsView,
	)
}

func viewUnregister() {
	view.Unregister(
		trDurationView,
		prTRDurationView,
		trCountView,
		runningTRsCountView,
		podLatencyView,
		cloudEventsView,
	)
}

func MetricsOnStore(logger *zap.SugaredLogger) func(name string,
	value interface{}) {
	return func(name string, value interface{}) {
		if name == config.GetMetricsConfigName() {
			cfg, ok := value.(*config.Metrics)
			if !ok {
				logger.Error("Failed to do type insertion for extracting metrics config")
				return
			}
			viewUnregister()
			err := viewRegister(cfg)
			if err != nil {
				logger.Errorf("Failed to register View %v ", err)
				return
			}
		}
	}
}

func pipelinerunInsertTag(pipeline, pipelinerun string) []tag.Mutator {
	return []tag.Mutator{tag.Insert(pipelineTag, pipeline),
		tag.Insert(pipelinerunTag, pipelinerun)}
}

func pipelineInsertTag(pipeline, pipelinerun string) []tag.Mutator {
	return []tag.Mutator{tag.Insert(pipelineTag, pipeline)}
}

func taskrunInsertTag(task, taskrun string) []tag.Mutator {
	return []tag.Mutator{tag.Insert(taskTag, task),
		tag.Insert(taskrunTag, taskrun)}
}

func taskInsertTag(task, taskrun string) []tag.Mutator {
	return []tag.Mutator{tag.Insert(taskTag, task)}
}

func nilInsertTag(task, taskrun string) []tag.Mutator {
	return []tag.Mutator{}
}

// DurationAndCount logs the duration of TaskRun execution and
// count for number of TaskRuns succeed or failed
// returns an error if its failed to log the metrics
func (r *Recorder) DurationAndCount(tr *v1beta1.TaskRun) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

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
	if cond := tr.Status.GetCondition(apis.ConditionSucceeded); cond.Status == corev1.ConditionFalse {
		status = "failed"
	}

	if ok, pipeline, pipelinerun := tr.IsPartOfPipeline(); ok {
		ctx, err := tag.New(
			context.Background(),
			append([]tag.Mutator{tag.Insert(namespaceTag, tr.Namespace),
				tag.Insert(statusTag, status)},
				append(r.insertPipelineTag(pipeline, pipelinerun),
					r.insertTaskTag(taskName, tr.Name)...)...)...)

		if err != nil {
			return err
		}

		metrics.Record(ctx, prTRDuration.M(float64(duration/time.Second)))
		metrics.Record(ctx, trCount.M(1))
		return nil
	}

	ctx, err := tag.New(
		context.Background(),
		append([]tag.Mutator{tag.Insert(namespaceTag, tr.Namespace),
			tag.Insert(statusTag, status)},
			r.insertTaskTag(taskName, tr.Name)...)...)
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
	r.mutex.Lock()
	defer r.mutex.Unlock()

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

// ReportRunningTaskRuns invokes RunningTaskRuns on our configured PeriodSeconds
// until the context is cancelled.
func (r *Recorder) ReportRunningTaskRuns(ctx context.Context, lister listers.TaskRunLister) {
	logger := logging.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			// When the context is cancelled, stop reporting.
			return

		case <-time.After(r.ReportingPeriod):
			// Every 30s surface a metric for the number of running tasks.
			if err := r.RunningTaskRuns(lister); err != nil {
				logger.Warnf("Failed to log the metrics : %v", err)
			}
		}
	}
}

// RecordPodLatency logs the duration required to schedule the pod for TaskRun
// returns an error if its failed to log the metrics
func (r *Recorder) RecordPodLatency(pod *corev1.Pod, tr *v1beta1.TaskRun) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

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
		append([]tag.Mutator{tag.Insert(namespaceTag, tr.Namespace),
			tag.Insert(podTag, pod.Name)},
			r.insertTaskTag(taskName, tr.Name)...)...)
	if err != nil {
		return err
	}

	metrics.Record(ctx, podLatency.M(float64(latency)))

	return nil
}

// CloudEvents logs the number of cloud events sent for TaskRun
// returns an error if it fails to log the metrics
func (r *Recorder) CloudEvents(tr *v1beta1.TaskRun) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording for %s , failed to initialize the metrics recorder", tr.Name)
	}

	taskName := "anonymous"
	if tr.Spec.TaskRef != nil {
		taskName = tr.Spec.TaskRef.Name
	}

	status := "success"
	if cond := tr.Status.GetCondition(apis.ConditionSucceeded); cond.Status == corev1.ConditionFalse {
		status = "failed"
	}

	if ok, pipeline, pipelinerun := tr.IsPartOfPipeline(); ok {
		ctx, err := tag.New(
			context.Background(),
			append([]tag.Mutator{tag.Insert(namespaceTag, tr.Namespace),
				tag.Insert(statusTag, status)},
				append(r.insertPipelineTag(pipeline, pipelinerun),
					r.insertTaskTag(taskName, tr.Name)...)...)...)
		if err != nil {
			return err
		}
		metrics.Record(ctx, cloudEvents.M(sentCloudEvents(tr)))
		return nil
	}

	ctx, err := tag.New(
		context.Background(),
		append([]tag.Mutator{tag.Insert(namespaceTag, tr.Namespace),
			tag.Insert(statusTag, status)},
			r.insertTaskTag(taskName, tr.Name)...)...)
	if err != nil {
		return err
	}

	metrics.Record(ctx, cloudEvents.M(sentCloudEvents(tr)))

	return nil
}

func sentCloudEvents(tr *v1beta1.TaskRun) int64 {
	var sent int64
	for _, event := range tr.Status.CloudEvents {
		if event.Status.Condition != v1beta1.CloudEventConditionUnknown {
			sent += 1 + int64(event.Status.RetryCount)
		}
	}
	return sent
}

func getScheduledTime(pod *corev1.Pod) metav1.Time {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled {
			return c.LastTransitionTime
		}
	}

	return metav1.Time{}
}
