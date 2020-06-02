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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
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

// Emit emits an event for object if afterCondition is different from beforeCondition
//
// Status "ConditionUnknown":
//   beforeCondition == nil, emit EventReasonStarted
//   beforeCondition != nil, emit afterCondition.Reason
//
//  Status "ConditionTrue": emit EventReasonSucceded
//  Status "ConditionFalse": emit EventReasonFailed
//
func Emit(c record.EventRecorder, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
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

// EmitEvent emits an event for object if afterCondition is different from beforeCondition
//
// Status "ConditionUnknown":
//   beforeCondition == nil, emit EventReasonStarted
//   beforeCondition != nil, emit afterCondition.Reason
//
//  Status "ConditionTrue": emit EventReasonSucceded
//  Status "ConditionFalse": emit EventReasonFailed
// Deprecated: use Emit
func EmitEvent(c record.EventRecorder, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	Emit(c, beforeCondition, afterCondition, object)
}

// EmitErrorEvent emits a failure associated to an error
// Deprecated: use EmitError instead
func EmitErrorEvent(c record.EventRecorder, err error, object runtime.Object) {
	EmitError(c, err, object)
}
