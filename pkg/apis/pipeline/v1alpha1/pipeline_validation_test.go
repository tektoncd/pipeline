/*
Copyright 2019 The Tekton Authors

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

package v1alpha1_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipeline_Validate(t *testing.T) {
	for _, tt := range []struct {
		name            string
		p               *v1alpha1.Pipeline
		failureExpected bool
	}{{
		name: "valid metadata",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "period in name",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipe.line"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline name too long",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", 64)},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec missing",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid (duplicate tasks)",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}, {
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec empty task name",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid task name",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "_foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid task name (capital)",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "Foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid taskref name",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "_foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec missing taskfref and taskspec",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name: "foo",
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec with taskref and taskspec",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					TaskSpec: &v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
						Steps: []v1alpha1.Step{{Container: corev1.Container{
							Name:  "foo",
							Image: "bar",
						}}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid taskspec",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:     "foo",
					TaskSpec: &v1alpha1.TaskSpec{},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec valid taskspec",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name: "foo",
					TaskSpec: &v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
						Steps: []v1alpha1.Step{{Container: corev1.Container{
							Name:  "foo",
							Image: "bar",
						}}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "no duplicate tasks",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}, {
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
				}},
			},
		},
		failureExpected: false,
	}, {
		// Adding this case because `task.Resources` is a pointer, explicitly making sure this is handled
		name: "task without resources",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "wonderful-resource",
					Type: v1alpha1.PipelineResourceTypeImage,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "wow-image", Resource: "wonderful-resource",
						}},
					},
				}, {
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid resource declarations and usage",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}, {
					Name: "wonderful-resource",
					Type: v1alpha1.PipelineResourceTypeImage,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "wow-image", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					},
					Conditions: []v1alpha1.PipelineTaskCondition{{
						ConditionRef: "some-condition-2",
						Resources: []v1alpha1.PipelineTaskInputResource{{
							Name: "wow-image", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					}},
				}, {
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
						Outputs: []v1alpha1.PipelineTaskOutputResource{{
							Name: "some-image", Resource: "wonderful-resource",
						}},
					},
					Conditions: []v1alpha1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid condition only resource",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Conditions: []v1alpha1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid parameter variables",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
					Type: v1alpha1.ParamTypeString,
				}, {
					Name: "foo-is-baz",
					Type: v1alpha1.ParamTypeString,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(baz) and $(foo-is-baz)"),
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid array parameter variables",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "baz",
					Type:    v1alpha1.ParamTypeArray,
					Default: v1beta1.NewArrayOrString("some", "default"),
				}, {
					Name: "foo-is-baz",
					Type: v1alpha1.ParamTypeArray,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(baz)", "and", "$(foo-is-baz)"),
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid star array parameter variables",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "baz",
					Type:    v1alpha1.ParamTypeArray,
					Default: v1beta1.NewArrayOrString("some", "default"),
				}, {
					Name: "foo-is-baz",
					Type: v1alpha1.ParamTypeArray,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(baz[*])", "and", "$(foo-is-baz[*])"),
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "pipeline parameter nested in task parameter",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name: "baz",
					Type: v1alpha1.ParamTypeString,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(input.workspace.$(baz))"),
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "from is on only task",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "from task doesnt exist",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource", From: []string{"bazzz"},
						}},
					},
				}, {
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "duplicate resource declaration",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}, {
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "output resources missing from declaration",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource",
						}},
						Outputs: []v1alpha1.PipelineTaskOutputResource{{
							Name: "the-magic-resource", Resource: "missing-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "input resources missing from declaration",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "missing-resource",
						}},
						Outputs: []v1alpha1.PipelineTaskOutputResource{{
							Name: "the-magic-resource", Resource: "great-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid condition only resource",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Conditions: []v1alpha1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "missing-resource",
						}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid from in condition",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}, {
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Conditions: []v1alpha1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "missing-resource", From: []string{"foo"},
						}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "from resource isn't output by task",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Resources: []v1alpha1.PipelineDeclaredResource{{
					Name: "great-resource",
					Type: v1alpha1.PipelineResourceTypeGit,
				}, {
					Name: "wonderful-resource",
					Type: v1alpha1.PipelineResourceTypeImage,
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
					},
				}, {
					Name:    "bar",
					TaskRef: &v1alpha1.TaskRef{Name: "bar-task"},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name: "wow-image", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "not defined parameter variable",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(params.does-not-exist)"),
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "not defined parameter variable with defined",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "foo",
					Type:    v1alpha1.ParamTypeString,
					Default: v1beta1.NewArrayOrString("something"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(params.foo) and $(params.does-not-exist)"),
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid parameter type",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "baz",
					Type:    v1alpha1.ParamType("invalidtype"),
					Default: v1beta1.NewArrayOrString("some", "default"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter mismatching default type",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "baz",
					Type:    v1alpha1.ParamTypeArray,
					Default: v1beta1.NewArrayOrString("astring"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "string parameter mismatching default type",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "baz",
					Type:    v1alpha1.ParamTypeString,
					Default: v1beta1.NewArrayOrString("an", "array"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter used as string",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "baz",
					Type:    v1alpha1.ParamTypeArray,
					Default: v1beta1.NewArrayOrString("an", "array"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(params.baz)"),
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "star array parameter used as string",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "baz",
					Type:    v1alpha1.ParamTypeArray,
					Default: v1beta1.NewArrayOrString("an", "array"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("$(params.baz[*])"),
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter string template not isolated",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "a-param",
					Type:    v1alpha1.ParamTypeArray,
					Default: v1beta1.NewArrayOrString("an", "array"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("first", "value: $(params.baz)", "last"),
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "star array parameter string template not isolated",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Params: []v1alpha1.ParamSpec{{
					Name:    "a-param",
					Type:    v1alpha1.ParamTypeArray,
					Default: v1beta1.NewArrayOrString("an", "array"),
				}},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Params: []v1alpha1.Param{{
						Name:  "a-param",
						Value: *v1beta1.NewArrayOrString("first", "value: $(params.baz[*])", "last"),
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "circular dependency graph between the tasks",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name:     "foo",
					TaskRef:  &v1alpha1.TaskRef{Name: "foo-task"},
					RunAfter: []string{"bar"},
				}, {
					Name:     "bar",
					TaskRef:  &v1alpha1.TaskRef{Name: "bar-task"},
					RunAfter: []string{"foo"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "unused pipeline spec workspaces do not cause an error",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Workspaces: []v1alpha1.PipelineWorkspaceDeclaration{
					{Name: "foo"},
					{Name: "bar"},
				},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "workspace bindings relying on a non-existent pipeline workspace cause an error",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Workspaces: []v1alpha1.PipelineWorkspaceDeclaration{
					{Name: "foo"},
				},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
					Workspaces: []v1alpha1.WorkspacePipelineTaskBinding{{
						Name: "taskWorkspaceName", Workspace: "pipelineWorkspaceName",
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "multiple workspaces sharing the same name are not allowed",
		p: &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha1.PipelineSpec{
				Workspaces: []v1alpha1.PipelineWorkspaceDeclaration{
					{Name: "foo"},
					{Name: "foo"},
				},
				Tasks: []v1alpha1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if (!tt.failureExpected) && (err != nil) {
				t.Errorf("Pipeline.Validate() returned error: %v", err)
			}

			if tt.failureExpected && (err == nil) {
				t.Error("Pipeline.Validate() did not return error, wanted error")
			}
		})
	}
}
