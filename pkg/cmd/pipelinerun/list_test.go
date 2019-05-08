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

package pipelinerun

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/test"
	tu "github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

func TestListPipelineRuns(t *testing.T) {
	clock := clockwork.NewFakeClock()
	runDuration := 1 * time.Minute

	pr1Started := clock.Now().Add(10 * time.Second)
	pr2Started := clock.Now().Add(-2 * time.Hour)
	pr3Started := clock.Now().Add(-450 * time.Hour)

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("pr1-1", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
				tb.PipelineRunStartTime(pr1Started),
				cb.PipelineRunCompletionTime(pr1Started.Add(runDuration)),
			),
		),
		tb.PipelineRun("pr2-1", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "random"),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonRunning,
				}),
				tb.PipelineRunStartTime(pr2Started),
			),
		),
		tb.PipelineRun("pr2-2", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "random"),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionFalse,
					Reason: resources.ReasonFailed,
				}),
				tb.PipelineRunStartTime(pr3Started),
				cb.PipelineRunCompletionTime(pr3Started.Add(runDuration)),
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
			name:    "by pipeline name",
			command: command(prs, clock.Now()),
			args:    []string{"list", "pipeline", "-n", "namespace"},
			expected: []string{
				"NAME    STARTED          DURATION   STATUS      ",
				"pr1-1   59 minutes ago   1 minute   Succeeded   ",
				"",
			},
		},
		{
			name:    "all in namespace",
			command: command(prs, clock.Now()),
			args:    []string{"list", "-n", "namespace"},
			expected: []string{
				"NAME    STARTED          DURATION   STATUS               ",
				"pr1-1   59 minutes ago   1 minute   Succeeded            ",
				"pr2-1   3 hours ago      ---        Succeeded(Running)   ",
				"pr2-2   2 weeks ago      1 minute   Failed               ",
				"",
			},
		},
		{
			name:    "by template",
			command: command(prs, clock.Now()),
			args:    []string{"list", "-n", "namespace", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}"},
			expected: []string{
				"pr1-1",
				"pr2-1",
				"pr2-2",
				"",
			},
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

func TestListPipeline_empty(t *testing.T) {
	cs, _ := pipelinetest.SeedTestData(pipelinetest.Data{})
	p := &tu.Params{Tekton: cs.Pipeline}

	pipeline := Command(p)
	output, err := tu.ExecuteCommand(pipeline, "list", "-n", "ns")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := msgNoPRsFound + "\n"
	if d := cmp.Diff(expected, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}

func command(prs []*v1alpha1.PipelineRun, now time.Time) *cobra.Command {
	// fake clock advanced by 1 hour
	clock := clockwork.NewFakeClockAt(now)
	clock.Advance(time.Duration(60) * time.Minute)

	cs, _ := pipelinetest.SeedTestData(pipelinetest.Data{PipelineRuns: prs})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	return Command(p)
}
