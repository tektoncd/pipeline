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
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestCloudEventDeliveryFromTargets(t *testing.T) {
	tests := []struct {
		name            string
		targets         []string
		wantCloudEvents []v1beta1.CloudEventDelivery
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
		wantCloudEvents: []v1beta1.CloudEventDelivery{
			{
				Target: "target1",
				Status: v1beta1.CloudEventDeliveryState{
					Condition:  v1beta1.CloudEventConditionUnknown,
					SentAt:     nil,
					Error:      "",
					RetryCount: 0,
				},
			},
			{
				Target: "target2",
				Status: v1beta1.CloudEventDeliveryState{
					Condition:  v1beta1.CloudEventConditionUnknown,
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
			if d := cmp.Diff(tc.wantCloudEvents, gotCloudEvents); d != "" {
				t.Errorf("Wrong Cloud Events %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestSendCloudEvents(t *testing.T) {
	tests := []struct {
		name        string
		taskRun     *v1beta1.TaskRun
		wantTaskRun *v1beta1.TaskRun
	}{{
		name: "testWithMultipleMixedCloudEvents",
		taskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSelfLink("/task/1234"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: "somethingelse",
				}),
				tb.TaskRunCloudEvent("http//notattemptedunknown", "", 0, v1beta1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//notattemptedfailed", "somehow", 0, v1beta1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//notattemptedsucceeded", "", 0, v1beta1.CloudEventConditionSent),
				tb.TaskRunCloudEvent("http//attemptedunknown", "", 1, v1beta1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//attemptedfailed", "iknewit", 1, v1beta1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//attemptedsucceeded", "", 1, v1beta1.CloudEventConditionSent),
			),
		),
		wantTaskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: "somethingelse",
				}),
				tb.TaskRunCloudEvent("http//notattemptedunknown", "", 1, v1beta1.CloudEventConditionSent),
				tb.TaskRunCloudEvent("http//notattemptedfailed", "somehow", 0, v1beta1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//notattemptedsucceeded", "", 0, v1beta1.CloudEventConditionSent),
				tb.TaskRunCloudEvent("http//attemptedunknown", "", 1, v1beta1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//attemptedfailed", "iknewit", 1, v1beta1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//attemptedsucceeded", "", 1, v1beta1.CloudEventConditionSent),
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
			if d := cmp.Diff(tc.wantTaskRun.Status, tc.taskRun.Status, opts...); d != "" {
				t.Errorf("Wrong Cloud Events Status %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestSendCloudEventsErrors(t *testing.T) {
	tests := []struct {
		name        string
		taskRun     *v1beta1.TaskRun
		wantTaskRun *v1beta1.TaskRun
	}{{
		name: "testWithMultipleMixedCloudEvents",
		taskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSelfLink("/task/1234"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.TaskRunCloudEvent("http//sink1", "", 0, v1beta1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http//sink2", "", 0, v1beta1.CloudEventConditionUnknown),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: "somethingelse",
				}),
			),
		),
		wantTaskRun: tb.TaskRun("test-taskrun-multiple-cloudeventdelivery",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				// Error message is not checked in the Diff below
				tb.TaskRunCloudEvent("http//sink1", "", 1, v1beta1.CloudEventConditionFailed),
				tb.TaskRunCloudEvent("http//sink2", "", 1, v1beta1.CloudEventConditionFailed),
				tb.StatusCondition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: "somethingelse",
				}),
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
			if d := cmp.Diff(tc.wantTaskRun.Status, tc.taskRun.Status, opts...); d != "" {
				t.Errorf("Wrong Cloud Events Status %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestInitializeCloudEvents(t *testing.T) {
	tests := []struct {
		name              string
		taskRun           *v1beta1.TaskRun
		pipelineResources []*resourcev1alpha1.PipelineResource
		wantTaskRun       *v1beta1.TaskRun
	}{{
		name: "testWithMultipleMixedResources",
		taskRun: tb.TaskRun("test-taskrun-multiple-mixed-resources",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSelfLink("/task/1234"), tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
				tb.TaskRunResources(
					tb.TaskRunResourcesOutput("ce1", tb.TaskResourceBindingRef("ce1")),
					tb.TaskRunResourcesOutput("git", tb.TaskResourceBindingRef("git")),
					tb.TaskRunResourcesOutput("ce2", tb.TaskResourceBindingRef("ce2")),
				),
			),
		),
		pipelineResources: []*resourcev1alpha1.PipelineResource{
			tb.PipelineResource("ce1", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
				resourcev1alpha1.PipelineResourceTypeCloudEvent,
				tb.PipelineResourceSpecParam("TargetURI", "http://foosink"),
			)),
			tb.PipelineResource("ce2", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
				resourcev1alpha1.PipelineResourceTypeCloudEvent,
				tb.PipelineResourceSpecParam("TargetURI", "http://barsink"),
			)),
			tb.PipelineResource("git", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
				resourcev1alpha1.PipelineResourceTypeGit,
				tb.PipelineResourceSpecParam("URL", "http://git.fake"),
				tb.PipelineResourceSpecParam("Revision", "abcd"),
			)),
		},
		wantTaskRun: tb.TaskRun("test-taskrun-multiple-mixed-resources",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(
				tb.TaskRunCloudEvent("http://barsink", "", 0, v1beta1.CloudEventConditionUnknown),
				tb.TaskRunCloudEvent("http://foosink", "", 0, v1beta1.CloudEventConditionUnknown),
			),
		),
	}, {
		name: "testWithNoCloudEventResources",
		taskRun: tb.TaskRun("test-taskrun-no-cloudevent-resources",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSelfLink("/task/1234"), tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
				tb.TaskRunResources(
					tb.TaskRunResourcesOutput("git", tb.TaskResourceBindingRef("git")),
				),
			),
		),
		pipelineResources: []*resourcev1alpha1.PipelineResource{
			tb.PipelineResource("git", tb.PipelineResourceNamespace("foo"), tb.PipelineResourceSpec(
				resourcev1alpha1.PipelineResourceTypeGit,
				tb.PipelineResourceSpecParam("URL", "http://git.fake"),
				tb.PipelineResourceSpecParam("Revision", "abcd"),
			)),
		},
		wantTaskRun: tb.TaskRun("test-taskrun-no-cloudevent-resources",
			tb.TaskRunNamespace("foo"),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("fakeTaskName"),
			),
			tb.TaskRunStatus(),
		),
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prMap := map[string]*resourcev1alpha1.PipelineResource{}
			for _, pr := range tc.pipelineResources {
				prMap[pr.Name] = pr
			}
			InitializeCloudEvents(tc.taskRun, prMap)
			opts := GetCloudEventDeliveryCompareOptions()
			if d := cmp.Diff(tc.wantTaskRun.Status, tc.taskRun.Status, opts...); d != "" {
				t.Errorf("Wrong Cloud Events Status %s", diff.PrintWantGot(d))
			}
		})
	}
}
