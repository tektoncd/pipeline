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
	"errors"
	"testing"

	"github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stest "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
)

func TestTaskRunCancel(t *testing.T) {
	seeds := make([]pipelinetest.Clients, 0)
	failures := make([]pipelinetest.Clients, 0)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("taskrun-1", "ns",
			tb.TaskRunLabel("tekton.dev/task", "task"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("task")),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonRunning,
				}),
			),
		),
		tb.TaskRun("taskrun-2", "ns",
			tb.TaskRunLabel("tekton.dev/task", "failure-task"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("failure-task")),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
			),
		),
	}

	trs2 := []*v1alpha1.TaskRun{
		tb.TaskRun("failure-taskrun-1", "ns",
			tb.TaskRunLabel("tekton.dev/task", "failure-task"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("failure-task")),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonFailed,
				}),
			),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{TaskRuns: trs, Namespaces: ns})
	cs2, _ := test.SeedTestData(t, pipelinetest.Data{TaskRuns: trs2, Namespaces: ns})

	cs2.Pipeline.PrependReactor("update", "taskruns", func(action k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("test error")
	})

	seeds = append(seeds, cs)
	failures = append(failures, cs2)

	testParams := []struct {
		name      string
		command   []string
		input     pipelinetest.Clients
		wantError bool
		want      string
	}{
		{
			name:      "Invalid namespace",
			command:   []string{"cancel", "taskrun-1", "-n", "invalid"},
			input:     seeds[0],
			wantError: true,
			want:      "namespaces \"invalid\" not found",
		},
		{
			name:      "Canceling taskrun successfully",
			command:   []string{"cancel", "taskrun-1", "-n", "ns"},
			input:     seeds[0],
			wantError: false,
			want:      "TaskRun cancelled: taskrun-1\n",
		},
		{
			name:      "Not found taskrun",
			command:   []string{"cancel", "nonexistent", "-n", "ns"},
			input:     seeds[0],
			wantError: true,
			want:      "failed to find taskrun: nonexistent",
		},
		{
			name:      "Failed canceling taskrun",
			command:   []string{"cancel", "failure-taskrun-1", "-n", "ns"},
			input:     failures[0],
			wantError: true,
			want:      "failed to cancel taskrun \"failure-taskrun-1\": test error",
		},
		{
			name:      "Failed canceling taskrun that succeeded",
			command:   []string{"cancel", "taskrun-2", "-n", "ns"},
			input:     seeds[0],
			wantError: true,
			want:      "failed to cancel taskrun taskrun-2: taskrun has already finished execution",
		},
	}

	for _, tp := range testParams {
		t.Run(tp.name, func(t *testing.T) {
			p := &test.Params{Tekton: tp.input.Pipeline, Kube: tp.input.Kube}
			taskrun := Command(p)

			out, err := test.ExecuteCommand(taskrun, tp.command...)
			if tp.wantError {
				if err == nil {
					t.Errorf("error expected here")
				}
				test.AssertOutput(t, tp.want, err.Error())
			} else {
				if err != nil {
					t.Errorf("unexpected Error")
				}
				test.AssertOutput(t, tp.want, out)
			}
		})
	}
}
