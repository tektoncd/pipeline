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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNameAndNamespace(t *testing.T) {
	tests := []struct {
		name         string
		ownerName    string
		ownerNS      string
		inputName    string
		inputNS      string
		expectErrMsg string
		expectNS     string
	}{
		{
			name:      "uses owner namespace when name is empty",
			ownerName: "my-run",
			ownerNS:   "my-ns",
			inputName: "",
			inputNS:   "",
			expectNS:  "my-ns",
		},
		{
			name:      "explicit name and namespace succeeds",
			ownerName: "my-run",
			ownerNS:   "my-ns",
			inputName: "explicit-name",
			inputNS:   "explicit-ns",
			expectNS:  "explicit-ns",
		},
		{
			name:      "explicit name and namespace ignores owner namespace",
			ownerName: "my-run",
			ownerNS:   "owner-ns",
			inputName: "explicit-name",
			inputNS:   "explicit-ns",
			expectNS:  "explicit-ns",
		},
		{
			name:      "explicit name with empty namespace falls back to owner namespace",
			ownerName: "my-run",
			ownerNS:   "my-ns",
			inputName: "explicit-name",
			inputNS:   "",
			expectNS:  "my-ns",
		},
		{
			name:         "errors when both input and owner namespace are empty",
			ownerName:    "my-run",
			ownerNS:      "",
			inputName:    "explicit-name",
			inputNS:      "",
			expectErrMsg: "namespace is required",
		},
		{
			name:         "errors on empty namespace from owner",
			ownerName:    "my-run",
			ownerNS:      "",
			inputName:    "",
			inputNS:      "",
			expectErrMsg: "namespace is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner := &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.ownerName,
					Namespace: tt.ownerNS,
				},
			}
			req := &v1beta1.ResolutionRequestSpec{
				Params: v1.Params{{
					Name:  "foo",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "bar"},
				}},
			}
			gotName, gotNS, err := resource.GetNameAndNamespace("git", owner, tt.inputName, tt.inputNS, req)
			if tt.expectErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q but got nil", tt.expectErrMsg)
				}
				if !strings.Contains(err.Error(), tt.expectErrMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.expectErrMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotName == "" {
				t.Error("expected non-empty name")
			}
			if gotNS != tt.expectNS {
				t.Errorf("expected namespace %q, got %q", tt.expectNS, gotNS)
			}
		})
	}
}

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
		{
			name: "long prefix with nil params truncates prefix preserving hash",
			args: args{
				prefix: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 31 chars
				base:   "default/my-taskrun",
			},
			want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-2d162955e9c9fcfef736fd389e7fd796",
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
		{
			name: "long prefix at exactly max length is not truncated",
			args: args{
				prefix: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 30 chars, name = 30+1+32 = 63 = maxLength
				base:   "default/my-taskrun",
				params: []v1.Param{{
					Name:  "foo",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "bar"},
				}},
			},
			want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-da9d0a4501a276745547b4b4b21a2e77", // exactly 63, fits
		},
		{
			name: "long prefix exceeding max length truncates prefix not hash",
			args: args{
				prefix: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 31 chars, would be 64 > maxLength
				base:   "default/my-taskrun",
				params: []v1.Param{{
					Name:  "foo",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "bar"},
				}},
			},
			want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-da9d0a4501a276745547b4b4b21a2e77", // prefix trimmed to 30, hash preserved
		},
		{
			name: "very long prefix truncates prefix preserving hash",
			args: args{
				prefix: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 50 chars
				base:   "default/my-taskrun",
			},
			want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-2d162955e9c9fcfef736fd389e7fd796", // prefix trimmed to 30, hash preserved
		},
		{
			name: "different inputs with same long prefix produce different names",
			args: args{
				prefix: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 31 chars
				base:   "default/other-taskrun",
				params: []v1.Param{{
					Name:  "baz",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "qux"},
				}},
			},
			want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-5592e5f983640b4274071dbe1c09068d", // same prefix length, different hash
		},
		{
			name: "prefix under max length returns full name",
			args: args{
				prefix: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 29 chars, name = 62 chars
				base:   "default/my-taskrun",
				params: []v1.Param{{
					Name:  "foo",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "bar"},
				}},
			},
			want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaa-da9d0a4501a276745547b4b4b21a2e77", // 62 chars, no truncation
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
			name:         "goo-array",
			resolverType: "bundle",
			isPresent:    true,
			params: []v1.Param{
				{
					Name: resource.ParamName,
					Value: v1.ParamValue{
						Type:     v1.ParamTypeArray,
						ArrayVal: []string{resource.ParamName, "goo-array"},
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
