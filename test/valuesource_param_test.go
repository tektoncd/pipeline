//go:build e2e
// +build e2e

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

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
)

func TestValueSourceParams(t *testing.T) {
	t.Parallel()

	expectedFeatureFlags := getFeatureFlagsBaseOnAPIFlag(t)
	type tests struct {
		name                      string
		pipelineRunFunc           func(*testing.T, string, string) (*v1.Task, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun, *corev1.ConfigMap)
		expectedPipelineRunReason string
		configMapNameUsedInRun    string
	}

	tds := []tests{{
		name:                      "ValueFrom In Param",
		pipelineRunFunc:           getValueSourcePipelineRun,
		expectedPipelineRunReason: "PipelineRunSuccess",
		configMapNameUsedInRun:    "valuesource-config-map",
	}, {
		name:                      "ValueFrom In Param but PipelineRunfails due to issue in ConfigMap",
		pipelineRunFunc:           getValueSourcePipelineRun,
		expectedPipelineRunReason: v1.PipelineRunReasonFetchingValueSourceFailed.String(),
		configMapNameUsedInRun:    "wrong name",
	}}

	for _, td := range tds {
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			c, namespace := setup(ctx, t)

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			task, pipeline, pipelineRun, expectedResolvedPipelineRun, expectedTaskRuns, valueSourceConfigMap := td.pipelineRunFunc(t, namespace, td.configMapNameUsedInRun)

			expectedResolvedPipelineRun.Status.Provenance = &v1.Provenance{
				FeatureFlags: expectedFeatureFlags,
			}

			if td.expectedPipelineRunReason != v1.PipelineRunReasonFetchingValueSourceFailed.String() {
				_, err := c.KubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, valueSourceConfigMap, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create ConfigMap `%s`: %s", valueSourceConfigMap.Name, err)
				}
			}

			_, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create task `%s`: %s", task.Name, err)
			}

			_, err = c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create pipeline `%s`: %s", pipeline.Name, err)
			}

			updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
				"enable-valuefrom-in-param": "false",
			})
			_, err = c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err == nil {
				t.Fatalf("Succeeded in creating PipelineRun `%s` with valuesource in params even though the feature flag is turned off", pipelineRun.Name)
			}

			updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
				"enable-valuefrom-in-param": "true",
			})
			_, err = c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
			}

			expectedCondition := PipelineRunSucceed(pipelineRun.Name)
			if td.expectedPipelineRunReason != "PipelineRunSuccess" {
				expectedCondition = FailedWithReason(td.expectedPipelineRunReason, pipelineRun.Name)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", pipelineRun.Name, namespace)
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, expectedCondition, td.expectedPipelineRunReason, v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", pipelineRun.Name, err)
			}

			cl, _ := c.V1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
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
				ignoreFeatureFlags,
			)
			if d != "" {
				t.Fatalf(`The resolved spec does not match the expected spec. Here is the diff: %v`, d)
			}
			for _, tr := range expectedTaskRuns {
				t.Logf("Checking Taskrun %s", tr.Name)
				tr.Status.Provenance = &v1.Provenance{
					FeatureFlags: expectedFeatureFlags,
				}
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
					ignoreFeatureFlags,
				)
				if d != "" {
					t.Fatalf(`The expected taskrun does not match created taskrun. Here is the diff: %v`, d)
				}
			}
			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getValueSourcePipelineRun(t *testing.T, namespace string, configMapNameUsedInRun string) (*v1.Task, *v1.Pipeline, *v1.PipelineRun, *v1.PipelineRun, []*v1.TaskRun, *corev1.ConfigMap) {
	t.Helper()
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: echo-message
  namespace: %s
spec:
  params:
    - name: MESSAGE
      type: string
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: echo $(params.MESSAGE)
`, namespace))
	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: pipeline-message
  namespace: %s
spec:
  params:
  - name: MESSAGE
    type: string
  tasks:
    - name: echo-message
      taskRef:
        name: echo-message
      params:
      - name: MESSAGE
        value: $(params.MESSAGE)
`, namespace))
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: valuefrom-in-param
  namespace: %s
spec:
  params:
    - name: MESSAGE
      valueFrom:
        configMapKeyRef:
          name: %s
          key: myKey
  pipelineRef:
    name: pipeline-message
`, namespace, configMapNameUsedInRun))

	valueSourceConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "valuesource-config-map"},
		Data: map[string]string{
			"myKey": "myValue",
		}}

	var taskRuns []*v1.TaskRun
	var expectedPipelineRun *v1.PipelineRun
	if configMapNameUsedInRun == "valuesource-config-map" {
		expectedPipelineRun = parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: valuefrom-in-param
  namespace: %s
spec:
  timeouts:
    pipeline: 1h
  params:
  - name: MESSAGE
    value: "myValue"
  pipelineRef:
    name: pipeline-message
status:
  pipelineSpec:
    params:
    - name: MESSAGE
      type: string
    tasks:
      - name: echo-message
        taskRef:
          name: echo-message
          kind: Task
        params:
        - name: MESSAGE
          value: myValue		
`, namespace))
		taskRuns = []*v1.TaskRun{
			parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: valuefrom-in-param-echo-message
  namespace: %s
spec:
  timeout: 1h
  params:
  - name: MESSAGE
    value: "myValue"
  taskRef:
    name: echo-message
    kind: Task
status:
  podName: valuefrom-in-param-echo-message-pod
  artifacts: {}
  steps:
  - name: echo
    container: step-echo
  taskSpec:
    params:
    - name: MESSAGE
      type: string
    steps:
    - name: echo
      image: mirror.gcr.io/ubuntu
      script: echo myValue
`, namespace)),
		}
	} else {
		taskRuns = []*v1.TaskRun{}
		expectedPipelineRun = parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: valuefrom-in-param
  namespace: %s
spec:
  timeouts:
    pipeline: 1h
  params:
    - name: MESSAGE
      valueFrom:
        configMapKeyRef:
          name: %s
          key: myKey
  pipelineRef:
    name: pipeline-message
status:
  pipelineSpec:
    params:
    - name: MESSAGE
      type: string
    tasks:
      - name: echo-message
        taskRef:
          name: echo-message
          kind: Task
        params:
        - name: MESSAGE
          value: "$(params.MESSAGE)"		
`, namespace, configMapNameUsedInRun))
	}

	return task, pipeline, pipelineRun, expectedPipelineRun, taskRuns, &valueSourceConfigMap
}
