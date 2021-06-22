/*
Copyright 2021 The Tekton Authors

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

package resolution

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCopyTaskMetaToTaskRun_Labels(t *testing.T) {
	for _, tc := range []struct {
		name           string
		taskMeta       metav1.ObjectMeta
		expectedLabels map[string]string
	}{{
		name:           "empty task meta labels result in no change to taskrun",
		taskMeta:       metav1.ObjectMeta{},
		expectedLabels: nil,
	}, {
		name:           "non-empty labels copied verbatim to taskrun",
		taskMeta:       metav1.ObjectMeta{Labels: map[string]string{"foo": "bar", "baz": "quux"}},
		expectedLabels: map[string]string{"foo": "bar", "baz": "quux"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &v1beta1.TaskRun{}
			CopyTaskMetaToTaskRun(&tc.taskMeta, tr)
			if d := cmp.Diff(tc.expectedLabels, tr.ObjectMeta.Labels); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestCopyTaskMetaToTaskRun_TaskRef(t *testing.T) {
	for _, tc := range []struct {
		name           string
		taskRef        v1beta1.TaskRef
		expectedLabels map[string]string
	}{{
		name:           "task ref with empty kind results in task label on taskrun",
		taskRef:        v1beta1.TaskRef{Name: "foo"},
		expectedLabels: map[string]string{pipeline.TaskLabelKey: "foo"},
	}, {
		name:           "task ref with task kind results in task label on taskrun",
		taskRef:        v1beta1.TaskRef{Kind: "Task", Name: "foo"},
		expectedLabels: map[string]string{pipeline.TaskLabelKey: "foo"},
	}, {
		name:           "task ref with clustertask kind results in clustertask label on taskrun",
		taskRef:        v1beta1.TaskRef{Kind: "ClusterTask", Name: "foo"},
		expectedLabels: map[string]string{pipeline.ClusterTaskLabelKey: "foo"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			tr := v1beta1.TaskRun{
				Spec: v1beta1.TaskRunSpec{TaskRef: &tc.taskRef},
			}
			CopyTaskMetaToTaskRun(&metav1.ObjectMeta{}, &tr)
			if d := cmp.Diff(tc.expectedLabels, tr.Labels); d != "" {
				diff.PrintWantGot(d)
			}
		})
	}
}

func TestCopyTaskMetaToTaskRun_Annotations(t *testing.T) {
	for _, tc := range []struct {
		name                string
		taskMeta            metav1.ObjectMeta
		expectedAnnotations map[string]string
	}{{
		name:                "nil task annotations still results in empty annotation map on taskrun",
		taskMeta:            metav1.ObjectMeta{},
		expectedAnnotations: map[string]string{},
	}, {
		name:                "empty task annotations results in empty annotation map on taskrun",
		taskMeta:            metav1.ObjectMeta{Annotations: map[string]string{}},
		expectedAnnotations: map[string]string{},
	}, {
		name:                "non-empty task annotations are copied verbatim to taskrun",
		taskMeta:            metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}},
		expectedAnnotations: map[string]string{"foo": "bar"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &v1beta1.TaskRun{}
			CopyTaskMetaToTaskRun(&tc.taskMeta, tr)
			if d := cmp.Diff(tc.expectedAnnotations, tr.ObjectMeta.Annotations); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestCopyPipelineMetaToPipelineRun_Labels(t *testing.T) {
	for _, tc := range []struct {
		name           string
		pipelineMeta   metav1.ObjectMeta
		expectedLabels map[string]string
	}{{
		name:           "pipeline name always added as a pipelinerun label",
		pipelineMeta:   metav1.ObjectMeta{Name: "foo"},
		expectedLabels: map[string]string{pipeline.PipelineLabelKey: "foo"},
	}, {
		name:           "empty pipeline meta results in pipelinerun receiving empty name label",
		pipelineMeta:   metav1.ObjectMeta{},
		expectedLabels: map[string]string{pipeline.PipelineLabelKey: ""},
	}, {
		name:         "non-empty pipeline labels copied verbatim to pipelinerun",
		pipelineMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{"foo": "bar", "baz": "quux"}},
		expectedLabels: map[string]string{
			"foo":                     "bar",
			"baz":                     "quux",
			pipeline.PipelineLabelKey: "foo",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1beta1.PipelineRun{}
			CopyPipelineMetaToPipelineRun(&tc.pipelineMeta, pr)
			if d := cmp.Diff(tc.expectedLabels, pr.ObjectMeta.Labels); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestCopyPipelineMetaToPipelineRun_Annotations(t *testing.T) {
	for _, tc := range []struct {
		name                string
		pipelineMeta        metav1.ObjectMeta
		expectedAnnotations map[string]string
	}{{
		name:                "nil pipeline annotations still results in empty annotation map on pipelinerun",
		pipelineMeta:        metav1.ObjectMeta{},
		expectedAnnotations: map[string]string{},
	}, {
		name:                "empty pipeline annotations results in empty annotation map on pipelinerun",
		pipelineMeta:        metav1.ObjectMeta{Annotations: map[string]string{}},
		expectedAnnotations: map[string]string{},
	}, {
		name:                "non-empty pipeline annotations are copied verbatim to pipelinerun",
		pipelineMeta:        metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}},
		expectedAnnotations: map[string]string{"foo": "bar"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1beta1.PipelineRun{}
			CopyPipelineMetaToPipelineRun(&tc.pipelineMeta, pr)
			if d := cmp.Diff(tc.expectedAnnotations, pr.ObjectMeta.Annotations); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}
