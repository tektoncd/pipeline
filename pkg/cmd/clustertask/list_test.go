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

package clustertask

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	tu "github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestClusterTaskList_Empty(t *testing.T) {
	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{})
	p := &test.Params{Tekton: cs.Pipeline}

	clustertask := Command(p)
	output, err := test.ExecuteCommand(clustertask, "list", "-n", "foo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	tu.AssertOutput(t, emptyMsg+"\n", output)
}

func TestClusterTaskListOnlyClusterTasks(t *testing.T) {
	clock := clockwork.NewFakeClock()
	clustertasks := []*v1alpha1.ClusterTask{
		tb.ClusterTask("guavas", cb.ClusterTaskCreationTime(clock.Now().Add(-1*time.Minute))),
		tb.ClusterTask("avocados", cb.ClusterTaskCreationTime(clock.Now().Add(-20*time.Second))),
		tb.ClusterTask("pineapple", cb.ClusterTaskCreationTime(clock.Now().Add(-512*time.Hour))),
	}

	cs, _ := pipelinetest.SeedTestData(t, pipelinetest.Data{ClusterTasks: clustertasks})
	p := &test.Params{Tekton: cs.Pipeline, Clock: clock}

	clustertask := Command(p)
	output, err := test.ExecuteCommand(clustertask, "list")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"NAME        AGE",
		"guavas      1 minute ago",
		"avocados    20 seconds ago",
		"pineapple   3 weeks ago",
		"",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}
