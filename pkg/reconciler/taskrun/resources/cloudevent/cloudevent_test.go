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
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-contrib/pkg/kncloudevents"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

const (
	defaultSinkURI                = "http://sink"
	invalidSinkURI                = "invalid_URI"
	defaultEventID                = "event1234"
	defaultEventSourceURI         = "/taskrun/1234"
	invalidEventSourceURI         = "htt%23p://_invalid_URI##"
	defaultEventType              = TektonTaskRunUnknownV1
	taskRunName                   = "faketaskrunname"
	invalidConditionSuccessStatus = "foobar"
)

var (
	defaultRawData             = []byte(`{"metadata": {"name":"faketaskrun"}}`)
	nilEventType               TektonEventType
	defaultCloudEventClient, _ = kncloudevents.NewDefaultClient()
	happyClientBehaviour       = FakeClientBehaviour{SendSuccessfully: true}
	failingClientBehaviour     = FakeClientBehaviour{SendSuccessfully: false}
)

func TestSendCloudEvent(t *testing.T) {
	for _, c := range []struct {
		desc             string
		sinkURI          string
		eventSourceURI   string
		cloudEventClient CEClient
		wantErr          bool
		errRegexp        string
	}{{
		desc:             "send a cloud event when all inputs are valid",
		sinkURI:          defaultSinkURI,
		eventSourceURI:   defaultEventSourceURI,
		cloudEventClient: NewFakeClient(&happyClientBehaviour),
		wantErr:          false,
		errRegexp:        "",
	}, {
		desc:             "send a cloud event with invalid sink URI",
		sinkURI:          invalidSinkURI,
		eventSourceURI:   defaultEventSourceURI,
		cloudEventClient: defaultCloudEventClient,
		wantErr:          true,
		errRegexp:        fmt.Sprintf("%s: unsupported protocol scheme", invalidSinkURI),
	}, {
		desc:             "send a cloud event, fail to send",
		sinkURI:          defaultSinkURI,
		eventSourceURI:   defaultEventSourceURI,
		cloudEventClient: NewFakeClient(&failingClientBehaviour),
		wantErr:          true,
		errRegexp:        fmt.Sprintf("%s had to fail", defaultEventID),
	}, {
		desc:             "send a cloud event with invalid source URI",
		sinkURI:          defaultSinkURI,
		eventSourceURI:   invalidEventSourceURI,
		cloudEventClient: defaultCloudEventClient,
		wantErr:          true,
		errRegexp:        fmt.Sprintf("invalid eventSourceURI: %s", invalidEventSourceURI),
	}} {
		t.Run(c.desc, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "")
			names.TestingSeed()
			_, err := SendCloudEvent(c.sinkURI, defaultEventID, c.eventSourceURI, defaultRawData, defaultEventType, logger, c.cloudEventClient)
			if c.wantErr != (err != nil) {
				if c.wantErr {
					t.Fatalf("I expected an error but I got nil")
				} else {
					t.Fatalf("I did not expect an error but I got %s", err)
				}
			} else {
				if c.wantErr {
					match, _ := regexp.Match(c.errRegexp, []byte(err.Error()))
					if !match {
						t.Fatalf("I expected an error like %s, but I got %s", c.errRegexp, err)
					}
				}
			}
		})
	}
}

func getTaskRunByCondition(status corev1.ConditionStatus) *v1alpha1.TaskRun {
	return &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskRunName,
			Namespace: "marshmallow",
			SelfLink:  defaultEventSourceURI,
		},
		Spec: v1alpha1.TaskRunSpec{},
		Status: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: status,
				}},
			},
		},
	}
}

func TestSendTaskRunCloudEvent(t *testing.T) {
	for _, c := range []struct {
		desc          string
		taskRun       *v1alpha1.TaskRun
		wantEventType TektonEventType
	}{{
		desc:          "send a cloud event with unknown status taskrun",
		taskRun:       getTaskRunByCondition(corev1.ConditionUnknown),
		wantEventType: TektonTaskRunUnknownV1,
	}, {
		desc:          "send a cloud event with successful status taskrun",
		taskRun:       getTaskRunByCondition(corev1.ConditionTrue),
		wantEventType: TektonTaskRunSuccessfulV1,
	}, {
		desc:          "send a cloud event with unknown status taskrun",
		taskRun:       getTaskRunByCondition(corev1.ConditionFalse),
		wantEventType: TektonTaskRunFailedV1,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "")
			names.TestingSeed()
			event, err := SendTaskRunCloudEvent(defaultSinkURI, c.taskRun, logger, NewFakeClient(&happyClientBehaviour))
			if err != nil {
				t.Fatalf("I did not expect an error but I got %s", err)
			} else {
				wantEventID := taskRunName
				if diff := cmp.Diff(wantEventID, event.Context.GetID()); diff != "" {
					t.Errorf("Wrong Event ID (-want +got) = %s", diff)
				}
				gotEventType := event.Context.GetType()
				if diff := cmp.Diff(string(c.wantEventType), gotEventType); diff != "" {
					t.Errorf("Wrong Event Type (-want +got) = %s", diff)
				}
				wantData, _ := json.Marshal(NewTektonCloudEventData(c.taskRun))
				gotData, err := event.DataBytes()
				if err != nil {
					t.Fatalf("Could not get data from event %v: %v", event, err)
				}
				if diff := cmp.Diff(wantData, gotData); diff != "" {
					t.Errorf("Wrong Event data (-want +got) = %s", diff)
				}
			}
		})
	}
}

func TestSendTaskRunCloudEventErrors(t *testing.T) {
	for _, c := range []struct {
		desc          string
		taskRun       *v1alpha1.TaskRun
		wantEventType TektonEventType
		errRegexp     string
	}{{
		desc:          "send a cloud event with a nil taskrun",
		taskRun:       nil,
		wantEventType: nilEventType,
		errRegexp:     "Cannot send an event for an empty TaskRun",
	}, {
		desc:          "send a cloud event with invalid status taskrun",
		taskRun:       getTaskRunByCondition(invalidConditionSuccessStatus),
		wantEventType: nilEventType,
		errRegexp:     fmt.Sprintf("unknown condition for in TaskRun.Status %s", invalidConditionSuccessStatus),
	}} {
		t.Run(c.desc, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "")
			names.TestingSeed()
			_, err := SendTaskRunCloudEvent(defaultSinkURI, c.taskRun, logger, NewFakeClient(&happyClientBehaviour))
			if err == nil {
				t.Fatalf("I expected an error but I got nil")
			} else {
				match, _ := regexp.Match(c.errRegexp, []byte(err.Error()))
				if !match {
					t.Fatalf("I expected an error like %s, but I got %s", c.errRegexp, err)
				}
			}
		})
	}
}
