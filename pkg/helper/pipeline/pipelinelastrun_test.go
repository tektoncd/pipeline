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

package pipeline

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestPipelinesList_latest_run(t *testing.T) {
	clock := clockwork.NewFakeClock()
	//  Time --->
	//  |---5m ---|------------ ││--││------------- ---│--│
	//	now      pipeline       ││  │`secondRun stated │  `*first*RunCompleted
	//                          ││  `secondRun         `*second*RunCompleted
	//	                        │`firstRun started
	//	                        `firstRun
	// NOTE: firstRun completed **after** second but latest should still be
	// second run based on creationTimestamp

	var (
		pipelineCreated = clock.Now().Add(-5 * time.Minute)
		runDuration     = 5 * time.Minute

		firstRunCreated   = clock.Now().Add(10 * time.Minute)
		firstRunStarted   = firstRunCreated.Add(2 * time.Second)
		firstRunCompleted = firstRunStarted.Add(2 * runDuration) // take twice as long

		secondRunCreated   = firstRunCreated.Add(1 * time.Minute)
		secondRunStarted   = secondRunCreated.Add(2 * time.Second)
		secondRunCompleted = secondRunStarted.Add(runDuration) // takes less thus completes
	)

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(pipelineCreated),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun("pipeline-run-1", "ns",
				cb.PipelineRunCreationTimestamp(firstRunCreated),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.PipelineRunStartTime(firstRunStarted),
					cb.PipelineRunCompletionTime(firstRunCompleted),
				),
			),
			tb.PipelineRun("pipeline-run-2", "ns",
				cb.PipelineRunCreationTimestamp(secondRunCreated),
				tb.PipelineRunLabel("tekton.dev/pipeline", "pipeline"),
				tb.PipelineRunSpec("pipeline"),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					tb.PipelineRunStartTime(secondRunStarted),
					cb.PipelineRunCompletionTime(secondRunCompleted),
				),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}
	client, err := p.Clients()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	lastRun, err := LastRun(client.Tekton, "pipeline", "ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	test.AssertOutput(t, "pipeline-run-2", lastRun.Name)
}
