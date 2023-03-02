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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestMatrix_FanOut(t *testing.T) {
	tests := []struct {
		name   string
		matrix Matrix
		want   []Params
	}{{
		name: "matrix with no params",
		matrix: Matrix{
			Params: Params{},
		},
		want: nil,
	}, {
		name: "single array in matrix",
		matrix: Matrix{
			Params: Params{{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}},
		},
		want: []Params{{
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "mac"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "windows"},
			},
		}},
	}, {
		name: "multiple arrays in matrix",
		matrix: Matrix{
			Params: []Param{{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"chrome", "safari", "firefox"}},
			}}},
		want: []Params{{
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "chrome"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "chrome"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "chrome"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "safari"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "safari"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "safari"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "firefox"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "firefox"},
			},
		}, {
			{
				Name:  "platform",
				Value: ParamValue{Type: ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: ParamValue{Type: ParamTypeString, StringVal: "firefox"},
			},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := cmp.Diff(tt.want, tt.matrix.FanOut()); d != "" {
				t.Errorf("Combinations of Parameters did not match the expected Params: %s", d)
			}
		})
	}
}

func TestMatrix_HasParams(t *testing.T) {
	testCases := []struct {
		name   string
		matrix *Matrix
		want   bool
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   false,
		},
		{
			name:   "empty matrix",
			matrix: &Matrix{},
			want:   false,
		},
		{
			name: "matrixed with params",
			matrix: &Matrix{
				Params: []Param{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: true,
		}, {
			name: "matrixed with include",
			matrix: &Matrix{
				Include: []MatrixInclude{{
					Name: "build-1",
					Params: []Param{{
						Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: false,
		}, {
			name: "matrixed with params and include",
			matrix: &Matrix{
				Params: []Param{{
					Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: []MatrixInclude{{
					Name: "common-package",
					Params: []Param{{
						Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
				}},
			},
			want: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(tc.want, tc.matrix.hasParams()); d != "" {
				t.Errorf("matrix.hasParams() bool diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestMatrix_HasInclude(t *testing.T) {
	testCases := []struct {
		name   string
		matrix *Matrix
		want   bool
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   false,
		},
		{
			name:   "empty matrix",
			matrix: &Matrix{},
			want:   false,
		},
		{
			name: "matrixed with params",
			matrix: &Matrix{
				Params: []Param{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: false,
		}, {
			name: "matrixed with include",
			matrix: &Matrix{
				Include: []MatrixInclude{{
					Name: "build-1",
					Params: []Param{{
						Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: true,
		}, {
			name: "matrixed with params and include",
			matrix: &Matrix{
				Params: []Param{{
					Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: []MatrixInclude{{
					Name: "common-package",
					Params: []Param{{
						Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
				}},
			},
			want: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(tc.want, tc.matrix.hasInclude()); d != "" {
				t.Errorf("matrix.hasInclude() bool diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestPipelineTask_CountCombinations(t *testing.T) {
	tests := []struct {
		name   string
		matrix *Matrix
		want   int
	}{{
		name: "combinations count is zero",
		matrix: &Matrix{
			Params: []Param{{}}},
		want: 0,
	}, {
		name: "combinations count is one from one parameter",
		matrix: &Matrix{
			Params: []Param{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
			}}},

		want: 1,
	}, {
		name: "combinations count is one from two parameters",
		matrix: &Matrix{
			Params: []Param{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
			}, {
				Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar"}},
			}}},
		want: 1,
	}, {
		name: "combinations count is two from one parameter",
		matrix: &Matrix{
			Params: []Param{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}}},
		want: 2,
	}, {
		name: "combinations count is nine",
		matrix: &Matrix{
			Params: []Param{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}}},
		want: 9,
	}, {
		name: "combinations count is large",
		matrix: &Matrix{
			Params: []Param{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}, {
				Name: "quz", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"q", "u", "x"}},
			}, {
				Name: "xyzzy", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"x", "y", "z", "z", "y"}},
			}}},
		want: 135,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := cmp.Diff(tt.want, tt.matrix.CountCombinations()); d != "" {
				t.Errorf("Matrix.CountCombinations() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
