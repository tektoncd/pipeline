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

package k8s

import (
	"k8s.io/client-go/util/workqueue"
)

type (
	noopProvider struct{}
	noopMetric   struct{}
)

func NewNoopWorkqueueMetricsProvider() workqueue.MetricsProvider {
	return &noopProvider{}
}

func (noopMetric) Inc()            {}
func (noopMetric) Dec()            {}
func (noopMetric) Set(float64)     {}
func (noopMetric) Observe(float64) {}

var (
	_ workqueue.GaugeMetric         = (*noopMetric)(nil)
	_ workqueue.CounterMetric       = (*noopMetric)(nil)
	_ workqueue.HistogramMetric     = (*noopMetric)(nil)
	_ workqueue.SettableGaugeMetric = (*noopMetric)(nil)
)

func (noopProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return noopMetric{}
}

func (noopProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}

func (noopProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return noopMetric{}
}

func (noopProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return noopMetric{}
}

func (noopProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

func (noopProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

func (noopProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}
