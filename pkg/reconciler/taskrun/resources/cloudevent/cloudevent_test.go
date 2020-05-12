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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

const (
	defaultEventSourceURI = "/taskrun/1234"
	taskRunName           = "faketaskrunname"
)

func getTaskRunByCondition(status corev1.ConditionStatus) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskRunName,
			Namespace: "marshmallow",
			SelfLink:  defaultEventSourceURI,
		},
		Spec: v1beta1.TaskRunSpec{},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: status,
				}},
			},
		},
	}
}

func TestEventForTaskRun(t *testing.T) {
	for _, c := range []struct {
		desc          string
		taskRun       *v1beta1.TaskRun
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
			names.TestingSeed()

			got, err := EventForTaskRun(c.taskRun)
			if err != nil {
				t.Fatalf("I did not expect an error but I got %s", err)
			} else {
				wantSubject := taskRunName
				if d := cmp.Diff(wantSubject, got.Subject()); d != "" {
					t.Errorf("Wrong Event ID %s", diff.PrintWantGot(d))
				}
				if d := cmp.Diff(string(c.wantEventType), got.Type()); d != "" {
					t.Errorf("Wrong Event Type %s", diff.PrintWantGot(d))
				}
				wantData := NewTektonCloudEventData(c.taskRun)
				gotData := TektonCloudEventData{}
				if err := got.DataAs(&gotData); err != nil {
					t.Errorf("Unexpected error from DataAsl; %s", err)
				}
				if d := cmp.Diff(wantData, gotData); d != "" {
					t.Errorf("Wrong Event data %s", diff.PrintWantGot(d))
				}

				if err := got.Validate(); err != nil {
					t.Errorf("Expected event to be valid; %s", err)
				}
			}
		})
	}
}
