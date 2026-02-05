/*
Copyright 2025 The Knative Authors

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
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"knative.dev/pkg/observability/metrics/k8s"
)

// Copyright The Kubernetes Authors.
// Forked from: https://github.com/kubernetes/client-go/blob/release-1.33/util/workqueue/metrics.go

var (
	noopProvider = k8s.NewNoopWorkqueueMetricsProvider()

	// Unfortunately k8s package doesn't expose this variable so for now
	// we'll create our own and potentially upstream some changes in the future.
	globalMetricsProvider        workqueue.MetricsProvider = noopProvider
	setGlobalMetricsProviderOnce sync.Once
)

func SetMetricsProvider(metricsProvider workqueue.MetricsProvider) {
	setGlobalMetricsProviderOnce.Do(func() {
		globalMetricsProvider = metricsProvider
	})
}

// queueMetrics expects the caller to lock before setting any metrics.
type queueMetrics struct {
	clock clock.Clock

	// current depth of a workqueue
	depth workqueue.GaugeMetric
	// total number of adds handled by a workqueue
	adds workqueue.CounterMetric
	// how long an item stays in a workqueue
	latency workqueue.HistogramMetric
	// how long processing an item from a workqueue takes
	workDuration         workqueue.HistogramMetric
	addTimes             map[any]time.Time
	processingStartTimes map[any]time.Time

	mu sync.Mutex

	// how long have current threads been working?
	unfinishedWorkSeconds   workqueue.SettableGaugeMetric
	longestRunningProcessor workqueue.SettableGaugeMetric
}

func (m *queueMetrics) add(item any) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.addTimes[item]; !exists {
		m.adds.Inc()
		m.depth.Inc()
		m.addTimes[item] = m.clock.Now()
	}
}

func (m *queueMetrics) get(item any) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if startTime, exists := m.addTimes[item]; exists {
		m.depth.Dec()
		m.latency.Observe(m.sinceInSeconds(startTime))
		delete(m.addTimes, item)
	}

	if _, exists := m.processingStartTimes[item]; !exists {
		m.processingStartTimes[item] = m.clock.Now()
	}
}

func (m *queueMetrics) done(item any) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if startTime, exists := m.processingStartTimes[item]; exists {
		m.workDuration.Observe(m.sinceInSeconds(startTime))
		delete(m.processingStartTimes, item)
	}
}

func (m *queueMetrics) updateUnfinishedWork() {
	if m == nil {
		return
	}
	// Note that a summary metric would be better for this, but prometheus
	// doesn't seem to have non-hacky ways to reset the summary metrics.
	var total float64
	var oldest float64

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.processingStartTimes {
		age := m.sinceInSeconds(t)
		total += age
		if age > oldest {
			oldest = age
		}
	}
	m.unfinishedWorkSeconds.Set(total)
	m.longestRunningProcessor.Set(oldest)
}

func (m *queueMetrics) sinceInSeconds(start time.Time) float64 {
	return m.clock.Since(start).Seconds()
}
