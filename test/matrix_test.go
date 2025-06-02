//go:build e2e
// +build e2e

/*
Copyright 2023 The Tekton Authors

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
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var requireAlphaFeatureFlag = requireAnyGate(map[string]string{
	"enable-api-fields": "alpha",
})

// TestPipelineRunMatrixed is an integration test that verifies that a Matrixed PipelineRun
// succeeds with both `matrix params` and `matrix include params`. It also tests array indexing
// and whole array replacements by consuming results produced by other PipelineTasks.
func TestPipelineRunMatrixed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAlphaFeatureFlag)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)
	t.Logf("Creating Tasks in namespace %s", namespace)

	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: mytask
  namespace: %s
spec:
  params:
    - name: GOARCH
    - name: version
      default: ""
    - name: flags
      default: ""
    - name: context
      default: ""
    - name: package
      default: ""
  results:
  - name: str
    type: string
  steps:
    - name: echo
      image: mirror.gcr.io/alpine
      script: |
        echo -n "$(params.GOARCH) and $(params.version)" | tee $(results.str.path)
`, namespace))

	task1withresults := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: task1withresults
  namespace: %s
spec:
  results:
    - name: GOARCHs
      type: array
  steps:
    - name: produce-a-list-of-results
      image: mirror.gcr.io/bash
      script: |
        #!/usr/bin/env bash
        echo -n "[\"linux/amd64\",\"linux/ppc64le\"]" | tee $(results.GOARCHs.path)
`, namespace))

	task2withresults := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: task2withresults
  namespace: %s
spec:
  results:
    - name: versions
      type: array
  steps:
    - name: produce-a-list-of-versions
      image: mirror.gcr.io/bash
      script: |
        #!/usr/bin/env bash
        echo -n "[\"go1.17\",\"go1.18.1\"]" | tee $(results.versions.path)
`, namespace))

	task3printer := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: printer
  namespace: %s
spec:
  params:
    - name: platform
      value: "default-platform"
  steps:
    - name: produce-a-list-of-versions
      image: mirror.gcr.io/bash
      script: |
        #!/usr/bin/env bash
        echo "platform: $(params.platform)"
`, namespace))

	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}
	if _, err := c.V1TaskClient.Create(ctx, task1withresults, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task1withresults.Name, err)
	}
	if _, err := c.V1TaskClient.Create(ctx, task2withresults, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task2withresults.Name, err)
	}
	if _, err := c.V1TaskClient.Create(ctx, task3printer, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task3printer.Name, err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
    - name: pt1-with-result
      taskRef:
        name: task1withresults
        kind: Task
    - name: pt2-with-result
      taskRef:
        name: task2withresults
        kind: Task
    - name: matrix-include
      taskRef:
        name: mytask
      matrix:
        params:
          - name: GOARCH
            value: $(tasks.pt1-with-result.results.GOARCHs[*])
          - name: version
            value:
              - $(tasks.pt2-with-result.results.versions[0])
              - $(tasks.pt2-with-result.results.versions[1])
        include:
         - name: common-package
           params:
            - name: package
              value: path/to/common/package/
         - name: go117-context
           params:
            - name: version
              value: go1.17
            - name: context
              value: path/to/go117/context
         - name: non-existent-arch
           params:
            - name: GOARCH
              value: I-do-not-exist
    - name: printer-matrix
      taskRef:
        name: printer
      matrix:
        params:
          - name: platform
            value: $(tasks.matrix-include.results.str[*])
`, helpers.ObjectNameForTest(t), namespace))

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  serviceAccountName: "default"
  pipelineRef:
    name: %s
`, helpers.ObjectNameForTest(t), namespace, pipeline.Name))

	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineRun.Name, err)
	}
	prName := pipelineRun.Name

	expectedTaskRuns := []v1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-matrix-include-0",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "context",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "mytask", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  "True",
				Reason:  "Succeeded",
				Message: "All Steps have completed executing",
			}}},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
				Results: []v1.TaskRunResult{{
					Name:  "str",
					Type:  v1.ResultsTypeString,
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64 and go1.17"},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-matrix-include-1",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "context",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "mytask", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  "True",
				Reason:  "Succeeded",
				Message: "All Steps have completed executing",
			}}},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
				Results: []v1.TaskRunResult{{
					Name:  "str",
					Type:  v1.ResultsTypeString,
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le and go1.17"},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-matrix-include-2",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "mytask", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  "True",
				Reason:  "Succeeded",
				Message: "All Steps have completed executing",
			}}},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
				Results: []v1.TaskRunResult{{
					Name:  "str",
					Type:  v1.ResultsTypeString,
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64 and go1.18.1"},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-matrix-include-3",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "mytask", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  "True",
				Reason:  "Succeeded",
				Message: "All Steps have completed executing",
			}}},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
				Results: []v1.TaskRunResult{{
					Name:  "str",
					Type:  v1.ResultsTypeString,
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le and go1.18.1"},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-matrix-include-4",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "mytask", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  "True",
				Reason:  "Succeeded",
				Message: "All Steps have completed executing",
			}}},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
				Results: []v1.TaskRunResult{{
					Name:  "str",
					Type:  v1.ResultsTypeString,
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist and "},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-printer-matrix-0",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64 and go1.17"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "printer", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Reason:  "Succeeded",
					Message: "All Steps have completed executing",
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-printer-matrix-1",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le and go1.17"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "printer", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Reason:  "Succeeded",
					Message: "All Steps have completed executing",
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-printer-matrix-2",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64 and go1.18.1"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "printer", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Reason:  "Succeeded",
					Message: "All Steps have completed executing",
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-printer-matrix-3",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le and go1.18.1"},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "printer", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Reason:  "Succeeded",
					Message: "All Steps have completed executing",
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-printer-matrix-4",
		},
		Spec: v1.TaskRunSpec{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist and "},
			}},
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "printer", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Reason:  "Succeeded",
					Message: "All Steps have completed executing",
				}},
			},
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Artifacts: &v1.Artifacts{},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-pt1-with-result",
		},
		Spec: v1.TaskRunSpec{
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "task1withresults", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "GOARCHs",
					Type:  "array",
					Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le"}},
				}},
				Artifacts: &v1.Artifacts{},
			},
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  "True",
				Reason:  "Succeeded",
				Message: "All Steps have completed executing",
			}}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name: "pr-pt2-with-result",
		},
		Spec: v1.TaskRunSpec{
			ServiceAccountName: "default",
			TaskRef:            &v1.TaskRef{Name: "task2withresults", Kind: v1.NamespacedTaskKind},
		},
		Status: v1.TaskRunStatus{
			TaskRunStatusFields: v1.TaskRunStatusFields{
				Results: []v1.TaskRunResult{{
					Name:  "versions",
					Type:  "array",
					Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"go1.17", "go1.18.1"}},
				}},
				Artifacts: &v1.Artifacts{},
			},
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  "True",
				Reason:  "Succeeded",
				Message: "All Steps have completed executing",
			}}},
		},
	}}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}

	actualTaskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + prName})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", prName, err)
	}

	actualTaskRuns := actualTaskrunList.Items
	ignoreTypeMeta := cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	ignoreObjectMeta := cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Name", "Namespace", "UID", "Generation", "CreationTimestamp", "OwnerReferences", "ManagedFields", "Labels", "ResourceVersion", "Annotations")
	ignoreTaskRunStatusFields := cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime", "PodName", "Steps", "TaskSpec", "Provenance", "Sidecars")
	ignoreLastTransitionTime := cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")
	sortTaskRuns := cmpopts.SortSlices(func(x, y *v1.TaskRun) bool { return x.Name < y.Name })
	ignoreSericeAccountName := cmpopts.IgnoreFields(v1.TaskRunSpec{}, "ServiceAccountName")
	ignoreTimeout := cmpopts.IgnoreFields(v1.TaskRunSpec{}, "Timeout")

	if d := cmp.Diff(expectedTaskRuns, actualTaskRuns, ignoreLastTransitionTime, ignoreObjectMeta, ignoreTypeMeta, ignoreTaskRunStatusFields, ignoreSericeAccountName, ignoreTimeout, sortTaskRuns); d != "" {
		t.Fatalf("Did not get expected TaskRuns: %s", diff.PrintWantGot(d))
	}

	t.Logf("Successfully finished test TestPipelineRunMatrixed")
}

// TestPipelineRunMatrixedFailed is an integration test with a Matrixed PipelineRun
// that contains an unsuccessful TaskRun. This test verifies that the taskRun will fail,
// which will cause the entire PipelineRun to fail.
func TestPipelineRunMatrixedFailed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t, requireAlphaFeatureFlag)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)
	t.Logf("Creating Task in namespace %s", namespace)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: mytask
  namespace: %s
spec:
  params:
    - name: exit-code
  steps:
    - name: echo
      image: mirror.gcr.io/alpine
      script: |
        exit "$(params.exit-code)"
`, namespace))

	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
	}

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  serviceAccountName: "default"
  pipelineSpec:
    tasks:
      - name: exit-with-zero-and-non-zero
        taskRef:
          name: mytask
        matrix:
          params:
          - name: exit-code
            value:
              - "1"
`, helpers.ObjectNameForTest(t), namespace))
	prName := pipelineRun.Name

	t.Logf("Creating PipelineRun %s", prName)
	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to fail", namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunFailed(prName), "PipelineRunFailed", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish: %s", err)
	}

	pr, err := c.V1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun %s: %s", pr.Name, err)
	}

	if pr.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
		t.Errorf("Expected PipelineRun to fail but found condition: %s", pr.Status.GetCondition(apis.ConditionSucceeded))
	}

	actualTaskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil || len(actualTaskrunList.Items) == 0 {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", prName, err)
	}
	if !isFailed(t, "", actualTaskrunList.Items[0].Status.Conditions) {
		t.Fatalf("taskRun should have been a failure")
	}
	t.Logf("Successfully finished test TestPipelineRunMatrixedFailed")
}
