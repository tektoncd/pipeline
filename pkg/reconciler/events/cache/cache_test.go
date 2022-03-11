/*
Copyright 2022 The Tekton Authors

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

package cache

import (
	"net/url"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	lru "github.com/hashicorp/golang-lru"

	"github.com/cloudevents/sdk-go/v2/event"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func strptr(s string) *string { return &s }

func getEventData(run interface{}) map[string]interface{} {
	cloudEventData := map[string]interface{}{}
	if v, ok := run.(*v1alpha1.Run); ok {
		cloudEventData["run"] = v
	}
	return cloudEventData
}

func getEventToTest(eventtype string, run interface{}) *event.Event {
	e := event.Event{
		Context: event.EventContextV1{
			Type:    eventtype,
			Source:  cetypes.URIRef{URL: url.URL{Path: "/foo/bar/source"}},
			ID:      "test-event",
			Time:    &cetypes.Timestamp{Time: time.Now()},
			Subject: strptr("topic"),
		}.AsV1(),
	}
	if err := e.SetData(cloudevents.ApplicationJSON, getEventData(run)); err != nil {
		panic(err)
	}
	return &e
}

func getRunByMeta(name string, namespace string) *v1alpha1.Run {
	return &v1alpha1.Run{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Run",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec:   v1alpha1.RunSpec{},
		Status: v1alpha1.RunStatus{},
	}
}

// TestEventsKey verifies that keys are extracted correctly from events
func TestEventsKey(t *testing.T) {
	testcases := []struct {
		name      string
		eventtype string
		run       interface{}
		wantKey   string
		wantErr   bool
	}{{
		name:      "run event",
		eventtype: "my.test.run.event",
		run:       getRunByMeta("myrun", "mynamespace"),
		wantKey:   "my.test.run.event/run/mynamespace/myrun",
		wantErr:   false,
	}, {
		name:      "run event missing data",
		eventtype: "my.test.run.event",
		run:       nil,
		wantKey:   "",
		wantErr:   true,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			gotEvent := getEventToTest(tc.eventtype, tc.run)
			gotKey, err := EventKey(gotEvent)
			if err != nil {
				if !tc.wantErr {
					t.Fatalf("Expecting an error, got none")
				}
			}
			if d := cmp.Diff(tc.wantKey, gotKey); d != "" {
				t.Errorf("Wrong Event key %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestAddCheckEvent(t *testing.T) {
	run := getRunByMeta("arun", "anamespace")
	runb := getRunByMeta("arun", "bnamespace")
	baseEvent := getEventToTest("some.event.type", run)

	testcases := []struct {
		name        string
		firstEvent  *event.Event
		secondEvent *event.Event
		wantFound   bool
	}{{
		name:        "identical events",
		firstEvent:  baseEvent,
		secondEvent: baseEvent,
		wantFound:   true,
	}, {
		name:        "new timestamp event",
		firstEvent:  baseEvent,
		secondEvent: getEventToTest("some.event.type", run),
		wantFound:   true,
	}, {
		name:        "different namespace",
		firstEvent:  baseEvent,
		secondEvent: getEventToTest("some.event.type", runb),
		wantFound:   false,
	}, {
		name:        "different event type",
		firstEvent:  baseEvent,
		secondEvent: getEventToTest("some.other.event.type", run),
		wantFound:   false,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			testCache, _ := lru.New(10)
			AddEventSentToCache(testCache, tc.firstEvent)
			found, _ := IsCloudEventSent(testCache, tc.secondEvent)
			if d := cmp.Diff(tc.wantFound, found); d != "" {
				t.Errorf("Cache check failure %s", diff.PrintWantGot(d))
			}
		})
	}
}
