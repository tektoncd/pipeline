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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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
	noTaskRuns     map[string]*v1.PipelineRunTaskRunStatus
	missingTaskRun map[string]*v1.PipelineRunTaskRunStatus
	foundTaskRun   map[string]*v1.PipelineRunTaskRunStatus
	recovered      map[string]*v1.PipelineRunTaskRunStatus
	simple         map[string]*v1.PipelineRunTaskRunStatus
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

	noTaskRuns := map[string]*v1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-2-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask2Yaml),
		"pr-task-3-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask3Yaml),
		"pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask4Yaml),
	}

	missingTaskRuns := map[string]*v1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-2-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask2Yaml),
		"pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask4Yaml),
	}

	foundTaskRun := map[string]*v1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-2-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask2Yaml),
		"pr-task-3-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask3Yaml),
		"pr-task-4-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask4Yaml),
	}

	recovered := map[string]*v1.PipelineRunTaskRunStatus{
		"pr-task-1-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask1Yaml),
		"pr-task-3-xxyyy": mustParsePipelineRunTaskRunStatus(t, prTask3Yaml),
	}

	simple := map[string]*v1.PipelineRunTaskRunStatus{
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

type updateStatusChildRefsData struct {
	noTaskRuns     []v1.ChildStatusReference
	missingTaskRun []v1.ChildStatusReference
	foundTaskRun   []v1.ChildStatusReference
	missingRun     []v1.ChildStatusReference
	recovered      []v1.ChildStatusReference
	simple         []v1.ChildStatusReference
	simpleRun      []v1.ChildStatusReference
}

func getUpdateStatusChildRefsData(t *testing.T) updateStatusChildRefsData {
	t.Helper()
	prTask1Yaml := `
apiVersion: tekton.dev/v1
kind: TaskRun
name: pr-task-1-xxyyy
pipelineTaskName: task-1
`

	prTask2Yaml := `
apiVersion: tekton.dev/v1
kind: TaskRun
name: pr-task-2-xxyyy
pipelineTaskName: task-2
`

	prTask3Yaml := `
apiVersion: tekton.dev/v1
kind: TaskRun
name: pr-task-3-xxyyy
pipelineTaskName: task-3
`

	prTask4Yaml := `
apiVersion: tekton.dev/v1
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
apiVersion: tekton.dev/v1
kind: TaskRun
name: pr-task-3-xxyyy
pipelineTaskName: task-3
`

	noTaskRuns := []v1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask3Yaml),
		mustParseChildStatusReference(t, prTask4Yaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	missingTaskRun := []v1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask4Yaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	foundTaskRun := []v1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask3NoStatusYaml),
		mustParseChildStatusReference(t, prTask4Yaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	missingRun := []v1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask2Yaml),
		mustParseChildStatusReference(t, prTask3Yaml),
		mustParseChildStatusReference(t, prTask4Yaml),
	}

	recovered := []v1.ChildStatusReference{
		mustParseChildStatusReference(t, prTask1Yaml),
		mustParseChildStatusReference(t, prTask3NoStatusYaml),
		mustParseChildStatusReference(t, prTask6Yaml),
	}

	simple := []v1.ChildStatusReference{mustParseChildStatusReference(t, prTask1Yaml)}

	simpleRun := []v1.ChildStatusReference{mustParseChildStatusReference(t, prTask6Yaml)}

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

	prStatusWithNoTaskRuns := v1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.noTaskRuns,
		},
	}

	prStatusMissingTaskRun := v1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.missingTaskRun,
		},
	}

	prStatusFoundTaskRun := v1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.foundTaskRun,
		},
	}

	prStatusMissingRun := v1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.missingRun,
		},
	}

	prStatusWithEmptyChildRefs := v1.PipelineRunStatus{
		Status:                  prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{},
	}

	prStatusWithOrphans := v1.PipelineRunStatus{
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
		PipelineRunStatusFields: v1.PipelineRunStatusFields{},
	}

	prStatusRecovered := v1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.recovered,
		},
	}

	prStatusRecoveredSimple := v1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			ChildReferences: childRefsPRStatusData.simple,
		},
	}

	prStatusRecoveredSimpleWithRun := v1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{
			ChildReferences: []v1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1beta1",
					Kind:       customRun,
				},
				Name:             "pr-run-6-xxyyy",
				PipelineTaskName: "task-6",
			}},
		},
	}

	allTaskRuns, taskRunsFromAnotherPR, taskRunsWithNoOwner, _, runsFromAnotherPR, runsWithNoOwner := getTestTaskRunsAndCustomRuns(t)

	singleRun := []*v1beta1.CustomRun{parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-6
  name: pr-run-6-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)}

	tcs := []struct {
		prName           string
		prStatus         v1.PipelineRunStatus
		trs              []*v1.TaskRun
		customRuns       []*v1beta1.CustomRun
		expectedPrStatus v1.PipelineRunStatus
	}{
		{
			prName:           "no-status-no-taskruns-or-runs",
			prStatus:         v1.PipelineRunStatus{},
			trs:              nil,
			customRuns:       nil,
			expectedPrStatus: v1.PipelineRunStatus{},
		}, {
			prName:           "status-no-taskruns-or-runs",
			prStatus:         prStatusWithNoTaskRuns,
			trs:              nil,
			customRuns:       nil,
			expectedPrStatus: prStatusWithNoTaskRuns,
		}, {
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyChildRefs,
			trs: []*v1.TaskRun{parse.MustParseV1TaskRun(t, `
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
			customRuns:       singleRun,
			expectedPrStatus: prStatusRecoveredSimpleWithRun,
		}, {
			prName:   "status-missing-taskruns",
			prStatus: prStatusMissingTaskRun,
			trs: []*v1.TaskRun{
				parse.MustParseV1TaskRun(t, `
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
			customRuns:       singleRun,
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
			customRuns:       singleRun,
			expectedPrStatus: prStatusRecovered,
		}, {
			prName:           "tr-and-run-from-another-pr",
			prStatus:         prStatusWithEmptyChildRefs,
			trs:              taskRunsFromAnotherPR,
			customRuns:       runsFromAnotherPR,
			expectedPrStatus: prStatusWithEmptyChildRefs,
		}, {
			prName:           "tr-and-run-with-no-owner",
			prStatus:         prStatusWithEmptyChildRefs,
			trs:              taskRunsWithNoOwner,
			customRuns:       runsWithNoOwner,
			expectedPrStatus: prStatusWithEmptyChildRefs,
		}, {
			prName:   "matrixed-taskruns-pr",
			prStatus: prStatusWithEmptyChildRefs,
			trs: []*v1.TaskRun{
				parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task
  name: pr-task-0-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
				parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
			},
			customRuns: nil,
			expectedPrStatus: v1.PipelineRunStatus{
				Status: prRunningStatus,
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: []v1.ChildStatusReference{
						mustParseChildStatusReference(t, `
apiVersion: tekton.dev/v1
kind: TaskRun
name: pr-task-0-xxyyy
pipelineTaskName: task
`),
						mustParseChildStatusReference(t, `
apiVersion: tekton.dev/v1
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

			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: tc.prName, UID: prUID},
				Status:     tc.prStatus,
			}

			updatePipelineRunStatusFromChildRefs(logger, pr, tc.trs, tc.customRuns)

			actualPrStatus := pr.Status

			actualChildRefs := actualPrStatus.ChildReferences
			if len(actualChildRefs) != 0 {
				var fixedChildRefs []v1.ChildStatusReference
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

	prStatusWithOrphans := v1.PipelineRunStatus{
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
		PipelineRunStatusFields: v1.PipelineRunStatusFields{},
	}

	prStatusWithEmptyEverything := func() v1.PipelineRunStatus {
		return v1.PipelineRunStatus{
			Status:                  prRunningStatus,
			PipelineRunStatusFields: v1.PipelineRunStatusFields{},
		}
	}

	allTaskRuns, _, _, _, _, _ := getTestTaskRunsAndCustomRuns(t)

	singleCustomRun := []*v1beta1.CustomRun{parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-6
  name: pr-run-6-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`)}

	tcs := []struct {
		prName             string
		prStatus           func() v1.PipelineRunStatus
		trs                []*v1.TaskRun
		runs               []*v1beta1.CustomRun
		expectedStatusTRs  map[string]*v1.PipelineRunTaskRunStatus
		expectedStatusRuns map[string]*v1.PipelineRunRunStatus
		expectedStatusCRs  []v1.ChildStatusReference
	}{
		{
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyEverything,
			trs: []*v1.TaskRun{parse.MustParseV1TaskRun(t, `
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
			expectedStatusRuns: map[string]*v1.PipelineRunRunStatus{
				"pr-run-6-xxyyy": {
					PipelineTaskName: "task-6",
					Status:           &v1beta1.CustomRunStatus{},
				},
			},
		}, {
			prName:            "orphaned-taskruns-pr",
			prStatus:          func() v1.PipelineRunStatus { return prStatusWithOrphans },
			trs:               allTaskRuns,
			runs:              singleCustomRun,
			expectedStatusTRs: taskRunsPRStatusData.recovered,
			expectedStatusCRs: childRefsPRStatusData.recovered,
			expectedStatusRuns: map[string]*v1.PipelineRunRunStatus{
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

			pr := &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: tc.prName, UID: prUID},
				Status:     tc.prStatus(),
			}

			if err := updatePipelineRunStatusFromChildObjects(ctx, logger, pr, tc.trs, tc.runs); err != nil {
				t.Fatalf("received an unexpected error: %v", err)
			}

			actualPrStatus := pr.Status

			actualChildRefs := actualPrStatus.ChildReferences
			if len(actualChildRefs) != 0 {
				var fixedChildRefs []v1.ChildStatusReference
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
		prStatus        v1.PipelineRunStatus
		expectedErrStrs []string
	}{
		{
			name: "empty everything",
			prStatus: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{},
			},
			expectedErrStrs: nil,
		}, {
			name: "error ChildObjects",
			prStatus: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: []v1.ChildStatusReference{
						{
							TypeMeta:         runtime.TypeMeta{Kind: taskRun},
							Name:             "t1",
							PipelineTaskName: "task-1",
						}, {
							TypeMeta:         runtime.TypeMeta{Kind: customRun},
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
			prStatus: v1.PipelineRunStatus{
				PipelineRunStatusFields: v1.PipelineRunStatusFields{
					ChildReferences: []v1.ChildStatusReference{
						{
							TypeMeta:         runtime.TypeMeta{Kind: taskRun},
							Name:             "t1",
							PipelineTaskName: "task-1",
						}, {
							TypeMeta:         runtime.TypeMeta{Kind: customRun},
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

func prStatusFromInputs(status duckv1.Status, taskRuns map[string]*v1.PipelineRunTaskRunStatus, runs map[string]*v1.PipelineRunRunStatus, childRefs []v1.ChildStatusReference) v1.PipelineRunStatus {
	prs := v1.PipelineRunStatus{
		Status:                  status,
		PipelineRunStatusFields: v1.PipelineRunStatusFields{},
	}

	prs.ChildReferences = append(prs.ChildReferences, childRefs...)

	return prs
}

func getTestTaskRunsAndCustomRuns(t *testing.T) ([]*v1.TaskRun, []*v1.TaskRun, []*v1.TaskRun, []*v1beta1.CustomRun, []*v1beta1.CustomRun, []*v1beta1.CustomRun) {
	t.Helper()
	allTaskRuns := []*v1.TaskRun{
		parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
		parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-3
  name: pr-task-3-xxyyy
  ownerReferences:
  - uid: 11111111-1111-1111-1111-111111111111
`),
	}

	taskRunsFromAnotherPR := []*v1.TaskRun{parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
  ownerReferences:
  - uid: 22222222-2222-2222-2222-222222222222
`)}

	taskRunsWithNoOwner := []*v1.TaskRun{parse.MustParseV1TaskRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: task-1
  name: pr-task-1-xxyyy
`)}

	allCustomRuns := []*v1beta1.CustomRun{
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

	customRunsFromAnotherPR := []*v1beta1.CustomRun{parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
  ownerReferences:
  - uid: 22222222-2222-2222-2222-222222222222
`)}

	customRunsWithNoOwner := []*v1beta1.CustomRun{parse.MustParseCustomRun(t, `
metadata:
  labels:
    tekton.dev/pipelineTask: run-1
  name: pr-run-1-xxyyy
`)}

	return allTaskRuns, taskRunsFromAnotherPR, taskRunsWithNoOwner, allCustomRuns, customRunsFromAnotherPR, customRunsWithNoOwner
}

func mustParsePipelineRunTaskRunStatus(t *testing.T, yamlStr string) *v1.PipelineRunTaskRunStatus {
	t.Helper()
	var output v1.PipelineRunTaskRunStatus
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing task run status %s: %v", yamlStr, err)
	}
	return &output
}

func mustParseChildStatusReference(t *testing.T, yamlStr string) v1.ChildStatusReference {
	t.Helper()
	var output v1.ChildStatusReference
	if err := yaml.Unmarshal([]byte(yamlStr), &output); err != nil {
		t.Fatalf("parsing task run status %s: %v", yamlStr, err)
	}
	return output
}

func lessChildReferences(i, j v1.ChildStatusReference) bool {
	return i.Name < j.Name
}
