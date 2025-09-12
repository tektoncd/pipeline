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
	"encoding/hex"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestPipelineTask_OnError(t *testing.T) {
	tests := []struct {
		name          string
		p             PipelineTask
		expectedError *apis.FieldError
		wc            func(context.Context) context.Context
	}{{
		name: "valid PipelineTask with onError:continue",
		p: PipelineTask{
			Name:    "foo",
			OnError: PipelineTaskContinue,
			TaskRef: &TaskRef{Name: "foo"},
		},
		wc: cfgtesting.EnableBetaAPIFields,
	}, {
		name: "valid PipelineTask with onError:stopAndFail",
		p: PipelineTask{
			Name:    "foo",
			OnError: PipelineTaskStopAndFail,
			TaskRef: &TaskRef{Name: "foo"},
		},
		wc: cfgtesting.EnableBetaAPIFields,
	}, {
		name: "invalid OnError value",
		p: PipelineTask{
			Name:    "foo",
			OnError: "invalid-val",
			TaskRef: &TaskRef{Name: "foo"},
		},
		expectedError: apis.ErrInvalidValue("invalid-val", "OnError", "PipelineTask OnError must be either \"continue\" or \"stopAndFail\""),
		wc:            cfgtesting.EnableBetaAPIFields,
	}, {
		name: "OnError:stopAndFail and retries coexist - success",
		p: PipelineTask{
			Name:    "foo",
			OnError: PipelineTaskStopAndFail,
			Retries: 1,
			TaskRef: &TaskRef{Name: "foo"},
		},
		wc: cfgtesting.EnableBetaAPIFields,
	}, {
		name: "OnError:continue and retries coexists - failure",
		p: PipelineTask{
			Name:    "foo",
			OnError: PipelineTaskContinue,
			Retries: 1,
			TaskRef: &TaskRef{Name: "foo"},
		},
		expectedError: apis.ErrGeneric("PipelineTask OnError cannot be set to \"continue\" when Retries is greater than 0"),
		wc:            cfgtesting.EnableBetaAPIFields,
	}, {
		name: "setting OnError in stable API version - failure",
		p: PipelineTask{
			Name:    "foo",
			OnError: PipelineTaskContinue,
			TaskRef: &TaskRef{Name: "foo"},
		},
		expectedError: apis.ErrGeneric("OnError requires \"enable-api-fields\" feature gate to be \"alpha\" or \"beta\" but it is \"stable\""),
		wc:            cfgtesting.EnableStableAPIFields,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.p.Validate(ctx)
			if tt.expectedError == nil {
				if err != nil {
					t.Error("PipelineTask.Validate() returned error for valid pipeline task")
				}
			} else {
				if err == nil {
					t.Error("PipelineTask.Validate() did not return error for invalid pipeline task with OnError")
				}
				if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
					t.Errorf("PipelineTask.Validate() errors diff %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestPipelineTask_ValidateRefOrSpec(t *testing.T) {
	tests := []struct {
		name          string
		p             PipelineTask
		expectedError *apis.FieldError
		wc            func(ctx context.Context) context.Context
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
		name: "valid pipeline task - with pipelineRef only",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:        "foo",
			PipelineRef: &PipelineRef{},
		},
	}, {
		name: "valid pipeline task - with pipelineSpec only",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			PipelineSpec: &PipelineSpec{},
		},
	}, {
		name: "invalid pipeline task missing taskRef or taskSpec",
		p: PipelineTask{
			Name: "foo",
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got neither`,
			Paths:   []string{"taskRef", "taskSpec"},
		},
	}, {
		name: "invalid pipeline task missing taskRef or taskSpec or pipelineRef(alpha api) or pipelineSpec(alpha api)",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name: "foo",
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got neither`,
			Paths:   []string{"taskRef", "taskSpec", "pipelineRef", "pipelineSpec"},
		},
	}, {
		name: "invalid pipeline task with both taskRef and taskSpec",
		p: PipelineTask{
			Name:     "foo",
			TaskRef:  &TaskRef{Name: "foo-task"},
			TaskSpec: &EmbeddedTask{TaskSpec: getTaskSpec()},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskRef", "taskSpec"},
		},
	}, {
		name: "invalid pipeline task with both taskRef and pipelineRef",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:        "foo",
			TaskRef:     &TaskRef{Name: "foo-task"},
			PipelineRef: &PipelineRef{Name: "foo-pipeline"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskRef", "pipelineRef"},
		},
	}, {
		name: "invalid pipeline task with both taskRef and pipelineSpec",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			TaskRef:      &TaskRef{Name: "foo-task"},
			PipelineSpec: &PipelineSpec{Description: "description"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskRef", "pipelineSpec"},
		},
	}, {
		name: "invalid pipeline task with both taskSpec and pipelineRef",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:        "foo",
			TaskSpec:    &EmbeddedTask{TaskSpec: getTaskSpec()},
			PipelineRef: &PipelineRef{Name: "foo-pipeline"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskSpec", "pipelineRef"},
		},
	}, {
		name: "invalid pipeline task with both taskSpec and pipelineSpec",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			TaskSpec:     &EmbeddedTask{TaskSpec: getTaskSpec()},
			PipelineSpec: &PipelineSpec{Description: "description"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskSpec", "pipelineSpec"},
		},
	}, {
		name: "invalid pipeline task with both pipelineRef and pipelineSpec",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			PipelineRef:  &PipelineRef{Name: "foo-pipeline"},
			PipelineSpec: &PipelineSpec{Description: "description"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"pipelineRef", "pipelineSpec"},
		},
	}, {
		name: "invalid pipeline task with taskRef and taskSpec and pipelineRef",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:        "foo",
			TaskRef:     &TaskRef{Name: "foo-task"},
			TaskSpec:    &EmbeddedTask{TaskSpec: getTaskSpec()},
			PipelineRef: &PipelineRef{Name: "foo-pipeline"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskRef", "taskSpec", "pipelineRef"},
		},
	}, {
		name: "invalid pipeline task with taskRef and taskSpec and pipelineSpec",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			TaskRef:      &TaskRef{Name: "foo-task"},
			TaskSpec:     &EmbeddedTask{TaskSpec: getTaskSpec()},
			PipelineSpec: &PipelineSpec{Description: "description"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskRef", "taskSpec", "pipelineSpec"},
		},
	}, {
		name: "invalid pipeline task with taskRef and pipelineRef and pipelineSpec",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			TaskRef:      &TaskRef{Name: "foo-task"},
			PipelineRef:  &PipelineRef{Name: "foo-pipeline"},
			PipelineSpec: &PipelineSpec{Description: "description"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskRef", "pipelineRef", "pipelineSpec"},
		},
	}, {
		name: "invalid pipeline task with taskSpec and pipelineRef and pipelineSpec",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			TaskSpec:     &EmbeddedTask{TaskSpec: getTaskSpec()},
			PipelineRef:  &PipelineRef{Name: "foo-pipeline"},
			PipelineSpec: &PipelineSpec{Description: "description"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskSpec", "pipelineRef", "pipelineSpec"},
		},
	}, {
		name: "invalid pipeline task with taskRef and taskSpec and pipelineRef and pipelineSpec",
		wc:   cfgtesting.EnableAlphaAPIFields,
		p: PipelineTask{
			Name:         "foo",
			TaskRef:      &TaskRef{Name: "foo-task"},
			TaskSpec:     &EmbeddedTask{TaskSpec: getTaskSpec()},
			PipelineRef:  &PipelineRef{Name: "foo-pipeline"},
			PipelineSpec: &PipelineSpec{Description: "description"},
		},
		expectedError: &apis.FieldError{
			Message: `expected exactly one, got multiple`,
			Paths:   []string{"taskRef", "taskSpec", "pipelineRef", "pipelineSpec"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.p.validateRefOrSpec(ctx)
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

// TestPipelineTask_ValidateRefOrSpec_APIVersionsCompatibility validates `pipelineRef` and `pipelineSpec`
// are guarded behind the appropriate feature flags i.e. alpha for now.
func TestPipelineTask_ValidateRefOrSpec_APIVersionsCompatibility(t *testing.T) {
	tests := []struct {
		name    string
		pt      PipelineTask
		wc      func(ctx context.Context) context.Context
		wantErr *apis.FieldError
	}{
		{
			name: "pipelineRef requires alpha version",
			pt: PipelineTask{
				PipelineRef: &PipelineRef{},
			},
			wc: cfgtesting.EnableAlphaAPIFields,
		}, {
			name: "pipelineSpec requires alpha version",
			pt: PipelineTask{
				PipelineSpec: &PipelineSpec{},
			},
			wc: cfgtesting.EnableAlphaAPIFields,
		}, {
			name: "pipelineRef not allowed with beta version",
			pt: PipelineTask{
				PipelineRef: &PipelineRef{},
			},
			wc:      cfgtesting.EnableBetaAPIFields,
			wantErr: apis.ErrGeneric("pipelineRef requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"beta\""),
		}, {
			name: "pipelineSpec not allowed with beta version",
			pt: PipelineTask{
				PipelineSpec: &PipelineSpec{},
			},
			wc:      cfgtesting.EnableBetaAPIFields,
			wantErr: apis.ErrGeneric("pipelineSpec requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"beta\""),
		}, {
			name: "pipelineRef not allowed with stable version",
			pt: PipelineTask{
				PipelineRef: &PipelineRef{},
			},
			wc:      cfgtesting.EnableStableAPIFields,
			wantErr: apis.ErrGeneric("pipelineRef requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
		}, {
			name: "pipelineSpec not allowed with beta version",
			pt: PipelineTask{
				PipelineSpec: &PipelineSpec{},
			},
			wc:      cfgtesting.EnableStableAPIFields,
			wantErr: apis.ErrGeneric("pipelineSpec requires \"enable-api-fields\" feature gate to be \"alpha\" but it is \"stable\""),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			if test.wc != nil {
				ctx = test.wc(ctx)
			}
			err := test.pt.validateRefOrSpec(ctx)
			if test.wantErr != nil {
				if d := cmp.Diff(test.wantErr.Error(), err.Error()); d != "" {
					t.Error(diff.PrintWantGot(d))
				}
			} else {
				if err := test.pt.validateRefOrSpec(ctx); err != nil {
					t.Fatalf("pipelineTask.validateRefOrSpec() error = %v", err)
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

func TestPipelineTask_ValidateRegularTask_Success(t *testing.T) {
	tests := []struct {
		name      string
		tasks     PipelineTask
		configMap map[string]string
	}{{
		name: "pipeline task - valid taskRef name",
		tasks: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "example.com/my-foo-task"},
		},
		configMap: map[string]string{"enable-api-fields": "stable"},
	}, {
		name: "pipeline task - valid taskSpec",
		tasks: PipelineTask{
			Name:     "foo",
			TaskSpec: &EmbeddedTask{TaskSpec: getTaskSpec()},
		},
		configMap: map[string]string{"enable-api-fields": "stable"},
	}, {
		name: "pipeline task - valid taskSpec with param enum",
		tasks: PipelineTask{
			Name: "foo",
			TaskSpec: &EmbeddedTask{
				TaskSpec: TaskSpec{
					Steps: []Step{
						{
							Name:  "foo",
							Image: "bar",
						},
					},
					Params: []ParamSpec{
						{
							Name: "param1",
							Type: ParamTypeString,
							Enum: []string{"v1", "v2"},
						},
					},
				},
			},
		},
		configMap: map[string]string{"enable-param-enum": "true"},
	}, {
		name: "pipeline task - use of resolver",
		tasks: PipelineTask{
			TaskRef: &TaskRef{ResolverRef: ResolverRef{Resolver: "bar"}},
		},
		configMap: map[string]string{"enable-api-fields": "beta"},
	}, {
		name: "pipeline task - use of params",
		tasks: PipelineTask{
			TaskRef: &TaskRef{ResolverRef: ResolverRef{Resolver: "bar", Params: Params{}}},
		},
		configMap: map[string]string{"enable-api-fields": "beta"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := cfgtesting.SetFeatureFlags(t.Context(), t, tt.configMap)
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
		configMap     map[string]string
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
			Message: `missing field(s)`,
			Paths:   []string{"taskRef.name"},
		},
	}, {
		name: "pipeline task - taskRef with resolver and k8s style name",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "foo", ResolverRef: ResolverRef{Resolver: "git"}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: invalid URI for request`,
			Paths:   []string{"taskRef.name"},
		},
		configMap: map[string]string{"enable-concise-resolver-syntax": "true"},
	}, {
		name: "pipeline task - taskRef with url-like name without enable-concise-resolver-syntax",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "https://foo.com/bar"},
		},
		expectedError: *apis.ErrMissingField("taskRef.resolver").Also(&apis.FieldError{
			Message: `feature flag enable-concise-resolver-syntax should be set to true to use concise resolver syntax`,
			Paths:   []string{"taskRef"},
		}),
	}, {
		name: "pipeline task - taskRef without enable-concise-resolver-syntax",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "https://foo.com/bar", ResolverRef: ResolverRef{Resolver: "git"}},
		},
		expectedError: apis.FieldError{
			Message: `feature flag enable-concise-resolver-syntax should be set to true to use concise resolver syntax`,
			Paths:   []string{"taskRef"},
		},
	}, {
		name: "pipeline task - taskRef with url-like name without resolver",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "https://foo.com/bar"},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"taskRef.resolver"},
		},
		configMap: map[string]string{"enable-concise-resolver-syntax": "true"},
	}, {
		name: "pipeline task - taskRef with name and params",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{Name: "https://foo/bar", ResolverRef: ResolverRef{Resolver: "git", Params: Params{{Name: "foo", Value: ParamValue{StringVal: "bar"}}}}},
		},
		expectedError: apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"taskRef.name", "taskRef.params"},
		},
		configMap: map[string]string{"enable-concise-resolver-syntax": "true"},
	}, {
		name: "pipeline task - taskRef with resolver params but no resolver",
		task: PipelineTask{
			Name:    "foo",
			TaskRef: &TaskRef{ResolverRef: ResolverRef{Params: Params{{Name: "foo", Value: ParamValue{StringVal: "bar"}}}}},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"taskRef.resolver"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := cfgtesting.SetFeatureFlags(t.Context(), t, tt.configMap)
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
		name: "custom task reference in taskref missing apiversion Kind",
		p: PipelineTask{
			Name:    "invalid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com"},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: custom task ref must specify kind`,
			Paths:   []string{"taskRef.kind"},
		},
	}, {
		name: "custom task reference in taskspec missing kind",
		p: PipelineTask{Name: "foo", TaskSpec: &EmbeddedTask{
			TypeMeta: runtime.TypeMeta{
				APIVersion: "example.com",
			},
		}},
		expectedError: *apis.ErrInvalidValue("custom task spec must specify kind", "taskSpec.kind"),
	}, {
		name:          "custom task reference in taskref missing apiversion",
		p:             PipelineTask{Name: "foo", TaskRef: &TaskRef{Kind: "Example", Name: ""}},
		expectedError: *apis.ErrInvalidValue("custom task ref must specify apiVersion", "taskRef.apiVersion"),
	}, {
		name: "custom task reference in taskspec missing apiversion",
		p: PipelineTask{Name: "foo", TaskSpec: &EmbeddedTask{
			TypeMeta: runtime.TypeMeta{
				Kind: "Example",
			},
		}},
		expectedError: *apis.ErrInvalidValue("custom task spec must specify apiVersion", "taskSpec.apiVersion"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
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
		name: "valid pipeline with Task Results deps",
		tasks: []PipelineTask{
			{
				Name: "task-1",
			}, {
				Name: "task-2",
				Params: Params{
					{
						Value: ParamValue{
							Type:      "string",
							StringVal: "$(tasks.task-1.results.result)",
						},
					},
				},
			},
		},
		expectedDeps: map[string][]string{
			"task-2": {"task-1"},
		},
	}, {
		name: "valid pipeline with Task Results in Matrix deps",
		tasks: []PipelineTask{
			{
				Name: "task-1",
			}, {
				Name: "task-2",
				Matrix: &Matrix{
					Params: Params{
						{
							Value: ParamValue{
								Type: ParamTypeArray,
								ArrayVal: []string{
									"$(tasks.task-1.results.result)",
								},
							},
						},
					},
				},
			},
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
		name: "validate all valid custom task, and regular task",
		tasks: PipelineTaskList{{
			Name:    "valid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com", Kind: "custom"},
		}, {
			Name:    "valid-task",
			TaskRef: &TaskRef{Name: "task"},
		}},
		path: "tasks",
	}, {
		name: "validate list of tasks with valid custom task and invalid regular task",
		tasks: PipelineTaskList{{
			Name:    "valid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com", Kind: "custom"},
		}, {
			Name:    "invalid-task-without-name",
			TaskRef: &TaskRef{Name: ""},
		}},
		path:          "tasks",
		expectedError: apis.ErrGeneric(`missing field(s)`, "tasks[1].taskRef.name"),
	}, {
		name: "validate all invalid tasks - custom task and regular task",
		tasks: PipelineTaskList{{
			Name:    "invalid-custom-task",
			TaskRef: &TaskRef{APIVersion: "example.com"},
		}, {
			Name:    "invalid-task",
			TaskRef: &TaskRef{Name: ""},
		}},
		path: "tasks",
		expectedError: apis.ErrGeneric(`missing field(s)`, "tasks[1].taskRef.name").Also(
			apis.ErrGeneric(`invalid value: custom task ref must specify kind`, "tasks[0].taskRef.kind")),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
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

func TestPipelineTask_ValidateMatrix(t *testing.T) {
	tests := []struct {
		name     string
		pt       *PipelineTask
		wantErrs *apis.FieldError
		tasks    []PipelineTask
	}{{
		name: "parameter duplicated in matrix.params and pipelinetask.params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: Params{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}},
			},
			Params: Params{{
				Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
		},
		wantErrs: apis.ErrMultipleOneOf("matrix[foobar]", "params[foobar]"),
	}, {
		name: "parameter duplicated in matrix.include.params and pipelinetask.params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Include: IncludeParamsList{
					{
						Name: "duplicate-param",
						Params: Params{{
							Name: "duplicate", Value: ParamValue{Type: ParamTypeString, StringVal: "foo"},
						}},
					},
				},
			},
			Params: Params{{
				Name: "duplicate", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}},
		},
		wantErrs: apis.ErrMultipleOneOf("matrix[duplicate]", "params[duplicate]"),
	}, {
		name: "duplicate parameters in matrix.params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: Params{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}, {
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo-1", "bar-1"}},
				}},
			},
		},
		wantErrs: &apis.FieldError{
			Message: `parameter names must be unique, the parameter "foobar" is also defined at`,
			Paths:   []string{"matrix.params[1].name"},
		},
	}, {
		name: "parameters unique in matrix and params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: Params{{
					Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
				}},
			},
			Params: Params{{
				Name: "barfoo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
		},
	}, {
		name: "duplicate parameters in matrix.include.params",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Include: IncludeParamsList{
					{
						Name: "invalid-include",
						Params: Params{{
							Name: "foobar", Value: ParamValue{Type: ParamTypeString, StringVal: "foo"},
						}, {
							Name: "foobar", Value: ParamValue{Type: ParamTypeString, StringVal: "foo-1"},
						}},
					},
				},
			},
		},
		wantErrs: &apis.FieldError{
			Message: `parameter names must be unique, the parameter "foobar" is also defined at`,
			Paths:   []string{"matrix.include[0].params[1].name"},
		},
	}, {
		name: "parameters in matrix contain references to param arrays",
		pt: &PipelineTask{
			Name: "task",
			Params: Params{{
				Name: "foobar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}, {
				Name: "barfoo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar", "foo"}},
			}},
			Matrix: &Matrix{
				Params: Params{{
					Name: "foo", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.foobar[*])"},
				}, {
					Name: "bar", Value: ParamValue{Type: ParamTypeString, StringVal: "$(params.barfoo[*])"},
				}},
			},
		},
	}, {
		name: "parameters in matrix contain result references",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: Params{{
					Name: "a-param", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"$(tasks.foo-task.results.a-result)"}},
				}},
			},
		},
	}, {
		name: "count of combinations of parameters in the matrix exceeds the maximum",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: Params{{
					Name: "platform", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
				}, {
					Name: "browser", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"chrome", "firefox", "safari"}},
				}},
			},
		},
		wantErrs: &apis.FieldError{
			Message: "expected 0 <= 9 <= 4",
			Paths:   []string{"matrix"},
		},
	}, {
		name: "count of combinations of parameters in the matrix equals the maximum",
		pt: &PipelineTask{
			Name: "task",
			Matrix: &Matrix{
				Params: Params{{
					Name: "platform", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac"}},
				}, {
					Name: "browser", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"chrome", "firefox"}},
				}},
			},
		},
	}, {
		name: "valid matrix emitting string results consumed in aggregate by another pipelineTask",
		pt: &PipelineTask{
			Name:    "task-consuming-results",
			TaskRef: &TaskRef{Name: "echoarrayurl"},
			Params: Params{{
				Name: "b-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.matrix-emitting-results-embedded.results.array-result[*])"},
			}},
		},
		tasks: PipelineTaskList{},
	}, {
		name: "valid matrix emitting string results consumed in aggregate by another pipelineTask (embedded taskSpec)",
		pt: &PipelineTask{
			Name: "task-consuming-results",
			TaskSpec: &EmbeddedTask{TaskSpec: TaskSpec{
				Params: ParamSpecs{{
					Name: "url", Type: "array",
				}},
				Steps: []Step{{
					Name:  "use-environments",
					Image: "bash:latest",
					Args:  []string{"$(params.url[*])"},
					Script: `for arg in "$@"; do
							echo "URL: $arg"
								done`,
				}},
			}},
			Params: Params{{
				Name: "b-param", Value: ParamValue{Type: ParamTypeString, StringVal: "$(tasks.matrix-emitting-results.results.report-url[*])"},
			}},
		},
		tasks: PipelineTaskList{},
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
			ctx := config.ToContext(t.Context(), cfg)
			if d := cmp.Diff(tt.wantErrs.Error(), tt.pt.validateMatrix(ctx).Error()); d != "" {
				t.Errorf("PipelineTask.validateMatrix() errors diff %s", diff.PrintWantGot(d))
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
			name: "just steps with DisplayName",
			pt: PipelineTask{
				TaskSpec: &EmbeddedTask{
					TaskSpec: TaskSpec{
						Steps: []Step{{
							Name:        "foo",
							DisplayName: "step DisplayName",
							Image:       "bar",
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
					Params: Params{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
				},
			},
			expected: true,
		},
		{
			name: "matrixed with include",
			arg: arg{
				Matrix: &Matrix{
					Include: IncludeParamsList{{
						Name: "build-1",
						Params: Params{{
							Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
						}, {
							Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"},
						}},
					}},
				},
			},
			expected: true,
		},
		{
			name: "matrixed with params and include",
			arg: arg{
				Matrix: &Matrix{
					Params: Params{{
						Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
					}},
					Include: IncludeParamsList{{
						Name: "common-package",
						Params: Params{{
							Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"},
						}},
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

func TestEmbeddedTask_IsCustomTask(t *testing.T) {
	tests := []struct {
		name string
		et   *EmbeddedTask
		want bool
	}{
		{
			name: "not a custom task - APIVersion and Kind are not set",
			et:   &EmbeddedTask{},
			want: false,
		}, {
			name: "not a custom task - APIVersion is not set",
			et: &EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					Kind: "Example",
				},
			},
			want: false,
		}, {
			name: "not a custom task - Kind is not set",
			et: &EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "example/v0",
				},
			},
			want: false,
		}, {
			name: "custom task - APIVersion and Kind are set",
			et: &EmbeddedTask{
				TypeMeta: runtime.TypeMeta{
					Kind:       "Example",
					APIVersion: "example/v0",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.et.IsCustomTask(); got != tt.want {
				t.Errorf("IsCustomTask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPipelineChecksum(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *Pipeline
	}{{
		name: "pipeline ignore uid",
		pipeline: &Pipeline{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "Pipeline",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        "pipeline",
				Namespace:   "pipeline-ns",
				UID:         "abc",
				Labels:      map[string]string{"label": "foo"},
				Annotations: map[string]string{"foo": "bar"},
			},
			Spec: PipelineSpec{},
		},
	}, {
		name: "pipeline ignore system annotations",
		pipeline: &Pipeline{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "Pipeline",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pipeline",
				Namespace: "pipeline-ns",
				UID:       "abc",
				Labels:    map[string]string{"label": "foo"},
				Annotations: map[string]string{
					"foo":                       "bar",
					"kubectl-client-side-apply": "client",
					"kubectl.kubernetes.io/last-applied-configuration": "config",
				},
			},
			Spec: PipelineSpec{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sha, err := tt.pipeline.Checksum()
			if err != nil {
				t.Fatalf("Error computing checksum: %v", err)
			}

			if d := cmp.Diff("ef400089e645c69a588e71fe629ce2a989743e303c058073b0829c6c6338ab8a", hex.EncodeToString(sha)); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
