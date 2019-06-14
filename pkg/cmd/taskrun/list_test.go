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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestListTaskRuns(t *testing.T) {
	now := time.Now()
	aMinute, _ := time.ParseDuration("1m")

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("tr1-1", "foo",
			tb.TaskRunLabel("tekton.dev/task", "bar"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("bar")),
			tb.TaskRunStatus(
				tb.Condition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
				tb.TaskRunStartTime(now),
				taskRunCompletionTime(now.Add(aMinute)),
			),
		),
		tb.TaskRun("tr2-1", "foo",
			tb.TaskRunLabel("tekton.dev/Task", "random"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("random")),
			tb.TaskRunStatus(
				tb.Condition(apis.Condition{
					Status: corev1.ConditionUnknown,
					Reason: resources.ReasonRunning,
				}),
				tb.TaskRunStartTime(now),
			),
		),
		tb.TaskRun("tr2-2", "foo",
			tb.TaskRunLabel("tekton.dev/Task", "random"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("random")),
			tb.TaskRunStatus(
				tb.Condition(apis.Condition{
					Status: corev1.ConditionFalse,
					Reason: resources.ReasonFailed,
				}),
				tb.TaskRunStartTime(now),
				taskRunCompletionTime(now.Add(aMinute)),
			),
		),
	}

	tests := []struct {
		name     string
		command  *cobra.Command
		args     []string
		expected []string
	}{
		{
			name:    "by Task name",
			command: command(trs, now),
			args:    []string{"list", "bar", "-n", "foo"},
			expected: []string{
				"NAME    STARTED      DURATION   STATUS      ",
				"tr1-1   1 hour ago   1 minute   Succeeded   ",
				"",
			},
		},
		{
			name:    "all in namespace",
			command: command(trs, now),
			args:    []string{"list", "-n", "foo"},
			expected: []string{
				"NAME    STARTED      DURATION   STATUS      ",
				"tr1-1   1 hour ago   1 minute   Succeeded   ",
				"tr2-1   1 hour ago   ---        Running     ",
				"tr2-2   1 hour ago   1 minute   Failed      ",
				"",
			},
		},
		{
			name:    "print by template",
			command: command(trs, now),
			args:    []string{"list", "-n", "foo", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}"},
			expected: []string{
				"tr1-1",
				"tr2-1",
				"tr2-2",
				"",
			},
		},
		{
			name:     "empty list",
			command:  command(trs, now),
			args:     []string{"list", "-n", "random"},
			expected: []string{emptyMsg, ""},
		},
	}

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			got, err := test.ExecuteCommand(td.command, td.args...)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if d := cmp.Diff(strings.Join(td.expected, "\n"), got); d != "" {
				t.Errorf("Unexpected output mismatch: \n%s\n", d)
			}
		})
	}
}

func command(trs []*v1alpha1.TaskRun, now time.Time) *cobra.Command {
	// fake clock advanced by 1 hour
	clock := clockwork.NewFakeClockAt(now)
	clock.Advance(time.Duration(60) * time.Minute)

	cs, _ := pipelinetest.SeedTestData(pipelinetest.Data{TaskRuns: trs})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	return Command(p)
}

func taskRunCompletionTime(ct time.Time) tb.TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		s.CompletionTime = &metav1.Time{Time: ct}
	}
}
