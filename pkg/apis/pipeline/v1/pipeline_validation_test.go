/*
Copyright 2022 The Tekton Authors

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

package v1

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

func TestPipeline_Validate_Success(t *testing.T) {
	tests := []struct {
		name string
		p    *Pipeline
		wc   func(context.Context) context.Context
	}{{
		name: "valid metadata",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}}},
			},
		},
	}, {
		name: "pipelinetask custom task references",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{APIVersion: "example.dev/v0", Kind: "Example", Name: ""}}},
			},
		},
	}, {
		name: "pipelinetask custom task spec",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo",
					TaskSpec: &EmbeddedTask{
						TypeMeta: runtime.TypeMeta{
							APIVersion: "example.dev/v0",
							Kind:       "Example",
						},
						Spec: runtime.RawExtension{
							Raw: []byte(`{"field1":123,"field2":"value"}`),
						}},
				}},
			},
		},
	}, {
		name: "valid Task without apiversion",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "bar", Kind: NamespacedTaskKind}}},
			},
		},
	}, {
		name: "valid reference to clusterTask",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "bar", Kind: ClusterTaskRefKind}}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.p.Validate(ctx)
			if err != nil {
				t.Errorf("Pipeline.Validate() returned error for valid Pipeline: %v", err)
			}
		})
	}
}

func TestPipeline_Validate_Failure(t *testing.T) {
	tests := []struct {
		name          string
		p             *Pipeline
		expectedError apis.FieldError
		wc            func(context.Context) context.Context
	}{{
		name: "comma in name",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipe,line"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}}},
			},
		},
		expectedError: apis.FieldError{
			Message: `invalid resource name "pipe,line": must be a valid DNS label`,
			Paths:   []string{"metadata.name"},
		},
	}, {
		name: "pipeline name too long",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "asdf123456789012345678901234567890123456789012345678901234567890"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}}},
			},
		},
		expectedError: apis.FieldError{
			Message: "Invalid resource name: length must be no more than 63 characters",
			Paths:   []string{"metadata.name"},
		},
	}, {
		name: "pipeline spec missing",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
		},
		expectedError: apis.FieldError{
			Message: `expected at least one, got none`,
			Paths:   []string{"spec.description", "spec.params", "spec.resources", "spec.tasks", "spec.workspaces"},
		},
	}, {
		name: "invalid parameter usage in pipeline task",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name: "invalid-pipeline-task",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Steps: []Step{{
							Name:   "some-step",
							Image:  "some-image",
							Script: "$(params.doesnotexist)",
						}},
					}},
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.doesnotexist)"`,
			Paths:   []string{"spec.tasks[0].steps[0].script"},
		},
	}, {
		name: "invalid parameter usage in finally pipeline task",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Tasks: []PipelineTask{{
					Name: "pipeline-task",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Steps: []Step{{
							Name:    "some-step",
							Image:   "some-image",
							Command: []string{"cmd"},
						}},
					}},
				}},
				Finally: []PipelineTask{{
					Name: "invalid-pipeline-task",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Steps: []Step{{
							Name:   "some-step",
							Image:  "some-image",
							Script: "$(params.doesnotexist)",
						}},
					}},
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.doesnotexist)"`,
			Paths:   []string{"spec.finally[0].steps[0].script"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.p.Validate(ctx)
			if err == nil {
				t.Error("Pipeline.Validate() did not return error for invalid pipeline")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Pipeline.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineSpec_Validate_Failure(t *testing.T) {
	tests := []struct {
		name          string
		ps            *PipelineSpec
		expectedError apis.FieldError
	}{{
		name: "invalid pipeline with one pipeline task having taskRef and taskSpec both",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:     "invalid-pipeline-task",
				TaskRef:  &TaskRef{Name: "foo-task"},
				TaskSpec: &EmbeddedTask{TaskSpec: getTaskSpec()},
			}},
		},
		expectedError: apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"tasks[1].taskRef", "tasks[1].taskSpec"},
		},
	}, {
		name: "invalid pipeline with one pipeline task having when expression with invalid operator (not In/NotIn)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "bar-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.Exists,
					Values:   []string{"foo"},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: operator "exists" is not recognized. valid operators: in,notin`,
			Paths:   []string{"tasks[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with final task having when expression with invalid operator (not In/NotIn)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "bar-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "bar-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.Exists,
					Values:   []string{"foo"},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: operator "exists" is not recognized. valid operators: in,notin`,
			Paths:   []string{"finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with dag task and final task having when expression with invalid operator (not In/NotIn)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "bar-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.Exists,
					Values:   []string{"foo"},
				}},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "bar-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.Exists,
					Values:   []string{"foo"},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: operator "exists" is not recognized. valid operators: in,notin`,
			Paths:   []string{"tasks[0].when[0]", "finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with one pipeline task having when expression with invalid values (empty)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: expecting non-empty values field`,
			Paths:   []string{"tasks[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with final task having when expression with invalid values (empty)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: expecting non-empty values field`,
			Paths:   []string{"finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with dag task and final task having when expression with invalid values (empty)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{},
				}},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
					Values:   []string{},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: expecting non-empty values field`,
			Paths:   []string{"tasks[0].when[0]", "finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with one pipeline task having when expression with invalid operator (missing)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:  "foo",
					Values: []string{"foo"},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: operator "" is not recognized. valid operators: in,notin`,
			Paths:   []string{"tasks[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with final task having when expression with invalid operator (missing)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:  "foo",
					Values: []string{"foo"},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: operator "" is not recognized. valid operators: in,notin`,
			Paths:   []string{"finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with dag task and final task having when expression with invalid operator (missing)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:  "foo",
					Values: []string{"foo"},
				}},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:  "foo",
					Values: []string{"foo"},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: operator "" is not recognized. valid operators: in,notin`,
			Paths:   []string{"tasks[0].when[0]", "finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with one pipeline task having when expression with invalid values (missing)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: expecting non-empty values field`,
			Paths:   []string{"tasks[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with final task having when expression with invalid values (missing)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: expecting non-empty values field`,
			Paths:   []string{"finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with dag task and final task having when expression with invalid values (missing)",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
				}},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "foo",
					Operator: selection.In,
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: expecting non-empty values field`,
			Paths:   []string{"tasks[0].when[0]", "finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with a pipelineTask having when expression with invalid result reference - empty referenced task",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "$(tasks..results.bResult)",
					Operator: selection.In,
					Values:   []string{"bar"},
				}},
			}},
		},
		expectedError: *apis.ErrGeneric(`invalid value: couldn't add link between invalid-pipeline-task and : task invalid-pipeline-task depends on  but  wasn't present in Pipeline`, "tasks"),
	}, {
		name: "invalid pipeline with a pipelineTask having when expression with invalid result reference - referenced task does not exist in the pipeline",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "$(tasks.a-task.results.bResult)",
					Operator: selection.In,
					Values:   []string{"bar"},
				}},
			}},
		},
		expectedError: *apis.ErrGeneric(`invalid value: couldn't add link between invalid-pipeline-task and a-task: task invalid-pipeline-task depends on a-task but a-task wasn't present in Pipeline`, "tasks"),
	}, {
		name: "invalid pipeline with a pipelineTask having when expression with invalid result reference - referenced task does not exist in the pipeline",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Params:      []ParamSpec{{Name: "prefix", Type: ParamTypeString}},
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				Params: Params{{
					Name: "prefix", Value: ParamValue{Type: ParamTypeString, StringVal: "bar"},
				}},
				When: []WhenExpression{{
					Input:    "$(params.prefix):$(tasks.a-task.results.bResult)",
					Operator: selection.In,
					Values:   []string{"bar"},
				}},
			}},
		},
		expectedError: *apis.ErrGeneric(`invalid value: couldn't add link between invalid-pipeline-task and a-task: task invalid-pipeline-task depends on a-task but a-task wasn't present in Pipeline`, "tasks"),
	}, {
		name: "invalid pipeline with final task having when expression with invalid result reference - referenced task does not exist in the pipeline",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "$(tasks.a-task.results.bResult)",
					Operator: selection.In,
					Values:   []string{"bar"},
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: invalid task result reference, final task has task result reference from a task a-task which is not defined in the pipeline`,
			Paths:   []string{"finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with dag task and final task having when expression with invalid result reference - referenced task does not exist in the pipeline",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "$(tasks.a-task.results.bResult)",
					Operator: selection.In,
					Values:   []string{"bar"},
				}},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When: []WhenExpression{{
					Input:    "$(tasks.a-task.results.bResult)",
					Operator: selection.In,
					Values:   []string{"bar"},
				}},
			}},
		},
		expectedError: *apis.ErrGeneric(`invalid value: couldn't add link between invalid-pipeline-task and a-task: task invalid-pipeline-task depends on a-task but a-task wasn't present in Pipeline`, "tasks").Also(
			&apis.FieldError{
				Message: `invalid value: invalid task result reference, final task has task result reference from a task a-task which is not defined in the pipeline`,
				Paths:   []string{"finally[0].when[0]"},
			}),
	}, {
		name: "invalid pipeline with one pipeline task having blank when expression",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When:    []WhenExpression{{}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"tasks[1].when[0]"},
		},
	}, {
		name: "invalid pipeline with final task having blank when expression",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When:    []WhenExpression{{}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"finally[0].when[0]"},
		},
	}, {
		name: "invalid pipeline with dag task and final task having blank when expression",
		ps: &PipelineSpec{
			Description: "this is an invalid pipeline with invalid pipeline task",
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}, {
				Name:    "invalid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
				When:    []WhenExpression{{}},
			}},
			Finally: []PipelineTask{{
				Name:    "invalid-pipeline-task-finally",
				TaskRef: &TaskRef{Name: "foo-task"},
				When:    []WhenExpression{{}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"tasks[1].when[0]", "finally[0].when[0]"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ps.Validate(context.Background())
			if err == nil {
				t.Errorf("PipelineSpec.Validate() did not return error for invalid pipelineSpec")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineSpec_Validate_Failure_CycleDAG(t *testing.T) {
	name := "invalid pipeline spec with DAG having cyclic dependency"
	ps := &PipelineSpec{
		Tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}, RunAfter: []string{"baz"},
		}, {
			Name: "bar", TaskRef: &TaskRef{Name: "bar-task"}, RunAfter: []string{"foo"},
		}, {
			Name: "baz", TaskRef: &TaskRef{Name: "baz-task"}, RunAfter: []string{"bar"},
		}},
	}
	err := ps.Validate(context.Background())
	if err == nil {
		t.Errorf("PipelineSpec.Validate() did not return error for invalid pipelineSpec: %s", name)
	}
}

func TestValidatePipelineTasks_Failure(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []PipelineTask
		finalTasks    []PipelineTask
		expectedError apis.FieldError
	}{{
		name: "pipeline tasks invalid (duplicate tasks)",
		tasks: []PipelineTask{
			{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}},
			{Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}},
		},
		expectedError: apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"tasks[1].name"},
		},
	}, {
		name: "apiVersion with steps",
		tasks: []PipelineTask{{
			Name: "foo",
			TaskSpec: &EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "tekton.dev/v1",
				},
				TaskSpec: TaskSpec{
					Steps: []Step{{
						Name:  "some-step",
						Image: "some-image",
					}},
				},
			},
		}},
		finalTasks: nil,
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: "taskSpec.apiVersion cannot be specified when using taskSpec.steps",
			Paths:   []string{"tasks[0].taskSpec.apiVersion"},
		}).Also(apis.ErrInvalidValue("custom task spec must specify kind", "tasks[0].taskSpec.kind")),
	}, {
		name: "kind with steps",
		tasks: []PipelineTask{{
			Name: "foo",
			TaskSpec: &EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					Kind: "Task",
				},
				TaskSpec: TaskSpec{
					Steps: []Step{{
						Name:  "some-step",
						Image: "some-image",
					}},
				},
			},
		}},
		finalTasks: nil,
		expectedError: apis.FieldError{
			Message: "taskSpec.kind cannot be specified when using taskSpec.steps",
			Paths:   []string{"tasks[0].taskSpec.kind"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePipelineTasks(context.Background(), tt.tasks, tt.finalTasks)
			if err == nil {
				t.Error("ValidatePipelineTasks() did not return error for invalid pipeline tasks")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("ValidatePipelineTasks() errors diff %s", diff.PrintWantGot(d))
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
	if err := validateGraph(tasks); err != nil {
		t.Errorf("Pipeline.validateGraph() returned error for valid DAG of pipeline tasks: %s: %v", desc, err)
	}
}

func TestValidateGraph_Failure(t *testing.T) {
	desc := "invalid dependency graph between the tasks with cyclic dependency"
	tasks := []PipelineTask{{
		Name: "foo", TaskRef: &TaskRef{Name: "foo-task"}, RunAfter: []string{"bar"},
	}, {
		Name: "bar", TaskRef: &TaskRef{Name: "bar-task"}, RunAfter: []string{"foo"},
	}}
	expectedError := apis.FieldError{
		Message: `invalid value: cycle detected; task "bar" depends on "foo"`,
		Paths:   []string{"tasks"},
	}
	err := validateGraph(tasks)
	if err == nil {
		t.Error("Pipeline.validateGraph() did not return error for invalid DAG of pipeline tasks:", desc)
	} else if d := cmp.Diff(expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
		t.Errorf("Pipeline.validateGraph() errors diff %s", diff.PrintWantGot(d))
	}
}

func TestValidatePipelineResults_Success(t *testing.T) {
	desc := "valid pipeline with valid pipeline results syntax"
	results := []PipelineResult{{
		Name:        "my-pipeline-result",
		Description: "this is my pipeline result",
		Value:       *NewStructuredValues("$(tasks.a-task.results.output)"),
	}, {
		Name:        "my-pipeline-object-result",
		Description: "this is my pipeline result",
		Value:       *NewStructuredValues("$(tasks.a-task.results.gitrepo.commit)"),
	}}
	if err := validatePipelineResults(results, []PipelineTask{{Name: "a-task"}}, []PipelineTask{}); err != nil {
		t.Errorf("Pipeline.validatePipelineResults() returned error for valid pipeline: %s: %v", desc, err)
	}
}

func TestValidatePipelineResults_Failure(t *testing.T) {
	tests := []struct {
		desc          string
		results       []PipelineResult
		expectedError apis.FieldError
	}{{
		desc: "invalid pipeline task result reference",
		results: []PipelineResult{{
			Name:        "my-pipeline-result",
			Description: "this is my pipeline result",
			Value:       *NewStructuredValues("$(tasks.a-task.results.output.key1.extra)"),
		}},
		expectedError: *apis.ErrInvalidValue(`expected all of the expressions [tasks.a-task.results.output.key1.extra] to be result expressions but only [] were`, "results[0].value").Also(
			apis.ErrInvalidValue("referencing a nonexistent task", "results[0].value")),
	}, {
		desc: "invalid pipeline finally result reference variable",
		results: []PipelineResult{{
			Name:        "my-pipeline-result",
			Description: "this is my pipeline result",
			Value:       *NewStructuredValues("$(finally.a-task.results.output.key1.extra)"),
		}},
		expectedError: *apis.ErrInvalidValue(`expected all of the expressions [finally.a-task.results.output.key1.extra] to be result expressions but only [] were`, "results[0].value").Also(
			apis.ErrInvalidValue("referencing a nonexistent task", "results[0].value")),
	}, {
		desc: "invalid pipeline result value with static string",
		results: []PipelineResult{{
			Name:        "my-pipeline-result",
			Description: "this is my pipeline result",
			Value:       *NewStructuredValues("foo.bar"),
		}},
		expectedError: *apis.ErrInvalidValue(`expected pipeline results to be task result expressions but an invalid expressions was found`, "results[0].value").Also(
			apis.ErrInvalidValue(`expected pipeline results to be task result expressions but no expressions were found`, "results[0].value")).Also(
			apis.ErrInvalidValue(`referencing a nonexistent task`, "results[0].value")),
	}, {
		desc: "invalid pipeline result value with invalid expression",
		results: []PipelineResult{{
			Name:        "my-pipeline-result",
			Description: "this is my pipeline result",
			Value:       *NewStructuredValues("$(foo.bar)"),
		}},
		expectedError: *apis.ErrInvalidValue(`expected pipeline results to be task result expressions but an invalid expressions was found`, "results[0].value").Also(
			apis.ErrInvalidValue("referencing a nonexistent task", "results[0].value")),
	}}
	for _, tt := range tests {
		err := validatePipelineResults(tt.results, []PipelineTask{{Name: "a-task"}}, []PipelineTask{})
		if err == nil {
			t.Errorf("Pipeline.validatePipelineResults() did not return for invalid pipeline: %s", tt.desc)
		}
		if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
			t.Errorf("Pipeline.validatePipelineResults() errors diff %s", diff.PrintWantGot(d))
		}
	}
}

func TestFinallyTaskResultsToPipelineResults_Success(t *testing.T) {
	tests := []struct {
		name string
		p    *Pipeline
		wc   func(context.Context) context.Context
	}{{
		name: "valid pipeline with pipeline results",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Results: []PipelineResult{{
					Name:  "initialized",
					Value: *NewStructuredValues("$(tasks.clone-app-repo.results.initialized)"),
				}},
				Tasks: []PipelineTask{{
					Name: "clone-app-repo",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Results: []TaskResult{{
							Name: "initialized",
							Type: "string",
						}},
						Steps: []Step{{
							Name: "foo", Image: "bar",
						}},
					}},
				}},
			},
		}}, {
		name: "referencing existent finally task result",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Results: []PipelineResult{{
					Name:  "initialized",
					Value: *NewStructuredValues("$(finally.check-git-commit.results.init)"),
				}},
				Tasks: []PipelineTask{{
					Name: "clone-app-repo",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Results: []TaskResult{{
							Name: "current-date-unix-timestamp",
							Type: "string",
						}},
						Steps: []Step{{
							Name: "foo", Image: "bar",
						}},
					}},
				}},
				Finally: []PipelineTask{{
					Name: "check-git-commit",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Results: []TaskResult{{
							Name: "init",
							Type: "string",
						}},
						Steps: []Step{{
							Name: "foo2", Image: "bar",
						}},
					}},
				}},
			},
		},
	},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.p.Validate(ctx)
			if err != nil {
				t.Errorf("Pipeline.finallyTaskResultsToPipelineResults() returned error for valid Pipeline: %v", err)
			}
		})
	}
}

func TestFinallyTaskResultsToPipelineResults_Failure(t *testing.T) {
	tests := []struct {
		desc          string
		p             *Pipeline
		expectedError apis.FieldError
		wc            func(context.Context) context.Context
	}{{
		desc: "invalid propagation of finally task results from pipeline results",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Results: []PipelineResult{{
					Name:  "initialized",
					Value: *NewStructuredValues("$(tasks.check-git-commit.results.init)"),
				}},
				Tasks: []PipelineTask{{
					Name: "clone-app-repo",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Results: []TaskResult{{
							Name: "current-date-unix-timestamp",
							Type: "string",
						}},
						Steps: []Step{{
							Name: "foo", Image: "bar",
						}},
					}},
				}},
				Finally: []PipelineTask{{
					Name: "check-git-commit",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Results: []TaskResult{{
							Name: "init",
							Type: "string",
						}},
						Steps: []Step{{
							Name: "foo2", Image: "bar",
						}},
					}},
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: referencing a nonexistent task`,
			Paths:   []string{"spec.results[0].value"},
		},
	}, {
		desc: "referencing nonexistent finally task result",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Results: []PipelineResult{{
					Name:  "initialized",
					Value: *NewStructuredValues("$(finally.nonexistent-task.results.init)"),
				}},
				Tasks: []PipelineTask{{
					Name: "clone-app-repo",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Results: []TaskResult{{
							Name: "current-date-unix-timestamp",
							Type: "string",
						}},
						Steps: []Step{{
							Name: "foo", Image: "bar",
						}},
					}},
				}},
				Finally: []PipelineTask{{
					Name: "check-git-commit",
					TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
						Results: []TaskResult{{
							Name: "init",
							Type: "string",
						}},
						Steps: []Step{{
							Name: "foo2", Image: "bar",
						}},
					}},
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: referencing a nonexistent task`,
			Paths:   []string{"spec.results[0].value"},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.p.Validate(ctx)
			if err == nil {
				t.Errorf("Pipeline.finallyTaskResultsToPipelineResults() did not return for invalid pipeline: %s", tt.desc)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Pipeline.finallyTaskResultsToPipelineResults() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
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
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.baz) and $(params.foo-is-baz)"},
			}},
		}},
	}, {
		name: "valid string parameter variables in when expression",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString,
		}, {
			Name: "foo-is-baz", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "$(params.baz)",
				Operator: selection.In,
				Values:   []string{"foo"},
			}, {
				Input:    "baz",
				Operator: selection.In,
				Values:   []string{"$(params.foo-is-baz)"},
			}},
		}},
	}, {
		name: "valid string parameter variables in input, array reference in values in when expression",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString,
		}, {
			Name: "foo", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "$(params.baz)",
				Operator: selection.In,
				Values:   []string{"$(params.foo[*])"},
			}},
		}},
	}, {
		name: "valid array parameter variables",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"some", "default"}},
		}, {
			Name: "foo-is-baz", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.baz)", "and", "$(params.foo-is-baz)"}},
			}},
		}},
	}, {
		name: "valid star array parameter variables",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"some", "default"}},
		}, {
			Name: "foo-is-baz", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.baz[*])", "and", "$(params.foo-is-baz[*])"}},
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
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(input.workspace.$(params.baz))"},
			}},
		}},
	}, {
		name: "valid array parameter variables in matrix",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"some", "default"}},
		}, {
			Name: "foo-is-baz", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.baz)", "and", "$(params.foo-is-baz)"}},
				}}},
		}},
	}, {
		name: "valid star array parameter variables in matrix",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"some", "default"}},
		}, {
			Name: "foo-is-baz", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.baz[*])", "and", "$(params.foo-is-baz[*])"}},
				}}},
		}},
	}, {
		name: "array param - using the whole variable as a param's value that is intended to be array type",
		params: []ParamSpec{{
			Name: "myArray",
			Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param-intended-to-be-array", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.myArray[*])"},
			}},
		}},
	}, {
		name: "valid string parameter variables in matrix include",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.baz)"}},
					}}}},
		}},
	}, {
		name: "object param - using single individual variable in string param",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-string-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.myObject.key1)"},
			}},
		}},
	}, {
		name: "object param - using multiple individual variables in string param",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-string-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.myObject.key1) and $(params.myObject.key2)"},
			}},
		}},
	}, {
		name: "object param - using individual variables in array param",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "an-array-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.myObject.key1)", "another one $(params.myObject.key2)"}},
			}},
		}},
	}, {
		name: "object param - using individual variables and string param as the value of other object individual keys",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}, {
			Name: "myString",
			Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "an-object-param", Value: ParamValue{Type: ParamTypeObject, ObjectVal: map[string]string{
					"url":    "$(params.myObject.key1)",
					"commit": "$(params.myString)",
				}},
			}},
		}},
	}, {
		name: "object param - using individual variables in matrix",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.myObject.key1)", "and", "$(params.myObject.key2)"}},
				}}},
		}},
	}, {
		name: "object param - using the whole variable as a param's value that is intended to be object type",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param-intended-to-be-object", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.myObject[*])"},
			}},
		}},
	}, {
		name: "object param - using individual variable in input of when expression, and using both object individual variable and array reference in values of when expression",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}, {
			Name: "foo", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "$(params.myObject.key1)",
				Operator: selection.In,
				Values:   []string{"$(params.foo[*])", "$(params.myObject.key2)"},
			}},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := config.EnableAlphaAPIFields(context.Background())
			err := ValidatePipelineParameterVariables(ctx, tt.tasks, tt.params)
			if err != nil {
				t.Errorf("Pipeline.ValidatePipelineParameterVariables() returned error for valid pipeline parameters: %v", err)
			}
		})
	}
}

func TestValidatePipelineDeclaredParameterUsage_Failure(t *testing.T) {
	tests := []struct {
		name          string
		params        []ParamSpec
		tasks         []PipelineTask
		expectedError apis.FieldError
	}{{
		name: "invalid pipeline task with a parameter which is missing from the param declarations",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.does-not-exist)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.does-not-exist)"`,
			Paths:   []string{"[0].params[a-param]"},
		},
	}, {
		name: "invalid string parameter variables in when expression, missing input param from the param declarations",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "$(params.baz)",
				Operator: selection.In,
				Values:   []string{"foo"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.baz)"`,
			Paths:   []string{"[0].when[0].input"},
		},
	}, {
		name: "invalid string parameter variables in when expression, missing values param from the param declarations",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "bax",
				Operator: selection.In,
				Values:   []string{"$(params.foo-is-baz)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.foo-is-baz)"`,
			Paths:   []string{"[0].when[0].values"},
		},
	}, {
		name: "invalid string parameter variables in when expression, array reference in input",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "$(params.foo)",
				Operator: selection.In,
				Values:   []string{"foo"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.foo)"`,
			Paths:   []string{"[0].when[0].input"},
		},
	}, {
		name: "Invalid array parameter variable in when expression, array reference in input with array notation [*]",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "$(params.foo)[*]",
				Operator: selection.In,
				Values:   []string{"$(params.foo[*])"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.foo)[*]"`,
			Paths:   []string{"[0].when[0].input"},
		},
	}, {
		name: "invalid pipeline task with a parameter combined with missing param from the param declarations",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.foo) and $(params.does-not-exist)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.foo) and $(params.does-not-exist)"`,
			Paths:   []string{"[0].params[a-param]"},
		},
	}, {
		name: "invalid pipeline task with two parameters and one of them missing from the param declarations",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.foo)"},
			}, {
				Name: "b-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.does-not-exist)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.does-not-exist)"`,
			Paths:   []string{"[0].params[b-param]"},
		},
	}, {
		name: "invalid pipeline task with a matrix parameter which is missing from the param declarations",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.does-not-exist)"}},
				}}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.does-not-exist)"`,
			Paths:   []string{"[0].matrix.params[a-param].value[0]"},
		},
	}, {
		name: "invalid pipeline task with a matrix parameter combined with missing param from the param declarations",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.foo)", "and", "$(params.does-not-exist)"}},
				}}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.does-not-exist)"`,
			Paths:   []string{"[0].matrix.params[a-param].value[2]"},
		},
	}, {
		name: "invalid pipeline task with two matrix parameters and one of them missing from the param declarations",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeArray,
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.foo)"}},
				}, {
					Name: "b-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.does-not-exist)"}}}}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.does-not-exist)"`,
			Paths:   []string{"[0].matrix.params[b-param].value[0]"},
		},
	}, {
		name: "invalid pipeline task with two matrix include parameters and one of them missing from the param declarations",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Matrix: &Matrix{
				Include: IncludeParamsList{{
					Params: Params{{
						Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.foo)"},
					}, {
						Name: "b-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.does-not-exist)"},
					}},
				}}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.does-not-exist)"`,
			Paths:   []string{"[0].matrix.include.params[1]"},
		},
	}, {
		name: "invalid object key in the input of the when expression",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "$(params.myObject.non-exist-key)",
				Operator: selection.In,
				Values:   []string{"foo"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.myObject.non-exist-key)"`,
			Paths:   []string{"[0].when[0].input"},
		},
	}, {
		name: "invalid object key in the Values of the when expression",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			When: []WhenExpression{{
				Input:    "bax",
				Operator: selection.In,
				Values:   []string{"$(params.myObject.non-exist-key)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.myObject.non-exist-key)"`,
			Paths:   []string{"[0].when[0].values"},
		},
	}, {
		name: "invalid object key is used to provide values for array params",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.myObject.non-exist-key)", "last"}},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.myObject.non-exist-key)"`,
			Paths:   []string{"[0].params[a-param].value[0]"},
		},
	}, {
		name: "invalid object key is used to provide values for string params",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.myObject.non-exist-key)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.myObject.non-exist-key)"`,
			Paths:   []string{"[0].params[a-param]"},
		},
	}, {
		name: "invalid object key is used to provide values for object params",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}, {
			Name: "myString",
			Type: ParamTypeString,
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "an-object-param", Value: ParamValue{Type: ParamTypeObject, ObjectVal: map[string]string{
					"url":    "$(params.myObject.non-exist-key)",
					"commit": "$(params.myString)",
				}},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.myObject.non-exist-key)"`,
			Paths:   []string{"[0].params[an-object-param].properties[url]"},
		},
	}, {
		name: "invalid object key is used to provide values for matrix params",
		params: []ParamSpec{{
			Name: "myObject",
			Type: ParamTypeObject,
			Properties: map[string]PropertySpec{
				"key1": {Type: "string"},
				"key2": {Type: "string"},
			},
		}},
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.myObject.key1)"}},
				}, {
					Name: "b-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(params.myObject.non-exist-key)"}},
				}}},
		}},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.myObject.non-exist-key)"`,
			Paths:   []string{"[0].matrix.params[b-param].value[0]"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineTaskParameterUsage(tt.tasks, tt.params)
			if err == nil {
				t.Errorf("Pipeline.ValidatePipelineParameterVariables() did not return error for invalid pipeline parameters")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidatePipelineParameterVariables_Failure(t *testing.T) {
	tests := []struct {
		name          string
		params        []ParamSpec
		tasks         []PipelineTask
		expectedError apis.FieldError
	}{{
		name: "invalid parameter type",
		params: []ParamSpec{{
			Name: "foo", Type: "invalidtype",
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: invalidtype`,
			Paths:   []string{"params.foo.type"},
		},
	}, {
		name: "array parameter mismatching default type",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeArray, Default: &ParamValue{Type: ParamTypeString, StringVal: "astring"},
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
		expectedError: apis.FieldError{
			Message: `"array" type does not match default value's type: "string"`,
			Paths:   []string{"params.foo.default.type", "params.foo.type"},
		},
	}, {
		name: "string parameter mismatching default type",
		params: []ParamSpec{{
			Name: "foo", Type: ParamTypeString, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
		}},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"params.foo.default.type", "params.foo.type"},
		},
	}, {
		name: "array parameter used as string",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.baz)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"params.baz.default.type", "params.baz.type"},
		},
	}, {
		name: "star array parameter used as string",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.baz[*])"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"params.baz.default.type", "params.baz.type"},
		},
	}, {
		name: "array parameter string template not isolated",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"value: $(params.baz)", "last"}},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"params.baz.default.type", "params.baz.type"},
		},
	}, {
		name: "star array parameter string template not isolated",
		params: []ParamSpec{{
			Name: "baz", Type: ParamTypeString, Default: &ParamValue{Type: ParamTypeArray, ArrayVal: []string{"anarray", "elements"}},
		}},
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"value: $(params.baz[*])", "last"}},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"params.baz.default.type", "params.baz.type"},
		},
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
		expectedError: apis.FieldError{
			Message: `parameter appears more than once`,
			Paths:   []string{"params[baz]"},
		},
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
		expectedError: apis.FieldError{
			Message: `parameter appears more than once`,
			Paths:   []string{"params[baz]"},
		},
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
		expectedError: apis.FieldError{
			Message: `parameter appears more than once`,
			Paths:   []string{"params[baz]"},
		},
	}, {
		name: "invalid task use duplicate parameters",
		tasks: []PipelineTask{{
			Name:    "foo-task",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "duplicate-param", Value: ParamValue{Type: ParamTypeString, StringVal: "val1"},
			}, {
				Name: "duplicate-param", Value: ParamValue{Type: ParamTypeString, StringVal: "val2"},
			}, {
				Name: "duplicate-param", Value: ParamValue{Type: ParamTypeString, StringVal: "val3"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `parameter names must be unique, the parameter "duplicate-param" is also defined at`,
			Paths:   []string{"[0].params[1].name, [0].params[2].name"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := ValidatePipelineParameterVariables(ctx, tt.tasks, tt.params)
			if err == nil {
				t.Errorf("Pipeline.ValidatePipelineParameterVariables() did not return error for invalid pipeline parameters")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidatePipelineWorkspacesDeclarations_Success(t *testing.T) {
	desc := "pipeline spec workspaces do not cause an error"
	workspaces := []PipelineWorkspaceDeclaration{{
		Name: "foo",
	}, {
		Name: "bar",
	}}
	t.Run(desc, func(t *testing.T) {
		err := validatePipelineWorkspacesDeclarations(workspaces)
		if err != nil {
			t.Errorf("Pipeline.validatePipelineWorkspacesDeclarations() returned error for valid pipeline workspaces: %v", err)
		}
	})
}

func TestValidatePipelineWorkspacesUsage_Success(t *testing.T) {
	tests := []struct {
		name           string
		workspaces     []PipelineWorkspaceDeclaration
		tasks          []PipelineTask
		skipValidation bool
	}{{
		name: "unused pipeline spec workspaces do not cause an error",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "foo",
		}, {
			Name: "bar",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
		}},
		skipValidation: false,
	}, {
		name: "valid mapping pipeline-task workspace name with pipeline workspace name",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "pipelineWorkspaceName",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
			Workspaces: []WorkspacePipelineTaskBinding{{
				Name:      "pipelineWorkspaceName",
				Workspace: "",
			}},
		}},
		skipValidation: false,
	}, {
		name: "skip validating workspace usage",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "pipelineWorkspaceName",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
		}},
		skipValidation: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validatePipelineTasksWorkspacesUsage(tt.workspaces, tt.tasks).ViaField("tasks")
			if errs != nil {
				t.Errorf("Pipeline.validatePipelineWorkspacesUsage() returned error for valid pipeline workspaces: %v", errs)
			}
		})
	}
}

func TestValidatePipelineWorkspacesDeclarations_Failure(t *testing.T) {
	tests := []struct {
		name          string
		workspaces    []PipelineWorkspaceDeclaration
		tasks         []PipelineTask
		expectedError apis.FieldError
	}{{
		name: "multiple workspaces sharing the same name are not allowed",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "foo",
		}, {
			Name: "foo",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: workspace with name "foo" appears more than once`,
			Paths:   []string{"workspaces[1]"},
		},
	}, {
		name: "workspace name must not be empty",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: workspace 0 has empty name`,
			Paths:   []string{"workspaces[0]"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validatePipelineWorkspacesDeclarations(tt.workspaces)
			if errs == nil {
				t.Errorf("Pipeline.validatePipelineWorkspacesDeclarations() did not return error for invalid pipeline workspaces")
			}
			if d := cmp.Diff(tt.expectedError.Error(), errs.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.validatePipelineWorkspacesDeclarations() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidatePipelineWorkspacesUsage_Failure(t *testing.T) {
	tests := []struct {
		name          string
		workspaces    []PipelineWorkspaceDeclaration
		tasks         []PipelineTask
		expectedError apis.FieldError
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
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task "foo" expects workspace with name "pipelineWorkspaceName" but none exists in pipeline spec`,
			Paths:   []string{"tasks[0].workspaces[0]"},
		},
	}, {
		name: "invalid mapping workspace with different name",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "pipelineWorkspaceName",
		}},
		tasks: []PipelineTask{{
			Name: "foo", TaskRef: &TaskRef{Name: "foo"},
			Workspaces: []WorkspacePipelineTaskBinding{{
				Name:      "taskWorkspaceName",
				Workspace: "",
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task "foo" expects workspace with name "taskWorkspaceName" but none exists in pipeline spec`,
			Paths:   []string{"tasks[0].workspaces[0]"},
		},
	}, {
		name: "invalid pipeline task use duplicate workspace binding name",
		workspaces: []PipelineWorkspaceDeclaration{{
			Name: "foo",
		}},
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo"},
			Workspaces: []WorkspacePipelineTaskBinding{
				{
					Name:      "repo",
					Workspace: "foo",
				},
				{
					Name:      "repo",
					Workspace: "foo",
				},
			},
		}},
		expectedError: apis.FieldError{
			Message: `workspace name "repo" must be unique`,
			Paths:   []string{"tasks[0].workspaces[1]"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validatePipelineTasksWorkspacesUsage(tt.workspaces, tt.tasks).ViaField("tasks")
			if errs == nil {
				t.Errorf("Pipeline.validatePipelineWorkspacesUsage() did not return error for invalid pipeline workspaces")
			}
			if d := cmp.Diff(tt.expectedError.Error(), errs.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.validatePipelineWorkspacesUsage() errors diff %s", diff.PrintWantGot(d))
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
					Name:     "final-task-2",
					TaskSpec: &EmbeddedTask{TaskSpec: getTaskSpec()},
				}},
			},
		},
	}, {
		name: "valid pipeline with final tasks referring to task results from a dag task",
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
					Params: Params{{
						Name: "param1", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.non-final-task.results.output)"},
					}},
				}},
			},
		},
	}, {
		name: "valid pipeline with final tasks referring to context variables",
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
					Params: Params{{
						Name: "param1", Value: ParamValue{Type: ParamTypeString, StringVal: "$(context.pipelineRun.name)"},
					}},
				}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if err != nil {
				t.Errorf("Pipeline.Validate() returned error for valid pipeline with finally: %v", err)
			}
		})
	}
}

func TestValidatePipelineWithFinalTasks_Failure(t *testing.T) {
	tests := []struct {
		name          string
		p             *Pipeline
		expectedError apis.FieldError
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
		expectedError: apis.FieldError{
			Message: `invalid value: spec.tasks is empty but spec.finally has 1 tasks`,
			Paths:   []string{"spec.finally"},
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
		expectedError: *apis.ErrMissingOneOf("spec.tasks[0].taskRef", "spec.tasks[0].taskSpec").Also(
			&apis.FieldError{
				Message: `invalid value ""`,
				Paths:   []string{"spec.tasks[0].name"},
				Details: "Pipeline Task name must be a valid DNS Label." +
					"For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
			}),
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
		expectedError: *apis.ErrMissingOneOf("spec.finally[0].taskRef", "spec.finally[0].taskSpec").Also(
			&apis.FieldError{
				Message: `invalid value ""`,
				Paths:   []string{"spec.finally[0].name"},
				Details: "Pipeline Task name must be a valid DNS Label." +
					"For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
			}),
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
		expectedError: apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"spec.finally[1].name"},
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
		expectedError: apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"spec.finally[0].name"},
		},
	}, {
		name: "final task missing taskref and taskspec",
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
		expectedError: apis.FieldError{
			Message: `expected exactly one, got neither`,
			Paths:   []string{"spec.finally[0].taskRef", "spec.finally[0].taskSpec"},
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
					Name:     "final-task",
					TaskRef:  &TaskRef{Name: "non-final-task"},
					TaskSpec: &EmbeddedTask{TaskSpec: getTaskSpec()},
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"spec.finally[0].taskRef", "spec.finally[0].taskSpec"},
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
					Params: Params{{
						Name: "final-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.foo) and $(params.does-not-exist)"},
					}},
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.foo) and $(params.does-not-exist)"`,
			Paths:   []string{"spec.finally[0].params[final-param]"},
		},
	}, {
		name: "invalid pipeline with invalid final tasks with runAfter",
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
				}},
			},
		},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `invalid value: no runAfter allowed under spec.finally, final task final-task-1 has runAfter specified`,
			Paths:   []string{"spec.finally[0]"},
		}),
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
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task "final-task" expects workspace with name "pipeline-shared-workspace" but none exists in pipeline spec`,
			Paths:   []string{"spec.finally[0].workspaces[0]"},
		},
	}, {
		name: "invalid pipeline with no tasks under tasks section and empty finally section",
		p: &Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline"},
			Spec: PipelineSpec{
				Finally: []PipelineTask{},
			},
		},
		expectedError: *apis.ErrGeneric("expected at least one, got none", "spec.description", "spec.params", "spec.resources", "spec.tasks", "spec.workspaces"),
	}, {
		name: "invalid pipeline with final tasks referring to invalid context variables",
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
					Params: Params{{
						Name: "param1", Value: ParamValue{Type: ParamTypeString, StringVal: "$(context.pipelineRun.missing)"},
					}},
				}},
			},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(context.pipelineRun.missing)"`,
			Paths:   []string{"spec.finally.value"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate(context.Background())
			if err == nil {
				t.Errorf("Pipeline.Validate() did not return error for invalid pipeline with finally")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.Validate() errors diff %s", diff.PrintWantGot(d))
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
				t.Errorf("Pipeline.ValidateTasksAndFinallySection() returned error for valid pipeline with finally: %v", err)
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
	expectedError := apis.FieldError{
		Message: `invalid value: spec.tasks is empty but spec.finally has 1 tasks`,
		Paths:   []string{"finally"},
	}
	err := validateTasksAndFinallySection(ps)
	if err == nil {
		t.Errorf("Pipeline.ValidateTasksAndFinallySection() did not return error for invalid pipeline with finally: %s", desc)
	}
	if d := cmp.Diff(expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
		t.Errorf("Pipeline.validateParamResults() errors diff %s", diff.PrintWantGot(d))
	}
}

func TestValidateFinalTasks_Failure(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []PipelineTask
		finalTasks    []PipelineTask
		expectedError apis.FieldError
	}{{
		name: "invalid pipeline with final task specifying runAfter",
		finalTasks: []PipelineTask{{
			Name:     "final-task",
			TaskRef:  &TaskRef{Name: "final-task"},
			RunAfter: []string{"non-final-task"},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: no runAfter allowed under spec.finally, final task final-task has runAfter specified`,
			Paths:   []string{"finally[0]"},
		},
	}, {
		name: "invalid pipeline with final tasks having task results reference from a final task",
		finalTasks: []PipelineTask{{
			Name:    "final-task-1",
			TaskRef: &TaskRef{Name: "final-task"},
		}, {
			Name:    "final-task-2",
			TaskRef: &TaskRef{Name: "final-task"},
			Params: Params{{
				Name: "param1", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.final-task-1.results.output)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: invalid task result reference, final task has task result reference from a final task final-task-1`,
			Paths:   []string{"finally[1].params[param1].value"},
		},
	}, {
		name: "invalid pipeline with final tasks having task results reference from a final task",
		finalTasks: []PipelineTask{{
			Name:    "final-task-1",
			TaskRef: &TaskRef{Name: "final-task"},
		}, {
			Name:    "final-task-2",
			TaskRef: &TaskRef{Name: "final-task"},
			When: WhenExpressions{{
				Input:    "$(tasks.final-task-1.results.output)",
				Operator: selection.In,
				Values:   []string{"result"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: invalid task result reference, final task has task result reference from a final task final-task-1`,
			Paths:   []string{"finally[1].when[0]"},
		},
	}, {
		name: "invalid pipeline with final tasks having task results reference from non existent dag task",
		finalTasks: []PipelineTask{{
			Name:    "final-task",
			TaskRef: &TaskRef{Name: "final-task"},
			Params: Params{{
				Name: "param1", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.no-dag-task-1.results.output)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: invalid task result reference, final task has task result reference from a task no-dag-task-1 which is not defined in the pipeline`,
			Paths:   []string{"finally[0].params[param1].value"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFinalTasks(tt.tasks, tt.finalTasks)
			if err == nil {
				t.Errorf("Pipeline.ValidateFinalTasks() did not return error for invalid pipeline")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
func TestContextValid(t *testing.T) {
	tests := []struct {
		name  string
		tasks []PipelineTask
	}{{
		name: "valid string context variable for Pipeline name",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipeline.name)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-mat", Value: ParamValue{ArrayVal: []string{"$(context.pipeline.name)"}},
				}}},
		}},
	}, {
		name: "valid string context variable for PipelineRun name",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineRun.name)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-mat", Value: ParamValue{ArrayVal: []string{"$(context.pipelineRun.name)"}},
				}}},
		}},
	}, {
		name: "valid string context variable for PipelineRun namespace",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineRun.namespace)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-mat", Value: ParamValue{ArrayVal: []string{"$(context.pipelineRun.namespace)"}},
				}}},
		}},
	}, {
		name: "valid string context variable for PipelineRun uid",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineRun.uid)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-mat", Value: ParamValue{ArrayVal: []string{"$(context.pipelineRun.uid)"}},
				}}},
		}},
	}, {
		name: "valid array context variables for Pipeline and PipelineRun names",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{ArrayVal: []string{"$(context.pipeline.name)", "and", "$(context.pipelineRun.name)"}},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-mat", Value: ParamValue{ArrayVal: []string{"$(context.pipeline.name)", "and", "$(context.pipelineRun.name)"}},
				}}},
		}},
	}, {
		name: "valid string context variable for PipelineTask retries",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineTask.retries)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineTask.retries)"},
				}}},
		}},
	}, {
		name: "valid array context variable for PipelineTask retries",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{ArrayVal: []string{"$(context.pipelineTask.retries)"}},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-mat", Value: ParamValue{ArrayVal: []string{"$(context.pipelineTask.retries)"}},
				}}},
		}},
	}, {
		name: "valid string context variable for Pipeline name in include params",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipeline.name)"},
			}},
			Matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "a-param-mat", Value: ParamValue{Type: ParamTypeString, StringVal: "$(context.pipeline.name)"}}},
				}}},
		}},
	}, {
		name: "valid string context variable for PipelineTask retries in matrix include",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineTask.retries)"},
			}},
			Matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "a-param-mat", Value: ParamValue{Type: ParamTypeString, StringVal: "$(context.pipelineTask.retries)"}}},
				}}},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validatePipelineContextVariables(tt.tasks); err != nil {
				t.Errorf("Pipeline.validatePipelineContextVariables() returned error for valid pipeline context variables: %v", err)
			}
		})
	}
}

func TestContextInvalid(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []PipelineTask
		expectedError apis.FieldError
	}{{
		name: "invalid string context variable for pipeline",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipeline.missing)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-foo", Value: ParamValue{ArrayVal: []string{"$(context.pipeline.missing-foo)"}},
				}}},
		}},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipeline.missing)"`,
			Paths:   []string{"value"},
		}).Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipeline.missing-foo)"`,
			Paths:   []string{"value"},
		}),
	}, {
		name: "invalid string context variable for pipelineRun",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineRun.missing)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-foo", Value: ParamValue{ArrayVal: []string{"$(context.pipelineRun.missing-foo)"}},
				}}},
		}},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipelineRun.missing)"`,
			Paths:   []string{"value"},
		}).Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipelineRun.missing-foo)"`,
			Paths:   []string{"value"},
		}),
	}, {
		name: "invalid string context variable for pipelineTask",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{StringVal: "$(context.pipelineTask.missing)"},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param-foo", Value: ParamValue{ArrayVal: []string{"$(context.pipelineTask.missing-foo)"}},
				}}},
		}},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipelineTask.missing)"`,
			Paths:   []string{"value"},
		}).Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipelineTask.missing-foo)"`,
			Paths:   []string{"value"},
		}),
	}, {
		name: "invalid array context variables for pipeline, pipelineTask and pipelineRun",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "a-param", Value: ParamValue{ArrayVal: []string{"$(context.pipeline.missing)", "$(context.pipelineTask.missing)", "$(context.pipelineRun.missing)"}},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{ArrayVal: []string{"$(context.pipeline.missing-foo)", "$(context.pipelineTask.missing-foo)", "$(context.pipelineRun.missing-foo)"}},
				}}},
		}},
		expectedError: *apis.ErrGeneric(`non-existent variable in "$(context.pipeline.missing)"`, "value").
			Also(apis.ErrGeneric(`non-existent variable in "$(context.pipelineRun.missing)"`, "value")).
			Also(apis.ErrGeneric(`non-existent variable in "$(context.pipelineTask.missing)"`, "value")).
			Also(apis.ErrGeneric(`non-existent variable in "$(context.pipeline.missing-foo)"`, "value")).
			Also(apis.ErrGeneric(`non-existent variable in "$(context.pipelineRun.missing-foo)"`, "value")).
			Also(apis.ErrGeneric(`non-existent variable in "$(context.pipelineTask.missing-foo)"`, "value")),
	}, {
		name: "invalid string context variable for pipeline in include matrix",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "a-param-foo", Value: ParamValue{Type: ParamTypeString, StringVal: "$(context.pipeline.missing)"}}},
				}}},
		}},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipeline.missing)"`,
			Paths:   []string{"value"},
		}),
	}, {
		name: "invalid string context variable for pipelineRun in include matrix",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "a-param-foo", Value: ParamValue{Type: ParamTypeString, StringVal: "$(context.pipelineRun.missing)"}}},
				}}},
		}},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipelineRun.missing)"`,
			Paths:   []string{"value"},
		}),
	}, {
		name: "invalid string context variable for pipelineTask include matrix",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "a-param-foo", Value: ParamValue{Type: ParamTypeString, StringVal: "$(context.pipelineTask.missing)"}}},
				}}},
		}},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `non-existent variable in "$(context.pipelineTask.missing)"`,
			Paths:   []string{"value"},
		}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePipelineContextVariables(tt.tasks)
			if err == nil {
				t.Errorf("Pipeline.validatePipelineContextVariables() did not return error for invalid pipeline parameters: %s", tt.tasks[0].Params)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTasksExecutionStatus(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []PipelineTask
		finalTasks    []PipelineTask
		expectedError apis.FieldError
	}{{
		name: "valid string variable in finally accessing pipelineTask status",
		tasks: []PipelineTask{{
			Name: "foo",
		}},
		finalTasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "foo-status", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.foo.status)"},
			}, {
				Name: "tasks-status", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.status)"},
			}},
			When: WhenExpressions{{
				Input:    "$(tasks.foo.status)",
				Operator: selection.In,
				Values:   []string{"Failure"},
			}, {
				Input:    "$(tasks.status)",
				Operator: selection.In,
				Values:   []string{"Success"},
			}},
		}},
	}, {
		name: "valid task result reference with status as a variable must not cause validation failure",
		tasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "foo-status", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.foo.results.status)"},
			}},
			When: WhenExpressions{WhenExpression{
				Input:    "$(tasks.foo.results.status)",
				Operator: selection.In,
				Values:   []string{"Failure"},
			}},
		}},
	}, {
		name: "valid variable concatenated with extra string in finally accessing pipelineTask status",
		tasks: []PipelineTask{{
			Name: "foo",
		}},
		finalTasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "foo-status", Value: ParamValue{Type: ParamTypeString, StringVal: "Execution status of foo is $(tasks.foo.status)."},
			}},
		}},
	}, {
		name: "valid variable concatenated with other param in finally accessing pipelineTask status",
		tasks: []PipelineTask{{
			Name: "foo",
		}},
		finalTasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "foo-status", Value: ParamValue{Type: ParamTypeString, StringVal: "Execution status of $(tasks.taskname) is $(tasks.foo.status)."},
			}},
		}},
	}, {
		name: "invalid string variable in dag task accessing pipelineTask status",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "bar-status", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.bar.status)"},
			}},
			When: WhenExpressions{WhenExpression{
				Input:    "$(tasks.bar.status)",
				Operator: selection.In,
				Values:   []string{"foo"},
			}},
		}},
		expectedError: *apis.ErrGeneric("").Also(&apis.FieldError{
			Message: `invalid value: pipeline tasks can not refer to execution status of any other pipeline task or aggregate status of tasks`,
			Paths:   []string{"tasks[0].params[bar-status].value", "tasks[0].when[0]"},
		}),
	}, {
		name: "invalid string variable in dag task accessing aggregate status of tasks",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "tasks-status", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.status)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline tasks can not refer to execution status of any other pipeline task or aggregate status of tasks`,
			Paths:   []string{"tasks[0].params[tasks-status].value"},
		},
	}, {
		name: "invalid variable concatenated with extra string in dag task accessing pipelineTask status",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "bar-status", Value: ParamValue{Type: ParamTypeString, StringVal: "Execution status of bar is $(tasks.bar.status)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline tasks can not refer to execution status of any other pipeline task or aggregate status of tasks`,
			Paths:   []string{"tasks[0].params[bar-status].value"},
		},
	}, {
		name: "invalid array variable in dag task accessing pipelineTask status",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "bar-status", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.bar.status)"}},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline tasks can not refer to execution status of any other pipeline task or aggregate status of tasks`,
			Paths:   []string{"tasks[0].params[bar-status].value"},
		},
	}, {
		name: "invalid array variable in dag task accessing aggregate tasks status",
		tasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			Params: Params{{
				Name: "tasks-status", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.status)"}},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline tasks can not refer to execution status of any other pipeline task or aggregate status of tasks`,
			Paths:   []string{"tasks[0].params[tasks-status].value"},
		},
	}, {
		name: "invalid string variable in finally accessing missing pipelineTask status",
		finalTasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "notask-status", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.notask.status)"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task notask is not defined in the pipeline`,
			Paths:   []string{"finally[0].params[notask-status].value"},
		},
	}, {
		name: "invalid string variable in finally accessing missing pipelineTask status in when expression",
		finalTasks: []PipelineTask{{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			When: WhenExpressions{{
				Input:    "$(tasks.notask.status)",
				Operator: selection.In,
				Values:   []string{"Success"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task notask is not defined in the pipeline`,
			Paths:   []string{"finally[0].when[0]"},
		},
	}, {
		name: "invalid string variable in finally accessing missing pipelineTask status in params and when expression",
		finalTasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "notask-status", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.notask.status)"},
			}},
		}, {
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo-task"},
			When: WhenExpressions{{
				Input:    "$(tasks.notask.status)",
				Operator: selection.In,
				Values:   []string{"Success"},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task notask is not defined in the pipeline`,
			Paths:   []string{"finally[0].params[notask-status].value", "finally[1].when[0]"},
		},
	}, {
		name: "invalid variable concatenated with extra string in finally accessing missing pipelineTask status",
		finalTasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "notask-status", Value: ParamValue{Type: ParamTypeString, StringVal: "Execution status of notask is $(tasks.notask.status)."},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task notask is not defined in the pipeline`,
			Paths:   []string{"finally[0].params[notask-status].value"},
		},
	}, {
		name: "invalid variable concatenated with other params in finally accessing missing pipelineTask status",
		finalTasks: []PipelineTask{{
			Name:    "bar",
			TaskRef: &TaskRef{Name: "bar-task"},
			Params: Params{{
				Name: "notask-status", Value: ParamValue{Type: ParamTypeString, StringVal: "Execution status of $(tasks.taskname) is $(tasks.notask.status)."},
			}},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: pipeline task notask is not defined in the pipeline`,
			Paths:   []string{"finally[0].params[notask-status].value"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExecutionStatusVariables(tt.tasks, tt.finalTasks)
			if len(tt.expectedError.Error()) == 0 {
				if err != nil {
					t.Errorf("Pipeline.validateExecutionStatusVariables() returned error for valid pipeline variable accessing execution status: %s: %v", tt.name, err)
				}
			} else {
				if err == nil {
					t.Errorf("Pipeline.validateExecutionStatusVariables() did not return error for invalid pipeline parameters accessing execution status: %s, %s", tt.name, tt.tasks[0].Params)
				}
				if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
					t.Errorf("PipelineSpec.Validate() errors diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

// TestMatrixIncompatibleAPIVersions exercises validation of matrix
// that requires alpha feature gate version in order to work.
func TestMatrixIncompatibleAPIVersions(t *testing.T) {
	tests := []struct {
		name            string
		requiredVersion string
		spec            PipelineSpec
	}{{
		name:            "matrix requires alpha - check tasks",
		requiredVersion: "alpha",
		spec: PipelineSpec{
			Tasks: PipelineTaskList{{
				Name:    "a-task",
				TaskRef: &TaskRef{Name: "a-task"},
				Matrix: &Matrix{
					Params: Params{{
						Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
					}}},
			}},
		},
	}, {
		name:            "matrix requires alpha - check finally tasks",
		requiredVersion: "alpha",
		spec: PipelineSpec{
			Tasks: PipelineTaskList{{
				Name:    "a-task",
				TaskRef: &TaskRef{Name: "a-task"},
			}},
			Finally: PipelineTaskList{{
				Name:    "b-task",
				TaskRef: &TaskRef{Name: "b-task"},
				Matrix: &Matrix{
					Params: Params{{
						Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
					}}},
			}},
		},
	}}
	versions := []string{"alpha", "stable"}
	for _, tt := range tests {
		for _, version := range versions {
			testName := fmt.Sprintf("(using %s) %s", version, tt.name)
			t.Run(testName, func(t *testing.T) {
				ps := tt.spec
				featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
					"enable-api-fields": version,
				})
				defaults := &config.Defaults{
					DefaultMaxMatrixCombinationsCount: 4,
				}
				cfg := &config.Config{
					FeatureFlags: featureFlags,
					Defaults:     defaults,
				}

				ctx := config.ToContext(context.Background(), cfg)

				ps.SetDefaults(ctx)
				err := ps.Validate(ctx)

				if tt.requiredVersion != version && err == nil {
					t.Fatalf("no error received even though version required is %q while feature gate is %q", tt.requiredVersion, version)
				}

				if tt.requiredVersion == version && err != nil {
					t.Fatalf("error received despite required version and feature gate matching %q: %v", version, err)
				}
			})
		}
	}
}

func Test_validateMatrix(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []PipelineTask
		wantErrs *apis.FieldError
	}{{
		name: "parameter in both matrix and params",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
			Params: Params{{
				Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
		}},
		wantErrs: apis.ErrMultipleOneOf("[0].matrix[foobar]", "[0].params[foobar]"),
	}, {
		name: "parameters unique in matrix and params",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
			Params: Params{{
				Name: "barfoo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
		}},
	}, {
		name: "parameters in matrix contain results references",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.foo-task.results.a-result)"}},
				}}},
		}, {
			Name:    "b-task",
			TaskRef: &TaskRef{Name: "b-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "b-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.bar-task.results.b-result)"}},
				}}},
		}},
	}, {
		name: "parameters in matrix contain whole array results references",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.foo-task.results.a-task-results[*])"}},
				}}},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
				"enable-api-fields": "alpha",
			})
			defaults := &config.Defaults{
				DefaultMaxMatrixCombinationsCount: 4,
			}
			cfg := &config.Config{
				FeatureFlags: featureFlags,
				Defaults:     defaults,
			}

			ctx := config.ToContext(context.Background(), cfg)
			if d := cmp.Diff(tt.wantErrs.Error(), validateMatrix(ctx, tt.tasks).Error()); d != "" {
				t.Errorf("validateMatrix() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func Test_validateResultsFromMatrixedPipelineTasksNotConsumed(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []PipelineTask
		finally  []PipelineTask
		wantErrs *apis.FieldError
	}{{
		name: "results from matrixed task consumed in tasks through parameters",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
		}, {
			Name:    "b-task",
			TaskRef: &TaskRef{Name: "b-task"},
			Params: Params{{
				Name: "b-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.a-task.results.a-result)"}},
			}},
		}},
		wantErrs: &apis.FieldError{
			Message: "invalid value: consuming results from matrixed task a-task is not allowed",
			Paths:   []string{"tasks[1]"},
		},
	}, {
		name: "results from matrixed task consumed in finally through parameters",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
		}},
		finally: PipelineTaskList{{
			Name:    "b-task",
			TaskRef: &TaskRef{Name: "b-task"},
			Params: Params{{
				Name: "b-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.a-task.results.a-result)"}},
			}},
		}},
		wantErrs: &apis.FieldError{
			Message: "invalid value: consuming results from matrixed task a-task is not allowed",
			Paths:   []string{"finally[0]"},
		},
	}, {
		name: "results from matrixed task consumed in tasks and finally through parameters",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
		}, {
			Name:    "b-task",
			TaskRef: &TaskRef{Name: "b-task"},
			Params: Params{{
				Name: "b-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.a-task.results.a-result)"}},
			}},
		}},
		finally: PipelineTaskList{{
			Name:    "c-task",
			TaskRef: &TaskRef{Name: "c-task"},
			Params: Params{{
				Name: "b-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.a-task.results.a-result)"}},
			}},
		}},
		wantErrs: &apis.FieldError{
			Message: "invalid value: consuming results from matrixed task a-task is not allowed",
			Paths:   []string{"tasks[1]", "finally[0]"},
		},
	}, {
		name: "results from matrixed task consumed in tasks through when expressions",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
		}, {
			Name:    "b-task",
			TaskRef: &TaskRef{Name: "b-task"},
			When: WhenExpressions{{
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"$(tasks.a-task.results.a-result)"},
			}},
		}},
		wantErrs: &apis.FieldError{
			Message: "invalid value: consuming results from matrixed task a-task is not allowed",
			Paths:   []string{"tasks[1]"},
		},
	}, {
		name: "results from matrixed task consumed in finally through when expressions",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
		}},
		finally: PipelineTaskList{{
			Name:    "b-task",
			TaskRef: &TaskRef{Name: "b-task"},
			When: WhenExpressions{{
				Input:    "$(tasks.a-task.results.a-result)",
				Operator: selection.In,
				Values:   []string{"foo", "bar"},
			}},
		}},
		wantErrs: &apis.FieldError{
			Message: "invalid value: consuming results from matrixed task a-task is not allowed",
			Paths:   []string{"finally[0]"},
		},
	}, {
		name: "results from matrixed task consumed in tasks and finally through when expressions",
		tasks: PipelineTaskList{{
			Name:    "a-task",
			TaskRef: &TaskRef{Name: "a-task"},
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
		}, {
			Name:    "b-task",
			TaskRef: &TaskRef{Name: "b-task"},
			When: WhenExpressions{{
				Input:    "$(tasks.a-task.results.a-result)",
				Operator: selection.In,
				Values:   []string{"foo", "bar"},
			}},
		}},
		finally: PipelineTaskList{{
			Name:    "c-task",
			TaskRef: &TaskRef{Name: "c-task"},
			When: WhenExpressions{{
				Input:    "foo",
				Operator: selection.In,
				Values:   []string{"$(tasks.a-task.results.a-result)"},
			}},
		}},
		wantErrs: &apis.FieldError{
			Message: "invalid value: consuming results from matrixed task a-task is not allowed",
			Paths:   []string{"tasks[1]", "finally[0]"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := cmp.Diff(tt.wantErrs.Error(), validateResultsFromMatrixedPipelineTasksNotConsumed(tt.tasks, tt.finally).Error()); d != "" {
				t.Errorf("validateResultsFromMatrixedPipelineTasksNotConsumed() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func getTaskSpec() TaskSpec {
	return TaskSpec{
		Steps: []Step{{
			Name: "foo", Image: "bar",
		}},
	}
}

func TestPipelineWithBetaFields(t *testing.T) {
	tts := []struct {
		name string
		spec PipelineSpec
	}{{
		name: "array indexing in Tasks",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
			},
			Tasks: []PipelineTask{{
				Name: "foo",
				Params: Params{
					{Name: "first-task-first-param", Value: *NewStructuredValues("$(params.first-param[0])")},
				},
				TaskRef: &TaskRef{Name: "foo"},
			}},
		},
	}, {
		name: "array indexing in Finally",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
			},
			Tasks: []PipelineTask{{
				Name:    "foo",
				TaskRef: &TaskRef{Name: "foo"},
			}},
			Finally: []PipelineTask{{
				Name: "bar",
				Params: Params{
					{Name: "first-task-first-param", Value: *NewStructuredValues("$(params.first-param[0])")},
				},
				TaskRef: &TaskRef{Name: "bar"},
			}},
		},
	}, {
		name: "pipeline tasks - use of resolver without the feature flag set",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "uses-resolver",
				TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Resolver: "bar"}},
			}},
		},
	}, {
		name: "pipeline tasks - use of resolver params without the feature flag set",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "uses-resolver-params",
				TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Params: Params{{}}}},
			}},
		},
	}, {
		name: "finally tasks - use of resolver without the feature flag set",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "uses-resolver",
				TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Resolver: "bar"}},
			}},
		},
	}, {
		name: "finally tasks - use of resolver params without the feature flag set",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name:    "uses-resolver-params",
				TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Params: Params{{}}}},
			}},
		},
	}, {
		name: "object params",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeObject, Properties: map[string]PropertySpec{}},
			},
			Tasks: []PipelineTask{{
				Name:    "foo",
				TaskRef: &TaskRef{Name: "foo"},
			}},
		},
	}, {
		name: "object params in Tasks",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name: "valid-pipeline-task",
				TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
					Steps:  []Step{{Image: "busybox", Script: "echo hello"}},
					Params: []ParamSpec{{Name: "my-object-param", Type: ParamTypeObject, Properties: map[string]PropertySpec{}}},
				}},
			}},
		},
	}, {
		name: "object params in Finally",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "foo",
				TaskRef: &TaskRef{Name: "foo"},
			}},
			Finally: []PipelineTask{{
				Name: "valid-finally-task",
				TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
					Steps:  []Step{{Image: "busybox", Script: "echo hello"}},
					Params: []ParamSpec{{Name: "my-object-param", Type: ParamTypeObject, Properties: map[string]PropertySpec{}}},
				}},
			}},
		},
	}, {
		name: "array results",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Results: []PipelineResult{{Name: "my-array-result", Type: ResultsTypeArray, Value: *NewStructuredValues("$(tasks.valid-pipeline-task.results.foo[*])")}},
		},
	}, {
		name: "array results in Tasks",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name: "valid-pipeline-task",
				TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
					Steps:   []Step{{Image: "busybox", Script: "echo hello"}},
					Results: []TaskResult{{Name: "my-array-result", Type: ResultsTypeArray}},
				}},
			}},
		},
	}, {
		name: "array results in Finally",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name: "valid-finally-task",
				TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
					Steps:   []Step{{Image: "busybox", Script: "echo hello"}},
					Results: []TaskResult{{Name: "my-array-result", Type: ResultsTypeArray}},
				}},
			}},
		},
	}, {
		name: "object results",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Results: []PipelineResult{{Name: "my-object-result", Type: ResultsTypeObject, Value: *NewStructuredValues("$(tasks.valid-pipeline-task.results.foo[*])")}},
		},
	}, {
		name: "object results in Tasks",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name: "valid-pipeline-task",
				TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
					Steps:   []Step{{Image: "busybox", Script: "echo hello"}},
					Results: []TaskResult{{Name: "my-object-result", Type: ResultsTypeObject, Properties: map[string]PropertySpec{}}},
				}},
			}},
		},
	}, {
		name: "object results in Finally",
		spec: PipelineSpec{
			Tasks: []PipelineTask{{
				Name:    "valid-pipeline-task",
				TaskRef: &TaskRef{Name: "foo-task"},
			}},
			Finally: []PipelineTask{{
				Name: "valid-finally-task",
				TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
					Steps:   []Step{{Image: "busybox", Script: "echo hello"}},
					Results: []TaskResult{{Name: "my-object-result", Type: ResultsTypeObject, Properties: map[string]PropertySpec{}}},
				}},
			}},
		},
	}}
	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := Pipeline{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: tt.spec}
			ctx := config.EnableStableAPIFields(context.Background())
			if err := pipeline.Validate(ctx); err == nil {
				t.Errorf("no error when using beta field when `enable-api-fields` is stable")
			}

			ctx = config.EnableBetaAPIFields(context.Background())
			if err := pipeline.Validate(ctx); err != nil {
				t.Errorf("unexpected error when using beta field: %s", err)
			}
		})
	}
}

func TestGetIndexingReferencesToArrayParams(t *testing.T) {
	for _, tt := range []struct {
		name string
		spec PipelineSpec
		want sets.String
	}{{
		name: "references in task params",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: ParamTypeString},
			},
			Tasks: []PipelineTask{{
				Params: Params{
					{Name: "first-task-first-param", Value: *NewStructuredValues("$(params.first-param[1])")},
					{Name: "first-task-second-param", Value: *NewStructuredValues("$(params.second-param[0])")},
					{Name: "first-task-third-param", Value: *NewStructuredValues("static value")},
				},
			}},
		},
		want: sets.NewString("$(params.first-param[1])", "$(params.second-param[0])"),
	}, {
		name: "references in when expression",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: ParamTypeString},
			},
			Tasks: []PipelineTask{{
				When: []WhenExpression{{
					Input:    "$(params.first-param[1])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[0])"},
				}},
			}},
		},
		want: sets.NewString("$(params.first-param[1])", "$(params.second-param[0])"),
	}, {
		name: "nested references in task params",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
			},
			Tasks: []PipelineTask{{
				Params: Params{
					{Name: "first-task-first-param", Value: *NewStructuredValues("$(input.workspace.$(params.first-param[0]))")},
					{Name: "first-task-second-param", Value: *NewStructuredValues("$(input.workspace.$(params.second-param[1]))")},
				},
			}},
		},
		want: sets.NewString("$(params.first-param[0])", "$(params.second-param[1])"),
	}, {
		name: "array parameter",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default", "array", "value")},
				{Name: "second-param", Type: ParamTypeArray},
			},
			Tasks: []PipelineTask{{
				Params: Params{
					{Name: "first-task-first-param", Value: *NewStructuredValues("firstelement", "$(params.first-param)")},
					{Name: "first-task-second-param", Value: *NewStructuredValues("firstelement", "$(params.second-param[0])")},
				},
			}},
		},
		want: sets.NewString("$(params.second-param[0])"),
	}, {
		name: "references in finally params",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: ParamTypeArray},
			},
			Finally: []PipelineTask{{
				Params: Params{
					{Name: "final-task-first-param", Value: *NewStructuredValues("$(params.first-param[0])")},
					{Name: "final-task-second-param", Value: *NewStructuredValues("$(params.second-param[1])")},
				},
			}},
		},
		want: sets.NewString("$(params.first-param[0])", "$(params.second-param[1])"),
	}, {
		name: "references in finally when expressions",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: ParamTypeArray},
			},
			Finally: []PipelineTask{{
				When: WhenExpressions{{
					Input:    "$(params.first-param[0])",
					Operator: selection.In,
					Values:   []string{"$(params.second-param[1])"},
				}},
			}},
		},
		want: sets.NewString("$(params.first-param[0])", "$(params.second-param[1])"),
	}, {
		name: "parameter references with bracket notation and special characters",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first.param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "second/param", Type: ParamTypeArray},
				{Name: "third.param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "fourth/param", Type: ParamTypeArray},
			},
			Tasks: []PipelineTask{{
				Params: Params{
					{Name: "first-task-first-param", Value: *NewStructuredValues(`$(params["first.param"][0])`)},
					{Name: "first-task-second-param", Value: *NewStructuredValues(`$(params["second.param"][0])`)},
					{Name: "first-task-third-param", Value: *NewStructuredValues(`$(params['third.param'][1])`)},
					{Name: "first-task-fourth-param", Value: *NewStructuredValues(`$(params['fourth/param'][1])`)},
					{Name: "first-task-fifth-param", Value: *NewStructuredValues("static value")},
				},
			}},
		},
		want: sets.NewString(`$(params["first.param"][0])`, `$(params["second.param"][0])`, `$(params['third.param'][1])`, `$(params['fourth/param'][1])`),
	}, {
		name: "single parameter in workspace subpath",
		spec: PipelineSpec{
			Params: []ParamSpec{
				{Name: "first-param", Type: ParamTypeArray, Default: NewStructuredValues("default-value", "default-value-again")},
				{Name: "second-param", Type: ParamTypeArray},
			},
			Tasks: []PipelineTask{{
				Params: Params{
					{Name: "first-task-first-param", Value: *NewStructuredValues("$(params.first-param[0])")},
					{Name: "first-task-second-param", Value: *NewStructuredValues("static value")},
				},
				Workspaces: []WorkspacePipelineTaskBinding{
					{
						Name:      "first-workspace",
						Workspace: "first-workspace",
						SubPath:   "$(params.second-param[1])",
					},
				},
			}},
		},
		want: sets.NewString("$(params.first-param[0])", "$(params.second-param[1])"),
	},
	} {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.spec.GetIndexingReferencesToArrayParams()
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("wrong array index references: %s", d)
			}
		})
	}
}
