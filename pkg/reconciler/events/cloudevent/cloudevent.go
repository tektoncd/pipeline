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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
	// RunStartedEventV1 is sent for Runs with "ConditionSucceeded" "Unknown"
	// the first time they are picked up by the reconciler
	RunStartedEventV1 TektonEventType = "dev.tekton.event.run.started.v1"
	// RunRunningEventV1 is sent for Runs with "ConditionSucceeded" "Unknown"
	// once the Run is validated and Pod created
	RunRunningEventV1 TektonEventType = "dev.tekton.event.run.running.v1"
	// RunSuccessfulEventV1 is sent for Runs with "ConditionSucceeded" "True"
	RunSuccessfulEventV1 TektonEventType = "dev.tekton.event.run.successful.v1"
	// RunFailedEventV1 is sent for Runs with "ConditionSucceeded" "False"
	RunFailedEventV1 TektonEventType = "dev.tekton.event.run.failed.v1"
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
	Run         *v1alpha1.Run        `json:"run,omitempty"`
	CustomRun   *v1beta1.CustomRun   `json:"customRun,omitempty"`
}

// newTektonCloudEventData returns a new instance of TektonCloudEventData
func newTektonCloudEventData(runObject objectWithCondition) TektonCloudEventData {
	tektonCloudEventData := TektonCloudEventData{}
	switch v := runObject.(type) {
	case *v1beta1.TaskRun:
		tektonCloudEventData.TaskRun = v
	case *v1beta1.PipelineRun:
		tektonCloudEventData.PipelineRun = v
	case *v1alpha1.Run:
		tektonCloudEventData.Run = v
	case *v1beta1.CustomRun:
		tektonCloudEventData.CustomRun = v
	}
	return tektonCloudEventData
}

// eventForObjectWithCondition creates a new event based for a objectWithCondition,
// or return an error if not possible.
func eventForObjectWithCondition(runObject objectWithCondition) (*cloudevents.Event, error) {
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
		return nil, errors.New("No matching event type found")
	}
	event.SetType(eventType.String())

	if err := event.SetData(cloudevents.ApplicationJSON, newTektonCloudEventData(runObject)); err != nil {
		return nil, err
	}
	return &event, nil
}

// eventForTaskRun will create a new event based on a TaskRun,
// or return an error if not possible.
func eventForTaskRun(taskRun *v1beta1.TaskRun) (*cloudevents.Event, error) {
	// Check if the TaskRun is defined
	if taskRun == nil {
		return nil, errors.New("Cannot send an event for an empty TaskRun")
	}
	return eventForObjectWithCondition(taskRun)
}

// eventForPipelineRun will create a new event based on a PipelineRun,
// or return an error if not possible.
func eventForPipelineRun(pipelineRun *v1beta1.PipelineRun) (*cloudevents.Event, error) {
	// Check if the TaskRun is defined
	if pipelineRun == nil {
		return nil, errors.New("Cannot send an event for an empty PipelineRun")
	}
	return eventForObjectWithCondition(pipelineRun)
}

// eventForRun will create a new event based on a Run, or return an error if
// not possible.
func eventForRun(run *v1alpha1.Run) (*cloudevents.Event, error) {
	// Check if the Run is defined
	if run == nil {
		return nil, errors.New("Cannot send an event for an empty Run")
	}
	return eventForObjectWithCondition(run)
}

// eventForCustomRun will create a new event based on a CustomRun, or return an error if
// not possible.
func eventForCustomRun(customRun *v1beta1.CustomRun) (*cloudevents.Event, error) {
	// Check if the CustomRun is defined
	if customRun == nil {
		return nil, errors.New("Cannot send an event for an empty CustomRun")
	}
	return eventForObjectWithCondition(customRun)
}

func getEventType(runObject objectWithCondition) (*TektonEventType, error) {
	var eventType TektonEventType
	c := runObject.GetStatusCondition().GetCondition(apis.ConditionSucceeded)
	if c == nil {
		// When the `Run` is created, it may not have any condition until it's
		// picked up by the `Run` reconciler. In that case we consider the run
		// as started. In all other cases, conditions have to be initialised
		switch runObject.(type) {
		case *v1alpha1.Run:
			eventType = RunStartedEventV1
			return &eventType, nil
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
		case *v1beta1.PipelineRun:
			switch c.Reason {
			case v1beta1.PipelineRunReasonStarted.String():
				eventType = PipelineRunStartedEventV1
			case v1beta1.PipelineRunReasonRunning.String():
				eventType = PipelineRunRunningEventV1
			default:
				eventType = PipelineRunUnknownEventV1
			}
		case *v1alpha1.Run:
			// Run controller have the freedom of setting reasons as they wish
			// so we cannot make many assumptions here. If a condition is set
			// to unknown (not finished), we sent the running event
			eventType = RunRunningEventV1
		case *v1beta1.CustomRun:
			// CustomRun controller have the freedom of setting reasons as they wish
			// so we cannot make many assumptions here. If a condition is set
			// to unknown (not finished), we sent the running event
			eventType = CustomRunRunningEventV1
		}
	case c.IsFalse():
		switch runObject.(type) {
		case *v1beta1.TaskRun:
			eventType = TaskRunFailedEventV1
		case *v1beta1.PipelineRun:
			eventType = PipelineRunFailedEventV1
		case *v1alpha1.Run:
			eventType = RunFailedEventV1
		case *v1beta1.CustomRun:
			eventType = CustomRunFailedEventV1
		}
	case c.IsTrue():
		switch runObject.(type) {
		case *v1beta1.TaskRun:
			eventType = TaskRunSuccessfulEventV1
		case *v1beta1.PipelineRun:
			eventType = PipelineRunSuccessfulEventV1
		case *v1alpha1.Run:
			eventType = RunSuccessfulEventV1
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
