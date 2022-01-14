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
	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/cloudevent"
	"github.com/tektoncd/pipeline/pkg/clock"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controller "knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// InitializeCloudEvents initializes the CloudEvents part of the
// TaskRunStatus from a slice of PipelineResources
func InitializeCloudEvents(tr *v1beta1.TaskRun, prs map[string]*resource.PipelineResource) {
	// If there are no cloud event resources, this check will run on every reconcile
	if len(tr.Status.CloudEvents) == 0 {
		var targets []string
		for name, output := range prs {
			if output.Spec.Type == resource.PipelineResourceTypeCloudEvent {
				cer, _ := cloudevent.NewResource(name, output)
				targets = append(targets, cer.TargetURI)
			}
		}
		if len(targets) > 0 {
			tr.Status.CloudEvents = cloudEventDeliveryFromTargets(targets)
		}
	}
}

func cloudEventDeliveryFromTargets(targets []string) []v1beta1.CloudEventDelivery {
	if len(targets) > 0 {
		initialState := v1beta1.CloudEventDeliveryState{
			Condition:  v1beta1.CloudEventConditionUnknown,
			RetryCount: 0,
		}
		events := make([]v1beta1.CloudEventDelivery, len(targets))
		for idx, target := range targets {
			events[idx] = v1beta1.CloudEventDelivery{
				Target: target,
				Status: initialState,
			}
		}
		return events
	}
	return nil
}

// SendCloudEvents is used by the TaskRun controller to send cloud events once
// the TaskRun is complete. `tr` is used to obtain the list of targets
func SendCloudEvents(tr *v1beta1.TaskRun, ceclient CEClient, logger *zap.SugaredLogger, c clock.Clock) error {
	logger = logger.With(zap.String("taskrun", tr.Name))

	// Make the event we would like to send:
	event, err := eventForTaskRun(tr)
	if err != nil || event == nil {
		logger.With(zap.Error(err)).Error("failed to produce a cloudevent from TaskRun.")
		return err
	}

	// Using multierror here so we can attempt to send all cloud events defined,
	// regardless of whether they fail or not, and report all failed ones
	var merr *multierror.Error
	for idx, cloudEventDelivery := range tr.Status.CloudEvents {
		eventStatus := &(tr.Status.CloudEvents[idx].Status)
		// Skip events that have already been sent (successfully or unsuccessfully)
		// Ensure we try to send all events once (possibly through different reconcile calls)
		if eventStatus.Condition != v1beta1.CloudEventConditionUnknown || eventStatus.RetryCount > 0 {
			continue
		}

		// Send the event.
		result := ceclient.Send(cloudevents.ContextWithTarget(cloudevents.ContextWithRetriesExponentialBackoff(context.Background(), 10*time.Millisecond, 10), cloudEventDelivery.Target), *event)

		// Record the result.
		eventStatus.SentAt = &metav1.Time{Time: c.Now()}
		eventStatus.RetryCount++
		if !cloudevents.IsACK(result) {
			merr = multierror.Append(merr, result)
			eventStatus.Condition = v1beta1.CloudEventConditionFailed
			eventStatus.Error = merr.Error()
		} else {
			logger.Infow("Event sent.", zap.String("target", cloudEventDelivery.Target))
			eventStatus.Condition = v1beta1.CloudEventConditionSent
		}
	}
	if merr != nil && merr.Len() > 0 {
		logger.With(zap.Error(merr)).Errorw("Failed to send events for TaskRun.", zap.Int("count", merr.Len()))
	}
	return merr.ErrorOrNil()
}

// SendCloudEventWithRetries sends a cloud event for the specified resource.
// It does not block and it perform retries with backoff using the cloudevents
// sdk-go capabilities.
// It accepts a runtime.Object to avoid making objectWithCondition public since
// it's only used within the events/cloudevents packages.
func SendCloudEventWithRetries(ctx context.Context, object runtime.Object) error {
	var (
		o  objectWithCondition
		ok bool
	)
	if o, ok = object.(objectWithCondition); !ok {
		return errors.New("Input object does not satisfy objectWithCondition")
	}
	logger := logging.FromContext(ctx)
	ceClient := Get(ctx)
	if ceClient == nil {
		return errors.New("No cloud events client found in the context")
	}
	event, err := eventForObjectWithCondition(o)
	if err != nil {
		return err
	}

	wasIn := make(chan error)
	go func() {
		wasIn <- nil
		logger.Debugf("Sending cloudevent of type %q", event.Type())
		if result := ceClient.Send(cloudevents.ContextWithRetriesExponentialBackoff(ctx, 10*time.Millisecond, 10), *event); !cloudevents.IsACK(result) {
			logger.Warnf("Failed to send cloudevent: %s", result.Error())
			recorder := controller.GetEventRecorder(ctx)
			if recorder == nil {
				logger.Warnf("No recorder in context, cannot emit error event")
			}
			recorder.Event(object, corev1.EventTypeWarning, "Cloud Event Failure", result.Error())
		}
	}()

	return <-wasIn
}
