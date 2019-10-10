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
	"path"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"knative.dev/pkg/metrics/metricskey"
)

const (
	customMetricTypePrefix = "custom.googleapis.com"
	// defaultCustomMetricSubDomain is the default subdomain to use for unsupported metrics by monitored resource types.
	// See: https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.metricDescriptors#MetricDescriptor
	defaultCustomMetricSubDomain = "knative.dev"
)

var (
	// gcpMetadataFunc is the function used to fetch GCP metadata.
	// In product usage, this is always set to function retrieveGCPMetadata.
	// In unit tests this is set to a fake one to avoid calling GCP metadata
	// service.
	gcpMetadataFunc func() *gcpMetadata

	// newStackdriverExporterFunc is the function used to create new stackdriver
	// exporter.
	// In product usage, this is always set to function newOpencensusSDExporter.
	// In unit tests this is set to a fake one to avoid calling actual Google API
	// service.
	newStackdriverExporterFunc func(stackdriver.Options) (view.Exporter, error)
)

func init() {
	// Set gcpMetadataFunc to call GCP metadata service.
	gcpMetadataFunc = retrieveGCPMetadata

	newStackdriverExporterFunc = newOpencensusSDExporter
}

func newOpencensusSDExporter(o stackdriver.Options) (view.Exporter, error) {
	return stackdriver.NewExporter(o)
}

// TODO should be properly refactored to be able to inject the getMonitoredResourceFunc function.
// 	See https://github.com/knative/pkg/issues/608
func newStackdriverExporter(config *metricsConfig, logger *zap.SugaredLogger) (view.Exporter, error) {
	gm := gcpMetadataFunc()
	mtf := getMetricTypeFunc(config.stackdriverMetricTypePrefix, config.stackdriverCustomMetricTypePrefix)
	e, err := newStackdriverExporterFunc(stackdriver.Options{
		ProjectID:               config.stackdriverProjectID,
		GetMetricDisplayName:    mtf, // Use metric type for display name for custom metrics. No impact on built-in metrics.
		GetMetricType:           mtf,
		GetMonitoredResource:    getMonitoredResourceFunc(config.stackdriverMetricTypePrefix, gm),
		DefaultMonitoringLabels: &stackdriver.Labels{},
	})
	if err != nil {
		logger.Errorw("Failed to create the Stackdriver exporter: ", zap.Error(err))
		return nil, err
	}
	logger.Infof("Created Opencensus Stackdriver exporter with config %v", config)
	return e, nil
}

func getMonitoredResourceFunc(metricTypePrefix string, gm *gcpMetadata) func(v *view.View, tags []tag.Tag) ([]tag.Tag, monitoredresource.Interface) {
	return func(view *view.View, tags []tag.Tag) ([]tag.Tag, monitoredresource.Interface) {
		metricType := path.Join(metricTypePrefix, view.Measure.Name())
		if metricskey.KnativeRevisionMetrics.Has(metricType) {
			return GetKnativeRevisionMonitoredResource(view, tags, gm)
		} else if metricskey.KnativeBrokerMetrics.Has(metricType) {
			return GetKnativeBrokerMonitoredResource(view, tags, gm)
		} else if metricskey.KnativeTriggerMetrics.Has(metricType) {
			return GetKnativeTriggerMonitoredResource(view, tags, gm)
		} else if metricskey.KnativeSourceMetrics.Has(metricType) {
			return GetKnativeSourceMonitoredResource(view, tags, gm)
		}
		// Unsupported metric by knative_revision, knative_broker, knative_trigger, and knative_source, use "global" resource type.
		return getGlobalMonitoredResource(view, tags)
	}
}

func getGlobalMonitoredResource(v *view.View, tags []tag.Tag) ([]tag.Tag, monitoredresource.Interface) {
	return tags, &Global{}
}

func getMetricTypeFunc(metricTypePrefix, customMetricTypePrefix string) func(view *view.View) string {
	return func(view *view.View) string {
		metricType := path.Join(metricTypePrefix, view.Measure.Name())
		inServing := metricskey.KnativeRevisionMetrics.Has(metricType)
		inEventing := metricskey.KnativeBrokerMetrics.Has(metricType) ||
			metricskey.KnativeTriggerMetrics.Has(metricType) ||
			metricskey.KnativeSourceMetrics.Has(metricType)
		if inServing || inEventing {
			return metricType
		}
		// Unsupported metric by knative_revision, use custom domain.
		return path.Join(customMetricTypePrefix, view.Measure.Name())
	}
}
