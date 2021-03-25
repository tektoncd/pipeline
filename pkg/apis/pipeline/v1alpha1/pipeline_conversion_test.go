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
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/apis"
)

var ignoreAnnotations = cmp.FilterPath(func(p cmp.Path) bool {
	if len(p) < 2 {
		return false
	}
	structField, ok := p.Index(-2).(cmp.StructField)
	if !ok {
		return false
	}
	if structField.Name() != "ObjectMeta" {
		return false
	}

	structField, ok = p.Index(-1).(cmp.StructField)
	if !ok {
		return false
	}
	if structField.Name() != "Annotations" {
		return false
	}
	return true
}, cmp.Ignore())

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
					Retries: 10,
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

				if d := cmp.Diff(test.in, got, ignoreAnnotations); d != "" {
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
				t.Errorf("ConvertFrom() unexpected error: %v", err)
			}
		})
	}
}

// TestV1Beta1PipelineConversionRoundTrip checks that a populated v1beta1 pipeline
// correctly round-trips through v1alpha1 conversion and back again without losing
// any v1beta1-specific fields.
func TestV1Beta1PipelineConversionRoundTrip(t *testing.T) {
	p := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "foo",
			Namespace:  "bar",
			Generation: 1,
		},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{Name: "mytask", TaskRef: &TaskRef{Kind: "Task", Name: "task"}},
				{
					Name:    "task-with-when",
					TaskRef: &TaskRef{Kind: "Task", Name: "task"},
					WhenExpressions: v1beta1.WhenExpressions{{
						Input:    "test",
						Operator: selection.NotIn,
						Values:   []string{"foo", "bar"},
					}},
				},
			},
			Finally: []v1beta1.PipelineTask{{Name: "myfinallytask", TaskRef: &TaskRef{Kind: "Task", Name: "task"}}},
		},
	}
	downgrade := &Pipeline{}
	if err := downgrade.ConvertFrom(context.Background(), p); err != nil {
		t.Errorf("error converting from v1beta1 to v1alpha1: %v", err)
	}
	upgrade := &v1beta1.Pipeline{}
	if err := downgrade.ConvertTo(context.Background(), upgrade); err != nil {
		t.Errorf("error converting from v1alpha1 to v1beta1: %v", err)
	}
	if d := cmp.Diff(p, upgrade); d != "" {
		t.Errorf("unexpected difference between original and round-tripped v1beta1 document: %s", diff.PrintWantGot(d))
	}
}

// TestConvertToDeserializationErrors confirms that errors are returned
// for invalid serialized data.
func TestConvertToDeserializationErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source Pipeline
	}{
		{
			name: "invalid v1beta1 spec annotation",
			source: Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						V1Beta1PipelineSpecSerializedAnnotationKey: "invalid json",
					},
				},
			},
		},
		{
			name: "invalid finally annotation",
			source: Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						finallyAnnotationKey: "invalid json",
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &v1beta1.Pipeline{}
			err := tc.source.ConvertTo(context.Background(), sink)
			t.Logf("received error %q", err)
			if err == nil {
				t.Errorf("expected error but received none")
			}
		})
	}
}

// TestAlphaPipelineConversionWithFinallyAnnotation tests that a v1alpha1
// pipeline with a specific annotation correctly gets turned into a v1beta1
// pipeline with Finally section. The annotation was given to pipelines with
// Finally when converted from v1beta1 to v1alpha1 in the 0.22 Pipelines
// release.
func TestAlphaPipelineConversionWithFinallyAnnotation(t *testing.T) {
	p := &Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "foo",
			Namespace:  "bar",
			Generation: 1,
			Annotations: map[string]string{
				finallyAnnotationKey: `[{"name": "myfinallytask", "taskRef": {"name": "task"}}]`,
			},
		},
		Spec: PipelineSpec{
			Tasks: []PipelineTask{{Name: "task1", TaskRef: &TaskRef{Name: "task1"}}},
		},
	}
	expected := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "foo",
			Namespace:  "bar",
			Generation: 1,
		},
		Spec: v1beta1.PipelineSpec{
			Tasks:   []v1beta1.PipelineTask{{Name: "task1", TaskRef: &TaskRef{Name: "task1", Kind: "Task"}}},
			Finally: []v1beta1.PipelineTask{{Name: "myfinallytask", TaskRef: &TaskRef{Kind: "Task", Name: "task"}}},
		},
	}
	upgrade := &v1beta1.Pipeline{}
	if err := p.ConvertTo(context.Background(), upgrade); err != nil {
		t.Errorf("error converting from v1alpha1 to v1beta1: %v", err)
	}
	if d := cmp.Diff(expected, upgrade); d != "" {
		t.Errorf("unexpected difference: %s", diff.PrintWantGot(d))
	}
}
