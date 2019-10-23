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
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterTaskList_Inavlid_Namespace(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Namespaces: ns,
	})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	clustertask := Command(p)
	output, err := test.ExecuteCommand(clustertask, "list", "-n", "invalid")
	if err == nil {
		t.Errorf("Expected error for invalid namespace")
	}

	test.AssertOutput(t, "Error: namespaces \"invalid\" not found\n", output)
}

func TestClusterTaskList_Empty(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{
		Namespaces: ns,
	})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	clustertask := Command(p)
	output, err := test.ExecuteCommand(clustertask, "list", "-n", "foo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	test.AssertOutput(t, emptyMsg+"\n", output)
}

func TestClusterTaskListOnlyClusterTasks(t *testing.T) {
	clock := clockwork.NewFakeClock()

	clustertasks := []*v1alpha1.ClusterTask{
		tb.ClusterTask("guavas", cb.ClusterTaskCreationTime(clock.Now().Add(-1*time.Minute))),
		tb.ClusterTask("avocados", cb.ClusterTaskCreationTime(clock.Now().Add(-20*time.Second))),
		tb.ClusterTask("pineapple", cb.ClusterTaskCreationTime(clock.Now().Add(-512*time.Hour))),
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{ClusterTasks: clustertasks, Namespaces: ns})
	p := &test.Params{Tekton: cs.Pipeline, Clock: clock, Kube: cs.Kube}

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
