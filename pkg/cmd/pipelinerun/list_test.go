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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestListPipelineRuns(t *testing.T) {
	clock := clockwork.NewFakeClock()
	runDuration := 1 * time.Minute

	pr1Started := clock.Now().Add(10 * time.Second)
	pr2Started := clock.Now().Add(-2 * time.Hour)
	pr3Started := clock.Now().Add(-1 * time.Hour)

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("pr0-1", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "random"),
			tb.PipelineRunStatus(),
		),
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
		tb.PipelineRun("pr3-1", "namespace",
			tb.PipelineRunLabel("tekton.dev/pipeline", "random"),
			tb.PipelineRunStatus(),
		),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace",
			},
		},
	}

	tests := []struct {
		name      string
		command   *cobra.Command
		args      []string
		wantError bool
		expected  []string
	}{
		{
			name:      "Invalid namespace",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "-n", "invalid"},
			wantError: true,
			expected:  []string{"Error: namespaces \"invalid\" not found\n"},
		},
		{
			name:      "by pipeline name",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "pipeline", "-n", "namespace"},
			wantError: false,
			expected: []string{
				"NAME    STARTED          DURATION   STATUS      ",
				"pr1-1   59 minutes ago   1 minute   Succeeded   ",
				"",
			},
		},
		{
			name:      "all in namespace",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "-n", "namespace"},
			wantError: false,
			expected: []string{
				"NAME    STARTED          DURATION   STATUS               ",
				"pr0-1   ---              ---        ---                  ",
				"pr3-1   ---              ---        ---                  ",
				"pr1-1   59 minutes ago   1 minute   Succeeded            ",
				"pr2-2   2 hours ago      1 minute   Failed               ",
				"pr2-1   3 hours ago      ---        Succeeded(Running)   ",
				"",
			},
		},
		{
			name:      "by template",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "-n", "namespace", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}"},
			wantError: false,
			expected: []string{
				"pr0-1",
				"pr3-1",
				"pr1-1",
				"pr2-2",
				"pr2-1",
				"",
			},
		},
		{
			name:      "limit pipelineruns returned to 1",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "-n", "namespace", "--limit", fmt.Sprintf("%d", 1)},
			wantError: false,
			expected: []string{
				"NAME    STARTED   DURATION   STATUS   ",
				"pr0-1   ---       ---        ---      ",
				"",
			},
		},
		{
			name:      "limit pipelineruns negative case",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "-n", "namespace", "--limit", fmt.Sprintf("%d", -1)},
			wantError: false,
			expected: []string{
				"",
			},
		},
		{
			name:      "limit pipelineruns greater than maximum case",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "-n", "namespace", "--limit", fmt.Sprintf("%d", 7)},
			wantError: false,
			expected: []string{
				"NAME    STARTED          DURATION   STATUS               ",
				"pr0-1   ---              ---        ---                  ",
				"pr3-1   ---              ---        ---                  ",
				"pr1-1   59 minutes ago   1 minute   Succeeded            ",
				"pr2-2   2 hours ago      1 minute   Failed               ",
				"pr2-1   3 hours ago      ---        Succeeded(Running)   ",
				"",
			},
		},
		{
			name:      "limit pipelineruns with output flag set",
			command:   command(t, prs, clock.Now(), ns),
			args:      []string{"list", "-n", "namespace", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}", "--limit", fmt.Sprintf("%d", 2)},
			wantError: false,
			expected: []string{
				"pr0-1",
				"pr3-1",
				"",
			},
		},
	}

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			got, err := test.ExecuteCommand(td.command, td.args...)

			if !td.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			test.AssertOutput(t, strings.Join(td.expected, "\n"), got)
		})
	}
}

func TestListPipeline_empty(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pipeline := Command(p)
	output, err := test.ExecuteCommand(pipeline, "list", "-n", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	test.AssertOutput(t, emptyMsg+"\n", output)
}

func command(t *testing.T, prs []*v1alpha1.PipelineRun, now time.Time, ns []*corev1.Namespace) *cobra.Command {
	// fake clock advanced by 1 hour
	clock := clockwork.NewFakeClockAt(now)
	clock.Advance(time.Duration(60) * time.Minute)

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Namespaces: ns})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

	return Command(p)
}
