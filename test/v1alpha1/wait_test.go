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
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

var (
	success = apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}
	failure = apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse}
)

func TestWaitForTaskRunStateSucceed(t *testing.T) {
	d := Data{
		TaskRuns: []*v1alpha1.TaskRun{{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Status: v1alpha1.TaskRunStatus{Status: duckv1beta1.Status{
				Conditions: []apis.Condition{success},
			}},
		}},
	}
	c, ctx, cancel := fakeClients(t, d)
	defer cancel()
	if err := WaitForTaskRunState(ctx, c, "foo", Succeed("foo"), "TestTaskRunSucceed"); err != nil {
		t.Fatal(err)
	}
}
func TestWaitForTaskRunStateFailed(t *testing.T) {
	d := Data{
		TaskRuns: []*v1alpha1.TaskRun{{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Status: v1alpha1.TaskRunStatus{Status: duckv1beta1.Status{
				Conditions: []apis.Condition{failure},
			}},
		}},
	}
	c, ctx, cancel := fakeClients(t, d)
	defer cancel()
	err := WaitForTaskRunState(ctx, c, "foo", TaskRunFailed("foo"), "TestTaskRunFailed")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForPipelineRunStateSucceed(t *testing.T) {
	d := Data{
		PipelineRuns: []*v1alpha1.PipelineRun{{
			ObjectMeta: metav1.ObjectMeta{Name: "bar"},
			Status: v1alpha1.PipelineRunStatus{Status: duckv1beta1.Status{
				Conditions: []apis.Condition{success},
			}},
		}},
	}
	c, ctx, cancel := fakeClients(t, d)
	defer cancel()
	err := WaitForPipelineRunState(ctx, c, "bar", 2*time.Second, PipelineRunSucceed("bar"), "TestWaitForPipelineRunSucceed")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForPipelineRunStateFailed(t *testing.T) {
	d := Data{
		PipelineRuns: []*v1alpha1.PipelineRun{{
			ObjectMeta: metav1.ObjectMeta{Name: "bar"},
			Status: v1alpha1.PipelineRunStatus{Status: duckv1beta1.Status{
				Conditions: []apis.Condition{failure},
			}},
		}},
	}
	c, ctx, cancel := fakeClients(t, d)
	defer cancel()
	err := WaitForPipelineRunState(ctx, c, "bar", 2*time.Second, Failed("bar"), "TestWaitForPipelineRunFailed")
	if err != nil {
		t.Fatal(err)
	}
}

func fakeClients(t *testing.T, d Data) (*clients, context.Context, func()) {
	ctx, _ := rtesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	fakeClients, _ := SeedTestData(t, ctx, d)
	return &clients{
		PipelineClient:         fakeClients.Pipeline.TektonV1alpha1().Pipelines(""),
		PipelineResourceClient: fakeClients.Resource.TektonV1alpha1().PipelineResources(""),
		PipelineRunClient:      fakeClients.Pipeline.TektonV1alpha1().PipelineRuns(""),
		TaskClient:             fakeClients.Pipeline.TektonV1alpha1().Tasks(""),
		TaskRunClient:          fakeClients.Pipeline.TektonV1alpha1().TaskRuns(""),
	}, ctx, cancel
}
