// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipelinerun

import (
	"errors"
	"testing"

	"github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stest "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"

	tu "github.com/tektoncd/cli/pkg/test"
)

var (
	success = apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}
	failure = apis.Condition{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse}
)

func Test_cancel_invalid_namespace(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	prName := "test-pipeline-run-123"

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pRun := Command(p)
	got, _ := tu.ExecuteCommand(pRun, "cancel", prName, "-n", "invalid")

	expected := "Error: namespaces \"invalid\" not found\n"
	tu.AssertOutput(t, expected, got)
}

func Test_cancel_pipelinerun(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	prName := "test-pipeline-run-123"

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, "ns",
			tb.PipelineRunLabel("tekton.dev/pipeline", "pipelineName"),
			tb.PipelineRunSpec("pipelineName",
				tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Namespaces: ns})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pRun := Command(p)
	got, _ := tu.ExecuteCommand(pRun, "cancel", prName, "-n", "ns")

	expected := "Pipelinerun cancelled: " + prName + "\n"
	tu.AssertOutput(t, expected, got)
}

func Test_cancel_pipelinerun_not_found(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	prName := "test-pipeline-run-123"

	cs, _ := test.SeedTestData(t, pipelinetest.Data{Namespaces: ns})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pRun := Command(p)
	got, _ := tu.ExecuteCommand(pRun, "cancel", prName, "-n", "ns")

	expected := "Error: failed to find pipelinerun: " + prName + "\n"
	tu.AssertOutput(t, expected, got)
}

func Test_cancel_pipelinerun_client_err(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	prName := "test-pipeline-run-123"
	errStr := "test generated error"

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, "ns",
			tb.PipelineRunLabel("tekton.dev/pipeline", "pipelineName"),
			tb.PipelineRunSpec("pipelineName",
				tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Namespaces: ns})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	cs.Pipeline.PrependReactor("update", "pipelineruns", func(action k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New(errStr)
	})

	pRun := Command(p)
	got, _ := tu.ExecuteCommand(pRun, "cancel", prName, "-n", "ns")

	expected := "Error: failed to cancel pipelinerun: " + prName + ", err: " + errStr + "\n"
	tu.AssertOutput(t, expected, got)
}

func Test_finished_pipelinerun_success(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	prName := "test-pipeline-run-123"

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, "ns",
			tb.PipelineRunLabel("tekton.dev/pipeline", "pipelineName"),
			tb.PipelineRunSpec("pipelineName",
				tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(success),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Namespaces: ns})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pRun := Command(p)
	got, _ := tu.ExecuteCommand(pRun, "cancel", prName, "-n", "ns")

	expected := "Error: failed to cancel pipelinerun " + prName + ": pipelinerun has already finished execution\n"
	tu.AssertOutput(t, expected, got)
}

func Test_finished_pipelinerun_failure(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		},
	}

	prName := "test-pipeline-run-123"

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, "ns",
			tb.PipelineRunLabel("tekton.dev/pipeline", "pipelineName"),
			tb.PipelineRunSpec("pipelineName",
				tb.PipelineRunDeprecatedServiceAccountName("", "test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("build-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("pipeline-param-1", "somethingmorefun"),
				tb.PipelineRunParam("rev-param", "revision1"),
			),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(failure),
			),
		),
	}

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Namespaces: ns})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	pRun := Command(p)
	got, _ := tu.ExecuteCommand(pRun, "cancel", prName, "-n", "ns")

	expected := "Error: failed to cancel pipelinerun " + prName + ": pipelinerun has already finished execution\n"
	tu.AssertOutput(t, expected, got)
}
