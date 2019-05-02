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

package metrics

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// metricsBackend specifies the backend to use for metrics
type metricsBackend string

const (
	// The following keys are used to configure metrics reporting.
	// See https://github.com/knative/serving/blob/master/config/config-observability.yaml
	// for details.
	AllowStackdriverCustomMetricsKey = "metrics.allow-stackdriver-custom-metrics"
	BackendDestinationKey            = "metrics.backend-destination"
	ReportingPeriodKey               = "metrics.reporting-period-seconds"
	StackdriverProjectIDKey          = "metrics.stackdriver-project-id"

	// Stackdriver is used for Stackdriver backend
	Stackdriver metricsBackend = "stackdriver"
	// Prometheus is used for Prometheus backend
	Prometheus metricsBackend = "prometheus"

	defaultBackendEnvName = "DEFAULT_METRICS_BACKEND"

	defaultPrometheusPort = 9090
	maxPrometheusPort     = 65535
	minPrometheusPort     = 1024
)

// ExporterOptions contains options for configuring the exporter.
type ExporterOptions struct {
	// Domain is the metrics domain. e.g. "knative.dev". Must be present.
	Domain string

	// Component is the name of the component that emits the metrics. e.g.
	// "activator", "queue_proxy". Should only contains alphabets and underscore.
	// Must be present.
	Component string

	// PrometheusPort is the port to expose metrics if metrics backend is Prometheus.
	// It should be between maxPrometheusPort and maxPrometheusPort. 0 value means
	// using the default 9090 value. If is ignored if metrics backend is not
	// Prometheus.
	PrometheusPort int

	// ConfigMap is the data from config map config-observability. Must be present.
	// See https://github.com/knative/serving/blob/master/config/config-observability.yaml
	// for details.
	ConfigMap map[string]string
}

type metricsConfig struct {
	// The metrics domain. e.g. "serving.knative.dev" or "build.knative.dev".
	domain string
	// The component that emits the metrics. e.g. "activator", "autoscaler".
	component string
	// The metrics backend destination.
	backendDestination metricsBackend
	// reportingPeriod specifies the interval between reporting aggregated views.
	// If duration is less than or equal to zero, it enables the default behavior.
	reportingPeriod time.Duration

	// ---- Prometheus specific below ----
	// prometheusPort is the port where metrics are exposed in Prometheus
	// format. It defaults to 9090.
	prometheusPort int

	// ---- Stackdriver specific below ----
	// stackdriverProjectID is the stackdriver project ID where the stats data are
	// uploaded to. This is not the GCP project ID.
	stackdriverProjectID string
	// allowStackdriverCustomMetrics indicates whether it is allowed to send metrics to
	// Stackdriver using "global" resource type and custom metric type if the
	// metrics are not supported by "knative_revision" resource type. Setting this
	// flag to "true" could cause extra Stackdriver charge.
	// If backendDestination is not Stackdriver, this is ignored.
	allowStackdriverCustomMetrics bool
	// True if backendDestination equals to "stackdriver". Store this in a variable
	// to reduce string comparison operations.
	isStackdriverBackend bool
	// stackdriverMetricTypePrefix is the metric domain joins component, e.g.
	// "knative.dev/serving/activator". Store this in a variable to reduce string
	// join operations.
	stackdriverMetricTypePrefix string
	// stackdriverCustomMetricTypePrefix is "custom.googleapis.com/knative.dev" joins
	// component, e.g. "custom.googleapis.com/knative.dev/serving/activator".
	// Store this in a variable to reduce string join operations.
	stackdriverCustomMetricTypePrefix string
}

func getMetricsConfig(ops ExporterOptions, logger *zap.SugaredLogger) (*metricsConfig, error) {
	var mc metricsConfig

	if ops.Domain == "" {
		return nil, errors.New("metrics domain cannot be empty")
	}
	mc.domain = ops.Domain

	if ops.Component == "" {
		return nil, errors.New("metrics component name cannot be empty")
	}
	mc.component = ops.Component

	if ops.ConfigMap == nil {
		return nil, errors.New("metrics config map cannot be empty")
	}
	m := ops.ConfigMap
	// Read backend setting from environment variable first
	backend := os.Getenv(defaultBackendEnvName)
	if backend == "" {
		// Use Prometheus if DEFAULT_METRICS_BACKEND does not exist or is empty
		backend = string(Prometheus)
	}
	// Override backend if it is setting in config map.
	if backendFromConfig, ok := m[BackendDestinationKey]; ok {
		backend = backendFromConfig
	}
	lb := metricsBackend(strings.ToLower(backend))
	switch lb {
	case Stackdriver, Prometheus:
		mc.backendDestination = lb
	default:
		return nil, fmt.Errorf("unsupported metrics backend value %q", backend)
	}

	if mc.backendDestination == Prometheus {
		pp := ops.PrometheusPort
		if pp == 0 {
			pp = defaultPrometheusPort
		}
		if pp < minPrometheusPort || pp > maxPrometheusPort {
			return nil, fmt.Errorf("invalid port %v, should between %v and %v", pp, minPrometheusPort, maxPrometheusPort)
		}
		mc.prometheusPort = pp
	}

	// If stackdriverProjectIDKey is not provided for stackdriver backend destination, OpenCensus will try to
	// use the application default credentials. If that is not available, Opencensus would fail to create the
	// metrics exporter.
	if mc.backendDestination == Stackdriver {
		mc.stackdriverProjectID = m[StackdriverProjectIDKey]
		mc.isStackdriverBackend = true
		mc.stackdriverMetricTypePrefix = path.Join(mc.domain, mc.component)
		mc.stackdriverCustomMetricTypePrefix = path.Join(customMetricTypePrefix, mc.component)
		if ascmStr, ok := m[AllowStackdriverCustomMetricsKey]; ok && ascmStr != "" {
			ascmBool, err := strconv.ParseBool(ascmStr)
			if err != nil {
				return nil, fmt.Errorf("invalid %s value %q", AllowStackdriverCustomMetricsKey, ascmStr)
			}
			mc.allowStackdriverCustomMetrics = ascmBool
		}
	}

	// If reporting period is specified, use the value from the configuration.
	// If not, set a default value based on the selected backend.
	// Each exporter makes different promises about what the lowest supported
	// reporting period is. For Stackdriver, this value is 1 minute.
	// For Prometheus, we will use a lower value since the exporter doesn't
	// push anything but just responds to pull requests, and shorter durations
	// do not really hurt the performance and we rely on the scraping configuration.
	if repStr, ok := m[ReportingPeriodKey]; ok && repStr != "" {
		repInt, err := strconv.Atoi(repStr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s value %q", ReportingPeriodKey, repStr)
		}
		mc.reportingPeriod = time.Duration(repInt) * time.Second
	} else if mc.backendDestination == Stackdriver {
		mc.reportingPeriod = 60 * time.Second
	} else if mc.backendDestination == Prometheus {
		mc.reportingPeriod = 5 * time.Second
	}

	return &mc, nil
}

// UpdateExporterFromConfigMap returns a helper func that can be used to update the exporter
// when a config map is updated.
// DEPRECATED. Use UpdateExporter instead.
func UpdateExporterFromConfigMap(domain string, component string, logger *zap.SugaredLogger) func(configMap *corev1.ConfigMap) {
	return func(configMap *corev1.ConfigMap) {
		UpdateExporter(ExporterOptions{
			Domain:    domain,
			Component: component,
			ConfigMap: configMap.Data,
		}, logger)
	}
}

// UpdateExporter updates the exporter based on the given ExporterOptions.
func UpdateExporter(ops ExporterOptions, logger *zap.SugaredLogger) error {
	newConfig, err := getMetricsConfig(ops, logger)
	if err != nil {
		if ce := getCurMetricsExporter(); ce == nil {
			// Fail the process if there doesn't exist an exporter.
			logger.Errorw("Failed to get a valid metrics config", zap.Error(err))
		} else {
			logger.Errorw("Failed to get a valid metrics config; Skip updating the metrics exporter", zap.Error(err))
		}
		return err
	}

	if isNewExporterRequired(newConfig) {
		e, err := newMetricsExporter(newConfig, logger)
		if err != nil {
			logger.Errorf("Failed to update a new metrics exporter based on metric config %v. error: %v", newConfig, err)
			return err
		}
		existingConfig := getCurMetricsConfig()
		setCurMetricsExporter(e)
		logger.Infof("Successfully updated the metrics exporter; old config: %v; new config %v", existingConfig, newConfig)
	}

	setCurMetricsConfig(newConfig)
	return nil
}

// isNewExporterRequired compares the non-nil newConfig against curMetricsConfig. When backend changes,
// or stackdriver project ID changes for stackdriver backend, we need to update the metrics exporter.
func isNewExporterRequired(newConfig *metricsConfig) bool {
	cc := getCurMetricsConfig()
	if cc == nil || newConfig.backendDestination != cc.backendDestination {
		return true
	} else if newConfig.backendDestination == Stackdriver && newConfig.stackdriverProjectID != cc.stackdriverProjectID {
		return true
	}

	return false
}
