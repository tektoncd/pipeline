/*
Copyright 2018 The Knative Authors

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

package controller

import (
	"context"
	"errors"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	kubemetrics "k8s.io/client-go/tools/metrics"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/metrics"
)

var (
	workQueueDepthStat   = stats.Int64("work_queue_depth", "Depth of the work queue", stats.UnitNone)
	reconcileCountStat   = stats.Int64("reconcile_count", "Number of reconcile operations", stats.UnitNone)
	reconcileLatencyStat = stats.Int64("reconcile_latency", "Latency of reconcile operations", stats.UnitMilliseconds)

	// reconcileDistribution defines the bucket boundaries for the histogram of reconcile latency metric.
	// Bucket boundaries are 10ms, 100ms, 1s, 10s, 30s and 60s.
	reconcileDistribution = view.Distribution(10, 100, 1000, 10000, 30000, 60000)

	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	reconcilerTagKey = tag.MustNewKey("reconciler")
	keyTagKey        = tag.MustNewKey("key")
	successTagKey    = tag.MustNewKey("success")
)

func init() {
	// Register to receive metrics from kubernetes workqueues.
	wp := &metrics.WorkqueueProvider{
		Adds: stats.Int64(
			"workqueue_adds_total",
			"Total number of adds handled by workqueue",
			stats.UnitNone,
		),
		Depth: stats.Int64(
			"workqueue_depth",
			"Current depth of workqueue",
			stats.UnitNone,
		),
		Latency: stats.Float64(
			"workqueue_queue_latency_seconds",
			"How long in seconds an item stays in workqueue before being requested.",
			"s",
		),
		Retries: stats.Int64(
			"workqueue_retries_total",
			"Total number of retries handled by workqueue",
			"s",
		),
		WorkDuration: stats.Float64(
			"workqueue_work_duration_seconds",
			"How long in seconds processing an item from workqueue takes.",
			"s",
		),
		UnfinishedWorkSeconds: stats.Float64(
			"workqueue_unfinished_work_seconds",
			"How long in seconds the outstanding workqueue items have been in flight (total).",
			"s",
		),
		LongestRunningProcessorSeconds: stats.Float64(
			"workqueue_longest_running_processor_seconds",
			"How long in seconds the longest outstanding workqueue item has been in flight.",
			"s",
		),
	}
	workqueue.SetProvider(wp)

	// Register to receive metrics from kubernetes reflectors (what powers informers)
	// NOTE: today these don't actually seem to wire up to anything in Kubernetes.
	rp := &metrics.ReflectorProvider{
		ItemsInList: stats.Float64(
			"reflector_items_in_list",
			"How many items an API list returns to the reflectors",
			stats.UnitNone,
		),
		// TODO(mattmoor): This is not in the latest version, so it will
		// be removed in a future version.
		ItemsInMatch: stats.Float64(
			"reflector_items_in_match",
			"",
			stats.UnitNone,
		),
		ItemsInWatch: stats.Float64(
			"reflector_items_in_watch",
			"How many items an API watch returns to the reflectors",
			stats.UnitNone,
		),
		LastResourceVersion: stats.Float64(
			"reflector_last_resource_version",
			"Last resource version seen for the reflectors",
			stats.UnitNone,
		),
		ListDuration: stats.Float64(
			"reflector_list_duration_seconds",
			"How long an API list takes to return and decode for the reflectors",
			stats.UnitNone,
		),
		Lists: stats.Int64(
			"reflector_lists_total",
			"Total number of API lists done by the reflectors",
			stats.UnitNone,
		),
		ShortWatches: stats.Int64(
			"reflector_short_watches_total",
			"Total number of short API watches done by the reflectors",
			stats.UnitNone,
		),
		WatchDuration: stats.Float64(
			"reflector_watch_duration_seconds",
			"How long an API watch takes to return and decode for the reflectors",
			stats.UnitNone,
		),
		Watches: stats.Int64(
			"reflector_watches_total",
			"Total number of API watches done by the reflectors",
			stats.UnitNone,
		),
	}
	cache.SetReflectorMetricsProvider(rp)

	cp := &metrics.ClientProvider{
		Latency: stats.Float64(
			"client_latency",
			"How long Kubernetes API requests take",
			"s",
		),
		Result: stats.Int64(
			"client_results",
			"Total number of API requests (broken down by status code)",
			stats.UnitNone,
		),
	}
	kubemetrics.Register(cp.NewLatencyMetric(), cp.NewResultMetric())

	views := []*view.View{{
		Description: "Depth of the work queue",
		Measure:     workQueueDepthStat,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{reconcilerTagKey},
	}, {
		Description: "Number of reconcile operations",
		Measure:     reconcileCountStat,
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{reconcilerTagKey, keyTagKey, successTagKey},
	}, {
		Description: "Latency of reconcile operations",
		Measure:     reconcileLatencyStat,
		Aggregation: reconcileDistribution,
		TagKeys:     []tag.Key{reconcilerTagKey, keyTagKey, successTagKey},
	}}
	views = append(views, wp.DefaultViews()...)
	views = append(views, rp.DefaultViews()...)
	views = append(views, cp.DefaultViews()...)

	// Create views to see our measurements. This can return an error if
	// a previously-registered view has the same name with a different value.
	// View name defaults to the measure name if unspecified.
	if err := view.Register(views...); err != nil {
		panic(err)
	}
}

// StatsReporter defines the interface for sending metrics
type StatsReporter interface {
	// ReportQueueDepth reports the queue depth metric
	ReportQueueDepth(v int64) error

	// ReportReconcile reports the count and latency metrics for a reconcile operation
	ReportReconcile(duration time.Duration, key, success string) error
}

// Reporter holds cached metric objects to report metrics
type reporter struct {
	reconciler string
	globalCtx  context.Context
}

// NewStatsReporter creates a reporter that collects and reports metrics
func NewStatsReporter(reconciler string) (StatsReporter, error) {
	// Reconciler tag is static. Create a context containing that and cache it.
	ctx, err := tag.New(
		context.Background(),
		tag.Insert(reconcilerTagKey, reconciler))
	if err != nil {
		return nil, err
	}

	return &reporter{reconciler: reconciler, globalCtx: ctx}, nil
}

// MustNewStatsReporter creates a new instance of StatsReporter.
// Logs fatally if creation fails.
func MustNewStatsReporter(reconciler string, logger *zap.SugaredLogger) StatsReporter {
	stats, err := NewStatsReporter(reconciler)
	if err != nil {
		logger.Fatalw("Failed to initialize the stats reporter", zap.Error(err))
	}
	return stats
}

// ReportQueueDepth reports the queue depth metric
func (r *reporter) ReportQueueDepth(v int64) error {
	if r.globalCtx == nil {
		return errors.New("reporter is not initialized correctly")
	}
	metrics.Record(r.globalCtx, workQueueDepthStat.M(v))
	return nil
}

// ReportReconcile reports the count and latency metrics for a reconcile operation
func (r *reporter) ReportReconcile(duration time.Duration, key, success string) error {
	ctx, err := tag.New(
		context.Background(),
		tag.Insert(reconcilerTagKey, r.reconciler),
		tag.Insert(keyTagKey, key),
		tag.Insert(successTagKey, success))
	if err != nil {
		return err
	}

	metrics.Record(ctx, reconcileCountStat.M(1))
	metrics.Record(ctx, reconcileLatencyStat.M(int64(duration/time.Millisecond)))
	return nil
}
