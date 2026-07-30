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

package pipelinerun

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestReconcile_EmbeddedTaskSpecDescriptionSurvivesInChildTaskRunSpec is a regression
// test proving that description-stripping (introduced to shrink status.pipelineSpec/
// status.taskSpec snapshots, see #10321) leaks beyond the status snapshot.
//
// pipelinerun.go's reconcile() assigns `pr.Status.PipelineSpec = pipelineSpec` and then
// calls `pr.Status.PipelineSpec.StripDescriptions()` on the *same* object still referenced
// by the local `pipelineSpec` variable (no copy is made at that call site, unlike in
// storePipelineSpecAndMergeMeta). For a PipelineTask with an inline `taskSpec`,
// PipelineTask.TaskSpec is a pointer (*EmbeddedTask), so stripping the status snapshot
// mutates the very same TaskSpec object that resolveTask/createTaskRun later assign,
// as-is, to the child TaskRun's Spec.TaskSpec.
//
// The result: with the default `keep-status-spec-descriptions: "false"`, the child
// TaskRun's spec.taskSpec.description ends up empty even though the user's inline task
// definition on the Pipeline/PipelineRun had one set. That is a change to a live,
// user-facing Spec field, not just to an internal status cache, and is out of scope for
// what this feature is meant to touch.
func TestReconcile_EmbeddedTaskSpecDescriptionSurvivesInChildTaskRunSpec(t *testing.T) {
	const wantDescription = "embedded task description"

	prs := []*v1.PipelineRun{parse.MustParseV1PipelineRun(t, `
metadata:
  name: repro-embedded-taskspec-description
  namespace: foo
spec:
  pipelineSpec:
    tasks:
    - name: a-task
      taskSpec:
        description: `+wantDescription+`
        steps:
        - name: step1
          image: foo
`)}

	d := test.Data{PipelineRuns: prs}
	prt := newPipelineRunTest(t, d)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "repro-embedded-taskspec-description", nil, false)

	tr, err := clients.Pipeline.TektonV1().TaskRuns("foo").Get(prt.TestAssets.Ctx, "repro-embedded-taskspec-description-a-task", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting child TaskRun: %v", err)
	}
	if tr.Spec.TaskSpec == nil {
		t.Fatal("expected child TaskRun to have an embedded TaskSpec")
	}

	// This is expected to fail against the current implementation: it proves that
	// description-stripping mutates the live PipelineTask.TaskSpec object shared with
	// the child TaskRun's Spec.TaskSpec, not just the PipelineRun's status snapshot.
	if got := tr.Spec.TaskSpec.Description; got != wantDescription {
		t.Errorf("child TaskRun spec.taskSpec.description = %q, want %q (description leaked out of the intended status-only strip)", got, wantDescription)
	}
}
