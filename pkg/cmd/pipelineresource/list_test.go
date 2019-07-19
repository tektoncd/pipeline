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

package pipelineresource

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/test"
	tu "github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPipelineResources(t *testing.T) {

	pres := []*v1alpha1.PipelineResource{
		tb.PipelineResource("test-1", "test-ns-1",
			tb.PipelineResourceSpec("image",
				tb.PipelineResourceSpecParam("URL", "quey.io/tekton/controller"),
			),
		),
		tb.PipelineResource("test-2", "test-ns-1",
			tb.PipelineResourceSpec("git",
				tb.PipelineResourceSpecParam("url", "git@github.com:tektoncd/cli.git"),
			),
		),
		tb.PipelineResource("test-3", "test-ns-1",
			tb.PipelineResourceSpec("image"),
		),
		tb.PipelineResource("test-4", "test-ns-2",
			tb.PipelineResourceSpec("image",
				tb.PipelineResourceSpecParam("URL", "quey.io/tekton/webhook"),
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
			name:    "Multiple pipeline resources",
			command: command(t, pres),
			args:    []string{"list", "-n", "test-ns-1"},
			expected: []string{
				"NAME     TYPE    DETAILS",
				"test-1   image   URL: quey.io/tekton/controller",
				"test-2   git     url: git@github.com:tektoncd/cli.git",
				"test-3   image   ---",
				"",
			},
		},
		{
			name:    "Single pipeline resource",
			command: command(t, pres),
			args:    []string{"list", "-n", "test-ns-2"},
			expected: []string{
				"NAME     TYPE    DETAILS",
				"test-4   image   URL: quey.io/tekton/webhook",
				"",
			},
		},
		{
			name:    "By template",
			command: command(t, pres),
			args:    []string{"list", "-n", "test-ns-1", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}"},
			expected: []string{
				"test-1",
				"test-2",
				"test-3",
				"",
			},
		},
	}

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			out, err := test.ExecuteCommand(td.command, td.args...)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			tu.AssertOutput(t, strings.Join(td.expected, "\n"), out)
		})
	}

}

func TestPipelineResource_empty(t *testing.T) {
	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}
	pipelineresource := Command(p)
	out, _ := test.ExecuteCommand(pipelineresource, "list", "-n", "test-ns-3")
	tu.AssertOutput(t, msgNoPREsFound+"\n", out)
}

func command(t *testing.T, pres []*v1alpha1.PipelineResource) *cobra.Command {
	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{PipelineResources: pres})
	p := &test.Params{Tekton: cs.Pipeline}
	return Command(p)
}
