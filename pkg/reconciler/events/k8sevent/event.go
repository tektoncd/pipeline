/*
Copyright 2022 The Tekton Authors

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

package k8sevent

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
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

// EmitK8sEvents emits kubernetes events for object
// k8s events are always sent if afterCondition is different from beforeCondition
func EmitK8sEvents(ctx context.Context, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	recorder := controller.GetEventRecorder(ctx)
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
			recorder.Event(object, corev1.EventTypeNormal, EventReasonSucceded, afterCondition.Message)
		case corev1.ConditionFalse:
			recorder.Event(object, corev1.EventTypeWarning, EventReasonFailed, afterCondition.Message)
		case corev1.ConditionUnknown:
			if beforeCondition == nil {
				// If the condition changed, the status is "unknown", and there was no condition before,
				// we emit the "Started event". We ignore further updates of the "unknown" status.
				recorder.Event(object, corev1.EventTypeNormal, EventReasonStarted, "")
			} else {
				// If the condition changed, the status is "unknown", and there was a condition before,
				// we emit an event that matches the reason and message of the condition.
				// This is used for instance to signal the transition from "started" to "running"
				recorder.Event(object, corev1.EventTypeNormal, afterCondition.Reason, afterCondition.Message)
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
