/*
Copyright 2024 The Tekton Authors

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
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewRequest(t *testing.T) {
	type args struct {
		resolverPayload resource.ResolverPayload
	}
	type want = args
	golden := args{
		resolverPayload: resource.ResolverPayload{
			Name:      "test-name",
			Namespace: "test-namespace",
			ResolutionSpec: &v1beta1.ResolutionRequestSpec{
				Params: v1.Params{
					{Name: "param1", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "value1"}},
					{Name: "param2", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "value2"}},
				},
			},
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
			request := resource.NewRequest(tt.args.resolverPayload)
			if request == nil {
				t.Errorf("NewRequest() return nil")
			}
			if d := cmp.Diff(tt.want.resolverPayload, request.ResolverPayload()); d != "" {
				t.Errorf("expected params to match %s", diff.PrintWantGot(d))
			}
		})
	}
}
