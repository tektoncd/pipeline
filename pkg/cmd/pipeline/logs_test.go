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

	"github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"

	tu "github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"

	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	// disable color output for all prompts to simplify testing
	core.DisableColor = true
}

var (
	pipelineName = "output-pipeline"
	prName       = "output-pipeline-run"
	prName2      = "output-pipeline-run-2"
	ns           = "namespace"
)

func TestInteractiveAskPAndPR(t *testing.T) {

	clock := clockwork.NewFakeClock()

	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline(pipelineName, ns,
				// created  15 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-15*time.Minute)),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun(prName, ns,
				cb.PipelineRunCreationTimestamp(clock.Now().Add(-10*time.Minute)),
				tb.PipelineRunLabel("tekton.dev/pipeline", pipelineName),
				tb.PipelineRunSpec(pipelineName),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run started 5 minutes ago
					tb.PipelineRunStartTime(clock.Now().Add(-5*time.Minute)),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
				),
			),
			tb.PipelineRun(prName2, ns,
				cb.PipelineRunCreationTimestamp(clock.Now().Add(-8*time.Minute)),
				tb.PipelineRunLabel("tekton.dev/pipeline", pipelineName),
				tb.PipelineRunSpec(pipelineName),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run started 3 minutes ago
					tb.PipelineRunStartTime(clock.Now().Add(-3*time.Minute)),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
				),
			),
		},
	})

	tests := []promptTest{
		{
			name:    "basic interaction",
			cmdArgs: []string{},

			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select pipeline :")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyArrowDown))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("output-pipeline")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select pipelinerun :")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyArrowDown))
				if err != nil {
					return err
				}
				_, err = c.ExpectString(prName + " started 5 minutes ago")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyArrowDown))
				if err != nil {
					return err
				}
				_, err = c.ExpectString(prName2 + " started 3 minutes ago")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyArrowUp))
				if err != nil {
					return err
				}
				_, err = c.ExpectString(prName + " started 5 minutes ago")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				return nil
			},
		},
	}
	opts := logOpts(prName, ns, cs)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts.RunPromptTest(t, test)
		})
	}
}

func TestInteractiveAskPR(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline(pipelineName, ns,
				// created  15 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-15*time.Minute)),
			),
		},
		PipelineRuns: []*v1alpha1.PipelineRun{

			tb.PipelineRun(prName, ns,
				cb.PipelineRunCreationTimestamp(clock.Now().Add(-10*time.Minute)),
				tb.PipelineRunLabel("tekton.dev/pipeline", pipelineName),
				tb.PipelineRunSpec(pipelineName),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run started 5 minutes ago
					tb.PipelineRunStartTime(clock.Now().Add(-5*time.Minute)),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
				),
			),
			tb.PipelineRun(prName2, ns,
				cb.PipelineRunCreationTimestamp(clock.Now().Add(-8*time.Minute)),
				tb.PipelineRunLabel("tekton.dev/pipeline", pipelineName),
				tb.PipelineRunSpec(pipelineName),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionTrue,
						Reason: resources.ReasonSucceeded,
					}),
					// pipeline run started 3 minutes ago
					tb.PipelineRunStartTime(clock.Now().Add(-3*time.Minute)),
					// takes 10 minutes to complete
					cb.PipelineRunCompletionTime(clock.Now().Add(10*time.Minute)),
				),
			),
		},
	})

	tests := []promptTest{
		{
			name:    "basic interaction",
			cmdArgs: []string{pipelineName},

			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select pipelinerun :")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyArrowDown))
				if err != nil {
					return err
				}
				_, err = c.ExpectString(prName + " started 5 minutes ago")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyArrowDown))
				if err != nil {
					return err
				}
				_, err = c.ExpectString(prName2 + "started 3 minutes ago")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				return nil
			},
		},
	}
	opts := logOpts(prName, ns, cs)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts.RunPromptTest(t, test)
		})
	}
}

func logOpts(name string, ns string, cs test.Clients) *logOptions {
	p := tu.Params{
		Kube:   cs.Kube,
		Tekton: cs.Pipeline,
	}
	p.SetNamespace(ns)
	logOp := logOptions{
		runName: name,
		params:  &p,
	}

	return &logOp
}
