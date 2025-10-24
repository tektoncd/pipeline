/*
Copyright 2025 The Tekton Authors

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

package taskrun

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"go.opentelemetry.io/otel/trace"

	_ "github.com/tektoncd/pipeline/pkg/taskrunmetrics/fake"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestFailTaskRun_Timeout(t *testing.T) {
	testCases := []struct {
		name           string
		taskRun        *v1.TaskRun
		pod            *corev1.Pod
		reason         v1.TaskRunReason
		message        string
		featureFlags   map[string]string
		expectedStatus apis.Condition
		expectedPods   []corev1.Pod
	}{
		{
			name: "taskrun-timeout-keep-pod-on-cancel",
			taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-timeout
  namespace: foo
spec:
  taskRef:
    name: test-task
  timeout: 10s
status:
  startTime: "2000-01-01T01:01:01Z"
  conditions:
  - status: Unknown
    type: Succeeded
  podName: foo-is-bar
  steps:
  - running:
      startedAt: "2022-01-01T00:00:00Z"
`),
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "foo-is-bar",
				Annotations: map[string]string{
					"test": "test value",
				},
			}},
			featureFlags: map[string]string{
				config.KeepPodOnCancel: "true",
			},
			reason:  v1.TaskRunReasonTimedOut,
			message: "TaskRun test-taskrun-run-timeout failed to finish within 10s",
			expectedStatus: apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  v1.TaskRunReasonTimedOut.String(),
				Message: "TaskRun test-taskrun-run-timeout failed to finish within 10s",
			},
			expectedPods: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "foo-is-bar",
				Annotations: map[string]string{
					"test":              "test value",
					"tekton.dev/cancel": "CANCEL",
				},
			}}},
		},
		{
			name: "taskrun-timeout-keep-pod-on-cancel-false",
			taskRun: parse.MustParseV1TaskRun(t, `
metadata:
  name: test-taskrun-run-timeout
  namespace: foo
spec:
  taskRef:
    name: test-task
  timeout: 10s
status:
  startTime: "2000-01-01T01:01:01Z"
  conditions:
  - status: Unknown
    type: Succeeded
  podName: foo-is-bar
  steps:
  - running:
      startedAt: "2022-01-01T00:00:00Z"
`),
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "foo-is-bar",
				Annotations: map[string]string{
					"test": "test value",
				},
			}},
			featureFlags: map[string]string{
				config.KeepPodOnCancel: "false",
			},
			reason:  v1.TaskRunReasonTimedOut,
			message: "TaskRun test-taskrun-run-timeout failed to finish within 10s",
			expectedStatus: apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  v1.TaskRunReasonTimedOut.String(),
				Message: "TaskRun test-taskrun-run-timeout failed to finish within 10s",
			},
		}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				TaskRuns: []*v1.TaskRun{tc.taskRun},
				ConfigMaps: []*corev1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      config.GetFeatureFlagsConfigName(),
							Namespace: system.Namespace(),
						},
						Data: tc.featureFlags,
					},
				},
				Pods: []*corev1.Pod{
					tc.pod,
				},
			}

			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()

			c := &Reconciler{
				KubeClientSet:     testAssets.Clients.Kube,
				PipelineClientSet: testAssets.Clients.Pipeline,
				Clock:             testClock,
				taskRunLister:     testAssets.Informers.TaskRun.Lister(),
				limitrangeLister:  testAssets.Informers.LimitRange.Lister(),
				cloudEventClient:  testAssets.Clients.CloudEvents,
				metrics:           nil, // Not used
				entrypointCache:   nil, // Not used
				pvcHandler:        volumeclaim.NewPVCHandler(testAssets.Clients.Kube, testAssets.Logger),
				tracerProvider:    trace.NewNoopTracerProvider(),
			}
			ctx := testAssets.Ctx

			ff, _ := config.NewFeatureFlagsFromMap(tc.featureFlags)

			ctx = config.ToContext(ctx, &config.Config{
				FeatureFlags: ff,
			})

			if err := c.failTaskRun(ctx, tc.taskRun, tc.reason, tc.message); err != nil {
				t.Errorf("fail timeout test: %v", err)
			}

			if d := cmp.Diff(&tc.expectedStatus, tc.taskRun.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
				t.Fatal(diff.PrintWantGot(d))
			}

			pods, err := c.KubeClientSet.CoreV1().Pods(tc.pod.Namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatal("Error while fetching pod: "+tc.pod.Name, err.Error())
			}
			if d := cmp.Diff(tc.expectedPods, pods.Items); d != "" {
				t.Fatal(diff.PrintWantGot(d))
			}
		})
	}
}
