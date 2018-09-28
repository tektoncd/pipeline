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
	"fmt"
	"testing"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/pkg/test/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

func TestReconcile(t *testing.T) {
	logging.InitializeLogger(false)

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
		}},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pipeline-run-fail",
				Namespace: "foo",
			},
			Spec: v1alpha1.PipelineRunSpec{
				PipelineRef: v1alpha1.PipelineRef{
					Name:       "test-pipeline-does-not-exist",
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
		name           string
		pipelineRun    string
		runtimeSuccess bool
		expectedRecord string
	}{
		{"success", "foo/test-pipeline-run-success", true, "Normal Synced Found pipeline test-pipeline"},
		{"fail", "test-pipeline-run-fail", false, ""},
	}

	rc := &reconcilerConfig{
		pipelineRunLister: &mockPipelineRunsLister{runs: prs},
		pipelineLister:    &mockPipelinesLister{p: ps},
	}
	c := newController(reconciler.Options{}, nil, nil, rc)
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			old := runtime.ErrorHandlers
			var actualErr error
			defer func() { runtime.ErrorHandlers = old }()
			runtime.ErrorHandlers = []func(error){
				func(err error) {
					actualErr = err
				},
			}
			err := c.Reconciler.Reconcile(nil, tc.pipelineRun)
			if err != actualErr {
				t.Errorf("expected to see error %t. Got error %v", tc.runtimeSuccess, actualErr)
			}
		})
	}
}

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
	for _, run := range m.runs {
		if run.Name == name {
			return run, nil
		}
	}
	return nil, fmt.Errorf("%s not found", name)
}

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
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%s not found", name)
}
