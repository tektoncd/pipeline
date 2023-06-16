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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	lru "github.com/hashicorp/golang-lru"
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

// EmitCloudEvents emits CloudEvents (only) for object
func EmitCloudEvents(ctx context.Context, object runtime.Object) {
	logger := logging.FromContext(ctx)
	configs := config.FromContextOrDefaults(ctx)
	sendCloudEvents := (configs.Defaults.DefaultCloudEventsSink != "")
	if sendCloudEvents {
		ctx = cloudevents.ContextWithTarget(ctx, configs.Defaults.DefaultCloudEventsSink)
	}

	if sendCloudEvents {
		err := SendCloudEventWithRetries(ctx, object)
		if err != nil {
			logger.Warnf("Failed to emit cloud events %v", err.Error())
		}
	}
}

// EmitCloudEventsWhenConditionChange emits CloudEvents when there is a change in condition
func EmitCloudEventsWhenConditionChange(ctx context.Context, beforeCondition *apis.Condition, afterCondition *apis.Condition, object runtime.Object) {
	logger := logging.FromContext(ctx)
	configs := config.FromContextOrDefaults(ctx)
	sendCloudEvents := (configs.Defaults.DefaultCloudEventsSink != "")
	if sendCloudEvents {
		ctx = cloudevents.ContextWithTarget(ctx, configs.Defaults.DefaultCloudEventsSink)
	}

	if sendCloudEvents {
		// Only send events if the new condition represents a change
		if !equality.Semantic.DeepEqual(beforeCondition, afterCondition) {
			err := SendCloudEventWithRetries(ctx, object)
			if err != nil {
				logger.Warnf("Failed to emit cloud events %v", err.Error())
			}
		}
	}
}

// SendCloudEventWithRetries sends a cloud event for the specified resource.
// It does not block and it perform retries with backoff using the cloudevents
// sdk-go capabilities.
// It accepts a runtime.Object to avoid making objectWithCondition public since
// it's only used within the events/cloudevents packages.
func SendCloudEventWithRetries(ctx context.Context, object runtime.Object) error {
	var (
		o           objectWithCondition
		ok          bool
		cacheClient *lru.Cache
	)
	if o, ok = object.(objectWithCondition); !ok {
		return errors.New("input object does not satisfy objectWithCondition")
	}
	logger := logging.FromContext(ctx)
	ceClient := Get(ctx)
	if ceClient == nil {
		return errors.New("no cloud events client found in the context")
	}
	event, err := eventForObjectWithCondition(ctx, o)
	if err != nil {
		return err
	}
	// Events for CustomRuns require a cache of events that have been sent
	_, isCustomRun := object.(*v1beta1.CustomRun)
	if isCustomRun {
		cacheClient = cache.Get(ctx)
	}

	wasIn := make(chan error)

	ceClient.addCount()
	go func() {
		defer ceClient.decreaseCount()
		wasIn <- nil
		logger.Debugf("Sending cloudevent of type %q", event.Type())
		// In case of Run event, check cache if cloudevent is already sent
		if isCustomRun {
			cloudEventSent, err := cache.ContainsOrAddCloudEvent(cacheClient, event)
			if err != nil {
				logger.Errorf("error while checking cache: %s", err)
			}
			if cloudEventSent {
				logger.Infof("cloudevent %v already sent", event)
				return
			}
		}
		if result := ceClient.Send(cloudevents.ContextWithRetriesExponentialBackoff(ctx, 10*time.Millisecond, 10), *event); !cloudevents.IsACK(result) {
			logger.Warnf("Failed to send cloudevent: %s", result.Error())
			recorder := controller.GetEventRecorder(ctx)
			if recorder == nil {
				logger.Warnf("No recorder in context, cannot emit error event")
				return
			}
			recorder.Event(object, corev1.EventTypeWarning, "Cloud Event Failure", result.Error())
		}
	}()

	return <-wasIn
}
