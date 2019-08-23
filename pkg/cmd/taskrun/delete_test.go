// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package taskrun

import (
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestTaskRunDelete_Empty(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	tr := Command(p)
	_, err := test.ExecuteCommand(tr, "rm", "bar")
	if err == nil {
		t.Errorf("Error expected here")
	}
	expected := "Failed to delete taskrun \"bar\": taskruns.tekton.dev \"bar\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestTaskRunDelete_WithParams(t *testing.T) {
	clock := clockwork.NewFakeClock()
	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr0-1", "ns",
			tb.TaskRunLabel("tekton.dev/task", "random"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("random")),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{TaskRuns: trs})
	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}
	tr := Command(p)
	out, _ := test.ExecuteCommand(tr, "rm", "tr0-1", "-n", "ns")
	expected := "TaskRun deleted: tr0-1\n"
	test.AssertOutput(t, expected, out)
}
