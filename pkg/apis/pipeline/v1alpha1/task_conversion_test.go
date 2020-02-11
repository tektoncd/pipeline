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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskConversionBadType(t *testing.T) {
	good, bad := &Task{}, &Pipeline{}

	if err := good.ConvertUp(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}

	if err := good.ConvertDown(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}
}

func TestTaskConversion(t *testing.T) {
	versions := []apis.Convertible{&v1alpha2.Task{}}

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
				TaskSpec: v1alpha2.TaskSpec{
					Steps: []v1alpha2.Step{{Container: corev1.Container{
						Image: "foo",
					}}},
					Volumes: []corev1.Volume{{}},
					Params: []v1alpha2.ParamSpec{{
						Name:        "param-1",
						Type:        v1alpha2.ParamTypeString,
						Description: "My first param",
					}},
					Resources: &v1alpha2.TaskResources{
						Inputs: []v1alpha2.TaskResource{{ResourceDeclaration: v1alpha2.ResourceDeclaration{
							Name: "input-1",
							Type: resource.PipelineResourceTypeGit,
						}}},
						Outputs: []v1alpha2.TaskResource{{ResourceDeclaration: v1alpha2.ResourceDeclaration{
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
				TaskSpec: v1alpha2.TaskSpec{
					Resources: &v1alpha2.TaskResources{
						Inputs: []v1alpha2.TaskResource{{ResourceDeclaration: v1alpha2.ResourceDeclaration{
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
				TaskSpec: v1alpha2.TaskSpec{
					Params: []v1alpha2.ParamSpec{{
						Name:        "param-1",
						Type:        v1alpha2.ParamTypeString,
						Description: "My first param",
					}},
				},
				Inputs: &Inputs{
					Params: []ParamSpec{{
						Name:        "param-1",
						Type:        v1alpha2.ParamTypeString,
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
				TaskSpec: v1alpha2.TaskSpec{
					Resources: &v1alpha2.TaskResources{
						Outputs: []v1alpha2.TaskResource{{ResourceDeclaration: v1alpha2.ResourceDeclaration{
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
				if err := test.in.ConvertUp(context.Background(), ver); err != nil {
					if !test.wantErr {
						t.Errorf("ConvertUp() = %v", err)
					}
					return
				}
				t.Logf("ConvertUp() = %#v", ver)
				got := &Task{}
				if err := got.ConvertDown(context.Background(), ver); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				t.Logf("ConvertDown() = %#v", got)
				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

func TestTaskConversionFromDeprecated(t *testing.T) {
	versions := []apis.Convertible{&v1alpha2.Task{}}
	tests := []struct {
		name     string
		in       *Task
		want     *Task
		badField string
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
						Type:        v1alpha2.ParamTypeString,
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
				TaskSpec: v1alpha2.TaskSpec{
					Params: []v1alpha2.ParamSpec{{
						Name:        "param-1",
						Type:        v1alpha2.ParamTypeString,
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
				TaskSpec: v1alpha2.TaskSpec{
					Resources: &v1alpha2.TaskResources{
						Inputs: []v1alpha2.TaskResource{{ResourceDeclaration: v1alpha2.ResourceDeclaration{
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
				TaskSpec: v1alpha2.TaskSpec{
					Resources: &v1alpha2.TaskResources{
						Outputs: []v1alpha2.TaskResource{{ResourceDeclaration: v1alpha2.ResourceDeclaration{
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
				if err := test.in.ConvertUp(context.Background(), ver); err != nil {
					if test.badField != "" {
						cce, ok := err.(*CannotConvertError)
						if ok && cce.Field == test.badField {
							return
						}
					}
					t.Errorf("ConvertUp() = %v", err)
				}
				t.Logf("ConvertUp() = %#v", ver)
				got := &Task{}
				if err := got.ConvertDown(context.Background(), ver); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				t.Logf("ConvertDown() = %#v", got)
				if diff := cmp.Diff(test.want, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
