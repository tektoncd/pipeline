/*
Copyright 2020 The Tetkon Authors

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

package v1beta1_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/apis"
)

func TestPipelineConversionBadType(t *testing.T) {
	good, bad := &v1beta1.Pipeline{}, &v1beta1.Task{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestPipelineConversion(t *testing.T) {
	for _, test := range []struct {
		name string
		in   *v1beta1.Pipeline
	}{{
		name: "simple pipeline",
		in: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineSpec{
				DisplayName: "pipeline-display-name",
				Description: "test",
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "example.com/my-foo-task"},
				}},
				Params: []v1beta1.ParamSpec{{
					Name:        "param-1",
					Type:        v1beta1.ParamTypeString,
					Description: "My first param",
				}},
			},
		},
	}, {
		name: "pipeline with deprecated fields in step and stepTemplate",
		in: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "task-1",
					TaskSpec: &v1beta1.EmbeddedTask{
						TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								DeprecatedLivenessProbe:  &corev1.Probe{InitialDelaySeconds: 1},
								DeprecatedReadinessProbe: &corev1.Probe{InitialDelaySeconds: 2},
								DeprecatedPorts:          []corev1.ContainerPort{{Name: "port"}},
								DeprecatedStartupProbe:   &corev1.Probe{InitialDelaySeconds: 3},
								DeprecatedLifecycle: &corev1.Lifecycle{PostStart: &corev1.LifecycleHandler{Exec: &corev1.ExecAction{
									Command: []string{"lifecycle command"},
								}}},
								DeprecatedTerminationMessagePath:   "path",
								DeprecatedTerminationMessagePolicy: corev1.TerminationMessagePolicy("policy"),
								DeprecatedStdin:                    true,
								DeprecatedStdinOnce:                true,
								DeprecatedTTY:                      true,
							}},
							StepTemplate: &v1beta1.StepTemplate{
								DeprecatedName:           "deprecated-step-template",
								DeprecatedLivenessProbe:  &corev1.Probe{InitialDelaySeconds: 1},
								DeprecatedReadinessProbe: &corev1.Probe{InitialDelaySeconds: 2},
								DeprecatedPorts:          []corev1.ContainerPort{{Name: "port"}},
								DeprecatedStartupProbe:   &corev1.Probe{InitialDelaySeconds: 3},
								DeprecatedLifecycle: &corev1.Lifecycle{PostStart: &corev1.LifecycleHandler{Exec: &corev1.ExecAction{
									Command: []string{"lifecycle command"},
								}}},
								DeprecatedTerminationMessagePath:   "path",
								DeprecatedTerminationMessagePolicy: corev1.TerminationMessagePolicy("policy"),
								DeprecatedStdin:                    true,
								DeprecatedStdinOnce:                true,
								DeprecatedTTY:                      true,
							},
						},
					},
				}, {
					Name: "task-2",
					TaskSpec: &v1beta1.EmbeddedTask{
						TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								DeprecatedLivenessProbe: &corev1.Probe{InitialDelaySeconds: 1},
							}},
							StepTemplate: &v1beta1.StepTemplate{
								DeprecatedLivenessProbe: &corev1.Probe{InitialDelaySeconds: 1},
							},
						},
					},
				}},
			},
		},
	}, {
		name: "pipeline conversion all non deprecated fields",
		in: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: v1beta1.PipelineSpec{
				DisplayName: "pipeline-display-name",
				Description: "test",
				Tasks: []v1beta1.PipelineTask{{
					Name: "task-1",
				}, {
					Name:        "foo",
					DisplayName: "task-display-name",
					Description: "task-description",
					TaskRef:     &v1beta1.TaskRef{Name: "example.com/my-foo-task"},
					TaskSpec: &v1beta1.EmbeddedTask{
						TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Name:  "mystep",
								Image: "myimage",
							}},
							Params: []v1beta1.ParamSpec{{
								Name:        "string-param",
								Type:        v1beta1.ParamTypeString,
								Description: "String param",
							}},
						},
					},
					WhenExpressions: v1beta1.WhenExpressions{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"foo", "bar"},
					}},
					Retries:  1,
					RunAfter: []string{"task-1"},
					Params: v1beta1.Params{{
						Name: "param-task-1",
						Value: v1beta1.ParamValue{
							ArrayVal: []string{"value-task-1"},
							Type:     "string",
						},
					}},
					Matrix: &v1beta1.Matrix{
						Params: v1beta1.Params{{
							Name: "a-param",
							Value: v1beta1.ParamValue{
								Type:     v1beta1.ParamTypeArray,
								ArrayVal: []string{"$(params.baz)", "and", "$(params.foo-is-baz)"},
							},
						}},
						Include: v1beta1.IncludeParamsList{{
							Name: "baz",
							Params: v1beta1.Params{{
								Name: "a-param", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "$(params.baz)"},
							}, {
								Name: "flags", Value: v1beta1.ParamValue{Type: v1beta1.ParamTypeString, StringVal: "-cover -v"}}},
						}},
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "my-task-workspace",
						Workspace: "source",
					}},
					Timeout: &metav1.Duration{Duration: 5 * time.Minute},
				},
				},
				Params: []v1beta1.ParamSpec{{
					Name:        "param-1",
					Type:        v1beta1.ParamTypeString,
					Description: "My first pipeline param",
					Properties:  map[string]v1beta1.PropertySpec{"foo": {Type: v1beta1.ParamTypeString}},
					Default:     v1beta1.NewStructuredValues("bar"),
				}},
				Workspaces: []v1beta1.WorkspacePipelineDeclaration{{
					Name:        "workspace",
					Description: "description",
					Optional:    true,
				}},
				Results: []v1beta1.PipelineResult{{
					Name:        "my-pipeline-result",
					Type:        v1beta1.ResultsTypeObject,
					Description: "this is my pipeline result",
					Value:       *v1beta1.NewStructuredValues("foo.bar"),
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:        "final-task",
					DisplayName: "final-task-display-name",
					Description: "final-task-description",
					TaskRef:     &v1beta1.TaskRef{Name: "foo-task"},
				}},
			},
		},
	}} {
		t.Run(test.name, func(t *testing.T) {
			versions := []apis.Convertible{&v1.Pipeline{}}
			for _, version := range versions {
				t.Run(test.name, func(t *testing.T) {
					ver := version
					if err := test.in.ConvertTo(context.Background(), ver); err != nil {
						t.Errorf("ConvertTo() = %v", err)
						return
					}
					t.Logf("ConvertTo() = %#v", ver)
					got := &v1beta1.Pipeline{}
					if err := got.ConvertFrom(context.Background(), ver); err != nil {
						t.Errorf("ConvertFrom() = %v", err)
					}
					t.Logf("ConvertFrom() = %#v", got)
					if d := cmp.Diff(test.in, got); d != "" {
						t.Errorf("roundtrip %s", diff.PrintWantGot(d))
					}
				})
			}
		})
	}
}
