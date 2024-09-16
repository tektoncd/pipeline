/*
Copyright 2023 The Tekton Authors
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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestStepActionValidate(t *testing.T) {
	tests := []struct {
		name string
		sa   *v1beta1.StepAction
		wc   func(context.Context) context.Context
	}{{
		name: "valid step action",
		sa: &v1beta1.StepAction{
			ObjectMeta: metav1.ObjectMeta{Name: "stepaction"},
			Spec: v1beta1.StepActionSpec{
				Image: "my-image",
				Script: `
				#!/usr/bin/env  bash
				echo hello`,
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.wc != nil {
				ctx = tt.wc(ctx)
			}
			err := tt.sa.Validate(ctx)
			if err != nil {
				t.Errorf("StepAction.Validate() returned error for valid StepAction: %v", err)
			}
		})
	}
}

func TestStepActionSpecValidate(t *testing.T) {
	type fields struct {
		Image        string
		Command      []string
		Args         []string
		Script       string
		Env          []corev1.EnvVar
		Params       []v1.ParamSpec
		Results      []v1.StepResult
		VolumeMounts []corev1.VolumeMount
	}
	tests := []struct {
		name   string
		fields fields
	}{{
		name: "step action with command",
		fields: fields{
			Image:   "myimage",
			Command: []string{"ls"},
			Args:    []string{"-lh"},
		},
	}, {
		name: "step action with script",
		fields: fields{
			Image:  "myimage",
			Script: "echo hi",
		},
	}, {
		name: "step action with env",
		fields: fields{
			Image:  "myimage",
			Script: "echo hi",
			Env: []corev1.EnvVar{{
				Name:  "HOME",
				Value: "/tekton/home",
			}},
		},
	}, {
		name: "valid params type explicit",
		fields: fields{
			Image: "myimage",
			Params: []v1.ParamSpec{{
				Name:        "stringParam",
				Type:        v1.ParamTypeString,
				Description: "param",
				Default:     v1.NewStructuredValues("default"),
			}, {
				Name:        "objectParam",
				Type:        v1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1.PropertySpec{
					"key1": {},
					"key2": {},
				},
				Default: v1.NewObject(map[string]string{
					"key1": "var1",
					"key2": "var2",
				}),
			}, {
				Name:        "objectParamWithoutDefault",
				Type:        v1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1.PropertySpec{
					"key1": {},
					"key2": {},
				},
			}, {
				Name:        "objectParamWithDefaultPartialKeys",
				Type:        v1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1.PropertySpec{
					"key1": {},
					"key2": {},
				},
				Default: v1.NewObject(map[string]string{
					"key1": "default",
				}),
			}},
		},
	}, {
		name: "valid string param usage",
		fields: fields{
			Image: "url",
			Params: []v1.ParamSpec{{
				Name: "baz",
			}, {
				Name: "foo-is-baz",
			}},
			Args: []string{"--flag=$(params.baz) && $(params.foo-is-baz)"},
		},
	}, {
		name: "valid array param usage",
		fields: fields{
			Image: "url",
			Params: []v1.ParamSpec{{
				Name: "baz",
				Type: v1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1.ParamTypeArray,
			}},
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"$(params.baz)", "middle string", "$(params.foo-is-baz)"},
		},
	}, {
		name: "valid object param usage",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "some-git-image",
			Args:  []string{"-url=$(params.gitrepo.url)", "-commit=$(params.gitrepo.commit)"},
		},
	}, {
		name: "valid star array usage",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "baz",
				Type: v1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1.ParamTypeArray,
			}},
			Image:   "myimage",
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"$(params.baz[*])", "middle string", "$(params.foo-is-baz[*])"},
		},
	}, {
		name: "valid result",
		fields: fields{
			Image: "my-image",
			Args:  []string{"arg"},
			Results: []v1.StepResult{{
				Name:        "MY-RESULT",
				Description: "my great result",
			}},
		},
	}, {
		name: "valid result type string",
		fields: fields{
			Image: "my-image",
			Args:  []string{"arg"},
			Results: []v1.StepResult{{
				Name:        "MY-RESULT",
				Type:        "string",
				Description: "my great result",
			}},
		},
	}, {
		name: "valid result type array",
		fields: fields{
			Image: "my-image",
			Args:  []string{"arg"},
			Results: []v1.StepResult{{
				Name:        "MY-RESULT",
				Type:        v1.ResultsTypeArray,
				Description: "my great result",
			}},
		},
	}, {
		name: "valid result type object",
		fields: fields{
			Image: "my-image",
			Args:  []string{"arg"},
			Results: []v1.StepResult{{
				Name:        "MY-RESULT",
				Type:        v1.ResultsTypeObject,
				Description: "my great result",
				Properties: map[string]v1.PropertySpec{
					"url":    {Type: "string"},
					"commit": {Type: "string"},
				},
			}},
		},
	}, {
		name: "valid volumeMounts",
		fields: fields{
			Image: "my-image",
			Args:  []string{"arg"},
			Params: []v1.ParamSpec{{
				Name: "foo",
			}, {
				Name: "array-params",
				Type: v1.ParamTypeArray,
			}, {
				Name: "object-params",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"key": {Type: "string"},
				},
			},
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.foo)",
				MountPath: "/config",
			}, {
				Name:      "$(params.array-params[0])",
				MountPath: "/config",
			}, {
				Name:      "$(params.object-params.key)",
				MountPath: "/config",
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &v1beta1.StepActionSpec{
				Image:        tt.fields.Image,
				Command:      tt.fields.Command,
				Args:         tt.fields.Args,
				Script:       tt.fields.Script,
				Env:          tt.fields.Env,
				Params:       tt.fields.Params,
				Results:      tt.fields.Results,
				VolumeMounts: tt.fields.VolumeMounts,
			}
			ctx := context.Background()
			sa.SetDefaults(ctx)
			if err := sa.Validate(ctx); err != nil {
				t.Errorf("StepActionSpec.Validate() = %v", err)
			}
		})
	}
}

func TestStepActionValidateError(t *testing.T) {
	type fields struct {
		Image        string
		Command      []string
		Args         []string
		Script       string
		Env          []corev1.EnvVar
		Params       []v1.ParamSpec
		Results      []v1.StepResult
		VolumeMounts []corev1.VolumeMount
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "inexistent image field",
		fields: fields{
			Args: []string{"flag"},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"spec.Image"},
		},
	}, {
		name: "object used in a string field",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "$(params.gitrepo)",
			Args:  []string{"echo"},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo)"`,
			Paths:   []string{"spec.image"},
		},
	}, {
		name: "object star used in a string field",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "$(params.gitrepo[*])",
			Args:  []string{"echo"},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo[*])"`,
			Paths:   []string{"spec.image"},
		},
	}, {
		name: "object used in a field that can accept array type",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "myimage",
			Args:  []string{"$(params.gitrepo)"},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo)"`,
			Paths:   []string{"spec.args[0]"},
		},
	}, {
		name: "object star used in a field that can accept array type",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "some-git-image",
			Args:  []string{"$(params.gitrepo[*])"},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo[*])"`,
			Paths:   []string{"spec.args[0]"},
		},
	}, {
		name: "non-existent individual key of an object param is used in task step",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "some-git-image",
			Args:  []string{"$(params.gitrepo.non-exist-key)"},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.gitrepo.non-exist-key)"`,
			Paths:   []string{"spec.args[0]"},
		},
	}, {
		name: "Inexistent param variable with existing",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "foo",
				Description: "param",
				Default:     v1.NewStructuredValues("default"),
			}},
			Image: "myimage",
			Args:  []string{"$(params.foo) && $(params.inexistent)"},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.foo) && $(params.inexistent)"`,
			Paths:   []string{"spec.args[0]"},
		},
	}, {
		name: "invalid param reference in volumeMount.Name - not a param reference",
		fields: fields{
			Image: "myimage",
			Params: []v1.ParamSpec{{
				Name: "foo",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "params.foo",
				MountPath: "/path",
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: params.foo`,
			Paths:   []string{"spec.volumeMounts[0].name"},
			Details: `expect the Name to be a single param reference`,
		},
	}, {
		name: "invalid param reference in volumeMount.Name - nested reference",
		fields: fields{
			Image: "myimage",
			Params: []v1.ParamSpec{{
				Name: "foo",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.foo)-foo",
				MountPath: "/path",
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: $(params.foo)-foo`,
			Paths:   []string{"spec.volumeMounts[0].name"},
			Details: `expect the Name to be a single param reference`,
		},
	}, {
		name: "invalid param reference in volumeMount.Name - multiple params references",
		fields: fields{
			Image: "myimage",
			Params: []v1.ParamSpec{{
				Name: "foo",
			}, {
				Name: "bar",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.foo)$(params.bar)",
				MountPath: "/path",
			}},
		},
		expectedError: apis.FieldError{
			Message: `invalid value: $(params.foo)$(params.bar)`,
			Paths:   []string{"spec.volumeMounts[0].name"},
			Details: `expect the Name to be a single param reference`,
		},
	}, {
		name: "invalid param reference in volumeMount.Name - not defined in params",
		fields: fields{
			Image: "myimage",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.foo)",
				MountPath: "/path",
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.foo)"`,
			Paths:   []string{"spec.volumeMounts[0]"},
		},
	}, {
		name: "invalid param reference in volumeMount.Name - array used in a volumeMounts name field",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeArray,
			}},
			Image: "image",
			VolumeMounts: []corev1.VolumeMount{{
				Name: "$(params.gitrepo)",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo)"`,
			Paths:   []string{"spec.volumeMounts[0]"},
		},
	}, {
		name: "invalid param reference in volumeMount.Name - object used in a volumeMounts name field",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "image",
			VolumeMounts: []corev1.VolumeMount{{
				Name: "$(params.gitrepo)",
			}},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.gitrepo)"`,
			Paths:   []string{"spec.volumeMounts[0]"},
		},
	}, {
		name: "invalid param reference in volumeMount.Name - object key not existent in params",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "gitrepo",
				Type: v1.ParamTypeObject,
				Properties: map[string]v1.PropertySpec{
					"url":    {},
					"commit": {},
				},
			}},
			Image: "image",
			VolumeMounts: []corev1.VolumeMount{{
				Name: "$(params.gitrepo.foo)",
			}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "$(params.gitrepo.foo)"`,
			Paths:   []string{"spec.volumeMounts[0]"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &v1beta1.StepAction{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: v1beta1.StepActionSpec{
					Image:        tt.fields.Image,
					Command:      tt.fields.Command,
					Args:         tt.fields.Args,
					Script:       tt.fields.Script,
					Env:          tt.fields.Env,
					Params:       tt.fields.Params,
					Results:      tt.fields.Results,
					VolumeMounts: tt.fields.VolumeMounts,
				},
			}
			ctx := context.Background()
			sa.SetDefaults(ctx)
			err := sa.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", sa)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("StepActionSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestStepActionSpecValidateError(t *testing.T) {
	type fields struct {
		Image   string
		Command []string
		Args    []string
		Script  string
		Env     []corev1.EnvVar
		Params  []v1.ParamSpec
		Results []v1.StepResult
	}
	tests := []struct {
		name          string
		fields        fields
		expectedError apis.FieldError
	}{{
		name: "inexistent image field",
		fields: fields{
			Args: []string{"flag"},
		},
		expectedError: apis.FieldError{
			Message: `missing field(s)`,
			Paths:   []string{"Image"},
		},
	}, {
		name: "command and script both used.",
		fields: fields{
			Image:   "my-image",
			Command: []string{"ls"},
			Script:  "echo hi",
		},
		expectedError: apis.FieldError{
			Message: `script cannot be used with command`,
			Paths:   []string{"script"},
		},
	}, {
		name: "windows script without alpha.",
		fields: fields{
			Image:  "my-image",
			Script: "#!win",
		},
		expectedError: apis.FieldError{
			Message: `windows script support requires "enable-api-fields" feature gate to be "alpha" but it is "beta"`,
			Paths:   []string{},
		},
	}, {
		name: "step script refers to nonexistent stepresult",
		fields: fields{
			Image: "my-image",
			Script: `
			#!/usr/bin/env bash
			date | tee $(step.results.non-exist.path)`,
			Results: []v1.StepResult{{Name: "a-result"}},
		},
		expectedError: apis.FieldError{
			Message: `non-existent variable in "\n\t\t\t#!/usr/bin/env bash\n\t\t\tdate | tee $(step.results.non-exist.path)"`,
			Paths:   []string{"script"},
		},
	}, {
		name: "invalid param name format",
		fields: fields{
			Params: []v1.ParamSpec{{
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
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: fmt.Sprintf("The format of following array and string variable names is invalid: %s", []string{"", "0ab", "a^b", "f oo"}),
			Paths:   []string{"params"},
			Details: "String/Array Names: \nMust only contain alphanumeric characters, hyphens (-), underscores (_), and dots (.)\nMust begin with a letter or an underscore (_)",
		},
	}, {
		name: "invalid object param format - object param name and key name shouldn't contain dots.",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "invalid.name1",
				Description: "object param name contains dots",
				Properties: map[string]v1.PropertySpec{
					"invalid.key1": {},
					"mykey2":       {},
				},
			}},
			Image: "myImage",
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
			Params: []v1.ParamSpec{{
				Name:        "foo",
				Type:        v1.ParamTypeString,
				Description: "parameter",
				Default:     v1.NewStructuredValues("value1"),
			}, {
				Name:        "foo",
				Type:        v1.ParamTypeString,
				Description: "parameter",
				Default:     v1.NewStructuredValues("value2"),
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: `parameter appears more than once`,
			Paths:   []string{"params[foo]"},
		},
	}, {
		name: "invalid param type",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "validparam",
				Type:        v1.ParamTypeString,
				Description: "parameter",
				Default:     v1.NewStructuredValues("default"),
			}, {
				Name:        "param-with-invalid-type",
				Type:        "invalidtype",
				Description: "invalidtypedesc",
				Default:     v1.NewStructuredValues("default"),
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: `invalid value: invalidtype`,
			Paths:   []string{"params.param-with-invalid-type.type"},
		},
	}, {
		name: "param mismatching default/type 1",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "task",
				Type:        v1.ParamTypeArray,
				Description: "param",
				Default:     v1.NewStructuredValues("default"),
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: `"array" type does not match default value's type: "string"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "param mismatching default/type 2",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "task",
				Type:        v1.ParamTypeString,
				Description: "param",
				Default:     v1.NewStructuredValues("default", "array"),
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: `"string" type does not match default value's type: "array"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "param mismatching default/type 3",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "task",
				Type:        v1.ParamTypeArray,
				Description: "param",
				Default: v1.NewObject(map[string]string{
					"key1": "var1",
					"key2": "var2",
				}),
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: `"array" type does not match default value's type: "object"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "param mismatching default/type 4",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "task",
				Type:        v1.ParamTypeObject,
				Description: "param",
				Properties:  map[string]v1.PropertySpec{"key1": {}},
				Default:     v1.NewStructuredValues("var"),
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: `"object" type does not match default value's type: "string"`,
			Paths:   []string{"params.task.type", "params.task.default.type"},
		},
	}, {
		name: "PropertySpec type is set with unsupported type",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "task",
				Type:        v1.ParamTypeObject,
				Description: "param",
				Properties: map[string]v1.PropertySpec{
					"key1": {Type: "number"},
					"key2": {Type: "string"},
				},
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: fmt.Sprintf("The value type specified for these keys %v is invalid", []string{"key1"}),
			Paths:   []string{"params.task.properties"},
		},
	}, {
		name: "Properties is missing",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:        "task",
				Type:        v1.ParamTypeObject,
				Description: "param",
			}},
			Image: "myImage",
		},
		expectedError: apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"task.properties"},
		},
	}, {
		name: "array used in unaccepted field",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "baz",
				Type: v1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1.ParamTypeArray,
			}},
			Image:   "$(params.baz)",
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"$(params.baz)", "middle string", "url"},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.baz)"`,
			Paths:   []string{"image"},
		},
	}, {
		name: "array star used in unaccepted field",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "baz",
				Type: v1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1.ParamTypeArray,
			}},
			Image:   "$(params.baz[*])",
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"$(params.baz)", "middle string", "url"},
		},
		expectedError: apis.FieldError{
			Message: `variable type invalid in "$(params.baz[*])"`,
			Paths:   []string{"image"},
		},
	}, {
		name: "array not properly isolated",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "baz",
				Type: v1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1.ParamTypeArray,
			}},
			Image:   "someimage",
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"not isolated: $(params.baz)", "middle string", "url"},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz)"`,
			Paths:   []string{"args[0]"},
		},
	}, {
		name: "array star not properly isolated",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "baz",
				Type: v1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1.ParamTypeArray,
			}},
			Image:   "someimage",
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"not isolated: $(params.baz[*])", "middle string", "url"},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz[*])"`,
			Paths:   []string{"args[0]"},
		},
	}, {
		name: "inferred array not properly isolated",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:    "baz",
				Default: v1.NewStructuredValues("implied", "array", "type"),
			}, {
				Name:    "foo-is-baz",
				Default: v1.NewStructuredValues("implied", "array", "type"),
			}},
			Image:   "someimage",
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"not isolated: $(params.baz)", "middle string", "url"},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz)"`,
			Paths:   []string{"args[0]"},
		},
	}, {
		name: "inferred array star not properly isolated",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name:    "baz",
				Default: v1.NewStructuredValues("implied", "array", "type"),
			}, {
				Name:    "foo-is-baz",
				Default: v1.NewStructuredValues("implied", "array", "type"),
			}},
			Image:   "someimage",
			Command: []string{"$(params.foo-is-baz)"},
			Args:    []string{"not isolated: $(params.baz[*])", "middle string", "url"},
		},
		expectedError: apis.FieldError{
			Message: `variable is not properly isolated in "not isolated: $(params.baz[*])"`,
			Paths:   []string{"args[0]"},
		},
	}, {
		name: "params used in script field",
		fields: fields{
			Params: []v1.ParamSpec{{
				Name: "baz",
				Type: v1.ParamTypeArray,
			}, {
				Name: "foo-is-baz",
				Type: v1.ParamTypeString,
			}},
			Script: "$(params.baz[0]), $(params.foo-is-baz)",
			Image:  "my-image",
		},
		expectedError: apis.FieldError{
			Message: `param substitution in scripts is not allowed.`,
			Paths:   []string{"script"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := v1beta1.StepActionSpec{
				Image:   tt.fields.Image,
				Command: tt.fields.Command,
				Args:    tt.fields.Args,
				Script:  tt.fields.Script,
				Env:     tt.fields.Env,
				Params:  tt.fields.Params,
				Results: tt.fields.Results,
			}
			ctx := context.Background()
			sa.SetDefaults(ctx)
			err := sa.Validate(ctx)
			if err == nil {
				t.Fatalf("Expected an error, got nothing for %v", sa)
			}
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("StepActionSpec.Validate() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
