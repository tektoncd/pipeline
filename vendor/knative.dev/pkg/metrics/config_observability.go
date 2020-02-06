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
	"os"
	"strings"
	texttemplate "text/template"

	corev1 "k8s.io/api/core/v1"
)

const (
	// The following is used to set the default log url template
	DefaultLogURLTemplate = "http://localhost:8001/api/v1/namespaces/knative-monitoring/services/kibana-logging/proxy/app/kibana#/discover?_a=(query:(match:(kubernetes.labels.knative-dev%2FrevisionUID:(query:'${REVISION_UID}',type:phrase))))"
	// The env var name for config-observability
	ConfigMapNameEnv = "CONFIG_OBSERVABILITY_NAME"
)

// ObservabilityConfig contains the configuration defined in the observability ConfigMap.
// +k8s:deepcopy-gen=true
type ObservabilityConfig struct {
	// EnableVarLogCollection specifies whether the logs under /var/log/ should be available
	// for collection on the host node by the fluentd daemon set.
	EnableVarLogCollection bool

	// LoggingURLTemplate is a string containing the logging url template where
	// the variable REVISION_UID will be replaced with the created revision's UID.
	LoggingURLTemplate string

	// RequestLogTemplate is the go template to use to shape the request logs.
	RequestLogTemplate string

	// EnableProbeRequestLog enables queue-proxy to write health check probe request logs.
	EnableProbeRequestLog bool

	// RequestMetricsBackend specifies the request metrics destination, e.g. Prometheus,
	// Stackdriver.
	RequestMetricsBackend string

	// EnableProfiling indicates whether it is allowed to retrieve runtime profiling data from
	// the pods via an HTTP server in the format expected by the pprof visualization tool.
	EnableProfiling bool
}

// NewObservabilityConfigFromConfigMap creates a ObservabilityConfig from the supplied ConfigMap
func NewObservabilityConfigFromConfigMap(configMap *corev1.ConfigMap) (*ObservabilityConfig, error) {
	oc := &ObservabilityConfig{}
	if evlc, ok := configMap.Data["logging.enable-var-log-collection"]; ok {
		oc.EnableVarLogCollection = strings.EqualFold(evlc, "true")
	}

	if rut, ok := configMap.Data["logging.revision-url-template"]; ok {
		oc.LoggingURLTemplate = rut
	} else {
		oc.LoggingURLTemplate = DefaultLogURLTemplate
	}

	if rlt, ok := configMap.Data["logging.request-log-template"]; ok {
		// Verify that we get valid templates.
		if _, err := texttemplate.New("requestLog").Parse(rlt); err != nil {
			return nil, err
		}
		oc.RequestLogTemplate = rlt
	}

	if eprl, ok := configMap.Data["logging.enable-probe-request-log"]; ok {
		oc.EnableProbeRequestLog = strings.EqualFold(eprl, "true")
	}

	if mb, ok := configMap.Data["metrics.request-metrics-backend-destination"]; ok {
		oc.RequestMetricsBackend = mb
	}

	if prof, ok := configMap.Data["profiling.enable"]; ok {
		oc.EnableProfiling = strings.EqualFold(prof, "true")
	}

	return oc, nil
}

// ConfigMapName gets the name of the metrics ConfigMap
func ConfigMapName() string {
	cm := os.Getenv(ConfigMapNameEnv)
	if cm == "" {
		return "config-observability"
	}
	return cm
}
