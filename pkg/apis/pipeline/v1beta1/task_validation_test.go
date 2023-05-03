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

package v1beta1_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var validSteps = []v1beta1.Step{{
	Name:  "mystep",
	Image: "myimage",
}}

var invalidSteps = []v1beta1.Step{{
	Name:  "replaceImage",
	Image: "myimage",
}}

func TestTaskValidate(t *testing.T) {
	tests := []struct {
		name string
		t    *v1beta1.Task
		wc   func(context.Context) context.Context
	}{{
		name: "valid task",
		t: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "task"},
			Spec: v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "my-step",
					Image: "my-image",
					Script: `
					#!/usr/bin/env  bash
					echo hello`,
				}},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.t.Validate(ctx)
			if err != nil {
				t.Errorf("Task.Validate() returned error for valid Task: %v", err)
			}
		})
	}
}

func TestTaskSpecValidatePropagatedParamsAndWorkspaces(t *testing.T) {
	type fields struct {
		Params       []v1beta1.ParamSpec
		Steps        []v1beta1.Step
		StepTemplate *v1beta1.StepTemplate
		Workspaces   []v1beta1.WorkspaceDeclaration
		Results      []v1beta1.TaskResult
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "propagating params valid step with script",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:  "propagatingparams",
				Image: "my-image",
				Script: `
				#!/usr/bin/env bash
				$(params.message)`,
			}},
		},
	}, {
		name: "propagating object params valid step with script skip validation",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:  "propagatingobjectparams",
				Image: "my-image",
				Script: `
				#!/usr/bin/env bash
				$(params.message.foo)`,
			}},
		},
	}, {
		name: "propagating params valid step with args",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:    "propagatingparams",
				Image:   "my-image",
				Command: []string{"$(params.command)"},
				Args:    []string{"$(params.message)"},
			}},
		},
	}, {
		name: "propagating object params valid step with args",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:    "propagatingobjectparams",
				Image:   "my-image",
				Command: []string{"$(params.command.foo)"},
				Args:    []string{"$(params.message.bar)"},
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1beta1.TaskSpec{
				Params:       tt.fields.Params,
				Steps:        tt.fields.Steps,
				StepTemplate: tt.fields.StepTemplate,
				Workspaces:   tt.fields.Workspaces,
				Results:      tt.fields.Results,
			}
			ctx := context.Background()
			ts.SetDefaults(ctx)
			if err := ts.Validate(ctx); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestTaskSpecValidate(t *testing.T) {
	type fields struct {
		Params       []v1beta1.ParamSpec
		Steps        []v1beta1.Step
		StepTemplate *v1beta1.StepTemplate
		Workspaces   []v1beta1.WorkspaceDeclaration
		Results      []v1beta1.TaskResult
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "unnamed steps",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "myimage",
			}, {
				Image: "myotherimage",
			}},
		},
	}, {
		name: "valid params type implied",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Description: "param",
				Default:     v1beta1.NewStructuredValues("default"),
			}},
			Steps: validSteps,
		},
	}, {
		name: "valid params type explicit",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Type:        v1beta1.ParamTypeString,
				Description: "param",
				Default:     v1beta1.NewStructuredValues("default"),
			}, {
				Name:        "myobj",
				Type:        v1beta1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {},
					"key2": {},
				},
				Default: v1beta1.NewObject(map[string]string{
					"key1": "var1",
					"key2": "var2",
				}),
			}, {
				Name:        "myobjWithoutDefault",
				Type:        v1beta1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {},
					"key2": {},
				},
			}, {
				Name:        "myobjWithDefaultPartialKeys",
				Type:        v1beta1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {},
					"key2": {},
				},
				Default: v1beta1.NewObject(map[string]string{
					"key1": "default",
				}),
			}},
			Steps: validSteps,
		},
	}, {
		name: "valid template variable",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
			}, {
				Name: "foo-is-baz",
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "url",
				Args:       []string{"--flag=$(params.baz) && $(params.foo-is-baz)"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
	}, {
		name: "valid array template variable",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
				Type: v1beta1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1beta1.ParamTypeArray,
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "myimage",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"$(params.baz)", "middle string", "$(params.foo-is-baz)"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
	}, {
		name: "valid object template variable",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "gitrepo",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Steps: []v1beta1.Step{{
				Name:       "do-the-clone",
				Image:      "some-git-image",
				Args:       []string{"-url=$(params.gitrepo.url)", "-commit=$(params.gitrepo.commit)"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
	}, {
		name: "valid star array template variable",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
				Type: v1beta1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1beta1.ParamTypeArray,
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "myimage",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"$(params.baz[*])", "middle string", "$(params.foo-is-baz[*])"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
	}, {
		name: "valid results path variable in script",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:  "step-name",
				Image: "my-image",
				Script: `
				#!/usr/bin/env bash
				date | tee $(results.a-result.path)`,
			}},
			Results: []v1beta1.TaskResult{{Name: "a-result"}},
		},
	}, {
		name: "valid path variable for legacy credential helper (aka creds-init)",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:  "mystep",
				Image: "echo",
				Args:  []string{"$(credentials.path)"},
			}},
		},
	}, {
		name: "step template included in validation",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:    "astep",
				Command: []string{"echo"},
				Args:    []string{"hello"},
			}},
			StepTemplate: &v1beta1.StepTemplate{
				Image: "some-image",
			},
		},
	}, {
		name: "valid step with script",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Script: `
				#!/usr/bin/env bash
				hello world`,
			}},
		},
	}, {
		name: "valid step with parameterized script",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
			}, {
				Name: "foo-is-baz",
			}},
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Script: `
					#!/usr/bin/env bash
					hello $(params.baz)`,
			}},
		},
	}, {
		name: "valid step with script and args",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Script: `
				#!/usr/bin/env  bash
				hello $1`,
			}},
		},
	}, {
		name: "valid step with volumeMount under /tekton/home",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "myimage",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "foo",
					MountPath: "/tekton/home",
				}},
			}},
		},
	}, {
		name: "valid workspace",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:        "foo-workspace",
				Description: "my great workspace",
				MountPath:   "some/path",
			}},
		},
	}, {
		name: "valid result",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
			}},
			Results: []v1beta1.TaskResult{{
				Name:        "MY-RESULT",
				Description: "my great result",
			}},
		},
	}, {
		name: "valid result type string",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
			}},
			Results: []v1beta1.TaskResult{{
				Name:        "MY-RESULT",
				Type:        "string",
				Description: "my great result",
			}},
		},
	}, {
		name: "valid result type array",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
			}},
			Results: []v1beta1.TaskResult{{
				Name:        "MY-RESULT",
				Type:        v1beta1.ResultsTypeArray,
				Description: "my great result",
			}},
		},
	}, {
		name: "valid result type object",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
			}},
			Results: []v1beta1.TaskResult{{
				Name:        "MY-RESULT",
				Type:        v1beta1.ResultsTypeObject,
				Description: "my great result",
				Properties: map[string]v1beta1.PropertySpec{
					"url":    {"string"},
					"commit": {"string"},
				},
			}},
		},
	}, {
		name: "the spec of object type parameter misses the definition of properties",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Type:        v1beta1.ParamTypeObject,
				Description: "param",
			}},
			Steps: validSteps,
		},
	}, {
		name: "valid task name context",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Script: `
				#!/usr/bin/env  bash
				hello "$(context.task.name)"`,
			}},
		},
	}, {
		name: "valid task retry count context",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Script: `
				#!/usr/bin/env  bash
				retry count "$(context.task.retry-count)"`,
			}},
		},
	}, {
		name: "valid taskrun name context",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Script: `
				#!/usr/bin/env  bash
				hello "$(context.taskRun.name)"`,
			}},
		},
	}, {
		name: "valid taskrun uid context",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Script: `
				#!/usr/bin/env  bash
				hello "$(context.taskRun.uid)"`,
			}},
		},
	}, {
		name: "valid context",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Script: `
				#!/usr/bin/env  bash
				hello "$(context.taskRun.namespace)"`,
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1beta1.TaskSpec{
				Params:       tt.fields.Params,
				Steps:        tt.fields.Steps,
				StepTemplate: tt.fields.StepTemplate,
				Workspaces:   tt.fields.Workspaces,
				Results:      tt.fields.Results,
			}
			ctx := context.Background()
			ts.SetDefaults(ctx)
			if err := ts.Validate(ctx); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestTaskValidateError(t *testing.T) {
	type fields struct {
		Params []v1beta1.ParamSpec
		Steps  []v1beta1.Step
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "inexistent param variable",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:  "mystep",
				Image: "myimage",
				Args:  []string{"--flag=$(params.inexistent)"},
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "--flag=$(params.inexistent)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "object used in a string field",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "gitrepo",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Steps: []v1beta1.Step{{
				Name:       "do-the-clone",
				Image:      "$(params.gitrepo)",
				Args:       []string{"echo"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo)"`,
			Paths:   []string{"spec.steps[0].image"},
		},
	}, {
		name: "object star used in a string field",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "gitrepo",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Steps: []v1beta1.Step{{
				Name:       "do-the-clone",
				Image:      "$(params.gitrepo[*])",
				Args:       []string{"echo"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo[*])"`,
			Paths:   []string{"spec.steps[0].image"},
		},
	}, {
		name: "object used in a field that can accept array type",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "gitrepo",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Steps: []v1beta1.Step{{
				Name:       "do-the-clone",
				Image:      "myimage",
				Args:       []string{"$(params.gitrepo)"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "object star used in a field that can accept array type",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "gitrepo",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Steps: []v1beta1.Step{{
				Name:       "do-the-clone",
				Image:      "some-git-image",
				Args:       []string{"$(params.gitrepo[*])"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo[*])"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "non-existent individual key of an object param is used in task step",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "gitrepo",
				Type: v1beta1.ParamTypeObject,
				Properties: map[string]v1beta1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Steps: []v1beta1.Step{{
				Name:       "do-the-clone",
				Image:      "some-git-image",
				Args:       []string{"$(params.gitrepo.non-exist-key)"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.gitrepo.non-exist-key)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "Inexistent param variable in volumeMount with existing",
		fields: fields{
			Params: []v1beta1.ParamSpec{
				{
					Name:        "foo",
					Description: "param",
					Default:     v1beta1.NewStructuredValues("default"),
				},
			},
			Steps: []v1beta1.Step{{
				Name:  "mystep",
				Image: "myimage",
				VolumeMounts: []corev1.VolumeMount{{
					Name: "$(params.inexistent)-foo",
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.inexistent)-foo"`,
			Paths:   []string{"spec.steps[0].volumeMount[0].name"},
		},
	}, {
		name: "Inexistent param variable with existing",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "foo",
				Description: "param",
				Default:     v1beta1.NewStructuredValues("default"),
			}},
			Steps: []v1beta1.Step{{
				Name:  "mystep",
				Image: "myimage",
				Args:  []string{"$(params.foo) && $(params.inexistent)"},
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.foo) && $(params.inexistent)"`,
			Paths:   []string{"spec.steps[0].args[0]"},
		},
	}, {
		name: "invalid step - invalid onError usage - set to a parameter which does not exist in the task",
		fields: fields{
			Steps: []v1beta1.Step{{
				OnError: "$(params.CONTINUE)",
				Image:   "image",
				Args:    []string{"arg"},
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.CONTINUE)"`,
			Paths:   []string{"spec.steps[0].onError"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: v1beta1.TaskSpec{
					Params: tt.fields.Params,
					Steps:  tt.fields.Steps,
				}}
			ctx := config.EnableAlphaAPIFields(context.Background())
			task.SetDefaults(ctx)
			err := task.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", task)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskSpecValidateError(t *testing.T) {
	type fields struct {
		Params       []v1beta1.ParamSpec
		Steps        []v1beta1.Step
		Volumes      []corev1.Volume
		StepTemplate *v1beta1.StepTemplate
		Workspaces   []v1beta1.WorkspaceDeclaration
		Results      []v1beta1.TaskResult
		Resources    *v1beta1.TaskResources
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "empty spec",
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"steps"},
		},
	}, {
		name: "no step",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "validparam",
				Type:        v1beta1.ParamTypeString,
				Description: "parameter",
				Default:     v1beta1.NewStructuredValues("default"),
			}},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"steps"},
		},
	}, {
		name: "step script refers to nonexistent result",
		fields: fields{
			Steps: []v1beta1.Step{{
				Name:  "step-name",
				Image: "my-image",
				Script: `
				#!/usr/bin/env bash
				date | tee $(results.non-exist.path)`,
			}},
			Results: []v1beta1.TaskResult{{Name: "a-result"}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "\n\t\t\t\t#!/usr/bin/env bash\n\t\t\t\tdate | tee $(results.non-exist.path)"`,
			Paths:   []string{"steps[0].script"},
		},
	}, {
		name: "invalid param name format",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "_validparam1",
				Description: "valid param name format",
			}, {
				Name:        "valid_param2",
				Description: "valid param name format",
			}, {
				Name:        "",
				Description: "invalid param name format",
			}, {
				Name:        "a^b",
				Description: "invalid param name format",
			}, {
				Name:        "0ab",
				Description: "invalid param name format",
			}, {
				Name:        "f oo",
				Description: "invalid param name format",
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: fmt.Sprintf("The format of following array and string variable names is invalid: %s", []string{"", "0ab", "a^b", "f oo"}),
			Paths:   []string{"params"},
			Details: "String/Array Names: \nMust only contain alphanumeric characters, hyphens (-), underscores (_), and dots (.)\nMust begin with a letter or an underscore (_)",
		},
	}, {
		name: "invalid object param format - object param name and key name shouldn't contain dots.",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "invalid.name1",
				Description: "object param name contains dots",
				Properties: map[string]v1beta1.PropertySpec{
					"invalid.key1": {},
					"mykey2":       {},
				},
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: fmt.Sprintf("Object param name and key name format is invalid: %v", map[string][]string{
				"invalid.name1": {"invalid.key1"},
			}),
			Paths:   []string{"params"},
			Details: "Object Names: \nMust only contain alphanumeric characters, hyphens (-), underscores (_) \nMust begin with a letter or an underscore (_)",
		},
	}, {
		name: "duplicated param names",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "foo",
				Type:        v1beta1.ParamTypeString,
				Description: "parameter",
				Default:     v1beta1.NewStructuredValues("value1"),
			}, {
				Name:        "foo",
				Type:        v1beta1.ParamTypeString,
				Description: "parameter",
				Default:     v1beta1.NewStructuredValues("value2"),
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `parameter appears more than once`,
			Paths:   []string{"params[foo]"},
		},
	}, {
		name: "invalid param type",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "validparam",
				Type:        v1beta1.ParamTypeString,
				Description: "parameter",
				Default:     v1beta1.NewStructuredValues("default"),
			}, {
				Name:        "param-with-invalid-type",
				Type:        "invalidtype",
				Description: "invalidtypedesc",
				Default:     v1beta1.NewStructuredValues("default"),
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value: invalidtype`,
			Paths:   []string{"params.param-with-invalid-type.type"},
		},
	}, {
		name: "param mismatching default/type 1",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Type:        v1beta1.ParamTypeArray,
				Description: "param",
				Default:     v1beta1.NewStructuredValues("default"),
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `"array" type does not match default value's type: "string"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "param mismatching default/type 2",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Type:        v1beta1.ParamTypeString,
				Description: "param",
				Default:     v1beta1.NewStructuredValues("default", "array"),
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "param mismatching default/type 3",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Type:        v1beta1.ParamTypeArray,
				Description: "param",
				Default: v1beta1.NewObject(map[string]string{
					"key1": "var1",
					"key2": "var2",
				}),
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `"array" type does not match default value's type: "object"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "param mismatching default/type 4",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Type:        v1beta1.ParamTypeObject,
				Description: "param",
				Properties:  map[string]v1beta1.PropertySpec{"key1": {}},
				Default:     v1beta1.NewStructuredValues("var"),
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: `"object" type does not match default value's type: "string"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "PropertySpec type is set with unsupported type",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "task",
				Type:        v1beta1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1beta1.PropertySpec{
					"key1": {Type: "number"},
					"key2": {Type: "string"},
				},
			}},
			Steps: validSteps,
		},
		expectedError: apis.FieldError{
			Message: fmt.Sprintf("The value type specified for these keys %v is invalid", []string{"key1"}),
			Paths:   []string{"params.task.properties"},
		},
	}, {
		name: "invalid step",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:        "validparam",
				Type:        v1beta1.ParamTypeString,
				Description: "parameter",
				Default:     v1beta1.NewStructuredValues("default"),
			}},
			Steps: []v1beta1.Step{},
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"steps"},
		},
	}, {
		name: "invalid step name",
		fields: fields{
			Steps: invalidSteps,
		},
		expectedError: apis.FieldError{
			Message: `invalid value "replaceImage"`,
			Paths:   []string{"steps[0].name"},
			Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
		},
	}, {
		name: "array used in unaccepted field",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
				Type: v1beta1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1beta1.ParamTypeArray,
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "$(params.baz)",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"$(params.baz)", "middle string", "url"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.baz)"`,
			Paths:   []string{"steps[0].image"},
		},
	}, {
		name: "array star used in unaccepted field",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
				Type: v1beta1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1beta1.ParamTypeArray,
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "$(params.baz[*])",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"$(params.baz)", "middle string", "url"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.baz[*])"`,
			Paths:   []string{"steps[0].image"},
		},
	}, {
		name: "array star used illegaly in script field",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
				Type: v1beta1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1beta1.ParamTypeArray,
			}},
			Steps: []v1beta1.Step{
				{
					Script:     "$(params.baz[*])",
					Name:       "mystep",
					Image:      "my-image",
					WorkingDir: "/foo/bar/src/",
				}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.baz[*])"`,
			Paths:   []string{"steps[0].script"},
		},
	}, {
		name: "array not properly isolated",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
				Type: v1beta1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1beta1.ParamTypeArray,
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "someimage",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"not isolated: $(params.baz)", "middle string", "url"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz)"`,
			Paths:   []string{"steps[0].args[0]"},
		},
	}, {
		name: "array star not properly isolated",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name: "baz",
				Type: v1beta1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1beta1.ParamTypeArray,
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "someimage",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"not isolated: $(params.baz[*])", "middle string", "url"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz[*])"`,
			Paths:   []string{"steps[0].args[0]"},
		},
	}, {
		name: "inferred array not properly isolated",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:    "baz",
				Default: v1beta1.NewStructuredValues("implied", "array", "type"),
			}, {
				Name:    "foo-is-baz",
				Default: v1beta1.NewStructuredValues("implied", "array", "type"),
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "someimage",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"not isolated: $(params.baz)", "middle string", "url"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz)"`,
			Paths:   []string{"steps[0].args[0]"},
		},
	}, {
		name: "inferred array star not properly isolated",
		fields: fields{
			Params: []v1beta1.ParamSpec{{
				Name:    "baz",
				Default: v1beta1.NewStructuredValues("implied", "array", "type"),
			}, {
				Name:    "foo-is-baz",
				Default: v1beta1.NewStructuredValues("implied", "array", "type"),
			}},
			Steps: []v1beta1.Step{{
				Name:       "mystep",
				Image:      "someimage",
				Command:    []string{"$(params.foo-is-baz)"},
				Args:       []string{"not isolated: $(params.baz[*])", "middle string", "url"},
				WorkingDir: "/foo/bar/src/",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz[*])"`,
			Paths:   []string{"steps[0].args[0]"},
		},
	}, {
		name: "Multiple volumes with same name",
		fields: fields{
			Steps: validSteps,
			Volumes: []corev1.Volume{{
				Name: "workspace",
			}, {
				Name: "workspace",
			}},
		},
		expectedError: apis.FieldError{
			Message: `multiple volumes with same name "workspace"`,
			Paths:   []string{"volumes[1].name"},
		},
	}, {
		name: "step with script and command",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image:   "myimage",
				Command: []string{"command"},
				Script:  "script",
			}},
		},
		expectedError: apis.FieldError{
			Message: "script cannot be used with command",
			Paths:   []string{"steps[0].script"},
		},
	}, {
		name: "step volume mounts under /tekton/",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "myimage",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "foo",
					MountPath: "/tekton/foo",
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `volumeMount cannot be mounted under /tekton/ (volumeMount "foo" mounted at "/tekton/foo")`,
			Paths:   []string{"steps[0].volumeMounts[0].mountPath"},
		},
	}, {
		name: "step volume mount name starts with tekton-internal-",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "myimage",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "tekton-internal-foo",
					MountPath: "/this/is/fine",
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `volumeMount name "tekton-internal-foo" cannot start with "tekton-internal-"`,
			Paths:   []string{"steps[0].volumeMounts[0].name"},
		},
	}, {
		name: "declared workspaces names are not unique",
		fields: fields{
			Steps: validSteps,
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "same-workspace",
				MountPath: "/foo",
			}, {
				Name:      "same-workspace",
				MountPath: "/bar",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace name \"same-workspace\" must be unique",
			Paths:   []string{"workspaces[1].name"},
		},
	}, {
		name: "declared workspaces clash with each other",
		fields: fields{
			Steps: validSteps,
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "some-workspace",
				MountPath: "/foo",
			}, {
				Name:      "another-workspace",
				MountPath: "/foo",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/foo\" must be unique",
			Paths:   []string{"workspaces[1].mountpath"},
		},
	}, {
		name: "workspace mount path already in volumeMounts",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image:   "myimage",
				Command: []string{"command"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "my-mount",
					MountPath: "/foo",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "some-workspace",
				MountPath: "/foo",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/foo\" must be unique",
			Paths:   []string{"workspaces[0].mountpath"},
		},
	}, {
		name: "workspace default mount path already in volumeMounts",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image:   "myimage",
				Command: []string{"command"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "my-mount",
					MountPath: "/workspace/some-workspace/",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "some-workspace",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/workspace/some-workspace\" must be unique",
			Paths:   []string{"workspaces[0].mountpath"},
		},
	}, {
		name: "workspace mount path already in stepTemplate",
		fields: fields{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "my-mount",
					MountPath: "/foo",
				}},
			},
			Steps: validSteps,
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:      "some-workspace",
				MountPath: "/foo",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/foo\" must be unique",
			Paths:   []string{"workspaces[0].mountpath"},
		},
	}, {
		name: "workspace default mount path already in stepTemplate",
		fields: fields{
			StepTemplate: &v1beta1.StepTemplate{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "my-mount",
					MountPath: "/workspace/some-workspace",
				}},
			},
			Steps: validSteps,
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "some-workspace",
			}},
		},
		expectedError: apis.FieldError{
			Message: "workspace mount path \"/workspace/some-workspace\" must be unique",
			Paths:   []string{"workspaces[0].mountpath"},
		},
	}, {
		name: "result name not validate",
		fields: fields{
			Steps: validSteps,
			Results: []v1beta1.TaskResult{{
				Name:        "MY^RESULT",
				Description: "my great result",
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid key name "MY^RESULT"`,
			Paths:   []string{"results[0].name"},
			Details: "Name must consist of alphanumeric characters, '-', '_', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my-name',  or 'my_name', regex used for validation is '^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$')",
		},
	}, {
		name: "result type not validate",
		fields: fields{
			Steps: validSteps,
			Results: []v1beta1.TaskResult{{
				Name:        "MY-RESULT",
				Type:        "wrong",
				Description: "my great result",
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: wrong`,
			Paths:   []string{"results[0].type"},
			Details: "type must be string",
		},
	}, {
		name: "context not validate",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Script: `
				#!/usr/bin/env  bash
				hello "$(context.task.missing)"`,
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "\n\t\t\t\t#!/usr/bin/env  bash\n\t\t\t\thello \"$(context.task.missing)\""`,
			Paths:   []string{"steps[0].script"},
		},
	}, {
		name: "negative timeout string",
		fields: fields{
			Steps: []v1beta1.Step{{
				Timeout: &metav1.Duration{Duration: -10 * time.Second},
			}},
		},
		expectedError: apis.FieldError{
			Message: "invalid value: -10s",
			Paths:   []string{"steps[0].negative timeout"},
		},
	}, {
		name: "with resources",
		fields: fields{
			Steps:     []v1beta1.Step{{Image: "my-image"}},
			Resources: &v1beta1.TaskResources{},
		},
		expectedError: apis.FieldError{Message: "must not set the field(s)", Paths: []string{"resources"}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1beta1.TaskSpec{
				Params:       tt.fields.Params,
				Steps:        tt.fields.Steps,
				Volumes:      tt.fields.Volumes,
				StepTemplate: tt.fields.StepTemplate,
				Workspaces:   tt.fields.Workspaces,
				Results:      tt.fields.Results,
				Resources:    tt.fields.Resources,
			}
			ctx := context.Background()
			ts.SetDefaults(ctx)
			err := ts.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", ts)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskSpecValidateErrorSidecarName(t *testing.T) {
	tests := []struct {
		name          string
		sidecars      []v1beta1.Sidecar
		expectedError apis.FieldError
	}{{
		name: "cannot use reserved sidecar name",
		sidecars: []v1beta1.Sidecar{{
			Name:  "tekton-log-results",
			Image: "my-image",
		}},
		expectedError: apis.FieldError{
			Message: fmt.Sprintf("Invalid: cannot use reserved sidecar name %v ", pipeline.ReservedResultsSidecarName),
			Paths:   []string{"sidecars"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Name:  "does-not-matter",
					Image: "does-not-matter",
				}},
				Sidecars: tt.sidecars,
			}
			err := ts.Validate(context.Background())
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", ts)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestStepAndSidecarWorkspaces(t *testing.T) {
	type fields struct {
		Steps      []v1beta1.Step
		Sidecars   []v1beta1.Sidecar
		Workspaces []v1beta1.WorkspaceDeclaration
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "valid step workspace usage",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Args:  []string{"arg"},
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name:      "foo-workspace",
					MountPath: "/a/custom/mountpath",
				}},
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name:        "foo-workspace",
				Description: "my great workspace",
				MountPath:   "some/path",
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1beta1.TaskSpec{
				Steps:      tt.fields.Steps,
				Sidecars:   tt.fields.Sidecars,
				Workspaces: tt.fields.Workspaces,
			}
			ctx := config.EnableAlphaAPIFields(context.Background())
			ts.SetDefaults(ctx)
			if err := ts.Validate(ctx); err != nil {
				t.Errorf("TaskSpec.Validate() = %v", err)
			}
		})
	}
}

func TestStepAndSidecarWorkspacesErrors(t *testing.T) {
	type fields struct {
		Steps    []v1beta1.Step
		Sidecars []v1beta1.Sidecar
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "step workspace that refers to non-existent workspace declaration fails",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "foo",
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "foo",
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `undefined workspace "foo"`,
			Paths:   []string{"steps[0].workspaces[0].name"},
		},
	}, {
		name: "sidecar workspace that refers to non-existent workspace declaration fails",
		fields: fields{
			Steps: []v1beta1.Step{{
				Image: "foo",
			}},
			Sidecars: []v1beta1.Sidecar{{
				Image: "foo",
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "foo",
				}},
			}},
		},
		expectedError: apis.FieldError{
			Message: `undefined workspace "foo"`,
			Paths:   []string{"sidecars[0].workspaces[0].name"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1beta1.TaskSpec{
				Steps:    tt.fields.Steps,
				Sidecars: tt.fields.Sidecars,
			}

			ctx := config.EnableAlphaAPIFields(context.Background())
			ts.SetDefaults(ctx)
			err := ts.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", ts)
			}

			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestStepOnError(t *testing.T) {
	tests := []struct {
		name          string
		params        []v1beta1.ParamSpec
		steps         []v1beta1.Step
		expectedError *apis.FieldError
	}{{
		name: "valid step - valid onError usage - set to continue",
		steps: []v1beta1.Step{{
			OnError: v1beta1.Continue,
			Image:   "image",
			Args:    []string{"arg"},
		}},
	}, {
		name: "valid step - valid onError usage - set to stopAndFail",
		steps: []v1beta1.Step{{
			OnError: v1beta1.StopAndFail,
			Image:   "image",
			Args:    []string{"arg"},
		}},
	}, {
		name: "valid step - valid onError usage - set to a task parameter",
		params: []v1beta1.ParamSpec{{
			Name:    "CONTINUE",
			Default: &v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: string(v1beta1.Continue)},
		}},
		steps: []v1beta1.Step{{
			OnError: "$(params.CONTINUE)",
			Image:   "image",
			Args:    []string{"arg"},
		}},
	}, {
		name: "invalid step - onError set to invalid value",
		steps: []v1beta1.Step{{
			OnError: "onError",
			Image:   "image",
			Args:    []string{"arg"},
		}},
		expectedError: &apis.FieldError{
			Message: `invalid value: "onError"`,
			Paths:   []string{"steps[0].onError"},
			Details: `Task step onError must be either "continue" or "stopAndFail"`,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &v1beta1.TaskSpec{
				Params: tt.params,
				Steps:  tt.steps,
			}
			ctx := context.Background()
			ts.SetDefaults(ctx)
			err := ts.Validate(ctx)
			if tt.expectedError == nil && err != nil {
				t.Errorf("No error expected from TaskSpec.Validate() but got = %v", err)
			} else if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error from TaskSpec.Validate() = %v, but got none", tt.expectedError)
				} else if d := cmp.Diff(tt.expectedError.Error(), err.Error()); d != "" {
					t.Errorf("returned error from TaskSpec.Validate() does not match with the expected error: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

// TestIncompatibleAPIVersions exercises validation of fields that
// require a specific feature gate version in order to work.
func TestIncompatibleAPIVersions(t *testing.T) {
	tests := []struct {
		name            string
		requiredVersion string
		spec            v1beta1.TaskSpec
	}{{
		name:            "step workspace requires alpha",
		requiredVersion: "alpha",
		spec: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "foo",
			}},
			Steps: []v1beta1.Step{{
				Image: "foo",
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "foo",
				}},
			}},
		},
	}, {
		name:            "sidecar workspace requires alpha",
		requiredVersion: "alpha",
		spec: v1beta1.TaskSpec{
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				Name: "foo",
			}},
			Steps: []v1beta1.Step{{
				Image: "foo",
			}},
			Sidecars: []v1beta1.Sidecar{{
				Image: "foo",
				Workspaces: []v1beta1.WorkspaceUsage{{
					Name: "foo",
				}},
			}},
		},
	}, {
		name:            "windows script support requires alpha",
		requiredVersion: "alpha",
		spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "my-image",
				Script: `
				#!win powershell -File
				script-1`,
			}},
		},
	}, {
		name:            "stdout stream support requires alpha",
		requiredVersion: "alpha",
		spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "foo",
				StdoutConfig: &v1beta1.StepOutputConfig{
					Path: "/tmp/stdout.txt",
				},
			}},
		},
	}, {
		name:            "stderr stream support requires alpha",
		requiredVersion: "alpha",
		spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "foo",
				StderrConfig: &v1beta1.StepOutputConfig{
					Path: "/tmp/stderr.txt",
				},
			}},
		},
	}}
	versions := []string{"alpha", "stable"}
	for _, tt := range tests {
		for _, version := range versions {
			testName := fmt.Sprintf("(using %s) %s", version, tt.name)
			t.Run(testName, func(t *testing.T) {
				ts := tt.spec
				ctx := context.Background()
				if version == "alpha" {
					ctx = config.EnableAlphaAPIFields(ctx)
				}
				ts.SetDefaults(ctx)
				err := ts.Validate(ctx)

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

func TestGetArrayIndexParamRefs(t *testing.T) {
	stepsReferences := []string{}
	for i := 10; i <= 26; i++ {
		stepsReferences = append(stepsReferences, fmt.Sprintf("$(params.array-params[%d])", i))
	}
	volumesReferences := []string{}
	for i := 10; i <= 22; i++ {
		volumesReferences = append(volumesReferences, fmt.Sprintf("$(params.array-params[%d])", i))
	}

	tcs := []struct {
		name     string
		taskspec *v1beta1.TaskSpec
		want     sets.String
	}{{
		name: "steps reference",
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Steps: []v1beta1.Step{{
				Name:    "$(params.array-params[10])",
				Image:   "$(params.array-params[11])",
				Command: []string{"$(params.array-params[12])"},
				Args:    []string{"$(params.array-params[13])"},
				Script:  "echo $(params.array-params[14])",
				Env: []corev1.EnvVar{{
					Value: "$(params.array-params[15])",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "$(params.array-params[16])",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.array-params[17])",
							},
						},
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							Key: "$(params.array-params[18])",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.array-params[19])",
							},
						},
					},
				}},
				EnvFrom: []corev1.EnvFromSource{{
					Prefix: "$(params.array-params[20])",
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(params.array-params[21])",
						},
					},
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(params.array-params[22])",
						},
					},
				}},
				WorkingDir: "$(params.array-params[23])",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "$(params.array-params[24])",
					MountPath: "$(params.array-params[25])",
					SubPath:   "$(params.array-params[26])",
				}},
			}},
			StepTemplate: &v1beta1.StepTemplate{
				Image: "$(params.array-params[27])",
			},
		},
		want: sets.NewString("$(params.array-params[10])", "$(params.array-params[11])", "$(params.array-params[12])", "$(params.array-params[13])", "$(params.array-params[14])",
			"$(params.array-params[15])", "$(params.array-params[16])", "$(params.array-params[17])", "$(params.array-params[18])", "$(params.array-params[19])", "$(params.array-params[20])",
			"$(params.array-params[21])", "$(params.array-params[22])", "$(params.array-params[23])", "$(params.array-params[24])", "$(params.array-params[25])", "$(params.array-params[26])", "$(params.array-params[27])"),
	}, {
		name: "stepTemplate reference",
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			StepTemplate: &v1beta1.StepTemplate{
				Image: "$(params.array-params[3])",
			},
		},
		want: sets.NewString("$(params.array-params[3])"),
	}, {
		name: "volumes references",
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Volumes: []corev1.Volume{{
				Name: "$(params.array-params[10])",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(params.array-params[11])",
						},
						Items: []corev1.KeyToPath{{
							Key:  "$(params.array-params[12])",
							Path: "$(params.array-params[13])",
						},
						},
					},
					Secret: &corev1.SecretVolumeSource{
						SecretName: "$(params.array-params[14])",
						Items: []corev1.KeyToPath{{
							Key:  "$(params.array-params[15])",
							Path: "$(params.array-params[16])",
						}},
					},
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "$(params.array-params[17])",
					},
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "$(params.array-params[18])",
								},
							},
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "$(params.array-params[19])",
								},
							},
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Audience: "$(params.array-params[20])",
							},
						}},
					},
					CSI: &corev1.CSIVolumeSource{
						NodePublishSecretRef: &corev1.LocalObjectReference{
							Name: "$(params.array-params[21])",
						},
						VolumeAttributes: map[string]string{"key": "$(params.array-params[22])"},
					},
				},
			},
			},
		},
		want: sets.NewString("$(params.array-params[10])", "$(params.array-params[11])", "$(params.array-params[12])", "$(params.array-params[13])", "$(params.array-params[14])",
			"$(params.array-params[15])", "$(params.array-params[16])", "$(params.array-params[17])", "$(params.array-params[18])", "$(params.array-params[19])", "$(params.array-params[20])",
			"$(params.array-params[21])", "$(params.array-params[22])"),
	}, {
		name: "workspaces references",
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				MountPath: "$(params.array-params[3])",
			}},
		},
		want: sets.NewString("$(params.array-params[3])"),
	}, {
		name: "sidecar references",
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Sidecars: []v1beta1.Sidecar{{
				Script: "$(params.array-params[3])",
			},
			},
		},
		want: sets.NewString("$(params.array-params[3])"),
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.taskspec.GetIndexingReferencesToArrayParams()
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("GetIndexingReferencesToArrayParams diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
