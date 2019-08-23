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

package task

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

func TestTaskDelete_Empty(t *testing.T) {
	cs, _ := test.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	task := Command(p)
	_, err := test.ExecuteCommand(task, "rm", "bar")
	if err == nil {
		t.Errorf("Error expected here")
	}
	expected := "Failed to delete task \"bar\": tasks.tekton.dev \"bar\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestTaskDelete_WithParams(t *testing.T) {
	clock := clockwork.NewFakeClock()
	tasks := []*v1alpha1.Task{
		tb.Task("tomatoes", "ns", cb.TaskCreationTime(clock.Now().Add(-1*time.Minute))),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Tasks: tasks})
	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}
	task := Command(p)
	out, _ := test.ExecuteCommand(task, "rm", "tomatoes", "-n", "ns")
	expected := "Task deleted: tomatoes\n"
	test.AssertOutput(t, expected, out)
}
