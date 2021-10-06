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

func TestPipelineConversion_Success(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.Pipeline{}}

	tests := []struct {
		name string
		in   *Pipeline
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
				Workspaces: []PipelineWorkspaceDeclaration{{
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
						Value: *v1beta1.NewArrayOrString("str"),
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
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				// convert v1alpha1 Pipeline to v1beta1 Pipeline
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &Pipeline{}
				// converting it back to v1alpha1 pipeline and storing it in got variable to compare with original input
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				// compare origin input and roundtrip Pipeline i.e. v1alpha1 pipeline converted to v1beta1 and then converted back to v1alpha1
				// this check is making sure that we do not end up with different object than what we started with
				if d := cmp.Diff(test.in, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestPipelineConversion_Failure(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.Pipeline{}}

	tests := []struct {
		name string
		in   *Pipeline
	}{{
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
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err == nil {
					t.Errorf("Expected ConvertTo to fail but did not produce any error")
				}
				return
			})
		}
	}
}

func TestPipelineConversionFromWithFinally(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.Pipeline{}}
	p := &Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "foo",
			Namespace:  "bar",
			Generation: 1,
		},
		Spec: PipelineSpec{
			Tasks: []PipelineTask{{Name: "mytask", TaskRef: &TaskRef{Name: "task"}}},
		},
	}
	for _, version := range versions {
		t.Run("finally not available in v1alpha1", func(t *testing.T) {
			ver := version
			// convert v1alpha1 to v1beta1
			if err := p.ConvertTo(context.Background(), ver); err != nil {
				t.Errorf("ConvertTo() = %v", err)
			}
			source := ver
			source.(*v1beta1.Pipeline).Spec.Finally = []v1beta1.PipelineTask{{Name: "finaltask", TaskRef: &TaskRef{Name: "task"}}}
			got := &Pipeline{}
			if err := got.ConvertFrom(context.Background(), source); err != nil {
				t.Errorf("ConvertFrom() should have failed")
			}
		})
	}
}

func TestPipelineConversionFromBetaToAlphaWithFinally(t *testing.T) {
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "foo",
			Namespace:  "bar",
			Generation: 1,
		},
		Spec: v1beta1.PipelineSpec{
			Tasks:   []v1beta1.PipelineTask{{Name: "mytask", TaskRef: &TaskRef{Name: "task"}}},
			Finally: []v1beta1.PipelineTask{{Name: "myfinallytask", TaskRef: &TaskRef{Name: "task"}}},
		},
	}
	t.Run("finally stored by v1alpha1 and rehydrated for v1beta1", func(t *testing.T) {
		downgrade := &Pipeline{}
		if err := downgrade.ConvertFrom(context.Background(), p); err != nil {
			t.Errorf("error converting from v1beta1 with finally field to v1alpha1 with finally annotation: %v", err)
		}
		upgrade := &v1beta1.Pipeline{}
		if err := downgrade.ConvertTo(context.Background(), upgrade); err != nil {
			t.Errorf("error converting from v1alpha1 with finally annotation to v1beta1 with finally field: %v", err)
		}
		if d := cmp.Diff(p, upgrade); d != "" {
			t.Errorf("unexpected difference between v1beta1 with finally field and round-tripped v1beta1 with finally field: %s", diff.PrintWantGot(d))
		}
	})
}
