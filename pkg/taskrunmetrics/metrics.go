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
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
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

	r.meter = otel.GetMeterProvider().Meter("tekton.pipeline.taskrun")

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
		"taskrun_duration_seconds",
		metric.WithDescription("The taskrun execution time in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400),
	)
	if err != nil {
		return fmt.Errorf("failed to create taskrun duration histogram: %w", err)
	}
	r.trDurationHistogram = trDurationHistogram

	prTRDurationHistogram, err := r.meter.Float64Histogram(
		"pipelinerun_taskrun_duration_seconds",
		metric.WithDescription("The pipelinerun taskrun execution time in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400),
	)
	if err != nil {
		return fmt.Errorf("failed to create pipelinerun taskrun duration histogram: %w", err)
	}
	r.prTRDurationHistogram = prTRDurationHistogram

	trTotalCounter, err := r.meter.Int64Counter(
		"taskrun_total",
		metric.WithDescription("Number of taskruns"),
	)
	if err != nil {
		return fmt.Errorf("failed to create taskrun total counter: %w", err)
	}
	r.trTotalCounter = trTotalCounter

	runningTRsGauge, err := r.meter.Float64UpDownCounter(
		"running_taskruns",
		metric.WithDescription("Number of taskruns executing currently"),
	)
	if err != nil {
		return fmt.Errorf("failed to create running taskruns gauge: %w", err)
	}
	r.runningTRsGauge = runningTRsGauge

	runningTRsThrottledByQuotaGauge, err := r.meter.Float64UpDownCounter(
		"running_taskruns_throttled_by_quota",
		metric.WithDescription("Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of quota constraints. Such suspensions can occur as part of initial scheduling of the Pod, or scheduling of any of the subsequent Container(s) in the Pod after the first Container is started"),
	)
	if err != nil {
		return fmt.Errorf("failed to create running taskruns throttled by quota gauge: %w", err)
	}
	r.runningTRsThrottledByQuotaGauge = runningTRsThrottledByQuotaGauge

	runningTRsThrottledByNodeGauge, err := r.meter.Float64UpDownCounter(
		"running_taskruns_throttled_by_node",
		metric.WithDescription("Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of Node level constraints. Such suspensions can occur as part of initial scheduling of the Pod, or scheduling of any of the subsequent Container(s) in the Pod after the first Container is started"),
	)
	if err != nil {
		return fmt.Errorf("failed to create running taskruns throttled by node gauge: %w", err)
	}
	r.runningTRsThrottledByNodeGauge = runningTRsThrottledByNodeGauge

	podLatencyHistogram, err := r.meter.Float64Histogram(
		"taskruns_pod_latency_milliseconds",
		metric.WithDescription("scheduling latency for the taskruns pods"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400),
	)
	if err != nil {
		return fmt.Errorf("failed to create pod latency histogram: %w", err)
	}
	r.podLatencyHistogram = podLatencyHistogram

	return nil
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

	if r.insertTaskTag != nil {
		attrs = append(attrs, r.insertTaskTag(taskName, tr.Name)...)
	}

	r.trDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	r.trTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	if isPartOfPipeline, pipelineName, pipelinerunName := IsPartOfPipeline(tr); isPartOfPipeline {
		pipelineAttrs := append(attrs, r.insertPipelineTag(pipelineName, pipelinerunName)...)
		r.prTRDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(pipelineAttrs...))
	}

	return nil
}

// RunningTaskRuns logs the number of TaskRuns running right now
func (r *Recorder) RunningTaskRuns(ctx context.Context, lister listers.TaskRunLister) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	trs, err := lister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list taskruns: %w", err)
	}

	r.runningTRsGauge.Add(ctx, 0, metric.WithAttributes())
	r.runningTRsThrottledByQuotaGauge.Add(ctx, 0, metric.WithAttributes())
	r.runningTRsThrottledByNodeGauge.Add(ctx, 0, metric.WithAttributes())

	runningCount := 0
	throttledByQuotaCount := 0
	throttledByNodeCount := 0

	for _, tr := range trs {
		if tr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
			runningCount++

			if isThrottledByQuota(tr) {
				throttledByQuotaCount++
			}

			if isThrottledByNode(tr) {
				throttledByNodeCount++
			}
		}
	}

	r.runningTRsGauge.Add(ctx, float64(runningCount), metric.WithAttributes())
	r.runningTRsThrottledByQuotaGauge.Add(ctx, float64(throttledByQuotaCount), metric.WithAttributes())
	r.runningTRsThrottledByNodeGauge.Add(ctx, float64(throttledByNodeCount), metric.WithAttributes())

	return nil
}

// ReportRunningTaskRuns invokes RunningTaskRuns on our configured PeriodSeconds
// until the context is cancelled.
func (r *Recorder) ReportRunningTaskRuns(ctx context.Context, lister listers.TaskRunLister) {
	for {
		delay := time.NewTimer(r.ReportingPeriod)
		select {
		case <-ctx.Done():
			// When the context is cancelled, stop reporting.
			if !delay.Stop() {
				<-delay.C
			}
			return

		case <-delay.C:
			// Every 30s surface a metric for the number of running tasks.
			if err := r.RunningTaskRuns(ctx, lister); err != nil {
				// Log error but continue reporting
				// In a real implementation, you might want to use a logger here
			}
		}
	}
}

// PodLatency logs the pod scheduling latency for TaskRuns
func (r *Recorder) PodLatency(ctx context.Context, tr *v1.TaskRun, pod *corev1.Pod) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording for %s , failed to initialize the metrics recorder", tr.Name)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if pod.Status.StartTime != nil && tr.Status.StartTime != nil {
		latency := pod.Status.StartTime.Sub(tr.Status.StartTime.Time)
		attrs := []attribute.KeyValue{
			attribute.String("namespace", tr.Namespace),
		}
		taskName := getTaskTagName(tr)
		if r.insertTaskTag != nil {
			attrs = append(attrs, r.insertTaskTag(taskName, tr.Name)...)
		}
		r.podLatencyHistogram.Record(ctx, float64(latency.Milliseconds()), metric.WithAttributes(attrs...))
	}

	return nil
}

// Helper functions for tag insertion
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

func getTaskTagName(tr *v1.TaskRun) string {
	if tr.Spec.TaskRef != nil && tr.Spec.TaskRef.Name != "" {
		return tr.Spec.TaskRef.Name
	}
	if tr.Spec.TaskSpec != nil {
		return taskrunAnonymous
	}
	return taskrunAnonymous
}

// IsPartOfPipeline checks if a TaskRun is part of a PipelineRun
func IsPartOfPipeline(tr *v1.TaskRun) (bool, string, string) {
	if tr.Labels == nil {
		return false, "", ""
	}
	pipelineName, hasPipelineLabel := tr.Labels["tekton.dev/pipeline"]
	pipelinerunName, hasPipelineRunLabel := tr.Labels["tekton.dev/pipelinerun"]
	return hasPipelineLabel && hasPipelineRunLabel, pipelineName, pipelinerunName
}

func isThrottledByQuota(tr *v1.TaskRun) bool {
	return false
}

func isThrottledByNode(tr *v1.TaskRun) bool {
	return false
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
