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
	"fmt"
	"path"
	"sync"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"knative.dev/pkg/metrics/metricskey"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	customMetricTypePrefix = "custom.googleapis.com"
	// defaultCustomMetricSubDomain is the default subdomain to use for unsupported metrics by monitored resource types.
	// See: https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.metricDescriptors#MetricDescriptor
	defaultCustomMetricSubDomain = "knative.dev"
	// StackdriverSecretNamespaceDefault is the default namespace to search for a k8s Secret to pass to Stackdriver client to authenticate with Stackdriver.
	StackdriverSecretNamespaceDefault = "default"
	// StackdriverSecretNameDefault is the default name of the k8s Secret to pass to Stackdriver client to authenticate with Stackdriver.
	StackdriverSecretNameDefault = "stackdriver-service-account-key"
	// secretDataFieldKey is the name of the k8s Secret field that contains the Secret's key.
	secretDataFieldKey = "key.json"
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

	// kubeclient is the in-cluster Kubernetes kubeclient, which is lazy-initialized on first use.
	kubeclient *kubernetes.Clientset
	// initClientOnce is the lazy initializer for kubeclient.
	initClientOnce sync.Once
	// kubeclientInitErr capture an error during initClientOnce
	kubeclientInitErr error

	// stackdriverMtx protects setting secretNamespace and secretName and useStackdriverSecretEnabled
	stackdriverMtx sync.RWMutex
	// secretName is the name of the k8s Secret to pass to Stackdriver client to authenticate with Stackdriver.
	secretName = StackdriverSecretNameDefault
	// secretNamespace is the namespace to search for a k8s Secret to pass to Stackdriver client to authenticate with Stackdriver.
	secretNamespace = StackdriverSecretNamespaceDefault
	// useStackdriverSecretEnabled specifies whether or not the exporter can be configured with a Secret.
	// Consuming packages must do explicitly enable this by calling SetStackdriverSecretLocation.
	useStackdriverSecretEnabled = false
)

// SetStackdriverSecretLocation sets the name and namespace of the Secret that can be used to authenticate with Stackdriver.
// The Secret is only used if both:
// 1. This function has been explicitly called to set the name and namespace
// 2. Users set metricsConfig.stackdriverClientConfig.UseSecret to "true"
func SetStackdriverSecretLocation(name string, namespace string) {
	stackdriverMtx.Lock()
	defer stackdriverMtx.Unlock()
	secretName = name
	secretNamespace = namespace
	useStackdriverSecretEnabled = true
}

func init() {
	// Set gcpMetadataFunc to call GCP metadata service.
	gcpMetadataFunc = retrieveGCPMetadata
	newStackdriverExporterFunc = newOpencensusSDExporter

	kubeclientInitErr = nil
}

func newOpencensusSDExporter(o stackdriver.Options) (view.Exporter, error) {
	e, err := stackdriver.NewExporter(o)
	if err == nil {
		// Start the exporter.
		// TODO(https://github.com/knative/pkg/issues/866): Move this to an interface.
		e.StartMetricsExporter()
	}
	return e, nil
}

// TODO should be properly refactored to be able to inject the getResourceByDescriptorFunc function.
// 	See https://github.com/knative/pkg/issues/608
func newStackdriverExporter(config *metricsConfig, logger *zap.SugaredLogger) (view.Exporter, error) {
	gm := getMergedGCPMetadata(config)
	mpf := getMetricPrefixFunc(config.stackdriverMetricTypePrefix, config.stackdriverCustomMetricTypePrefix)
	co, err := getStackdriverExporterClientOptions(config)
	if err != nil {
		logger.Warnw("Issue configuring Stackdriver exporter client options, no additional client options will be used: ", zap.Error(err))
	}
	// Automatically fall back on Google application default credentials
	e, err := newStackdriverExporterFunc(stackdriver.Options{
		ProjectID:               gm.project,
		Location:                gm.location,
		MonitoringClientOptions: co,
		TraceClientOptions:      co,
		GetMetricPrefix:         mpf,
		ResourceByDescriptor:    getResourceByDescriptorFunc(config.stackdriverMetricTypePrefix, gm),
		ReportingInterval:       config.reportingPeriod,
		DefaultMonitoringLabels: &stackdriver.Labels{},
	})
	if err != nil {
		logger.Errorw("Failed to create the Stackdriver exporter: ", zap.Error(err))
		return nil, err
	}
	logger.Infof("Created Opencensus Stackdriver exporter with config %v", config)
	return e, nil
}

// getStackdriverExporterClientOptions creates client options for the opencensus Stackdriver exporter from the given stackdriverClientConfig.
// On error, an empty array of client options is returned.
func getStackdriverExporterClientOptions(config *metricsConfig) ([]option.ClientOption, error) {
	var co []option.ClientOption

	// SetStackdriverSecretLocation must have been called by calling package for this to work.
	if config.stackdriverClientConfig.UseSecret {
		if config.secret == nil {
			return co, fmt.Errorf("No secret provided for component %q; cannot use stackdriver-use-secret=true", config.component)
		}

		if opt, err := convertSecretToExporterOption(config.secret); err == nil {
			co = append(co, opt)
		} else {
			return co, err
		}
	}
	return co, nil
}

// getMergedGCPMetadata returns GCP metadata required to export metrics
// to Stackdriver. Values can come from the GCE metadata server or the config.
//  Values explicitly set in the config take the highest precedent.
func getMergedGCPMetadata(config *metricsConfig) *gcpMetadata {
	gm := gcpMetadataFunc()
	if config.stackdriverClientConfig.ProjectID != "" {
		gm.project = config.stackdriverClientConfig.ProjectID
	}

	if config.stackdriverClientConfig.GCPLocation != "" {
		gm.location = config.stackdriverClientConfig.GCPLocation
	}

	if config.stackdriverClientConfig.ClusterName != "" {
		gm.cluster = config.stackdriverClientConfig.ClusterName
	}

	return gm
}

func getResourceByDescriptorFunc(metricTypePrefix string, gm *gcpMetadata) func(*metricdata.Descriptor, map[string]string) (map[string]string, monitoredresource.Interface) {
	return func(des *metricdata.Descriptor, tags map[string]string) (map[string]string, monitoredresource.Interface) {
		metricType := path.Join(metricTypePrefix, des.Name)
		if metricskey.KnativeRevisionMetrics.Has(metricType) {
			return GetKnativeRevisionMonitoredResource(des, tags, gm)
		} else if metricskey.KnativeBrokerMetrics.Has(metricType) {
			return GetKnativeBrokerMonitoredResource(des, tags, gm)
		} else if metricskey.KnativeTriggerMetrics.Has(metricType) {
			return GetKnativeTriggerMonitoredResource(des, tags, gm)
		} else if metricskey.KnativeSourceMetrics.Has(metricType) {
			return GetKnativeSourceMonitoredResource(des, tags, gm)
		}
		// Unsupported metric by knative_revision, knative_broker, knative_trigger, and knative_source, use "global" resource type.
		return getGlobalMonitoredResource(des, tags)
	}
}

func getGlobalMonitoredResource(des *metricdata.Descriptor, tags map[string]string) (map[string]string, monitoredresource.Interface) {
	return tags, &Global{}
}

func getMetricPrefixFunc(metricTypePrefix, customMetricTypePrefix string) func(name string) string {
	return func(name string) string {
		metricType := path.Join(metricTypePrefix, name)
		inServing := metricskey.KnativeRevisionMetrics.Has(metricType)
		inEventing := metricskey.KnativeBrokerMetrics.Has(metricType) ||
			metricskey.KnativeTriggerMetrics.Has(metricType) ||
			metricskey.KnativeSourceMetrics.Has(metricType)
		if inServing || inEventing {
			return metricTypePrefix
		}
		// Unsupported metric by knative_revision, use custom domain.
		return customMetricTypePrefix
	}
}

// getStackdriverSecret returns the Kubernetes Secret specified in the given config.
// SetStackdriverSecretLocation must have been called by calling package for this to work.
// TODO(anniefu): Update exporter if Secret changes (https://github.com/knative/pkg/issues/842)
func getStackdriverSecret(secretFetcher SecretFetcher) (*corev1.Secret, error) {
	stackdriverMtx.RLock()
	defer stackdriverMtx.RUnlock()

	if !useStackdriverSecretEnabled {
		return nil, nil
	}

	var secErr error
	var sec *corev1.Secret
	if secretFetcher != nil {
		sec, secErr = secretFetcher(fmt.Sprintf("%s/%s", secretNamespace, secretName))
	} else {
		// This else-block can be removed once UpdateExporterFromConfigMap is fully deprecated in favor of ConfigMapWatcher
		if err := ensureKubeclient(); err != nil {
			return nil, err
		}

		sec, secErr = kubeclient.CoreV1().Secrets(secretNamespace).Get(secretName, metav1.GetOptions{})
	}

	if secErr != nil {
		return nil, fmt.Errorf("error getting Secret [%v] in namespace [%v]: %v", secretName, secretNamespace, secErr)
	}

	return sec, nil
}

// convertSecretToExporterOption converts a Kubernetes Secret to an OpenCensus Stackdriver Exporter Option.
func convertSecretToExporterOption(secret *corev1.Secret) (option.ClientOption, error) {
	if data, ok := secret.Data[secretDataFieldKey]; ok {
		return option.WithCredentialsJSON(data), nil
	}
	return nil, fmt.Errorf("Expected Secret to store key in data field named [%s]", secretDataFieldKey)
}

// ensureKubeclient is the lazy initializer for kubeclient.
func ensureKubeclient() error {
	// initClientOnce is only run once and cannot return error, so kubeclientInitErr is used to capture errors.
	initClientOnce.Do(func() {
		config, err := rest.InClusterConfig()
		if err != nil {
			kubeclientInitErr = err
			return
		}

		cs, err := kubernetes.NewForConfig(config)
		if err != nil {
			kubeclientInitErr = err
			return
		}
		kubeclient = cs
	})

	return kubeclientInitErr
}
