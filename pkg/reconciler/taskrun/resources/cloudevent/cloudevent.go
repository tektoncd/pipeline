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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	"knative.dev/eventing-contrib/pkg/kncloudevents"
	"knative.dev/pkg/apis"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// TektonEventType holds the types of cloud events sent by Tekton
type TektonEventType string

const (
	// TektonTaskRunUnknownV1 is sent for TaskRuns with "ConditionSucceeded" "Unknown"
	TektonTaskRunUnknownV1 TektonEventType = "dev.tekton.event.task.unknown.v1"
	// TektonTaskRunSuccessfulV1 is sent for TaskRuns with "ConditionSucceeded" "True"
	TektonTaskRunSuccessfulV1 TektonEventType = "dev.tekton.event.task.successful.v1"
	// TektonTaskRunFailedV1 is sent for TaskRuns with "ConditionSucceeded" "False"
	TektonTaskRunFailedV1 TektonEventType = "dev.tekton.event.task.failed.v1"
)

// CEClient matches the `Client` interface from github.com/cloudevents/sdk-go/pkg/cloudevents
type CEClient client.Client

// TektonCloudEventData type is used to marshal and unmarshal the payload of
// a Tekton cloud event. It only includes a TaskRun for now. Using a type opens
// the possibility for the future to add more data to the payload
type TektonCloudEventData struct {
	TaskRun *v1alpha1.TaskRun `json:"taskRun"`
}

// NewTektonCloudEventData returns a new instance of NewTektonCloudEventData
func NewTektonCloudEventData(taskRun *v1alpha1.TaskRun) TektonCloudEventData {
	return TektonCloudEventData{
		TaskRun: taskRun,
	}
}

// SendCloudEvent sends a Cloud Event to the specified SinkURI
func SendCloudEvent(sinkURI, eventID, eventSourceURI string, data []byte, eventType TektonEventType, logger *zap.SugaredLogger, cloudEventClient CEClient) (cloudevents.Event, error) {
	var event cloudevents.Event

	cloudEventSource := types.ParseURLRef(eventSourceURI)
	if cloudEventSource == nil {
		logger.Errorf("Invalid eventSourceURI: %s", eventSourceURI)
		return event, fmt.Errorf("invalid eventSourceURI: %s", eventSourceURI)
	}

	event = cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:         eventID,
			Type:       string(eventType),
			Source:     *cloudEventSource,
			Time:       &types.Timestamp{Time: time.Now()},
			Extensions: nil,
		}.AsV02(),
		Data: data,
	}
	ctxt := cecontext.WithTarget(context.TODO(), sinkURI)
	_, err := cloudEventClient.Send(ctxt, event)
	if err != nil {
		logger.Errorf("Error sending the cloud-event: %s", err)
		return event, err
	}
	return event, nil
}

// SendTaskRunCloudEvent sends a cloud event for a TaskRun
func SendTaskRunCloudEvent(sinkURI string, taskRun *v1alpha1.TaskRun, logger *zap.SugaredLogger, cloudEventClient CEClient) (cloudevents.Event, error) {
	var event cloudevents.Event
	var err error
	// Check if a client was provided, if not build one on the fly
	if cloudEventClient == nil {
		cloudEventClient, err = kncloudevents.NewDefaultClient()
		if err != nil {
			logger.Errorf("Error creating the cloud-event client: %s", err)
			return event, err
		}
	}
	// Check if the TaskRun is defined
	if taskRun == nil {
		return event, errors.New("Cannot send an event for an empty TaskRun")
	}
	eventID := taskRun.ObjectMeta.Name
	taskRunStatus := taskRun.Status.GetCondition(apis.ConditionSucceeded)
	var eventType TektonEventType
	switch {
	case taskRunStatus.IsUnknown():
		eventType = TektonTaskRunUnknownV1
	case taskRunStatus.IsFalse():
		eventType = TektonTaskRunFailedV1
	case taskRunStatus.IsTrue():
		eventType = TektonTaskRunSuccessfulV1
	default:
		return event, fmt.Errorf("unknown condition for in TaskRun.Status %s", taskRunStatus.Status)
	}
	eventSourceURI := taskRun.ObjectMeta.SelfLink
	data, _ := json.Marshal(NewTektonCloudEventData(taskRun))
	event, err = SendCloudEvent(sinkURI, eventID, eventSourceURI, data, eventType, logger, cloudEventClient)
	return event, err
}

// GetCloudEventDeliveryCompareOptions returns compare options to sort
// and compare a list of CloudEventDelivery
func GetCloudEventDeliveryCompareOptions() []cmp.Option {
	// Setup cmp options
	cloudDeliveryStateCompare := func(x, y v1alpha1.CloudEventDeliveryState) bool {
		return cmp.Equal(x.Condition, y.Condition) && cmp.Equal(x.RetryCount, y.RetryCount)
	}
	less := func(x, y v1alpha1.CloudEventDelivery) bool {
		return strings.Compare(x.Target, y.Target) < 0 || (strings.Compare(x.Target, y.Target) == 0 && x.Status.SentAt.Before(y.Status.SentAt))
	}
	return []cmp.Option{
		cmpopts.SortSlices(less),
		cmp.Comparer(func(x, y v1alpha1.CloudEventDelivery) bool {
			return (strings.Compare(x.Target, y.Target) == 0) && cloudDeliveryStateCompare(x.Status, y.Status)
		}),
	}
}
