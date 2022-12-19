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

package v1_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineRunStatusConditions(t *testing.T) {
	p := &v1.PipelineRun{}
	foo := &apis.Condition{
		Type:   "Foo",
		Status: "True",
	}
	bar := &apis.Condition{
		Type:   "Bar",
		Status: "True",
	}

	var ignoreVolatileTime = cmp.Comparer(func(_, _ apis.VolatileTime) bool {
		return true
	})

	// Add a new condition.
	p.Status.SetCondition(foo)

	fooStatus := p.Status.GetCondition(foo.Type)
	if d := cmp.Diff(fooStatus, foo, ignoreVolatileTime); d != "" {
		t.Errorf("Unexpected pipeline run condition type; diff %v", diff.PrintWantGot(d))
	}

	// Add a second condition.
	p.Status.SetCondition(bar)

	barStatus := p.Status.GetCondition(bar.Type)

	if d := cmp.Diff(barStatus, bar, ignoreVolatileTime); d != "" {
		t.Fatalf("Unexpected pipeline run condition type; diff %s", diff.PrintWantGot(d))
	}
}

func TestInitializePipelineRunConditions(t *testing.T) {
	p := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-ns",
		},
	}
	p.Status.InitializeConditions(testClock)

	if p.Status.StartTime.IsZero() {
		t.Fatalf("PipelineRun StartTime not initialized correctly")
	}

	condition := p.Status.GetCondition(apis.ConditionSucceeded)
	if condition.Reason != v1.PipelineRunReasonStarted.String() {
		t.Fatalf("PipelineRun initialize reason should be %s, got %s instead", v1.PipelineRunReasonStarted.String(), condition.Reason)
	}

	// Change the reason before we initialize again
	p.Status.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  "not just started",
		Message: "hello",
	})

	p.Status.InitializeConditions(testClock)

	newCondition := p.Status.GetCondition(apis.ConditionSucceeded)
	if newCondition.Reason != "not just started" {
		t.Fatalf("PipelineRun initialize reset the condition reason to %s", newCondition.Reason)
	}
}

func TestPipelineRunIsDone(t *testing.T) {
	pr := &v1.PipelineRun{}
	foo := &apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionFalse,
	}
	pr.Status.SetCondition(foo)
	if !pr.IsDone() {
		t.Fatal("Expected pipelinerun status to be done")
	}
}

func TestPipelineRunIsCancelled(t *testing.T) {
	pr := &v1.PipelineRun{
		Spec: v1.PipelineRunSpec{
			Status: v1.PipelineRunSpecStatusCancelled,
		},
	}
	if !pr.IsCancelled() {
		t.Fatal("Expected pipelinerun status to be cancelled")
	}
}

func TestPipelineRunIsGracefullyCancelled(t *testing.T) {
	pr := &v1.PipelineRun{
		Spec: v1.PipelineRunSpec{
			Status: v1.PipelineRunSpecStatusCancelledRunFinally,
		},
	}
	if !pr.IsGracefullyCancelled() {
		t.Fatal("Expected pipelinerun status to be gracefully cancelled")
	}
}

func TestPipelineRunIsGracefullyStopped(t *testing.T) {
	pr := &v1.PipelineRun{
		Spec: v1.PipelineRunSpec{
			Status: v1.PipelineRunSpecStatusStoppedRunFinally,
		},
	}
	if !pr.IsGracefullyStopped() {
		t.Fatal("Expected pipelinerun status to be gracefully stopped")
	}
}

func TestPipelineRunHasVolumeClaimTemplate(t *testing.T) {
	pr := &v1.PipelineRun{
		Spec: v1.PipelineRunSpec{
			Workspaces: []v1.WorkspaceBinding{{
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
	if !pr.HasVolumeClaimTemplate() {
		t.Fatal("Expected pipelinerun to have a volumeClaimTemplate workspace")
	}
}

func TestGetNamespacedName(t *testing.T) {
	pr := &v1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "prunname"}}
	n := pr.GetNamespacedName()
	expected := "foo/prunname"
	if n.String() != expected {
		t.Fatalf("Expected name to be %s but got %s", expected, n.String())
	}
}

func TestPipelineRunHasStarted(t *testing.T) {
	params := []struct {
		name          string
		prStatus      v1.PipelineRunStatus
		expectedValue bool
	}{{
		name:          "prWithNoStartTime",
		prStatus:      v1.PipelineRunStatus{},
		expectedValue: false,
	}, {
		name: "prWithStartTime",
		prStatus: v1.PipelineRunStatus{
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
		expectedValue: true,
	}, {
		name: "prWithZeroStartTime",
		prStatus: v1.PipelineRunStatus{
			PipelineRunStatusFields: v1.PipelineRunStatusFields{
				StartTime: &metav1.Time{},
			},
		},
		expectedValue: false,
	}}
	for _, tc := range params {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prunname",
					Namespace: "testns",
				},
				Status: tc.prStatus,
			}
			if pr.HasStarted() != tc.expectedValue {
				t.Fatalf("Expected pipelinerun HasStarted() to return %t but got %t", tc.expectedValue, pr.HasStarted())
			}
		})
	}
}

func TestPipelineRunTimeouts(t *testing.T) {
	tcs := []struct {
		name                   string
		timeouts               *v1.TimeoutFields
		expectedTasksTimeout   *metav1.Duration
		expectedFinallyTimeout *metav1.Duration
	}{{
		name: "no timeouts",
	}, {
		name:     "pipeline timeout set",
		timeouts: &v1.TimeoutFields{Pipeline: &metav1.Duration{Duration: time.Minute}},
	}, {
		name:                   "pipeline and tasks timeout set",
		timeouts:               &v1.TimeoutFields{Pipeline: &metav1.Duration{Duration: time.Hour}, Tasks: &metav1.Duration{Duration: 10 * time.Minute}},
		expectedTasksTimeout:   &metav1.Duration{Duration: 10 * time.Minute},
		expectedFinallyTimeout: &metav1.Duration{Duration: 50 * time.Minute},
	}, {
		name:                   "pipeline and finally timeout set",
		timeouts:               &v1.TimeoutFields{Pipeline: &metav1.Duration{Duration: time.Hour}, Finally: &metav1.Duration{Duration: 10 * time.Minute}},
		expectedTasksTimeout:   &metav1.Duration{Duration: 50 * time.Minute},
		expectedFinallyTimeout: &metav1.Duration{Duration: 10 * time.Minute},
	}, {
		name:                 "tasks timeout set",
		timeouts:             &v1.TimeoutFields{Tasks: &metav1.Duration{Duration: 10 * time.Minute}},
		expectedTasksTimeout: &metav1.Duration{Duration: 10 * time.Minute},
	}, {
		name:                   "finally timeout set",
		timeouts:               &v1.TimeoutFields{Finally: &metav1.Duration{Duration: 10 * time.Minute}},
		expectedFinallyTimeout: &metav1.Duration{Duration: 10 * time.Minute},
	}, {
		name:                 "no tasks timeout",
		timeouts:             &v1.TimeoutFields{Pipeline: &metav1.Duration{Duration: 0}, Tasks: &metav1.Duration{Duration: 0}},
		expectedTasksTimeout: &metav1.Duration{Duration: 0},
	}, {
		name:                   "no finally timeout",
		timeouts:               &v1.TimeoutFields{Pipeline: &metav1.Duration{Duration: 0}, Finally: &metav1.Duration{Duration: 0}},
		expectedFinallyTimeout: &metav1.Duration{Duration: 0},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: v1.PipelineRunSpec{
					Timeouts: tc.timeouts,
				},
			}

			tasksTimeout := pr.TasksTimeout()
			if ok := cmp.Equal(tc.expectedTasksTimeout, pr.TasksTimeout()); !ok {
				t.Errorf("Unexpected tasks timeout %v, expected %v", tasksTimeout, tc.expectedTasksTimeout)
			}
			finallyTimeout := pr.FinallyTimeout()
			if ok := cmp.Equal(tc.expectedFinallyTimeout, pr.FinallyTimeout()); !ok {
				t.Errorf("Unexpected finally timeout %v, expected %v", finallyTimeout, tc.expectedFinallyTimeout)
			}
		})
	}
}

func TestPipelineRunGetPodSpecSABackcompatibility(t *testing.T) {
	for _, tt := range []struct {
		name        string
		pr          *v1.PipelineRun
		expectedSAs map[string]string
	}{
		{
			name: "test backward compatibility",
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pr"},
				Spec: v1.PipelineRunSpec{
					PipelineRef: &v1.PipelineRef{Name: "prs"},
					TaskRunTemplate: v1.PipelineTaskRunTemplate{
						ServiceAccountName: "defaultSA",
					},
					TaskRunSpecs: []v1.PipelineTaskRunSpec{{
						PipelineTaskName:   "taskName",
						ServiceAccountName: "newTaskSA",
					}},
				},
			},
			expectedSAs: map[string]string{
				"unknown":  "defaultSA",
				"taskName": "newTaskSA",
			},
		}, {
			name: "mixed default SA backward compatibility",
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pr"},
				Spec: v1.PipelineRunSpec{
					PipelineRef: &v1.PipelineRef{Name: "prs"},
					TaskRunTemplate: v1.PipelineTaskRunTemplate{
						ServiceAccountName: "defaultSA",
					},
					TaskRunSpecs: []v1.PipelineTaskRunSpec{{
						PipelineTaskName:   "taskNameOne",
						ServiceAccountName: "TaskSAOne",
					}, {
						PipelineTaskName:   "taskNameTwo",
						ServiceAccountName: "newTaskTwo",
					}},
				},
			},
			expectedSAs: map[string]string{
				"unknown":     "defaultSA",
				"taskNameOne": "TaskSAOne",
				"taskNameTwo": "newTaskTwo",
			},
		}, {
			name: "mixed SA and TaskRunSpec",
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pr"},
				Spec: v1.PipelineRunSpec{
					PipelineRef: &v1.PipelineRef{Name: "prs"},
					TaskRunTemplate: v1.PipelineTaskRunTemplate{
						ServiceAccountName: "defaultSA",
					},
					TaskRunSpecs: []v1.PipelineTaskRunSpec{{
						PipelineTaskName: "taskNameOne",
					}, {
						PipelineTaskName:   "taskNameTwo",
						ServiceAccountName: "newTaskTwo",
					}},
				},
			},
			expectedSAs: map[string]string{
				"unknown":     "defaultSA",
				"taskNameOne": "defaultSA",
				"taskNameTwo": "newTaskTwo",
			},
		},
	} {
		for taskName, expected := range tt.expectedSAs {
			t.Run(tt.name, func(t *testing.T) {
				s := tt.pr.GetTaskRunSpec(taskName)
				if expected != s.ServiceAccountName {
					t.Errorf("wrong service account: got: %v, want: %v", s.ServiceAccountName, expected)
				}
			})
		}
	}
}

func TestPipelineRunGetPodSpec(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		pr                   *v1.PipelineRun
		expectedPodTemplates map[string][]string
	}{
		{
			name: "mix default and none default",
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pr"},
				Spec: v1.PipelineRunSpec{
					PipelineRef: &v1.PipelineRef{Name: "prs"},
					TaskRunTemplate: v1.PipelineTaskRunTemplate{
						ServiceAccountName: "defaultSA",
						PodTemplate:        &pod.Template{SchedulerName: "scheduleTest"},
					},
					TaskRunSpecs: []v1.PipelineTaskRunSpec{{
						PipelineTaskName:   "taskNameOne",
						ServiceAccountName: "TaskSAOne",
						PodTemplate:        &pod.Template{SchedulerName: "scheduleTestOne"},
					}, {
						PipelineTaskName:   "taskNameTwo",
						ServiceAccountName: "newTaskTwo",
						PodTemplate:        &pod.Template{SchedulerName: "scheduleTestTwo"},
					}},
				},
			},
			expectedPodTemplates: map[string][]string{
				"unknown":     {"scheduleTest", "defaultSA"},
				"taskNameOne": {"scheduleTestOne", "TaskSAOne"},
				"taskNameTwo": {"scheduleTestTwo", "newTaskTwo"},
			},
		},
	} {
		for taskName, values := range tt.expectedPodTemplates {
			t.Run(tt.name, func(t *testing.T) {
				s := tt.pr.GetTaskRunSpec(taskName)
				if values[0] != s.PodTemplate.SchedulerName {
					t.Errorf("wrong task podtemplate scheduler name: got: %v, want: %v", s.PodTemplate.SchedulerName, values[0])
				}
				if values[1] != s.ServiceAccountName {
					t.Errorf("wrong service account: got: %v, want: %v", s.ServiceAccountName, values[1])
				}
			})
		}
	}
}
