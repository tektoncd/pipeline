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

package pipelinerunmetrics

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	"go.uber.org/zap"
)

const (
	ReasonCancelled = v1.PipelineRunReasonCancelled
	anonymous       = "anonymous"
)

// Recorder holds OpenTelemetry instruments for PipelineRun metrics
type Recorder struct {
	mutex       sync.Mutex
	initialized bool
	cfg         *config.Metrics

	meter metric.Meter

	prDurationHistogram                        metric.Float64Histogram
	prTotalCounter                             metric.Int64Counter
	runningPRsGauge                            metric.Float64UpDownCounter
	runningPRsWaitingOnPipelineResolutionGauge metric.Float64UpDownCounter
	runningPRsWaitingOnTaskResolutionGauge     metric.Float64UpDownCounter

	insertTag func(pipeline, pipelinerun string) []attribute.KeyValue

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
		r = &Recorder{
			initialized:     true,
			ReportingPeriod: 30 * time.Second,
		}

		cfg := config.FromContextOrDefaults(ctx)
		r.cfg = cfg.Metrics
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

	r.meter = otel.GetMeterProvider().Meter("tekton.pipeline.pipelinerun")

	switch cfg.PipelinerunLevel {
	case config.PipelinerunLevelAtPipelinerun:
		r.insertTag = pipelinerunInsertTag
	case config.PipelinerunLevelAtPipeline:
		r.insertTag = pipelineInsertTag
	case config.PipelinerunLevelAtNS:
		r.insertTag = nilInsertTag
	default:
		return errors.New("invalid config for PipelinerunLevel: " + cfg.PipelinerunLevel)
	}

	prDurationHistogram, err := r.meter.Float64Histogram(
		"pipelinerun_duration_seconds",
		metric.WithDescription("The pipelinerun execution time in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400),
	)
	if err != nil {
		return fmt.Errorf("failed to create pipelinerun duration histogram: %w", err)
	}
	r.prDurationHistogram = prDurationHistogram

	prTotalCounter, err := r.meter.Int64Counter(
		"pipelinerun_total",
		metric.WithDescription("Number of pipelineruns"),
	)
	if err != nil {
		return fmt.Errorf("failed to create pipelinerun total counter: %w", err)
	}
	r.prTotalCounter = prTotalCounter

	runningPRsGauge, err := r.meter.Float64UpDownCounter(
		"running_pipelineruns",
		metric.WithDescription("Number of pipelineruns executing currently"),
	)
	if err != nil {
		return fmt.Errorf("failed to create running pipelineruns gauge: %w", err)
	}
	r.runningPRsGauge = runningPRsGauge

	runningPRsWaitingOnPipelineResolutionGauge, err := r.meter.Float64UpDownCounter(
		"running_pipelineruns_waiting_on_pipeline_resolution",
		metric.WithDescription("Number of pipelineruns executing currently that are waiting on resolution requests for their pipeline references."),
	)
	if err != nil {
		return fmt.Errorf("failed to create running pipelineruns waiting on pipeline resolution gauge: %w", err)
	}
	r.runningPRsWaitingOnPipelineResolutionGauge = runningPRsWaitingOnPipelineResolutionGauge

	runningPRsWaitingOnTaskResolutionGauge, err := r.meter.Float64UpDownCounter(
		"running_pipelineruns_waiting_on_task_resolution",
		metric.WithDescription("Number of pipelineruns executing currently that are waiting on resolution requests for the task references of their taskrun children."),
	)
	if err != nil {
		return fmt.Errorf("failed to create running pipelineruns waiting on task resolution gauge: %w", err)
	}
	r.runningPRsWaitingOnTaskResolutionGauge = runningPRsWaitingOnTaskResolutionGauge

	return nil
}

// DurationAndCount logs the duration of PipelineRun execution and
// count for number of PipelineRuns succeed or failed
func (r *Recorder) DurationAndCount(ctx context.Context, pr *v1.PipelineRun, beforeCondition *apis.Condition) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording for %s , failed to initialize the metrics recorder", pr.Name)
	}

	afterCondition := pr.Status.GetCondition(apis.ConditionSucceeded)
	if equality.Semantic.DeepEqual(beforeCondition, afterCondition) {
		return nil
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	duration := time.Since(pr.Status.StartTime.Time)
	if pr.Status.CompletionTime != nil {
		duration = pr.Status.CompletionTime.Sub(pr.Status.StartTime.Time)
	}

	pipelineName := getPipelineTagName(pr)

	cond := pr.Status.GetCondition(apis.ConditionSucceeded)
	status := "success"
	if cond.Status == corev1.ConditionFalse {
		status = "failed"
	}
	reason := cond.Reason

	attrs := []attribute.KeyValue{
		attribute.String("namespace", pr.Namespace),
		attribute.String("status", status),
		attribute.String("reason", reason),
	}

	if r.insertTag != nil {
		attrs = append(attrs, r.insertTag(pipelineName, pr.Name)...)
	}

	r.prDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	r.prTotalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	return nil
}

// RunningPipelineRuns logs the number of PipelineRuns running right now
func (r *Recorder) RunningPipelineRuns(ctx context.Context, lister listers.PipelineRunLister) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	prs, err := lister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list pipelineruns: %w", err)
	}

	r.runningPRsGauge.Add(ctx, 0, metric.WithAttributes())
	r.runningPRsWaitingOnPipelineResolutionGauge.Add(ctx, 0, metric.WithAttributes())
	r.runningPRsWaitingOnTaskResolutionGauge.Add(ctx, 0, metric.WithAttributes())

	runningCount := 0
	waitingOnPipelineCount := 0
	waitingOnTaskCount := 0

	for _, pr := range prs {
		if pr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
			runningCount++

			if pr.Status.PipelineSpec == nil && pr.Spec.PipelineRef != nil {
				waitingOnPipelineCount++
			}

			if pr.Status.ChildReferences != nil {
				for _, childRef := range pr.Status.ChildReferences {
					if childRef.Kind == "TaskRun" && childRef.PipelineTaskName != "" {
						waitingOnTaskCount++
					}
				}
			}
		}
	}

	r.runningPRsGauge.Add(ctx, float64(runningCount), metric.WithAttributes())
	r.runningPRsWaitingOnPipelineResolutionGauge.Add(ctx, float64(waitingOnPipelineCount), metric.WithAttributes())
	r.runningPRsWaitingOnTaskResolutionGauge.Add(ctx, float64(waitingOnTaskCount), metric.WithAttributes())

	return nil
}

// ReportRunningPipelineRuns invokes RunningPipelineRuns on our configured PeriodSeconds
// until the context is cancelled.
func (r *Recorder) ReportRunningPipelineRuns(ctx context.Context, lister listers.PipelineRunLister) {
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
			// Every 30s surface a metric for the number of running pipelines.
			if err := r.RunningPipelineRuns(ctx, lister); err != nil {
				// Log error but continue reporting
				// In a real implementation, you might want to use a logger here
			}
		}
	}
}

// Helper functions for tag insertion
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

func nilInsertTag(pipeline, pipelinerun string) []attribute.KeyValue {
	return []attribute.KeyValue{}
}

func getPipelineTagName(pr *v1.PipelineRun) string {
	if pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "" {
		return pr.Spec.PipelineRef.Name
	}
	if pr.Status.PipelineSpec != nil {
		return anonymous
	}
	return anonymous
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
