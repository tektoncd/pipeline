/*
Copyright 2018 The Knative Authors

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
	"testing"

	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCancelPipelineRun(t *testing.T) {
	testCases := []struct {
		name          string
		pipelineRun   *v1alpha1.PipelineRun
		pipelineState []*resources.ResolvedPipelineRunTask
		taskRuns      []*v1alpha1.TaskRun
	}{{
		name: "no-resolved-taskrun",
		pipelineRun: tb.PipelineRun("test-pipeline-run-cancelled", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunCancelled,
			),
		),
	}, {
		name: "1-of-resolved-taskrun",
		pipelineRun: tb.PipelineRun("test-pipeline-run-cancelled", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunCancelled,
			),
		),
		pipelineState: []*resources.ResolvedPipelineRunTask{
			{TaskRunName: "t1", TaskRun: tb.TaskRun("t1", "foo")},
			{TaskRunName: "t2"},
		},
		taskRuns: []*v1alpha1.TaskRun{tb.TaskRun("t1", "foo")},
	}, {
		name: "resolved-taskruns",
		pipelineRun: tb.PipelineRun("test-pipeline-run-cancelled", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunCancelled,
			),
		),
		pipelineState: []*resources.ResolvedPipelineRunTask{
			{TaskRunName: "t1", TaskRun: tb.TaskRun("t1", "foo")},
			{TaskRunName: "t2", TaskRun: tb.TaskRun("t2", "foo")},
		},
		taskRuns: []*v1alpha1.TaskRun{tb.TaskRun("t1", "foo"), tb.TaskRun("t2", "foo")},
	}}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				PipelineRuns: []*v1alpha1.PipelineRun{tc.pipelineRun},
				TaskRuns:     tc.taskRuns,
			}
			c, _ := test.SeedTestData(t, d)
			err := cancelPipelineRun(tc.pipelineRun, tc.pipelineState, c.Pipeline)
			if err != nil {
				t.Fatal(err)
			}
			// This PipelineRun should still be complete and false, and the status should reflect that
			cond := tc.pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
			if cond.IsTrue() {
				t.Errorf("Expected PipelineRun status to be complete and false, but was %v", cond)
			}
			l, err := c.Pipeline.TektonV1alpha1().TaskRuns("foo").List(metav1.ListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range l.Items {
				if tr.Spec.Status != v1alpha1.TaskRunSpecStatusCancelled {
					t.Errorf("expected task %q to be marked as cancelled, was %q", tr.Name, tr.Spec.Status)
				}
			}
		})
	}
}
