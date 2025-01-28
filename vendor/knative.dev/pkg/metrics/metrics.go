/*
Copyright 2019 The Knative Authors

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

package metrics

import (
	"context"
	"net/url"
	"sync/atomic"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/metrics"
	"k8s.io/client-go/util/workqueue"
)

var (
	// tagName is used to associate the provided name with each metric created
	// through the WorkqueueProvider's methods to implement workqueue.MetricsProvider.
	// For the kubernetes workqueue implementations this is the queue name provided
	// to the workqueue constructor.
	tagName = tag.MustNewKey("name")

	// tagVerb is used to associate the verb of the client action with latency metrics.
	tagVerb = tag.MustNewKey("verb")
	// tagCode is used to associate the status code the client gets back from an API call.
	tagCode = tag.MustNewKey("code")
	// tagMethod is used to associate the HTTP method the client used for the rest call.
	tagMethod = tag.MustNewKey("method")
	// tagHost is used to associate the host to which the HTTP request was made.
	tagHost = tag.MustNewKey("host")
	// tagPath is used to associate the path to which the HTTP request as made.
	tagPath = tag.MustNewKey("path")
)

type counterMetric struct {
	mutators []tag.Mutator
	measure  *stats.Int64Measure
}

var (
	_ cache.CounterMetric     = (*counterMetric)(nil)
	_ workqueue.CounterMetric = (*counterMetric)(nil)
)

// Inc implements CounterMetric
func (m counterMetric) Inc() {
	Record(context.Background(), m.measure.M(1), stats.WithTags(m.mutators...))
}

type gaugeMetric struct {
	mutators []tag.Mutator
	measure  *stats.Int64Measure
	total    atomic.Int64
}

var _ workqueue.GaugeMetric = (*gaugeMetric)(nil)

// Inc implements CounterMetric
func (m *gaugeMetric) Inc() {
	total := m.total.Add(1)
	Record(context.Background(), m.measure.M(total), stats.WithTags(m.mutators...))
}

// Dec implements GaugeMetric
func (m *gaugeMetric) Dec() {
	total := m.total.Add(-1)
	Record(context.Background(), m.measure.M(total), stats.WithTags(m.mutators...))
}

type floatMetric struct {
	mutators []tag.Mutator
	measure  *stats.Float64Measure
}

var (
	_ workqueue.SummaryMetric       = (*floatMetric)(nil)
	_ workqueue.SettableGaugeMetric = (*floatMetric)(nil)
	_ workqueue.HistogramMetric     = (*floatMetric)(nil)
	_ cache.GaugeMetric             = (*floatMetric)(nil)
)

// Observe implements SummaryMetric
func (m floatMetric) Observe(v float64) {
	Record(context.Background(), m.measure.M(v), stats.WithTags(m.mutators...))
}

// Set implements GaugeMetric
func (m floatMetric) Set(v float64) {
	m.Observe(v)
}

type latencyMetric struct {
	measure *stats.Float64Measure
}

var _ metrics.LatencyMetric = (*latencyMetric)(nil)

// Observe implements LatencyMetric
func (m latencyMetric) Observe(ctx context.Context, verb string, u url.URL, t time.Duration) {
	Record(ctx, m.measure.M(t.Seconds()), stats.WithTags(
		tag.Insert(tagVerb, verb),
		tag.Insert(tagHost, u.Host),
		tag.Insert(tagPath, u.Path),
	))
}

type resultMetric struct {
	measure *stats.Int64Measure
}

var _ metrics.ResultMetric = (*resultMetric)(nil)

// Increment implements ResultMetric
func (m resultMetric) Increment(ctx context.Context, code, method, host string) {
	Record(ctx, m.measure.M(1), stats.WithTags(
		tag.Insert(tagCode, code),
		tag.Insert(tagMethod, method),
		tag.Insert(tagHost, host),
	))
}

// measureView returns a view of the supplied metric.
func measureView(m stats.Measure, agg *view.Aggregation) *view.View {
	return &view.View{
		Name:        m.Name(),
		Description: m.Description(),
		Measure:     m,
		Aggregation: agg,
		TagKeys:     []tag.Key{tagName},
	}
}

// noopMetric implements all the cache and workqueue metric interfaces.
// Note: we cannot implement the metrics.FooMetric types due to
// overlapping method names.
type noopMetric struct{}

var (
	_ cache.CounterMetric           = (*noopMetric)(nil)
	_ cache.GaugeMetric             = (*noopMetric)(nil)
	_ workqueue.CounterMetric       = (*noopMetric)(nil)
	_ workqueue.GaugeMetric         = (*noopMetric)(nil)
	_ workqueue.HistogramMetric     = (*noopMetric)(nil)
	_ workqueue.SettableGaugeMetric = (*noopMetric)(nil)
)

func (noopMetric) Inc()            {}
func (noopMetric) Dec()            {}
func (noopMetric) Set(float64)     {}
func (noopMetric) Observe(float64) {}
