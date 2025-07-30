// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package runtime // import "go.opentelemetry.io/contrib/instrumentation/runtime"

import (
	"context"
	"errors"
	"math"
	"runtime/metrics"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var startTime time.Time

func init() {
	startTime = time.Now()
}

var histogramMetrics = []string{goSchedLatencies}

// Producer is a metric.Producer, which provides precomputed histogram metrics from the go runtime.
type Producer struct {
	lock      sync.Mutex
	collector *goCollector
}

var _ metric.Producer = (*Producer)(nil)

// NewProducer creates a Producer which provides precomputed histogram metrics from the go runtime.
func NewProducer(opts ...ProducerOption) *Producer {
	c := newProducerConfig(opts...)
	return &Producer{
		collector: newCollector(c.MinimumReadMemStatsInterval, histogramMetrics),
	}
}

// Produce returns precomputed histogram metrics from the go runtime, or an error if unsuccessful.
func (p *Producer) Produce(context.Context) ([]metricdata.ScopeMetrics, error) {
	p.lock.Lock()
	p.collector.refresh()
	schedHist := p.collector.getHistogram(goSchedLatencies)
	p.lock.Unlock()
	// Use the last collection time (which may or may not be now) for the timestamp.
	histDp := convertRuntimeHistogram(schedHist, p.collector.lastCollect)
	if len(histDp) == 0 {
		return nil, errors.New("unable to obtain go.schedule.duration metric from the runtime")
	}
	return []metricdata.ScopeMetrics{
		{
			Scope: instrumentation.Scope{
				Name:    ScopeName,
				Version: Version(),
			},
			Metrics: []metricdata.Metrics{
				{
					Name:        "go.schedule.duration",
					Description: "The time goroutines have spent in the scheduler in a runnable state before actually running.",
					Unit:        "s",
					Data: metricdata.Histogram[float64]{
						Temporality: metricdata.CumulativeTemporality,
						DataPoints:  histDp,
					},
				},
			},
		},
	}, nil
}

var emptySet = attribute.EmptySet()

func convertRuntimeHistogram(runtimeHist *metrics.Float64Histogram, ts time.Time) []metricdata.HistogramDataPoint[float64] {
	if runtimeHist == nil {
		return nil
	}
	bounds := runtimeHist.Buckets
	counts := runtimeHist.Counts
	if len(bounds) < 2 {
		// runtime histograms are guaranteed to have at least two bucket boundaries.
		return nil
	}
	// trim the first bucket since it is a lower bound. OTel histogram boundaries only have an upper bound.
	bounds = bounds[1:]
	if bounds[len(bounds)-1] == math.Inf(1) {
		// trim the last bucket if it is +Inf, since the +Inf boundary is implicit in OTel.
		bounds = bounds[:len(bounds)-1]
	} else {
		// if the last bucket is not +Inf, append an extra zero count since
		// the implicit +Inf bucket won't have any observations.
		counts = append(counts, 0)
	}
	count := uint64(0)
	sum := float64(0)
	for i, c := range counts {
		count += c
		// This computed sum is an underestimate, since it assumes each
		// observation happens at the bucket's lower bound.
		if i > 0 && count != 0 {
			sum += bounds[i-1] * float64(count)
		}
	}

	return []metricdata.HistogramDataPoint[float64]{
		{
			StartTime:    startTime,
			Count:        count,
			Sum:          sum,
			Time:         ts,
			Bounds:       bounds,
			BucketCounts: counts,
			Attributes:   *emptySet,
		},
	}
}
