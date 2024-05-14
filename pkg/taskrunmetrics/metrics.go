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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/pod"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
)

const anonymous = "anonymous"

var (
	pipelinerunTag = tag.MustNewKey("pipelinerun")
	pipelineTag    = tag.MustNewKey("pipeline")
	taskrunTag     = tag.MustNewKey("taskrun")
	taskTag        = tag.MustNewKey("task")
	namespaceTag   = tag.MustNewKey("namespace")
	statusTag      = tag.MustNewKey("status")
	reasonTag      = tag.MustNewKey("reason")
	podTag         = tag.MustNewKey("pod")

	trDurationView                             *view.View
	prTRDurationView                           *view.View
	trCountView                                *view.View
	trTotalView                                *view.View
	runningTRsCountView                        *view.View
	runningTRsView                             *view.View
	runningTRsThrottledByQuotaCountView        *view.View
	runningTRsThrottledByNodeCountView         *view.View
	runningTRsThrottledByQuotaView             *view.View
	runningTRsThrottledByNodeView              *view.View
	runningTRsWaitingOnTaskResolutionCountView *view.View
	podLatencyView                             *view.View

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

	trTotal = stats.Float64("taskrun_total",
		"Number of taskruns",
		stats.UnitDimensionless)

	runningTRsCount = stats.Float64("running_taskruns_count",
		"Number of taskruns executing currently",
		stats.UnitDimensionless)

	runningTRs = stats.Float64("running_taskruns",
		"Number of taskruns executing currently",
		stats.UnitDimensionless)

	runningTRsThrottledByQuotaCount = stats.Float64("running_taskruns_throttled_by_quota_count",
		"Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of defined ResourceQuotas.  Such suspensions can occur as part of initial scheduling of the Pod, or scheduling of any of the subsequent Container(s) in the Pod after the first Container is started",
		stats.UnitDimensionless)

	runningTRsThrottledByNodeCount = stats.Float64("running_taskruns_throttled_by_node_count",
		"Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of Node level constraints. Such suspensions can occur as part of initial scheduling of the Pod, or scheduling of any of the subsequent Container(s) in the Pod after the first Container is started",
		stats.UnitDimensionless)

	runningTRsWaitingOnTaskResolutionCount = stats.Float64("running_taskruns_waiting_on_task_resolution_count",
		"Number of taskruns executing currently that are waiting on resolution requests for their task references.",
		stats.UnitDimensionless)

	runningTRsThrottledByQuota = stats.Float64("running_taskruns_throttled_by_quota",
		"Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of defined ResourceQuotas.  Such suspensions can occur as part of initial scheduling of the Pod, or scheduling of any of the subsequent Container(s) in the Pod after the first Container is started",
		stats.UnitDimensionless)

	runningTRsThrottledByNode = stats.Float64("running_taskruns_throttled_by_node",
		"Number of taskruns executing currently, but whose underlying Pods or Containers are suspended by k8s because of Node level constraints. Such suspensions can occur as part of initial scheduling of the Pod, or scheduling of any of the subsequent Container(s) in the Pod after the first Container is started",
		stats.UnitDimensionless)

	podLatency = stats.Float64("taskruns_pod_latency_milliseconds",
		"scheduling latency for the taskruns pods",
		stats.UnitMilliseconds)
)

// Recorder is used to actually record TaskRun metrics
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
	once           sync.Once
	r              *Recorder
	errRegistering error
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

	var trunTag []tag.Key
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

	trCountViewTags := []tag.Key{statusTag}
	if cfg.CountWithReason {
		trCountViewTags = append(trCountViewTags, reasonTag)
		trunTag = append(trunTag, reasonTag)
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
		TagKeys:     trCountViewTags,
	}
	trTotalView = &view.View{
		Description: trTotal.Description(),
		Measure:     trTotal,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{statusTag},
	}
	runningTRsCountView = &view.View{
		Description: runningTRsCount.Description(),
		Measure:     runningTRsCount,
		Aggregation: view.LastValue(),
	}

	runningTRsView = &view.View{
		Description: runningTRs.Description(),
		Measure:     runningTRs,
		Aggregation: view.LastValue(),
	}
	runningTRsThrottledByQuotaCountView = &view.View{
		Description: runningTRsThrottledByQuotaCount.Description(),
		Measure:     runningTRsThrottledByQuotaCount,
		Aggregation: view.LastValue(),
	}
	runningTRsThrottledByNodeCountView = &view.View{
		Description: runningTRsThrottledByNodeCount.Description(),
		Measure:     runningTRsThrottledByNodeCount,
		Aggregation: view.LastValue(),
	}
	runningTRsWaitingOnTaskResolutionCountView = &view.View{
		Description: runningTRsWaitingOnTaskResolutionCount.Description(),
		Measure:     runningTRsWaitingOnTaskResolutionCount,
		Aggregation: view.LastValue(),
	}

	throttleViewTags := []tag.Key{}
	if cfg.ThrottleWithNamespace {
		throttleViewTags = append(throttleViewTags, namespaceTag)
	}
	runningTRsThrottledByQuotaView = &view.View{
		Description: runningTRsThrottledByQuota.Description(),
		Measure:     runningTRsThrottledByQuota,
		Aggregation: view.LastValue(),
		TagKeys:     throttleViewTags,
	}
	runningTRsThrottledByNodeView = &view.View{
		Description: runningTRsThrottledByNode.Description(),
		Measure:     runningTRsThrottledByNode,
		Aggregation: view.LastValue(),
		TagKeys:     throttleViewTags,
	}
	podLatencyView = &view.View{
		Description: podLatency.Description(),
		Measure:     podLatency,
		Aggregation: view.LastValue(),
		TagKeys:     append([]tag.Key{namespaceTag, podTag}, trunTag...),
	}
	return view.Register(
		trDurationView,
		prTRDurationView,
		trCountView,
		trTotalView,
		runningTRsCountView,
		runningTRsView,
		runningTRsThrottledByQuotaCountView,
		runningTRsThrottledByNodeCountView,
		runningTRsWaitingOnTaskResolutionCountView,
		runningTRsThrottledByQuotaView,
		runningTRsThrottledByNodeView,
		podLatencyView,
	)
}

func viewUnregister() {
	view.Unregister(
		trDurationView,
		prTRDurationView,
		trCountView,
		trTotalView,
		runningTRsCountView,
		runningTRsView,
		runningTRsThrottledByQuotaCountView,
		runningTRsThrottledByNodeCountView,
		runningTRsWaitingOnTaskResolutionCountView,
		runningTRsThrottledByQuotaView,
		runningTRsThrottledByNodeView,
		podLatencyView,
	)
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

	taskName := anonymous
	if tr.Spec.TaskRef != nil {
		taskName = tr.Spec.TaskRef.Name
	}

	cond := tr.Status.GetCondition(apis.ConditionSucceeded)
	status := "success"
	if cond.Status == corev1.ConditionFalse {
		status = "failed"
	}
	reason := cond.Reason

	durationStat := trDuration
	tags := []tag.Mutator{tag.Insert(namespaceTag, tr.Namespace), tag.Insert(statusTag, status), tag.Insert(reasonTag, reason)}
	if ok, pipeline, pipelinerun := IsPartOfPipeline(tr); ok {
		durationStat = prTRDuration
		tags = append(tags, r.insertPipelineTag(pipeline, pipelinerun)...)
	}
	tags = append(tags, r.insertTaskTag(taskName, tr.Name)...)

	ctx, err := tag.New(ctx, tags...)
	if err != nil {
		return err
	}

	metrics.Record(ctx, durationStat.M(duration.Seconds()))
	metrics.Record(ctx, trCount.M(1))
	metrics.Record(ctx, trTotal.M(1))

	return nil
}

// RunningTaskRuns logs the number of TaskRuns running right now
// returns an error if its failed to log the metrics
func (r *Recorder) RunningTaskRuns(ctx context.Context, lister listers.TaskRunLister) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.initialized {
		return errors.New("ignoring the metrics recording, failed to initialize the metrics recorder")
	}

	trs, err := lister.List(labels.Everything())
	if err != nil {
		return err
	}

	cfg := config.FromContextOrDefaults(ctx)
	addNamespaceLabelToQuotaThrottleMetric := cfg.Metrics != nil && cfg.Metrics.ThrottleWithNamespace

	var runningTrs int
	trsThrottledByQuota := map[string]int{}
	trsThrottledByQuotaCount := 0
	trsThrottledByNode := map[string]int{}
	trsThrottledByNodeCount := 0
	var trsWaitResolvingTaskRef int
	for _, pr := range trs {
		// initialize metrics with namespace tag to zero if unset; will then update as needed below
		_, ok := trsThrottledByQuota[pr.Namespace]
		if !ok {
			trsThrottledByQuota[pr.Namespace] = 0
		}
		_, ok = trsThrottledByNode[pr.Namespace]
		if !ok {
			trsThrottledByNode[pr.Namespace] = 0
		}

		if pr.IsDone() {
			continue
		}
		runningTrs++

		succeedCondition := pr.Status.GetCondition(apis.ConditionSucceeded)
		if succeedCondition != nil && succeedCondition.Status == corev1.ConditionUnknown {
			switch succeedCondition.Reason {
			case pod.ReasonExceededResourceQuota:
				trsThrottledByQuotaCount++
				cnt := trsThrottledByQuota[pr.Namespace]
				cnt++
				trsThrottledByQuota[pr.Namespace] = cnt
			case pod.ReasonExceededNodeResources:
				trsThrottledByNodeCount++
				cnt := trsThrottledByNode[pr.Namespace]
				cnt++
				trsThrottledByNode[pr.Namespace] = cnt
			case v1.TaskRunReasonResolvingTaskRef:
				trsWaitResolvingTaskRef++
			}
		}
	}

	ctx, err = tag.New(ctx)
	if err != nil {
		return err
	}
	metrics.Record(ctx, runningTRsCount.M(float64(runningTrs)))
	metrics.Record(ctx, runningTRs.M(float64(runningTrs)))
	metrics.Record(ctx, runningTRsWaitingOnTaskResolutionCount.M(float64(trsWaitResolvingTaskRef)))
	metrics.Record(ctx, runningTRsThrottledByQuotaCount.M(float64(trsThrottledByQuotaCount)))
	metrics.Record(ctx, runningTRsThrottledByNodeCount.M(float64(trsThrottledByNodeCount)))

	for ns, cnt := range trsThrottledByQuota {
		var mutators []tag.Mutator
		if addNamespaceLabelToQuotaThrottleMetric {
			mutators = []tag.Mutator{tag.Insert(namespaceTag, ns)}
		}
		ctx, err = tag.New(ctx, mutators...)
		if err != nil {
			return err
		}
		metrics.Record(ctx, runningTRsThrottledByQuota.M(float64(cnt)))
	}
	for ns, cnt := range trsThrottledByNode {
		ctx, err = tag.New(ctx, []tag.Mutator{tag.Insert(namespaceTag, ns)}...)
		if err != nil {
			return err
		}
		metrics.Record(ctx, runningTRsThrottledByNode.M(float64(cnt)))
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
			// When the context is cancelled, stop reporting.
			if !delay.Stop() {
				<-delay.C
			}
			return

		case <-delay.C:
			// Every 30s surface a metric for the number of running tasks, as well as those running tasks that are currently throttled by k8s,
			// and those running tasks waiting on task reference resolution
			if err := r.RunningTaskRuns(ctx, lister); err != nil {
				logger.Warnf("Failed to log the metrics : %v", err)
			}
		}
	}
}

// RecordPodLatency logs the duration required to schedule the pod for TaskRun
// returns an error if its failed to log the metrics
func (r *Recorder) RecordPodLatency(ctx context.Context, pod *corev1.Pod, tr *v1.TaskRun) error {
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
	taskName := anonymous
	if tr.Spec.TaskRef != nil {
		taskName = tr.Spec.TaskRef.Name
	}

	ctx, err := tag.New(
		ctx,
		append([]tag.Mutator{tag.Insert(namespaceTag, tr.Namespace),
			tag.Insert(podTag, pod.Name)},
			r.insertTaskTag(taskName, tr.Name)...)...)
	if err != nil {
		return err
	}

	metrics.Record(ctx, podLatency.M(float64(latency.Milliseconds())))

	return nil
}

// IsPartOfPipeline return true if TaskRun is a part of a Pipeline.
// It also return the name of Pipeline and PipelineRun
func IsPartOfPipeline(tr *v1.TaskRun) (bool, string, string) {
	pipelineLabel, hasPipelineLabel := tr.Labels[pipeline.PipelineLabelKey]
	pipelineRunLabel, hasPipelineRunLabel := tr.Labels[pipeline.PipelineRunLabelKey]

	if hasPipelineLabel && hasPipelineRunLabel {
		return true, pipelineLabel, pipelineRunLabel
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
