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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/stats"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/metrics/metricskey"
)

// metricsBackend specifies the backend to use for metrics
type metricsBackend string

const (
	// BackendDestinationKey points to the config map entry key for metrics backend destination.
	BackendDestinationKey = "metrics.backend-destination"
	// DomainEnv points to the metrics domain env var.
	DomainEnv = "METRICS_DOMAIN"

	// The following keys are used to configure metrics reporting.
	// See https://github.com/knative/serving/blob/main/config/core/configmaps/observability.yaml
	// for details.
	collectorAddressKey = "metrics.opencensus-address"
	collectorSecureKey  = "metrics.opencensus-require-tls"
	reportingPeriodKey  = "metrics.reporting-period-seconds"

	defaultBackendEnvName            = "DEFAULT_METRICS_BACKEND"
	defaultPrometheusPort            = 9090
	defaultPrometheusReportingPeriod = 5
	defaultOpenCensusReportingPeriod = 60
	maxPrometheusPort                = 65535
	minPrometheusPort                = 1024
	defaultPrometheusHost            = "0.0.0.0"
	prometheusPortEnvName            = "METRICS_PROMETHEUS_PORT"
	prometheusHostEnvName            = "METRICS_PROMETHEUS_HOST"
)

var (
	// TestOverrideBundleCount is a variable for testing to reduce the size (number of metrics) buffered before
	// OpenCensus will send a bundled metric report. Only applies if non-zero.
	TestOverrideBundleCount = 0
)

// Metrics backend "enum".
const (
	// prometheus is used for Prometheus backend
	prometheus metricsBackend = "prometheus"
	// openCensus is used to export to the OpenCensus Agent / Collector,
	// which can send to many other services.
	openCensus metricsBackend = "opencensus"
	// none is used to export, well, nothing.
	none metricsBackend = "none"
)

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

	// secret contains credentials for an exporter to use for authentication.
	secret *corev1.Secret

	// ---- OpenCensus specific below ----
	// collectorAddress is the address of the collector, if not `localhost:55678`
	collectorAddress string

	// Require mutual TLS. Defaults to "false" because mutual TLS is hard to set up.
	requireSecure bool

	// ---- Prometheus specific below ----
	// prometheusPort is the port where metrics are exposed in Prometheus
	// format. It defaults to 9090.
	prometheusPort int

	// prometheusHost is the host where the metrics are exposed in Prometheus
	// format. It defaults to "0.0.0.0"
	prometheusHost string
}

// record applies the `ros` Options to each measurement in `mss` and then records the resulting
// measurements in the metricsConfig's designated backend.
func (mc *metricsConfig) record(ctx context.Context, mss []stats.Measurement, ros ...stats.Options) error {
	if mc == nil || mc.backendDestination == none {
		// Don't record data points if the metric config is not initialized yet or if
		// the defined backend is "none" explicitly.
		return nil
	}

	opt, err := optionForResource(metricskey.GetResource(ctx))
	if err != nil {
		return err
	}
	ros = append(ros, opt)

	return stats.RecordWithOptions(ctx, append(ros, stats.WithMeasurements(mss...))...)
}

func createMetricsConfig(_ context.Context, ops ExporterOptions) (*metricsConfig, error) {
	var mc metricsConfig
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
		backend = string(prometheus)
	}
	// Override backend if it is set in the config map.
	if backendFromConfig, ok := m[BackendDestinationKey]; ok {
		backend = backendFromConfig
	}

	switch lb := metricsBackend(strings.ToLower(backend)); lb {
	case prometheus, openCensus, none:
		mc.backendDestination = lb
	default:
		return nil, fmt.Errorf("unsupported metrics backend value %q", backend)
	}

	switch mc.backendDestination {
	case openCensus:
		if ops.Domain == "" {
			return nil, errors.New("metrics domain cannot be empty")
		}
		mc.collectorAddress = ops.ConfigMap[collectorAddressKey]
		if isSecure := ops.ConfigMap[collectorSecureKey]; isSecure != "" {
			var err error
			if mc.requireSecure, err = strconv.ParseBool(isSecure); err != nil {
				return nil, fmt.Errorf("invalid %s value %q", collectorSecureKey, isSecure)
			}

			if mc.requireSecure {
				mc.secret, err = getOpenCensusSecret(ops.Component, ops.Secrets)
				if err != nil {
					return nil, err
				}
			}
		}
	case prometheus:
		pp := ops.PrometheusPort
		if pp == 0 {
			var err error
			pp, err = prometheusPort()
			if err != nil {
				return nil, fmt.Errorf("failed to determine Prometheus port: %w", err)
			}
		}

		if pp < minPrometheusPort || pp > maxPrometheusPort {
			return nil, fmt.Errorf("invalid port %d, should be between %d and %d",
				pp, minPrometheusPort, maxPrometheusPort)
		}

		mc.prometheusPort = pp
		mc.prometheusHost = prometheusHost()
	}

	// If reporting period is specified, use the value from the configuration.
	// If not, set a default value based on the selected backend.
	// Each exporter makes different promises about what the lowest supported
	// reporting period is. For OpenCensus, this value is 1 minute.
	// For Prometheus, we will use a lower value since the exporter doesn't
	// push anything but just responds to pull requests, and shorter durations
	// do not really hurt the performance and we rely on the scraping configuration.
	if repStr := m[reportingPeriodKey]; repStr != "" {
		repInt, err := strconv.Atoi(repStr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s value %q", reportingPeriodKey, repStr)
		}
		mc.reportingPeriod = time.Duration(repInt) * time.Second
	} else {
		switch mc.backendDestination {
		case openCensus:
			mc.reportingPeriod = defaultOpenCensusReportingPeriod * time.Second
		case prometheus:
			mc.reportingPeriod = defaultPrometheusReportingPeriod * time.Second
		}
	}
	return &mc, nil
}

// Domain holds the metrics domain to use for surfacing metrics.
func Domain() string {
	if domain := os.Getenv(DomainEnv); domain != "" {
		return domain
	}
	return ""
}

// prometheusPort returns the TCP port number configured via the environment
// for the Prometheus metrics exporter if it's set, a default value otherwise.
// No validation is performed on the port value, other than ensuring that value
// is a valid port number (16-bit unsigned integer).
func prometheusPort() (int, error) {
	ppStr := os.Getenv(prometheusPortEnvName)
	if ppStr == "" {
		return defaultPrometheusPort, nil
	}

	pp, err := strconv.ParseUint(ppStr, 10, 16)
	if err != nil {
		return -1, fmt.Errorf("the environment variable %q could not be parsed as a port number: %w",
			prometheusPortEnvName, err)
	}

	return int(pp), nil
}

// prometheusHost returns the host configured via the environment
// for the Prometheus metrics exporter if it's set, a default value otherwise.
// No validation is done here.
func prometheusHost() string {
	phStr := os.Getenv(prometheusHostEnvName)
	if phStr == "" {
		return defaultPrometheusHost
	}
	return phStr
}

// JSONToOptions converts a json string to ExporterOptions.
func JSONToOptions(jsonOpts string) (*ExporterOptions, error) {
	var opts ExporterOptions
	if jsonOpts == "" {
		return nil, errors.New("json options string is empty")
	}

	if err := json.Unmarshal([]byte(jsonOpts), &opts); err != nil {
		return nil, err
	}

	return &opts, nil
}

// OptionsToJSON converts an ExporterOptions object to a JSON string.
func OptionsToJSON(opts *ExporterOptions) (string, error) {
	if opts == nil {
		return "", nil
	}

	jsonOpts, err := json.Marshal(opts)
	return string(jsonOpts), err
}
