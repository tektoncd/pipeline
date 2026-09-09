//go:build e2e

/*
Copyright 2026 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// @test:execution=parallel
func TestTaskResultsDoNotAccumulateAcrossSteps(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, requireAllGates(map[string]string{
		"results-from":                           "termination-message",
		"enable-termination-message-compression": "false",
	}))
	t.Parallel()
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// The aggregate exceeds 4 KiB, while each step fits within the kubelet's
	// per-container share of the 12 KiB Pod budget, including init containers.
	const count = 12
	task := &v1.TaskSpec{}
	want := map[string]string{}
	image := getTestImage(busyboxImage)
	for i := range count {
		name := fmt.Sprintf("result-%d", i)
		value := strings.Repeat(fmt.Sprintf("%02d", i), 225)
		want[name] = value
		task.Results = append(task.Results, v1.TaskResult{Name: name, Type: v1.ResultsTypeString})
		script := fmt.Sprintf("printf '%%s' '%s' > /tekton/results/%s\n", value, name)
		if i > 0 {
			// Keep supporting direct reads of results written by earlier steps.
			previousName := fmt.Sprintf("result-%d", i-1)
			script += fmt.Sprintf("test \"$(cat /tekton/results/%s)\" = '%s'\n", previousName, want[previousName])
		}
		task.Steps = append(task.Steps, v1.Step{Name: fmt.Sprintf("write-%02d", i), Image: image, Script: "#!/bin/sh\nset -eu\n" + script})
	}
	// Reusing a result name must retain last-writer-wins behavior.
	// The new value has the same length as the old one.
	want["result-0"] = strings.Repeat("zz", 225)
	task.Steps = append(task.Steps, v1.Step{
		Name: "overwrite", Image: image,
		Script: fmt.Sprintf("#!/bin/sh\nset -eu\nprintf '%%s' '%s' > $(results.result-0.path)\n", want["result-0"]),
	})
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: helpers.ObjectNameForTest(t), Namespace: namespace},
		Spec:       v1.TaskRunSpec{TaskSpec: task},
	}
	if _, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSucceed", v1Version); err != nil {
		t.Fatal(err)
	}
	tr, err := c.V1TaskRunClient.Get(ctx, tr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range tr.Status.Results {
		got[r.Name] = r.Value.StringVal
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("TaskRun results (-want +got): %s", d)
	}
}
