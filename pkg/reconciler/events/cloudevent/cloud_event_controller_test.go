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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	eventstest "github.com/tektoncd/pipeline/test/events"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	rtesting "knative.dev/pkg/reconciler/testing"
)

var now = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)

type testClock struct{}

func (testClock) Now() time.Time                  { return now }
func (testClock) Since(t time.Time) time.Duration { return now.Sub(t) }

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
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-multiple-cloudeventdelivery",
				Namespace: "foo",
				SelfLink:  "/task/1234",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: "somethingelse",
					}},
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					CloudEvents: []v1beta1.CloudEventDelivery{
						{
							Target: "http//notattemptedunknown",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionUnknown,
							},
						},
						{
							Target: "http//notattemptedfailed",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionFailed,
								Error:     "somehow",
							},
						},
						{
							Target: "http//notattemptedsucceeded",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionSent,
							},
						},
						{
							Target: "http//attemptedunknown",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionUnknown,
								RetryCount: 1,
							},
						},
						{
							Target: "http//attemptedfailed",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionFailed,
								Error:      "iknewit",
								RetryCount: 1,
							},
						},
						{
							Target: "http//attemptedsucceeded",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionSent,
								RetryCount: 1,
							},
						},
					},
				},
			},
		},
		wantTaskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-multiple-cloudeventdelivery",
				Namespace: "foo",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: "somethingelse",
					}},
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					CloudEvents: []v1beta1.CloudEventDelivery{
						{
							Target: "http//notattemptedunknown",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionSent,
								RetryCount: 1,
							},
						},
						{
							Target: "http//notattemptedfailed",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionFailed,
								Error:     "somehow",
							},
						},
						{
							Target: "http//notattemptedsucceeded",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionSent,
							},
						},
						{
							Target: "http//attemptedunknown",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionUnknown,
								RetryCount: 1,
							},
						},
						{
							Target: "http//attemptedfailed",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionFailed,
								Error:      "iknewit",
								RetryCount: 1,
							},
						},
						{
							Target: "http//attemptedsucceeded",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionSent,
								RetryCount: 1,
							},
						},
					},
				},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "")
			successfulBehaviour := FakeClientBehaviour{
				SendSuccessfully: true,
			}
			err := SendCloudEvents(tc.taskRun, newFakeClient(&successfulBehaviour), logger, testClock{})
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
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-multiple-cloudeventdelivery",
				Namespace: "foo",
				SelfLink:  "/task/1234",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: "somethingelse",
					}},
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					CloudEvents: []v1beta1.CloudEventDelivery{
						{
							Target: "http//sink1",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionUnknown,
							},
						},
						{
							Target: "http//sink2",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionUnknown,
							},
						},
					},
				},
			},
		},
		wantTaskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-multiple-cloudeventdelivery",
				Namespace: "foo",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: "somethingelse",
					}},
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					CloudEvents: []v1beta1.CloudEventDelivery{
						{
							Target: "http//sink1",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionFailed,
								RetryCount: 1,
							},
						},
						{
							Target: "http//sink2",
							Status: v1beta1.CloudEventDeliveryState{
								Condition:  v1beta1.CloudEventConditionFailed,
								RetryCount: 1,
							},
						},
					},
				},
			},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "")
			unsuccessfulBehaviour := FakeClientBehaviour{
				SendSuccessfully: false,
			}
			err := SendCloudEvents(tc.taskRun, newFakeClient(&unsuccessfulBehaviour), logger, testClock{})
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
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-multiple-mixed-resources",
				Namespace: "foo",
				SelfLink:  "/task/1234",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{
						{
							PipelineResourceBinding: v1beta1.PipelineResourceBinding{
								Name: "ce1",
								ResourceRef: &v1beta1.PipelineResourceRef{
									Name: "ce1",
								},
							},
						},
						{
							PipelineResourceBinding: v1beta1.PipelineResourceBinding{
								Name: "git",
								ResourceRef: &v1beta1.PipelineResourceRef{
									Name: "git",
								},
							},
						},
						{
							PipelineResourceBinding: v1beta1.PipelineResourceBinding{
								Name: "ce2",
								ResourceRef: &v1beta1.PipelineResourceRef{
									Name: "ce2",
								},
							},
						},
					},
				},
			},
		},
		pipelineResources: []*resourcev1alpha1.PipelineResource{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ce1",
					Namespace: "foo",
				},
				Spec: resourcev1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeCloudEvent,
					Params: []resourcev1alpha1.ResourceParam{{
						Name:  "TargetURI",
						Value: "http://foosink",
					}},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ce2",
					Namespace: "foo",
				},
				Spec: resourcev1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeCloudEvent,
					Params: []resourcev1alpha1.ResourceParam{{
						Name:  "TargetURI",
						Value: "http://barsink",
					}},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git",
					Namespace: "foo",
				},
				Spec: resourcev1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeGit,
					Params: []resourcev1alpha1.ResourceParam{
						{
							Name:  "URL",
							Value: "http://git.fake",
						},
						{
							Name:  "Revision",
							Value: "abcd",
						},
					},
				},
			},
		},
		wantTaskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-multiple-mixed-resources",
				Namespace: "foo",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					CloudEvents: []v1beta1.CloudEventDelivery{
						{
							Target: "http://barsink",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionUnknown,
							},
						},
						{
							Target: "http://foosink",
							Status: v1beta1.CloudEventDeliveryState{
								Condition: v1beta1.CloudEventConditionUnknown,
							},
						},
					},
				},
			},
		},
	}, {
		name: "testWithNoCloudEventResources",
		taskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-no-cloudevent-resources",
				Namespace: "foo",
				SelfLink:  "/task/1234",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "git",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "git",
							},
						},
					}},
				},
			},
		},
		pipelineResources: []*resourcev1alpha1.PipelineResource{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git",
					Namespace: "foo",
				},
				Spec: resourcev1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeGit,
					Params: []resourcev1alpha1.ResourceParam{
						{
							Name:  "URL",
							Value: "http://git.fake",
						},
						{
							Name:  "Revision",
							Value: "abcd",
						},
					},
				},
			},
		},
		wantTaskRun: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-no-cloudevent-resources",
				Namespace: "foo",
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "fakeTaskName",
				},
			},
			Status: v1beta1.TaskRunStatus{},
		},
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

func TestSendCloudEventWithRetries(t *testing.T) {

	objectStatus := duckv1beta1.Status{
		Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		}},
	}

	tests := []struct {
		name            string
		clientBehaviour FakeClientBehaviour
		object          objectWithCondition
		wantCEvents     []string
		wantEvents      []string
	}{{
		name: "test-send-cloud-event-taskrun",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: true,
		},
		object: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				SelfLink: "/taskruns/test1",
			},
			Status: v1beta1.TaskRunStatus{Status: objectStatus},
		},
		wantCEvents: []string{"Context Attributes,"},
		wantEvents:  []string{},
	}, {
		name: "test-send-cloud-event-pipelinerun",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: true,
		},
		object: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				SelfLink: "/pipelineruns/test1",
			},
			Status: v1beta1.PipelineRunStatus{Status: objectStatus},
		},
		wantCEvents: []string{"Context Attributes,"},
		wantEvents:  []string{},
	}, {
		name: "test-send-cloud-event-failed",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: false,
		},
		object: &v1beta1.PipelineRun{
			Status: v1beta1.PipelineRunStatus{Status: objectStatus},
		},
		wantCEvents: []string{},
		wantEvents:  []string{"Warning Cloud Event Failure"},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupFakeContext(t, tc.clientBehaviour, true)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			if err := SendCloudEventWithRetries(ctx, tc.object); err != nil {
				t.Fatalf("Unexpected error sending cloud events: %v", err)
			}
			ceClient := Get(ctx).(FakeClient)
			if err := eventstest.CheckEventsUnordered(t, ceClient.Events, tc.name, tc.wantCEvents); err != nil {
				t.Fatalf(err.Error())
			}
			recorder := controller.GetEventRecorder(ctx).(*record.FakeRecorder)
			if err := eventstest.CheckEventsOrdered(t, recorder.Events, tc.name, tc.wantEvents); err != nil {
				t.Fatalf(err.Error())
			}
		})
	}
}

func TestSendCloudEventWithRetriesInvalid(t *testing.T) {

	tests := []struct {
		name       string
		object     objectWithCondition
		wantCEvent string
		wantEvent  string
	}{{
		name: "test-send-cloud-event-invalid-taskrun",
		object: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{},
		},
		wantCEvent: "Context Attributes,",
		wantEvent:  "",
	}, {
		name: "test-send-cloud-event-pipelinerun",
		object: &v1beta1.PipelineRun{
			Status: v1beta1.PipelineRunStatus{},
		},
		wantCEvent: "Context Attributes,",
		wantEvent:  "",
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupFakeContext(t, FakeClientBehaviour{
				SendSuccessfully: true,
			}, true)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			err := SendCloudEventWithRetries(ctx, tc.object)
			if err == nil {
				t.Fatalf("Expected an error sending cloud events for invalid object, got none")
			}
		})
	}
}

func TestSendCloudEventWithRetriesNoClient(t *testing.T) {

	ctx := setupFakeContext(t, FakeClientBehaviour{}, false)
	err := SendCloudEventWithRetries(ctx, &v1beta1.TaskRun{Status: v1beta1.TaskRunStatus{}})
	if err == nil {
		t.Fatalf("Expected an error sending cloud events with no client in the context, got none")
	}
	if d := cmp.Diff("No cloud events client found in the context", err.Error()); d != "" {
		t.Fatalf("Unexpected error message %s", diff.PrintWantGot(d))
	}
}

func setupFakeContext(t *testing.T, behaviour FakeClientBehaviour, withClient bool) context.Context {
	var ctx context.Context
	ctx, _ = rtesting.SetupFakeContext(t)
	if withClient {
		ctx = WithClient(ctx, &behaviour)
	}
	return ctx
}
