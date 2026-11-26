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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

const (
	// ReasonCancelled indicates that a PipelineRun was cancelled.
	// Aliased for backwards compatibility; additional reasons should not be added here.
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
	prDurationGauge                            metric.Float64Gauge
	prResolutionWaitHistogram                  metric.Float64Histogram
	prTotalCounter                             metric.Int64Counter
	runningPRsGauge                            metric.Int64ObservableGauge
	runningPRsWaitingOnPipelineResolutionGauge metric.Int64ObservableGauge
	runningPRsWaitingOnTaskResolutionGauge     metric.Int64ObservableGauge

	insertTag func(pipeline, pipelinerun string) []attribute.KeyValue

	// resolvingSince and resolutionWaitSeconds track, per PipelineRun UID, how
	// long a run has been waiting on resolution of its Pipeline reference.
	//
	// Status.Conditions only ever holds the *current* reason for a given
	// condition type (see knative.dev/pkg/apis ConditionSet.SetCondition) — once
	// the Succeeded condition's reason moves past
	// PipelineRunReasonResolvingPipelineRef, the LastTransitionTime at which
	// resolution started is gone. ObserveResolutionWait is called on every
	// reconcile to capture that timestamp while it is still visible, and to
	// compute+stash the elapsed wait once the run leaves the resolving reason.
	// DurationAndCount consumes (and clears) the stashed value when the run
	// finishes, so the metric can carry the run's final status label.
	//
	// Entries are removed as soon as they're consumed. A run that is deleted
	// while still resolving (and thus never reaches DurationAndCount) leaves a
	// small, bounded leak in resolvingSince; this is an accepted trade-off to
	// avoid writing bookkeeping data onto the PipelineRun object itself.
	resolvingSince        map[types.UID]time.Time
	resolutionWaitSeconds map[types.UID]float64
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
			initialized:           true,
			cfg:                   cfg.Metrics,
			resolvingSince:        make(map[types.UID]time.Time),
			resolutionWaitSeconds: make(map[types.UID]float64),
		}

		errRegistering = r.configure(cfg.Metrics)
		if errRegistering != nil {
			r.initialized = false
			return
		}
	})
	return r, errRegistering
}

func (r *Recorder) configure(cfg *config.Metrics) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.meter == nil {
		r.meter = otel.GetMeterProvider().Meter("tekton_pipelines_controller")
	}

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

	// Configure Duration Measure
	if cfg.DurationPipelinerunType == config.DurationPipelinerunTypeLastValue {
		if r.prDurationGauge == nil {
			prDurationGauge, err := r.meter.Float64Gauge(
				"tekton_pipelines_controller_pipelinerun_duration_seconds",
				metric.WithDescription("The pipelinerun execution time in seconds"),
				metric.WithUnit("s"),
			)
			if err != nil {
				return fmt.Errorf("failed to create pipelinerun duration gauge: %w", err)
			}
			r.prDurationGauge = prDurationGauge
		}
		r.prDurationHistogram = nil
	} else {
		if r.prDurationHistogram == nil {
			prDurationHistogram, err := r.meter.Float64Histogram(
				"tekton_pipelines_controller_pipelinerun_duration_seconds",
				metric.WithDescription("The pipelinerun execution time in seconds"),
				metric.WithUnit("s"),
				metric.WithExplicitBucketBoundaries(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400),
			)
			if err != nil {
				return fmt.Errorf("failed to create pipelinerun duration histogram: %w", err)
			}
			r.prDurationHistogram = prDurationHistogram
		}
		r.prDurationGauge = nil
	}

	if r.prResolutionWaitHistogram == nil {
		prResolutionWaitHistogram, err := r.meter.Float64Histogram(
			"tekton_pipelines_controller_pipelinerun_resolution_wait_seconds",
			metric.WithDescription("The time a pipelinerun spent waiting on resolution of its Pipeline reference, in seconds. Runs that never entered the resolving state (e.g. inline Pipeline specs) record 0."),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600),
		)
		if err != nil {
			return fmt.Errorf("failed to create pipelinerun resolution wait histogram: %w", err)
		}
		r.prResolutionWaitHistogram = prResolutionWaitHistogram
	}

	prTotalCounter, err := r.meter.Int64Counter(
		"tekton_pipelines_controller_pipelinerun_total",
		metric.WithDescription("Number of pipelineruns"),
	)
	if err != nil {
		return fmt.Errorf("failed to create pipelinerun total counter: %w", err)
	}
	r.prTotalCounter = prTotalCounter

	runningPRsGauge, err := r.meter.Int64ObservableGauge(
		"tekton_pipelines_controller_running_pipelineruns",
		metric.WithDescription("Number of pipelineruns executing currently"),
	)
	if err != nil {
		return fmt.Errorf("failed to create running pipelineruns gauge: %w", err)
	}
	r.runningPRsGauge = runningPRsGauge

	runningPRsWaitingOnPipelineResolutionGauge, err := r.meter.Int64ObservableGauge(
		"tekton_pipelines_controller_running_pipelineruns_waiting_on_pipeline_resolution",
		metric.WithDescription("Number of pipelineruns executing currently that are waiting on resolution requests for their pipeline references."),
	)
	if err != nil {
		return fmt.Errorf("failed to create running pipelineruns waiting on pipeline resolution gauge: %w", err)
	}
	r.runningPRsWaitingOnPipelineResolutionGauge = runningPRsWaitingOnPipelineResolutionGauge

	runningPRsWaitingOnTaskResolutionGauge, err := r.meter.Int64ObservableGauge(
		"tekton_pipelines_controller_running_pipelineruns_waiting_on_task_resolution",
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

	duration := time.Duration(0)
	if pr.Status.StartTime != nil {
		duration = time.Since(pr.Status.StartTime.Time)
		if pr.Status.CompletionTime != nil {
			duration = pr.Status.CompletionTime.Sub(pr.Status.StartTime.Time)
		}
	}

	pipelineName := getPipelineTagName(pr)

	cond := pr.Status.GetCondition(apis.ConditionSucceeded)
	status := "success"
	if cond.Status == corev1.ConditionFalse {
		status = "failed"
		if cond.Reason == v1.PipelineRunReasonCancelled.String() {
			status = "cancelled"
		}
	}
	reason := cond.Reason

	attrs := []attribute.KeyValue{
		attribute.String("namespace", pr.Namespace),
		attribute.String("status", status),
	}

	if r.cfg.CountWithReason {
		attrs = append(attrs, attribute.String("reason", reason))
	}

	if r.insertTag != nil {
		attrs = append(attrs, r.insertTag(pipelineName, pr.Name)...)
	}

	if r.prDurationGauge != nil {
		r.prDurationGauge.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	} else if r.prDurationHistogram != nil {
		r.prDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}

	if r.prResolutionWaitHistogram != nil {
		waitSeconds := r.resolutionWaitSeconds[pr.UID]
		delete(r.resolutionWaitSeconds, pr.UID)
		r.prResolutionWaitHistogram.Record(ctx, waitSeconds, metric.WithAttributes(attrs...))
	}

	r.prTotalCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))

	return nil
}

// ObserveResolutionWait tracks how long pr has been waiting on resolution of
// its Pipeline reference. It should be called on every reconcile of pr,
// after the reconcile logic has run (so it observes the up-to-date
// condition). While the Succeeded condition's reason is
// PipelineRunReasonResolvingPipelineRef, it records the point in time
// resolution started (idempotently — LastTransitionTime does not change
// while the reason is unchanged). Once the reason moves past resolving, it
// computes the elapsed wait and stashes it for DurationAndCount to consume
// when the run finishes. Runs that never enter the resolving state never
// populate an entry, so DurationAndCount records 0 for them.
func (r *Recorder) ObserveResolutionWait(pr *v1.PipelineRun) {
	if r == nil || !r.initialized || pr == nil {
		return
	}

	cond := pr.Status.GetCondition(apis.ConditionSucceeded)
	if cond == nil {
		return
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if cond.Reason == v1.PipelineRunReasonResolvingPipelineRef.String() {
		r.resolvingSince[pr.UID] = cond.LastTransitionTime.Inner.Time
		return
	}

	if startedAt, ok := r.resolvingSince[pr.UID]; ok {
		r.resolutionWaitSeconds[pr.UID] = time.Since(startedAt).Seconds()
		delete(r.resolvingSince, pr.UID)
	}
}

// observeRunningPipelineRuns logs the number of PipelineRuns running right now
func (r *Recorder) observeRunningPipelineRuns(ctx context.Context, o metric.Observer, lister listers.PipelineRunLister) error {
	if !r.initialized {
		return errors.New("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	r.mutex.Lock()
	cfg := r.cfg
	runningPRsGauge := r.runningPRsGauge
	waitingOnPipelineGauge := r.runningPRsWaitingOnPipelineResolutionGauge
	waitingOnTaskGauge := r.runningPRsWaitingOnTaskResolutionGauge
	r.mutex.Unlock()

	prs, err := lister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list pipelineruns: %w", err)
	}

	// This map holds the new counts for the current interval.
	currentCounts := make(map[attribute.Set]int64)
	var waitingOnPipelineCount int64
	var waitingOnTaskCount int64

	for _, pr := range prs {
		succeedCondition := pr.Status.GetCondition(apis.ConditionSucceeded)
		if succeedCondition == nil || !succeedCondition.IsUnknown() {
			continue
		}

		// Handle waiting metrics (these are cluster-wide, no extra attributes).
		switch succeedCondition.Reason {
		case v1.PipelineRunReasonResolvingPipelineRef.String():
			waitingOnPipelineCount++
		case v1.TaskRunReasonResolvingTaskRef:
			waitingOnTaskCount++
		}

		// Handle running_pipelineruns metric with per-level aggregation.
		pipelineName := getPipelineTagName(pr)
		var attrs []attribute.KeyValue

		// Build attributes based on configured level.
		switch cfg.RunningPipelinerunLevel {
		case config.PipelinerunLevelAtPipelinerun:
			attrs = append(attrs, attribute.String("pipelinerun", pr.Name))
			fallthrough
		case config.PipelinerunLevelAtPipeline:
			attrs = append(attrs, attribute.String("pipeline", pipelineName))
			fallthrough
		case config.PipelinerunLevelAtNS:
			attrs = append(attrs, attribute.String("namespace", pr.Namespace))
		}

		attrSet := attribute.NewSet(attrs...)
		currentCounts[attrSet]++
	}

	// Report running_pipelineruns.
	// At cluster level the loop already emits the empty-attribute observation,
	// so only emit the cluster-level total when per-level observations were made.
	totalRunningPRs := int64(0)
	for attrSet, value := range currentCounts {
		o.ObserveInt64(runningPRsGauge, value, metric.WithAttributes(attrSet.ToSlice()...))
		totalRunningPRs += value
	}
	if _, clusterAlreadyReported := currentCounts[attribute.NewSet()]; !clusterAlreadyReported {
		o.ObserveInt64(runningPRsGauge, totalRunningPRs)
	}

	o.ObserveInt64(waitingOnPipelineGauge, waitingOnPipelineCount)
	o.ObserveInt64(waitingOnTaskGauge, waitingOnTaskCount)

	return nil
}

// ReportRunningPipelineRuns invokes observeRunningPipelineRuns on our configured PeriodSeconds
// until the context is cancelled.
func (r *Recorder) ReportRunningPipelineRuns(ctx context.Context, lister listers.PipelineRunLister) {
	logger := logging.FromContext(ctx)

	_, err := r.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		return r.observeRunningPipelineRuns(ctx, o, lister)
	}, r.runningPRsGauge, r.runningPRsWaitingOnPipelineResolutionGauge, r.runningPRsWaitingOnTaskResolutionGauge)
	if err != nil {
		logger.Errorf("failed to register callback for running pipelineruns: %v", err)
		return
	}

	<-ctx.Done()
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
	pipelineName := anonymous
	switch {
	case pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "":
		pipelineName = pr.Spec.PipelineRef.Name
	case pr.Spec.PipelineSpec != nil:
		if pr.ObjectMeta.GenerateName == "" {
			// anonymous — inline spec, no external pipeline name and no generic name
			break
		}
		fallthrough
	default:
		if label, ok := pr.Labels[pipeline.PipelineLabelKey]; ok && label != "" {
			pipelineName = label
		}
	}
	return pipelineName
}

// updateConfig atomically checks whether cfg differs from the current config and,
// if so, commits it. It returns the committed cfg so the caller can pass the
// exact same value to configure, eliminating the window between the two calls.
// Returns nil when the config is unchanged.
func (r *Recorder) updateConfig(cfg *config.Metrics) *config.Metrics {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if equality.Semantic.DeepEqual(r.cfg, cfg) {
		return nil
	}
	r.cfg = cfg
	return cfg
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
		if accepted := recorder.updateConfig(cfg); accepted != nil {
			if err := recorder.configure(accepted); err != nil {
				logger.Errorw("failed to configure recorder", "error", err)
			}
		}
	}
}
