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
	"testing"
	"time"

	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

const (
	waitNamespace = "wait"
)

var (
	success = apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}
	failure = apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse}
)

func TestWaitForTaskRunStateSucceed(t *testing.T) {
	d := Data{
		TaskRuns: []*v1alpha1.TaskRun{
			tb.TaskRun("foo", waitNamespace, tb.TaskRunStatus(
				tb.Condition(success),
			)),
		},
	}
	c := fakeClients(t, d)
	err := WaitForTaskRunState(c, "foo", TaskRunSucceed("foo"), "TestTaskRunSucceed")
	if err != nil {
		t.Fatal(err)
	}
}
func TestWaitForTaskRunStateFailed(t *testing.T) {
	d := Data{
		TaskRuns: []*v1alpha1.TaskRun{
			tb.TaskRun("foo", waitNamespace, tb.TaskRunStatus(
				tb.Condition(failure),
			)),
		},
	}
	c := fakeClients(t, d)
	err := WaitForTaskRunState(c, "foo", TaskRunFailed("foo"), "TestTaskRunFailed")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForPipelineRunStateSucceed(t *testing.T) {
	d := Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("bar", waitNamespace, tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(success),
			)),
		},
	}
	c := fakeClients(t, d)
	err := WaitForPipelineRunState(c, "bar", 2*time.Second, PipelineRunSucceed("bar"), "TestWaitForPipelineRunSucceed")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForPipelineRunStateFailed(t *testing.T) {
	d := Data{
		PipelineRuns: []*v1alpha1.PipelineRun{
			tb.PipelineRun("bar", waitNamespace, tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(failure),
			)),
		},
	}
	c := fakeClients(t, d)
	err := WaitForPipelineRunState(c, "bar", 2*time.Second, PipelineRunFailed("bar"), "TestWaitForPipelineRunFailed")
	if err != nil {
		t.Fatal(err)
	}
}

func fakeClients(t *testing.T, d Data) *clients {
	fakeClients, _ := SeedTestData(t, d)
	// 	c.KubeClient = fakeClients.Kube
	return &clients{
		PipelineClient:         fakeClients.Pipeline.TektonV1alpha1().Pipelines(waitNamespace),
		PipelineResourceClient: fakeClients.Pipeline.TektonV1alpha1().PipelineResources(waitNamespace),
		PipelineRunClient:      fakeClients.Pipeline.TektonV1alpha1().PipelineRuns(waitNamespace),
		TaskClient:             fakeClients.Pipeline.TektonV1alpha1().Tasks(waitNamespace),
		TaskRunClient:          fakeClients.Pipeline.TektonV1alpha1().TaskRuns(waitNamespace),
	}
}
