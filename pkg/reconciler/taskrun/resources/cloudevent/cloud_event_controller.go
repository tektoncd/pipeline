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
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitializeCloudEvents initializes the CloudEvents part of the
// TaskRunStatus from a slice of PipelineResources
func InitializeCloudEvents(tr *v1alpha1.TaskRun, prs []*v1alpha1.PipelineResource) {
	// If there are no cloud event resources, this check will run on every reconcile
	if len(tr.Status.CloudEvents) == 0 {
		var targets []string
		for _, output := range prs {
			if output.Spec.Type == v1alpha1.PipelineResourceTypeCloudEvent {
				cer, _ := v1alpha1.NewCloudEventResource(output)
				targets = append(targets, cer.TargetURI)
			}
		}
		if len(targets) > 0 {
			tr.Status.CloudEvents = cloudEventDeliveryFromTargets(targets)
		}
	}
}

func cloudEventDeliveryFromTargets(targets []string) []v1alpha1.CloudEventDelivery {
	if len(targets) > 0 {
		initialState := v1alpha1.CloudEventDeliveryState{
			Condition:  v1alpha1.CloudEventConditionUnknown,
			RetryCount: 0,
		}
		events := make([]v1alpha1.CloudEventDelivery, len(targets))
		for idx, target := range targets {
			events[idx] = v1alpha1.CloudEventDelivery{
				Target: target,
				Status: initialState,
			}
		}
		return events
	}
	return nil
}

// SendCloudEvents is used by the TaskRun controller to send cloud events once
// the TaskRun is complete. `tr` is used to obtain the list of targets but also
// to construct the body of the
func SendCloudEvents(tr *v1alpha1.TaskRun, ceclient CEClient, logger *zap.SugaredLogger) error {
	// Using multierror here so we can attempt to send all cloud events defined,
	// regardless of whether they fail or not, and report all failed ones
	var merr *multierror.Error
	for idx, cloudEventDelivery := range tr.Status.CloudEvents {
		eventStatus := &(tr.Status.CloudEvents[idx].Status)
		// Skip events that have already been sent (successfully or unsuccessfully)
		// Ensure we try to send all events once (possibly through different reconcile calls)
		if eventStatus.Condition != v1alpha1.CloudEventConditionUnknown || eventStatus.RetryCount > 0 {
			continue
		}
		_, err := SendTaskRunCloudEvent(cloudEventDelivery.Target, tr, logger, ceclient)
		eventStatus.SentAt = &metav1.Time{Time: time.Now()}
		eventStatus.RetryCount++
		if err != nil {
			merr = multierror.Append(merr, err)
			eventStatus.Condition = v1alpha1.CloudEventConditionFailed
			eventStatus.Error = merr.Error()
		} else {
			logger.Infof("Sent event for target %s", cloudEventDelivery.Target)
			eventStatus.Condition = v1alpha1.CloudEventConditionSent
		}
	}
	if merr != nil && merr.Len() > 0 {
		logger.Errorf("Failed to send %d cloud events for TaskRun %s", merr.Len(), tr.Name)
	}
	return merr.ErrorOrNil()
}
