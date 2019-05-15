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

package reconciler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// Test case for providing recorder in the option
func TestRecorderOptions(t *testing.T) {

	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-completed", "foo",
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccount("test-sa")),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Reason:  resources.ReasonSucceeded,
			Message: "All Tasks have completed executing",
		})),
	)}
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hellow-world"),
	))}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	c, _ := test.SeedTestData(t, d)

	observer, _ := observer.New(zap.InfoLevel)

	// recorder is ont provided in the option
	b := NewBase(Options{
		Logger:            zap.New(observer).Sugar(),
		KubeClientSet:     c.Kube,
		PipelineClientSet: c.Pipeline,
	}, "test")

	if strings.Compare(reflect.TypeOf(b.Recorder).String(), "*record.recorderImpl") != 0 {
		t.Errorf("Expected recorder type '*record.recorderImpl' but actual type is: %s", reflect.TypeOf(b.Recorder).String())
	}

	fr := record.NewFakeRecorder(1)

	// recorder is provided in the option
	b = NewBase(Options{
		Logger:            zap.New(observer).Sugar(),
		KubeClientSet:     c.Kube,
		PipelineClientSet: c.Pipeline,
		Recorder:          fr,
	}, "test")

	if strings.Compare(reflect.TypeOf(b.Recorder).String(), "*record.FakeRecorder") != 0 {
		t.Errorf("Expected recorder type '*record.FakeRecorder' but actual type is: %s", reflect.TypeOf(b.Recorder).String())
	}
}
