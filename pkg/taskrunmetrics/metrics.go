/*
Copyright 2024 The Tekton Authors

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
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/pod"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

const taskrunAnonymous = "anonymous"

// Recorder holds OpenTelemetry instruments for TaskRun metrics
type Recorder struct {
	mutex       sync.Mutex
	initialized bool
	cfg         *config.Metrics

	meter metric.Meter

	trDurationHistogram             metric.Float64Histogram
	prTRDurationHistogram           metric.Float64Histogram
	trTotalCounter                  metric.Int64Counter
	runningTRsGauge                 metric.Float64UpDownCounter
	runningTRsThrottledByQuotaGauge metric.Float64UpDownCounter
	runningTRsThrottledByNodeGauge  metric.Float64UpDownCounter
	podLatencyHistogram             metric.Float64Histogram

	insertTaskTag     func(task, taskrun string) []attribute.KeyValue
	insertPipelineTag func(pipeline, pipelinerun string) []attribute.KeyValue

	ReportingPeriod time.Duration
}

var (
	once           sync.Once
	r              *Recorder
	errRegistering error
)

// NewRecorder creates a new OpenTelemetry-based metrics recorder instance
func NewRecorder(ctx context.Context) (*Recorder, error) {
	once.Do(func() {
		cfg := config.FromContextOrDefaults(ctx)
		r = &Recorder{
			initialized:     true,
			cfg:             cfg.Metrics,
			ReportingPeriod: 30 * time.Second,
		}

		errRegistering = r.initializeInstruments(cfg.Metrics)
		if errRegistering != nil {
			r.initialized = false
			return
		}
	})

	return r, errRegistering
}

func (r *Recorder) initializeInstruments(cfg *config.Metrics) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.meter = otel.GetMeterProvider().Meter("tekton_pipelines_controller")

	switch cfg.TaskrunLevel {
	case config.TaskrunLevelAtTaskrun:
		r.insertTaskTag = taskrunInsertTag
	case config.TaskrunLevelAtTask:
		r.insertTaskTag = taskInsertTag
	case config.TaskrunLevelAtNS:
		r.insertTaskTag = nilInsertTag
	default:
		return errors.New("invalid config for TaskrunLevel: " + cfg.TaskrunLevel)
	}

	switch cfg.PipelinerunLevel {
	case config.PipelinerunLevelAtPipelinerun:
		r.insertPipelineTag = pipelinerunInsertTag
	case config.PipelinerunLevelAtPipeline:
		r.insertPipelineTag = pipelineInsertTag
	case config.PipelinerunLevelAtNS:
		r.insertPipelineTag = nilInsertTag
	default:
		r.insertPipelineTag = nilInsertTag
	}

	trDurationHistogram, err := r.meter.Float64Histogram(
		"tekton_pipelines_controller_taskrun_duration_seconds",
		metric.WithDescription("The taskrun's execution time in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create taskrun duration histogram: %w", err)
	}
	r.trDurationHistogram = trDurationHistogram

	prTRDurationHistogram, err := r.meter.Float64Histogram(
		"tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds",
		metric.WithDescription("The pipelinerun's taskrun execution time in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create pipelinerun taskrun duration histogram: %w", err)
	}
	r.prTRDurationHistogram = prTRDurationHistogram

	trTotalCounter, err := r.meter.Int64Counter(
		"tekton_pipelines_controller_taskrun_total",
		metric.WithDescription("Number of taskruns"),
	)
	if err != nil {
		return fmt.Errorf("failed to create taskrun total counter: %w", err)
	}
	r.trTotalCounter = trTotalCounter

	runningTRsGauge, err := r.meter.Float64UpDownCounter(
		"tekton_pipelines_controller_running_taskruns",
		metric.WithDescription("Number of taskruns executing currently"),
	)
	if err != nil {
		return fmt.Errorf("failed to create running taskruns gauge: %w", err)
	}
	r.runningTRsGauge = runningTRsGauge

	runningTRsThrottledByQuotaGauge, err := r.meter.Float64UpDownCounter(
		"tekton_pipelines_controller_running_taskruns_throttled_by_quota",
		metric.WithDescription("Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of defined ResourceQuotas."),
	)
	if err != nil {
		return fmt.Errorf("failed to create running taskruns throttled by quota gauge: %w", err)
	}
	r.runningTRsThrottledByQuotaGauge = runningTRsThrottledByQuotaGauge

	runningTRsThrottledByNodeGauge, err := r.meter.Float64UpDownCounter(
		"tekton_pipelines_controller_running_taskruns_throttled_by_node",
		metric.WithDescription("Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of Node level constraints."),
	)
	if err != nil {
		return fmt.Errorf("failed to create running taskruns throttled by node gauge: %w", err)
	}
	r.runningTRsThrottledByNodeGauge = runningTRsThrottledByNodeGauge

	podLatencyHistogram, err := r.meter.Float64Histogram(
		"tekton_pipelines_controller_taskrun_pod_latency_milliseconds",
		metric.WithDescription("scheduling latency for the taskrun pods"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create taskrun pod latency histogram: %w", err)
	}
	r.podLatencyHistogram = podLatencyHistogram

	return nil
}

// OnStore returns a function that can be passed to a configmap watcher for dynamic updates
func OnStore(logger *zap.SugaredLogger, recorder *Recorder) func(name string, value interface{}) {
	return func(name string, value interface{}) {
		if name != config.GetMetricsConfigName() {
			return
		}
		cfg, ok := value.(*config.Metrics)
		if !ok {
			logger.Errorw("failed to convert value to metrics config", "value", value)
			return
		}
		if recorder.updateConfig(cfg) {
			if err := recorder.initializeInstruments(cfg); err != nil {
				logger.Errorw("failed to initialize instruments", "error", err)
			}
		}
	}
}

func pipelinerunInsertTag(pipeline, pipelinerun string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("pipeline", pipeline),
		attribute.String("pipelinerun", pipelinerun),
	}
}

func pipelineInsertTag(pipeline, pipelinerun string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("pipeline", pipeline),
	}
}

func taskrunInsertTag(task, taskrun string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("task", task),
		attribute.String("taskrun", taskrun),
	}
}

func taskInsertTag(task, taskrun string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("task", task),
	}
}

func nilInsertTag(task, taskrun string) []attribute.KeyValue {
	return []attribute.KeyValue{}
}

func getTaskTagName(tr *v1.TaskRun) string {
	taskName := taskrunAnonymous
	if tr.Spec.TaskRef != nil && tr.Spec.TaskRef.Name != "" {
		taskName = tr.Spec.TaskRef.Name
	} else if tr.Spec.TaskSpec != nil {
		if pipelineTask, ok := tr.Labels[pipeline.PipelineTaskLabelKey]; ok {
			taskName = pipelineTask
		}
	} else if task, ok := tr.Labels[pipeline.TaskLabelKey]; ok {
		taskName = task
	}
	return taskName
}

func (r *Recorder) updateConfig(cfg *config.Metrics) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if equality.Semantic.DeepEqual(r.cfg, cfg) {
		return false
	}
	r.cfg = cfg
	return true
}

// DurationAndCount logs the duration of TaskRun execution and
// count for number of TaskRuns succeed or failed
func (r *Recorder) DurationAndCount(ctx context.Context, tr *v1.TaskRun, beforeCondition *apis.Condition) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording for %s , failed to initialize the metrics recorder", tr.Name)
	}

	afterCondition := tr.Status.GetCondition(apis.ConditionSucceeded)
	if equality.Semantic.DeepEqual(beforeCondition, afterCondition) {
		return nil
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	duration := time.Since(tr.Status.StartTime.Time)
	if tr.Status.CompletionTime != nil {
		duration = tr.Status.CompletionTime.Sub(tr.Status.StartTime.Time)
	}

	taskName := getTaskTagName(tr)

	cond := tr.Status.GetCondition(apis.ConditionSucceeded)
	status := "success"
	if cond.Status == corev1.ConditionFalse {
		status = "failed"
	}
	reason := cond.Reason

	attrs := []attribute.KeyValue{
		attribute.String("namespace", tr.Namespace),
		attribute.String("status", status),
		attribute.String("reason", reason),
	}
	attrs = append(attrs, r.insertTaskTag(taskName, tr.Name)...)

	if ok, pipeline, pipelinerun := IsPartOfPipeline(tr); ok {
		attrs = append(attrs, r.insertPipelineTag(pipeline, pipelinerun)...)
		r.prTRDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	} else {
		r.trDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}

	r.trTotalCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))

	return nil
}

// RunningTaskRuns logs the number of TaskRuns running right now
func (r *Recorder) RunningTaskRuns(ctx context.Context, lister listers.TaskRunLister) error {
	if !r.initialized {
		return errors.New("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	trs, err := lister.List(labels.Everything())
	if err != nil {
		return err
	}

	addNamespaceLabelToThrottleMetric := r.cfg != nil && r.cfg.ThrottleWithNamespace

	runningTrs := 0
	trsThrottledByQuota := make(map[attribute.Set]float64)
	trsThrottledByNode := make(map[attribute.Set]float64)

	for _, tr := range trs {
		if !tr.IsDone() {
			runningTrs++
			succeedCondition := tr.Status.GetCondition(apis.ConditionSucceeded)
			if succeedCondition != nil && succeedCondition.Status == corev1.ConditionUnknown {
				var attrs []attribute.KeyValue
				if addNamespaceLabelToThrottleMetric {
					attrs = append(attrs, attribute.String("namespace", tr.Namespace))
				}
				attrSet := attribute.NewSet(attrs...)

				switch succeedCondition.Reason {
				case pod.ReasonExceededResourceQuota:
					trsThrottledByQuota[attrSet]++
				case pod.ReasonExceededNodeResources:
					trsThrottledByNode[attrSet]++
				}
			}
		}
	}

	r.runningTRsGauge.Add(ctx, float64(runningTrs))
	for attrSet, value := range trsThrottledByQuota {
		r.runningTRsThrottledByQuotaGauge.Add(ctx, value, metric.WithAttributes(attrSet.ToSlice()...))
	}
	for attrSet, value := range trsThrottledByNode {
		r.runningTRsThrottledByNodeGauge.Add(ctx, value, metric.WithAttributes(attrSet.ToSlice()...))
	}

	return nil
}

// ReportRunningTaskRuns invokes RunningTaskRuns on our configured PeriodSeconds
// until the context is cancelled.
func (r *Recorder) ReportRunningTaskRuns(ctx context.Context, lister listers.TaskRunLister) {
	logger := logging.FromContext(ctx)
	for {
		delay := time.NewTimer(r.ReportingPeriod)
		select {
		case <-ctx.Done():
			delay.Stop()
			return

		case <-delay.C:
			if err := r.RunningTaskRuns(ctx, lister); err != nil {
				logger.Warnw("Failed to log running taskruns metric", zap.Error(err))
			}
		}
	}
}

// RecordPodLatency logs the duration required to schedule the pod for TaskRun
func (r *Recorder) RecordPodLatency(ctx context.Context, pod *corev1.Pod, tr *v1.TaskRun) error {
	if !r.initialized {
		return errors.New("ignoring the metrics recording for pod , failed to initialize the metrics recorder")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	scheduledTime := getScheduledTime(pod)
	if scheduledTime.IsZero() {
		return errors.New("pod has never got scheduled")
	}

	latency := scheduledTime.Sub(pod.CreationTimestamp.Time)
	taskName := getTaskTagName(tr)

	attrs := []attribute.KeyValue{
		attribute.String("namespace", tr.Namespace),
		attribute.String("pod", pod.Name),
	}
	attrs = append(attrs, r.insertTaskTag(taskName, tr.Name)...)

	r.podLatencyHistogram.Record(ctx, float64(latency.Milliseconds()), metric.WithAttributes(attrs...))

	return nil
}

// IsPartOfPipeline return true if TaskRun is a part of a Pipeline.
// It also return the name of Pipeline and PipelineRun
func IsPartOfPipeline(tr *v1.TaskRun) (bool, string, string) {
	if pipelineName, ok := tr.Labels[pipeline.PipelineLabelKey]; ok {
		if pipelineRun, ok := tr.Labels[pipeline.PipelineRunLabelKey]; ok {
			return true, pipelineName, pipelineRun
		}
	}
	return false, "", ""
}

func getScheduledTime(pod *corev1.Pod) metav1.Time {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled {
			return c.LastTransitionTime
		}
	}
	return metav1.Time{}
}
