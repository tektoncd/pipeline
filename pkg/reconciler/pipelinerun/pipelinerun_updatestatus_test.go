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
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func getUpdateStatusTaskRunsData() updateStatusTaskRunsData {
	// PipelineRunConditionCheckStatus recovered by updatePipelineRunStatusFromTaskRuns
	// It does not include the status, which is then retrieved via the regular reconcile
	prccs2Recovered := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-2-running-condition-check-xxyyy": {
			ConditionName: "running-condition-0",
		},
	}
	prccs3Recovered := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-3-successful-condition-check-xxyyy": {
			ConditionName: "successful-condition-0",
		},
	}
	prccs4Recovered := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-4-failed-condition-check-xxyyy": {
			ConditionName: "failed-condition-0",
		},
	}

	// PipelineRunConditionCheckStatus full is used to test the behaviour of updatePipelineRunStatusFromTaskRuns
	// when no orphan TaskRuns are found, to check we don't alter good ones
	prccs2Full := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-2-running-condition-check-xxyyy": {
			ConditionName: "running-condition-0",
			Status: &v1beta1.ConditionCheckStatus{
				ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
					Check: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionUnknown}},
				},
			},
		},
	}
	prccs3Full := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-3-successful-condition-check-xxyyy": {
			ConditionName: "successful-condition-0",
			Status: &v1beta1.ConditionCheckStatus{
				ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
					Check: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}},
				},
			},
		},
	}
	prccs4Full := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-4-failed-condition-check-xxyyy": {
			ConditionName: "failed-condition-0",
			Status: &v1beta1.ConditionCheckStatus{
				ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
					Check: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 127},
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse}},
				},
			},
		},
	}

	return updateStatusTaskRunsData{
		withConditions: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"pr-task-1-xxyyy": {
				PipelineTaskName: "task-1",
				Status:           &v1beta1.TaskRunStatus{},
			},
			"pr-task-2-xxyyy": {
				PipelineTaskName: "task-2",
				Status:           nil,
				ConditionChecks:  prccs2Full,
			},
			"pr-task-3-xxyyy": {
				PipelineTaskName: "task-3",
				Status:           &v1beta1.TaskRunStatus{},
				ConditionChecks:  prccs3Full,
			},
			"pr-task-4-xxyyy": {
				PipelineTaskName: "task-4",
				Status:           nil,
				ConditionChecks:  prccs4Full,
			},
		},
		missingTaskRun: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"pr-task-1-xxyyy": {
				PipelineTaskName: "task-1",
				Status:           &v1beta1.TaskRunStatus{},
			},
			"pr-task-2-xxyyy": {
				PipelineTaskName: "task-2",
				Status:           nil,
				ConditionChecks:  prccs2Full,
			},
			"pr-task-4-xxyyy": {
				PipelineTaskName: "task-4",
				Status:           nil,
				ConditionChecks:  prccs4Full,
			},
		},
		foundTaskRun: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"pr-task-1-xxyyy": {
				PipelineTaskName: "task-1",
				Status:           &v1beta1.TaskRunStatus{},
			},
			"pr-task-2-xxyyy": {
				PipelineTaskName: "task-2",
				Status:           nil,
				ConditionChecks:  prccs2Full,
			},
			"pr-task-3-xxyyy": {
				PipelineTaskName: "task-3",
				Status:           &v1beta1.TaskRunStatus{},
				ConditionChecks:  prccs3Recovered,
			},
			"pr-task-4-xxyyy": {
				PipelineTaskName: "task-4",
				Status:           nil,
				ConditionChecks:  prccs4Full,
			},
		},
		recovered: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"pr-task-1-xxyyy": {
				PipelineTaskName: "task-1",
				Status:           &v1beta1.TaskRunStatus{},
			},
			"orphaned-taskruns-pr-task-2-xxyyy": {
				PipelineTaskName: "task-2",
				Status:           nil,
				ConditionChecks:  prccs2Recovered,
			},
			"pr-task-3-xxyyy": {
				PipelineTaskName: "task-3",
				Status:           &v1beta1.TaskRunStatus{},
				ConditionChecks:  prccs3Recovered,
			},
			"orphaned-taskruns-pr-task-4-xxyyy": {
				PipelineTaskName: "task-4",
				Status:           nil,
				ConditionChecks:  prccs4Recovered,
			},
		},
		simple: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"pr-task-1-xxyyy": {
				PipelineTaskName: "task-1",
				Status:           &v1beta1.TaskRunStatus{},
			},
		},
	}
}

func TestUpdatePipelineRunStatusFromTaskRuns(t *testing.T) {

	prUID := types.UID("11111111-1111-1111-1111-111111111111")
	otherPrUID := types.UID("22222222-2222-2222-2222-222222222222")

	taskRunsPRStatusData := getUpdateStatusTaskRunsData()

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

	allTaskRuns := []*v1beta1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-1",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-2-running-condition-check-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-2",
					pipeline.ConditionCheckKey:    "pr-task-2-running-condition-check-xxyyy",
					pipeline.ConditionNameKey:     "running-condition",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-3-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-3",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-3-successful-condition-check-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-3",
					pipeline.ConditionCheckKey:    "pr-task-3-successful-condition-check-xxyyy",
					pipeline.ConditionNameKey:     "successful-condition",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-4-failed-condition-check-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-4",
					pipeline.ConditionCheckKey:    "pr-task-4-failed-condition-check-xxyyy",
					pipeline.ConditionNameKey:     "failed-condition",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
	}

	taskRunsFromAnotherPR := []*v1beta1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-1",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: otherPrUID}},
			},
		},
	}

	taskRunsWithNoOwner := []*v1beta1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-1",
				},
			},
		},
	}

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
			trs: []*v1beta1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pr-task-1-xxyyy",
						Labels: map[string]string{
							pipeline.PipelineTaskLabelKey: "task-1",
						},
						OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
					},
				},
			},
			expectedPrStatus: prStatusRecoveredSimple,
		}, {
			prName:   "status-missing-taskruns",
			prStatus: prStatusMissingTaskRun,
			trs: []*v1beta1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pr-task-3-xxyyy",
						Labels: map[string]string{
							pipeline.PipelineTaskLabelKey: "task-3",
						},
						OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pr-task-3-successful-condition-check-xxyyy",
						Labels: map[string]string{
							pipeline.PipelineTaskLabelKey: "task-3",
							pipeline.ConditionCheckKey:    "pr-task-3-successful-condition-check-xxyyy",
							pipeline.ConditionNameKey:     "successful-condition",
						},
						OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
					},
				},
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

			// TODO(abayer): Change function call when TEP-0100 impl is done.
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
	otherPrUID := types.UID("22222222-2222-2222-2222-222222222222")

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

	allRuns := []*v1alpha1.Run{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-run-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "run-1",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-run-2-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "run-2",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-run-3-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "run-3",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
	}

	runsFromAnotherPR := []*v1alpha1.Run{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-run-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "run-1",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: otherPrUID}},
			},
		},
	}

	runsWithNoOwner := []*v1alpha1.Run{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-run-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "run-1",
				},
			},
		},
	}

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
			runs: []*v1alpha1.Run{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pr-run-1-xxyyy",
						Labels: map[string]string{
							pipeline.PipelineTaskLabelKey: "run-1",
						},
						OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
					},
				},
			},
			expectedPrStatus: prStatusRecoveredSimple,
		}, {
			prName:   "status-missing-runs",
			prStatus: prStatusWithSomeRuns,
			runs: []*v1alpha1.Run{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pr-run-3-xxyyy",
					Labels: map[string]string{
						pipeline.PipelineTaskLabelKey: "run-3",
					},
					OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
				},
			}},
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
