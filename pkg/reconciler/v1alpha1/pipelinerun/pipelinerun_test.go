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
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestReconcile(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name:       "test-pipeline",
				APIVersion: "a1",
			},
		}}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name:       "pipeline-not-exist",
				APIVersion: "a1",
			},
		}},
	}

	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "foo",
		}},
	}
	rc := &reconcilerConfig{
		pipelineRunLister: &mockPipelineRunsLister{runs: prs},
		pipelineLister:    &mockPipelinesLister{p: ps},
		taskLister:        &mockTasksLister{},
	}
	tcs := []struct {
		name        string
		pipelineRun string
		shdErr      bool
		log         string
	}{
		{"success", "foo/test-pipeline-run-success", false, ""},
		{"invalid-pipeline-run-shd-succeed-with-logs", "foo/test-pipeline-run-doesnot-exist", false,
			"pipeline run \"foo/test-pipeline-run-doesnot-exist\" in work queue no longer exists"},
		{"invalid-pipeline-shd-succeed-with-logs", "foo/invalid-pipeline", false,
			"\"foo/invalid-pipeline\" failed to Get Pipeline: \"foo/pipeline-not-exist\". not found"},
		{"invalid-pipeline-run-name-shd-succed-with-logs", "test/pipeline-fail/t", false,
			"invalid resource key: test/pipeline-fail/t"},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c, logs, _ := getController(rc, t)
			err := c.Reconciler.Reconcile(context.TODO(), tc.pipelineRun)
			if tc.shdErr != (err != nil) {
				t.Errorf("expected to see error %t. Got error %v", tc.shdErr, err)
			}
			if tc.log == "" && logs.Len() > 0 {
				t.Errorf("expected to see no error log. However found errors in logs: %v", logs)
			} else if tc.log != "" && logs.FilterMessage(tc.log).Len() == 0 {
				m := getLogMessages(logs)
				t.Errorf("Log lines diff %s", cmp.Diff(tc.log, m))
			}
		})
	}
}
func TestCreatePipeline(t *testing.T) {
	successPipeline := v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-frontend",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task", APIVersion: "t"},
			}, {
				Name:    "unit-test-backend",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task", APIVersion: "t"},
			}},
		}}
	failPipeline := v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-invalid",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-frontend",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task", APIVersion: "t"},
			}, {
				Name:    "random-task",
				TaskRef: v1alpha1.TaskRef{Name: "random-task", APIVersion: "t"},
			}},
		}}
	ps := []*v1alpha1.Pipeline{&successPipeline, &failPipeline}
	tasks := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{},
	}}
	rc := &reconcilerConfig{
		pipelineRunLister: &mockPipelineRunsLister{},
		pipelineLister:    &mockPipelinesLister{p: ps},
		taskLister:        &mockTasksLister{t: tasks},
	}
	tcs := []struct {
		name        string
		pipeline    v1alpha1.Pipeline
		shdErr      bool
		createCalls int
	}{
		{"success", successPipeline, false, 2},
		{"invalidPipeline", failPipeline, true, 0},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c, _, client := getController(rc, t)
			err := c.Reconciler.(*Reconciler).createPipelineRun(context.TODO(), &tc.pipeline)
			if tc.shdErr != (err != nil) {
				t.Errorf("expected to see error %t. Got error %v", tc.shdErr, err)
			}
			if len(client.Actions()) != tc.createCalls {
				t.Errorf("expected to see %d create call call. Found %d", tc.createCalls, len(client.Actions()))
			}
		})
	}
}

func getController(prs []*v1alpha1.PipelineRun, ps []*v1alpha1.Pipeline, t *testing.T) (*controller.Impl, *observer.ObservedLogs) {
	pipelineClient := fakepipelineclientset.NewSimpleClientset()
	sharedInfomer := informers.NewSharedInformerFactory(pipelineClient, 0)
	pipelineRunsInformer := sharedInfomer.Pipeline().V1alpha1().PipelineRuns()
	pipelineInformer := sharedInfomer.Pipeline().V1alpha1().Pipelines()

	for _, pr := range prs {
		pipelineRunsInformer.Informer().GetIndexer().Add(pr)
	}
	for _, p := range ps {
		pipelineInformer.Informer().GetIndexer().Add(p)
	}

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
	), logs
}

func getLogMessages(logs *observer.ObservedLogs) []string {
	messages := []string{}
	for _, l := range logs.All() {
		messages = append(messages, l.Message)
	}
	return messages
}
