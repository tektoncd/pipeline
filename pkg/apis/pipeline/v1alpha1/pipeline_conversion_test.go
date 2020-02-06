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

func TestPipelineConversionBadType(t *testing.T) {
	good, bad := &Pipeline{}, &Task{}

	if err := good.ConvertUp(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}

	if err := good.ConvertDown(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}
}

func TestPipelineConversion(t *testing.T) {
	versions := []apis.Convertible{&v1alpha2.Pipeline{}}

	tests := []struct {
		name    string
		in      *Pipeline
		wantErr bool
	}{{
		name: "simple conversion",
		in: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: PipelineSpec{
				Resources: []PipelineDeclaredResource{{
					Name: "resource1",
					Type: resource.PipelineResourceTypeGit,
				}, {
					Name: "resource2",
					Type: resource.PipelineResourceTypeImage,
				}},
				Params: []ParamSpec{{
					Name:        "param-1",
					Type:        v1alpha2.ParamTypeString,
					Description: "My first param",
				}},
				Workspaces: []WorkspacePipelineDeclaration{{
					Name: "workspace1",
				}},
				Tasks: []PipelineTask{{
					Name: "task1",
					TaskRef: &TaskRef{
						Name: "taskref",
					},
					Conditions: []PipelineTaskCondition{{
						ConditionRef: "condition1",
					}},
					Retries:  10,
					RunAfter: []string{"task1"},
					Resources: &PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "resource1",
						}},
						Outputs: []v1alpha2.PipelineTaskOutputResource{{
							Name:     "output1",
							Resource: "resource2",
						}},
					},
					Params: []Param{{
						Name:  "param1",
						Value: v1alpha2.ArrayOrString{StringVal: "str", Type: v1alpha2.ParamTypeString},
					}},
					Workspaces: []WorkspacePipelineTaskBinding{{
						Name:      "w1",
						Workspace: "workspace1",
					}},
				}, {
					Name: "task2",
					TaskSpec: &TaskSpec{TaskSpec: v1alpha2.TaskSpec{
						Steps: []v1alpha2.Step{{Container: corev1.Container{
							Image: "foo",
						}}},
					}},
					RunAfter: []string{"task1"},
				}},
			},
		},
	}, {
		name: "simple conversion with task spec error",
		in: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: PipelineSpec{
				Params: []ParamSpec{{
					Name:        "param-1",
					Type:        v1alpha2.ParamTypeString,
					Description: "My first param",
				}},
				Tasks: []PipelineTask{{
					Name: "task2",
					TaskSpec: &TaskSpec{
						TaskSpec: v1alpha2.TaskSpec{
							Steps: []v1alpha2.Step{{Container: corev1.Container{
								Image: "foo",
							}}},
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
						}},
					RunAfter: []string{"task1"},
				}},
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
				got := &Pipeline{}
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
