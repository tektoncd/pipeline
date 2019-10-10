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

	"go.opencensus.io/stats"
	"knative.dev/pkg/metrics/metricskey"
)

// TODO should be properly refactored and pieces should move to eventing and serving, as appropriate.
// 	See https://github.com/knative/pkg/issues/608

// Record decides whether to record one measurement via OpenCensus based on the
// following conditions:
//   1) No package level metrics config. In this case it just proxies to OpenCensus
//      based on the assumption that users expect the metrics to be recorded when
//      they call this function. Users must ensure metrics config are set before
//      using this function to get expected behavior.
//   2) The backend is not Stackdriver.
//   3) The backend is Stackdriver and it is allowed to use custom metrics.
//   4) The backend is Stackdriver and the metric is one of the built-in metrics: "knative_revision", "knative_broker",
//      "knative_trigger", "knative_source".
func Record(ctx context.Context, ms stats.Measurement, ros ...stats.Options) {
	mc := getCurMetricsConfig()

	ros = append(ros, stats.WithMeasurements(ms))

	// Condition 1)
	if mc == nil {
		stats.RecordWithOptions(ctx, ros...)
		return
	}

	// Condition 2) and 3)
	if !mc.isStackdriverBackend || mc.allowStackdriverCustomMetrics {
		stats.RecordWithOptions(ctx, ros...)
		return
	}

	// Condition 4)
	metricType := path.Join(mc.stackdriverMetricTypePrefix, ms.Measure().Name())
	isServingBuiltIn := metricskey.KnativeRevisionMetrics.Has(metricType)
	isEventingBuiltIn := metricskey.KnativeTriggerMetrics.Has(metricType) ||
		metricskey.KnativeBrokerMetrics.Has(metricType) ||
		metricskey.KnativeSourceMetrics.Has(metricType)

	if isServingBuiltIn || isEventingBuiltIn {
		stats.RecordWithOptions(ctx, ros...)
	}
}

// Buckets125 generates an array of buckets with approximate powers-of-two
// buckets that also aligns with powers of 10 on every 3rd step. This can
// be used to create a view.Distribution.
func Buckets125(low, high float64) []float64 {
	buckets := []float64{low}
	for last := low; last < high; last = last * 10 {
		buckets = append(buckets, 2*last, 5*last, 10*last)
	}
	return buckets
}

// BucketsNBy10 generates an array of N buckets starting from low and
// multiplying by 10 n times.
func BucketsNBy10(low float64, n int) []float64 {
	buckets := []float64{low}
	for last, i := low, len(buckets); i < n; last, i = 10*last, i+1 {
		buckets = append(buckets, 10*last)
	}
	return buckets
}
