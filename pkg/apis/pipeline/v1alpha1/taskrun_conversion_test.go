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

func TestTaskRunConversionBadType(t *testing.T) {
	good, bad := &TaskRun{}, &Task{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}
}

func TestTaskRunConversion(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.TaskRun{}}

	tests := []struct {
		name    string
		in      *TaskRun
		wantErr bool
	}{{
		name: "simple conversion taskref",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				TaskRef: &TaskRef{
					Name: "task",
				},
				ServiceAccountName: "sa",
				Timeout:            &metav1.Duration{Duration: 1 * time.Minute},
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
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "i1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "o1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r2"},
						},
					}},
				},
			},
			Status: TaskRunStatus{
				TaskRunStatusFields: TaskRunStatusFields{
					PodName:        "foo",
					StartTime:      &metav1.Time{Time: time.Now().Add(-4 * time.Minute)},
					CompletionTime: &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
					Steps: []StepState{{
						Name: "s1",
					}},
				},
			},
		},
	}, {
		name: "simple conversion taskspec",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				TaskSpec: &TaskSpec{TaskSpec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image: "foo",
					}}},
				}},
				ServiceAccountName: "sa",
				Timeout:            &metav1.Duration{Duration: 1 * time.Minute},
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
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "i1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "o1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r2"},
						},
					}},
				},
			},
		},
	}, {
		name: "deprecated and non deprecated params",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				TaskSpec: &TaskSpec{TaskSpec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image: "foo",
					}}},
				}},
				ServiceAccountName: "sa",
				Timeout:            &metav1.Duration{Duration: 1 * time.Minute},
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
				Inputs: &TaskRunInputs{
					Params: []Param{{
						Name:  "p2",
						Value: *v1beta1.NewArrayOrString("bar"),
					}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "deprecated and non deprecated inputs",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				TaskSpec: &TaskSpec{TaskSpec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image: "foo",
					}}},
				}},
				ServiceAccountName: "sa",
				Timeout:            &metav1.Duration{Duration: 1 * time.Minute},
				PodTemplate: &PodTemplate{
					NodeSelector: map[string]string{"foo": "bar"},
				},
				Workspaces: []WorkspaceBinding{{
					Name:     "w1",
					SubPath:  "foo",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "i1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
				Inputs: &TaskRunInputs{
					Resources: []TaskResourceBinding{{
						PipelineResourceBinding: PipelineResourceBinding{
							Name:        "i1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "deprecated and non deprecated outputs",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				TaskSpec: &TaskSpec{TaskSpec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image: "foo",
					}}},
				}},
				ServiceAccountName: "sa",
				Timeout:            &metav1.Duration{Duration: 1 * time.Minute},
				PodTemplate: &PodTemplate{
					NodeSelector: map[string]string{"foo": "bar"},
				},
				Workspaces: []WorkspaceBinding{{
					Name:     "w1",
					SubPath:  "foo",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "o1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
				Outputs: &TaskRunOutputs{
					Resources: []TaskResourceBinding{{
						PipelineResourceBinding: PipelineResourceBinding{
							Name:        "o1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
			},
		},
		wantErr: true,
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
				got := &TaskRun{}
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

func TestTaskRunConversionFromDeprecated(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.TaskRun{}}
	tests := []struct {
		name string
		in   *TaskRun
		want *TaskRun
	}{{
		name: "inputs params",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				Inputs: &TaskRunInputs{
					Params: []Param{{
						Name:  "p2",
						Value: *v1beta1.NewArrayOrString("bar"),
					}},
				},
			},
		},
		want: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				Params: []Param{{
					Name:  "p2",
					Value: *v1beta1.NewArrayOrString("bar"),
				}},
			},
		},
	}, {
		name: "inputs resource",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				Inputs: &TaskRunInputs{
					Resources: []TaskResourceBinding{{
						PipelineResourceBinding: PipelineResourceBinding{
							Name:        "i1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
			},
		},
		want: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Inputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "i1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
			},
		},
	}, {
		name: "outputs resource",
		in: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				Outputs: &TaskRunOutputs{
					Resources: []TaskResourceBinding{{
						PipelineResourceBinding: PipelineResourceBinding{
							Name:        "o1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
			},
		},
		want: &TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskRunSpec{
				Resources: &v1beta1.TaskRunResources{
					Outputs: []v1beta1.TaskResourceBinding{{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name:        "o1",
							ResourceRef: &v1beta1.PipelineResourceRef{Name: "r1"},
						},
						Paths: []string{"foo", "bar"},
					}},
				},
			},
		},
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &TaskRun{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.want, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}
