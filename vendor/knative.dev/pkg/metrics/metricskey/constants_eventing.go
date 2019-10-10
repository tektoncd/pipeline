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

	// ResourceTypeKnativeSource is the Stackdriver resource type for Knative Sources.
	ResourceTypeKnativeSource = "knative_source"

	// LabelName is the label for the name of the resource.
	LabelName = "name"

	// LabelResourceGroup is the name of the resource CRD.
	LabelResourceGroup = "resource_group"

	// LabelBrokerName is the label for the name of the Trigger's broker.
	LabelBrokerName = "broker_name"

	// LabelEventType is the label for the name of the event type.
	LabelEventType = "event_type"

	// LabelEventSource is the label for the name of the event source.
	LabelEventSource = "event_source"

	// LabelFilterType is the label for the Trigger filter attribute "type".
	LabelFilterType = "filter_type"

	// LabelFilterSource is the label for the Trigger filter attribute "source".
	LabelFilterSource = "filter_source"
)

var (
	// KnativeTriggerLabels stores the set of resource labels for resource type knative_trigger.
	KnativeTriggerLabels = sets.NewString(
		LabelProject,
		LabelLocation,
		LabelClusterName,
		LabelNamespaceName,
		LabelName,
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
		LabelName,
	)

	// KnativeBrokerMetrics stores a set of metric types which are supported
	// by resource type knative_trigger.
	KnativeBrokerMetrics = sets.NewString(
		"knative.dev/eventing/broker/event_count",
	)

	// KnativeSourceLabels stores the set of resource labels for resource type knative_source.
	KnativeSourceLabels = sets.NewString(
		LabelProject,
		LabelLocation,
		LabelClusterName,
		LabelNamespaceName,
		LabelName,
		LabelResourceGroup,
	)

	// KnativeSourceMetrics stores a set of metric types which are supported
	// by resource type knative_source.
	KnativeSourceMetrics = sets.NewString(
		"knative.dev/eventing/source/event_count",
	)
)
