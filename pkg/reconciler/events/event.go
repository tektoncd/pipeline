/*
Copyright 2019 The Tekton Authors

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

package events

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	// EventReasonSucceded is the reason set for events about successful completion of TaskRuns / PipelineRuns
	EventReasonSucceded = "Succeeded"
	// EventReasonFailed is the reason set for events about unsuccessful completion of TaskRuns / PipelineRuns
	EventReasonFailed = "Failed"
	// EventReasonStarted is the reason set for events about the start of TaskRuns / PipelineRuns
	EventReasonStarted = "Started"
	// EventReasonError is the reason set for events related to TaskRuns / PipelineRuns reconcile errors
	EventReasonError = "Error"
)

// Emit emits events for object
// Two types of events are supported, k8s and cloud events.
//
// k8s events are always sent if afterCondition is different from beforeCondition
// Cloud events are always sent if enabled, i.e. if a sink is available
func Emit(ctx context.Context, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	recorder := controller.GetEventRecorder(ctx)
	logger := logging.FromContext(ctx)
	configs := config.FromContextOrDefaults(ctx)
	sendCloudEvents := (configs.Defaults.DefaultCloudEventsSink != "")
	if sendCloudEvents {
		ctx = cloudevents.ContextWithTarget(ctx, configs.Defaults.DefaultCloudEventsSink)
	}

	sendKubernetesEvents(recorder, beforeCondition, afterCondition, object)

	if sendCloudEvents {
		// Only send events if the new condition represents a change
		if !equality.Semantic.DeepEqual(beforeCondition, afterCondition) {
			err := cloudevent.SendCloudEventWithRetries(ctx, object)
			if err != nil {
				logger.Warnf("Failed to emit cloud events %v", err.Error())
			}
		}
	}
}

func sendKubernetesEvents(c record.EventRecorder, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	// Events that are going to be sent
	//
	// Status "ConditionUnknown":
	//   beforeCondition == nil, emit EventReasonStarted
	//   beforeCondition != nil, emit afterCondition.Reason
	//
	//  Status "ConditionTrue": emit EventReasonSucceded
	//  Status "ConditionFalse": emit EventReasonFailed
	if !equality.Semantic.DeepEqual(beforeCondition, afterCondition) && afterCondition != nil {
		// If the condition changed, and the target condition is not empty, we send an event
		switch afterCondition.Status {
		case corev1.ConditionTrue:
			c.Event(object, corev1.EventTypeNormal, EventReasonSucceded, afterCondition.Message)
		case corev1.ConditionFalse:
			c.Event(object, corev1.EventTypeWarning, EventReasonFailed, afterCondition.Message)
		case corev1.ConditionUnknown:
			if beforeCondition == nil {
				// If the condition changed, the status is "unknown", and there was no condition before,
				// we emit the "Started event". We ignore further updates of the "unknown" status.
				c.Event(object, corev1.EventTypeNormal, EventReasonStarted, "")
			} else {
				// If the condition changed, the status is "unknown", and there was a condition before,
				// we emit an event that matches the reason and message of the condition.
				// This is used for instance to signal the transition from "started" to "running"
				c.Event(object, corev1.EventTypeNormal, afterCondition.Reason, afterCondition.Message)
			}
		}
	}
}

// EmitError emits a failure associated to an error
func EmitError(c record.EventRecorder, err error, object runtime.Object) {
	if err != nil {
		c.Event(object, corev1.EventTypeWarning, EventReasonError, err.Error())
	}
}
