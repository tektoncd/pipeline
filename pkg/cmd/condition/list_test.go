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

package condition

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"k8s.io/apimachinery/pkg/runtime"
	k8stest "k8s.io/client-go/testing"
)

func TestConditionList(t *testing.T) {
	clock := clockwork.NewFakeClock()

	seeds := make([]pipelinetest.Clients, 0)

	// Testdata pattern1.
	conditions := []*v1alpha1.Condition{
		tb.Condition("condition1", "ns", cb.ConditionCreationTime(clock.Now().Add(-1*time.Minute))),
		tb.Condition("condition2", "ns", cb.ConditionCreationTime(clock.Now().Add(-20*time.Second))),
		tb.Condition("condition3", "ns", cb.ConditionCreationTime(clock.Now().Add(-512*time.Hour))),
	}
	s, _ := test.SeedTestData(t, pipelinetest.Data{Conditions: conditions})

	// Testdata pattern2.
	conditions2 := []*v1alpha1.Condition{
		tb.Condition("condition1", "ns", cb.ConditionCreationTime(clock.Now().Add(-1*time.Minute))),
	}
	s2, _ := test.SeedTestData(t, pipelinetest.Data{Conditions: conditions2})
	s2.Pipeline.PrependReactor("list", "conditions", func(action k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("test error")
	})

	seeds = append(seeds, s)
	seeds = append(seeds, s2)

	testParams := []struct {
		name      string
		command   []string
		input     pipelinetest.Clients
		wantError bool
		want      []string
	}{
		{
			name:      "Found no conditions",
			command:   []string{"ls", "-n", "notexist"},
			input:     seeds[0],
			wantError: false,
			want:      []string{"No conditions found", ""},
		},
		{
			name:      "Found conditions",
			command:   []string{"ls", "-n", "ns"},
			input:     seeds[0],
			wantError: false,
			want: []string{
				"NAME         AGE",
				"condition1   1 minute ago",
				"condition2   20 seconds ago",
				"condition3   3 weeks ago",
				"",
			},
		},
		{
			name:      "Specify output flag",
			command:   []string{"ls", "-n", "ns", "--output", "yaml"},
			input:     seeds[0],
			wantError: false,
			want: []string{
				"apiVersion: tekton.dev/v1alpha1",
				"items:",
				"- metadata:",
				"    creationTimestamp: \"1984-04-03T23:59:00Z\"",
				"    name: condition1",
				"    namespace: ns",
				"  spec:",
				"    check:",
				"      name: \"\"",
				"      resources: {}",
				"- metadata:",
				"    creationTimestamp: \"1984-04-03T23:59:40Z\"",
				"    name: condition2",
				"    namespace: ns",
				"  spec:",
				"    check:",
				"      name: \"\"",
				"      resources: {}",
				"- metadata:",
				"    creationTimestamp: \"1984-03-13T16:00:00Z\"",
				"    name: condition3",
				"    namespace: ns",
				"  spec:",
				"    check:",
				"      name: \"\"",
				"      resources: {}",
				"kind: ConditionList",
				"metadata: {}",
				"",
			},
		},
		{
			name:      "Failed to list condition resources",
			command:   []string{"ls", "-n", "ns"},
			input:     seeds[1],
			wantError: true,
			want:      []string{"test error"},
		},
		{
			name:      "Failed to list condition resources with specify output flag",
			command:   []string{"ls", "-n", "ns", "--output", "yaml"},
			input:     seeds[1],
			wantError: true,
			want:      []string{"test error"},
		},
	}

	for _, tp := range testParams {
		t.Run(tp.name, func(t *testing.T) {
			p := &test.Params{Tekton: tp.input.Pipeline}
			pipelineResource := Command(p)

			want := strings.Join(tp.want, "\n")

			out, err := test.ExecuteCommand(pipelineResource, tp.command...)
			if tp.wantError {
				if err == nil {
					t.Errorf("Error expected here")
				}
				test.AssertOutput(t, want, err.Error())
			} else {
				if err != nil {
					t.Errorf("Unexpected Error")
				}
				test.AssertOutput(t, want, out)
			}
		})
	}
}
