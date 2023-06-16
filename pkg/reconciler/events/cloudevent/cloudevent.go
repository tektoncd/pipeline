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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

// TektonEventType holds the types of cloud events sent by Tekton
type TektonEventType string

const (
	// TaskRunStartedEventV1 is sent for TaskRuns with "ConditionSucceeded" "Unknown"
	// the first time they are picked up by the reconciler
	TaskRunStartedEventV1 TektonEventType = "dev.tekton.event.taskrun.started.v1"
	// TaskRunRunningEventV1 is sent for TaskRuns with "ConditionSucceeded" "Unknown"
	// once the TaskRun is validated and Pod created
	TaskRunRunningEventV1 TektonEventType = "dev.tekton.event.taskrun.running.v1"
	// TaskRunUnknownEventV1 is sent for TaskRuns with "ConditionSucceeded" "Unknown"
	// It can be used as a confirmation that the TaskRun is still running.
	TaskRunUnknownEventV1 TektonEventType = "dev.tekton.event.taskrun.unknown.v1"
	// TaskRunSuccessfulEventV1 is sent for TaskRuns with "ConditionSucceeded" "True"
	TaskRunSuccessfulEventV1 TektonEventType = "dev.tekton.event.taskrun.successful.v1"
	// TaskRunFailedEventV1 is sent for TaskRuns with "ConditionSucceeded" "False"
	TaskRunFailedEventV1 TektonEventType = "dev.tekton.event.taskrun.failed.v1"
	// PipelineRunStartedEventV1 is sent for PipelineRuns with "ConditionSucceeded" "Unknown"
	// the first time they are picked up by the reconciler
	PipelineRunStartedEventV1 TektonEventType = "dev.tekton.event.pipelinerun.started.v1"
	// PipelineRunRunningEventV1 is sent for PipelineRuns with "ConditionSucceeded" "Unknown"
	// once the PipelineRun is validated and Pod created
	PipelineRunRunningEventV1 TektonEventType = "dev.tekton.event.pipelinerun.running.v1"
	// PipelineRunUnknownEventV1 is sent for PipelineRuns with "ConditionSucceeded" "Unknown"
	// It can be used as a confirmation that the PipelineRun is still running.
	PipelineRunUnknownEventV1 TektonEventType = "dev.tekton.event.pipelinerun.unknown.v1"
	// PipelineRunSuccessfulEventV1 is sent for PipelineRuns with "ConditionSucceeded" "True"
	PipelineRunSuccessfulEventV1 TektonEventType = "dev.tekton.event.pipelinerun.successful.v1"
	// PipelineRunFailedEventV1 is sent for PipelineRuns with "ConditionSucceeded" "False"
	PipelineRunFailedEventV1 TektonEventType = "dev.tekton.event.pipelinerun.failed.v1"
	// CustomRunStartedEventV1 is sent for CustomRuns with "ConditionSucceeded" "Unknown"
	// the first time they are picked up by the reconciler
	CustomRunStartedEventV1 TektonEventType = "dev.tekton.event.customrun.started.v1"
	// CustomRunRunningEventV1 is sent for CustomRuns with "ConditionSucceeded" "Unknown"
	// once the CustomRun is validated and Pod created
	CustomRunRunningEventV1 TektonEventType = "dev.tekton.event.customrun.running.v1"
	// CustomRunSuccessfulEventV1 is sent for CustomRuns with "ConditionSucceeded" "True"
	CustomRunSuccessfulEventV1 TektonEventType = "dev.tekton.event.customrun.successful.v1"
	// CustomRunFailedEventV1 is sent for CustomRuns with "ConditionSucceeded" "False"
	CustomRunFailedEventV1 TektonEventType = "dev.tekton.event.customrun.failed.v1"
)

func (t TektonEventType) String() string {
	return string(t)
}

// CEClient wraps the `Client` interface from github.com/cloudevents/sdk-go/v2/cloudevents
// and has methods to count the cloud events being sent, those methods are for testing purposes.
type CEClient interface {
	cloudevents.Client
	// addCount increments the count of events to be sent
	addCount()
	// decreaseCount decrements the count of events to be sent, indicating the event has been sent
	decreaseCount()
}

// TektonCloudEventData type is used to marshal and unmarshal the payload of
// a Tekton cloud event. It can include a TaskRun or a PipelineRun
type TektonCloudEventData struct {
	TaskRun     *v1beta1.TaskRun     `json:"taskRun,omitempty"`
	PipelineRun *v1beta1.PipelineRun `json:"pipelineRun,omitempty"`
	CustomRun   *v1beta1.CustomRun   `json:"customRun,omitempty"`
}

// newTektonCloudEventData returns a new instance of TektonCloudEventData
func newTektonCloudEventData(ctx context.Context, runObject objectWithCondition) (TektonCloudEventData, error) {
	tektonCloudEventData := TektonCloudEventData{}
	switch v := runObject.(type) {
	case *v1beta1.TaskRun:
		tektonCloudEventData.TaskRun = v
	case *v1beta1.PipelineRun:
		tektonCloudEventData.PipelineRun = v
	case *v1.TaskRun:
		v1beta1TaskRun := &v1beta1.TaskRun{}
		if err := v1beta1TaskRun.ConvertFrom(ctx, v); err != nil {
			return TektonCloudEventData{}, err
		}
		tektonCloudEventData.TaskRun = v1beta1TaskRun
	case *v1.PipelineRun:
		v1beta1PipelineRun := &v1beta1.PipelineRun{}
		if err := v1beta1PipelineRun.ConvertFrom(ctx, v); err != nil {
			return TektonCloudEventData{}, err
		}
		tektonCloudEventData.PipelineRun = v1beta1PipelineRun
	case *v1beta1.CustomRun:
		tektonCloudEventData.CustomRun = v
	}
	return tektonCloudEventData, nil
}

// eventForObjectWithCondition creates a new event based for a objectWithCondition,
// or return an error if not possible.
func eventForObjectWithCondition(ctx context.Context, runObject objectWithCondition) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetSubject(runObject.GetObjectMeta().GetName())
	// TODO: SelfLink is deprecated https://github.com/tektoncd/pipeline/issues/2676
	source := runObject.GetObjectMeta().GetSelfLink()
	if source == "" {
		gvk := runObject.GetObjectKind().GroupVersionKind()
		source = fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s/%s",
			gvk.Group,
			gvk.Version,
			runObject.GetObjectMeta().GetNamespace(),
			gvk.Kind,
			runObject.GetObjectMeta().GetName())
	}
	event.SetSource(source)
	eventType, err := getEventType(runObject)
	if err != nil {
		return nil, err
	}
	if eventType == nil {
		return nil, errors.New("no matching event type found")
	}
	event.SetType(eventType.String())

	tektonCloudEventData, err := newTektonCloudEventData(ctx, runObject)
	if err != nil {
		return nil, err
	}

	if err := event.SetData(cloudevents.ApplicationJSON, tektonCloudEventData); err != nil {
		return nil, err
	}
	return &event, nil
}

func getEventType(runObject objectWithCondition) (*TektonEventType, error) {
	var eventType TektonEventType
	c := runObject.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
	if c == nil {
		// When the `Run` is created, it may not have any condition until it's
		// picked up by the `Run` reconciler. In that case we consider the run
		// as started. In all other cases, conditions have to be initialised
		switch runObject.(type) {
		case *v1beta1.CustomRun:
			eventType = CustomRunStartedEventV1
			return &eventType, nil
		default:
			return nil, fmt.Errorf("no condition for ConditionSucceeded in %T", runObject)
		}
	}
	switch {
	case c.IsUnknown():
		switch runObject.(type) {
		case *v1beta1.TaskRun:
			switch c.Reason {
			case v1beta1.TaskRunReasonStarted.String():
				eventType = TaskRunStartedEventV1
			case v1beta1.TaskRunReasonRunning.String():
				eventType = TaskRunRunningEventV1
			default:
				eventType = TaskRunUnknownEventV1
			}
		case *v1.TaskRun:
			switch c.Reason {
			case v1.TaskRunReasonStarted.String():
				eventType = TaskRunStartedEventV1
			case v1.TaskRunReasonRunning.String():
				eventType = TaskRunRunningEventV1
			default:
				eventType = TaskRunUnknownEventV1
			}
		case *v1beta1.PipelineRun:
			switch c.Reason {
			case v1beta1.PipelineRunReasonStarted.String():
				eventType = PipelineRunStartedEventV1
			case v1beta1.PipelineRunReasonRunning.String():
				eventType = PipelineRunRunningEventV1
			default:
				eventType = PipelineRunUnknownEventV1
			}
		case *v1.PipelineRun:
			switch c.Reason {
			case v1.PipelineRunReasonStarted.String():
				eventType = PipelineRunStartedEventV1
			case v1.PipelineRunReasonRunning.String():
				eventType = PipelineRunRunningEventV1
			default:
				eventType = PipelineRunUnknownEventV1
			}

		case *v1beta1.CustomRun:
			// CustomRun controller have the freedom of setting reasons as they wish
			// so we cannot make many assumptions here. If a condition is set
			// to unknown (not finished), we sent the running event
			eventType = CustomRunRunningEventV1
		}
	case c.IsFalse():
		switch runObject.(type) {
		case *v1.TaskRun:
			eventType = TaskRunFailedEventV1
		case *v1.PipelineRun:
			eventType = PipelineRunFailedEventV1
		case *v1beta1.TaskRun:
			eventType = TaskRunFailedEventV1
		case *v1beta1.PipelineRun:
			eventType = PipelineRunFailedEventV1
		case *v1beta1.CustomRun:
			eventType = CustomRunFailedEventV1
		}
	case c.IsTrue():
		switch runObject.(type) {
		case *v1beta1.TaskRun:
			eventType = TaskRunSuccessfulEventV1
		case *v1beta1.PipelineRun:
			eventType = PipelineRunSuccessfulEventV1
		case *v1.TaskRun:
			eventType = TaskRunSuccessfulEventV1
		case *v1.PipelineRun:
			eventType = PipelineRunSuccessfulEventV1
		case *v1beta1.CustomRun:
			eventType = CustomRunSuccessfulEventV1
		}
	default:
		return nil, fmt.Errorf("unknown condition for in %T.Status %s", runObject, c.Status)
	}
	return &eventType, nil
}

// GetCloudEventDeliveryCompareOptions returns compare options to sort
// and compare a list of CloudEventDelivery
func GetCloudEventDeliveryCompareOptions() []cmp.Option {
	// Setup cmp options
	cloudDeliveryStateCompare := func(x, y v1beta1.CloudEventDeliveryState) bool {
		return cmp.Equal(x.Condition, y.Condition) && cmp.Equal(x.RetryCount, y.RetryCount)
	}
	less := func(x, y v1beta1.CloudEventDelivery) bool {
		return strings.Compare(x.Target, y.Target) < 0 || (strings.Compare(x.Target, y.Target) == 0 && x.Status.SentAt.Before(y.Status.SentAt))
	}
	return []cmp.Option{
		cmpopts.SortSlices(less),
		cmp.Comparer(func(x, y v1beta1.CloudEventDelivery) bool {
			return (strings.Compare(x.Target, y.Target) == 0) && cloudDeliveryStateCompare(x.Status, y.Status)
		}),
	}
}
