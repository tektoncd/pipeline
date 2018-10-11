/*
Copyright 2018 The Knative Authors
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

package pipelinerun

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	informersv1alpha1 "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestReconcile(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline",
			},
		},
	}}

	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{{
			Name:    "unit-test-1",
			TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
		}},
		}},
	}
	ts := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{},
	}}
	d := testData{
		prs: prs,
		ps:  ps,
		ts:  ts,
	}
	c, logs, client := getController(d)
	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-success")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling valid Pipeline but saw %s", err)
	}
	if logs.Len() > 0 {
		t.Errorf("expected to see no error log. However found errors in logs: %v", logs)
	}
	if len(client.Actions()) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := client.Pipeline().PipelineRuns("foo").Get("test-pipeline-run-success", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}
	condition := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected PipelineRun status to be in progress, but was %s", condition)
	}

	// Check that the expected TaskRun was created
	actual := client.Actions()[0].(ktesting.CreateAction).GetObject()
	trueB := true
	expectedTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-success-unit-test-1",
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "pipeline.knative.dev/v1alpha1",
				Kind:               "PipelineRun",
				Name:               "test-pipeline-run-success",
				Controller:         &trueB,
				BlockOwnerDeletion: &trueB,
			}},
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-test-task",
			},
		},
	}
	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, d)
	}
}

func TestReconcileInvalid(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "pipeline-not-exist",
			},
		}},
	}
	d := testData{
		prs: prs,
	}
	tcs := []struct {
		name        string
		pipelineRun string
		log         string
	}{
		{"invalid-pipeline-run-shd-succeed-with-logs", "foo/test-pipeline-run-doesnot-exist",
			"pipeline run \"foo/test-pipeline-run-doesnot-exist\" in work queue no longer exists"},
		{"invalid-pipeline-shd-succeed-with-logs", "foo/invalid-pipeline",
			"\"foo/invalid-pipeline\" failed to Get Pipeline: \"foo/pipeline-not-exist\""},
		{"invalid-pipeline-run-name-shd-succed-with-logs", "test/pipeline-fail/t",
			"invalid resource key: test/pipeline-fail/t"},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c, logs, _ := getController(d)
			err := c.Reconciler.Reconcile(context.Background(), tc.pipelineRun)
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling but saw %s", err)
			}
			if tc.log != "" && logs.FilterMessage(tc.log).Len() == 0 {
				m := getLogMessages(logs)
				t.Errorf("Log lines diff %s", cmp.Diff(tc.log, m))
			}
		})
	}
}

func getLogMessages(logs *observer.ObservedLogs) []string {
	messages := []string{}
	for _, l := range logs.All() {
		messages = append(messages, l.Message)
	}
	return messages
}

type testData struct {
	prs []*v1alpha1.PipelineRun
	ps  []*v1alpha1.Pipeline
	trs []*v1alpha1.TaskRun
	ts  []*v1alpha1.Task
}

func seedTestData(d testData) (*fakepipelineclientset.Clientset, informersv1alpha1.PipelineRunInformer, informersv1alpha1.PipelineInformer, informersv1alpha1.TaskRunInformer, informersv1alpha1.TaskInformer) {
	objs := []runtime.Object{}
	for _, pr := range d.prs {
		objs = append(objs, pr)
	}
	for _, p := range d.ps {
		objs = append(objs, p)
	}
	for _, tr := range d.trs {
		objs = append(objs, tr)
	}
	for _, t := range d.ts {
		objs = append(objs, t)
	}
	pipelineClient := fakepipelineclientset.NewSimpleClientset(objs...)

	sharedInfomer := informers.NewSharedInformerFactory(pipelineClient, 0)
	pipelineRunsInformer := sharedInfomer.Pipeline().V1alpha1().PipelineRuns()
	pipelineInformer := sharedInfomer.Pipeline().V1alpha1().Pipelines()
	taskRunInformer := sharedInfomer.Pipeline().V1alpha1().TaskRuns()
	taskInformer := sharedInfomer.Pipeline().V1alpha1().Tasks()

	for _, pr := range d.prs {
		pipelineRunsInformer.Informer().GetIndexer().Add(pr)
	}
	for _, p := range d.ps {
		pipelineInformer.Informer().GetIndexer().Add(p)
	}
	for _, tr := range d.trs {
		taskRunInformer.Informer().GetIndexer().Add(tr)
	}
	for _, t := range d.ts {
		taskInformer.Informer().GetIndexer().Add(t)
	}
	return pipelineClient, pipelineRunsInformer, pipelineInformer, taskRunInformer, taskInformer
}

func getController(d testData) (*controller.Impl, *observer.ObservedLogs, *fakepipelineclientset.Clientset) {
	pipelineClient, pipelineRunsInformer, pipelineInformer, taskRunInformer, taskInformer := seedTestData(d)
	// Create a log observer to record all error logs.
	observer, logs := observer.New(zap.ErrorLevel)
	return NewController(
		reconciler.Options{
			Logger:            zap.New(observer).Sugar(),
			KubeClientSet:     fakekubeclientset.NewSimpleClientset(),
			PipelineClientSet: pipelineClient,
		},
		pipelineRunsInformer,
		pipelineInformer,
		taskInformer,
		taskRunInformer,
	), logs, pipelineClient
}
