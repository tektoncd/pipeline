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
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"k8s.io/client-go/tools/metrics"
)

// ClientProvider implements the pattern of Kubernetes MetricProvider that may
// be used to produce suitable metrics for use with metrics.Register()
type ClientProvider struct {
	Latency *stats.Float64Measure
	Result  *stats.Int64Measure
}

// NewLatencyMetric implements MetricsProvider
func (cp *ClientProvider) NewLatencyMetric() metrics.LatencyMetric {
	return latencyMetric{
		measure: cp.Latency,
	}
}

// LatencyView returns a view of the Latency metric.
func (cp *ClientProvider) LatencyView() *view.View {
	return measureView(cp.Latency, view.Distribution(BucketsNBy10(0.00001, 8)...))
}

// NewResultMetric implements MetricsProvider
func (cp *ClientProvider) NewResultMetric() metrics.ResultMetric {
	return resultMetric{
		measure: cp.Result,
	}
}

// ResultView returns a view of the Result metric.
func (cp *ClientProvider) ResultView() *view.View {
	return measureView(cp.Result, view.Count())
}

// DefaultViews returns a list of views suitable for passing to view.Register
func (cp *ClientProvider) DefaultViews() []*view.View {
	return []*view.View{
		cp.LatencyView(),
		cp.ResultView(),
	}
}
