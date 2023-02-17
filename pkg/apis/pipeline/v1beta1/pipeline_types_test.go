/*
Copyright 2021 The Tekton Authors

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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

func TestPipelineTask_ValidateName(t *testing.T) {
	pipelineTasks := []struct {
		name    string
		task    PipelineTask
		message string
	}{{
		name:    "pipeline task with empty task name",
		task:    PipelineTask{Name: ""},
		message: `invalid value ""`,
	}, {
		name:    "pipeline task with invalid task name",
		task:    PipelineTask{Name: "_foo"},
		message: `invalid value "_foo"`,
	}, {
		name:    "pipeline task with invalid task name (camel case)",
		task:    PipelineTask{Name: "fooTask"},
		message: `invalid value "fooTask"`,
	}}

	// expected error if a task name is not valid
	expectedError := apis.FieldError{
		Paths: []string{"name"},
		Details: "Pipeline Task name must be a valid DNS Label." +
			"For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
	}

	for _, tc := range pipelineTasks {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.task.ValidateName()
			if err == nil {
				t.Error("PipelineTask.ValidateName() did not return error for invalid pipeline task name")
			}
			// error message changes for each test as it includes the task name in the message
			expectedError.Message = tc.message
			if d := cmp.Diff(expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineTask.ValidateName() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_ValidateRefOrSpec(t *testing.T) {
	tests := []struct {
		name          string
		p             PipelineTask
		expectedError *apis.FieldError
	}{{
		name: "valid pipeline task - with taskRef only",
		p: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{},
		},
	}, {
		name: "valid pipeline task - with taskSpec only",
		p: PipelineTask{
			Name:     "foo",
			TaskSpec: &EmbeddedTask{},
		},
	}, {
		name: "invalid pipeline task missing taskRef and taskSpec",
		p: PipelineTask{
			Name: "foo",
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got neither`,
			Paths:   []string{"taskRef", "taskSpec"},
		},
	}, {
		name: "invalid pipeline task with both taskRef and taskSpec",
		p: PipelineTask{
			Name:     "foo",
			TaskRef:  &TaskRef{Name: "foo-task"},
			TaskSpec: &EmbeddedTask{TaskSpec: getTaskSpec()},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"taskRef", "taskSpec"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.validateRefOrSpec()
			if tt.expectedError == nil {
				if err != nil {
					t.Error("PipelineTask.validateRefOrSpec() returned error for valid pipeline task")
				}
			} else {
				if err == nil {
					t.Error("PipelineTask.validateRefOrSpec() did not return error for invalid pipeline task")
				}
				if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
					t.Errorf("PipelineTask.validateRefOrSpec() errors diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestPipelineTask_ValidateCustomTask(t *testing.T) {
	tests := []struct {
		name          string
		task          PipelineTask
		expectedError apis.FieldError
	}{{
		name: "custom task - taskRef without kind",
		task: PipelineTask{Name: "foo", TaskRef: &TaskRef{APIVersion: "example.dev/v0", Kind: "", Name: ""}},
		expectedError: apis.FieldError{
			Message: `invalid value: custom task ref must specify kind`,
			Paths:   []string{"taskRef.kind"},
		},
	}, {
		name: "custom task - taskSpec without kind",
		task: PipelineTask{Name: "foo", TaskSpec: &EmbeddedTask{
			TypeMeta: runtime.TypeMeta{
				APIVersion: "example.dev/v0",
				Kind:       "",
			},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: custom task spec must specify kind`,
			Paths:   []string{"taskSpec.kind"},
		},
	}, {
		name: "custom task - taskSpec without apiVersion",
		task: PipelineTask{Name: "foo", TaskSpec: &EmbeddedTask{
			TypeMeta: runtime.TypeMeta{
				APIVersion: "",
				Kind:       "some-kind",
			},
		}},
		expectedError: apis.FieldError{
			Message: `invalid value: custom task spec must specify apiVersion`,
			Paths:   []string{"taskSpec.apiVersion"},
		},
	}, {
		name: "custom task - taskRef without apiVersion",
		task: PipelineTask{Name: "foo", TaskRef: &TaskRef{APIVersion: "", Kind: "some-kind", Name: ""}},
		expectedError: apis.FieldError{
			Message: `invalid value: custom task ref must specify apiVersion`,
			Paths:   []string{"taskRef.apiVersion"},
		},
	}, {
		name: "custom task doesn't support pipeline resources",
		task: PipelineTask{
			Name:      "foo",
			Resources: &PipelineTaskResources{},
			TaskRef:   &TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: custom tasks do not support PipelineResources`,
			Paths:   []string{"resources"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.validateCustomTask()
			if err == nil {
				t.Error("PipelineTaskList.ValidateCustomTask() did not return error for invalid pipeline task")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineTaskList.ValidateCustomTask() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_ValidateBundle_Failure(t *testing.T) {
	tests := []struct {
		name          string
		p             PipelineTask
		expectedError apis.FieldError
	}{{
		name: "bundle - invalid reference",
		p: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "bar", Bundle: "invalid reference"},
		},
		expectedError: *apis.ErrInvalidValue("invalid bundle reference (could not parse reference: invalid reference)", "taskRef.bundle"),
	}, {
		name: "bundle - missing taskRef name",
		p: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Bundle: "valid-bundle"},
		},
		expectedError: *apis.ErrMissingField("taskRef.name"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.validateBundle()
			if err == nil {
				t.Error("PipelineTask.ValidateBundles() did not return error for invalid bundle in a pipelineTask")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("Pipeline.ValidateBundles() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_ValidateRegularTask_Success(t *testing.T) {
	tests := []struct {
		name            string
		tasks           PipelineTask
		enableAPIFields bool
		enableBundles   bool
	}{{
		name: "pipeline task - valid taskRef name",
		tasks: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "example.com/my-foo-task"},
		},
	}, {
		name: "pipeline task - valid taskSpec",
		tasks: PipelineTask{
			Name:     "foo",
			TaskSpec: &EmbeddedTask{TaskSpec: getTaskSpec()},
		},
	}, {
		name: "pipeline task - use of resolver",
		tasks: PipelineTask{
			TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Resolver: "bar"}},
		},
	}, {
		name: "pipeline task - use of params",
		tasks: PipelineTask{
			TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Params: []Param{}}},
		},
	}, {
		name: "pipeline task - use of bundle with the feature flag set",
		tasks: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "bar", Bundle: "docker.io/foo"},
		},
		enableBundles: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{
				FeatureFlags: &config.FeatureFlags{},
			}
			if tt.enableAPIFields {
				cfg.FeatureFlags.EnableAPIFields = config.AlphaAPIFields
			}
			if tt.enableBundles {
				cfg.FeatureFlags.EnableTektonOCIBundles = true
			}
			ctx = config.ToContext(ctx, cfg)
			ctx = config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, false)
			err := tt.tasks.validateTask(ctx)
			if err != nil {
				t.Errorf("PipelineTask.validateTask() returned error for valid pipeline task: %v", err)
			}
		})
	}
}

func TestPipelineTask_ValidateRegularTask_Failure(t *testing.T) {
	tests := []struct {
		name          string
		task          PipelineTask
		expectedError apis.FieldError
	}{{
		name: "pipeline task - invalid taskSpec",
		task: PipelineTask{
			Name:     "foo",
			TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{}},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"taskSpec.steps"},
		},
	}, {
		name: "pipeline task - invalid taskRef name",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "_foo-task"},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
			Paths:   []string{"taskRef.name"},
		},
	}, {
		name: "pipeline task - taskRef without name",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: ""},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: taskRef must specify name`,
			Paths:   []string{"taskRef.name"},
		},
	}, {
		name: "pipeline task - use of bundle without the feature flag set",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "bar", Bundle: "docker.io/foo"},
		},
		expectedError: *apis.ErrDisallowedFields("taskRef.bundle"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := config.SkipValidationDueToPropagatedParametersAndWorkspaces(context.Background(), false)
			err := tt.task.validateTask(ctx)
			if err == nil {
				t.Error("PipelineTask.validateTask() did not return error for invalid pipeline task")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineTask.validateTask() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_Validate_Failure(t *testing.T) {
	tests := []struct {
		name          string
		p             PipelineTask
		expectedError apis.FieldError
		wc            func(context.Context) context.Context
	}{{
		name: "invalid custom task without Kind",
		p: PipelineTask{
			Name:    "invalid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com"},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: custom task ref must specify kind`,
			Paths:   []string{"taskRef.kind"},
		},
	}, {
		name: "invalid bundle without bundle name",
		p: PipelineTask{
			Name:    "invalid-bundle",
			TaskRef: &TaskRef{Bundle: "bundle"},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"taskRef.name"},
		},
		wc: enableFeatures(t, []string{"enable-tekton-oci-bundles"}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			ctx = config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, false)
			err := tt.p.Validate(ctx)
			if err == nil {
				t.Error("PipelineTask.Validate() did not return error for invalid pipeline task")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineTask.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTaskList_Names(t *testing.T) {
	tasks := []PipelineTask{
		{Name: "task-1"},
		{Name: "task-2"},
	}
	expectedTaskNames := sets.String{}
	expectedTaskNames.Insert("task-1")
	expectedTaskNames.Insert("task-2")
	actualTaskNames := PipelineTaskList(tasks).Names()
	if d := cmp.Diff(expectedTaskNames, actualTaskNames); d != "" {
		t.Fatalf("Failed to get list of pipeline task names, diff: %s", diff.PrintWantGot(d))
	}
}

func TestPipelineTaskList_Deps(t *testing.T) {
	pipelines := []struct {
		name         string
		tasks        PipelineTaskList
		expectedDeps map[string][]string
	}{{
		name: "valid pipeline without any deps",
		tasks: []PipelineTask{
			{Name: "task-1"},
			{Name: "task-2"},
		},
		expectedDeps: map[string][]string{},
	}, {
		name: "valid pipeline with ordering deps - runAfter",
		tasks: []PipelineTask{
			{Name: "task-1"},
			{Name: "task-2", RunAfter: []string{"task-1"}},
		},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
		},
	}, {
		name: "valid pipeline with resource deps - Inputs",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2",
		}, {
			Name: "task-3",
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					From: []string{"task-1", "task-2"},
				}},
			}},
		},
		expectedDeps: map[string][]string{
			"task-3": {"task-1", "task-2"},
		},
	}, {
		name: "valid pipeline with Task Results deps",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2",
			Params: []Param{{
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-1.results.result)",
				}},
			}},
		},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
		},
	}, {
		name: "valid pipeline with Task Results in Matrix deps",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2",
			Matrix: &Matrix{
				Params: []Param{{
					Value: ParamValue{
						Type: ParamTypeArray,
						ArrayVal: []string{
							"$(tasks.task-1.results.result)",
						},
					}},
				}}},
		},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
		},
	}, {
		name: "valid pipeline with When Expressions deps",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2",
			WhenExpressions: WhenExpressions{{
				Input:    "$(tasks.task-1.results.result)",
				Operator: "in",
				Values:   []string{"foo"},
			}},
		}},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
		},
	}, {
		name: "valid pipeline with ordering deps and resource deps",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2", RunAfter: []string{"task-1"},
		}, {
			Name:     "task-3",
			RunAfter: []string{"task-1"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					From: []string{"task-1", "task-2"},
				}},
			},
		}, {
			Name:     "task-4",
			RunAfter: []string{"task-1"},
			Params: []Param{{
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-3.results.result)",
				}},
			},
		}, {
			Name:     "task-5",
			RunAfter: []string{"task-1"},
			WhenExpressions: WhenExpressions{{
				Input:    "$(tasks.task-4.results.result)",
				Operator: "in",
				Values:   []string{"foo"},
			}},
		}, {
			Name:     "task-6",
			RunAfter: []string{"task-1"},
			Matrix: &Matrix{
				Params: []Param{{
					Value: ParamValue{
						Type: ParamTypeArray,
						ArrayVal: []string{
							"$(tasks.task-2.results.result)",
							"$(tasks.task-5.results.result)",
						},
					}}},
			},
		}},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
			"task-3": {"task-1", "task-2"},
			"task-4": {"task-1", "task-3"},
			"task-5": {"task-1", "task-4"},
			"task-6": {"task-1", "task-2", "task-5"},
		},
	}, {
		name: "valid pipeline with ordering deps and resource deps - verify unique dependencies",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2", RunAfter: []string{"task-1"},
		}, {
			Name:     "task-3",
			RunAfter: []string{"task-1"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					From: []string{"task-1", "task-2"},
				}},
			},
		}, {
			Name:     "task-4",
			RunAfter: []string{"task-1", "task-3"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					From: []string{"task-1", "task-2"},
				}},
			},
			Params: []Param{{
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-2.results.result)",
				}}, {
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-3.results.result)",
				}},
			},
		}, {
			Name:     "task-5",
			RunAfter: []string{"task-1", "task-2", "task-3", "task-4"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					From: []string{"task-1", "task-2"},
				}},
			},
			Params: []Param{{
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-4.results.result)",
				}},
			},
			WhenExpressions: WhenExpressions{{
				Input:    "$(tasks.task-3.results.result)",
				Operator: "in",
				Values:   []string{"foo"},
			}, {
				Input:    "$(tasks.task-4.results.result)",
				Operator: "in",
				Values:   []string{"foo"},
			}},
		}, {
			Name:     "task-6",
			RunAfter: []string{"task-1", "task-2", "task-3", "task-4", "task-5"},
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					From: []string{"task-1", "task-2"},
				}},
			},
			Params: []Param{{
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-4.results.result)",
				}},
			},
			WhenExpressions: WhenExpressions{{
				Input:    "$(tasks.task-3.results.result)",
				Operator: "in",
				Values:   []string{"foo"},
			}, {
				Input:    "$(tasks.task-4.results.result)",
				Operator: "in",
				Values:   []string{"foo"},
			}},
			Matrix: &Matrix{
				Params: []Param{{
					Value: ParamValue{
						Type: ParamTypeArray,
						ArrayVal: []string{
							"$(tasks.task-2.results.result)",
							"$(tasks.task-5.results.result)",
						},
					}}},
			},
		}, {
			Name: "task-7",
			Resources: &PipelineTaskResources{
				Inputs: []PipelineTaskInputResource{{
					From: []string{"task-1", "task-1"},
				}},
			},
		}, {
			Name: "task-8",
			WhenExpressions: WhenExpressions{{
				Input:    "$(tasks.task-3.results.result1)",
				Operator: "in",
				Values:   []string{"foo"},
			}, {
				Input:    "$(tasks.task-3.results.result2)",
				Operator: "in",
				Values:   []string{"foo"},
			}},
		}, {
			Name: "task-9",
			Params: []Param{{
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-4.results.result1)",
				}}, {
				Value: ParamValue{
					Type:      "string",
					StringVal: "$(tasks.task-4.results.result2)",
				}},
			},
		}, {
			Name:     "task-10",
			RunAfter: []string{"task-1", "task-1", "task-1", "task-1"},
		}},
		expectedDeps: map[string][]string{
			"task-2":  {"task-1"},
			"task-3":  {"task-1", "task-2"},
			"task-4":  {"task-1", "task-2", "task-3"},
			"task-5":  {"task-1", "task-2", "task-3", "task-4"},
			"task-6":  {"task-1", "task-2", "task-3", "task-4", "task-5"},
			"task-7":  {"task-1"},
			"task-8":  {"task-3"},
			"task-9":  {"task-4"},
			"task-10": {"task-1"},
		},
	}}
	for _, tc := range pipelines {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(tc.expectedDeps, tc.tasks.Deps()); d != "" {
				t.Fatalf("Failed to get the right set of dependencies, diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_validateMatrix(t *testing.T) {
	tests := []struct {
		name     string
		pt       *PipelineTask
		wantErrs *apis.FieldError
	}{{
		name: "parameter duplicated in matrix and params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: []Param{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
			Params: []Param{{
				Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
		},
		wantErrs: apis.ErrMultipleOneOf("matrix[foobar]", "params[foobar]"),
	}, {
		name: "parameters unique in matrix and params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: []Param{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}}},
			Params: []Param{{
				Name: "barfoo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
		},
	}, {
		name: "parameters in matrix are strings",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: []Param{{
					Name: "foo", Value: ParamValue{Type: ParamTypeString, StringVal: "foo"},
				}, {
					Name: "bar", Value: ParamValue{Type: ParamTypeString, StringVal: "bar"},
				}}},
		},
		wantErrs: &apis.FieldError{
			Message: "invalid value: parameters of type array only are allowed in matrix",
			Paths:   []string{"matrix[foo]", "matrix[bar]"},
		},
	}, {
		name: "parameters in matrix are arrays",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: []Param{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}, {
					Name: "barfoo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
				}}},
		},
	}, {
		name: "parameters in include matrix are strings",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Include: []MatrixInclude{{
					Name: "build-1",
					Params: []Param{{
						Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"},
					}}},
				}},
		},
	}, {
		name: "parameters in include matrix are arrays",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Include: []MatrixInclude{{
					Name: "build-1",
					Params: []Param{{
						Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
					}, {
						Name: "barfoo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}}}},
				}}},
		},
		wantErrs: &apis.FieldError{
			Message: "invalid value: parameters of type string only are allowed in matrix",
			Paths:   []string{"matrix[barfoo]", "matrix[foobar]"},
		},
	}, {
		name: "parameters in matrix contain results references",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: []Param{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.foo-task.results.a-result)"}},
				}}},
		},
	}, {
		name: "include parameters in matrix contain results references",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Include: []MatrixInclude{{
					Name: "build-1",
					Params: []Param{{
						Name: "a-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.foo-task.results.a-result)"},
					}, {
						Name: "b-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.bar-task.results.b-result)"}},
					}}},
			},
		},
	},
		{
			name: "count of combinations of parameters in the matrix exceeds the maximum",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "platform", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
					}, {
						Name: "browser", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"chrome", "firefox", "safari"}},
					}}},
			},
			wantErrs: &apis.FieldError{
				Message: "expected 0 <= 9 <= 4",
				Paths:   []string{"matrix"},
			},
		},
		{
			name: "count of combinations of include parameters in the matrix exceeds the maximum",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Include: []MatrixInclude{{
						Name: "build-1",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
					}, {
						Name: "build-2",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-2"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile2"}}},
					}, {
						Name: "build-3",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-3"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile3"}}},
					}, {
						Name: "build-4",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-4"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile4"}}},
					}, {
						Name: "build-5",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-5"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile5"}}},
					}},
				},
			},
			wantErrs: &apis.FieldError{
				Message: "expected 0 <= 5 <= 4",
				Paths:   []string{"matrix"},
			},
		},
		{
			name: "count of combinations of include parameters in the matrix exceeds the maximum",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "GOARCH", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
					}, {
						Name: "version", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"go1.17", "go1.18.1"}},
					}},
					Include: []MatrixInclude{{
						Name: "common-package",
						Params: []Param{{
							Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
					}, {
						Name: "s390x-no-race",
						Params: []Param{{
							Name: "GOARCH", Value: ParamValue{Type: ParamTypeString, StringVal: "linux/s390x"},
						}, {
							Name: "flags", Value: ParamValue{Type: ParamTypeString, StringVal: "-cover -v"}}},
					}, {
						Name: "go117-context",
						Params: []Param{{
							Name: "version", Value: ParamValue{Type: ParamTypeString, StringVal: "go1.17"},
						}, {
							Name: "context", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/go117/context"}}},
					}, {
						Name: "non-existent-arch",
						Params: []Param{{
							Name: "GOARCH", Value: ParamValue{Type: ParamTypeString, StringVal: "I-do-not-exist"}}},
					}},
				},
			},
			wantErrs: &apis.FieldError{
				Message: "expected 0 <= 7 <= 4",
				Paths:   []string{"matrix"},
			},
		}, {
			name: "count of combinations of parameters in the matrix equals the maximum",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "platform", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac"}},
					}, {
						Name: "browser", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"chrome", "firefox"}},
					}}},
			},
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
			if d := cmp.Diff(tt.wantErrs.Error(), tt.pt.validateMatrix(ctx).Error()); d != "" {
				t.Errorf("PipelineTask.validateMatrix() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_GetMatrixCombinationsCount(t *testing.T) {
	tests := []struct {
		name                    string
		pt                      *PipelineTask
		matrixCombinationsCount int
	}{
		{
			name: "combinations count is zero",
			pt: &PipelineTask{
				Name: "task",
			},
			matrixCombinationsCount: 0,
		}, {
			name: "combinations count is one from one parameter",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
					}}},
			},
			matrixCombinationsCount: 1,
		}, {
			name: "combinations count is one from two parameters",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
					}, {
						Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar"}},
					}}},
			},
			matrixCombinationsCount: 1,
		}, {
			name: "combinations count is two from one parameter",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
					}}},
			},
			matrixCombinationsCount: 2,
		}, {
			name: "combinations count is nine",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
					}, {
						Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
					}}},
			},
			matrixCombinationsCount: 9,
		}, {
			name: "combinations count is large",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
					}, {
						Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
					}, {
						Name: "quz", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"q", "u", "x"}},
					}, {
						Name: "xyzzy", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"x", "y", "z", "z", "y"}},
					}}},
			},
			matrixCombinationsCount: 135,
		}, {
			name: "explicit combinations in the matrix",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Include: []MatrixInclude{{
						Name: "build-1",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
					}, {
						Name: "build-2",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-2"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile2"}}},
					}, {
						Name: "build-3",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-3"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile3"}},
						}},
					}},
			},
			matrixCombinationsCount: 3,
		}, {
			name: "explicit combinations with include params and matrix params",
			pt: &PipelineTask{
				Name: "task",
				Matrix: &Matrix{
					Params: []Param{{
						Name: "GOARCH", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
					}, {
						Name: "version", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"go1.17", "go1.18.1"}},
					}},
					Include: []MatrixInclude{{
						Name: "common-package",
						Params: []Param{{
							Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
					}, {
						Name: "s390x-no-race",
						Params: []Param{{
							Name: "GOARCH", Value: ParamValue{Type: ParamTypeString, StringVal: "linux/s390x"},
						}, {
							Name: "flags", Value: ParamValue{Type: ParamTypeString, StringVal: "-cover -v"}}},
					}, {
						Name: "go117-context",
						Params: []Param{{
							Name: "version", Value: ParamValue{Type: ParamTypeString, StringVal: "go1.17"},
						}, {
							Name: "context", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/go117/context"}}},
					}, {
						Name: "non-existent-arch",
						Params: []Param{{
							Name: "GOARCH", Value: ParamValue{Type: ParamTypeString, StringVal: "I-do-not-exist"}}},
					}},
				}},
			matrixCombinationsCount: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := cmp.Diff(tt.matrixCombinationsCount, tt.pt.GetMatrixCombinationsCount()); d != "" {
				t.Errorf("PipelineTask.GetMatrixCombinationsCount() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_ValidateEmbeddedOrType(t *testing.T) {
	testCases := []struct {
		name          string
		pt            PipelineTask
		expectedError *apis.FieldError
	}{
		{
			name: "just apiVersion and kind",
			pt: PipelineTask{
				TaskSpec: &EmbeddedTask{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "something",
						Kind:       "whatever",
					},
				},
			},
		}, {
			name: "just steps",
			pt: PipelineTask{
				TaskSpec: &EmbeddedTask{
					TaskSpec: TaskSpec{
						Steps: []Step{{
							Name:  "foo",
							Image: "bar",
						}},
					},
				},
			},
		}, {
			name: "apiVersion and steps",
			pt: PipelineTask{
				TaskSpec: &EmbeddedTask{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "something",
					},
					TaskSpec: TaskSpec{
						Steps: []Step{{
							Name:  "foo",
							Image: "bar",
						}},
					},
				},
			},
			expectedError: &apis.FieldError{
				Message: "taskSpec.apiVersion cannot be specified when using taskSpec.steps",
				Paths:   []string{"taskSpec.apiVersion"},
			},
		}, {
			name: "kind and steps",
			pt: PipelineTask{
				TaskSpec: &EmbeddedTask{
					TypeMeta: runtime.TypeMeta{
						Kind: "something",
					},
					TaskSpec: TaskSpec{
						Steps: []Step{{
							Name:  "foo",
							Image: "bar",
						}},
					},
				},
			},
			expectedError: &apis.FieldError{
				Message: "taskSpec.kind cannot be specified when using taskSpec.steps",
				Paths:   []string{"taskSpec.kind"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pt.validateEmbeddedOrType()
			if err == nil && tc.expectedError != nil {
				t.Fatalf("PipelineTask.validateEmbeddedOrType() did not return expected error '%s'", tc.expectedError.Error())
			}
			if err != nil {
				if tc.expectedError == nil {
					t.Fatalf("PipelineTask.validateEmbeddedOrType() returned unexpected error '%s'", err.Error())
				}
				if d := cmp.Diff(tc.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
					t.Errorf("PipelineTask.validateEmbeddedOrType() errors diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestPipelineTask_IsMatrixed(t *testing.T) {
	type arg struct {
		*Matrix
	}
	testCases := []struct {
		name     string
		arg      arg
		expected bool
	}{
		{
			name: "nil matrix",
			arg: arg{
				Matrix: nil,
			},
			expected: false,
		},
		{
			name: "empty matrix",
			arg: arg{
				Matrix: &Matrix{},
			},
			expected: false,
		},
		{
			name: "matrixed with params",
			arg: arg{
				Matrix: &Matrix{
					Params: []Param{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
				},
			},
			expected: true,
		}, {
			name: "matrixed with include",
			arg: arg{
				Matrix: &Matrix{
					Include: []MatrixInclude{{
						Name: "build-1",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
					}},
				},
			},
			expected: true,
		}, {
			name: "matrixed with params and include",
			arg: arg{
				Matrix: &Matrix{
					Params: []Param{{
						Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
					}},
					Include: []MatrixInclude{{
						Name: "common-package",
						Params: []Param{{
							Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
					}},
				},
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pt := &PipelineTask{
				Matrix: tc.arg.Matrix,
			}
			isMatrixed := pt.IsMatrixed()
			if isMatrixed != tc.expected {
				t.Errorf("PipelineTask.IsMatrixed() return bool: %v, but wanted: %v", isMatrixed, tc.expected)
			}
		})
	}
}

func TestPipelineTask_MatrixHasParams(t *testing.T) {
	type arg struct {
		*Matrix
	}
	testCases := []struct {
		name     string
		arg      arg
		expected bool
	}{
		{
			name: "nil matrix",
			arg: arg{
				Matrix: nil,
			},
			expected: false,
		},
		{
			name: "empty matrix",
			arg: arg{
				Matrix: &Matrix{},
			},
			expected: false,
		},
		{
			name: "matrixed with params",
			arg: arg{
				Matrix: &Matrix{
					Params: []Param{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
				},
			},
			expected: true,
		}, {
			name: "matrixed with include",
			arg: arg{
				Matrix: &Matrix{
					Include: []MatrixInclude{{
						Name: "build-1",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
					}},
				},
			},
			expected: false,
		}, {
			name: "matrixed with params and include",
			arg: arg{
				Matrix: &Matrix{
					Params: []Param{{
						Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
					}},
					Include: []MatrixInclude{{
						Name: "common-package",
						Params: []Param{{
							Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
					}},
				},
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matrix := tc.arg.Matrix
			matrixHasParams := matrix.MatrixHasParams()
			if matrixHasParams != tc.expected {
				t.Errorf("Matrix.MatrixHasParams() return bool: %v, but wanted: %v", matrixHasParams, tc.expected)
			}
		})
	}
}

func TestPipelineTask_MatrixHasInclude(t *testing.T) {
	type arg struct {
		*Matrix
	}
	testCases := []struct {
		name     string
		arg      arg
		expected bool
	}{
		{
			name: "nil matrix",
			arg: arg{
				Matrix: nil,
			},
			expected: false,
		},
		{
			name: "empty matrix",
			arg: arg{
				Matrix: &Matrix{},
			},
			expected: false,
		},
		{
			name: "matrixed with params",
			arg: arg{
				Matrix: &Matrix{
					Params: []Param{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
				},
			},
			expected: false,
		}, {
			name: "matrixed with include",
			arg: arg{
				Matrix: &Matrix{
					Include: []MatrixInclude{{
						Name: "build-1",
						Params: []Param{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
					}},
				},
			},
			expected: true,
		}, {
			name: "matrixed with params and include",
			arg: arg{
				Matrix: &Matrix{
					Params: []Param{{
						Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
					}},
					Include: []MatrixInclude{{
						Name: "common-package",
						Params: []Param{{
							Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
					}},
				},
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matrix := tc.arg.Matrix
			matrixHasInclude := matrix.MatrixHasInclude()
			if matrixHasInclude != tc.expected {
				t.Errorf("Matrix.MatrixHasParams() return bool: %v, but wanted: %v", matrixHasInclude, tc.expected)
			}
		})
	}
}
