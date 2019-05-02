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
	"path"

	"github.com/knative/pkg/metrics/metricskey"
	"go.opencensus.io/stats"
)

// Record decides whether to record one measurement via OpenCensus based on the
// following conditions:
//   1) No package level metrics config. In this case it just proxies to OpenCensus
//      based on the assumption that users expect the metrics to be recorded when
//      they call this function. Users must ensure metrics config are set before
//      using this function to get expected behavior.
//   2) The backend is not Stackdriver.
//   3) The backend is Stackdriver and it is allowed to use custom metrics.
//   4) The backend is Stackdriver and the metric is "knative_revison" built-in metric.
func Record(ctx context.Context, ms stats.Measurement) {
	mc := getCurMetricsConfig()

	// Condition 1)
	if mc == nil {
		stats.Record(ctx, ms)
		return
	}

	// Condition 2) and 3)
	if !mc.isStackdriverBackend || mc.allowStackdriverCustomMetrics {
		stats.Record(ctx, ms)
		return
	}

	// Condition 4)
	metricType := path.Join(mc.stackdriverMetricTypePrefix, ms.Measure().Name())
	if metricskey.KnativeRevisionMetrics.Has(metricType) {
		stats.Record(ctx, ms)
	}
}
