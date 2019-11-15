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

package metricskey

import "k8s.io/apimachinery/pkg/util/sets"

// TODO should be moved to eventing. See https://github.com/knative/pkg/issues/608

const (
	// ResourceTypeKnativeTrigger is the Stackdriver resource type for Knative Triggers.
	ResourceTypeKnativeTrigger = "knative_trigger"

	// ResourceTypeKnativeBroker is the Stackdriver resource type for Knative Brokers.
	ResourceTypeKnativeBroker = "knative_broker"

	// ResourceTypeKnativeImporter is the Stackdriver resource type for Knative Importers.
	ResourceTypeKnativeImporter = "knative_importer"

	// LabelTriggerName is the label for the name of the Trigger.
	LabelTriggerName = "trigger_name"

	// LabelBrokerName is the label for the name of the Broker.
	LabelBrokerName = "broker_name"

	// LabelEventType is the label for the name of the event type.
	LabelEventType = "event_type"

	// LabelEventSource is the label for the name of the event source.
	LabelEventSource = "event_source"

	// LabelFilterType is the label for the Trigger filter attribute "type".
	LabelFilterType = "filter_type"

	// LabelFilterSource is the label for the Trigger filter attribute "source".
	LabelFilterSource = "filter_source"

	// LabelImporterName is the label for the name of the Importer.
	LabelImporterName = "importer_name"

	// LabelImporterResourceGroup is the name of the Importer CRD.
	LabelImporterResourceGroup = "importer_resource_group"
)

var (
	// KnativeTriggerLabels stores the set of resource labels for resource type knative_trigger.
	KnativeTriggerLabels = sets.NewString(
		LabelProject,
		LabelLocation,
		LabelClusterName,
		LabelNamespaceName,
		LabelTriggerName,
		LabelBrokerName,
	)

	// KnativeTriggerMetrics stores a set of metric types which are supported
	// by resource type knative_trigger.
	KnativeTriggerMetrics = sets.NewString(
		"knative.dev/eventing/trigger/event_count",
		"knative.dev/eventing/trigger/event_processing_latencies",
		"knative.dev/eventing/trigger/event_dispatch_latencies",
	)

	// KnativeBrokerLabels stores the set of resource labels for resource type knative_broker.
	KnativeBrokerLabels = sets.NewString(
		LabelProject,
		LabelLocation,
		LabelClusterName,
		LabelNamespaceName,
		LabelBrokerName,
	)

	// KnativeBrokerMetrics stores a set of metric types which are supported
	// by resource type knative_trigger.
	KnativeBrokerMetrics = sets.NewString(
		"knative.dev/eventing/broker/event_count",
	)

	// KnativeImporterLabels stores the set of resource labels for resource type knative_importer.
	KnativeImporterLabels = sets.NewString(
		LabelProject,
		LabelLocation,
		LabelClusterName,
		LabelNamespaceName,
		LabelImporterName,
		LabelImporterResourceGroup,
	)

	// KnativeImporterMetrics stores a set of metric types which are supported
	// by resource type knative_importer.
	KnativeImporterMetrics = sets.NewString(
		"knative.dev/eventing/importer/event_count",
	)
)
