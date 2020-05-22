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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// TektonEventType holds the types of cloud events sent by Tekton
type TektonEventType string

const (
	// TektonTaskRunStartedV1 is sent for TaskRuns with "ConditionSucceeded" "Unknown"
	// the first time they are picked up by the reconciler
	TektonTaskRunStartedV1 TektonEventType = "dev.tekton.event.taskrun.started.v1"
	// TektonTaskRunRunningV1 is sent for TaskRuns with "ConditionSucceeded" "Unknown"
	// once the TaskRun is validated and Pod created
	TektonTaskRunRunningV1 TektonEventType = "dev.tekton.event.taskrun.running.v1"
	// TektonTaskRunUnknownV1 is sent for TaskRuns with "ConditionSucceeded" "Unknown"
	// It can be used as a confirmation that the TaskRun is still running.
	TektonTaskRunUnknownV1 TektonEventType = "dev.tekton.event.taskrun.unknown.v1"
	// TektonTaskRunSuccessfulV1 is sent for TaskRuns with "ConditionSucceeded" "True"
	TektonTaskRunSuccessfulV1 TektonEventType = "dev.tekton.event.taskrun.successful.v1"
	// TektonTaskRunFailedV1 is sent for TaskRuns with "ConditionSucceeded" "False"
	TektonTaskRunFailedV1 TektonEventType = "dev.tekton.event.taskrun.failed.v1"
	// TektonPipelineRunStartedV1 is sent for PipelineRuns with "ConditionSucceeded" "Unknown"
	// the first time they are picked up by the reconciler
	TektonPipelineRunStartedV1 TektonEventType = "dev.tekton.event.pipelinerun.started.v1"
	// TektonPipelineRunRunningV1 is sent for PipelineRuns with "ConditionSucceeded" "Unknown"
	// once the PipelineRun is validated and Pod created
	TektonPipelineRunRunningV1 TektonEventType = "dev.tekton.event.pipelinerun.running.v1"
	// TektonPipelineRunUnknownV1 is sent for PipelineRuns with "ConditionSucceeded" "Unknown"
	// It can be used as a confirmation that the PipelineRun is still running.
	TektonPipelineRunUnknownV1 TektonEventType = "dev.tekton.event.pipelinerun.unknown.v1"
	// TektonPipelineRunSuccessfulV1 is sent for PipelineRuns with "ConditionSucceeded" "True"
	TektonPipelineRunSuccessfulV1 TektonEventType = "dev.tekton.event.pipelinerun.successful.v1"
	// TektonPipelineRunFailedV1 is sent for PipelineRuns with "ConditionSucceeded" "False"
	TektonPipelineRunFailedV1 TektonEventType = "dev.tekton.event.pipelinerun.failed.v1"
)

func (t TektonEventType) String() string {
	return string(t)
}

// CEClient matches the `Client` interface from github.com/cloudevents/sdk-go/v2/cloudevents
type CEClient cloudevents.Client

// TektonCloudEventData type is used to marshal and unmarshal the payload of
// a Tekton cloud event. It can include a PipelineRun or a PipelineRun
type TektonCloudEventData struct {
	TaskRun     *v1beta1.TaskRun     `json:"taskRun,omitempty"`
	PipelineRun *v1beta1.PipelineRun `json:"pipelineRun,omitempty"`
}

// NewTektonCloudEventData returns a new instance of NewTektonCloudEventData
func NewTektonCloudEventData(runObject v1beta1.RunsToCompletion) TektonCloudEventData {
	tektonCloudEventData := TektonCloudEventData{}
	switch runObject.(type) {
	case *v1beta1.TaskRun:
		tektonCloudEventData.TaskRun = runObject.(*v1beta1.TaskRun)
	case *v1beta1.PipelineRun:
		tektonCloudEventData.PipelineRun = runObject.(*v1beta1.PipelineRun)
	}
	return tektonCloudEventData
}

// EventForRunsToCompletion creates a new event based for a RunsToCompletion,
// or return an error if not possible.
func EventForRunsToCompletion(runObject v1beta1.RunsToCompletion) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetSubject(runObject.GetObjectMeta().Name)
	event.SetSource(runObject.GetObjectMeta().SelfLink) // TODO: SelfLink is deprecated https://github.com/tektoncd/pipeline/issues/2676
	eventType, err := getEventType(runObject)
	if err != nil {
		return nil, err
	}
	event.SetType(eventType.String())

	if err := event.SetData(cloudevents.ApplicationJSON, NewTektonCloudEventData(runObject)); err != nil {
		return nil, err
	}
	return &event, nil
}

// EventForTaskRun will create a new event based on a TaskRun,
// or return an error if not possible.
func EventForTaskRun(taskRun *v1beta1.TaskRun) (*cloudevents.Event, error) {
	// Check if the TaskRun is defined
	if taskRun == nil {
		return nil, errors.New("Cannot send an event for an empty TaskRun")
	}
	return EventForRunsToCompletion(taskRun)
}

// EventForPipelineRun will create a new event based on a TaskRun,
// or return an error if not possible.
func EventForPipelineRun(pipelineRun *v1beta1.PipelineRun) (*cloudevents.Event, error) {
	// Check if the TaskRun is defined
	if pipelineRun == nil {
		return nil, errors.New("Cannot send an event for an empty PipelineRun")
	}
	return EventForRunsToCompletion(pipelineRun)
}

func getEventType(runObject v1beta1.RunsToCompletion) (*TektonEventType, error) {
	c := runObject.GetStatus().GetCondition(apis.ConditionSucceeded)
	t := runObject.GetTypeMeta()
	var eventType TektonEventType
	switch {
	case c.IsUnknown():
		// TBD We should have different event types here, e.g. started, running
		// That requires having either knowledge about the previous condition or
		// TaskRun and PipelineRun using dedicated "Reasons" or "Conditions"
		switch t.Kind {
		case "TaskRun":
			eventType = TektonTaskRunUnknownV1
		case "PipelineRun":
			eventType = TektonPipelineRunUnknownV1
		}
	case c.IsFalse():
		switch t.Kind {
		case "TaskRun":
			eventType = TektonTaskRunFailedV1
		case "PipelineRun":
			eventType = TektonPipelineRunFailedV1
		}
	case c.IsTrue():
		switch t.Kind {
		case "TaskRun":
			eventType = TektonTaskRunSuccessfulV1
		case "PipelineRun":
			eventType = TektonPipelineRunSuccessfulV1
		}
	default:
		return nil, fmt.Errorf("unknown condition for in TaskRun.Status %s", c.Status)
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
