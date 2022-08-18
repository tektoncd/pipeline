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

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/diff"

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
		name           string
		embeddedStatus string
		pipelineRun    *v1beta1.PipelineRun
		taskRuns       []*v1beta1.TaskRun
		runs           []*v1alpha1.Run
		wantErr        bool
	}{{
		name:           "no-resolved-taskrun",
		embeddedStatus: config.DefaultEmbeddedStatus,
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-cancelled"},
			Spec: v1beta1.PipelineRunSpec{
				Status: v1beta1.PipelineRunSpecStatusCancelled,
			},
		},
	}, {
		name:           "one-taskrun",
		embeddedStatus: config.DefaultEmbeddedStatus,
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
		name:           "multiple-taskruns",
		embeddedStatus: config.DefaultEmbeddedStatus,
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
		name:           "multiple-runs",
		embeddedStatus: config.DefaultEmbeddedStatus,
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
		name:           "child-references-with-both",
		embeddedStatus: config.BothEmbeddedStatus,
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
		name:           "child-references-with-minimal",
		embeddedStatus: config.MinimalEmbeddedStatus,
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
		name:           "unknown-kind-on-child-references",
		embeddedStatus: config.MinimalEmbeddedStatus,
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
			}
			ctx, _ := ttesting.SetupFakeContext(t)
			cfg := config.NewStore(logtesting.TestLogger(t))
			cfg.OnConfigChanged(withCustomTasks(withEmbeddedStatus(newFeatureFlagsConfigMap(), tc.embeddedStatus)))
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
			}
		})
	}
}

func TestGetChildObjectsFromPRStatusForTaskNames(t *testing.T) {
	testCases := []struct {
		name             string
		embeddedStatus   string
		prStatus         v1beta1.PipelineRunStatus
		taskNames        sets.String
		expectedTRNames  []string
		expectedRunNames []string
		hasError         bool
	}{
		{
			name:           "single taskrun, default embedded",
			embeddedStatus: config.DefaultEmbeddedStatus,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
				},
			}},
			expectedTRNames:  []string{"t1"},
			expectedRunNames: nil,
			hasError:         false,
		}, {
			name:           "single run, default embedded",
			embeddedStatus: config.DefaultEmbeddedStatus,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				Runs: map[string]*v1beta1.PipelineRunRunStatus{
					"r1": {PipelineTaskName: "run-1"},
				},
			}},
			expectedTRNames:  nil,
			expectedRunNames: []string{"r1"},
			hasError:         false,
		}, {
			name:           "taskrun and run, default embedded",
			embeddedStatus: config.DefaultEmbeddedStatus,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
				},
				Runs: map[string]*v1beta1.PipelineRunRunStatus{
					"r1": {PipelineTaskName: "run-1"},
				},
			}},
			expectedTRNames:  []string{"t1"},
			expectedRunNames: []string{"r1"},
			hasError:         false,
		}, {
			name:           "taskrun and run, default embedded, just want taskrun",
			embeddedStatus: config.DefaultEmbeddedStatus,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
				},
				Runs: map[string]*v1beta1.PipelineRunRunStatus{
					"r1": {PipelineTaskName: "run-1"},
				},
			}},
			taskNames:        sets.NewString("task-1"),
			expectedTRNames:  []string{"t1"},
			expectedRunNames: nil,
			hasError:         false,
		}, {
			name:           "full embedded",
			embeddedStatus: config.FullEmbeddedStatus,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
				},
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "v1alpha1",
						Kind:       "Run",
					},
					Name:             "r1",
					PipelineTaskName: "run-1",
				}},
			}},
			expectedTRNames:  []string{"t1"},
			expectedRunNames: nil,
			hasError:         false,
		}, {
			name:           "both embedded",
			embeddedStatus: config.BothEmbeddedStatus,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
				},
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "v1alpha1",
						Kind:       "Run",
					},
					Name:             "r1",
					PipelineTaskName: "run-1",
				}},
			}},
			expectedTRNames:  nil,
			expectedRunNames: []string{"r1"},
			hasError:         false,
		}, {
			name:           "minimal embedded",
			embeddedStatus: config.MinimalEmbeddedStatus,
			prStatus: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"t1": {PipelineTaskName: "task-1"},
				},
				ChildReferences: []v1beta1.ChildStatusReference{{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "v1alpha1",
						Kind:       "Run",
					},
					Name:             "r1",
					PipelineTaskName: "run-1",
				}},
			}},
			expectedTRNames:  nil,
			expectedRunNames: []string{"r1"},
			hasError:         false,
		}, {
			name:           "unknown kind",
			embeddedStatus: config.MinimalEmbeddedStatus,
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
			expectedTRNames:  nil,
			expectedRunNames: nil,
			hasError:         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)
			cfg := config.NewStore(logtesting.TestLogger(t))
			cfg.OnConfigChanged(withCustomTasks(withEmbeddedStatus(newFeatureFlagsConfigMap(), tc.embeddedStatus)))
			ctx = cfg.ToContext(ctx)

			trNames, runNames, err := getChildObjectsFromPRStatusForTaskNames(ctx, tc.prStatus, tc.taskNames)

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
		})
	}
}
