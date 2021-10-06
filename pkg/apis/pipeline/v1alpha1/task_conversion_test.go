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

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskConversionBadType(t *testing.T) {
	good, bad := &Task{}, &Pipeline{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}
}

func TestTaskConversion(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.Task{}}

	tests := []struct {
		name    string
		in      *Task
		wantErr bool
	}{{
		name: "simple conversion",
		in: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				TaskSpec: v1beta1.TaskSpec{
					Description: "test",
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image: "foo",
					}}},
					Volumes: []corev1.Volume{{}},
					Params: []v1beta1.ParamSpec{{
						Name:        "param-1",
						Type:        v1beta1.ParamTypeString,
						Description: "My first param",
					}},
					Resources: &v1beta1.TaskResources{
						Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "input-1",
							Type: resource.PipelineResourceTypeGit,
						}}},
						Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "output-1",
							Type: resource.PipelineResourceTypeGit,
						}}},
					},
				},
			},
		},
	}, {
		name: "deprecated and non deprecated inputs",
		in: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				TaskSpec: v1beta1.TaskSpec{
					Resources: &v1beta1.TaskResources{
						Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "input-1",
							Type: resource.PipelineResourceTypeGit,
						}}},
					},
				},
				Inputs: &Inputs{
					Resources: []TaskResource{{ResourceDeclaration: ResourceDeclaration{
						Name: "input-1",
						Type: resource.PipelineResourceTypeGit,
					}}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "deprecated and non deprecated params",
		in: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				TaskSpec: v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name:        "param-1",
						Type:        v1beta1.ParamTypeString,
						Description: "My first param",
					}},
				},
				Inputs: &Inputs{
					Params: []ParamSpec{{
						Name:        "param-1",
						Type:        v1beta1.ParamTypeString,
						Description: "My first param",
					}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "deprecated and non deprecated outputs",
		in: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				TaskSpec: v1beta1.TaskSpec{
					Resources: &v1beta1.TaskResources{
						Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "output-1",
							Type: resource.PipelineResourceTypeGit,
						}}},
					},
				},
				Outputs: &Outputs{
					Resources: []TaskResource{{ResourceDeclaration: ResourceDeclaration{
						Name: "output-1",
						Type: resource.PipelineResourceTypeGit,
					}}},
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
				got := &Task{}
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

func TestTaskConversionFromDeprecated(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.Task{}}
	tests := []struct {
		name string
		in   *Task
		want *Task
	}{{
		name: "inputs params",
		in: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				Inputs: &Inputs{
					Params: []ParamSpec{{
						Name:        "param-1",
						Type:        v1beta1.ParamTypeString,
						Description: "My first param",
					}},
				},
			},
		},
		want: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				TaskSpec: v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name:        "param-1",
						Type:        v1beta1.ParamTypeString,
						Description: "My first param",
					}},
				},
			},
		},
	}, {
		name: "inputs resource",
		in: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				Inputs: &Inputs{
					Resources: []TaskResource{{ResourceDeclaration: ResourceDeclaration{
						Name: "input-1",
						Type: resource.PipelineResourceTypeGit,
					}}},
				},
			},
		},
		want: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				TaskSpec: v1beta1.TaskSpec{
					Resources: &v1beta1.TaskResources{
						Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "input-1",
							Type: resource.PipelineResourceTypeGit,
						}}},
					},
				},
			},
		},
	}, {
		name: "outputs resource",
		in: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				Outputs: &Outputs{
					Resources: []TaskResource{{ResourceDeclaration: ResourceDeclaration{
						Name: "output-1",
						Type: resource.PipelineResourceTypeGit,
					}}},
				},
			},
		},
		want: &Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: TaskSpec{
				TaskSpec: v1beta1.TaskSpec{
					Resources: &v1beta1.TaskResources{
						Outputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "output-1",
							Type: resource.PipelineResourceTypeGit,
						}}},
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
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &Task{}
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
