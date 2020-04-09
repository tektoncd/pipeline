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

package v1beta1_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipeline_Validate(t *testing.T) {
	tests := []struct {
		name            string
		p               *v1beta1.Pipeline
		failureExpected bool
	}{{
		name: "valid metadata",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid resource declarations and usage",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}, {
					Name: "wonderful-resource", Type: v1beta1.PipelineResourceTypeImage,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name: "some-imagee", Resource: "wonderful-resource",
						}},
					},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1beta1.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
					}},
				}, {
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "some-imagee", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "some-condition-2",
						Resources: []v1beta1.PipelineTaskInputResource{{
							Name: "some-image", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "period in name",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipe.line"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline name too long",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "asdf123456789012345678901234567890123456789012345678901234567890"},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec missing",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec missing taskref and taskspec",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{Name: "foo"},
				},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec with taskref and taskspec",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name:    "foo",
						TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
						TaskSpec: &v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Container: corev1.Container{Name: "foo", Image: "bar"},
							}},
						},
					},
				},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid taskspec",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name:     "foo",
						TaskSpec: &v1beta1.TaskSpec{},
					},
				},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec valid taskspec",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "foo",
						TaskSpec: &v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{
								Container: corev1.Container{Name: "foo", Image: "bar"},
							}},
						},
					},
				},
			},
		},
		failureExpected: false,
	}, {
		name: "pipeline spec invalid (duplicate tasks)",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}},
					{Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}},
				},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec empty task name",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{Name: "", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid task name",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{Name: "_foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid task name 2",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{Name: "fooTask", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid taskref name",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "_foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "valid condition only resource",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1beta1.PipelineTaskInputResource{{
							Name: "sowe-workspace", Resource: "great-resource",
						}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid parameter variables",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeString,
				}, {
					Name: "foo-is-baz", Type: v1beta1.ParamTypeString,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{StringVal: "$(baz) and $(foo-is-baz)"},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid array parameter variables",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeArray, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"some", "default"}},
				}, {
					Name: "foo-is-baz", Type: v1beta1.ParamTypeArray,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{ArrayVal: []string{"$(baz)", "and", "$(foo-is-baz)"}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid star array parameter variables",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeArray, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"some", "default"}},
				}, {
					Name: "foo-is-baz", Type: v1beta1.ParamTypeArray,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{ArrayVal: []string{"$(baz[*])", "and", "$(foo-is-baz[*])"}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "pipeline parameter nested in task parameter",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeString,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{StringVal: "$(input.workspace.$(baz))"},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "from is on first task",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "from task doesnt exist",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name: "baz", TaskRef: &v1beta1.TaskRef{Name: "baz-task"},
				}, {
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "duplicate resource declaration",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "duplicate-resource", Type: v1beta1.PipelineResourceTypeGit,
				}, {
					Name: "duplicate-resource", Type: v1beta1.PipelineResourceTypeGit,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "duplicate-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "output resources missing from declaration",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name: "the-magic-resource", Resource: "missing-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "input resources missing from declaration",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "missing-resource",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name: "the-magic-resource", Resource: "great-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid condition only resource",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1beta1.PipelineTaskInputResource{{
							Name: "sowe-workspace", Resource: "missing-resource",
						}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid from in condition",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}, {
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1beta1.PipelineTaskInputResource{{
							Name: "sowe-workspace", Resource: "missing-resource", From: []string{"foo"},
						}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "from resource isn't output by task",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}, {
					Name: "wonderful-resource", Type: v1beta1.PipelineResourceTypeImage,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "some-resource", Resource: "great-resource",
						}},
					},
				}, {
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "wow-image", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "not defined parameter variable",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "$(params.does-not-exist)"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "not defined parameter variable with defined",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "foo", Type: v1beta1.ParamTypeString,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "$(params.foo) and $(params.does-not-exist)"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid parameter type",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "foo", Type: "invalidtype",
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter mismatching default type",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "foo", Type: v1beta1.ParamTypeArray, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "astring"},
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "string parameter mismatching default type",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "foo", Type: v1beta1.ParamTypeString, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter used as string",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeString, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "$(params.baz)"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "star array parameter used as string",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeString, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "$(params.baz[*])"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter string template not isolated",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeString, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"value: $(params.baz)", "last"}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "star array parameter string template not isolated",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "baz", Type: v1beta1.ParamTypeString, Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1beta1.TaskRef{Name: "bar-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeArray, ArrayVal: []string{"value: $(params.baz[*])", "last"}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid dependency graph between the tasks",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo-task"}, RunAfter: []string{"bar"},
				}, {
					Name: "bar", TaskRef: &v1beta1.TaskRef{Name: "bar-task"}, RunAfter: []string{"foo"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "unused pipeline spec workspaces do not cause an error",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo"},
				}},
				Workspaces: []v1beta1.WorkspacePipelineDeclaration{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "workspace bindings relying on a non-existent pipeline workspace cause an error",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "foo", TaskRef: &v1beta1.TaskRef{Name: "foo"},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: "pipelineWorkspaceName",
					}},
				}},
				Workspaces: []v1beta1.WorkspacePipelineDeclaration{{
					Name: "foo",
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "multiple workspaces sharing the same name are not allowed",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"}, Spec: v1beta1.PipelineSpec{
				Workspaces: []v1beta1.WorkspacePipelineDeclaration{{
					Name: "foo",
				}, {
					Name: "foo",
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "task params results malformed variable substitution expression",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "name"}, Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "a-task", TaskRef: &v1beta1.TaskRef{Name: "a-task"},
				}, {
					Name: "b-task", TaskRef: &v1beta1.TaskRef{Name: "b-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "$(tasks.a-task.resultTypo.bResult)"}}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "not defined parameter variable with defined",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "foo", Type: v1beta1.ParamTypeString,
				}},
				Tasks: []v1beta1.PipelineTask{{
					TaskSpec: &v1beta1.TaskSpec{
						Results: []v1beta1.TaskResult{{
							Name: "output",
						}},
						Steps: []v1beta1.Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
					Name: "a-task",
				}, {
					Name:    "foo",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Params: []v1beta1.Param{{
						Name: "a-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "$(params.foo) and $(tasks.a-task.results.output)"},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid pipeline with one final task",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "non-final-task",
					TaskSpec: &v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name: "final-task",
					TaskSpec: &v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "invalid pipeline without any non final task but at least one final task",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{}},
				Finally: []v1beta1.PipelineTask{{
					Name: "final-task",
					TaskSpec: &v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid pipeline with empty finally section",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "non-final-task",
					TaskSpec: &v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
				Finally: []v1beta1.PipelineTask{},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid pipeline with duplicate final tasks",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "non-final-task",
					TaskSpec: &v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task",
					TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				}, {
					Name:    "final-task",
					TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid pipeline with final task same as non final task",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "non-final-and-final-task",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "non-final-and-final-task",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "final task missing tasfref and taskspec",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "non-final-task",
					TaskSpec: &v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name: "final-task",
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "undefined parameter variable in final task",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "foo", Type: v1beta1.ParamTypeString,
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Params: []v1beta1.Param{{
						Name: "final-param", Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "$(params.foo) and $(params.does-not-exist)"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid pipeline with final task specifying runAfter",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:     "final-task",
					TaskRef:  &v1beta1.TaskRef{Name: "foo-task"},
					RunAfter: []string{"non-final-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid pipeline with final task specifying conditions",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "some-condition",
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid pipeline with final task output resources referring to other final task input",
		p: &v1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1beta1.PipelineSpec{
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1beta1.PipelineResourceTypeGit,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "final-input-1", Resource: "great-resource",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name: "final-output-2", Resource: "great-resource",
						}},
					},
				}, {
					Name:    "final-task-2",
					TaskRef: &v1beta1.TaskRef{Name: "foo-task"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name: "final-input-2", Resource: "great-resource", From: []string{"final-task-1"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}}
	for _, tt := range tests {
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
