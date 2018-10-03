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
	"fmt"
	"testing"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	fakepipelineclientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
			Name:      "test-pipeline-run-fetch-error",
			Namespace: "foo",
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
	tcs := []struct {
		name        string
		pipelineRun string
		shdErr      bool
		log         string
	}{
		{"success", "foo/test-pipeline-run-success", false, ""},
		{"pipeline-fetching-error-shd-error", "test-pipeline-run-fetch-error", true, ""},
		{"invalid-pipeline-run-shd-succeed-with-logs", "foo/test-pipeline-run-doesnot-exist", false,
			"pipeline run \"foo/test-pipeline-run-doesnot-exist\" in work queue no longer exists"},
		{"invalid-pipeline-shd-succeed-with-logs", "foo/invalid-pipeline", false,
			"\"foo/invalid-pipeline\" failed to Get Pipeline: \"foo/pipeline-not-exist\""},
		{"invalid-pipeline-run-name-shd-succed-with-logs", "test/pipeline-fail/t", false,
			"invalid resource key: test/pipeline-fail/t"},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c, logs := getController(prs, ps, t)
			err := c.Reconciler.Reconcile(context.TODO(), tc.pipelineRun)
			if tc.shdErr != (err != nil) {
				t.Errorf("expected to see error %t. Got error %v", tc.shdErr, err)
			}
			if tc.log == "" && logs.Len() > 0 {
				t.Errorf("expected to see no error log. However found errors in logs: %v", logs)
			} else if tc.log != "" && logs.FilterMessage(tc.log).Len() == 0 {
				t.Errorf("expected to see error log %s. However found no matching logs in %v", tc.log, logs)
			}
		})
	}
}

func getController(prs []*v1alpha1.PipelineRun, ps []*v1alpha1.Pipeline, t *testing.T) (*controller.Impl, *observer.ObservedLogs) {
	pipelineClient := fakepipelineclientset.NewSimpleClientset()
	pipelineFactory := informers.NewSharedInformerFactory(pipelineClient, time.Second*30)
	// Confugure mock methods to get pipeline runs and pipelines for the mock data.
	rc := &reconcilerConfig{
		pipelineRunLister: &mockPipelineRunsLister{runs: prs},
		pipelineLister:    &mockPipelinesLister{p: ps},
	}
	// Create a log observer to record all error logs.
	observer, logs := observer.New(zap.ErrorLevel)
	return new(
		reconciler.Options{
			Logger:            zap.New(observer).Sugar(),
			KubeClientSet:     fakekubeclientset.NewSimpleClientset(),
			PipelineClientSet: pipelineClient,
		},
		pipelineFactory.Pipeline().V1alpha1().PipelineRuns(),
		pipelineFactory.Pipeline().V1alpha1().Pipelines(),
		rc), logs
}

// mockPipelineRunsLister implements the the interface lister.PipelineRunsLister and
// lister.PipelineRunsNamespaceLister.
type mockPipelineRunsLister struct {
	runs []*v1alpha1.PipelineRun
}

func (m *mockPipelineRunsLister) List(selector labels.Selector) (ret []*v1alpha1.PipelineRun, err error) {
	return m.runs, nil
}

func (m *mockPipelineRunsLister) PipelineRuns(namespace string) listers.PipelineRunNamespaceLister {
	return m
}

func (m *mockPipelineRunsLister) Get(name string) (*v1alpha1.PipelineRun, error) {
	///return error to mimic runtime error when fetching pipeline.
	if name == "test-pipeline-run-fetch-error" {
		return nil, fmt.Errorf("random error while fetching")
	}
	for _, run := range m.runs {
		if run.Name == name {
			return run, nil
		}
	}
	return nil, errors.NewNotFound(v1alpha1.Resource("pipelinerun"), name)
}

// mockPipelineRunsLister implements the the interface lister.PipelineLister and
// lister.PipelineNamespaceLister
type mockPipelinesLister struct {
	p []*v1alpha1.Pipeline
}

func (m *mockPipelinesLister) List(selector labels.Selector) (ret []*v1alpha1.Pipeline, err error) {
	return m.p, nil
}

func (m *mockPipelinesLister) Pipelines(namespace string) listers.PipelineNamespaceLister {
	return m
}

func (m *mockPipelinesLister) Get(name string) (*v1alpha1.Pipeline, error) {
	for _, p := range m.p {
		fmt.Println("$$$$$$$$", p)
		if p.Name == name {
			return p, nil
		}
	}
	return nil, errors.NewNotFound(v1alpha1.Resource("pipeline"), name)
}
