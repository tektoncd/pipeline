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
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"k8s.io/client-go/util/workqueue"
)

type gauge struct {
	m     metric.Int64UpDownCounter
	attrs attribute.Set
}

func (g *gauge) Inc() {
	g.m.Add(context.Background(), 1, metric.WithAttributeSet(g.attrs))
}

func (g *gauge) Dec() {
	g.m.Add(context.Background(), -1, metric.WithAttributeSet(g.attrs))
}

type counter struct {
	m     metric.Int64Counter
	attrs attribute.Set
}

func (c *counter) Inc() {
	c.m.Add(context.Background(), 1, metric.WithAttributeSet(c.attrs))
}

type histogram struct {
	m     metric.Float64Histogram
	attrs attribute.Set
}

func (h *histogram) Observe(val float64) {
	h.m.Record(context.Background(), val, metric.WithAttributeSet(h.attrs))
}

type settableGauge struct {
	m     metric.Float64Gauge
	attrs attribute.Set
}

func (s *settableGauge) Set(val float64) {
	s.m.Record(context.Background(), val, metric.WithAttributeSet(s.attrs))
}

var (
	_ workqueue.GaugeMetric         = (*gauge)(nil)
	_ workqueue.CounterMetric       = (*counter)(nil)
	_ workqueue.HistogramMetric     = (*histogram)(nil)
	_ workqueue.SettableGaugeMetric = (*settableGauge)(nil)
)
