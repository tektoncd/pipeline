/*
 Copyright 2022 The Tekton Authors

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

package status

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/yaml"
)

func TestGetTaskRunStatusForPipelineTask(t *testing.T) {
	testCases := []struct {
		name           string
		taskRun        *v1beta1.TaskRun
		childRef       v1beta1.ChildStatusReference
		expectedStatus *v1beta1.TaskRunStatus
		expectedErr    error
	}{
		{
			name: "wrong kind",
			childRef: v1beta1.ChildStatusReference{
				TypeMeta: runtime.TypeMeta{
					Kind: "something-else",
				},
				PipelineTaskName: "some-task",
			},
			expectedErr: errors.New("could not fetch status for PipelineTask some-task: should have kind TaskRun, but is something-else"),
		}, {
			name: "taskrun not found",
			childRef: v1beta1.ChildStatusReference{
				TypeMeta: runtime.TypeMeta{
					Kind: "TaskRun",
				},
				Name:             "some-task-run",
				PipelineTaskName: "some-task",
			},
		}, {
			name: "success",
			taskRun: parse.MustParseV1beta1TaskRun(t, `
metadata:
  name: some-task-run
spec: {}
status:
  conditions:
  - status: "False"
    type: Succeeded
  podName: my-pod-name
`),
			childRef: v1beta1.ChildStatusReference{
				TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
				Name:             "some-task-run",
				PipelineTaskName: "some-task",
			},
			expectedStatus: &v1beta1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					PodName: "my-pod-name",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)
			d := test.Data{}
			if tc.taskRun != nil {
				d.TaskRuns = []*v1beta1.TaskRun{tc.taskRun}
			}
			clients, _ := test.SeedTestData(t, ctx, d)

			trStatus, err := GetTaskRunStatusForPipelineTask(ctx, clients.Pipeline, "", tc.childRef)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("no error, but expected '%s'", tc.expectedErr.Error())
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Fatalf("expected error '%s', but got '%s'", tc.expectedErr.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("received unexpected error '%s'", err.Error())
				}
				if d := cmp.Diff(tc.expectedStatus, trStatus); d != "" {
					t.Errorf("status does not match expected. Diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestGetRunStatusForPipelineTask(t *testing.T) {
	testCases := []struct {
		name           string
		run            v1beta1.RunObject
		childRef       v1beta1.ChildStatusReference
		expectedStatus *v1beta1.CustomRunStatus
		expectedErr    error
	}{
		{
			name: "wrong kind",
			childRef: v1beta1.ChildStatusReference{
				TypeMeta: runtime.TypeMeta{
					Kind: "something-else",
				},
				PipelineTaskName: "some-task",
			},
			expectedErr: errors.New("could not fetch status for PipelineTask some-task: should have kind Run or CustomRun, but is something-else"),
		}, {
			name: "run not found",
			childRef: v1beta1.ChildStatusReference{
				TypeMeta: runtime.TypeMeta{
					Kind: "CustomRun",
				},
				Name:             "some-run",
				PipelineTaskName: "some-task",
			},
		}, {
			name: "success",
			run: parse.MustParseCustomRun(t, `
metadata:
  name: some-run
spec: {}
status:
  conditions:
  - status: "False"
    type: Succeeded
`),
			childRef: v1beta1.ChildStatusReference{
				TypeMeta:         runtime.TypeMeta{Kind: "CustomRun"},
				Name:             "some-run",
				PipelineTaskName: "some-task",
			},
			expectedStatus: &v1beta1.CustomRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		}, {
			name: "success with alpha run",
			run: parse.MustParseRun(t, `
metadata:
  name: some-run
spec: {}
status:
  conditions:
  - status: "False"
    type: Succeeded
`),
			childRef: v1beta1.ChildStatusReference{
				TypeMeta:         runtime.TypeMeta{Kind: "Run"},
				Name:             "some-run",
				PipelineTaskName: "some-task",
			},
			expectedStatus: &v1beta1.CustomRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)
			d := test.Data{}
			if tc.run != nil {
				switch ro := tc.run.(type) {
				case *v1beta1.CustomRun:
					d.CustomRuns = []*v1beta1.CustomRun{ro}
				case *v1alpha1.Run:
					d.Runs = []*v1alpha1.Run{ro}
				}
			}
			clients, _ := test.SeedTestData(t, ctx, d)

			runStatus, err := GetRunStatusForPipelineTask(ctx, clients.Pipeline, "", tc.childRef)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("no error, but expected '%s'", tc.expectedErr.Error())
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Fatalf("expected error '%s', but got '%s'", tc.expectedErr.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("received unexpected error '%s'", err.Error())
				}
				if d := cmp.Diff(tc.expectedStatus, runStatus); d != "" {
					t.Errorf("status does not match expected. Diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestGetFullPipelineTaskStatuses(t *testing.T) {
	tr1 := parse.MustParseV1beta1TaskRun(t, `
metadata:
  name: pr-task-1
spec: {}
status:
  conditions:
  - status: "True"
    type: Succeeded
  taskResults:
  - name: aResult
    value: aResultValue
`)

	customRun1 := parse.MustParseCustomRun(t, `
metadata:
  name: pr-run-1
spec: {}
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: foo
    value: oof
  - name: bar
    value: rab
`)

	run2 := parse.MustParseRun(t, `
metadata:
  name: pr-run-2
spec: {}
status:
  conditions:
  - status: "True"
    type: Succeeded
  results:
  - name: foo
    value: oof
  - name: bar
    value: rab
`)

	testCases := []struct {
		name                string
		originalPR          *v1beta1.PipelineRun
		taskRuns            []*v1beta1.TaskRun
		runs                []v1beta1.RunObject
		expectedTRStatuses  map[string]*v1beta1.PipelineRunTaskRunStatus
		expectedRunStatuses map[string]*v1beta1.PipelineRunRunStatus
		expectedErr         error
	}{
		{
			name:                "nil pr",
			originalPR:          nil,
			expectedTRStatuses:  nil,
			expectedRunStatuses: nil,
			expectedErr:         nil,
		},
		{
			name: "minimal embedded",
			originalPR: parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: pr
spec: {}
status:
  childReferences:
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: pr-task-1
    pipelineTaskName: task-1
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-run-1
    pipelineTaskName: run-1
  - apiVersion: tekton.dev/v1alpha1
    kind: Run
    name: pr-run-2
    pipelineTaskName: run-2
  conditions:
  - message: Not all Tasks in the Pipeline have finished executing
    reason: Running
    status: Unknown
    type: Succeeded
`),
			taskRuns: []*v1beta1.TaskRun{tr1},
			runs:     []v1beta1.RunObject{customRun1, run2},
			expectedTRStatuses: mustParseTaskRunStatusMap(t, `
pr-task-1:
  pipelineTaskName: task-1
  status:
    conditions:
    - status: "True"
      type: Succeeded
    taskResults:
    - name: aResult
      value: aResultValue
`),
			expectedRunStatuses: mustParseRunStatusMap(t, `
pr-run-1:
  pipelineTaskName: run-1
  status:
    conditions:
    - status: "True"
      type: Succeeded
    results:
    - name: foo
      value: oof
    - name: bar
      value: rab
pr-run-2:
  pipelineTaskName: run-2
  status:
    conditions:
    - status: "True"
      type: Succeeded
    results:
    - name: foo
      value: oof
    - name: bar
      value: rab
`),
			expectedErr: nil,
		}, {
			name: "full embedded",
			originalPR: parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: pr
spec: {}
status:
  taskRuns:
    pr-task-1:
      pipelineTaskName: task-1
      status:
        conditions:
        - status: "True"
          type: Succeeded
        taskResults:
        - name: aResult
          value: aResultValue
  runs:
    pr-run-1:
      pipelineTaskName: run-1
      status:
        conditions:
        - status: "True"
          type: Succeeded
        results:
        - name: foo
          value: oof
        - name: bar
          value: rab
    pr-run-2:
      pipelineTaskName: run-2
      status:
        conditions:
        - status: "True"
          type: Succeeded
        results:
        - name: foo
          value: oof
        - name: bar
          value: rab
  conditions:
  - message: Not all Tasks in the Pipeline have finished executing
    reason: Running
    status: Unknown
    type: Succeeded
`),
			taskRuns: []*v1beta1.TaskRun{tr1},
			runs:     []v1beta1.RunObject{customRun1, run2},
			expectedTRStatuses: mustParseTaskRunStatusMap(t, `
pr-task-1:
  pipelineTaskName: task-1
  status:
    conditions:
    - status: "True"
      type: Succeeded
    taskResults:
    - name: aResult
      value: aResultValue
`),
			expectedRunStatuses: mustParseRunStatusMap(t, `
pr-run-1:
  pipelineTaskName: run-1
  status:
    conditions:
    - status: "True"
      type: Succeeded
    results:
    - name: foo
      value: oof
    - name: bar
      value: rab
pr-run-2:
  pipelineTaskName: run-2
  status:
    conditions:
    - status: "True"
      type: Succeeded
    results:
    - name: foo
      value: oof
    - name: bar
      value: rab
`),
			expectedErr: nil,
		}, {
			name: "both embedded",
			originalPR: parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: pr
spec: {}
status:
  taskRuns:
    pr-task-1:
      pipelineTaskName: task-1
      status:
        conditions:
        - status: "True"
          type: Succeeded
        taskResults:
        - name: aResult
          value: aResultValue
  runs:
    pr-run-1:
      pipelineTaskName: run-1
      status:
        conditions:
        - status: "True"
          type: Succeeded
        results:
        - name: foo
          value: oof
        - name: bar
          value: rab
    pr-run-2:
      pipelineTaskName: run-2
      status:
        conditions:
        - status: "True"
          type: Succeeded
        results:
        - name: foo
          value: oof
        - name: bar
          value: rab
  childReferences:
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: pr-task-2
    pipelineTaskName: task-2
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-run-1
    pipelineTaskName: run-1
  - apiVersion: tekton.dev/v1alpha1
    kind: Run
    name: pr-run-2
    pipelineTaskName: run-2
  conditions:
  - message: Not all Tasks in the Pipeline have finished executing
    reason: Running
    status: Unknown
    type: Succeeded
`),
			taskRuns: []*v1beta1.TaskRun{tr1},
			runs:     []v1beta1.RunObject{customRun1, run2},
			expectedTRStatuses: mustParseTaskRunStatusMap(t, `
pr-task-1:
  pipelineTaskName: task-1
  status:
    conditions:
    - status: "True"
      type: Succeeded
    taskResults:
    - name: aResult
      value: aResultValue
`),
			expectedRunStatuses: mustParseRunStatusMap(t, `
pr-run-1:
  pipelineTaskName: run-1
  status:
    conditions:
    - status: "True"
      type: Succeeded
    results:
    - name: foo
      value: oof
    - name: bar
      value: rab
pr-run-2:
  pipelineTaskName: run-2
  status:
    conditions:
    - status: "True"
      type: Succeeded
    results:
    - name: foo
      value: oof
    - name: bar
      value: rab
`),
			expectedErr: nil,
		}, {
			name: "missing run",
			originalPR: parse.MustParseV1beta1PipelineRun(t, `
metadata:
  name: pr
spec: {}
status:
  childReferences:
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: pr-task-1
    pipelineTaskName: task-1
  - apiVersion: tekton.dev/v1beta1
    kind: CustomRun
    name: pr-run-1
    pipelineTaskName: run-1
  conditions:
  - message: Not all Tasks in the Pipeline have finished executing
    reason: Running
    status: Unknown
    type: Succeeded
`),
			taskRuns: []*v1beta1.TaskRun{tr1},
			expectedTRStatuses: mustParseTaskRunStatusMap(t, `
pr-task-1:
  pipelineTaskName: task-1
  status:
    conditions:
    - status: "True"
      type: Succeeded
    taskResults:
    - name: aResult
      value: aResultValue
`),
			expectedRunStatuses: mustParseRunStatusMap(t, `
pr-run-1:
  pipelineTaskName: run-1
`),
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)

			d := test.Data{}

			if tc.originalPR != nil {
				d.PipelineRuns = []*v1beta1.PipelineRun{tc.originalPR}
			}
			if len(tc.taskRuns) > 0 {
				d.TaskRuns = tc.taskRuns
			}
			if len(tc.runs) > 0 {
				for _, r := range tc.runs {
					switch ro := r.(type) {
					case *v1beta1.CustomRun:
						d.CustomRuns = append(d.CustomRuns, ro)
					case *v1alpha1.Run:
						d.Runs = append(d.Runs, ro)
					}
				}
			}

			clients, _ := test.SeedTestData(t, ctx, d)

			trStatuses, runStatuses, err := GetFullPipelineTaskStatuses(ctx, clients.Pipeline, "", tc.originalPR)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("no error, but expected '%s'", tc.expectedErr.Error())
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Fatalf("expected error '%s', but got '%s'", tc.expectedErr.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("received unexpected error '%s'", err.Error())
				}
				if d := cmp.Diff(tc.expectedTRStatuses, trStatuses); d != "" {
					t.Errorf("TaskRun statuses do not match expected. Diff %s", diff.PrintWantGot(d))
				}
				if d := cmp.Diff(tc.expectedRunStatuses, runStatuses); d != "" {
					t.Errorf("Run statuses do not match expected. Diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func mustParseTaskRunStatusMap(t *testing.T, yamlStr string) map[string]*v1beta1.PipelineRunTaskRunStatus {
	var output map[string]*v1beta1.PipelineRunTaskRunStatus
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing task run status map %s: %v", yamlStr, err)
	}
	return output
}

func mustParseRunStatusMap(t *testing.T, yamlStr string) map[string]*v1beta1.PipelineRunRunStatus {
	var output map[string]*v1beta1.PipelineRunRunStatus
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing run status map %s: %v", yamlStr, err)
	}
	return output
}
