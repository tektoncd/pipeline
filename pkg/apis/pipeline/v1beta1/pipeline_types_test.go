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
		name: "pipeline task - use of resolver with the feature flag set",
		tasks: PipelineTask{
			TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Resolver: "bar"}},
		},
		enableAPIFields: true,
	}, {
		name: "pipeline task - use of resource with the feature flag set",
		tasks: PipelineTask{
			TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Resource: []ResolverParam{{}}}},
		},
		enableAPIFields: true,
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
			Paths:   []string{"name"},
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
		expectedError: *apis.ErrDisallowedFields("taskref.bundle"),
	}, {
		name: "pipeline task - use of resolver without the feature flag set",
		task: PipelineTask{
			TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Resolver: "bar"}},
		},
		expectedError: *apis.ErrDisallowedFields("taskref.resolver"),
	}, {
		name: "pipeline task - use of resource without the feature flag set",
		task: PipelineTask{
			TaskRef: &TaskRef{Name: "boo", ResolverRef: ResolverRef{Resource: []ResolverParam{{}}}},
		},
		expectedError: *apis.ErrDisallowedFields("taskref.resource"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.validateTask(context.Background())
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
		wc: enableFeatures(t, []string{"enable-custom-tasks"}),
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
		name: "valid pipeline with resource deps - Task Results",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2",
			Params: []Param{{
				Value: ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.task-1.results.result)",
				}},
			}},
		},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
		},
	}, {
		name: "valid pipeline with resource deps - Task Results in Matrix",
		tasks: []PipelineTask{{
			Name: "task-1",
		}, {
			Name: "task-2",
			Matrix: []Param{{
				Value: ArrayOrString{
					Type: ParamTypeArray,
					ArrayVal: []string{
						"$(tasks.task-1.results.result)",
					},
				}},
			}},
		},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
		},
	}, {
		name: "valid pipeline with resource deps - When Expressions",
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
				Value: ArrayOrString{
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
			Matrix: []Param{{
				Value: ArrayOrString{
					Type: ParamTypeArray,
					ArrayVal: []string{
						"$(tasks.task-2.results.result)",
						"$(tasks.task-5.results.result)",
					},
				}},
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
				Value: ArrayOrString{
					Type:      "string",
					StringVal: "$(tasks.task-2.results.result)",
				}}, {
				Value: ArrayOrString{
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
				Value: ArrayOrString{
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
				Value: ArrayOrString{
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
			Matrix: []Param{{
				Value: ArrayOrString{
					Type: ParamTypeArray,
					ArrayVal: []string{
						"$(tasks.task-2.results.result)",
						"$(tasks.task-5.results.result)",
					},
				}},
			},
		}},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
			"task-3": {"task-1", "task-2"},
			"task-4": {"task-1", "task-2", "task-3"},
			"task-5": {"task-1", "task-2", "task-3", "task-4"},
			"task-6": {"task-1", "task-2", "task-3", "task-4", "task-5"},
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

func TestPipelineTaskList_Validate(t *testing.T) {
	tests := []struct {
		name          string
		tasks         PipelineTaskList
		path          string
		expectedError *apis.FieldError
		wc            func(context.Context) context.Context
	}{{
		name: "validate all three valid custom task, bundle, and regular task",
		tasks: PipelineTaskList{{
			Name:    "valid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com", Kind: "custom"},
		}, {
			Name:    "valid-bundle",
			TaskRef: &TaskRef{Bundle: "bundle", Name: "bundle"},
		}, {
			Name:    "valid-task",
			TaskRef: &TaskRef{Name: "task"},
		}},
		path: "tasks",
		wc:   enableFeatures(t, []string{"enable-custom-tasks", "enable-tekton-oci-bundles"}),
	}, {
		name: "validate list of tasks with valid custom task and bundle but invalid regular task",
		tasks: PipelineTaskList{{
			Name:    "valid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com", Kind: "custom"},
		}, {
			Name:    "valid-bundle",
			TaskRef: &TaskRef{Bundle: "bundle", Name: "bundle"},
		}, {
			Name:    "invalid-task-without-name",
			TaskRef: &TaskRef{Name: ""},
		}},
		path:          "tasks",
		expectedError: apis.ErrGeneric(`invalid value: taskRef must specify name`, "tasks[2].taskRef.name"),
		wc:            enableFeatures(t, []string{"enable-custom-tasks", "enable-tekton-oci-bundles"}),
	}, {
		name: "validate list of tasks with valid custom task but invalid bundle and invalid regular task",
		tasks: PipelineTaskList{{
			Name:    "valid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com", Kind: "custom"},
		}, {
			Name:    "invalid-bundle",
			TaskRef: &TaskRef{Bundle: "bundle"},
		}, {
			Name:    "invalid-task-without-name",
			TaskRef: &TaskRef{Name: ""},
		}},
		path: "tasks",
		expectedError: apis.ErrGeneric(`invalid value: taskRef must specify name`, "tasks[2].taskRef.name").Also(
			apis.ErrGeneric(`missing field(s)`, "tasks[1].taskRef.name")),
		wc: enableFeatures(t, []string{"enable-custom-tasks", "enable-tekton-oci-bundles"}),
	}, {
		name: "validate all three invalid tasks - custom task, bundle and regular task",
		tasks: PipelineTaskList{{
			Name:    "invalid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com"},
		}, {
			Name:    "invalid-bundle",
			TaskRef: &TaskRef{Bundle: "bundle"},
		}, {
			Name:    "invalid-task",
			TaskRef: &TaskRef{Name: ""},
		}},
		path: "tasks",
		expectedError: apis.ErrGeneric(`invalid value: taskRef must specify name`, "tasks[2].taskRef.name").Also(
			apis.ErrGeneric(`missing field(s)`, "tasks[1].taskRef.name")).Also(
			apis.ErrGeneric(`invalid value: custom task ref must specify kind`, "tasks[0].taskRef.kind")),
		wc: enableFeatures(t, []string{"enable-custom-tasks", "enable-tekton-oci-bundles"}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			taskNames := sets.String{}
			err := tt.tasks.Validate(ctx, taskNames, tt.path)
			if tt.expectedError != nil && err == nil {
				t.Error("PipelineTaskList.Validate() did not return error for invalid pipeline tasks")
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineTaskList.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_validateMatrix(t *testing.T) {
	tests := []struct {
		name           string
		pt             *PipelineTask
		embeddedStatus string
		wantErrs       *apis.FieldError
	}{{
		name: "parameter duplicated in matrix and params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foobar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
			Params: []Param{{
				Name: "foobar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
		},
		wantErrs: apis.ErrMultipleOneOf("matrix[foobar]", "params[foobar]"),
	}, {
		name: "parameters unique in matrix and params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foobar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
			Params: []Param{{
				Name: "barfoo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
		},
	}, {
		name: "parameters in matrix are strings",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foo", Value: ArrayOrString{Type: ParamTypeString, StringVal: "foo"},
			}, {
				Name: "bar", Value: ArrayOrString{Type: ParamTypeString, StringVal: "bar"},
			}},
		},
		wantErrs: &apis.FieldError{
			Message: "invalid value: parameters of type array only are allowed in matrix",
			Paths:   []string{"matrix[foo]", "matrix[bar]"},
		},
	}, {
		name: "parameters in matrix are arrays",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foobar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}, {
				Name: "barfoo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
		},
	}, {
		name: "parameters in matrix contain results references",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "a-param", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.foo-task.results.a-result)"}},
			}},
		},
	}, {
		name: "count of combinations of parameters in the matrix exceeds the maximum",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "platform", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}, {
				Name: "browser", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"chrome", "firefox", "safari"}},
			}},
		},
		wantErrs: &apis.FieldError{
			Message: "expected 0 <= 9 <= 4",
			Paths:   []string{"matrix"},
		},
	}, {
		name: "count of combinations of parameters in the matrix equals the maximum",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "platform", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac"}},
			}, {
				Name: "browser", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"chrome", "firefox"}},
			}},
		},
	}, {
		name: "pipeline has a matrix but embedded status is full",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foobar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}, {
				Name: "barfoo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
		},
		embeddedStatus: config.FullEmbeddedStatus,
		wantErrs: &apis.FieldError{
			Message: "matrix requires \"embedded-status\" feature gate to be \"minimal\" but it is \"full\"",
		},
	}, {
		name: "pipeline has a matrix but embedded status is both",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foobar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}, {
				Name: "barfoo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
		},
		embeddedStatus: config.BothEmbeddedStatus,
		wantErrs: &apis.FieldError{
			Message: "matrix requires \"embedded-status\" feature gate to be \"minimal\" but it is \"both\"",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.embeddedStatus == "" {
				tt.embeddedStatus = config.MinimalEmbeddedStatus
			}
			featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
				"enable-api-fields": "alpha",
				"embedded-status":   tt.embeddedStatus,
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
	}{{
		name: "combinations count is zero",
		pt: &PipelineTask{
			Name: "task",
		},
		matrixCombinationsCount: 0,
	}, {
		name: "combinations count is one from one parameter",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
			}},
		},
		matrixCombinationsCount: 1,
	}, {
		name: "combinations count is one from two parameters",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
			}, {
				Name: "bar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"bar"}},
			}},
		},
		matrixCombinationsCount: 1,
	}, {
		name: "combinations count is two from one parameter",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
		},
		matrixCombinationsCount: 2,
	}, {
		name: "combinations count is nine",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}},
		},
		matrixCombinationsCount: 9,
	}, {
		name: "combinations count is large",
		pt: &PipelineTask{
			Name: "task",
			Matrix: []Param{{
				Name: "foo", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}, {
				Name: "quz", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"q", "u", "x"}},
			}, {
				Name: "xyzzy", Value: ArrayOrString{Type: ParamTypeArray, ArrayVal: []string{"x", "y", "z", "z", "y"}},
			}},
		},
		matrixCombinationsCount: 135,
	}}
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
