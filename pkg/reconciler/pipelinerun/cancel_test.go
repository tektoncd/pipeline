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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	_ "github.com/tektoncd/pipeline/pkg/pipelinerunmetrics/fake" // Make sure the pipelinerunmetrics are setup
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestCancelPipelineRun(t *testing.T) {
	testCases := []struct {
		name string

		pipelineRun *v1beta1.PipelineRun
		taskRuns    []*v1beta1.TaskRun
		runs        []*v1alpha1.Run
		customRuns  []*v1beta1.CustomRun
		wantErr     bool
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
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta:         runtime.TypeMeta{Kind: taskRun},
					Name:             "t1",
					PipelineTaskName: "task-1",
				}},
			}},
		},
		taskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
		},
	}, {
		name: "multiple-taskruns-one-missing",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta:         runtime.TypeMeta{Kind: taskRun},
					Name:             "t1",
					PipelineTaskName: "task-1",
				}, {
					TypeMeta:         runtime.TypeMeta{Kind: taskRun},
					Name:             "t2",
					PipelineTaskName: "task-2",
				}},
			}},
		},
		taskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
	}, {
		name: "multiple-taskruns",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta:         runtime.TypeMeta{Kind: taskRun},
					Name:             "t1",
					PipelineTaskName: "task-1",
				}, {
					TypeMeta:         runtime.TypeMeta{Kind: taskRun},
					Name:             "t2",
					PipelineTaskName: "task-2",
				}},
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
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta:         runtime.TypeMeta{Kind: customRun},
					Name:             "t1",
					PipelineTaskName: "task-1",
				}, {
					TypeMeta:         runtime.TypeMeta{Kind: customRun},
					Name:             "t2",
					PipelineTaskName: "task-2",
				}},
			}},
		},
		customRuns: []*v1beta1.CustomRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
	}, {
		name: "multiple-runs-one-missing",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta:         runtime.TypeMeta{Kind: customRun},
					Name:             "t1",
					PipelineTaskName: "task-1",
				}, {
					TypeMeta:         runtime.TypeMeta{Kind: customRun},
					Name:             "t2",
					PipelineTaskName: "task-2",
				}},
			}},
		},
		customRuns: []*v1beta1.CustomRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
		},
	}, {
		name: "multiple-taskruns-and-runs",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{
					{
						TypeMeta:         runtime.TypeMeta{Kind: taskRun},
						Name:             "t1",
						PipelineTaskName: "task-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: taskRun},
						Name:             "t2",
						PipelineTaskName: "task-2",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: run},
						Name:             "r1",
						PipelineTaskName: "run-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: run},
						Name:             "r2",
						PipelineTaskName: "run-2",
					},
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
	}, {
		name: "child-references-some-missing",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{
					{
						TypeMeta:         runtime.TypeMeta{Kind: taskRun},
						Name:             "t1",
						PipelineTaskName: "task-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: taskRun},
						Name:             "t2",
						PipelineTaskName: "task-2",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: run},
						Name:             "r1",
						PipelineTaskName: "run-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: run},
						Name:             "r2",
						PipelineTaskName: "run-2",
					},
				},
			}},
		},
		taskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
		runs: []*v1alpha1.Run{
			{ObjectMeta: metav1.ObjectMeta{Name: "r1"}},
		},
	}, {
		name: "child-references-with-customruns",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{
					{
						TypeMeta:         runtime.TypeMeta{Kind: taskRun},
						Name:             "t1",
						PipelineTaskName: "task-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: taskRun},
						Name:             "t2",
						PipelineTaskName: "task-2",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: customRun},
						Name:             "cr1",
						PipelineTaskName: "customrun-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: customRun},
						Name:             "cr2",
						PipelineTaskName: "customrun-2",
					},
				},
			}},
		},
		taskRuns: []*v1beta1.TaskRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
		customRuns: []*v1beta1.CustomRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "cr1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "cr2"}},
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
		wantErr: true,
	}}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				PipelineRuns: []*v1beta1.PipelineRun{tc.pipelineRun},
				TaskRuns:     tc.taskRuns,
				Runs:         tc.runs,
				CustomRuns:   tc.customRuns,
			}
			ctx, _ := ttesting.SetupFakeContext(t)
			cfg := config.NewStore(logtesting.TestLogger(t))
			ctx = cfg.ToContext(ctx)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, _ := test.SeedTestData(t, ctx, d)

			err := cancelPipelineRun(ctx, logtesting.TestLogger(t), tc.pipelineRun, c.Pipeline)
			if tc.wantErr {
				if err == nil {
					t.Error("expected an error, but did not get one")
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
					for _, expectedTR := range tc.taskRuns {
						tr, err := c.Pipeline.TektonV1beta1().TaskRuns("").Get(ctx, expectedTR.Name, metav1.GetOptions{})
						if err != nil {
							t.Fatalf("couldn't get expected TaskRun %s, got error %s", expectedTR.Name, err)
						}
						if tr.Spec.Status != v1beta1.TaskRunSpecStatusCancelled {
							t.Errorf("expected task %q to be marked as cancelled, was %q", tr.Name, tr.Spec.Status)
						}
						expectedStatusMessage := v1beta1.TaskRunCancelledByPipelineMsg
						if tr.Spec.StatusMessage != expectedStatusMessage {
							t.Errorf("expected task %q to have status message %s but was %s", tr.Name, expectedStatusMessage, tr.Spec.StatusMessage)
						}
					}
				}
				if tc.runs != nil {
					for _, expectedRun := range tc.runs {
						r, err := c.Pipeline.TektonV1alpha1().Runs("").Get(ctx, expectedRun.Name, metav1.GetOptions{})
						if err != nil {
							t.Fatalf("couldn't get expected Run %s, got error %s", expectedRun.Name, err)
						}
						if r.Spec.Status != v1alpha1.RunSpecStatusCancelled {
							t.Errorf("expected task %q to be marked as cancelled, was %q", r.Name, r.Spec.Status)
						}
						expectedStatusMessage := v1alpha1.RunCancelledByPipelineMsg
						if r.Spec.StatusMessage != expectedStatusMessage {
							t.Errorf("expected task %q to have status message %s but was %s", r.Name, expectedStatusMessage, r.Spec.StatusMessage)
						}
					}
				}
				if tc.customRuns != nil {
					for _, expectedCustomRun := range tc.customRuns {
						cr, err := c.Pipeline.TektonV1beta1().CustomRuns("").Get(ctx, expectedCustomRun.Name, metav1.GetOptions{})
						if err != nil {
							t.Fatalf("couldn't get expected CustomRun %s, got error %s", expectedCustomRun.Name, err)
						}
						if cr.Spec.Status != v1beta1.CustomRunSpecStatusCancelled {
							t.Errorf("expected task %q to be marked as cancelled, was %q", cr.Name, cr.Spec.Status)
						}
						expectedStatusMessage := v1beta1.CustomRunCancelledByPipelineMsg
						if cr.Spec.StatusMessage != expectedStatusMessage {
							t.Errorf("expected task %q to have status message %s but was %s", cr.Name, expectedStatusMessage, cr.Spec.StatusMessage)
						}
					}
				}
			}
		})
	}
}

func TestGetChildObjectsFromPRStatusForTaskNames(t *testing.T) {
	testCases := []struct {
		name                   string
		useV1Beta1CustomTask   bool
		prStatus               v1beta1.PipelineRunStatus
		taskNames              sets.String
		expectedTRNames        []string
		expectedRunNames       []string
		expectedCustomRunNames []string
		hasError               bool
	}{
		{
			name: "runs",
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta: runtime.TypeMeta{
						APIVersion: v1alpha1.SchemeGroupVersion.String(),
						Kind:       run,
					},
					Name:             "r1",
					PipelineTaskName: "run-1",
				}},
			}},
			expectedTRNames:  nil,
			expectedRunNames: []string{"r1"},
			hasError:         false,
		}, {
			name:                 "beta custom tasks",
			useV1Beta1CustomTask: true,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta: runtime.TypeMeta{
						APIVersion: v1beta1.SchemeGroupVersion.String(),
						Kind:       customRun,
					},
					Name:             "r1",
					PipelineTaskName: "run-1",
				}},
			}},
			expectedCustomRunNames: []string{"r1"},
			hasError:               false,
		}, {
			name: "unknown kind",
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "v1",
						Kind:       "UnknownKind",
					},
					Name:             "u1",
					PipelineTaskName: "unknown-1",
				}},
			}},
			expectedTRNames:        nil,
			expectedRunNames:       nil,
			expectedCustomRunNames: nil,
			hasError:               true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)
			trNames, customRunNames, runNames, err := getChildObjectsFromPRStatusForTaskNames(ctx, tc.prStatus, tc.taskNames)

			if tc.hasError {
				if err == nil {
					t.Error("expected to see an error, but did not")
				}
			} else if err != nil {
				t.Errorf("did not expect to see an error, but saw %v", err)
			}

			if d := cmp.Diff(tc.expectedTRNames, trNames); d != "" {
				t.Errorf("expected to see TaskRun names %v. Diff %s", tc.expectedTRNames, diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedRunNames, runNames); d != "" {
				t.Errorf("expected to see Run names %v. Diff %s", tc.expectedRunNames, diff.PrintWantGot(d))
			}
			if d := cmp.Diff(tc.expectedCustomRunNames, customRunNames); d != "" {
				t.Errorf("expected to see CustomRun names %v. Diff %s", tc.expectedCustomRunNames, diff.PrintWantGot(d))
			}
		})
	}
}
