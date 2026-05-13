//go:build e2e

/*
Copyright 2026 The Tekton Authors

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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestPodReschedulingFeatureFlag verifies that enabling the pod-rescheduling
// feature flag does not regress normal TaskRun execution, and that a successful
// TaskRun does not get the reschedule annotation.
//
// @test:execution=serial
// @test:reason=modifies enable-pod-rescheduling in feature-flags ConfigMap
func TestPodReschedulingFeatureFlag(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	previousValue := ""
	updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
		config.EnablePodRescheduling: "true",
	})

	knativetest.CleanupOnInterrupt(func() {
		updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
			config.EnablePodRescheduling: previousValue,
		})
		tearDown(ctx, t, c, namespace)
	}, t.Logf)
	defer func() {
		updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
			config.EnablePodRescheduling: previousValue,
		})
		tearDown(ctx, t, c, namespace)
	}()

	taskRunName := helpers.ObjectNameForTest(t)

	if _, err := c.V1TaskRunClient.Create(ctx, parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: mirror.gcr.io/busybox
      script: echo hello
`, taskRunName, namespace)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun %q: %v", taskRunName, err)
	}

	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceeded", v1Version); err != nil {
		t.Fatalf("Waiting for TaskRun to succeed: %v", err)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get TaskRun %q: %v", taskRunName, err)
	}

	if tr.Annotations != nil {
		if _, ok := tr.Annotations[v1.TaskRunPodRescheduleCountAnnotation]; ok {
			t.Errorf("Successful TaskRun should not have reschedule annotation, but it does")
		}
	}
}
