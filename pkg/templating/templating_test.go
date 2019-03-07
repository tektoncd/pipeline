/*
 Copyright 2019 Knative Authors LLC
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

package templating_test

import (
	"testing"

	"github.com/knative/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/templating"
)

func TestValidateVariables(t *testing.T) {
	type args struct {
		input         string
		prefix        string
		contextPrefix string
		locationName  string
		path          string
		vars          map[string]struct{}
	}
	tests := []struct {
		name          string
		args          args
		expectedError *apis.FieldError
	}{
		{
			name: "valid variable",
			args: args{
				input:         "--flag=${inputs.params.baz}",
				prefix:        "params",
				contextPrefix: "inputs.",
				locationName:  "step",
				path:          "taskspec.steps",
				vars: map[string]struct{}{
					"baz": {},
				},
			},
			expectedError: nil,
		},
		{
			name: "multiple variables",
			args: args{
				input:         "--flag=${inputs.params.baz} ${input.params.foo}",
				prefix:        "params",
				contextPrefix: "inputs.",
				locationName:  "step",
				path:          "taskspec.steps",
				vars: map[string]struct{}{
					"baz": {},
					"foo": {},
				},
			},
			expectedError: nil,
		},
		{
			name: "different context and prefix",
			args: args{
				input:        "--flag=${something.baz}",
				prefix:       "something",
				locationName: "step",
				path:         "taskspec.steps",
				vars: map[string]struct{}{
					"baz": {},
				},
			},
			expectedError: nil,
		},
		{
			name: "undefined variable",
			args: args{
				input:         "--flag=${inputs.params.baz}",
				prefix:        "params",
				contextPrefix: "inputs.",
				locationName:  "step",
				path:          "taskspec.steps",
				vars: map[string]struct{}{
					"foo": {},
				},
			},
			expectedError: &apis.FieldError{
				Message: `non-existent variable in "--flag=${inputs.params.baz}" for step somefield`,
				Paths:   []string{"taskspec.steps.somefield"},
			},
		},
		{
			name: "undefined variable and defined variable",
			args: args{
				input:         "--flag=${inputs.params.baz} ${input.params.foo}",
				prefix:        "params",
				contextPrefix: "inputs.",
				locationName:  "step",
				path:          "taskspec.steps",
				vars: map[string]struct{}{
					"foo": {},
				},
			},
			expectedError: &apis.FieldError{
				Message: `non-existent variable in "--flag=${inputs.params.baz} ${input.params.foo}" for step somefield`,
				Paths:   []string{"taskspec.steps.somefield"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := templating.ValidateVariable("somefield", tt.args.input, tt.args.prefix, tt.args.contextPrefix, tt.args.locationName, tt.args.path, tt.args.vars)

			if d := cmp.Diff(got, tt.expectedError, cmp.AllowUnexported(apis.FieldError{})); d != "" {
				t.Errorf("ValidateVariable() error did not match expected error %s", d)
			}
		})
	}
}
