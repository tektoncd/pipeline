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

package v1beta1

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipeline_Validate_Success(t *testing.T) {
	tests := []struct {
		name string
		p    *Pipeline
	}{{
		name: "valid metadata",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}}},
			},
		},
	}, {
		name: "valid pipeline with params, resources, workspaces, task results, and pipeline results",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Description: "this is a valid pipeline with all possible fields initialized",
				Resources: []PipelineDeclaredResource{{
					Name:     "app-repo",
					Type:     "git",
					Optional: false,
				}, {
					Name:     "app-image",
					Type:     "git",
					Optional: false,
				}},
				Tasks: []PipelineTask{{
					Name:    "my-task",
					TaskRef: &TaskRef{Name: "foo-task"},
					Retries: 5,
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:     "task-app-repo",
							Resource: "app-repo",
						}},
						Outputs: []PipelineTaskOutputResource{{
							Name:     "task-app-image",
							Resource: "app-image",
						}},
					},
					Params: []Param{{
						Name:  "param1",
						Value: ArrayOrString{},
					}},
					Workspaces: []WorkspacePipelineTaskBinding{{
						Name:      "task-shared-workspace",
						Workspace: "shared-workspace",
					}},
					Timeout: &metav1.Duration{Duration: 5 * time.Minute},
				}},
				Params: []ParamSpec{{
					Name:        "param1",
					Type:        ParamType("string"),
					Description: "this is my param",
					Default: &ArrayOrString{
						Type:      ParamType("string"),
						StringVal: "pipeline-default",
					},
				}},
				Workspaces: []PipelineWorkspaceDeclaration{{
					Name:        "shared-workspace",
					Description: "this is my shared workspace",
				}},
				Results: []PipelineResult{{
					Name:        "pipeline-result",
					Description: "this is my pipeline result",
					Value:       "pipeline-result-default",
				}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if err != nil {
				t.Errorf("Pipeline.Validate() returned error for valid Pipeline: %s: %v", tt.name, err)
			}
		})
	}
}

func TestPipeline_Validate_Failure(t *testing.T) {
	tests := []struct {
		name string
		p    *Pipeline
	}{{
		name: "period in name",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipe.line"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}}},
			},
		},
	}, {
		name: "pipeline name too long",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "asdf123456789012345678901234567890123456789012345678901234567890"},
		},
	}, {
		name: "pipeline spec missing",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if err == nil {
				t.Errorf("Pipeline.Validate() did not return error for invalid pipeline: %s", tt.name)
			}
		})
	}
}

func TestPipelineSpec_Validate_Failure(t *testing.T) {
	tests := []struct {
		name string
		ps   *PipelineSpec
	}{{
		name: "invalid pipeline with one pipeline task having taskRef and taskSpec both",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				TaskSpec: &TaskSpec{
					Steps: []Step{{
						Container: corev1.Container{Name: "foo", Image: "bar"},
					}},
				},
			}},
		},
	}, {
		name: "invalid pipeline with pipeline task having reference to resources which does not exist",
		ps: &PipelineSpec{
			Resources: []PipelineDeclaredResource{{
				Name: "great-resource", Type: PipelineResourceTypeGit,
			}, {
				Name: "wonderful-resource", Type: PipelineResourceTypeImage,
			}},
			Tasks: []PipelineTask{{
				Name:    "bar",
				TaskRef: &TaskRef{Name: "bar-task"},
				Resources: &PipelineTaskResources{
					Inputs: []PipelineTaskInputResource{{
						Name: "some-workspace", Resource: "missing-great-resource",
					}},
					Outputs: []PipelineTaskOutputResource{{
						Name: "some-imagee", Resource: "missing-wonderful-resource",
					}},
				},
				Conditions: []PipelineTaskCondition{{
					ConditionRef: "some-condition",
					Resources: []PipelineTaskInputResource{{
						Name: "some-workspace", Resource: "missing-great-resource",
					}},
				}},
			}, {
				Name:    "foo",
				TaskRef: &TaskRef{Name: "foo-task"},
				Resources: &PipelineTaskResources{
					Inputs: []PipelineTaskInputResource{{
						Name: "some-image", Resource: "wonderful-resource",
					}},
				},
				Conditions: []PipelineTaskCondition{{
					ConditionRef: "some-condition-2",
					Resources: []PipelineTaskInputResource{{
						Name: "some-image", Resource: "wonderful-resource",
					}},
				}},
			}},
		},
	}, {
		name: "invalid pipeline spec - from referring to a pipeline task which does not exist",
		ps: &PipelineSpec{
			Tasks: []PipelineTask{{
				Name: "baz", TaskRef: &TaskRef{Name: "baz-task"},
			}, {
				Name:    "foo",
				TaskRef: &TaskRef{Name: "foo-task"},
				Resources: &PipelineTaskResources{
					Inputs: []PipelineTaskInputResource{{
						Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
					}},
				},
			}},
		},
	}, {
		name: "invalid pipeline spec with DAG having cyclic dependency",
		ps: &PipelineSpec{
			Tasks: []PipelineTask{{
				Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}, RunAfter: []string{"bar"},
			}, {
				Name: "bar", TaskRef: &TaskRef{Name: "bar-task"}, RunAfter: []string{"foo"},
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ps.Validate(context.Background())
			if err == nil {
				t.Errorf("PipelineSpec.Validate() did not return error for invalid pipelineSpec: %s", tt.name)
			}
		})
	}
}

func TestValidatePipelineTasks_Success(t *testing.T) {
	tests := []struct {
		name  string
		tasks []PipelineTask
	}{{
		name: "pipeline task with valid taskref name",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "example.com/my-foo-task"},
		}},
	}, {
		name: "pipeline task with valid taskspec",
		tasks: []PipelineTask{{
			Name: "foo",
			TaskSpec: &TaskSpec{
				Steps: []Step{{
					Container: corev1.Container{Name: "foo", Image: "bar"},
				}},
			},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineTasks(context.Background(), tt.tasks, []PipelineTask{})
			if err != nil {
				t.Errorf("Pipeline.validatePipelineTasks() returned error for valid pipeline tasks: %s: %v", tt.name, err)
			}
		})
	}
}

func TestValidatePipelineTasks_Failure(t *testing.T) {
	tests := []struct {
		name  string
		tasks []PipelineTask
	}{{
		name: "pipeline task missing taskref and taskspec",
		tasks: []PipelineTask{{
			Name: "foo",
		}},
	}, {
		name: "pipeline task with both taskref and taskspec",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			TaskSpec: &TaskSpec{
				Steps: []Step{{
					Container: corev1.Container{Name: "foo", Image: "bar"},
				}},
			},
		}},
	}, {
		name: "pipeline task with invalid taskspec",
		tasks: []PipelineTask{{
			Name:     "foo",
			TaskSpec: &TaskSpec{},
		}},
	}, {
		name: "pipeline tasks invalid (duplicate tasks)",
		tasks: []PipelineTask{
			{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}},
			{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}},
		},
	}, {
		name:  "pipeline task with empty task name",
		tasks: []PipelineTask{{Name: "", TaskRef: &TaskRef{Name: "foo-task"}}},
	}, {
		name:  "pipeline task with invalid task name",
		tasks: []PipelineTask{{Name: "_foo", TaskRef: &TaskRef{Name: "foo-task"}}},
	}, {
		name:  "pipeline task with invalid task name (camel case)",
		tasks: []PipelineTask{{Name: "fooTask", TaskRef: &TaskRef{Name: "foo-task"}}},
	}, {
		name:  "pipeline task with invalid taskref name",
		tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "_foo-task"}}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineTasks(context.Background(), tt.tasks, []PipelineTask{})
			if err == nil {
				t.Error("Pipeline.validatePipelineTasks() did not return error for invalid pipeline tasks:", tt.name)
			}
		})
	}
}

func TestValidateFrom_Success(t *testing.T) {
	desc := "valid pipeline task - from resource referring to valid output resource of the pipeline task"
	tasks := []PipelineTask{{
		Name:    "bar",
		TaskRef: &TaskRef{Name: "bar-task"},
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{
				Name: "some-resource", Resource: "some-resource",
			}},
			Outputs: []PipelineTaskOutputResource{{
				Name: "output-resource", Resource: "output-resource",
			}},
		},
	}, {
		Name:    "foo",
		TaskRef: &TaskRef{Name: "foo-task"},
		Resources: &PipelineTaskResources{
			Inputs: []PipelineTaskInputResource{{
				Name: "wow-image", Resource: "output-resource", From: []string{"bar"},
			}},
		},
	}}
	t.Run(desc, func(t *testing.T) {
		err := validateFrom(tasks)
		if err != nil {
			t.Errorf("Pipeline.validateFrom() returned error for: %s: %v", desc, err)
		}
	})
}

func TestValidateFrom_Failure(t *testing.T) {
	tests := []struct {
		name  string
		tasks []PipelineTask
	}{{
		name: "invalid pipeline task - from in a pipeline with single pipeline task",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
				}},
			},
		},
		},
	}, {
		name: "invalid pipeline task - from referencing pipeline task which does not exist",
		tasks: []PipelineTask{{
			Name: "baz", TaskRef: &TaskRef{Name: "baz-task"},
		}, {
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "the-resource", Resource: "great-resource", From: []string{"bar"},
				}},
			},
		}},
	}, {
		name: "invalid pipeline task - pipeline task condition resource does not exist",
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo-task"},
		}, {
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Conditions: []PipelineTaskCondition{{
				ConditionRef: "some-condition",
				Resources: []PipelineTaskInputResource{{
					Name: "some-workspace", Resource: "missing-resource", From: []string{"foo"},
				}},
			}},
		}},
	}, {
		name: "invalid pipeline task - from resource referring to a pipeline task which has no output",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "some-resource", Resource: "great-resource",
				}},
			},
		}, {
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "wow-image", Resource: "wonderful-resource", From: []string{"bar"},
				}},
			},
		}},
	}, {
		name: "invalid pipeline task - from resource referring to input resource of the pipeline task instead of output",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "some-resource", Resource: "great-resource",
				}},
				Outputs: []PipelineTaskOutputResource{{
					Name: "output-resource", Resource: "great-output-resource",
				}},
			},
		}, {
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "wow-image", Resource: "some-resource", From: []string{"bar"},
				}},
			},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFrom(tt.tasks)
			if err == nil {
				t.Error("Pipeline.validateFrom() did not return error for invalid pipeline task resources: ", tt.name)
			}
		})
	}
}

func TestValidateDeclaredResources_Success(t *testing.T) {
	tests := []struct {
		name      string
		resources []PipelineDeclaredResource
		tasks     []PipelineTask
	}{{
		name: "valid resource declarations and usage",
		resources: []PipelineDeclaredResource{{
			Name: "great-resource", Type: PipelineResourceTypeGit,
		}, {
			Name: "wonderful-resource", Type: PipelineResourceTypeImage,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "some-workspace", Resource: "great-resource",
				}},
				Outputs: []PipelineTaskOutputResource{{
					Name: "some-imagee", Resource: "wonderful-resource",
				}},
			},
			Conditions: []PipelineTaskCondition{{
				ConditionRef: "some-condition",
				Resources: []PipelineTaskInputResource{{
					Name: "some-workspace", Resource: "great-resource",
				}},
			}},
		}, {
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "some-image", Resource: "wonderful-resource", From: []string{"bar"},
				}},
			},
			Conditions: []PipelineTaskCondition{{
				ConditionRef: "some-condition-2",
				Resources: []PipelineTaskInputResource{{
					Name: "some-image", Resource: "wonderful-resource", From: []string{"bar"},
				}},
			}},
		}},
	}, {
		name: "valid resource declaration with single reference in the pipeline task condition",
		resources: []PipelineDeclaredResource{{
			Name: "great-resource", Type: PipelineResourceTypeGit,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Conditions: []PipelineTaskCondition{{
				ConditionRef: "some-condition",
				Resources: []PipelineTaskInputResource{{
					Name: "some-workspace", Resource: "great-resource",
				}},
			}},
		}},
	}, {
		name: "valid resource declarations with extra resources, not used in any pipeline task",
		resources: []PipelineDeclaredResource{{
			Name: "great-resource", Type: PipelineResourceTypeGit,
		}, {
			Name: "awesome-resource", Type: PipelineResourceTypeImage,
		}, {
			Name: "yet-another-great-resource", Type: PipelineResourceTypeGit,
		}, {
			Name: "yet-another-awesome-resource", Type: PipelineResourceTypeImage,
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "the-resource", Resource: "great-resource",
				}},
				Outputs: []PipelineTaskOutputResource{{
					Name: "the-awesome-resource", Resource: "awesome-resource",
				}},
			},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeclaredResources(tt.resources, tt.tasks, []PipelineTask{})
			if err != nil {
				t.Errorf("Pipeline.validateDeclaredResources() returned error for valid resource declarations: %s: %v", tt.name, err)
			}
		})
	}
}

func TestValidateDeclaredResources_Failure(t *testing.T) {
	tests := []struct {
		name      string
		resources []PipelineDeclaredResource
		tasks     []PipelineTask
	}{{
		name: "duplicate resource declaration - resource declarations must be unique",
		resources: []PipelineDeclaredResource{{
			Name: "duplicate-resource", Type: PipelineResourceTypeGit,
		}, {
			Name: "duplicate-resource", Type: PipelineResourceTypeGit,
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "the-resource", Resource: "duplicate-resource",
				}},
			},
		}},
	}, {
		name: "output resource is missing from resource declarations",
		resources: []PipelineDeclaredResource{{
			Name: "great-resource", Type: PipelineResourceTypeGit,
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "the-resource", Resource: "great-resource",
				}},
				Outputs: []PipelineTaskOutputResource{{
					Name: "the-magic-resource", Resource: "missing-resource",
				}},
			},
		}},
	}, {
		name: "input resource is missing from resource declarations",
		resources: []PipelineDeclaredResource{{
			Name: "great-resource", Type: PipelineResourceTypeGit,
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "the-resource", Resource: "missing-resource",
				}},
				Outputs: []PipelineTaskOutputResource{{
					Name: "the-magic-resource", Resource: "great-resource",
				}},
			},
		}},
	}, {
		name: "invalid condition only resource -" +
			" pipeline task condition referring to a resource which is missing from resource declarations",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Conditions: []PipelineTaskCondition{{
				ConditionRef: "some-condition",
				Resources: []PipelineTaskInputResource{{
					Name: "some-workspace", Resource: "missing-resource",
				}},
			}},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeclaredResources(tt.resources, tt.tasks, []PipelineTask{})
			if err == nil {
				t.Errorf("Pipeline.validateDeclaredResources() did not return error for invalid resource declarations: %s", tt.name)
			}
		})
	}
}

func TestValidateGraph_Success(t *testing.T) {
	desc := "valid dependency graph with multiple tasks"
	tasks := []PipelineTask{{
		Name: "foo", TaskRef: &TaskRef{Name: "foo-task"},
	}, {
		Name: "bar", TaskRef: &TaskRef{Name: "bar-task"},
	}, {
		Name: "foo1", TaskRef: &TaskRef{Name: "foo-task"}, RunAfter: []string{"foo"},
	}, {
		Name: "bar1", TaskRef: &TaskRef{Name: "bar-task"}, RunAfter: []string{"bar"},
	}, {
		Name: "foo-bar", TaskRef: &TaskRef{Name: "bar-task"}, RunAfter: []string{"foo1", "bar1"},
	}}
	t.Run(desc, func(t *testing.T) {
		err := validateGraph(tasks)
		if err != nil {
			t.Errorf("Pipeline.validateGraph() returned error for valid DAG of pipeline tasks: %s: %v", desc, err)
		}
	})
}

func TestValidateGraph_Failure(t *testing.T) {
	desc := "invalid dependency graph between the tasks with cyclic dependency"
	tasks := []PipelineTask{{
		Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}, RunAfter: []string{"bar"},
	}, {
		Name: "bar", TaskRef: &TaskRef{Name: "bar-task"}, RunAfter: []string{"foo"},
	}}
	t.Run(desc, func(t *testing.T) {
		err := validateGraph(tasks)
		if err == nil {
			t.Error("Pipeline.validateGraph() did not return error for invalid DAG of pipeline tasks:", desc)

		}
	})
}

func TestValidateParamResults_Success(t *testing.T) {
	desc := "valid pipeline task referencing task result along with parameter variable"
	tasks := []PipelineTask{{
		TaskSpec: &TaskSpec{
			Results: []TaskResult{{
				Name: "output",
			}},
			Steps: []Step{{
				Container: corev1.Container{Name: "foo", Image: "bar"},
			}},
		},
		Name: "a-task",
	}, {
		Name:    "foo",
		TaskRef: &TaskRef{Name: "foo-task"},
		Params: []Param{{
			Name: "a-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.foo) and $(tasks.a-task.results.output)"},
		}},
	}}
	t.Run(desc, func(t *testing.T) {
		err := validateParamResults(tasks)
		if err != nil {
			t.Errorf("Pipeline.validateParamResults() returned error for valid pipeline: %s: %v", desc, err)
		}
	})
}

func TestValidateParamResults_Failure(t *testing.T) {
	desc := "invalid pipeline task referencing task results with malformed variable substitution expression"
	tasks := []PipelineTask{{
		Name: "a-task", TaskRef: &TaskRef{Name: "a-task"},
	}, {
		Name: "b-task", TaskRef: &TaskRef{Name: "b-task"},
		Params: []Param{{
			Name: "a-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(tasks.a-task.resultTypo.bResult)"}}},
	}}
	t.Run(desc, func(t *testing.T) {
		err := validateParamResults(tasks)
		if err == nil {
			t.Errorf("Pipeline.validateParamResults() did not return error for invalid pipeline: %s", desc)
		}
	})
}

func TestValidatePipelineResults_Success(t *testing.T) {
	desc := "valid pipeline with valid pipeline results syntax"
	results := []PipelineResult{{
		Name:        "my-pipeline-result",
		Description: "this is my pipeline result",
		Value:       "$(tasks.a-task.results.output)",
	}}
	t.Run(desc, func(t *testing.T) {
		err := validatePipelineResults(results)
		if err != nil {
			t.Errorf("Pipeline.validatePipelineResults() returned error for valid pipeline: %s: %v", desc, err)
		}
	})
}

func TestValidatePipelineResults_Failure(t *testing.T) {
	desc := "invalid pipeline result reference"
	results := []PipelineResult{{
		Name:        "my-pipeline-result",
		Description: "this is my pipeline result",
		Value:       "$(tasks.a-task.results.output.output)",
	}}
	t.Run(desc, func(t *testing.T) {
		err := validatePipelineResults(results)
		if err == nil {
			t.Errorf("Pipeline.validatePipelineResults() did not return for invalid pipeline: %s", desc)
		}
	})
}

func TestValidatePipelineParameterVariables_Success(t *testing.T) {
	tests := []struct {
		name   string
		params []ParamSpec
		tasks  []PipelineTask
	}{{
		name: "valid string parameter variables",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString,
		}, {
			Name: "foo-is-baz", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{StringVal: "$(baz) and $(foo-is-baz)"},
			}},
		}},
	}, {
		name: "valid array parameter variables",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray, Default: &ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"some", "default"}},
		}, {
			Name: "foo-is-baz", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{ArrayVal: []string{"$(baz)", "and", "$(foo-is-baz)"}},
			}},
		}},
	}, {
		name: "valid star array parameter variables",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray, Default: &ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"some", "default"}},
		}, {
			Name: "foo-is-baz", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{ArrayVal: []string{"$(baz[*])", "and", "$(foo-is-baz[*])"}},
			}},
		}},
	}, {
		name: "pipeline parameter nested in task parameter",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{StringVal: "$(input.workspace.$(baz))"},
			}},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineParameterVariables(tt.tasks, tt.params)
			if err != nil {
				t.Errorf("Pipeline.validatePipelineParameterVariables() returned error for valid pipeline parameters: %s: %v", tt.name, err)
			}
		})
	}
}

func TestValidatePipelineParameterVariables_Failure(t *testing.T) {
	tests := []struct {
		name   string
		params []ParamSpec
		tasks  []PipelineTask
	}{{
		name: "invalid pipeline task with a parameter which is missing from the param declarations",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.does-not-exist)"},
			}},
		}},
	}, {
		name: "invalid pipeline task with a parameter combined with missing param from the param declarations",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.foo) and $(params.does-not-exist)"},
			}},
		}},
	}, {
		name: "invalid pipeline task with two parameters and one of them missing from the param declarations",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.foo)"},
			}, {
				Name: "b-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.does-not-exist)"},
			}},
		}},
	}, {
		name: "invalid parameter type",
		params: []ParamSpec{{
			Name: "foo", Type: "invalidtype",
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
	}, {
		name: "array parameter mismatching default type",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeArray, Default: &ArrayOrString{Type: ParamTypeString, StringVal: "astring"},
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
	}, {
		name: "string parameter mismatching default type",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString, Default: &ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
	}, {
		name: "array parameter used as string",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.baz)"},
			}},
		}},
	}, {
		name: "star array parameter used as string",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.baz[*])"},
			}},
		}},
	}, {
		name: "array parameter string template not isolated",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"value: $(params.baz)", "last"}},
			}},
		}},
	}, {
		name: "star array parameter string template not isolated",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"value: $(params.baz[*])", "last"}},
			}},
		}},
	}, {
		name: "multiple string parameters with the same name",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString,
		}, {
			Name: "baz", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
	}, {
		name: "multiple array parameters with the same name",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray,
		}, {
			Name: "baz", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
	}, {
		name: "multiple different type parameters with the same name",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray,
		}, {
			Name: "baz", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineParameterVariables(tt.tasks, tt.params)
			if err == nil {
				t.Errorf("Pipeline.validatePipelineParameterVariables() did not return error for invalid pipeline parameters: %s", tt.name)
			}
		})
	}
}

func TestValidatePipelineWorkspaces_Success(t *testing.T) {
	desc := "unused pipeline spec workspaces do not cause an error"
	workspaces := []PipelineWorkspaceDeclaration{{
		Name: "foo",
	}, {
		Name: "bar",
	}}
	tasks := []PipelineTask{{
		Name: "foo", TaskRef: &TaskRef{Name: "foo"},
	}}
	t.Run(desc, func(t *testing.T) {
		err := validatePipelineWorkspaces(workspaces, tasks, []PipelineTask{})
		if err != nil {
			t.Errorf("Pipeline.validatePipelineWorkspaces() returned error for valid pipeline workspaces: %s: %v", desc, err)
		}
	})
}

func TestValidatePipelineWorkspaces_Failure(t *testing.T) {
	tests := []struct {
		name       string
		workspaces []PipelineWorkspaceDeclaration
		tasks      []PipelineTask
	}{{
		name: "workspace bindings relying on a non-existent pipeline workspace cause an error",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "foo",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
			Workspaces: []WorkspacePipelineTaskBinding{{
				Name:      "taskWorkspaceName",
				Workspace: "pipelineWorkspaceName",
			}},
		}},
	}, {
		name: "multiple workspaces sharing the same name are not allowed",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "foo",
		}, {
			Name: "foo",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
		}},
	}, {
		name: "workspace name must not be empty",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineWorkspaces(tt.workspaces, tt.tasks, []PipelineTask{})
			if err == nil {
				t.Errorf("Pipeline.validatePipelineWorkspaces() did not return error for invalid pipeline workspaces: %s", tt.name)
			}
		})
	}
}

func TestValidatePipelineWithFinalTasks_Success(t *testing.T) {
	tests := []struct {
		name string
		p    *Pipeline
	}{{
		name: "valid pipeline with final tasks",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &TaskRef{Name: "final-task"},
				}, {
					Name: "final-task-2",
					TaskSpec: &TaskSpec{
						Steps: []Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
			},
		},
	}, {
		name: "valid pipeline with resource declarations and their valid usage",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource", Type: PipelineResourceTypeGit,
				}, {
					Name: "wonderful-resource", Type: PipelineResourceTypeImage,
				}},
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "bar-task"},
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
						Outputs: []PipelineTaskOutputResource{{
							Name: "some-image", Resource: "wonderful-resource",
						}},
					},
					Conditions: []PipelineTaskCondition{{
						ConditionRef: "some-condition",
						Resources: []PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
					}},
				}},
				Finally: []PipelineTask{{
					Name:    "foo",
					TaskRef: &TaskRef{Name: "foo-task"},
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name: "some-workspace", Resource: "great-resource",
						}},
						Outputs: []PipelineTaskOutputResource{{
							Name: "some-image", Resource: "wonderful-resource",
						}},
					},
				}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if err != nil {
				t.Errorf("Pipeline.Validate() returned error for valid pipeline with finally: %s: %v", tt.name, err)
			}
		})
	}
}

func TestValidatePipelineWithFinalTasks_Failure(t *testing.T) {
	tests := []struct {
		name string
		p    *Pipeline
	}{{
		name: "invalid pipeline without any non-final task (tasks set to nil) but at least one final task",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: nil,
				Finally: []PipelineTask{{
					Name:    "final-task",
					TaskRef: &TaskRef{Name: "final-task"},
				}},
			},
		},
	}, {
		name: "invalid pipeline without any non-final task (tasks set to empty list of pipeline task) but at least one final task",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{}},
				Finally: []PipelineTask{{
					Name:    "final-task",
					TaskRef: &TaskRef{Name: "final-task"},
				}},
			},
		},
	}, {
		name: "invalid pipeline with valid non-final tasks but empty finally section",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{}},
			},
		},
	}, {
		name: "invalid pipeline with duplicate final tasks",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{
					Name:    "final-task",
					TaskRef: &TaskRef{Name: "final-task"},
				}, {
					Name:    "final-task",
					TaskRef: &TaskRef{Name: "final-task"},
				}},
			},
		},
	}, {
		name: "invalid pipeline with same task name for final and non final task",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name:    "common-task-name",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{
					Name:    "common-task-name",
					TaskRef: &TaskRef{Name: "final-task"},
				}},
			},
		},
	}, {
		name: "final task missing tasfref and taskspec",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{
					Name: "final-task",
				}},
			},
		},
	}, {
		name: "final task with both tasfref and taskspec",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{
					Name:    "final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
					TaskSpec: &TaskSpec{
						Steps: []Step{{
							Container: corev1.Container{Name: "foo", Image: "bar"},
						}},
					},
				}},
			},
		},
	}, {
		name: "extra parameter called final-param provided to final task which is not specified in the Pipeline",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Params: []ParamSpec{{
					Name: "foo", Type: ParamTypeString,
				}},
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{
					Name:    "final-task",
					TaskRef: &TaskRef{Name: "final-task"},
					Params: []Param{{
						Name: "final-param", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(params.foo) and $(params.does-not-exist)"},
					}},
				}},
			},
		},
	}, {
		name: "invalid pipeline with invalid final tasks with runAfter and conditions",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name:    "non-final-task",
					TaskRef: &TaskRef{Name: "non-final-task"},
				}},
				Finally: []PipelineTask{{
					Name:     "final-task-1",
					TaskRef:  &TaskRef{Name: "final-task"},
					RunAfter: []string{"non-final-task"},
				}, {
					Name:    "final-task-2",
					TaskRef: &TaskRef{Name: "final-task"},
					Conditions: []PipelineTaskCondition{{
						ConditionRef: "some-condition",
					}},
				}},
			},
		},
	}, {
		name: "invalid pipeline - workspace bindings in final task relying on a non-existent pipeline workspace",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name: "non-final-task", TaskRef: &TaskRef{Name: "foo"},
				}},
				Finally: []PipelineTask{{
					Name: "final-task", TaskRef: &TaskRef{Name: "foo"},
					Workspaces: []WorkspacePipelineTaskBinding{{
						Name:      "shared-workspace",
						Workspace: "pipeline-shared-workspace",
					}},
				}},
				Workspaces: []WorkspacePipelineDeclaration{{
					Name: "foo",
				}},
			},
		},
	}, {
		name: "invalid pipeline with no tasks under tasks section and empty finally section",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Finally: []PipelineTask{{}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if err == nil {
				t.Errorf("Pipeline.Validate() did not return error for invalid pipeline with finally: %s", tt.name)
			}
		})
	}
}

func TestValidateTasksAndFinallySection_Success(t *testing.T) {
	tests := []struct {
		name string
		ps   *PipelineSpec
	}{{
		name: "pipeline with tasks and final tasks",
		ps: &PipelineSpec{
			Tasks: []PipelineTask{{
				Name: "non-final-task", TaskRef: &TaskRef{Name: "foo"},
			}},
			Finally: []PipelineTask{{
				Name: "final-task", TaskRef: &TaskRef{Name: "foo"},
			}},
		},
	}, {
		name: "valid pipeline with tasks and finally section without any tasks",
		ps: &PipelineSpec{
			Tasks: []PipelineTask{{
				Name: "my-task", TaskRef: &TaskRef{Name: "foo"},
			}},
			Finally: nil,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTasksAndFinallySection(tt.ps)
			if err != nil {
				t.Errorf("Pipeline.ValidateTasksAndFinallySection() returned error for valid pipeline with finally: %s: %v", tt.name, err)
			}
		})
	}
}

func TestValidateTasksAndFinallySection_Failure(t *testing.T) {
	desc := "invalid pipeline with empty tasks and a few final tasks"
	ps := &PipelineSpec{
		Tasks: nil,
		Finally: []PipelineTask{{
			Name: "final-task", TaskRef: &TaskRef{Name: "foo"},
		}},
	}
	t.Run(desc, func(t *testing.T) {
		err := validateTasksAndFinallySection(ps)
		if err == nil {
			t.Errorf("Pipeline.ValidateTasksAndFinallySection() did not return error for invalid pipeline with finally: %s", desc)
		}
	})
}

func TestValidateFinalTasks_Failure(t *testing.T) {
	tests := []struct {
		name       string
		finalTasks []PipelineTask
	}{{
		name: "invalid pipeline with final task specifying runAfter",
		finalTasks: []PipelineTask{{
			Name:     "final-task",
			TaskRef:  &TaskRef{Name: "final-task"},
			RunAfter: []string{"non-final-task"},
		}},
	}, {
		name: "invalid pipeline with final task specifying conditions",
		finalTasks: []PipelineTask{{
			Name:    "final-task",
			TaskRef: &TaskRef{Name: "final-task"},
			Conditions: []PipelineTaskCondition{{
				ConditionRef: "some-condition",
			}},
		}},
	}, {
		name: "invalid pipeline with final task output resources referring to other task input",
		finalTasks: []PipelineTask{{
			Name:    "final-task",
			TaskRef: &TaskRef{Name: "final-task"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					Name: "final-input-2", Resource: "great-resource", From: []string{"task"},
				}},
			},
		}},
	}, {
		name: "invalid pipeline with final tasks having reference to task results",
		finalTasks: []PipelineTask{{
			Name:    "final-task",
			TaskRef: &TaskRef{Name: "final-task"},
			Params: []Param{{
				Name: "param1", Value: ArrayOrString{Type: ParamTypeString, StringVal: "$(tasks.a-task.results.output)"},
			}},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFinalTasks(tt.finalTasks)
			if err == nil {
				t.Errorf("Pipeline.ValidateFinalTasks() did not return error for invalid pipeline: %s", tt.name)
			}
		})
	}
}
