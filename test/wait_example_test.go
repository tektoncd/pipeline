//go:build examples
// +build examples

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

package test

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

var (
	// This is a "hack" to make the example "look" like tests.
	// Golang Example functions do not take `t *testing.T` as argument, so we "fake"
	// it so that examples still compiles (`go test` tries to compile those) and look
	// nice in the go documentation.
	t testingT
	c *clients
)

type testingT interface {
	Errorf(string, ...interface{})
}

func ExampleWaitForTaskRunState() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// […] setup the test, get clients
	if err := WaitForTaskRunState(ctx, c, "taskRunName", func(ca apis.ConditionAccessor) (bool, error) {
		c := ca.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}, "TaskRunHasCondition", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun taskRunName to finish: %s", err)
	}
}

func ExampleWaitForPipelineRunState() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// […] setup the test, get clients
	if err := WaitForPipelineRunState(ctx, c, "pipelineRunName", 1*time.Minute, func(ca apis.ConditionAccessor) (bool, error) {
		c := ca.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}, "PipelineRunHasCondition", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun pipelineRunName to finish: %s", err)
	}
}
