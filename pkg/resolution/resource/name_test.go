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
	"strings"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/resolution/resource"
)

func TestGenerateDeterministicName(t *testing.T) {
	type args struct {
		prefix string
		base   string
		params []v1.Param
	}
	golden := args{
		prefix: "prefix",
		base:   "base",
		params: []v1.Param{
			{
				Name: "string-param",
				Value: v1.ParamValue{
					Type:      v1.ParamTypeString,
					StringVal: "value1",
				},
			},
			{
				Name: "array-param",
				Value: v1.ParamValue{
					Type:     v1.ParamTypeArray,
					ArrayVal: []string{"value1", "value2"},
				},
			},
			{
				Name: "object-param",
				Value: v1.ParamValue{
					Type:      v1.ParamTypeObject,
					ObjectVal: map[string]string{"key": "value"},
				},
			},
		},
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "only contains prefix",
			args: args{
				prefix: golden.prefix,
			},
			want: "prefix-6c62272e07bb014262b821756295c58d",
		},
		{
			name: "only contains base",
			args: args{
				base: golden.base,
			},
			want: "-6989337ae0757277b806e97e86444ef0",
		},
		{
			name: "only contains params",
			args: args{
				params: golden.params,
			},
			want: "-52921b17d3c2930a34419c618d6af0e9",
		},
		{
			name: "params with different order should generate same hash",
			args: args{
				params: []v1.Param{
					golden.params[2],
					golden.params[1],
					golden.params[0],
				},
			},
			want: "-52921b17d3c2930a34419c618d6af0e9",
		},
		{
			name: "contain all fields",
			args: golden,
			want: "prefix-ba2f256f318de7f4154da577c283cb9e",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resource.GenerateDeterministicName(tt.args.prefix, tt.args.base, tt.args.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDeterministicName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GenerateDeterministicName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateDeterministicNameFromSpec(t *testing.T) {
	type args struct {
		prefix string
		base   string
		params []v1.Param
		url    string
	}
	golden := args{
		prefix: "prefix",
		base:   "base",
		params: []v1.Param{
			{
				Name: "string-param",
				Value: v1.ParamValue{
					Type:      v1.ParamTypeString,
					StringVal: "value1",
				},
			},
			{
				Name: "array-param",
				Value: v1.ParamValue{
					Type:     v1.ParamTypeArray,
					ArrayVal: []string{"value1", "value2"},
				},
			},
			{
				Name: "object-param",
				Value: v1.ParamValue{
					Type:      v1.ParamTypeObject,
					ObjectVal: map[string]string{"key": "value"},
				},
			},
		},
		url: "https://foo/bar",
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "only contains prefix",
			args: args{
				prefix: golden.prefix,
			},
			want: "prefix-6c62272e07bb014262b821756295c58d",
		},
		{
			name: "only contains base",
			args: args{
				base: golden.base,
			},
			want: "-6989337ae0757277b806e97e86444ef0",
		},
		{
			name: "only contains url",
			args: args{
				url: golden.url,
			},
			want: "-dcfaf53735f4a84a3e319e17352940b4",
		},
		{
			name: "only contains params",
			args: args{
				params: golden.params,
			},
			want: "-52921b17d3c2930a34419c618d6af0e9",
		},
		{
			name: "params with different order should generate same hash",
			args: args{
				params: []v1.Param{
					golden.params[2],
					golden.params[1],
					golden.params[0],
				},
			},
			want: "-52921b17d3c2930a34419c618d6af0e9",
		},
		{
			name: "contain all fields",
			args: golden,
			want: "prefix-ff25bd24688ab610bdc530a5ab3aabbd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolutionSpec := &v1beta1.ResolutionRequestSpec{
				Params: tt.args.params,
				URL:    tt.args.url,
			}
			got, err := resource.GenerateDeterministicNameFromSpec(tt.args.prefix, tt.args.base, resolutionSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDeterministicNameFromSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GenerateDeterministicNameFromSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateErrorLogString(t *testing.T) {
	tests := []struct {
		resolverType string
		name         string
		url          string
		err          string
		params       []v1.Param
		isPresent    bool
	}{
		{
			name:         "foo",
			url:          "https://bar",
			resolverType: "git",
			isPresent:    true,
			params: []v1.Param{
				{
					Name: resource.ParamName,
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "foo",
					},
				},
				{
					Name: resource.ParamURL,
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "https://bar",
					},
				},
			},
		},
		{
			name:         "foo",
			url:          "https://bar",
			resolverType: "",
			err:          "name could not be marshalled",
			params:       []v1.Param{},
		},
		{
			name:         "goo",
			resolverType: "bundle",
			isPresent:    true,
			params: []v1.Param{
				{
					Name: resource.ParamName,
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "goo",
					},
				},
			},
		},
		{
			name:         "hoo",
			resolverType: "cluster",
			err:          "name could not be marshalled",
			isPresent:    true,
			params: []v1.Param{
				{
					Name: resource.ParamName,
					Value: v1.ParamValue{
						Type:      v1.ParamTypeString,
						StringVal: "hoo",
					},
				},
				{
					Name: resource.ParamName,
					Value: v1.ParamValue{
						Type: v1.ParamType("foo"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resource.GenerateErrorLogString(tt.resolverType, tt.params)
			if strings.Contains(got, tt.name) != tt.isPresent {
				t.Errorf("name %s presence in %s should be %v", tt.name, got, tt.isPresent)
			}
			if strings.Contains(got, tt.url) != tt.isPresent {
				t.Errorf("url %s presence in %s should be %v", tt.url, got, tt.isPresent)
			}
			if strings.Contains(got, tt.err) != tt.isPresent {
				t.Errorf("err %s presence in %s should be %v", tt.err, got, tt.isPresent)
			}
			// should always have resolver type
			if !strings.Contains(got, tt.resolverType) {
				t.Errorf("type %s not in %s", tt.resolverType, got)
			}
		})
	}
}
