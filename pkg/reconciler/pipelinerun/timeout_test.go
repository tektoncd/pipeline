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

package pipelinerun

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	_ "github.com/tektoncd/pipeline/pkg/pipelinerunmetrics/fake" // Make sure the pipelinerunmetrics are setup
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestTimeoutPipelineRun(t *testing.T) {
	testCases := []struct {
		name                  string
		embeddedStatus        string
		useV1Beta1CustomTasks bool
		pipelineRun           *v1beta1.PipelineRun
		taskRuns              []*v1beta1.TaskRun
		customRuns            []*v1beta1.CustomRun
		runs                  []*v1alpha1.Run
		wantErr               bool
	}{{
		name:           "no-resolved-taskrun",
		embeddedStatus: config.DefaultEmbeddedStatus,
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
		},
	}, {
		name:           "one-taskrun",
		embeddedStatus: config.DefaultEmbeddedStatus,
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
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
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
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
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
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
		name:                  "multiple-runs-beta-custom-tasks",
		embeddedStatus:        config.DefaultEmbeddedStatus,
		useV1Beta1CustomTasks: true,
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				Runs: map[string]*v1beta1.PipelineRunRunStatus{
					"t1": {PipelineTaskName: "task-1"},
					"t2": {PipelineTaskName: "task-2"},
				},
			}},
		},
		customRuns: []*v1beta1.CustomRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "t1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "t2"}},
		},
	}, {
		name:           "child-references-with-both",
		embeddedStatus: config.BothEmbeddedStatus,
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{
					{
						TypeMeta:         runtime.TypeMeta{Kind: pipeline.TaskRunControllerName},
						Name:             "t1",
						PipelineTaskName: "task-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: pipeline.TaskRunControllerName},
						Name:             "t2",
						PipelineTaskName: "task-2",
					},
					{
						TypeMeta: runtime.TypeMeta{
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
							Kind:       pipeline.RunControllerName,
						},
						Name:             "r1",
						PipelineTaskName: "run-1",
					},
					{
						TypeMeta: runtime.TypeMeta{
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
							Kind:       pipeline.RunControllerName,
						},
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
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{
					{
						TypeMeta:         runtime.TypeMeta{Kind: pipeline.TaskRunControllerName},
						Name:             "t1",
						PipelineTaskName: "task-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: pipeline.TaskRunControllerName},
						Name:             "t2",
						PipelineTaskName: "task-2",
					},
					{
						TypeMeta: runtime.TypeMeta{
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
							Kind:       pipeline.RunControllerName,
						},
						Name:             "r1",
						PipelineTaskName: "run-1",
					},
					{
						TypeMeta: runtime.TypeMeta{
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
							Kind:       pipeline.RunControllerName,
						},
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
		name:                  "child-references-with-minimal-beta-custom-tasks",
		embeddedStatus:        config.MinimalEmbeddedStatus,
		useV1Beta1CustomTasks: true,
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
			Status: v1beta1.PipelineRunStatus{PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				ChildReferences: []v1beta1.ChildStatusReference{
					{
						TypeMeta:         runtime.TypeMeta{Kind: pipeline.TaskRunControllerName},
						Name:             "t1",
						PipelineTaskName: "task-1",
					},
					{
						TypeMeta:         runtime.TypeMeta{Kind: pipeline.TaskRunControllerName},
						Name:             "t2",
						PipelineTaskName: "task-2",
					},
					{
						TypeMeta: runtime.TypeMeta{
							APIVersion: v1beta1.SchemeGroupVersion.String(),
							Kind:       pipeline.CustomRunControllerName,
						},
						Name:             "r1",
						PipelineTaskName: "run-1",
					},
					{
						TypeMeta: runtime.TypeMeta{
							APIVersion: v1beta1.SchemeGroupVersion.String(),
							Kind:       pipeline.CustomRunControllerName,
						},
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
		customRuns: []*v1beta1.CustomRun{
			{ObjectMeta: metav1.ObjectMeta{Name: "r1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "r2"}},
		},
	}, {
		name:           "unknown-kind-on-child-references",
		embeddedStatus: config.MinimalEmbeddedStatus,
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-timedout"},
			Spec:       v1beta1.PipelineRunSpec{},
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
				CustomRuns:   tc.customRuns,
				Runs:         tc.runs,
			}
			ctx, _ := ttesting.SetupFakeContext(t)
			cfg := config.NewStore(logtesting.TestLogger(t))
			cm := withEmbeddedStatus(newFeatureFlagsConfigMap(), tc.embeddedStatus)
			if tc.useV1Beta1CustomTasks {
				cm = withCustomTaskVersion(cm, config.CustomTaskVersionBeta)
			}
			cfg.OnConfigChanged(cm)
			ctx = cfg.ToContext(ctx)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, _ := test.SeedTestData(t, ctx, d)

			err := timeoutPipelineRun(ctx, logtesting.TestLogger(t), tc.pipelineRun, c.Pipeline)
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
							t.Errorf("expected task %q to be marked as timed out, was %q", tr.Name, tr.Spec.Status)
						}
						if tr.Spec.StatusMessage != v1beta1.TaskRunCancelledByPipelineTimeoutMsg {
							t.Errorf("expected task %s to have the timeout-specific status message, was %s", tr.Name, tr.Spec.StatusMessage)
						}
					}
				}
				if tc.customRuns != nil {
					for _, expectedCustomRun := range tc.customRuns {
						r, err := c.Pipeline.TektonV1beta1().CustomRuns("").Get(ctx, expectedCustomRun.Name, metav1.GetOptions{})
						if err != nil {
							t.Fatalf("couldn't get expected CustomRun %s, got error %s", expectedCustomRun.Name, err)
						}
						if r.Spec.Status != v1beta1.CustomRunSpecStatusCancelled {
							t.Errorf("expected task %q to be marked as cancelled, was %q", r.Name, r.Spec.Status)
						}
						if r.Spec.StatusMessage != v1beta1.CustomRunCancelledByPipelineTimeoutMsg {
							t.Errorf("expected run %s to have the timeout-specific status message, was %s", r.Name, r.Spec.StatusMessage)
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
						if r.Spec.StatusMessage != v1alpha1.RunCancelledByPipelineTimeoutMsg {
							t.Errorf("expected run %s to have the timeout-specific status message, was %s", r.Name, r.Spec.StatusMessage)
						}
					}
				}
			}
		})
	}
}
