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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestCloudEventDeliveryFromTargets(t *testing.T) {
	tests := []struct {
		name            string
		targets         []string
		wantCloudEvents []v1alpha1.CloudEventDelivery
	}{{
		name:            "testWithNilTarget",
		targets:         nil,
		wantCloudEvents: nil,
	}, {
		name:            "testWithEmptyListTarget",
		targets:         make([]string, 0),
		wantCloudEvents: nil,
	}, {
		name:    "testWithTwoTargets",
		targets: []string{"target1", "target2"},
		wantCloudEvents: []v1alpha1.CloudEventDelivery{
			{
				Target: "target1",
				Status: v1alpha1.CloudEventDeliveryState{
					Condition:  v1alpha1.CloudEventConditionUnknown,
					SentAt:     nil,
					Error:      "",
					RetryCount: 0,
				},
			},
			{
				Target: "target2",
				Status: v1alpha1.CloudEventDeliveryState{
					Condition:  v1alpha1.CloudEventConditionUnknown,
					SentAt:     nil,
					Error:      "",
					RetryCount: 0,
				},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCloudEvents := cloudEventDeliveryFromTargets(tc.targets)
			if diff := cmp.Diff(tc.wantCloudEvents, gotCloudEvents); diff != "" {
				t.Errorf("Wrong Cloud Events (-want +got) = %s", diff)
			}
		})
	}
}

func TestSendCloudEvents(t *testing.T) {
	tests := []struct {
		name        string
		taskRun     *v1alpha1.TaskRun
		wantTaskRun *v1alpha1.TaskRun
	}{{
		name: "testWithMultipleMixedCloudEvents",
		taskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery", "foo",
			tb.TaskRunSelfLink("/task/1234"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.TaskRunCloudEvent("http//notattemptedunknown", "", 0, v1alpha1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//notattemptedfailed", "somehow", 0, v1alpha1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//notattemptedsucceeded", "", 0, v1alpha1.CloudEventConditionSent),
				tb.TaskRunCloudEvent("http//attemptedunknown", "", 1, v1alpha1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//attemptedfailed", "iknewit", 1, v1alpha1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//attemptedsucceeded", "", 1, v1alpha1.CloudEventConditionSent),
			),
		),
		wantTaskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery", "foo",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.TaskRunCloudEvent("http//notattemptedunknown", "", 1, v1alpha1.CloudEventConditionSent),
				tb.TaskRunCloudEvent("http//notattemptedfailed", "somehow", 0, v1alpha1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//notattemptedsucceeded", "", 0, v1alpha1.CloudEventConditionSent),
				tb.TaskRunCloudEvent("http//attemptedunknown", "", 1, v1alpha1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//attemptedfailed", "iknewit", 1, v1alpha1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//attemptedsucceeded", "", 1, v1alpha1.CloudEventConditionSent),
			),
		),
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "")
			successfulBehaviour := FakeClientBehaviour{
				SendSuccessfully: true,
			}
			err := SendCloudEvents(tc.taskRun, NewFakeClient(&successfulBehaviour), logger)
			if err != nil {
				t.Fatalf("Unexpected error sending cloud events: %v", err)
			}
			opts := GetCloudEventDeliveryCompareOptions()
			if diff := cmp.Diff(tc.wantTaskRun.Status, tc.taskRun.Status, opts...); diff != "" {
				t.Errorf("Wrong Cloud Events Status (-want +got) = %s", diff)
			}
		})
	}
}

func TestSendCloudEventsErrors(t *testing.T) {
	tests := []struct {
		name        string
		taskRun     *v1alpha1.TaskRun
		wantTaskRun *v1alpha1.TaskRun
	}{{
		name: "testWithMultipleMixedCloudEvents",
		taskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery", "foo",
			tb.TaskRunSelfLink("/task/1234"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.TaskRunCloudEvent("http//sink1", "", 0, v1alpha1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//sink2", "", 0, v1alpha1.CloudEventConditionUnknown),
			),
		),
		wantTaskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery", "foo",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				// Error message is not checked in the Diff below
				tb.TaskRunCloudEvent("http//sink1", "", 1, v1alpha1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//sink2", "", 1, v1alpha1.CloudEventConditionFailed),
			),
		),
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "")
			unsuccessfulBehaviour := FakeClientBehaviour{
				SendSuccessfully: false,
			}
			err := SendCloudEvents(tc.taskRun, NewFakeClient(&unsuccessfulBehaviour), logger)
			if err == nil {
				t.Fatalf("Unexpected success sending cloud events: %v", err)
			}
			opts := GetCloudEventDeliveryCompareOptions()
			if diff := cmp.Diff(tc.wantTaskRun.Status, tc.taskRun.Status, opts...); diff != "" {
				t.Errorf("Wrong Cloud Events Status (-want +got) = %s", diff)
			}
		})
	}
}

func TestInitializeCloudEvents(t *testing.T) {
	tests := []struct {
		name              string
		taskRun           *v1alpha1.TaskRun
		pipelineResources []*v1alpha1.PipelineResource
		wantTaskRun       *v1alpha1.TaskRun
	}{{
		name: "testWithMultipleMixedResources",
		taskRun: tb.TaskRun("test-taskrun-multiple-mixed-resources", "foo",
			tb.TaskRunSelfLink("/task/1234"), tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
				tb.TaskRunOutputs(
					tb.TaskRunOutputsResource("ce1", tb.TaskResourceBindingRef("ce1")),
					tb.TaskRunOutputsResource("git", tb.TaskResourceBindingRef("git")),
					tb.TaskRunOutputsResource("ce2", tb.TaskResourceBindingRef("ce2")),
				),
			),
		),
		pipelineResources: []*v1alpha1.PipelineResource{
			tb.PipelineResource("ce1", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeCloudEvent,
				tb.PipelineResourceSpecParam("TargetURI", "http://foosink"),
			)),
			tb.PipelineResource("ce2", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeCloudEvent,
				tb.PipelineResourceSpecParam("TargetURI", "http://barsink"),
			)),
			tb.PipelineResource("git", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeGit,
				tb.PipelineResourceSpecParam("URL", "http://git.fake"),
				tb.PipelineResourceSpecParam("Revision", "abcd"),
			)),
		},
		wantTaskRun: tb.TaskRun("test-taskrun-multiple-mixed-resources", "foo",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.TaskRunCloudEvent("http://barsink", "", 0, v1alpha1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http://foosink", "", 0, v1alpha1.CloudEventConditionUnknown),
			),
		),
	}, {
		name: "testWithNoCloudEventResources",
		taskRun: tb.TaskRun("test-taskrun-no-cloudevent-resources", "foo",
			tb.TaskRunSelfLink("/task/1234"), tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
				tb.TaskRunOutputs(
					tb.TaskRunOutputsResource("git", tb.TaskResourceBindingRef("git")),
				),
			),
		),
		pipelineResources: []*v1alpha1.PipelineResource{
			tb.PipelineResource("git", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeGit,
				tb.PipelineResourceSpecParam("URL", "http://git.fake"),
				tb.PipelineResourceSpecParam("Revision", "abcd"),
			)),
		},
		wantTaskRun: tb.TaskRun("test-taskrun-no-cloudevent-resources", "foo",
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(),
		),
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			InitializeCloudEvents(tc.taskRun, tc.pipelineResources)
			opts := GetCloudEventDeliveryCompareOptions()
			if diff := cmp.Diff(tc.wantTaskRun.Status, tc.taskRun.Status, opts...); diff != "" {
				t.Errorf("Wrong Cloud Events Status (-want +got) = %s", diff)
			}
		})
	}
}
