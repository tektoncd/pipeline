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

package v1alpha2_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipeline_Validate(t *testing.T) {
	tests := []struct {
		name            string
		p               *v1alpha2.Pipeline
		failureExpected bool
	}{{
		name: "valid metadata",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{Name: "foo", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid resource declarations and usage",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Resources: []v1alpha2.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1alpha2.PipelineResourceTypeGit,
				}, {
					Name: "wonderful-resource", Type: v1alpha2.PipelineResourceTypeImage,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
						Outputs: []v1alpha2.PipelineTaskOutputResource{{
							Name: "some-imagee", Resource: "wonderful-resource",
						}},
					},
					Conditions: []v1alpha2.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha2.PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
					}},
				}, {
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "some-imagee", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					},
					Conditions: []v1alpha2.PipelineTaskCondition{{
						ConditionRef: "some-condition-2",
						Resources: []v1alpha2.PipelineTaskInputResource{{
							Name: "some-image", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "period in name",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipe.line"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{Name: "foo", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline name too long",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "asdf123456789012345678901234567890123456789012345678901234567890"},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec missing taskref and taskspec",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{
					{Name: "foo"},
				},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec with taskref and taskspec",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{
					{
						Name:    "foo",
						TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
						TaskSpec: &v1alpha2.TaskSpec{
							Steps: []v1alpha2.Step{{
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
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{
					{
						Name:     "foo",
						TaskSpec: &v1alpha2.TaskSpec{},
					},
				},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec valid taskspec",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{
					{
						Name: "foo",
						TaskSpec: &v1alpha2.TaskSpec{
							Steps: []v1alpha2.Step{{
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
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{
					{Name: "foo", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"}},
					{Name: "foo", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"}},
				},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec empty task name",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{Name: "", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid task name",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{Name: "_foo", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "pipeline spec invalid taskref name",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{Name: "foo", TaskRef: &v1alpha2.TaskRef{Name: "_foo-task"}}},
			},
		},
		failureExpected: true,
	}, {
		name: "valid condition only resource",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Resources: []v1alpha2.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1alpha2.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Conditions: []v1alpha2.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha2.PipelineTaskInputResource{{
							Name: "sowe-workspace", Resource: "great-resource",
						}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid parameter variables",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "baz", Type: v1alpha2.ParamTypeString,
				}, {
					Name: "foo-is-baz", Type: v1alpha2.ParamTypeString,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Params: []v1alpha2.Param{{
						Name: "a-param", Value: v1alpha2.ArrayOrString{StringVal: "$(baz) and $(foo-is-baz)"},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "valid array parameter variables",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "baz", Type: v1alpha2.ParamTypeArray, Default: &v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeArray, ArrayVal: []string{"some", "default"}},
				}, {
					Name: "foo-is-baz", Type: v1alpha2.ParamTypeArray,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Params: []v1alpha2.Param{{
						Name: "a-param", Value: v1alpha2.ArrayOrString{ArrayVal: []string{"$(baz)", "and", "$(foo-is-)"}},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "pipeline parameter nested in task parameter",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "baz", Type: v1alpha2.ParamTypeString,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Params: []v1alpha2.Param{{
						Name: "a-param", Value: v1alpha2.ArrayOrString{StringVal: "$(input.workspace.$(baz))"},
					}},
				}},
			},
		},
		failureExpected: false,
	}, {
		name: "from is on first task",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Resources: []v1alpha2.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1alpha2.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "from task doesnt exist",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Resources: []v1alpha2.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1alpha2.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name: "baz", TaskRef: &v1alpha2.TaskRef{Name: "baz-task"},
				}, {
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "output resources missing from declaration",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Resources: []v1alpha2.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1alpha2.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "great-resource",
						}},
						Outputs: []v1alpha2.PipelineTaskOutputResource{{
							Name: "the-magic-resource", Resource: "missing-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "input resources missing from declaration",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Resources: []v1alpha2.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1alpha2.PipelineResourceTypeGit,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "the-resource", Resource: "missing-resource",
						}},
						Outputs: []v1alpha2.PipelineTaskOutputResource{{
							Name: "the-magic-resource", Resource: "great-resource",
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid condition only resource",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Conditions: []v1alpha2.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha2.PipelineTaskInputResource{{
							Name: "sowe-workspace", Resource: "missing-resource",
						}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid from in condition",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{
					Name: "foo", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
				}, {
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Conditions: []v1alpha2.PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []v1alpha2.PipelineTaskInputResource{{
							Name: "sowe-workspace", Resource: "missing-resource", From: []string{"foo"},
						}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "from resource isn't output by task",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Resources: []v1alpha2.PipelineDeclaredResource{{
					Name: "great-resource", Type: v1alpha2.PipelineResourceTypeGit,
				}, {
					Name: "wonderful-resource", Type: v1alpha2.PipelineResourceTypeImage,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "some-resource", Resource: "great-resource",
						}},
					},
				}, {
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Resources: &v1alpha2.PipelineTaskResources{
						Inputs: []v1alpha2.PipelineTaskInputResource{{
							Name: "wow-image", Resource: "wonderful-resource", From: []string{"bar"},
						}},
					},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "not defined parameter variable",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Params: []v1alpha2.Param{{
						Name: "a-param", Value: v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeString, StringVal: "$(params.does-not-exist)"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "not defined parameter variable with defined",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "foo", Type: v1alpha2.ParamTypeString,
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
					Params: []v1alpha2.Param{{
						Name: "a-param", Value: v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeString, StringVal: "$(params.foo) and $(params.does-not-exist)"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid parameter type",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "foo", Type: "invalidtype",
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter mismatching default type",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "foo", Type: v1alpha2.ParamTypeArray, Default: &v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeString, StringVal: "astring"},
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "string parameter mismatching default type",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "foo", Type: v1alpha2.ParamTypeString, Default: &v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "foo",
					TaskRef: &v1alpha2.TaskRef{Name: "foo-task"},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter used as string",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "baz", Type: v1alpha2.ParamTypeString, Default: &v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Params: []v1alpha2.Param{{
						Name: "a-param", Value: v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeString, StringVal: "$(params.baz)"},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "array parameter string template not isolated",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Params: []v1alpha2.ParamSpec{{
					Name: "baz", Type: v1alpha2.ParamTypeString, Default: &v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
				}},
				Tasks: []v1alpha2.PipelineTask{{
					Name:    "bar",
					TaskRef: &v1alpha2.TaskRef{Name: "bar-task"},
					Params: []v1alpha2.Param{{
						Name: "a-param", Value: v1alpha2.ArrayOrString{Type: v1alpha2.ParamTypeArray, ArrayVal: []string{"value: $(params.baz)", "last"}},
					}},
				}},
			},
		},
		failureExpected: true,
	}, {
		name: "invalid dependency graph between the tasks",
		p: &v1alpha2.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: v1alpha2.PipelineSpec{
				Tasks: []v1alpha2.PipelineTask{{
					Name: "foo", TaskRef: &v1alpha2.TaskRef{Name: "foo-task"}, RunAfter: []string{"bar"},
				}, {
					Name: "bar", TaskRef: &v1alpha2.TaskRef{Name: "bar-task"}, RunAfter: []string{"foo"},
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
