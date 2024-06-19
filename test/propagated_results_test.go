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
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestPropagatedResults(t *testing.T) {
	t.Parallel()

	// Ignore the Results when comparing the Runs directly. Instead, check the results separately.
	// This is because the order of resutls changes and makes the test very flaky.
	ignoreTaskRunStatusFields := cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "Results")
	sortTaskRunResults := cmpopts.SortSlices(func(x, y v1.TaskRunResult) bool { return x.Name < y.Name })

	ignorePipelineRunStatusFields := cmpopts.IgnoreFields(v1.PipelineRunStatusFields{}, "Provenance")
	ignoreTaskRunStatus := cmpopts.IgnoreFields(v1.TaskRunStatusFields{}, "StartTime", "CompletionTime", "Sidecars", "Provenance")
	requireAlphaFeatureFlag = requireAnyGate(map[string]string{
		"enable-api-fields": "alpha",
	})

	type tests struct {
		name            string
		pipelineName    string
		pipelineRunFunc func(*testing.T, string) (*v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun)
	}

	tds := []tests{{
		name:            "propagated all type results",
		pipelineName:    "propagated-all-type-results",
		pipelineRunFunc: getPropagatedResultPipelineRun,
	}}

	for _, td := range tds {
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			c, namespace := setup(ctx, t, requireAlphaFeatureFlag)

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			pipelineRun, expectedResolvedPipelineRun, expectedTaskRuns := td.pipelineRunFunc(t, namespace)

			prName := pipelineRun.Name
			_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}
			cl, _ := c.V1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
			d := cmp.Diff(expectedResolvedPipelineRun, cl,
				ignoreTypeMeta,
				ignoreObjectMeta,
				ignoreCondition,
				ignorePipelineRunStatus,
				ignoreTaskRunStatus,
				ignoreConditions,
				ignoreContainerStates,
				ignoreStepState,
				ignoreSAPipelineRunSpec,
				ignorePipelineRunStatusFields,
				ignoreTaskRunStatusFields,
			)
			if d != "" {
				t.Fatalf(`The resolved spec does not match the expected spec. Here is the diff: %v`, d)
			}
			for _, tr := range expectedTaskRuns {
				t.Logf("Checking Taskrun %s", tr.Name)
				taskrun, _ := c.V1TaskRunClient.Get(ctx, tr.Name, metav1.GetOptions{})
				d = cmp.Diff(tr, taskrun,
					ignoreTypeMeta,
					ignoreObjectMeta,
					ignoreCondition,
					ignoreTaskRunStatus,
					ignoreConditions,
					ignoreContainerStates,
					ignoreStepState,
					ignoreSATaskRunSpec,
					ignoreTaskRunStatusFields,
				)
				if d != "" {
					t.Fatalf(`The expected taskrun does not match created taskrun. Here is the diff: %v`, d)
				}
				d = cmp.Diff(tr.Status.TaskRunStatusFields.Results, taskrun.Status.TaskRunStatusFields.Results, sortTaskRunResults)
				if d != "" {
					t.Fatalf(`The expected TaskRunResults does not what was received. Here is the diff: %v`, d)
				}
			}
			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getPropagatedResultPipelineRun(t *testing.T, namespace string) (*v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun) {
	t.Helper()
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-all-type-results
  namespace: %s
spec:
  pipelineSpec:
    tasks:
      - name: make-uid
        taskSpec:
          results:
            - name: strUid
              type: string
            - name: arrayUid
              type: array
            - name: mapUid
              type: object
              properties:
                uid: {
                  type: string
                }
          steps:
            - name: add-str-uid
              image: docker.io/library/busybox
              command: ["/bin/sh", "-c"]
              args:
                - echo "1001" | tee $(results.strUid.path)
            - name: add-array-uid
              image: docker.io/library/busybox
              command: ["/bin/sh", "-c"]
              args:
                - echo "[\"1002\", \"1003\"]" | tee $(results.arrayUid.path)
            - name: add-map-uid
              image: docker.io/library/busybox
              command: ["/bin/sh", "-c"]
              args:
                - echo -n "{\"uid\":\"1004\"}" | tee $(results.mapUid.path)
      - name: show-uid
        taskSpec:
          steps:
            - name: show-str-uid
              image: docker.io/library/busybox
              command: ["/bin/sh", "-c"]
              args:
                - echo
                - $(tasks.make-uid.results.strUid)
            - name: show-array-uid
              image: docker.io/library/busybox
              command: ["/bin/sh", "-c"]
              args:
                - echo
                - $(tasks.make-uid.results.arrayUid[*])
            - name: show-map-uid
              image: docker.io/library/busybox
              command: ["/bin/sh", "-c"]
              args:
                - echo
                - $(tasks.make-uid.results.mapUid.uid)
`, namespace))
	expectedPipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: propagated-all-type-results
  namespace: %s
spec:
  pipelineSpec:
    tasks:
    - name: make-uid
      taskSpec:
        results:
        - name: strUid
          type: string
        - name: arrayUid
          type: array
        - name: mapUid
          properties:
            uid:
              type: string
          type: object
        steps:
        - args:
          - echo "1001" | tee $(results.strUid.path)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: add-str-uid
        - args:
          - echo "[\"1002\", \"1003\"]" | tee $(results.arrayUid.path)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: add-array-uid
        - args:
          - echo -n "{\"uid\":\"1004\"}" | tee $(results.mapUid.path)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: add-map-uid
    - name: show-uid
      taskSpec:
        steps:
        - args:
          - echo
          - $(tasks.make-uid.results.strUid)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: show-str-uid
        - args:
          - echo
          - $(tasks.make-uid.results.arrayUid[*])
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: show-array-uid
        - args:
          - echo
          - $(tasks.make-uid.results.mapUid.uid)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: show-map-uid
  timeouts:
    pipeline: 1h0m0s
status:
  pipelineSpec:
    tasks:
    - name: make-uid
      taskSpec:
        results:
        - name: strUid
          type: string
        - name: arrayUid
          type: array
        - name: mapUid
          properties:
            uid:
              type: string
          type: object
        steps:
        - args:
          - echo "1001" | tee $(results.strUid.path)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: add-str-uid
        - args:
          - echo "[\"1002\", \"1003\"]" | tee $(results.arrayUid.path)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: add-array-uid
        - args:
          - echo -n "{\"uid\":\"1004\"}" | tee $(results.mapUid.path)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: add-map-uid
    - name: show-uid
      taskSpec:
        steps:
        - args:
          - echo
          - $(tasks.make-uid.results.strUid)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: show-str-uid
        - args:
          - echo
          - $(tasks.make-uid.results.arrayUid[*])
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: show-array-uid
        - args:
          - echo
          - $(tasks.make-uid.results.mapUid.uid)
          command:
          - /bin/sh
          - -c
          image: docker.io/library/busybox
          name: show-map-uid
`, namespace))
	makeUidTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: propagated-all-type-results-make-uid
  namespace: %s
spec:
  taskSpec:
    results:
    - name: strUid
      type: string
    - name: arrayUid
      type: array
    - name: mapUid
      properties:
        uid:
          type: string
      type: object
    steps:
    - args:
      - echo "1001" | tee $(results.strUid.path)
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: add-str-uid
    - args:
      - echo "[\"1002\", \"1003\"]" | tee $(results.arrayUid.path)
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: add-array-uid
    - args:
      - echo -n "{\"uid\":\"1004\"}" | tee $(results.mapUid.path)
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: add-map-uid
  timeout: 1h0m0s
status:
  podName: propagated-all-type-results-make-uid-pod
  results:
  - name: strUid
    type: string
    value: |
      1001
  - name: arrayUid
    type: array
    value:
    - "1002"
    - "1003"
  - name: mapUid
    type: object
    value:
      uid: "1004"
  steps:
  - container: step-add-str-uid
    name: add-str-uid
  - container: step-add-array-uid
    name: add-array-uid
  - container: step-add-map-uid
    name: add-map-uid
  taskSpec:
    results:
    - name: strUid
      type: string
    - name: arrayUid
      type: array
    - name: mapUid
      properties:
        uid:
          type: string
      type: object
    steps:
    - args:
      - echo "1001" | tee /tekton/results/strUid
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: add-str-uid
    - args:
      - echo "[\"1002\", \"1003\"]" | tee /tekton/results/arrayUid
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: add-array-uid
    - args:
      - echo -n "{\"uid\":\"1004\"}" | tee /tekton/results/mapUid
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: add-map-uid
`, namespace))
	showUidTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: propagated-all-type-results-show-uid
  namespace: %s
spec:
  taskSpec:
    steps:
    - args:
      - echo
      - |
        1001
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: show-str-uid
    - args:
      - echo
      - "1002"
      - "1003"
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: show-array-uid
    - args:
      - echo
      - "1004"
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: show-map-uid
  timeout: 1h0m0s
status:
  podName: propagated-all-type-results-show-uid-pod
  steps:
  - container: step-show-str-uid
    name: show-str-uid
  - container: step-show-array-uid
    name: show-array-uid
  - container: step-show-map-uid
    name: show-map-uid
  taskSpec:
    steps:
    - args:
      - echo
      - |
        1001
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: show-str-uid
    - args:
      - echo
      - "1002"
      - "1003"
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: show-array-uid
    - args:
      - echo
      - "1004"
      command:
      - /bin/sh
      - -c
      image: docker.io/library/busybox
      name: show-map-uid
`, namespace))
	return pipelineRun, expectedPipelineRun, []*v1.TaskRun{makeUidTaskRun, showUidTaskRun}
}
