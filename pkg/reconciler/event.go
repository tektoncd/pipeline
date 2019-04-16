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
package reconciler

import (
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// EmitEvent emits success or failed event for object
// if afterCondition is different from beforeCondition
func EmitEvent(c record.EventRecorder, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	if beforeCondition != afterCondition && afterCondition != nil {
		// Create events when the obj result is in.
		if afterCondition.Status == corev1.ConditionTrue {
			c.Event(object, corev1.EventTypeNormal, "Succeeded", afterCondition.Message)
		} else if afterCondition.Status == corev1.ConditionFalse {
			c.Event(object, corev1.EventTypeWarning, "Failed", afterCondition.Message)
		}
	}
}
