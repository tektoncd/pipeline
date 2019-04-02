// +build examples

/*
Copyright 2018 The Knative Authors.

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
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
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
	// […] setup the test, get clients
	if err := WaitForTaskRunState(c, "taskRunName", func(tr *v1alpha1.TaskRun) (bool, error) {
		if len(tr.Status.Conditions) > 0 {
			return true, nil
		}
		return false, nil
	}, "TaskRunHasCondition"); err != nil {
		t.Errorf("Error waiting for TaskRun taskRunName to finish: %s", err)
	}
}

func ExampleWaitForPipelineRunState() {
	// […] setup the test, get clients
	if err := WaitForPipelineRunState(c, "pipelineRunName", 1*time.Minute, func(pr *v1alpha1.PipelineRun) (bool, error) {
		if len(pr.Status.Conditions) > 0 {
			return true, nil
		}
		return false, nil
	}, "PipelineRunHasCondition"); err != nil {
		t.Errorf("Error waiting for PipelineRun pipelineRunName to finish: %s", err)
	}
}
