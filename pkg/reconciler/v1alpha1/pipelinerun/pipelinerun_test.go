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
				m := getLogMessages(logs)
				t.Errorf("Log lines diff %s", cmp.Diff(tc.log, m))
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
