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

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	th "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
)

func TestReconcilePublishesFinallyOutcomeWithTerminalCondition(t *testing.T) {
	for _, tc := range []struct {
		name       string
		embedded   bool
		failed     bool
		skipped    bool
		disabled   bool
		wantReason string
	}{
		{name: "referenced"},
		{name: "embedded", embedded: true},
		{name: "execution failure wins", failed: true, wantReason: "Failed"},
		{name: "skipped report is missing", skipped: true, wantReason: "InvalidPipelineOutcome"},
		{name: "disabled gate rejects resolved spec", disabled: true, wantReason: "PipelineValidationFailed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := parse.MustParseV1Pipeline(t, `
metadata:
  name: outcome-pipeline
  namespace: foo
spec:
  outcome:
    result: report
  results:
  - name: report
    type: object
    value: $(finally.report.results.outcome[*])
  tasks:
  - name: work
    taskRef:
      name: work
  finally:
  - name: report
    taskRef:
      name: report
`)
			work := parse.MustParseV1Task(t, `
metadata:
  name: work
  namespace: foo
spec:
  steps:
  - name: work
    image: alpine
`)
			report := work.DeepCopy()
			report.Name = "report"
			report.Spec.Results = []v1.TaskResult{{
				Name: "outcome", Type: v1.ResultsTypeObject,
				Properties: map[string]v1.PropertySpec{"reason": {Type: v1.ParamTypeString}, "message": {Type: v1.ParamTypeString}},
			}}
			if tc.skipped {
				p.Spec.Finally[0].When = append(p.Spec.Finally[0].When, v1.WhenExpression{Input: "skip", Operator: "in", Values: []string{"run"}})
			}
			trs := []*v1.TaskRun{}
			for _, task := range []*v1.Task{work, report} {
				if tc.skipped && task.Name == "report" {
					continue
				}
				tr := &v1.TaskRun{
					ObjectMeta: taskRunObjectMeta("outcome-run-"+task.Name, "foo", "outcome-run", p.Name, task.Name, true),
					Spec:       v1.TaskRunSpec{TaskRef: &v1.TaskRef{Name: task.Name}},
				}
				tr.Status.SetCondition(&apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue, Reason: v1.TaskRunReasonSuccessful.String()})
				if tc.failed && task.Name == "work" {
					tr.Status.SetCondition(&apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse, Reason: "Failed"})
				}
				if task.Name == "report" {
					tr.Status.Results = []v1.TaskRunResult{{Name: "outcome", Type: v1.ResultsTypeObject, Value: v1.ResultValue{
						Type: v1.ParamTypeObject, ObjectVal: map[string]string{"reason": "Neutral", "message": "review findings"},
					}}}
				}
				trs = append(trs, tr)
			}
			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "outcome-run", Namespace: "foo"},
				Spec:       v1.PipelineRunSpec{PipelineRef: &v1.PipelineRef{Name: p.Name}},
			}
			pr.Status.InitializeConditions(testClock)
			if tc.embedded {
				pr.Spec.PipelineRef = nil
				pr.Spec.PipelineSpec = p.Spec.DeepCopy()
			}
			cms := th.NewFeatureFlagsConfigMapInSlice()
			cms[0].Data["enable-pipelinerun-outcomes"] = "true"
			if tc.disabled {
				cms[0].Data["enable-pipelinerun-outcomes"] = "false"
			}
			prt := newPipelineRunTest(t, test.Data{Pipelines: []*v1.Pipeline{p}, PipelineRuns: []*v1.PipelineRun{pr}, Tasks: []*v1.Task{work, report}, TaskRuns: trs, ConfigMaps: cms})
			defer prt.Cancel()

			got, clients := prt.reconcileRun("foo", pr.Name, nil, tc.disabled)

			want := &v1.PipelineRunOutcome{Reason: "Neutral", Message: "review findings"}
			condition := got.Status.GetCondition(apis.ConditionSucceeded)
			if tc.wantReason != "" {
				want = nil
				if !condition.IsFalse() || condition.Reason != tc.wantReason {
					t.Fatalf("execution condition = %+v, want failure %s", condition, tc.wantReason)
				}
			} else if !condition.IsTrue() {
				t.Fatalf("execution condition = %+v", condition)
			}
			if diff := cmp.Diff(want, got.Status.Outcome); diff != "" {
				t.Fatalf("persisted outcome (-want +got): %s", diff)
			}
			published := false
			for _, action := range clients.Pipeline.Actions() {
				update, ok := action.(ktesting.UpdateAction)
				if !ok || action.GetResource().Resource != "pipelineruns" || action.GetSubresource() != "status" {
					continue
				}
				run := update.GetObject().(*v1.PipelineRun)
				if condition := run.Status.GetCondition(apis.ConditionSucceeded); condition != nil && condition.Status != corev1.ConditionUnknown {
					published = true
					if condition.Status != got.Status.GetCondition(apis.ConditionSucceeded).Status || cmp.Diff(want, run.Status.Outcome) != "" {
						t.Errorf("terminal status update must include the outcome: %+v", run.Status)
					}
				}
			}
			if !published {
				t.Error("no terminal PipelineRun status was published")
			}
		})
	}
}
