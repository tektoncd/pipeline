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

package resource_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewRequest(t *testing.T) {
	type args struct {
		name      string
		namespace string
		params    v1.Params
	}
	type want = args
	golden := args{
		name:      "test-name",
		namespace: "test-namespace",
		params: v1.Params{
			{Name: "param1", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "value1"}},
			{Name: "param2", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "value2"}},
		},
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "empty",
			args: args{},
			want: want{},
		},
		{
			name: "all",
			args: golden,
			want: golden,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := resource.NewRequest(tt.args.name, tt.args.namespace, tt.args.params)
			if request == nil {
				t.Errorf("NewRequest() return nil")
			}
			if request.Name() != tt.want.name {
				t.Errorf("NewRequest().Name() = %v, want %v", request.Name(), tt.want.name)
			}
			if request.Namespace() != tt.want.namespace {
				t.Errorf("NewRequest().Namespace() = %v, want %v", request.Namespace(), tt.want.namespace)
			}
			if d := cmp.Diff(request.Params(), tt.want.params); d != "" {
				t.Errorf("expected params to match %s", diff.PrintWantGot(d))
			}
		})
	}
}
