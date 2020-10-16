// +build conformance

/*
Copyright 2020 The Tekton Authors

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
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
)

func TestTaskRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	trName := "echo-hello-task-run"
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: trName, Namespace: namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{Container: corev1.Container{
					Image:   "busybox",
					Command: []string{"echo", "\"hello\""},
				}}},
			},
		},
	}

	t.Logf("Creating TaskRun %s", trName)
	if _, err := c.TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", trName, err)
	}

	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "WaitTaskRunDone"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
		return
	}
	tr, err := c.TaskRunClient.Get(ctx, trName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get TaskRun `%s`: %s", trName, err)
	}

	// check required fields in TaskRun ObjectMeta
	if tr.ObjectMeta.Name == "" {
		t.Errorf("TaskRun ObjectMeta doesn't have the Name.")
	}
	if len(tr.ObjectMeta.Labels) == 0 {
		t.Errorf("TaskRun ObjectMeta doesn't have the Labels.")
	}
	if len(tr.ObjectMeta.Annotations) == 0 {
		t.Errorf("TaskRun ObjectMeta doesn't have the Annotations.")
	}
	if tr.ObjectMeta.CreationTimestamp.IsZero() {
		t.Errorf("TaskRun ObjectMeta doesn't have the CreationTimestamp.")
	}

	// check required fields in TaskRun Status
	if len(tr.Status.Conditions) == 0 {
		t.Errorf("TaskRun Status doesn't have the Conditions.")
	}
	if tr.Status.StartTime.IsZero() {
		t.Errorf("TaskRun Status doesn't have the StartTime.")
	}
	if tr.Status.CompletionTime.IsZero() {
		t.Errorf("TaskRun Status doesn't have the CompletionTime.")
	}
	if len(tr.Status.Steps) == 0 {
		t.Errorf("TaskRun Status doesn't have the Steps.")
	}

	condition := tr.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil {
		t.Errorf("Expected a succeeded Condition but got nil.")
	}
	if condition.Status != corev1.ConditionTrue {
		t.Errorf("TaskRun Status Condition doesn't have the right Status.")
	}
	if condition.Reason == "" {
		t.Errorf("TaskRun Status Condition doesn't have the Reason.")
	}
	if condition.Message == "" {
		t.Errorf("TaskRun Status Condition doesn't have the Message.")
	}
	if condition.Severity != apis.ConditionSeverityError && condition.Severity != apis.ConditionSeverityWarning && condition.Severity != apis.ConditionSeverityInfo {
		t.Errorf("TaskRun Status Condition doesn't have the right Severity.")
	}
}
