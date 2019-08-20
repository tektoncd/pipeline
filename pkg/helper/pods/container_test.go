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

package pods

import (
	"testing"

	"github.com/tektoncd/cli/pkg/helper/pods/fake"
	"github.com/tektoncd/cli/pkg/test"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

func TestContainer_fetch_logs(t *testing.T) {
	podName := "build-and-push-xyz"
	ns := "test"
	container1 := "step-build-app"
	container2 := "nop"

	ps := []*corev1.Pod{
		tb.Pod(podName, ns,
			tb.PodSpec(
				tb.PodContainer(container1, "step-build-app:latest"),
				tb.PodContainer(container2, "override-with-nop:latest"),
			),
		),
	}

	logs := fake.Logs(
		fake.PodLog(podName,
			fake.NewContainer(container1, "pushed blob sha256:7be8c1df53f934d63b71db8595212e2955fd30a9b0054eccf42d732f53ef136b"),
			fake.NewContainer(container2, "Task completed successfully"),
		),
	)

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Pods: ps})

	pod := New(podName, ns, cs.Kube, fake.Streamer(logs))

	type testdata struct {
		container string
		follow    bool
		expected  []Log
	}

	td := []testdata{

		{
			container: container1, follow: false,
			expected: []Log{{
				PodName:       podName,
				ContainerName: container1,
				Log:           "pushed blob sha256:7be8c1df53f934d63b71db8595212e2955fd30a9b0054eccf42d732f53ef136b",
			}},
		},

		{
			container: container2, follow: false,
			expected: []Log{{
				PodName:       podName,
				ContainerName: container2,
				Log:           "Task completed successfully",
			}},
		},
	}

	for _, d := range td {
		lr := pod.Container(d.container).LogReader(d.follow)
		output, err := containerLogs(lr)

		if err != nil {
			t.Errorf("error occured %v", err)
		}

		test.AssertOutput(t, d.expected, output)
	}
}

func containerLogs(lr *LogReader) ([]Log, error) {
	logC, errC, err := lr.Read()

	output := []Log{}
	if err != nil {
		return output, err
	}

	for {
		select {
		case l, ok := <-logC:
			if !ok {
				return output, nil
			}
			output = append(output, l)

		case e, ok := <-errC:
			if !ok {
				return output, e
			}
		}
	}
}
