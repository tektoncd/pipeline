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
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestTaskDelete(t *testing.T) {
	clock := clockwork.NewFakeClock()

	seeds := make([]pipelinetest.Clients, 0)
	for i := 0; i < 3; i++ {
		tasks := []*v1alpha1.Task{
			tb.Task("tomatoes", "ns", cb.TaskCreationTime(clock.Now().Add(-1*time.Minute))),
		}
		cs, _ := test.SeedTestData(t, pipelinetest.Data{Tasks: tasks})
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
			command:     []string{"rm", "tomatoes", "-n", "ns", "-f"},
			input:       seeds[0],
			inputStream: nil,
			wantError:   false,
			want:        "Task deleted: tomatoes\n",
		},
		{
			name:        "With force delete flag",
			command:     []string{"rm", "tomatoes", "-n", "ns", "--force"},
			input:       seeds[1],
			inputStream: nil,
			wantError:   false,
			want:        "Task deleted: tomatoes\n",
		},
		{
			name:        "Without force delete flag, reply no",
			command:     []string{"rm", "tomatoes", "-n", "ns"},
			input:       seeds[2],
			inputStream: strings.NewReader("n"),
			wantError:   true,
			want:        "Canceled deleting task \"tomatoes\"",
		},
		{
			name:        "Without force delete flag, reply yes",
			command:     []string{"rm", "tomatoes", "-n", "ns"},
			input:       seeds[2],
			inputStream: strings.NewReader("y"),
			wantError:   false,
			want:        "Are you sure you want to delete task \"tomatoes\" (y/n): Task deleted: tomatoes\n",
		},
		{
			name:        "Remove non existent resource",
			command:     []string{"rm", "nonexistent", "-n", "ns"},
			input:       seeds[2],
			inputStream: strings.NewReader("y"),
			wantError:   true,
			want:        "Failed to delete task \"nonexistent\": tasks.tekton.dev \"nonexistent\" not found",
		},
	}

	for _, tp := range testParams {
		t.Run(tp.name, func(t *testing.T) {
			p := &test.Params{Tekton: tp.input.Pipeline}
			task := Command(p)

			if tp.inputStream != nil {
				task.SetIn(tp.inputStream)
			}

			out, err := test.ExecuteCommand(task, tp.command...)
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
