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
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

func (t TektonEventType) String() string {
	return string(t)
}

// CEClient matches the `Client` interface from github.com/cloudevents/sdk-go/v2/cloudevents
type CEClient cloudevents.Client

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

// EventForTaskRun will create a new event based on a TaskRun,
// or return an error if not possible.
func EventForTaskRun(taskRun *v1alpha1.TaskRun) (*cloudevents.Event, error) {
	// Check if the TaskRun is defined
	if taskRun == nil {
		return nil, errors.New("Cannot send an event for an empty TaskRun")
	}
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetSubject(taskRun.ObjectMeta.Name)
	event.SetSource(taskRun.ObjectMeta.SelfLink) // TODO: SelfLink is deprecated

	c := taskRun.Status.GetCondition(apis.ConditionSucceeded)
	switch {
	case c.IsUnknown():
		event.SetType(TektonTaskRunUnknownV1.String())
	case c.IsFalse():
		event.SetType(TektonTaskRunFailedV1.String())
	case c.IsTrue():
		event.SetType(TektonTaskRunSuccessfulV1.String())
	default:
		return nil, fmt.Errorf("unknown condition for in TaskRun.Status %s", c.Status)
	}

	if err := event.SetData(cloudevents.ApplicationJSON, NewTektonCloudEventData(taskRun)); err != nil {
		return nil, err
	}
	return &event, nil
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
