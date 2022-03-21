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

package pipelinerun

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	_ "github.com/tektoncd/pipeline/pkg/pipelinerunmetrics/fake" // Make sure the pipelinerunmetrics are setup
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestCancelPipelineRun(t *testing.T) {
	testCases := []struct {
		name            string
		pipelineRun     *v1beta1.PipelineRun
		taskRuns        []*v1beta1.TaskRun
		runs            []*v1alpha1.Run
		ignoredTaskRuns []*v1beta1.TaskRun
		ignoredRuns     []*v1alpha1.Run
		expectedErr     error
	}{{
		name: "no-resolved-taskrun",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
		},
	}, {
		name: "one-taskrun",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
				},
			}},
		},
		taskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
		},
	}, {
		name: "multiple-taskruns",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
					"t2": {PipelineTaskName: "task-2"},
				},
			}},
		},
		taskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
	}, {
		name: "multiple-runs",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				Runs: map[string]*v1beta1.PipelineRunRunStatus{
					"t1": {PipelineTaskName: "task-1"},
					"t2": {PipelineTaskName: "task-2"},
				},
			}},
		},
		runs: []*v1alpha1.Run{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
	}, {
		name: "deprecated-state",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelledDeprecated,
			},
		},
	}, {
		name: "child-references",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{
					{
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
						Name:             "t1",
						PipelineTaskName: "task-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
						Name:             "t2",
						PipelineTaskName: "task-2",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: "Run"},
						Name:             "r1",
						PipelineTaskName: "run-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: "Run"},
						Name:             "r2",
						PipelineTaskName: "run-2",
					},
				},
				// If ChildReferences is not empty, TaskRuns and Runs should be ignored.
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t3": {PipelineTaskName: "task-3"},
					"t4": {PipelineTaskName: "task-4"},
				},
				Runs: map[string]*v1beta1.PipelineRunRunStatus{
					"r3": {PipelineTaskName: "run-3"},
					"r4": {PipelineTaskName: "run-4"},
				},
			}},
		},
		taskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
		runs: []*v1alpha1.Run{
			{ObjectMeta: metav1.ObjectMeta{Name: "r1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "r2"}},
		},
		ignoredTaskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t3"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t4"}},
		},
		ignoredRuns: []*v1alpha1.Run{
			{ObjectMeta: metav1.ObjectMeta{Name: "r3"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "r4"}},
		},
	}, {
		name: "unknown-kind-on-child-references",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta:         runtime.TypeMeta{Kind: "InvalidKind"},
					Name:             "t1",
					PipelineTaskName: "task-1",
				}},
			}},
		},
		expectedErr: errors.New("error(s) from cancelling TaskRun(s) from PipelineRun test-pipeline-run-cancelled: unknown or unsupported kind `InvalidKind` for cancellation"),
	}}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var seedTaskRuns []*v1beta1.TaskRun
			var seedRuns []*v1alpha1.Run

			for _, tr := range tc.taskRuns {
				seedTaskRuns = append(seedTaskRuns, &v1beta1.TaskRun{
					ObjectMeta: metav1.ObjectMeta{Name: tr.Name,
						Labels: map[string]string{"shouldCancel": "true"}},
				})
			}
			for _, tr := range tc.ignoredTaskRuns {
				seedTaskRuns = append(seedTaskRuns, &v1beta1.TaskRun{
					ObjectMeta: metav1.ObjectMeta{Name: tr.Name,
						Labels: map[string]string{"shouldCancel": "false"}},
				})
			}

			for _, r := range tc.runs {
				seedRuns = append(seedRuns, &v1alpha1.Run{
					ObjectMeta: metav1.ObjectMeta{Name: r.Name,
						Labels: map[string]string{"shouldCancel": "true"}},
				})
			}
			for _, r := range tc.ignoredRuns {
				seedRuns = append(seedRuns, &v1alpha1.Run{
					ObjectMeta: metav1.ObjectMeta{Name: r.Name,
						Labels: map[string]string{"shouldCancel": "false"}},
				})
			}

			d := test.Data{
				PipelineRuns: []*v1beta1.PipelineRun{tc.pipelineRun},
				TaskRuns:     seedTaskRuns,
				Runs:         seedRuns,
			}
			ctx, _ := ttesting.SetupFakeContext(t)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, _ := test.SeedTestData(t, ctx, d)

			err := cancelPipelineRun(ctx, logtesting.TestLogger(t), tc.pipelineRun, c.Pipeline)
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error '%s', but did not get an error", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("expected error '%s', but got '%s'", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				// This PipelineRun should still be complete and false, and the status should reflect that
				cond := tc.pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
				if cond.IsTrue() {
					t.Errorf("Expected PipelineRun status to be complete and false, but was %v", cond)
				}
				if tc.taskRuns != nil {
					l, err := c.Pipeline.TektonV1beta1().TaskRuns("").List(ctx, metav1.ListOptions{
						LabelSelector: "shouldCancel=true",
					})
					if err != nil {
						t.Fatal(err)
					}
					for _, tr := range l.Items {
						if tr.Spec.Status != v1beta1.TaskRunSpecStatusCancelled {
							t.Errorf("expected task %q to be marked as cancelled, was %q", tr.Name, tr.Spec.Status)
						}
					}
				}
				if tc.runs != nil {
					l, err := c.Pipeline.TektonV1alpha1().Runs("").List(ctx, metav1.ListOptions{
						LabelSelector: "shouldCancel=true",
					})
					if err != nil {
						t.Fatal(err)
					}
					for _, r := range l.Items {
						if r.Spec.Status != v1alpha1.RunSpecStatusCancelled {
							t.Errorf("expected Run %q to be marked as cancelled, was %q", r.Name, r.Spec.Status)
						}
					}
				}
				if tc.ignoredTaskRuns != nil {
					l, err := c.Pipeline.TektonV1beta1().TaskRuns("").List(ctx, metav1.ListOptions{
						LabelSelector: "shouldCancel=false",
					})
					if err != nil {
						t.Fatal(err)
					}
					for _, tr := range l.Items {
						if tr.Spec.Status == v1beta1.TaskRunSpecStatusCancelled {
							t.Errorf("expected task %q to NOT be marked as cancelled, was %q", tr.Name, tr.Spec.Status)
						}
					}
				}
				if tc.ignoredRuns != nil {
					l, err := c.Pipeline.TektonV1alpha1().Runs("").List(ctx, metav1.ListOptions{
						LabelSelector: "shouldCancel=false",
					})
					if err != nil {
						t.Fatal(err)
					}
					for _, r := range l.Items {
						if r.Spec.Status == v1alpha1.RunSpecStatusCancelled {
							t.Errorf("expected Run %q to NOT be marked as cancelled, was %q", r.Name, r.Spec.Status)
						}
					}
				}
			}
		})
	}
}
