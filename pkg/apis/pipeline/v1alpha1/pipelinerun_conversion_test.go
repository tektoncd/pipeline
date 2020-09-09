/*
Copyright 2020 The Tekton Authors

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

package v1alpha1

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineRunConversionBadType(t *testing.T) {
	good, bad := &PipelineRun{}, &Pipeline{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}
}

func TestPipelineRunConversion(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.PipelineRun{}}

	tests := []struct {
		name    string
		in      *PipelineRun
		wantErr bool
	}{{
		name: "simple conversion pipelineref",
		in: &PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: PipelineRunSpec{
				PipelineRef: &PipelineRef{
					Name: "pipeline",
				},
				ServiceAccountName: "sa",
				ServiceAccountNames: []PipelineRunSpecServiceAccountName{{
					TaskName:           "t1",
					ServiceAccountName: "sa1",
				}},
				Timeout: &metav1.Duration{Duration: 1 * time.Minute},
				PodTemplate: &PodTemplate{
					NodeSelector: map[string]string{"foo": "bar"},
				},
				Workspaces: []WorkspaceBinding{{
					Name:     "w1",
					SubPath:  "foo",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				Params: []Param{{
					Name:  "p1",
					Value: *v1beta1.NewArrayOrString("baz"),
				}},
				Resources: []PipelineResourceBinding{{
					Name:        "i1",
					ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
				}},
			},
			Status: PipelineRunStatus{
				PipelineRunStatusFields: PipelineRunStatusFields{
					StartTime:      &metav1.Time{Time: time.Now().Add(-4 * time.Minute)},
					CompletionTime: &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
					TaskRuns: map[string]*PipelineRunTaskRunStatus{
						"foo-run": {
							PipelineTaskName: "foo",
						},
					},
				},
			},
		},
	}, {
		name: "simple conversion pipelinespec",
		in: &PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: PipelineRunSpec{
				PipelineSpec: &PipelineSpec{
					Tasks: []PipelineTask{{
						Name: "task1",
						TaskRef: &TaskRef{
							Name: "taskref",
						},
					}, {
						Name: "task2",
						TaskSpec: &TaskSpec{TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{Container: corev1.Container{
								Image: "foo",
							}}},
						}},
						RunAfter: []string{"task1"},
					}},
				},
				ServiceAccountName: "sa",
				ServiceAccountNames: []PipelineRunSpecServiceAccountName{{
					TaskName:           "t1",
					ServiceAccountName: "sa1",
				}},
				Timeout: &metav1.Duration{Duration: 1 * time.Minute},
				PodTemplate: &PodTemplate{
					NodeSelector: map[string]string{"foo": "bar"},
				},
				Workspaces: []WorkspaceBinding{{
					Name:     "w1",
					SubPath:  "foo",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				Params: []Param{{
					Name:  "p1",
					Value: *v1beta1.NewArrayOrString("baz"),
				}},
				Resources: []PipelineResourceBinding{{
					Name:        "i1",
					ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
				}},
			},
			Status: PipelineRunStatus{
				PipelineRunStatusFields: PipelineRunStatusFields{
					StartTime:      &metav1.Time{Time: time.Now().Add(-4 * time.Minute)},
					CompletionTime: &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
					TaskRuns: map[string]*PipelineRunTaskRunStatus{
						"foo-run": {
							PipelineTaskName: "foo",
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					if !test.wantErr {
						t.Errorf("ConvertTo() = %v", err)
					}
					return
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &PipelineRun{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.in, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}
