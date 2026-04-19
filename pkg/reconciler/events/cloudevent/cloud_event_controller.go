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

package cloudevent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	controller "knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

func cloudEventsSink(ctx context.Context) string {
	configs := config.FromContextOrDefaults(ctx)
	// Try the sink configuration first
	sink := configs.Events.Sink
	if sink == "" {
		// Fall back to the deprecated flag is the new one is not set
		// This ensures no changes in behaviour for existing users of the deprecated flag
		sink = configs.Defaults.DefaultCloudEventsSink
	}
	return sink
}

// EmitCloudEvents emits CloudEvents (only) for object.
//
// Checks the cache for the ObjectKey: gates on the full condition snapshot
// (Status+Reason+Message+GVK+name). A hit means this exact state was already
// processed; skip entirely — including the event build and send. A miss means
// a new object or the condition changed; proceed to call SendCloudEventWithRetries
func EmitCloudEvents(ctx context.Context, object runtime.Object) {
	logger := logging.FromContext(ctx)
	runObject, ok := object.(v1beta1.RunObject)
	if !ok {
		logger.Warnf("failed to emit cloud events, runtime.Object %v is not a v1beta1.RunObject", object)
		return
	}

	if sink := cloudEventsSink(ctx); sink != "" {
		// Level 1: skip if this exact condition state was already processed.
		cacheClient := cache.Get(ctx)
		wasPresent, err := cache.ContainsOrAddObject(cacheClient, runObject)
		if err != nil {
			logger.Warnf("failed to emit cloud events: could not check the events cache %v", err.Error())
		}
		if err == nil && !wasPresent {
			ctx = cloudevents.ContextWithTarget(ctx, sink)
			if err := SendCloudEventWithRetries(ctx, runObject); err != nil {
				logger.Warnf("failed to emit cloud events %v", err.Error())
			}
		}
	}
}

// EmitCloudEventsWhenConditionChange emits CloudEvents when there is a change in condition.
//
// Deprecated: CloudEvents are now sent by the dedicated tekton-events-controller
// (pkg/reconciler/notifications). This function is no longer called by any core reconciler
// and will be removed in a future release.
func EmitCloudEventsWhenConditionChange(ctx context.Context, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	logger := logging.FromContext(ctx)
	runObject, ok := object.(v1beta1.RunObject)
	if !ok {
		logger.Warnf("failed to emit cloud events, runtime.Object %v is not a v1beta1.RunObject", object)
	}
	if sink := cloudEventsSink(ctx); sink != "" {
		ctx = cloudevents.ContextWithTarget(ctx, sink)

		// Only send events if the new condition represents a change
		if !equality.Semantic.DeepEqual(beforeCondition, afterCondition) {
			err := SendCloudEventWithRetries(ctx, runObject)
			if err != nil {
				logger.Warnf("Failed to emit cloud events %v", err.Error())
			}
		}
	}
}

// SendCloudEventWithRetries sends a cloud event for the specified resource.
// It does not block and performs retries with backoff using the cloudevents
// sdk-go capabilities.
//
// Checks the cache for the EventKey: skips sending if this event type was already
// sent for this object, regardless of condition changes. This prevents duplicate
// terminal events (successful, failed). Unknown events are exempt:
// they must fire on every condition change while a run is in progress.
func SendCloudEventWithRetries(ctx context.Context, object runtime.Object) error {
	logger := logging.FromContext(ctx)
	runObject, ok := object.(v1beta1.RunObject)
	if !ok {
		return fmt.Errorf("failed to send cloud events, runtime.Object %v is not a v1beta1.RunObject", object)
	}
	ceClient := Get(ctx)
	if ceClient == nil {
		return errors.New("no cloud events client found in the context")
	}
	event, err := eventForRunObject(ctx, runObject)
	if err != nil {
		return err
	}

	// Skip if this event type was already sent for this object.
	// Unknown events are exempt — they fire on every condition change.
	if !strings.Contains(event.Type(), ".unknown.") {
		cacheClient := cache.Get(ctx)
		alreadySent, err := cache.ContainsOrAddCloudEvent(cacheClient, event, runObject)
		if err != nil {
			logger.Errorf("Error while checking cache: %s", err)
		}
		if alreadySent {
			logger.Infof("cloudevent %v already sent", event)
			return nil
		}
	}

	wasIn := make(chan error)

	ceClient.addCount()
	go func() {
		defer ceClient.decreaseCount()
		wasIn <- nil
		logger.Debugf("Sending cloudevent of type %q", event.Type())
		recorder := controller.GetEventRecorder(ctx)
		if result := ceClient.Send(cloudevents.ContextWithRetriesExponentialBackoff(ctx, 10*time.Millisecond, 10), *event); !cloudevents.IsACK(result) {
			logger.Warnf("Failed to send cloudevent: %s", result.Error())
			if recorder == nil {
				logger.Warnf("No recorder in context, cannot emit error event")
				return
			}
			recorder.Event(runObject, corev1.EventTypeWarning, "CloudEventFailed", result.Error())
		} else if recorder != nil {
			recorder.Eventf(runObject, corev1.EventTypeNormal, "CloudEventSent", "Sent %s", event.Type())
		}
	}()

	return <-wasIn
}
