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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/testutil"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskListEmpty(t *testing.T) {
	cs, _ := test.SeedTestData(test.Data{})
	p := &testutil.TestParams{Client: cs.Pipeline}

	task := Command(p)
	output, err := testutil.ExecuteCommand(task, "list", "-n", "foo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := emptyMsg + "\n"
	if d := cmp.Diff(expected, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}

func TestTaskListOnlyTasks(t *testing.T) {
	clock := clockwork.NewFakeClock()
	tasks := []*v1alpha1.Task{
		tb.Task("tomatoes", "namespace", creationTime(clock.Now().Add(-1*time.Minute))),
		tb.Task("mangoes", "namespace", creationTime(clock.Now().Add(-20*time.Second))),
		tb.Task("bananas", "namespace", creationTime(clock.Now().Add(-512*time.Hour))),
	}

	cs, _ := test.SeedTestData(test.Data{Tasks: tasks})
	p := &testutil.TestParams{Client: cs.Pipeline, Clock: clock}

	task := Command(p)
	output, err := testutil.ExecuteCommand(task, "list", "-n", "namespace")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{
		"NAME       AGE",
		"tomatoes   1 minute ago",
		"mangoes    20 seconds ago",
		"bananas    3 weeks ago",
		"",
	}

	text := strings.Join(expected, "\n")
	if d := cmp.Diff(text, output); d != "" {
		t.Errorf("Unexpected output mismatch: %s", d)
	}
}

// TODO(vdemeester): push upstream
func creationTime(t time.Time) tb.TaskOp {
	return func(task *v1alpha1.Task) {
		task.CreationTimestamp = metav1.Time{Time: t}
	}
}
