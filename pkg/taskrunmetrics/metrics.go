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
	"encoding/hex"
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
	"golang.org/x/crypto/blake2b"
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
	trTotalView                                *view.View
	runningTRsView                             *view.View
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

	trTotal = stats.Float64("taskrun_total",
		"Number of taskruns",
		stats.UnitDimensionless)

	runningTRs = stats.Float64("running_taskruns",
		"Number of taskruns executing currently",
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
	cfg         *config.Metrics

	ReportingPeriod time.Duration

	insertTaskTag func(task,
		taskrun string) []tag.Mutator

	insertPipelineTag func(pipeline,
		pipelinerun string) []tag.Mutator

	hash string
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
		cfg := config.FromContextOrDefaults(ctx)
		r = &Recorder{
			initialized: true,
			cfg:         cfg.Metrics,

			// Default to reporting metrics every 30s.
			ReportingPeriod: 30 * time.Second,
		}

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

	if cfg.CountWithReason {
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

	trTotalView = &view.View{
		Description: trTotal.Description(),
		Measure:     trTotal,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{statusTag},
	}

	runningTRsView = &view.View{
		Description: runningTRs.Description(),
		Measure:     runningTRs,
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
		trTotalView,
		runningTRsView,
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
		trTotalView,
		runningTRsView,
		runningTRsWaitingOnTaskResolutionCountView,
		runningTRsThrottledByQuotaView,
		runningTRsThrottledByNodeView,
		podLatencyView,
	)
}

// OnStore returns a function that checks if metrics are configured for a config.Store, and registers it if so
func OnStore(logger *zap.SugaredLogger, r *Recorder) func(name string, value interface{}) {
	return func(name string, value interface{}) {
		if name == config.GetMetricsConfigName() {
			cfg, ok := value.(*config.Metrics)
			if !ok {
				logger.Error("Failed to do type insertion for extracting metrics config")
				return
			}
			updated := r.updateConfig(cfg)
			if !updated {
				return
			}
			// Update metrics according to the configuration
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
	return []tag.Mutator{
		tag.Insert(pipelineTag, pipeline),
		tag.Insert(pipelinerunTag, pipelinerun),
	}
}

func pipelineInsertTag(pipeline, pipelinerun string) []tag.Mutator {
	return []tag.Mutator{tag.Insert(pipelineTag, pipeline)}
}

func taskrunInsertTag(task, taskrun string) []tag.Mutator {
	return []tag.Mutator{
		tag.Insert(taskTag, task),
		tag.Insert(taskrunTag, taskrun),
	}
}

func taskInsertTag(task, taskrun string) []tag.Mutator {
	return []tag.Mutator{tag.Insert(taskTag, task)}
}

func nilInsertTag(task, taskrun string) []tag.Mutator {
	return []tag.Mutator{}
}

func getTaskTagName(tr *v1.TaskRun) string {
	taskName := anonymous
	switch {
	case tr.Spec.TaskRef != nil && len(tr.Spec.TaskRef.Name) > 0:
		taskName = tr.Spec.TaskRef.Name
	case tr.Spec.TaskSpec != nil:
		pipelineTaskTable, hasPipelineTaskTable := tr.Labels[pipeline.PipelineTaskLabelKey]
		if hasPipelineTaskTable && len(pipelineTaskTable) > 0 {
			taskName = pipelineTaskTable
		}
	default:
		if len(tr.Labels) > 0 {
			taskLabel, hasTaskLabel := tr.Labels[pipeline.TaskLabelKey]
			if hasTaskLabel && len(taskLabel) > 0 {
				taskName = taskLabel
			}
		}
	}

	return taskName
}

func (r *Recorder) updateConfig(cfg *config.Metrics) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var hash string
	if cfg != nil {
		s := fmt.Sprintf("%v", *cfg)
		sum := blake2b.Sum256([]byte(s))
		hash = hex.EncodeToString(sum[:])
	}

	if r.hash == hash {
		return false
	}

	r.cfg = cfg
	r.hash = hash

	return true
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

	taskName := getTaskTagName(tr)

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

	addNamespaceLabelToQuotaThrottleMetric := r.cfg != nil && r.cfg.ThrottleWithNamespace

	var runningTrs int
	trsThrottledByQuota := map[string]int{}
	trsThrottledByNode := map[string]int{}
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
				cnt := trsThrottledByQuota[pr.Namespace]
				cnt++
				trsThrottledByQuota[pr.Namespace] = cnt
			case pod.ReasonExceededNodeResources:
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
	metrics.Record(ctx, runningTRs.M(float64(runningTrs)))
	metrics.Record(ctx, runningTRsWaitingOnTaskResolutionCount.M(float64(trsWaitResolvingTaskRef)))

	for ns, cnt := range trsThrottledByQuota {
		var mutators []tag.Mutator
		if addNamespaceLabelToQuotaThrottleMetric {
			mutators = []tag.Mutator{tag.Insert(namespaceTag, ns)}
		}
		ctx, err := tag.New(ctx, mutators...)
		if err != nil {
			return err
		}
		metrics.Record(ctx, runningTRsThrottledByQuota.M(float64(cnt)))
	}
	for ns, cnt := range trsThrottledByNode {
		var mutators []tag.Mutator
		if addNamespaceLabelToQuotaThrottleMetric {
			mutators = []tag.Mutator{tag.Insert(namespaceTag, ns)}
		}
		ctx, err := tag.New(ctx, mutators...)
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
	taskName := getTaskTagName(tr)

	ctx, err := tag.New(
		ctx,
		append([]tag.Mutator{
			tag.Insert(namespaceTag, tr.Namespace),
			tag.Insert(podTag, pod.Name),
		},
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
