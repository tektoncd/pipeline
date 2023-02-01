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
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	logtesting "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/yaml"
)

type updateStatusTaskRunsData struct {
	noTaskRuns     map[string]*v1beta1.PipelineRunTaskRunStatus
	missingTaskRun map[string]*v1beta1.PipelineRunTaskRunStatus
	foundTaskRun   map[string]*v1beta1.PipelineRunTaskRunStatus
	recovered      map[string]*v1beta1.PipelineRunTaskRunStatus
	simple         map[string]*v1beta1.PipelineRunTaskRunStatus
}

func getUpdateStatusTaskRunsData(t *testing.T) updateStatusTaskRunsData {
	t.Helper()
	prTask1Yaml := `
pipelineTaskName: task-1
status: {}
`
	prTask2Yaml := `
pipelineTaskName: task-2
`
	prTask3Yaml := `
pipelineTaskName: task-3
status: {}
`

	prTask4Yaml := `
pipelineTaskName: task-4
`

	noTaskRuns := map[string]*v1beta1.PipelineRunTaskRunStatus{
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
		"pr-task-3-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask3Yaml),
		"pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask4Yaml),
	}

	recovered := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-3-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask3Yaml),
	}

	simple := map[string]*v1beta1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
	}

	return updateStatusTaskRunsData{
		noTaskRuns:     noTaskRuns,
		missingTaskRun: missingTaskRuns,
		foundTaskRun:   foundTaskRun,
		recovered:      recovered,
		simple:         simple,
	}
}

func TestUpdatePipelineRunStatusFromTaskRuns(t *testing.T) {
	prUID := types.UID("11111111-1111-1111-1111-111111111111")

	taskRunsPRStatusData := getUpdateStatusTaskRunsData(t)

	prRunningStatus := duckv1.Status{
		Conditions: []apis.Condition{
			{
				Type:    "Succeeded",
				Status:  "Unknown",
				Reason:  "Running",
				Message: "Not all Tasks in the Pipeline have finished executing",
			},
		},
	}

	prStatusWithNoTaskRuns := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: taskRunsPRStatusData.noTaskRuns,
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
		Status: duckv1.Status{
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
			prStatus:         prStatusWithNoTaskRuns,
			trs:              nil,
			expectedPrStatus: prStatusWithNoTaskRuns,
		}, {
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyTaskRuns,
			trs: []*v1beta1.TaskRun{parse.MustParseV1beta1TaskRun(t, `
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
				parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
			},
			expectedPrStatus: prStatusFoundTaskRun,
		}, {
			prName:           "status-matching-taskruns-pr",
			prStatus:         prStatusWithNoTaskRuns,
			trs:              allTaskRuns,
			expectedPrStatus: prStatusWithNoTaskRuns,
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

	prRunningStatus := duckv1.Status{
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
					Status:           &v1beta1.CustomRunStatus{},
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
					Status:           &v1beta1.CustomRunStatus{},
				},
				"pr-run-2-xxyyy": {
					PipelineTaskName: "run-2",
					Status:           nil,
				},
				"pr-run-3-xxyyy": {
					PipelineTaskName: "run-3",
					Status:           &v1beta1.CustomRunStatus{},
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
					Status:           &v1beta1.CustomRunStatus{},
				},
			},
		},
	}

	_, _, _, allRuns, runsFromAnotherPR, runsWithNoOwner := getTestTaskRunsAndRuns(t)

	tcs := []struct {
		prName           string
		prStatus         v1beta1.PipelineRunStatus
		runs             []v1beta1.RunObject
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
			runs: []v1beta1.RunObject{parse.MustParseCustomRun(t, `
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
			runs: []v1beta1.RunObject{parse.MustParseCustomRun(t, `
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

			updatePipelineRunStatusFromCustomRunsOrRuns(logger, pr, tc.runs)
			actualPrStatus := pr.Status

			if d := cmp.Diff(tc.expectedPrStatus, actualPrStatus); d != "" {
				t.Errorf("expected the PipelineRun status to match %#v. Diff %s", tc.expectedPrStatus, diff.PrintWantGot(d))
			}
		})
	}
}

type updateStatusChildRefsData struct {
	noTaskRuns     []v1beta1.ChildStatusReference
	missingTaskRun []v1beta1.ChildStatusReference
	foundTaskRun   []v1beta1.ChildStatusReference
	missingRun     []v1beta1.ChildStatusReference
	recovered      []v1beta1.ChildStatusReference
	simple         []v1beta1.ChildStatusReference
	simpleRun      []v1beta1.ChildStatusReference
}

func getUpdateStatusChildRefsData(t *testing.T) updateStatusChildRefsData {
	t.Helper()
	prTask1Yaml := `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-1-xxyyy
pipelineTaskName: task-1
`

	prTask2Yaml := `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-2-xxyyy
pipelineTaskName: task-2
`

	prTask3Yaml := `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-3-xxyyy
pipelineTaskName: task-3
`

	prTask4Yaml := `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-4-xxyyy
pipelineTaskName: task-4
`

	prTask6Yaml := `
apiVersion: tekton.dev/v1beta1
kind: CustomRun
name: pr-run-6-xxyyy
pipelineTaskName: task-6
`

	prTask3NoStatusYaml := `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-3-xxyyy
pipelineTaskName: task-3
`

	noTaskRuns := []v1beta1.ChildStatusReference{
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
		mustParseChildStatusReference(t, prTask3NoStatusYaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	simple := []v1beta1.ChildStatusReference{mustParseChildStatusReference(t, prTask1Yaml)}

	simpleRun := []v1beta1.ChildStatusReference{mustParseChildStatusReference(t, prTask6Yaml)}

	return updateStatusChildRefsData{
		noTaskRuns:     noTaskRuns,
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

	prRunningStatus := duckv1.Status{
		Conditions: []apis.Condition{
			{
				Type:    "Succeeded",
				Status:  "Unknown",
				Reason:  "Running",
				Message: "Not all Tasks in the Pipeline have finished executing",
			},
		},
	}

	prStatusWithNoTaskRuns := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.noTaskRuns,
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
		Status: duckv1.Status{
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
					APIVersion: "tekton.dev/v1beta1",
					Kind:       "CustomRun",
				},
				Name:             "pr-run-6-xxyyy",
				PipelineTaskName: "task-6",
			}},
		},
	}

	allTaskRuns, taskRunsFromAnotherPR, taskRunsWithNoOwner, _, runsFromAnotherPR, runsWithNoOwner := getTestTaskRunsAndRuns(t)

	singleRun := []v1beta1.RunObject{parse.MustParseCustomRun(t, `
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
		runs             []v1beta1.RunObject
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
			prStatus:         prStatusWithNoTaskRuns,
			trs:              nil,
			runs:             nil,
			expectedPrStatus: prStatusWithNoTaskRuns,
		}, {
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyChildRefs,
			trs: []*v1beta1.TaskRun{parse.MustParseV1beta1TaskRun(t, `
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
				parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
			},
			expectedPrStatus: prStatusFoundTaskRun,
		}, {
			prName:           "status-missing-runs",
			prStatus:         prStatusMissingRun,
			runs:             singleRun,
			expectedPrStatus: prStatusWithNoTaskRuns,
		}, {
			prName:           "status-matching-taskruns-pr",
			prStatus:         prStatusWithNoTaskRuns,
			trs:              allTaskRuns,
			expectedPrStatus: prStatusWithNoTaskRuns,
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
		}, {
			prName:   "matrixed-taskruns-pr",
			prStatus: prStatusWithEmptyChildRefs,
			trs: []*v1beta1.TaskRun{
				parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task
  name: pr-task-0-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
				parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
			},
			runs: nil,
			expectedPrStatus: v1beta1.PipelineRunStatus{
				Status: prRunningStatus,
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: []v1beta1.ChildStatusReference{
						mustParseChildStatusReference(t, `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-0-xxyyy
pipelineTaskName: task
`),
						mustParseChildStatusReference(t, `
apiVersion: tekton.dev/v1beta1
kind: TaskRun
name: pr-task-1-xxyyy
pipelineTaskName: task
`),
					},
				},
			},
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

			if d := cmp.Diff(tc.expectedPrStatus, actualPrStatus, cmpopts.SortSlices(lessChildReferences)); d != "" {
				t.Errorf("expected the PipelineRun status to match %#v. Diff %s", tc.expectedPrStatus, diff.PrintWantGot(d))
			}
		})
	}
}

func TestUpdatePipelineRunStatusFromChildObjects(t *testing.T) {
	prUID := types.UID("11111111-1111-1111-1111-111111111111")

	childRefsPRStatusData := getUpdateStatusChildRefsData(t)
	taskRunsPRStatusData := getUpdateStatusTaskRunsData(t)

	prRunningStatus := duckv1.Status{
		Conditions: []apis.Condition{
			{
				Type:    "Succeeded",
				Status:  "Unknown",
				Reason:  "Running",
				Message: "Not all Tasks in the Pipeline have finished executing",
			},
		},
	}

	prStatusWithOrphans := v1beta1.PipelineRunStatus{
		Status: duckv1.Status{
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

	prStatusWithEmptyEverything := func() v1beta1.PipelineRunStatus {
		return v1beta1.PipelineRunStatus{
			Status:                  prRunningStatus,
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{},
		}
	}

	allTaskRuns, _, _, _, _, _ := getTestTaskRunsAndRuns(t)

	singleCustomRun := []v1beta1.RunObject{parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-6
  name: pr-run-6-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)}

	singleRunWithStatus := []v1beta1.RunObject{parse.MustParseRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-6
  name: pr-run-6-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
status:
  conditions:
  - status: Unknown
    type: Succeeded
  startTime: "2021-12-31T23:58:59Z"
`)}

	tcs := []struct {
		prName             string
		prStatus           func() v1beta1.PipelineRunStatus
		trs                []*v1beta1.TaskRun
		runs               []v1beta1.RunObject
		expectedStatusTRs  map[string]*v1beta1.PipelineRunTaskRunStatus
		expectedStatusRuns map[string]*v1beta1.PipelineRunRunStatus
		expectedStatusCRs  []v1beta1.ChildStatusReference
	}{
		{
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyEverything,
			trs: []*v1beta1.TaskRun{parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)},
			expectedStatusCRs: childRefsPRStatusData.simple,
			expectedStatusTRs: taskRunsPRStatusData.simple,
		}, {
			prName:            "status-nil-runs",
			prStatus:          prStatusWithEmptyEverything,
			runs:              singleCustomRun,
			expectedStatusCRs: childRefsPRStatusData.simpleRun,
			expectedStatusRuns: map[string]*v1beta1.PipelineRunRunStatus{
				"pr-run-6-xxyyy": {
					PipelineTaskName: "task-6",
					Status:           &v1beta1.CustomRunStatus{},
				},
			},
		}, {
			prName:   "status-nil-runs-with-alpha-run",
			prStatus: prStatusWithEmptyEverything,
			runs:     singleRunWithStatus,
			expectedStatusCRs: []v1beta1.ChildStatusReference{mustParseChildStatusReference(t, `
apiVersion: tekton.dev/v1alpha1
kind: Run
name: pr-run-6-xxyyy
pipelineTaskName: task-6
`)},
			expectedStatusRuns: map[string]*v1beta1.PipelineRunRunStatus{
				"pr-run-6-xxyyy": {
					PipelineTaskName: "task-6",
					Status: &v1beta1.CustomRunStatus{
						Status: duckv1.Status{
							Conditions: []apis.Condition{{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionUnknown,
							}},
						},
						CustomRunStatusFields: v1beta1.CustomRunStatusFields{
							StartTime: &metav1.Time{Time: time.Date(2021, 12, 31, 23, 58, 59, 0, time.UTC)},
						},
					},
				},
			},
		}, {
			prName:            "orphaned-taskruns-pr",
			prStatus:          func() v1beta1.PipelineRunStatus { return prStatusWithOrphans },
			trs:               allTaskRuns,
			runs:              singleCustomRun,
			expectedStatusTRs: taskRunsPRStatusData.recovered,
			expectedStatusCRs: childRefsPRStatusData.recovered,
			expectedStatusRuns: map[string]*v1beta1.PipelineRunRunStatus{
				"pr-run-6-xxyyy": {
					PipelineTaskName: "task-6",
					Status:           &v1beta1.CustomRunStatus{},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.prName, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.NewStore(logtesting.TestLogger(t))

			ctx = cfg.ToContext(ctx)
			logger := logtesting.TestLogger(t)

			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: tc.prName, UID: prUID},
				Status:     tc.prStatus(),
			}

			if err := updatePipelineRunStatusFromChildObjects(ctx, logger, pr, tc.trs, tc.runs); err != nil {
				t.Fatalf("received an unexpected error: %v", err)
			}

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

			expectedPRStatus := prStatusFromInputs(prRunningStatus, tc.expectedStatusTRs, tc.expectedStatusRuns, tc.expectedStatusCRs)

			if d := cmp.Diff(expectedPRStatus, actualPrStatus, cmpopts.SortSlices(lessChildReferences)); d != "" {
				t.Errorf("expected the PipelineRun status to match %#v. Diff %s", expectedPRStatus, diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateChildObjectsInPipelineRunStatus(t *testing.T) {
	testCases := []struct {
		name            string
		prStatus        v1beta1.PipelineRunStatus
		expectedErrStrs []string
	}{
		{
			name: "empty everything",
			prStatus: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{},
			},
			expectedErrStrs: nil,
		}, {
			name: "error ChildObjects",
			prStatus: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: []v1beta1.ChildStatusReference{
						{
							TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
							Name:             "t1",
							PipelineTaskName: "task-1",
						}, {
							TypeMeta:         runtime.TypeMeta{Kind: "Run"},
							Name:             "r1",
							PipelineTaskName: "run-1",
						}, {
							TypeMeta:         runtime.TypeMeta{Kind: "UnknownKind"},
							Name:             "u1",
							PipelineTaskName: "unknown-1",
						},
					},
				},
			},
			expectedErrStrs: []string{
				"child with name u1 has unknown kind UnknownKind",
			},
		}, {
			name: "valid ChildObjects",
			prStatus: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					ChildReferences: []v1beta1.ChildStatusReference{
						{
							TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
							Name:             "t1",
							PipelineTaskName: "task-1",
						}, {
							TypeMeta:         runtime.TypeMeta{Kind: "Run"},
							Name:             "r1",
							PipelineTaskName: "run-1",
						},
					},
				},
			},
			expectedErrStrs: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.NewStore(logtesting.TestLogger(t))
			cfg.OnConfigChanged(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
			})
			ctx = cfg.ToContext(ctx)

			err := validateChildObjectsInPipelineRunStatus(ctx, tc.prStatus)

			if len(tc.expectedErrStrs) == 0 {
				if err != nil {
					t.Errorf("expected no error, but got %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected an error, but did not receive one")
				}

				for _, errStr := range tc.expectedErrStrs {
					if !strings.Contains(err.Error(), errStr) {
						t.Errorf("expected to find '%s' but did not find in error %s", errStr, err.Error())
					}
				}
			}
		})
	}
}

func prStatusFromInputs(status duckv1.Status, taskRuns map[string]*v1beta1.PipelineRunTaskRunStatus, runs map[string]*v1beta1.PipelineRunRunStatus, childRefs []v1beta1.ChildStatusReference) v1beta1.PipelineRunStatus {
	prs := v1beta1.PipelineRunStatus{
		Status:                  status,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{},
	}

	prs.ChildReferences = append(prs.ChildReferences, childRefs...)

	return prs
}

func getTestTaskRunsAndRuns(t *testing.T) ([]*v1beta1.TaskRun, []*v1beta1.TaskRun, []*v1beta1.TaskRun, []v1beta1.RunObject, []v1beta1.RunObject, []v1beta1.RunObject) {
	t.Helper()
	allTaskRuns := []*v1beta1.TaskRun{
		parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
	}

	taskRunsFromAnotherPR := []*v1beta1.TaskRun{parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 22222222-2222-2222-2222-222222222222
`)}

	taskRunsWithNoOwner := []*v1beta1.TaskRun{parse.MustParseV1beta1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
`)}

	allRuns := []v1beta1.RunObject{
		parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-2
  name: pr-run-2-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-3
  name: pr-run-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
	}

	runsFromAnotherPR := []v1beta1.RunObject{parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
  ownerReferences:
  - uid: 22222222-2222-2222-2222-222222222222
`)}

	runsWithNoOwner := []v1beta1.RunObject{parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
`)}

	return allTaskRuns, taskRunsFromAnotherPR, taskRunsWithNoOwner, allRuns, runsFromAnotherPR, runsWithNoOwner
}

func mustParsePipelineRunTaskRunStatus(t *testing.T, yamlStr string) *v1beta1.PipelineRunTaskRunStatus {
	t.Helper()
	var output v1beta1.PipelineRunTaskRunStatus
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing task run status %s: %v", yamlStr, err)
	}
	return &output
}

func mustParseChildStatusReference(t *testing.T, yamlStr string) v1beta1.ChildStatusReference {
	t.Helper()
	var output v1beta1.ChildStatusReference
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing task run status %s: %v", yamlStr, err)
	}
	return output
}

func lessChildReferences(i, j v1beta1.ChildStatusReference) bool {
	return i.Name < j.Name
}
