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

package v1alpha1_test

import (
	"fmt"
	"testing"
	"time"

	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskRun_GetPipelineRunPVCName(t *testing.T) {
	tests := []struct {
		name            string
		tr              *v1alpha1.TaskRun
		expectedPVCName string
	}{{
		name: "invalid owner reference",
		tr: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunname",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "SomeOtherOwner",
					Name: "testpr",
				}},
			},
		},
		expectedPVCName: "",
	}, {
		name: "valid pipelinerun owner",
		tr: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunname",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "testpr",
				}},
			},
		},
		expectedPVCName: "testpr-pvc",
	}, {
		name:            "nil taskrun",
		expectedPVCName: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tr.GetPipelineRunPVCName() != tt.expectedPVCName {
				t.Fatalf("taskrun pipeline run pvc name mismatch: got %s ; expected %s", tt.tr.GetPipelineRunPVCName(), tt.expectedPVCName)
			}
		})
	}
}

func TestTaskRun_HasPipelineRun(t *testing.T) {
	tests := []struct {
		name string
		tr   *v1alpha1.TaskRun
		want bool
	}{{
		name: "invalid owner reference",
		tr: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunname",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "SomeOtherOwner",
					Name: "testpr",
				}},
			},
		},
		want: false,
	}, {
		name: "valid pipelinerun owner",
		tr: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunname",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "PipelineRun",
					Name: "testpr",
				}},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tr.HasPipelineRunOwnerReference() != tt.want {
				t.Fatalf("taskrun pipeline run pvc name mismatch: got %s ; expected %t", tt.tr.GetPipelineRunPVCName(), tt.want)
			}
		})
	}
}

func TestTaskRunIsDone(t *testing.T) {
	tr := &v1alpha1.TaskRun{
		Status: v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					},
				},
			},
		},
	}
	if !tr.IsDone() {
		t.Fatal("Expected pipelinerun status to be done")
	}
}

func TestTaskRunIsCancelled(t *testing.T) {
	tr := &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Status: v1alpha1.TaskRunSpecStatusCancelled,
		},
	}
	if !tr.IsCancelled() {
		t.Fatal("Expected pipelinerun status to be cancelled")
	}
}

func TestTaskRunHasVolumeClaimTemplate(t *testing.T) {
	tr := &v1alpha1.TaskRun{
		Spec: v1alpha1.TaskRunSpec{
			Workspaces: []v1alpha1.WorkspaceBinding{{
				Name: "my-workspace",
				VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc",
					},
					Spec: corev1.PersistentVolumeClaimSpec{},
				},
			}},
		},
	}
	if !tr.HasVolumeClaimTemplate() {
		t.Fatal("Expected taskrun to have a volumeClaimTemplate workspace")
	}
}

func TestTaskRunKey(t *testing.T) {
	tr := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "taskrunname",
		},
	}
	expectedKey := fmt.Sprintf("TaskRun/%p", tr)
	if tr.GetRunKey() != expectedKey {
		t.Fatalf("Expected taskrun key to be %s but got %s", expectedKey, tr.GetRunKey())
	}
}

func TestTaskRunHasStarted(t *testing.T) {
	params := []struct {
		name          string
		trStatus      v1alpha1.TaskRunStatus
		expectedValue bool
	}{{
		name:          "trWithNoStartTime",
		trStatus:      v1alpha1.TaskRunStatus{},
		expectedValue: false,
	}, {
		name: "trWithStartTime",
		trStatus: v1alpha1.TaskRunStatus{
			TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
		expectedValue: true,
	}, {
		name: "trWithZeroStartTime",
		trStatus: v1alpha1.TaskRunStatus{
			TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
				StartTime: &metav1.Time{},
			},
		},
		expectedValue: false,
	}}
	for _, tc := range params {
		t.Run(tc.name, func(t *testing.T) {
			tr := &v1alpha1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskrunname",
				},
			}
			tr.Status = tc.trStatus
			if tr.HasStarted() != tc.expectedValue {
				t.Fatalf("Expected taskrun HasStarted() to return %t but got %t", tc.expectedValue, tr.HasStarted())
			}
		})
	}
}

func TestTaskRunIsOfPipelinerun(t *testing.T) {
	tests := []struct {
		name                  string
		tr                    *v1alpha1.TaskRun
		expectedValue         bool
		expetectedPipeline    string
		expetectedPipelineRun string
	}{{
		name: "yes",
		tr: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunname",
				Labels: map[string]string{
					pipeline.PipelineLabelKey:    "pipeline",
					pipeline.PipelineRunLabelKey: "pipelinerun",
				},
			},
		},
		expectedValue:         true,
		expetectedPipeline:    "pipeline",
		expetectedPipelineRun: "pipelinerun",
	}, {
		name: "no",
		tr: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunname",
			},
		},
		expectedValue: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, pipeline, pipelineRun := test.tr.IsPartOfPipeline()
			if value != test.expectedValue {
				t.Fatalf("Expecting %v got %v", test.expectedValue, value)
			}

			if pipeline != test.expetectedPipeline {
				t.Fatalf("Mismatch in pipeline: got %s expected %s", pipeline, test.expetectedPipeline)
			}

			if pipelineRun != test.expetectedPipelineRun {
				t.Fatalf("Mismatch in pipelinerun: got %s expected %s", pipelineRun, test.expetectedPipelineRun)
			}
		})
	}
}

func TestHasTimedOut(t *testing.T) {
	// IsZero reports whether t represents the zero time instant, January 1, year 1, 00:00:00 UTC
	zeroTime := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	testCases := []struct {
		name           string
		taskRun        *v1alpha1.TaskRun
		expectedStatus bool
	}{{
		name: "TaskRun not started",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-taskrun-not-started",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "task-name",
				},
			},
			Status: v1alpha1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{
						apis.Condition{},
					},
				},
				TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
					StartTime: &metav1.Time{Time: zeroTime},
				},
			},
		},
		expectedStatus: false,
	}, {
		name: "TaskRun no timeout",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-taskrun-no-timeout",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "task-name",
				},
				Timeout: &metav1.Duration{Duration: 0},
			},
			Status: v1alpha1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{
						apis.Condition{},
					},
				},
				TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-15 * time.Hour)},
				},
			},
		},
		expectedStatus: false,
	}, {
		name: "TaskRun timed out",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-taskrun-timeout",
			},
			Spec: v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{
					Name: "task-name",
				},
				Timeout: &metav1.Duration{Duration: 10 * time.Second},
			},
			Status: v1alpha1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{
						apis.Condition{},
					},
				},
				TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-15 * time.Second)},
				},
			},
		},
		expectedStatus: true,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.taskRun.HasTimedOut(testClock{})
			if d := cmp.Diff(result, tc.expectedStatus); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}
