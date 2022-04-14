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

package pipelinerun

import (
	"fmt"
	"regexp"
	"sort"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	logtesting "knative.dev/pkg/logging/testing"
)

type updateStatusTaskRunsData struct {
	withConditions map[string]*v1beta1.PipelineRunTaskRunStatus
	missingTaskRun map[string]*v1beta1.PipelineRunTaskRunStatus
	foundTaskRun   map[string]*v1beta1.PipelineRunTaskRunStatus
	recovered      map[string]*v1beta1.PipelineRunTaskRunStatus
	simple         map[string]*v1beta1.PipelineRunTaskRunStatus
}

func getUpdateStatusTaskRunsData(t *testing.T) updateStatusTaskRunsData {
	prTask1Yaml := `
pipelineTaskName: task-1
status: {}
`
	prTask2Yaml := `
conditionChecks:
  pr-task-2-running-condition-check-xxyyy:
    conditionName: running-condition-0
    status:
      check:
        running: {}
      conditions:
      - status: Unknown
        type: Succeeded
pipelineTaskName: task-2
`
	prTask3Yaml := `
conditionChecks:
  pr-task-3-successful-condition-check-xxyyy:
    conditionName: successful-condition-0
    status:
      check:
        terminated:
          exitCode: 0
      conditions:
      - status: "True"
        type: Succeeded
pipelineTaskName: task-3
status: {}
`

	prTask4Yaml := `
conditionChecks:
  pr-task-4-failed-condition-check-xxyyy:
    conditionName: failed-condition-0
    status:
      check:
        terminated:
          exitCode: 127
      conditions:
      - status: "False"
        type: Succeeded
pipelineTaskName: task-4
`

	prTask3NoStatusYaml := `
conditionChecks:
  pr-task-3-successful-condition-check-xxyyy:
    conditionName: successful-condition-0
pipelineTaskName: task-3
status: {}
`

	orphanedPRTask2Yaml := `
conditionChecks:
  pr-task-2-running-condition-check-xxyyy:
    conditionName: running-condition-0
pipelineTaskName: task-2
`

	orphanedPRTask4Yaml := `
conditionChecks:
  pr-task-4-failed-condition-check-xxyyy:
    conditionName: failed-condition-0
pipelineTaskName: task-4
`

	withConditions := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-2-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask2Yaml),
		"pr-task-3-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask3Yaml),
		"pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask4Yaml),
	}

	missingTaskRuns := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-2-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask2Yaml),
		"pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask4Yaml),
	}

	foundTaskRun := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-2-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask2Yaml),
		"pr-task-3-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask3NoStatusYaml),
		"pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask4Yaml),
	}

	recovered := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"orphaned-taskruns-pr-task-2-xxyyy": mustParsePipelineRunTaskRunStatus(t, orphanedPRTask2Yaml),
		"orphaned-taskruns-pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, orphanedPRTask4Yaml),
		"pr-task-1-xxyyy":                   mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-3-xxyyy":                   mustParsePipelineRunTaskRunStatus(t, prTask3NoStatusYaml),
	}

	simple := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
	}

	return updateStatusTaskRunsData{
		withConditions: withConditions,
		missingTaskRun: missingTaskRuns,
		foundTaskRun:   foundTaskRun,
		recovered:      recovered,
		simple:         simple,
	}
}

func TestUpdatePipelineRunStatusFromTaskRuns(t *testing.T) {

	prUID := types.UID("11111111-1111-1111-1111-111111111111")

	taskRunsPRStatusData := getUpdateStatusTaskRunsData(t)

	prRunningStatus := duckv1beta1.Status{
		Conditions: []apis.Condition{
			{
				Type:    "Succeeded",
				Status:  "Unknown",
				Reason:  "Running",
				Message: "Not all Tasks in the Pipeline have finished executing",
			},
		},
	}

	prStatusWithCondition := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: taskRunsPRStatusData.withConditions,
		},
	}

	prStatusMissingTaskRun := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: taskRunsPRStatusData.missingTaskRun,
		},
	}

	prStatusFoundTaskRun := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: taskRunsPRStatusData.foundTaskRun,
		},
	}

	prStatusWithEmptyTaskRuns := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{},
		},
	}

	prStatusWithOrphans := v1beta1.PipelineRunStatus{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{
				{
					Type:    "Succeeded",
					Status:  "Unknown",
					Reason:  "Running",
					Message: "Not all Tasks in the Pipeline have finished executing",
				},
			},
		},
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{},
		},
	}

	prStatusRecovered := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: taskRunsPRStatusData.recovered,
		},
	}

	prStatusRecoveredSimple := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: taskRunsPRStatusData.simple,
		},
	}

	allTaskRuns, taskRunsFromAnotherPR, taskRunsWithNoOwner, _, _, _ := getTestTaskRunsAndRuns(t)

	tcs := []struct {
		prName           string
		prStatus         v1beta1.PipelineRunStatus
		trs              []*v1beta1.TaskRun
		expectedPrStatus v1beta1.PipelineRunStatus
	}{
		{
			prName:           "no-status-no-taskruns",
			prStatus:         v1beta1.PipelineRunStatus{},
			trs:              nil,
			expectedPrStatus: v1beta1.PipelineRunStatus{},
		}, {
			prName:           "status-no-taskruns",
			prStatus:         prStatusWithCondition,
			trs:              nil,
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyTaskRuns,
			trs: []*v1beta1.TaskRun{parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)},
			expectedPrStatus: prStatusRecoveredSimple,
		}, {
			prName:   "status-missing-taskruns",
			prStatus: prStatusMissingTaskRun,
			trs: []*v1beta1.TaskRun{
				parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
				parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/conditionCheck: pr-task-3-successful-condition-check-xxyyy
    tekton.dev/conditionName: successful-condition
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-successful-condition-check-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
			},
			expectedPrStatus: prStatusFoundTaskRun,
		}, {
			prName:           "status-matching-taskruns-pr",
			prStatus:         prStatusWithCondition,
			trs:              allTaskRuns,
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:           "orphaned-taskruns-pr",
			prStatus:         prStatusWithOrphans,
			trs:              allTaskRuns,
			expectedPrStatus: prStatusRecovered,
		}, {
			prName:           "tr-from-another-pr",
			prStatus:         prStatusWithEmptyTaskRuns,
			trs:              taskRunsFromAnotherPR,
			expectedPrStatus: prStatusWithEmptyTaskRuns,
		}, {
			prName:           "tr-with-no-owner",
			prStatus:         prStatusWithEmptyTaskRuns,
			trs:              taskRunsWithNoOwner,
			expectedPrStatus: prStatusWithEmptyTaskRuns,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.prName, func(t *testing.T) {
			logger := logtesting.TestLogger(t)

			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: tc.prName, UID: prUID},
				Status:     tc.prStatus,
			}

			updatePipelineRunStatusFromTaskRuns(logger, pr, tc.trs)
			actualPrStatus := pr.Status

			expectedPRStatus := tc.expectedPrStatus

			// The TaskRun keys for recovered taskruns will contain a new random key, appended to the
			// base name that we expect. Replace the random part so we can diff the whole structure
			actualTaskRuns := actualPrStatus.PipelineRunStatusFields.TaskRuns
			if actualTaskRuns != nil {
				fixedTaskRuns := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
				re := regexp.MustCompile(`^[a-z\-]*?-task-[0-9]`)
				for k, v := range actualTaskRuns {
					newK := re.FindString(k)
					fixedTaskRuns[newK+"-xxyyy"] = v
				}
				actualPrStatus.PipelineRunStatusFields.TaskRuns = fixedTaskRuns
			}

			if d := cmp.Diff(expectedPRStatus, actualPrStatus); d != "" {
				t.Errorf("expected the PipelineRun status to match %#v. Diff %s", expectedPRStatus, diff.PrintWantGot(d))
			}
		})
	}
}

func TestUpdatePipelineRunStatusFromRuns(t *testing.T) {
	prUID := types.UID("11111111-1111-1111-1111-111111111111")

	prRunningStatus := duckv1beta1.Status{
		Conditions: []apis.Condition{
			{
				Type:    "Succeeded",
				Status:  "Unknown",
				Reason:  "Running",
				Message: "Not all Tasks in the Pipeline have finished executing",
			},
		},
	}

	prStatusWithSomeRuns := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			Runs: map[string]*v1beta1.PipelineRunRunStatus{
				"pr-run-1-xxyyy": {
					PipelineTaskName: "run-1",
					Status:           &v1alpha1.RunStatus{},
				},
				"pr-run-2-xxyyy": {
					PipelineTaskName: "run-2",
					Status:           nil,
				},
			},
		},
	}

	prStatusWithAllRuns := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			Runs: map[string]*v1beta1.PipelineRunRunStatus{
				"pr-run-1-xxyyy": {
					PipelineTaskName: "run-1",
					Status:           &v1alpha1.RunStatus{},
				},
				"pr-run-2-xxyyy": {
					PipelineTaskName: "run-2",
					Status:           nil,
				},
				"pr-run-3-xxyyy": {
					PipelineTaskName: "run-3",
					Status:           &v1alpha1.RunStatus{},
				},
			},
		},
	}

	prStatusWithEmptyRuns := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			Runs: map[string]*v1beta1.PipelineRunRunStatus{},
		},
	}

	prStatusRecoveredSimple := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			Runs: map[string]*v1beta1.PipelineRunRunStatus{
				"pr-run-1-xxyyy": {
					PipelineTaskName: "run-1",
					Status:           &v1alpha1.RunStatus{},
				},
			},
		},
	}

	_, _, _, allRuns, runsFromAnotherPR, runsWithNoOwner := getTestTaskRunsAndRuns(t)

	tcs := []struct {
		prName           string
		prStatus         v1beta1.PipelineRunStatus
		runs             []*v1alpha1.Run
		expectedPrStatus v1beta1.PipelineRunStatus
	}{
		{
			prName:           "no-status-no-runs",
			prStatus:         v1beta1.PipelineRunStatus{},
			runs:             nil,
			expectedPrStatus: v1beta1.PipelineRunStatus{},
		}, {
			prName:           "status-no-runs",
			prStatus:         prStatusWithSomeRuns,
			runs:             nil,
			expectedPrStatus: prStatusWithSomeRuns,
		}, {
			prName:   "status-nil-runs",
			prStatus: prStatusWithEmptyRuns,
			runs: []*v1alpha1.Run{parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)},
			expectedPrStatus: prStatusRecoveredSimple,
		}, {
			prName:   "status-missing-runs",
			prStatus: prStatusWithSomeRuns,
			runs: []*v1alpha1.Run{parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-3
  name: pr-run-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)},
			expectedPrStatus: prStatusWithAllRuns,
		}, {
			prName:           "status-matching-runs-pr",
			prStatus:         prStatusWithAllRuns,
			runs:             allRuns,
			expectedPrStatus: prStatusWithAllRuns,
		}, {
			prName:           "run-from-another-pr",
			prStatus:         prStatusWithEmptyRuns,
			runs:             runsFromAnotherPR,
			expectedPrStatus: prStatusWithEmptyRuns,
		}, {
			prName:           "run-with-no-owner",
			prStatus:         prStatusWithEmptyRuns,
			runs:             runsWithNoOwner,
			expectedPrStatus: prStatusWithEmptyRuns,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.prName, func(t *testing.T) {
			logger := logtesting.TestLogger(t)

			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: tc.prName, UID: prUID},
				Status:     tc.prStatus,
			}

			updatePipelineRunStatusFromRuns(logger, pr, tc.runs)
			actualPrStatus := pr.Status

			if d := cmp.Diff(tc.expectedPrStatus, actualPrStatus); d != "" {
				t.Errorf("expected the PipelineRun status to match %#v. Diff %s", tc.expectedPrStatus, diff.PrintWantGot(d))
			}
		})
	}
}

type updateStatusChildRefsData struct {
	withConditions []v1beta1.ChildStatusReference
	missingTaskRun []v1beta1.ChildStatusReference
	foundTaskRun   []v1beta1.ChildStatusReference
	missingRun     []v1beta1.ChildStatusReference
	recovered      []v1beta1.ChildStatusReference
	simple         []v1beta1.ChildStatusReference
	simpleRun      []v1beta1.ChildStatusReference
}

func getUpdateStatusChildRefsData(t *testing.T) updateStatusChildRefsData {
	prTask1Yaml := `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-1-xxyyy
pipelineTaskName: task-1
`

	prTask2Yaml := `
apiVersion: tekton.dev/v1beta1
conditionChecks:
- conditionCheckName: pr-task-2-running-condition-check-xxyyy
  conditionName: running-condition-0
  status:
    check:
      running: {}
    conditions:
    - status: Unknown
      type: Succeeded
kind: TaskRun
name: pr-task-2-xxyyy
pipelineTaskName: task-2
`

	prTask3Yaml := `
apiVersion: tekton.dev/v1beta1
conditionChecks:
- conditionCheckName: pr-task-3-successful-condition-check-xxyyy
  conditionName: successful-condition-0
  status:
    check:
      terminated:
        exitCode: 0
    conditions:
    - status: "True"
      type: Succeeded
kind: TaskRun
name: pr-task-3-xxyyy
pipelineTaskName: task-3
`

	prTask4Yaml := `
apiVersion: tekton.dev/v1beta1
conditionChecks:
- conditionCheckName: pr-task-4-failed-condition-check-xxyyy
  conditionName: failed-condition-0
  status:
    check:
      terminated:
        exitCode: 127
    conditions:
    - status: "False"
      type: Succeeded
kind: TaskRun
name: pr-task-4-xxyyy
pipelineTaskName: task-4
`

	prTask6Yaml := `
apiVersion: tekton.dev/v1alpha1
kind: Run
name: pr-run-6-xxyyy
pipelineTaskName: task-6
`

	prTask3NoStatusYaml := `
apiVersion: tekton.dev/v1beta1
conditionChecks:
- conditionCheckName: pr-task-3-successful-condition-check-xxyyy
  conditionName: successful-condition-0
kind: TaskRun
name: pr-task-3-xxyyy
pipelineTaskName: task-3
`

	orphanedPRTask2Yaml := `
apiVersion: tekton.dev/v1beta1
conditionChecks:
- conditionCheckName: pr-task-2-running-condition-check-xxyyy
  conditionName: running-condition-0
kind: TaskRun
name: orphaned-taskruns-pr-task-2-xxyyy
pipelineTaskName: task-2
`

	orphanedPRTask4Yaml := `
apiVersion: tekton.dev/v1beta1
conditionChecks:
- conditionCheckName: pr-task-4-failed-condition-check-xxyyy
  conditionName: failed-condition-0
kind: TaskRun
name: orphaned-taskruns-pr-task-4-xxyyy
pipelineTaskName: task-4
`

	withConditions := []v1beta1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask3Yaml),
		mustParseChildStatusReference(t, prTask4Yaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	missingTaskRun := []v1beta1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask4Yaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	foundTaskRun := []v1beta1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask3NoStatusYaml),
		mustParseChildStatusReference(t, prTask4Yaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	missingRun := []v1beta1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask3Yaml),
		mustParseChildStatusReference(t, prTask4Yaml),
	}

	recovered := []v1beta1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, orphanedPRTask2Yaml),
		mustParseChildStatusReference(t, prTask3NoStatusYaml),
		mustParseChildStatusReference(t, orphanedPRTask4Yaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	simple := []v1beta1.ChildStatusReference{mustParseChildStatusReference(t, prTask1Yaml)}

	simpleRun := []v1beta1.ChildStatusReference{mustParseChildStatusReference(t, prTask6Yaml)}

	return updateStatusChildRefsData{
		withConditions: withConditions,
		missingTaskRun: missingTaskRun,
		foundTaskRun:   foundTaskRun,
		missingRun:     missingRun,
		recovered:      recovered,
		simple:         simple,
		simpleRun:      simpleRun,
	}
}

func TestUpdatePipelineRunStatusFromChildRefs(t *testing.T) {
	prUID := types.UID("11111111-1111-1111-1111-111111111111")

	childRefsPRStatusData := getUpdateStatusChildRefsData(t)

	prRunningStatus := duckv1beta1.Status{
		Conditions: []apis.Condition{
			{
				Type:    "Succeeded",
				Status:  "Unknown",
				Reason:  "Running",
				Message: "Not all Tasks in the Pipeline have finished executing",
			},
		},
	}

	prStatusWithCondition := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.withConditions,
		},
	}

	prStatusMissingTaskRun := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.missingTaskRun,
		},
	}

	prStatusFoundTaskRun := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.foundTaskRun,
		},
	}

	prStatusMissingRun := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.missingRun,
		},
	}

	prStatusWithEmptyChildRefs := v1beta1.PipelineRunStatus{
		Status:                  prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{},
	}

	prStatusWithOrphans := v1beta1.PipelineRunStatus{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{
				{
					Type:    "Succeeded",
					Status:  "Unknown",
					Reason:  "Running",
					Message: "Not all Tasks in the Pipeline have finished executing",
				},
			},
		},
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{},
	}

	prStatusRecovered := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.recovered,
		},
	}

	prStatusRecoveredSimple := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.simple,
		},
	}

	prStatusRecoveredSimpleWithRun := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1alpha1",
					Kind:       "Run",
				},
				Name:             "pr-run-6-xxyyy",
				PipelineTaskName: "task-6",
			}},
		},
	}

	allTaskRuns, taskRunsFromAnotherPR, taskRunsWithNoOwner, _, runsFromAnotherPR, runsWithNoOwner := getTestTaskRunsAndRuns(t)

	singleRun := []*v1alpha1.Run{parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-6
  name: pr-run-6-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)}

	tcs := []struct {
		prName           string
		prStatus         v1beta1.PipelineRunStatus
		trs              []*v1beta1.TaskRun
		runs             []*v1alpha1.Run
		expectedPrStatus v1beta1.PipelineRunStatus
	}{
		{
			prName:           "no-status-no-taskruns-or-runs",
			prStatus:         v1beta1.PipelineRunStatus{},
			trs:              nil,
			runs:             nil,
			expectedPrStatus: v1beta1.PipelineRunStatus{},
		}, {
			prName:           "status-no-taskruns-or-runs",
			prStatus:         prStatusWithCondition,
			trs:              nil,
			runs:             nil,
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyChildRefs,
			trs: []*v1beta1.TaskRun{parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)},
			expectedPrStatus: prStatusRecoveredSimple,
		}, {
			prName:           "status-nil-runs",
			prStatus:         prStatusWithEmptyChildRefs,
			runs:             singleRun,
			expectedPrStatus: prStatusRecoveredSimpleWithRun,
		}, {
			prName:   "status-missing-taskruns",
			prStatus: prStatusMissingTaskRun,
			trs: []*v1beta1.TaskRun{
				parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
				parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/conditionCheck: pr-task-3-successful-condition-check-xxyyy
    tekton.dev/conditionName: successful-condition
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-successful-condition-check-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
			},
			expectedPrStatus: prStatusFoundTaskRun,
		}, {
			prName:           "status-missing-runs",
			prStatus:         prStatusMissingRun,
			runs:             singleRun,
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:           "status-matching-taskruns-pr",
			prStatus:         prStatusWithCondition,
			trs:              allTaskRuns,
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:           "orphaned-taskruns-pr",
			prStatus:         prStatusWithOrphans,
			trs:              allTaskRuns,
			runs:             singleRun,
			expectedPrStatus: prStatusRecovered,
		}, {
			prName:           "tr-and-run-from-another-pr",
			prStatus:         prStatusWithEmptyChildRefs,
			trs:              taskRunsFromAnotherPR,
			runs:             runsFromAnotherPR,
			expectedPrStatus: prStatusWithEmptyChildRefs,
		}, {
			prName:           "tr-and-run-with-no-owner",
			prStatus:         prStatusWithEmptyChildRefs,
			trs:              taskRunsWithNoOwner,
			runs:             runsWithNoOwner,
			expectedPrStatus: prStatusWithEmptyChildRefs,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.prName, func(t *testing.T) {
			logger := logtesting.TestLogger(t)

			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: tc.prName, UID: prUID},
				Status:     tc.prStatus,
			}

			updatePipelineRunStatusFromChildRefs(logger, pr, tc.trs, tc.runs)

			actualPrStatus := pr.Status

			actualChildRefs := actualPrStatus.ChildReferences
			if len(actualChildRefs) != 0 {
				var fixedChildRefs []v1beta1.ChildStatusReference
				re := regexp.MustCompile(`^[a-z\-]*?-(task|run)-[0-9]`)
				for _, cr := range actualChildRefs {
					cr.Name = fmt.Sprintf("%s-xxyyy", re.FindString(cr.Name))
					fixedChildRefs = append(fixedChildRefs, cr)
				}
				actualPrStatus.ChildReferences = fixedChildRefs
			}

			// Sort the ChildReferences to deal with annoying ordering issues.
			sort.Slice(actualPrStatus.ChildReferences, func(i, j int) bool {
				return actualPrStatus.ChildReferences[i].PipelineTaskName < actualPrStatus.ChildReferences[j].PipelineTaskName
			})

			if d := cmp.Diff(tc.expectedPrStatus, actualPrStatus); d != "" {
				t.Errorf("expected the PipelineRun status to match %#v. Diff %s", tc.expectedPrStatus, diff.PrintWantGot(d))
			}
		})
	}
}

func getTestTaskRunsAndRuns(t *testing.T) ([]*v1beta1.TaskRun, []*v1beta1.TaskRun, []*v1beta1.TaskRun, []*v1alpha1.Run, []*v1alpha1.Run, []*v1alpha1.Run) {
	allTaskRuns := []*v1beta1.TaskRun{
		parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/conditionCheck: pr-task-2-running-condition-check-xxyyy
    tekton.dev/conditionName: running-condition
    tekton.dev/pipelineTask: task-2
  name: pr-task-2-running-condition-check-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/conditionCheck: pr-task-3-successful-condition-check-xxyyy
    tekton.dev/conditionName: successful-condition
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-successful-condition-check-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/conditionCheck: pr-task-4-failed-condition-check-xxyyy
    tekton.dev/conditionName: failed-condition
    tekton.dev/pipelineTask: task-4
  name: pr-task-4-failed-condition-check-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
	}

	taskRunsFromAnotherPR := []*v1beta1.TaskRun{parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 22222222-2222-2222-2222-222222222222
`)}

	taskRunsWithNoOwner := []*v1beta1.TaskRun{parse.MustParseTaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
`)}

	allRuns := []*v1alpha1.Run{
		parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-2
  name: pr-run-2-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-3
  name: pr-run-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
	}

	runsFromAnotherPR := []*v1alpha1.Run{parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
  ownerReferences:
  - uid: 22222222-2222-2222-2222-222222222222
`)}

	runsWithNoOwner := []*v1alpha1.Run{parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
`)}

	return allTaskRuns, taskRunsFromAnotherPR, taskRunsWithNoOwner, allRuns, runsFromAnotherPR, runsWithNoOwner
}

func mustParsePipelineRunTaskRunStatus(t *testing.T, yamlStr string) *v1beta1.PipelineRunTaskRunStatus {
	var output v1beta1.PipelineRunTaskRunStatus
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing task run status %s: %v", yamlStr, err)
	}
	return &output
}

func mustParseChildStatusReference(t *testing.T, yamlStr string) v1beta1.ChildStatusReference {
	var output v1beta1.ChildStatusReference
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing task run status %s: %v", yamlStr, err)
	}
	return output
}
