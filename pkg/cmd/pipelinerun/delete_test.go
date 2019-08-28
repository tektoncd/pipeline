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
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineRunDelete(t *testing.T) {
	clock := clockwork.NewFakeClock()

	seeds := make([]pipelinetest.Clients, 0)
	for i := 0; i < 3; i++ {
		cs, _ := test.SeedTestData(t, pipelinetest.Data{
			Pipelines: []*v1alpha1.Pipeline{
				tb.Pipeline("pipeline", "ns",
					// created  5 minutes back
					cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
				),
			},
			PipelineRuns: []*v1alpha1.PipelineRun{
				tb.PipelineRun("pipeline-run-1", "ns",
					cb.PipelineRunCreationTimestamp(clock.Now()),
					tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
					tb.PipelineRunSpec("pipeline"),
					tb.PipelineRunStatus(
						tb.PipelineRunStatusCondition(apis.Condition{
							Status: corev1.ConditionTrue,
							Reason: resources.ReasonSucceeded,
						}),
						// pipeline run starts now
						tb.PipelineRunStartTime(clock.Now()),
						// takes 10 minutes to complete
						cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
					),
				),
			},
		})
		seeds = append(seeds, cs)
	}

	testParams := []struct {
		name        string
		command     []string
		input       pipelinetest.Clients
		inputStream io.Reader
		wantError   bool
		want        string
	}{
		{
			name:        "With force delete flag (shorthand)",
			command:     []string{"rm", "pipeline-run-1", "-n", "ns", "-f"},
			input:       seeds[0],
			inputStream: nil,
			wantError:   false,
			want:        "PipelineRun deleted: pipeline-run-1\n",
		},
		{
			name:        "With force delete flag",
			command:     []string{"rm", "pipeline-run-1", "-n", "ns", "--force"},
			input:       seeds[1],
			inputStream: nil,
			wantError:   false,
			want:        "PipelineRun deleted: pipeline-run-1\n",
		},
		{
			name:        "Without force delete flag, reply no",
			command:     []string{"rm", "pipeline-run-1", "-n", "ns"},
			input:       seeds[2],
			inputStream: strings.NewReader("n"),
			wantError:   true,
			want:        "Canceled deleting pipelinerun \"pipeline-run-1\"",
		},
		{
			name:        "Without force delete flag, reply yes",
			command:     []string{"rm", "pipeline-run-1", "-n", "ns"},
			input:       seeds[2],
			inputStream: strings.NewReader("y"),
			wantError:   false,
			want:        "Make sure you really want to delete pipelinerun \"pipeline-run-1\" (y/n): PipelineRun deleted: pipeline-run-1\n",
		},
		{
			name:        "Remove non existent resource",
			command:     []string{"rm", "nonexistent", "-n", "ns"},
			input:       seeds[2],
			inputStream: strings.NewReader("y"),
			wantError:   true,
			want:        "Failed to delete pipelinerun \"nonexistent\": pipelineruns.tekton.dev \"nonexistent\" not found",
		},
	}

	for _, tp := range testParams {
		t.Run(tp.name, func(t *testing.T) {
			p := &test.Params{Tekton: tp.input.Pipeline}
			pipelinerun := Command(p)

			if tp.inputStream != nil {
				pipelinerun.SetIn(tp.inputStream)
			}

			out, err := test.ExecuteCommand(pipelinerun, tp.command...)
			if tp.wantError {
				if err == nil {
					t.Errorf("Error expected here")
				}
				test.AssertOutput(t, tp.want, err.Error())
			} else {
				if err != nil {
					t.Errorf("Unexpected Error")
				}
				test.AssertOutput(t, tp.want, out)
			}
		})
	}
}
