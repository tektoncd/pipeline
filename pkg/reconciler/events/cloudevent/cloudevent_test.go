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
	defaultEventSourceURI = "/runtocompletion/1234"
	taskRunName           = "faketaskrunname"
	pipelineRunName       = "fakepipelinerunname"
)

func getTaskRunByCondition(status corev1.ConditionStatus, reason string) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TaskRun",
			APIVersion: "v1beta1",
		},
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
					Reason: reason,
				}},
			},
		},
	}
}

func getPipelineRunByCondition(status corev1.ConditionStatus, reason string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PipelineRun",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineRunName,
			Namespace: "marshmallow",
			SelfLink:  defaultEventSourceURI,
		},
		Spec: v1beta1.PipelineRunSpec{},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: status,
					Reason: reason,
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
		desc:          "send a cloud event when a taskrun starts",
		taskRun:       getTaskRunByCondition(corev1.ConditionUnknown, v1beta1.TaskRunReasonStarted.String()),
		wantEventType: TaskRunStartedEventV1,
	}, {
		desc:          "send a cloud event when a taskrun starts running",
		taskRun:       getTaskRunByCondition(corev1.ConditionUnknown, v1beta1.TaskRunReasonRunning.String()),
		wantEventType: TaskRunRunningEventV1,
	}, {
		desc:          "send a cloud event with unknown status taskrun",
		taskRun:       getTaskRunByCondition(corev1.ConditionUnknown, "doesn't matter"),
		wantEventType: TaskRunUnknownEventV1,
	}, {
		desc:          "send a cloud event with failed status taskrun",
		taskRun:       getTaskRunByCondition(corev1.ConditionFalse, "meh"),
		wantEventType: TaskRunFailedEventV1,
	}, {
		desc:          "send a cloud event with successful status taskrun",
		taskRun:       getTaskRunByCondition(corev1.ConditionTrue, "yay"),
		wantEventType: TaskRunSuccessfulEventV1,
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

func TestEventForPipelineRun(t *testing.T) {
	for _, c := range []struct {
		desc          string
		pipelineRun   *v1beta1.PipelineRun
		wantEventType TektonEventType
	}{{
		desc:          "send a cloud event with unknown status pipelinerun, just started",
		pipelineRun:   getPipelineRunByCondition(corev1.ConditionUnknown, v1beta1.PipelineRunReasonStarted.String()),
		wantEventType: PipelineRunStartedEventV1,
	}, {
		desc:          "send a cloud event with unknown status pipelinerun, just started running",
		pipelineRun:   getPipelineRunByCondition(corev1.ConditionUnknown, v1beta1.PipelineRunReasonRunning.String()),
		wantEventType: PipelineRunRunningEventV1,
	}, {
		desc:          "send a cloud event with unknown status pipelinerun",
		pipelineRun:   getPipelineRunByCondition(corev1.ConditionUnknown, "doesn't matter"),
		wantEventType: PipelineRunUnknownEventV1,
	}, {
		desc:          "send a cloud event with successful status pipelinerun",
		pipelineRun:   getPipelineRunByCondition(corev1.ConditionTrue, "yay"),
		wantEventType: PipelineRunSuccessfulEventV1,
	}, {
		desc:          "send a cloud event with unknown status pipelinerun",
		pipelineRun:   getPipelineRunByCondition(corev1.ConditionFalse, "meh"),
		wantEventType: PipelineRunFailedEventV1,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()

			got, err := EventForPipelineRun(c.pipelineRun)
			if err != nil {
				t.Fatalf("I did not expect an error but I got %s", err)
			} else {
				wantSubject := pipelineRunName
				if d := cmp.Diff(wantSubject, got.Subject()); d != "" {
					t.Errorf("Wrong Event ID %s", diff.PrintWantGot(d))
				}
				if d := cmp.Diff(string(c.wantEventType), got.Type()); d != "" {
					t.Errorf("Wrong Event Type %s", diff.PrintWantGot(d))
				}
				wantData := NewTektonCloudEventData(c.pipelineRun)
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
