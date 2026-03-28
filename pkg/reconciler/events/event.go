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

	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/k8sevent"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

// Emit emits events for object.
//
// k8s events are sent if afterCondition is different from beforeCondition.
func Emit(ctx context.Context, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	k8sevent.EmitK8sEvents(ctx, beforeCondition, afterCondition, object)
}

// EmitCloudEvents is refactored to cloudevent, this is to avoid breaking change.
//
// Deprecated: Use pkg/reconciler/events/cloudevent.EmitCloudEvents directly.
var EmitCloudEvents = cloudevent.EmitCloudEvents

// EmitError is refactored to k8sevent, this is to avoid breaking change
var EmitError = k8sevent.EmitError
