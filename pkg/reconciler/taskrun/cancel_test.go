/*
Copyright 2019 The Tekton Authors

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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestCancelTaskRun(t *testing.T) {
	namespace := "the-namespace"
	taskRunName := "the-taskrun"
	wantStatus := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "TaskRunCancelled",
		Message: `TaskRun "the-taskrun" was cancelled`,
	}
	for _, c := range []struct {
		desc    string
		taskRun *v1alpha1.TaskRun
		pod     *corev1.Pod
	}{{
		desc: "no pod scheduled",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      taskRunName,
				Namespace: namespace,
			},
			Spec: v1alpha1.TaskRunSpec{
				Status: v1alpha1.TaskRunSpecStatusCancelled,
			},
		},
	}, {
		desc: "pod scheduled",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      taskRunName,
				Namespace: namespace,
			},
			Spec: v1alpha1.TaskRunSpec{
				Status: v1alpha1.TaskRunSpecStatusCancelled,
			},
		},
		pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "the-pod",
			Labels: map[string]string{
				"tekton.dev/taskRun": taskRunName,
			},
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			d := test.Data{
				TaskRuns: []*v1alpha1.TaskRun{c.taskRun},
			}
			if c.pod != nil {
				d.Pods = []*corev1.Pod{c.pod}
			}

			testAssets, cancel := getTaskRunController(t, d)
			defer cancel()
			if err := testAssets.Controller.Reconciler.Reconcile(context.Background(), getRunName(c.taskRun)); err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(wantStatus, c.taskRun.Status.GetCondition(apis.ConditionSucceeded), ignoreLastTransitionTime); d != "" {
				t.Errorf("Diff(-want, +got): %s", d)
			}

			if c.pod != nil {
				if _, err := testAssets.Controller.Reconciler.(*Reconciler).KubeClientSet.CoreV1().Pods(c.taskRun.Namespace).Get(c.pod.Name, metav1.GetOptions{}); !kerrors.IsNotFound(err) {
					t.Errorf("Pod was not deleted; wanted not-found error, got %v", err)
				}
			}
		})
	}
}
