// Copyright 2021 The Tekton Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAddContextParams(t *testing.T) {
	ctx := context.Background()

	// These test cases should run sequentially. Each step will modify the
	// above context.
	for _, tc := range []struct {
		name   string
		params []Param
		want   paramCtxVal
	}{
		{
			name:   "add-string-param",
			params: []Param{{Name: "a", Value: *NewArrayOrString("foo")}},
			want: paramCtxVal{
				"a": ParamSpec{
					Name: "a",
					Type: ParamTypeString,
				},
			},
		},
		{
			name:   "add-array-param",
			params: []Param{{Name: "b", Value: *NewArrayOrString("bar", "baz")}},
			want: paramCtxVal{
				"a": ParamSpec{
					Name: "a",
					Type: ParamTypeString,
				},
				"b": ParamSpec{
					Name: "b",
					Type: ParamTypeArray,
				},
			},
		},
		{
			name:   "add-object-param",
			params: []Param{{Name: "c", Value: *NewObject(map[string]string{"key1": "val1", "key2": "val2"})}},
			want: paramCtxVal{
				"a": ParamSpec{
					Name: "a",
					Type: ParamTypeString,
				},
				"b": ParamSpec{
					Name: "b",
					Type: ParamTypeArray,
				},
				"c": ParamSpec{
					Name: "c",
					Type: ParamTypeObject,
				},
			},
		},
		{
			name: "existing-param",
			params: []Param{
				{Name: "a", Value: *NewArrayOrString("foo1")},
				{Name: "b", Value: *NewArrayOrString("bar1", "baz1")},
				{Name: "c", Value: *NewObject(map[string]string{"key1": "val1", "key2": "val2"})},
			},
			want: paramCtxVal{
				"a": ParamSpec{
					Name: "a",
					Type: ParamTypeString,
				},
				"b": ParamSpec{
					Name: "b",
					Type: ParamTypeArray,
				},
				"c": ParamSpec{
					Name: "c",
					Type: ParamTypeObject,
				},
			},
		},
		{
			// This test case doesn't really make sense for typical use-cases,
			// but exists to document the behavior of how this would be
			// handled. The param context is simply responsible for propagating
			// the param values through, regardless of their underlying value.
			// Later validation should make sure these values make sense.
			name:   "empty-param",
			params: []Param{{}},
			want: paramCtxVal{
				"": ParamSpec{},
				"a": ParamSpec{
					Name: "a",
					Type: ParamTypeString,
				},
				"b": ParamSpec{
					Name: "b",
					Type: ParamTypeArray,
				},
				"c": ParamSpec{
					Name: "c",
					Type: ParamTypeObject,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx = addContextParams(ctx, tc.params)
			got := ctx.Value(paramCtxKey)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want,+got: %s", diff)
			}
		})
	}
}

func TestAddContextParamSpec(t *testing.T) {
	ctx := context.Background()

	// These test cases should run sequentially. Each step will modify the
	// above context.
	for _, tc := range []struct {
		name   string
		params []ParamSpec
		want   paramCtxVal
	}{
		{
			name: "add-paramspec",
			params: []ParamSpec{{
				Name: "a",
			}, {
				Name:       "b",
				Properties: map[string]PropertySpec{"key1": {}},
			}},
			want: paramCtxVal{
				"a": ParamSpec{
					Name: "a",
				},
				"b": ParamSpec{
					Name:       "b",
					Properties: map[string]PropertySpec{"key1": {}},
				},
			},
		},
		{
			name: "edit-paramspec",
			params: []ParamSpec{{
				Name:        "a",
				Type:        ParamTypeArray,
				Default:     NewArrayOrString("foo", "bar"),
				Description: "tacocat",
			}, {
				Name:        "b",
				Type:        ParamTypeObject,
				Properties:  map[string]PropertySpec{"key2": {}},
				Default:     NewObject(map[string]string{"key2": "val"}),
				Description: "my object",
			}},
			want: paramCtxVal{
				"a": ParamSpec{
					Name:        "a",
					Type:        ParamTypeArray,
					Default:     NewArrayOrString("foo", "bar"),
					Description: "tacocat",
				},
				"b": ParamSpec{
					Name:        "b",
					Type:        ParamTypeObject,
					Properties:  map[string]PropertySpec{"key2": {}},
					Default:     NewObject(map[string]string{"key2": "val"}),
					Description: "my object",
				},
			},
		},
		{
			// This test case doesn't really make sense for typical use-cases,
			// but exists to document the behavior of how this would be
			// handled. The param context is simply responsible for propagating
			// the ParamSpec values through, regardless of their underlying
			// value. Later validation should make sure these values make
			// sense.
			name:   "empty-param",
			params: []ParamSpec{{}},
			want: paramCtxVal{
				"": ParamSpec{},
				"a": ParamSpec{
					Name:        "a",
					Type:        ParamTypeArray,
					Default:     NewArrayOrString("foo", "bar"),
					Description: "tacocat",
				},
				"b": ParamSpec{
					Name:        "b",
					Type:        ParamTypeObject,
					Properties:  map[string]PropertySpec{"key2": {}},
					Default:     NewObject(map[string]string{"key2": "val"}),
					Description: "my object",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx = addContextParamSpec(ctx, tc.params)
			got := ctx.Value(paramCtxKey)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("-want,+got: %s", diff)
			}
		})
	}
}

func TestGetContextParams(t *testing.T) {
	ctx := context.Background()
	want := []ParamSpec{
		{
			Name:        "a",
			Type:        ParamTypeString,
			Default:     NewArrayOrString("foo"),
			Description: "tacocat",
		},
		{
			Name:        "b",
			Type:        ParamTypeArray,
			Default:     NewArrayOrString("bar"),
			Description: "racecar",
		},
		{
			Name:        "c",
			Type:        ParamTypeObject,
			Properties:  map[string]PropertySpec{"key1": {}},
			Default:     NewObject(map[string]string{"key1": "val1"}),
			Description: "my object",
		},
	}

	ctx = addContextParamSpec(ctx, want)

	for _, tc := range []struct {
		name    string
		overlay []Param
		want    []Param
	}{
		{
			name: "from-context",
			want: []Param{
				{
					Name:  "a",
					Value: *NewArrayOrString("$(params.a)"),
				},
				{
					Name: "b",
					Value: ArrayOrString{
						Type:     ParamTypeArray,
						ArrayVal: []string{"$(params.b[*])"},
					},
				},
				{
					Name: "c",
					Value: ArrayOrString{
						Type:      ParamTypeObject,
						ObjectVal: map[string]string{"c": "$(params.c[*])"},
					},
				},
			},
		},
		{
			name: "with-overlay",
			overlay: []Param{{
				Name:  "a",
				Value: *NewArrayOrString("tacocat"),
			}, {
				Name:  "c",
				Value: *NewObject(map[string]string{"key2": "val2"}),
			}},
			want: []Param{
				{
					Name:  "a",
					Value: *NewArrayOrString("tacocat"),
				},
				{
					Name: "b",
					Value: ArrayOrString{
						Type:     ParamTypeArray,
						ArrayVal: []string{"$(params.b[*])"},
					},
				},
				{
					Name: "c",
					Value: ArrayOrString{
						Type:      ParamTypeObject,
						ObjectVal: map[string]string{"key2": "val2"},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := getContextParams(ctx, tc.overlay...)
			if diff := cmp.Diff(tc.want, got, cmpopts.SortSlices(func(x, y Param) bool { return x.Name < y.Name })); diff != "" {
				t.Errorf("-want,+got: %s", diff)
			}
		})
	}
}

func TestGetContextParamSpecs(t *testing.T) {
	ctx := context.Background()
	want := []ParamSpec{
		{
			Name:        "a",
			Type:        ParamTypeString,
			Default:     NewArrayOrString("foo"),
			Description: "tacocat",
		},
		{
			Name:        "b",
			Type:        ParamTypeArray,
			Default:     NewArrayOrString("bar"),
			Description: "racecar",
		},
		{
			Name:        "c",
			Type:        ParamTypeObject,
			Properties:  map[string]PropertySpec{"key1": {}},
			Default:     NewObject(map[string]string{"key1": "val1"}),
			Description: "my object",
		},
	}

	ctx = addContextParamSpec(ctx, want)
	got := getContextParamSpecs(ctx)
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(x, y ParamSpec) bool { return x.Name < y.Name })); diff != "" {
		t.Errorf("-want,+got: %s", diff)
	}
}
