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
	"errors"
	"fmt"
	"strings"

	"go.opencensus.io/resource"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

var (
	curMetricsExporter view.Exporter
	curMetricsConfig   *metricsConfig
	mWorker            *metricsWorker
)

func init() {
	mWorker = newMetricsWorker()
	go mWorker.start()
}

// SecretFetcher is a function (extracted from SecretNamespaceLister) for fetching
// a specific Secret. This avoids requiring global or namespace list in controllers.
type SecretFetcher func(string) (*corev1.Secret, error)

type flushable interface {
	// Flush waits for metrics to be uploaded.
	Flush()
}

// ExporterOptions contains options for configuring the exporter.
type ExporterOptions struct {
	// Domain is the metrics domain. e.g. "knative.dev". Must be present.

	// TODO - using this as a prefix is being discussed here:
	//        https://github.com/knative/pkg/issues/2174
	//
	// OpenCensus uses the following format to construct full metric name:
	//    <domain>/<component>/<metric name from View>
	// Prometheus uses the following format to construct full metric name:
	//    <component>_<metric name from View>
	// Domain is actually not used if metrics backend is Prometheus.
	Domain string

	// Component is the name of the component that emits the metrics. e.g.
	// "activator", "queue_proxy". Should only contains alphabets and underscore.
	// Must be present.
	Component string

	// PrometheusPort is the port to expose metrics if metrics backend is Prometheus.
	// It should be between maxPrometheusPort and maxPrometheusPort. 0 value means
	// using the default 9090 value. It is ignored if metrics backend is not
	// Prometheus.
	PrometheusPort int

	// PrometheusHost is the host to expose metrics on if metrics backend is Prometheus.
	// The default value is "0.0.0.0". It is ignored if metrics backend is not
	// Prometheus.
	PrometheusHost string

	// ConfigMap is the data from config map config-observability. Must be present.
	// See https://github.com/knative/serving/blob/main/config/config-observability.yaml
	// for details.
	ConfigMap map[string]string

	// A lister for Secrets to allow dynamic configuration of outgoing TLS client cert.
	Secrets SecretFetcher `json:"-"`
}

// UpdateExporterFromConfigMap returns a helper func that can be used to update the exporter
// when a config map is updated.
//
// Deprecated: Callers should migrate to ConfigMapWatcher.
func UpdateExporterFromConfigMap(ctx context.Context, component string, logger *zap.SugaredLogger) func(configMap *corev1.ConfigMap) {
	return ConfigMapWatcher(ctx, component, nil, logger)
}

// ConfigMapWatcher returns a helper func which updates the exporter configuration based on
// values in the supplied ConfigMap. This method captures a corev1.SecretLister which is used
// to configure mTLS with the opencensus agent.
func ConfigMapWatcher(ctx context.Context, component string, secrets SecretFetcher, logger *zap.SugaredLogger) func(*corev1.ConfigMap) {
	domain := Domain()
	return func(configMap *corev1.ConfigMap) {
		UpdateExporter(ctx,
			ExporterOptions{
				Domain:    domain,
				Component: strings.ReplaceAll(component, "-", "_"),
				ConfigMap: configMap.Data,
				Secrets:   secrets,
			}, logger)
	}
}

// UpdateExporterFromConfigMapWithOpts returns a helper func that can be used to update the exporter
// when a config map is updated.
// opts.Component must be present.
// opts.ConfigMap must not be present as the value from the ConfigMap will be used instead.
func UpdateExporterFromConfigMapWithOpts(ctx context.Context, opts ExporterOptions, logger *zap.SugaredLogger) (func(configMap *corev1.ConfigMap), error) {
	if opts.Component == "" {
		return nil, errors.New("UpdateExporterFromConfigMapWithDefaults must provide Component")
	}
	if opts.ConfigMap != nil {
		return nil, errors.New("UpdateExporterFromConfigMapWithDefaults doesn't allow defaulting ConfigMap")
	}
	domain := opts.Domain
	if domain == "" {
		domain = Domain()
	}
	return func(configMap *corev1.ConfigMap) {
		UpdateExporter(ctx,
			ExporterOptions{
				Domain:         domain,
				Component:      opts.Component,
				ConfigMap:      configMap.Data,
				PrometheusPort: opts.PrometheusPort,
				Secrets:        opts.Secrets,
			}, logger)
	}, nil
}

// UpdateExporter updates the exporter based on the given ExporterOptions.
// This is a thread-safe function. The entire series of operations is locked
// to prevent a race condition between reading the current configuration
// and updating the current exporter.
func UpdateExporter(ctx context.Context, ops ExporterOptions, logger *zap.SugaredLogger) error {
	// TODO(https://github.com/knative/pkg/issues/1273): check if ops.secrets is `nil` after new metrics plan lands
	newConfig, err := createMetricsConfig(ctx, ops)
	if err != nil {
		if getCurMetricsConfig() == nil {
			// Fail the process if there doesn't exist an exporter.
			logger.Errorw("Failed to get a valid metrics config", zap.Error(err))
		} else {
			logger.Errorw("Failed to get a valid metrics config; Skip updating the metrics exporter", zap.Error(err))
		}
		return err
	}

	// Updating the metrics config and the metrics exporters needs to be atomic to
	// avoid using an outdated metrics config with new exporters.
	updateCmd := &updateMetricsConfigWithExporter{
		ctx:       ctx,
		newConfig: newConfig,
		done:      make(chan error),
	}
	mWorker.c <- updateCmd
	err = <-updateCmd.done
	return err
}

// isNewExporterRequired compares the non-nil newConfig against curMetricsConfig.
// When backend changes, we need to update the metrics exporter.
// This function must be called with the metricsMux reader (or writer) locked.
func isNewExporterRequired(newConfig *metricsConfig) bool {
	cc := curMetricsConfig
	if cc == nil || newConfig.backendDestination != cc.backendDestination {
		return true
	}

	// If the OpenCensus address has changed, restart the exporter.
	// TODO(evankanderson): Should we just always restart the opencensus agent?
	if newConfig.backendDestination == openCensus {
		return newConfig.collectorAddress != cc.collectorAddress || newConfig.requireSecure != cc.requireSecure
	}

	if newConfig.backendDestination == prometheus {
		return newConfig.prometheusHost != cc.prometheusHost || newConfig.prometheusPort != cc.prometheusPort
	}

	return false
}

// newMetricsExporter gets a metrics exporter based on the config.
// This function must be called with the metricsMux reader (or writer) locked.
func newMetricsExporter(config *metricsConfig, logger *zap.SugaredLogger) (view.Exporter, ResourceExporterFactory, error) {
	// If there is a Prometheus Exporter server running, stop it.
	resetCurPromSrv()

	factory := map[metricsBackend]func(*metricsConfig, *zap.SugaredLogger) (view.Exporter, ResourceExporterFactory, error){
		openCensus: newOpenCensusExporter,
		prometheus: newPrometheusExporter,
		none: func(*metricsConfig, *zap.SugaredLogger) (view.Exporter, ResourceExporterFactory, error) {
			noneFactory := func(*resource.Resource) (view.Exporter, error) {
				return &noneExporter{}, nil
			}
			return &noneExporter{}, noneFactory, nil
		},
	}

	ff := factory[config.backendDestination]
	if ff == nil {
		return nil, nil, fmt.Errorf("unsupported metrics backend %v", config.backendDestination)
	}
	return ff(config, logger)
}

func getCurMetricsExporter() view.Exporter {
	readCmd := &readExporter{done: make(chan *view.Exporter)}
	mWorker.c <- readCmd
	e := <-readCmd.done
	return *e
}

func setCurMetricsExporter(e view.Exporter) {
	setCmd := &setExporter{
		newExporter: &e,
		done:        make(chan struct{}),
	}
	mWorker.c <- setCmd
	<-setCmd.done
}

func getCurMetricsConfig() *metricsConfig {
	readCmd := &readMetricsConfig{done: make(chan *metricsConfig)}
	mWorker.c <- readCmd
	cfg := <-readCmd.done
	return cfg
}

func setCurMetricsConfig(c *metricsConfig) {
	setCmd := &setMetricsConfig{
		newConfig: c,
		done:      make(chan struct{}),
	}
	mWorker.c <- setCmd
	<-setCmd.done
}

func setCurMetricsConfigUnlocked(c *metricsConfig) {
	setReportingPeriod(c)
	curMetricsConfig = c
}

// FlushExporter waits for exported data to be uploaded.
// This should be called before the process shuts down or exporter is replaced.
// Return value indicates whether the exporter is flushable or not.
func FlushExporter() bool {
	e := getCurMetricsExporter()
	flushResourceExporters()
	return flushGivenExporter(e)
}

func flushGivenExporter(e view.Exporter) bool {
	if e == nil {
		return false
	}

	if f, ok := e.(flushable); ok {
		f.Flush()
		return true
	}
	return false
}

type noneExporter struct{}

// NoneExporter implements view.Exporter in the nil case.
func (*noneExporter) ExportView(*view.Data) {
}
