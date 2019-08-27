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
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPipelineDelete_Empty(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	pipeline := Command(p)
	_, err := test.ExecuteCommand(pipeline, "rm", "bar")
	if err == nil {
		t.Errorf("Error expected here")
	}
	expected := "Failed to delete pipeline \"bar\": pipelines.tekton.dev \"bar\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestPipelineDelete_WithParams(t *testing.T) {
	clock := clockwork.NewFakeClock()

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Pipelines: []*v1alpha1.Pipeline{
			tb.Pipeline("pipeline", "ns",
				// created  5 minutes back
				cb.PipelineCreationTimestamp(clock.Now().Add(-5*time.Minute)),
			),
		},
	})

	p := &test.Params{Tekton: cs.Pipeline}
	pipeline := Command(p)
	out, _ := test.ExecuteCommand(pipeline, "rm", "pipeline", "-n", "ns", "-f")
	expected := "Pipeline deleted: pipeline\n"
	test.AssertOutput(t, expected, out)
}
