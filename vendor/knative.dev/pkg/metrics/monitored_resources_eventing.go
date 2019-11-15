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
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
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

type KnativeImporter struct {
	Project               string
	Location              string
	ClusterName           string
	NamespaceName         string
	ImporterName          string
	ImporterResourceGroup string
}

func (kt *KnativeTrigger) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		metricskey.LabelProject:       kt.Project,
		metricskey.LabelLocation:      kt.Location,
		metricskey.LabelClusterName:   kt.ClusterName,
		metricskey.LabelNamespaceName: kt.NamespaceName,
		metricskey.LabelTriggerName:   kt.TriggerName,
		metricskey.LabelBrokerName:    kt.BrokerName,
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

func (ki *KnativeImporter) MonitoredResource() (resType string, labels map[string]string) {
	labels = map[string]string{
		metricskey.LabelProject:               ki.Project,
		metricskey.LabelLocation:              ki.Location,
		metricskey.LabelClusterName:           ki.ClusterName,
		metricskey.LabelNamespaceName:         ki.NamespaceName,
		metricskey.LabelImporterName:          ki.ImporterName,
		metricskey.LabelImporterResourceGroup: ki.ImporterResourceGroup,
	}
	return metricskey.ResourceTypeKnativeImporter, labels
}

func GetKnativeBrokerMonitoredResource(
	v *view.View, tags []tag.Tag, gm *gcpMetadata) ([]tag.Tag, monitoredresource.Interface) {
	tagsMap := getTagsMap(tags)
	kb := &KnativeBroker{
		// The first three resource labels are from metadata.
		Project:     gm.project,
		Location:    gm.location,
		ClusterName: gm.cluster,
		// The rest resource labels are from metrics labels.
		NamespaceName: valueOrUnknown(metricskey.LabelNamespaceName, tagsMap),
		BrokerName:    valueOrUnknown(metricskey.LabelBrokerName, tagsMap),
	}

	var newTags []tag.Tag
	for _, t := range tags {
		// Keep the metrics labels that are not resource labels
		if !metricskey.KnativeBrokerLabels.Has(t.Key.Name()) {
			newTags = append(newTags, t)
		}
	}

	return newTags, kb
}

func GetKnativeTriggerMonitoredResource(
	v *view.View, tags []tag.Tag, gm *gcpMetadata) ([]tag.Tag, monitoredresource.Interface) {
	tagsMap := getTagsMap(tags)
	kt := &KnativeTrigger{
		// The first three resource labels are from metadata.
		Project:     gm.project,
		Location:    gm.location,
		ClusterName: gm.cluster,
		// The rest resource labels are from metrics labels.
		NamespaceName: valueOrUnknown(metricskey.LabelNamespaceName, tagsMap),
		TriggerName:   valueOrUnknown(metricskey.LabelTriggerName, tagsMap),
		BrokerName:    valueOrUnknown(metricskey.LabelBrokerName, tagsMap),
	}

	var newTags []tag.Tag
	for _, t := range tags {
		// Keep the metrics labels that are not resource labels
		if !metricskey.KnativeTriggerLabels.Has(t.Key.Name()) {
			newTags = append(newTags, t)
		}
	}

	return newTags, kt
}

func GetKnativeImporterMonitoredResource(
	v *view.View, tags []tag.Tag, gm *gcpMetadata) ([]tag.Tag, monitoredresource.Interface) {
	tagsMap := getTagsMap(tags)
	ki := &KnativeImporter{
		// The first three resource labels are from metadata.
		Project:     gm.project,
		Location:    gm.location,
		ClusterName: gm.cluster,
		// The rest resource labels are from metrics labels.
		NamespaceName:         valueOrUnknown(metricskey.LabelNamespaceName, tagsMap),
		ImporterName:          valueOrUnknown(metricskey.LabelImporterName, tagsMap),
		ImporterResourceGroup: valueOrUnknown(metricskey.LabelImporterResourceGroup, tagsMap),
	}

	var newTags []tag.Tag
	for _, t := range tags {
		// Keep the metrics labels that are not resource labels
		if !metricskey.KnativeImporterLabels.Has(t.Key.Name()) {
			newTags = append(newTags, t)
		}
	}

	return newTags, ki
}
