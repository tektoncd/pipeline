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
	"path"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/stats"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/metrics/metricskey"
)

// metricsBackend specifies the backend to use for metrics
type metricsBackend string

const (
	// The following keys are used to configure metrics reporting.
	// See https://github.com/knative/serving/blob/master/config/config-observability.yaml
	// for details.
	AllowStackdriverCustomMetricsKey    = "metrics.allow-stackdriver-custom-metrics"
	BackendDestinationKey               = "metrics.backend-destination"
	ReportingPeriodKey                  = "metrics.reporting-period-seconds"
	StackdriverCustomMetricSubDomainKey = "metrics.stackdriver-custom-metrics-subdomain"
	// Stackdriver client configuration keys
	StackdriverProjectIDKey   = "metrics.stackdriver-project-id"
	StackdriverGCPLocationKey = "metrics.stackdriver-gcp-location"
	StackdriverClusterNameKey = "metrics.stackdriver-cluster-name"
	StackdriverUseSecretKey   = "metrics.stackdriver-use-secret"

	DomainEnv = "METRICS_DOMAIN"

	// Stackdriver is used for Stackdriver backend
	Stackdriver metricsBackend = "stackdriver"
	// Prometheus is used for Prometheus backend
	Prometheus metricsBackend = "prometheus"
	// OpenCensus is used to export to the OpenCensus Agent / Collector,
	// which can send to many other services.
	OpenCensus metricsBackend = "opencensus"
	// None is used to export, well, nothing.
	None metricsBackend = "none"

	defaultBackendEnvName = "DEFAULT_METRICS_BACKEND"

	CollectorAddressKey = "metrics.opencensus-address"
	CollectorSecureKey  = "metrics.opencensus-require-tls"

	defaultPrometheusPort = 9090
	maxPrometheusPort     = 65535
	minPrometheusPort     = 1024
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

	// recorder provides a hook for performing custom transformations before
	// writing the metrics to the stats.RecordWithOptions interface.
	recorder func(context.Context, []stats.Measurement, ...stats.Options) error

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

	// ---- Stackdriver specific below ----
	// True if backendDestination equals to "stackdriver". Store this in a variable
	// to reduce string comparison operations.
	isStackdriverBackend bool
	// stackdriverMetricTypePrefix is the metric domain joins component, e.g.
	// "knative.dev/serving/activator". Store this in a variable to reduce string
	// join operations.
	stackdriverMetricTypePrefix string
	// stackdriverCustomMetricTypePrefix is "custom.googleapis.com" joined with the subdomain and component.
	// E.g., "custom.googleapis.com/<subdomain>/<component>".
	// Store this in a variable to reduce string join operations.
	stackdriverCustomMetricTypePrefix string
	// stackdriverClientConfig is the metadata to configure the metrics exporter's Stackdriver client.
	stackdriverClientConfig StackdriverClientConfig
}

// StackdriverClientConfig encapsulates the metadata required to configure a Stackdriver client.
type StackdriverClientConfig struct {
	// ProjectID is the stackdriver project ID to which data is uploaded.
	// This is not necessarily the GCP project ID where the Kubernetes cluster is hosted.
	// Required when the Kubernetes cluster is not hosted on GCE.
	ProjectID string
	// GCPLocation is the GCP region or zone to which data is uploaded.
	// This is not necessarily the GCP location where the Kubernetes cluster is hosted.
	// Required when the Kubernetes cluster is not hosted on GCE.
	GCPLocation string
	// ClusterName is the cluster name with which the data will be associated in Stackdriver.
	// Required when the Kubernetes cluster is not hosted on GCE.
	ClusterName string
	// UseSecret is whether the credentials stored in a Kubernetes Secret should be used to
	// authenticate with Stackdriver. The Secret name and namespace can be specified by calling
	// metrics.SetStackdriverSecretLocation.
	// If UseSecret is false, Google Application Default Credentials
	// will be used (https://cloud.google.com/docs/authentication/production).
	UseSecret bool
}

// NewStackdriverClientConfigFromMap creates a stackdriverClientConfig from the given map
func NewStackdriverClientConfigFromMap(config map[string]string) *StackdriverClientConfig {
	return &StackdriverClientConfig{
		ProjectID:   config[StackdriverProjectIDKey],
		GCPLocation: config[StackdriverGCPLocationKey],
		ClusterName: config[StackdriverClusterNameKey],
		UseSecret:   strings.EqualFold(config[StackdriverUseSecretKey], "true"),
	}
}

// record applies the `ros` Options to each measurement in `mss` and then records the resulting
// measurements in the metricsConfig's designated backend.
func (mc *metricsConfig) record(ctx context.Context, mss []stats.Measurement, ros ...stats.Options) error {
	if mc == nil {
		// Don't record data points if the metric config is not initialized yet.
		// At this point, it's unclear whether should record or not.
		return nil
	}
	if mc.recorder == nil {
		return stats.RecordWithOptions(ctx, append(ros, stats.WithMeasurements(mss...))...)
	}
	return mc.recorder(ctx, mss, ros...)
}

func createMetricsConfig(ops ExporterOptions, logger *zap.SugaredLogger) (*metricsConfig, error) {
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
	// Override backend if it is set in the config map.
	if backendFromConfig, ok := m[BackendDestinationKey]; ok {
		backend = backendFromConfig
	}
	lb := metricsBackend(strings.ToLower(backend))
	switch lb {
	case Stackdriver, Prometheus, OpenCensus:
		mc.backendDestination = lb
	default:
		return nil, fmt.Errorf("unsupported metrics backend value %q", backend)
	}

	if mc.backendDestination == OpenCensus {
		mc.collectorAddress = ops.ConfigMap[CollectorAddressKey]
		if isSecure := ops.ConfigMap[CollectorSecureKey]; isSecure != "" {
			var err error
			if mc.requireSecure, err = strconv.ParseBool(isSecure); err != nil {
				return nil, fmt.Errorf("invalid %s value %q", CollectorSecureKey, isSecure)
			}

			if mc.requireSecure {
				mc.secret, err = getOpenCensusSecret(ops.Component, ops.Secrets)
				if err != nil {
					return nil, err
				}
			}
		}
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

	// If stackdriverClientConfig is not provided for stackdriver backend destination, OpenCensus will try to
	// use the application default credentials. If that is not available, Opencensus would fail to create the
	// metrics exporter.
	if mc.backendDestination == Stackdriver {
		scc := NewStackdriverClientConfigFromMap(m)
		mc.stackdriverClientConfig = *scc
		mc.isStackdriverBackend = true
		var allowCustomMetrics bool
		var err error
		mc.stackdriverMetricTypePrefix = path.Join(mc.domain, mc.component)

		customMetricsSubDomain := m[StackdriverCustomMetricSubDomainKey]
		if customMetricsSubDomain == "" {
			customMetricsSubDomain = defaultCustomMetricSubDomain
		}
		mc.stackdriverCustomMetricTypePrefix = path.Join(customMetricTypePrefix, customMetricsSubDomain, mc.component)
		if ascmStr := m[AllowStackdriverCustomMetricsKey]; ascmStr != "" {
			allowCustomMetrics, err = strconv.ParseBool(ascmStr)
			if err != nil {
				return nil, fmt.Errorf("invalid %s value %q", AllowStackdriverCustomMetricsKey, ascmStr)
			}
		}

		if !allowCustomMetrics {
			servingOrEventing := metricskey.KnativeRevisionMetrics.Union(
				metricskey.KnativeTriggerMetrics).Union(metricskey.KnativeBrokerMetrics)
			mc.recorder = func(ctx context.Context, mss []stats.Measurement, ros ...stats.Options) error {
				// Perform array filtering in place using two indices: w(rite)Index and r(ead)Index.
				wIdx := 0
				for rIdx := 0; rIdx < len(mss); rIdx++ {
					metricType := path.Join(mc.stackdriverMetricTypePrefix, mss[rIdx].Measure().Name())
					if servingOrEventing.Has(metricType) {
						mss[wIdx] = mss[rIdx]
						wIdx++
					}
					// Otherwise, skip the measurement (because it won't be accepted).
				}
				// Found no matched metrics.
				if wIdx == 0 {
					return nil
				}
				// Trim the list to the number of written objects.
				mss = mss[:wIdx]
				return stats.RecordWithOptions(ctx, append(ros, stats.WithMeasurements(mss...))...)
			}
		}

		if scc.UseSecret {
			secret, err := getStackdriverSecret(ops.Secrets)
			if err != nil {
				return nil, err
			}

			mc.secret = secret
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

// Domain holds the metrics domain to use for surfacing metrics.
func Domain() string {
	if domain := os.Getenv(DomainEnv); domain != "" {
		return domain
	}

	panic(fmt.Sprintf(`The environment variable %q is not set

If this is a process running on Kubernetes, then it should be specifying
this via:

  env:
  - name: %s
    value: knative.dev/some-repository

If this is a Go unit test consuming metric.Domain() then it should add the
following import:

import (
	_ "knative.dev/pkg/metrics/testing"
)`, DomainEnv, DomainEnv))
}

// JsonToMetricsOptions converts a json string of a
// ExporterOptions. Returns a non-nil ExporterOptions always.
func JsonToMetricsOptions(jsonOpts string) (*ExporterOptions, error) {
	var opts ExporterOptions
	if jsonOpts == "" {
		return nil, errors.New("json options string is empty")
	}

	if err := json.Unmarshal([]byte(jsonOpts), &opts); err != nil {
		return nil, err
	}

	return &opts, nil
}

// MetricsOptionsToJson converts a ExporterOptions to a json string.
func MetricsOptionsToJson(opts *ExporterOptions) (string, error) {
	if opts == nil {
		return "", nil
	}

	jsonOpts, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}

	return string(jsonOpts), nil
}
