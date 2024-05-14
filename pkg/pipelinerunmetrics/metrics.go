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
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
)

var (
	pipelinerunTag = tag.MustNewKey("pipelinerun")
	pipelineTag    = tag.MustNewKey("pipeline")
	namespaceTag   = tag.MustNewKey("namespace")
	statusTag      = tag.MustNewKey("status")
	reasonTag      = tag.MustNewKey("reason")

	prDuration = stats.Float64(
		"pipelinerun_duration_seconds",
		"The pipelinerun execution time in seconds",
		stats.UnitDimensionless)
	prDurationView *view.View

	prCount = stats.Float64("pipelinerun_count",
		"number of pipelineruns",
		stats.UnitDimensionless)
	prCountView *view.View

	prTotal = stats.Float64("pipelinerun_total",
		"Number of pipelineruns",
		stats.UnitDimensionless)
	prTotalView *view.View

	runningPRsCount = stats.Float64("running_pipelineruns_count",
		"Number of pipelineruns executing currently",
		stats.UnitDimensionless)
	runningPRsCountView *view.View

	runningPRs = stats.Float64("running_pipelineruns",
		"Number of pipelineruns executing currently",
		stats.UnitDimensionless)
	runningPRsView *view.View

	runningPRsWaitingOnPipelineResolutionCount = stats.Float64("running_pipelineruns_waiting_on_pipeline_resolution_count",
		"Number of pipelineruns executing currently that are waiting on resolution requests for their pipeline references.",
		stats.UnitDimensionless)
	runningPRsWaitingOnPipelineResolutionCountView *view.View

	runningPRsWaitingOnPipelineResolution = stats.Float64("running_pipelineruns_waiting_on_pipeline_resolution",
		"Number of pipelineruns executing currently that are waiting on resolution requests for their pipeline references.",
		stats.UnitDimensionless)
	runningPRsWaitingOnPipelineResolutionView *view.View

	runningPRsWaitingOnTaskResolutionCount = stats.Float64("running_pipelineruns_waiting_on_task_resolution_count",
		"Number of pipelineruns executing currently that are waiting on resolution requests for the task references of their taskrun children.",
		stats.UnitDimensionless)
	runningPRsWaitingOnTaskResolutionCountView *view.View

	runningPRsWaitingOnTaskResolution = stats.Float64("running_pipelineruns_waiting_on_task_resolution",
		"Number of pipelineruns executing currently that are waiting on resolution requests for the task references of their taskrun children.",
		stats.UnitDimensionless)
	runningPRsWaitingOnTaskResolutionView *view.View
)

const (
	// ReasonCancelled indicates that a PipelineRun was cancelled.
	// Aliased for backwards compatibility; additional reasons should not be added here.
	ReasonCancelled = v1.PipelineRunReasonCancelled
)

// Recorder holds keys for Tekton metrics
type Recorder struct {
	mutex       sync.Mutex
	initialized bool

	insertTag func(pipeline,
		pipelinerun string) []tag.Mutator

	ReportingPeriod time.Duration
}

// We cannot register the view multiple times, so NewRecorder lazily
// initializes this singleton and returns the same recorder across any
// subsequent invocations.
var (
	once           sync.Once
	r              *Recorder
	errRegistering error
)

// NewRecorder creates a new metrics recorder instance
// to log the PipelineRun related metrics
func NewRecorder(ctx context.Context) (*Recorder, error) {
	once.Do(func() {
		r = &Recorder{
			initialized: true,

			// Default to 30s intervals.
			ReportingPeriod: 30 * time.Second,
		}

		cfg := config.FromContextOrDefaults(ctx)
		errRegistering = viewRegister(cfg.Metrics)
		if errRegistering != nil {
			r.initialized = false
			return
		}
	})

	return r, errRegistering
}

func viewRegister(cfg *config.Metrics) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var prunTag []tag.Key

	switch cfg.PipelinerunLevel {
	case config.PipelinerunLevelAtPipelinerun:
		prunTag = []tag.Key{pipelinerunTag, pipelineTag}
		r.insertTag = pipelinerunInsertTag
	case config.PipelinerunLevelAtPipeline:
		prunTag = []tag.Key{pipelineTag}
		r.insertTag = pipelineInsertTag
	case config.PipelinerunLevelAtNS:
		prunTag = []tag.Key{}
		r.insertTag = nilInsertTag
	default:
		return errors.New("invalid config for PipelinerunLevel: " + cfg.PipelinerunLevel)
	}

	distribution := view.Distribution(10, 30, 60, 300, 900, 1800, 3600, 5400, 10800, 21600, 43200, 86400)

	if cfg.PipelinerunLevel == config.PipelinerunLevelAtPipelinerun {
		distribution = view.LastValue()
	} else {
		switch cfg.DurationPipelinerunType {
		case config.DurationTaskrunTypeHistogram:
		case config.DurationTaskrunTypeLastValue:
			distribution = view.LastValue()
		default:
			return errors.New("invalid config for DurationTaskrunType: " + cfg.DurationTaskrunType)
		}
	}

	prCountViewTags := []tag.Key{statusTag}
	if cfg.CountWithReason {
		prCountViewTags = append(prCountViewTags, reasonTag)
		prunTag = append(prunTag, reasonTag)
	}

	prDurationView = &view.View{
		Description: prDuration.Description(),
		Measure:     prDuration,
		Aggregation: distribution,
		TagKeys:     append([]tag.Key{statusTag, namespaceTag}, prunTag...),
	}

	prCountView = &view.View{
		Description: prCount.Description(),
		Measure:     prCount,
		Aggregation: view.Count(),
		TagKeys:     prCountViewTags,
	}
	prTotalView = &view.View{
		Description: prTotal.Description(),
		Measure:     prTotal,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{statusTag},
	}

	runningPRsCountView = &view.View{
		Description: runningPRsCount.Description(),
		Measure:     runningPRsCount,
		Aggregation: view.LastValue(),
	}
	runningPRsView = &view.View{
		Description: runningPRs.Description(),
		Measure:     runningPRs,
		Aggregation: view.LastValue(),
	}

	runningPRsWaitingOnPipelineResolutionCountView = &view.View{
		Description: runningPRsWaitingOnPipelineResolutionCount.Description(),
		Measure:     runningPRsWaitingOnPipelineResolutionCount,
		Aggregation: view.LastValue(),
	}
	runningPRsWaitingOnPipelineResolutionView = &view.View{
		Description: runningPRsWaitingOnPipelineResolution.Description(),
		Measure:     runningPRsWaitingOnPipelineResolution,
		Aggregation: view.LastValue(),
	}

	runningPRsWaitingOnTaskResolutionCountView = &view.View{
		Description: runningPRsWaitingOnTaskResolutionCount.Description(),
		Measure:     runningPRsWaitingOnTaskResolutionCount,
		Aggregation: view.LastValue(),
	}
	runningPRsWaitingOnTaskResolutionView = &view.View{
		Description: runningPRsWaitingOnTaskResolution.Description(),
		Measure:     runningPRsWaitingOnTaskResolution,
		Aggregation: view.LastValue(),
	}

	return view.Register(
		prDurationView,
		prCountView,
		prTotalView,
		runningPRsCountView,
		runningPRsView,
		runningPRsWaitingOnPipelineResolutionCountView,
		runningPRsWaitingOnPipelineResolutionView,
		runningPRsWaitingOnTaskResolutionCountView,
		runningPRsWaitingOnTaskResolutionView,
	)
}

func viewUnregister() {
	view.Unregister(prDurationView,
		prCountView,
		prTotalView,
		runningPRsCountView,
		runningPRsView,
		runningPRsWaitingOnPipelineResolutionCountView,
		runningPRsWaitingOnPipelineResolutionView,
		runningPRsWaitingOnTaskResolutionCountView,
		runningPRsWaitingOnTaskResolutionView)
}

// MetricsOnStore returns a function that checks if metrics are configured for a config.Store, and registers it if so
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

func nilInsertTag(task, taskrun string) []tag.Mutator {
	return []tag.Mutator{}
}

// DurationAndCount logs the duration of PipelineRun execution and
// count for number of PipelineRuns succeed or failed
// returns an error if its failed to log the metrics
func (r *Recorder) DurationAndCount(pr *v1.PipelineRun, beforeCondition *apis.Condition) error {
	if !r.initialized {
		return fmt.Errorf("ignoring the metrics recording for %s , failed to initialize the metrics recorder", pr.Name)
	}

	afterCondition := pr.Status.GetCondition(apis.ConditionSucceeded)
	// To avoid recount
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

	cond := pr.Status.GetCondition(apis.ConditionSucceeded)
	status := "success"
	if cond.Status == corev1.ConditionFalse {
		status = "failed"
		if cond.Reason == v1.PipelineRunReasonCancelled.String() {
			status = "cancelled"
		}
	}
	reason := cond.Reason

	pipelineName := "anonymous"
	if pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "" {
		pipelineName = pr.Spec.PipelineRef.Name
	}
	ctx, err := tag.New(
		context.Background(),
		append([]tag.Mutator{tag.Insert(namespaceTag, pr.Namespace),
			tag.Insert(statusTag, status), tag.Insert(reasonTag, reason)}, r.insertTag(pipelineName, pr.Name)...)...)
	if err != nil {
		return err
	}

	metrics.Record(ctx, prDuration.M(duration.Seconds()))
	metrics.Record(ctx, prCount.M(1))
	metrics.Record(ctx, prTotal.M(1))

	return nil
}

// RunningPipelineRuns logs the number of PipelineRuns running right now
// returns an error if its failed to log the metrics
func (r *Recorder) RunningPipelineRuns(lister listers.PipelineRunLister) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.initialized {
		return errors.New("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	prs, err := lister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list pipelineruns while generating metrics : %w", err)
	}

	var runningPipelineRuns int
	var trsWaitResolvingTaskRef int
	var prsWaitResolvingPipelineRef int

	for _, pr := range prs {
		if !pr.IsDone() {
			runningPipelineRuns++
			succeedCondition := pr.Status.GetCondition(apis.ConditionSucceeded)
			if succeedCondition != nil && succeedCondition.Status == corev1.ConditionUnknown {
				switch succeedCondition.Reason {
				case v1.TaskRunReasonResolvingTaskRef:
					trsWaitResolvingTaskRef++
				case v1.PipelineRunReasonResolvingPipelineRef.String():
					prsWaitResolvingPipelineRef++
				}
			}
		}
	}

	ctx, err := tag.New(context.Background())
	if err != nil {
		return err
	}
	metrics.Record(ctx, runningPRsWaitingOnPipelineResolutionCount.M(float64(prsWaitResolvingPipelineRef)))
	metrics.Record(ctx, runningPRsWaitingOnPipelineResolution.M(float64(prsWaitResolvingPipelineRef)))
	metrics.Record(ctx, runningPRsWaitingOnTaskResolutionCount.M(float64(trsWaitResolvingTaskRef)))
	metrics.Record(ctx, runningPRsWaitingOnTaskResolution.M(float64(trsWaitResolvingTaskRef)))
	metrics.Record(ctx, runningPRsCount.M(float64(runningPipelineRuns)))
	metrics.Record(ctx, runningPRs.M(float64(runningPipelineRuns)))

	return nil
}

// ReportRunningPipelineRuns invokes RunningPipelineRuns on our configured PeriodSeconds
// until the context is cancelled.
func (r *Recorder) ReportRunningPipelineRuns(ctx context.Context, lister listers.PipelineRunLister) {
	logger := logging.FromContext(ctx)

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
			if err := r.RunningPipelineRuns(lister); err != nil {
				logger.Warnf("Failed to log the metrics : %v", err)
			}
		}
	}
}
