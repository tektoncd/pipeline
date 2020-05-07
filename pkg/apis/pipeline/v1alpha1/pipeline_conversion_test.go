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
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineConversionBadType(t *testing.T) {
	good, bad := &Pipeline{}, &Task{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}
}

func TestPipelineConversion(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.Pipeline{}}

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
				Description: "test",
				Resources: []PipelineDeclaredResource{{
					Name: "resource1",
					Type: resource.PipelineResourceTypeGit,
				}, {
					Name: "resource2",
					Type: resource.PipelineResourceTypeImage,
				}},
				Params: []ParamSpec{{
					Name:        "param-1",
					Type:        v1beta1.ParamTypeString,
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
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "resource1",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name:     "output1",
							Resource: "resource2",
						}},
					},
					Params: []Param{{
						Name:  "param1",
						Value: v1beta1.ArrayOrString{StringVal: "str", Type: v1beta1.ParamTypeString},
					}},
					Workspaces: []WorkspacePipelineTaskBinding{{
						Name:      "w1",
						Workspace: "workspace1",
					}},
					Timeout: &metav1.Duration{Duration: 5 * time.Minute},
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
					Type:        v1beta1.ParamTypeString,
					Description: "My first param",
				}},
				Tasks: []PipelineTask{{
					Name: "task2",
					TaskSpec: &TaskSpec{
						TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{Container: corev1.Container{
								Image: "foo",
							}}},
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
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					if !test.wantErr {
						t.Errorf("ConvertTo() = %v", err)
					}
					return
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &Pipeline{}
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
