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

// TODO should be moved to eventing. See https://github.com/knative/pkg/issues/608

import (
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"go.opencensus.io/metric/metricdata"
	"knative.dev/pkg/metrics/metricskey"
)

type KnativeTrigger struct {
	Project               string
	Location              string
	ClusterName           string
	NamespaceName         string
	TriggerName           string
	BrokerName            string
	TypeFilterAttribute   string
	SourceFilterAttribute string
}

type KnativeBroker struct {
	Project       string
	Location      string
	ClusterName   string
	NamespaceName string
	BrokerName    string
}

type KnativeSource struct {
	Project             string
	Location            string
	ClusterName         string
	NamespaceName       string
	SourceName          string
	SourceResourceGroup string
}

func (kt *KnativeTrigger) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		metricskey.LabelProject:       kt.Project,
		metricskey.LabelLocation:      kt.Location,
		metricskey.LabelClusterName:   kt.ClusterName,
		metricskey.LabelNamespaceName: kt.NamespaceName,
		metricskey.LabelBrokerName:    kt.BrokerName,
		metricskey.LabelTriggerName:   kt.TriggerName,
	}
	return metricskey.ResourceTypeKnativeTrigger, labels
}

func (kb *KnativeBroker) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		metricskey.LabelProject:       kb.Project,
		metricskey.LabelLocation:      kb.Location,
		metricskey.LabelClusterName:   kb.ClusterName,
		metricskey.LabelNamespaceName: kb.NamespaceName,
		metricskey.LabelBrokerName:    kb.BrokerName,
	}
	return metricskey.ResourceTypeKnativeBroker, labels
}

func (ki *KnativeSource) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		metricskey.LabelProject:       ki.Project,
		metricskey.LabelLocation:      ki.Location,
		metricskey.LabelClusterName:   ki.ClusterName,
		metricskey.LabelNamespaceName: ki.NamespaceName,
		metricskey.LabelName:          ki.SourceName,
		metricskey.LabelResourceGroup: ki.SourceResourceGroup,
	}
	return metricskey.ResourceTypeKnativeSource, labels
}

func GetKnativeBrokerMonitoredResource(
	des *metricdata.Descriptor, tags map[string]string, gm *gcpMetadata) (map[string]string, monitoredresource.Interface) {
	kb := &KnativeBroker{
		// The first three resource labels are from metadata.
		Project:     gm.project,
		Location:    gm.location,
		ClusterName: gm.cluster,
		// The rest resource labels are from metrics labels.
		NamespaceName: valueOrUnknown(metricskey.LabelNamespaceName, tags),
		BrokerName:    valueOrUnknown(metricskey.LabelBrokerName, tags),
	}

	metricLabels := map[string]string{}
	for k, v := range tags {
		// Keep the metrics labels that are not resource labels
		if !metricskey.KnativeBrokerLabels.Has(k) {
			metricLabels[k] = v
		}
	}

	return metricLabels, kb
}

func GetKnativeTriggerMonitoredResource(
	des *metricdata.Descriptor, tags map[string]string, gm *gcpMetadata) (map[string]string, monitoredresource.Interface) {
	kt := &KnativeTrigger{
		// The first three resource labels are from metadata.
		Project:     gm.project,
		Location:    gm.location,
		ClusterName: gm.cluster,
		// The rest resource labels are from metrics labels.
		NamespaceName: valueOrUnknown(metricskey.LabelNamespaceName, tags),
		BrokerName:    valueOrUnknown(metricskey.LabelBrokerName, tags),
		TriggerName:   valueOrUnknown(metricskey.LabelTriggerName, tags),
	}

	metricLabels := map[string]string{}
	for k, v := range tags {
		// Keep the metrics labels that are not resource labels
		if !metricskey.KnativeTriggerLabels.Has(k) {
			metricLabels[k] = v
		}
	}

	return metricLabels, kt
}

func GetKnativeSourceMonitoredResource(
	des *metricdata.Descriptor, tags map[string]string, gm *gcpMetadata) (map[string]string, monitoredresource.Interface) {
	ks := &KnativeSource{
		// The first three resource labels are from metadata.
		Project:     gm.project,
		Location:    gm.location,
		ClusterName: gm.cluster,
		// The rest resource labels are from metrics labels.
		NamespaceName:       valueOrUnknown(metricskey.LabelNamespaceName, tags),
		SourceName:          valueOrUnknown(metricskey.LabelName, tags),
		SourceResourceGroup: valueOrUnknown(metricskey.LabelResourceGroup, tags),
	}

	metricLabels := map[string]string{}
	for k, v := range tags {
		// Keep the metrics labels that are not resource labels
		if !metricskey.KnativeSourceLabels.Has(k) {
			metricLabels[k] = v
		}
	}

	return metricLabels, ks
}
