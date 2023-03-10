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

package v1beta1

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
		want: []Params{},
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
			Params: Params{{
				Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: IncludeParamsList{{}},
		},
		want: []Params{{
			{
				Name:  "GOARCH",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: ParamValue{Type: ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: ParamValue{Type: ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: ParamValue{Type: ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: ParamValue{Type: ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: ParamValue{Type: ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: ParamValue{Type: ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: ParamValue{Type: ParamTypeString, StringVal: "go1.18.1"},
			},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := cmp.Diff(tt.want, tt.matrix.FanOut()); d != "" {
				t.Errorf("Combinations of Parameters did not match the expected Params: %s", diff.PrintWantGot(d))
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
				Include: IncludeParamsList{{
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
				Params: Params{{
					Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: IncludeParamsList{{
					Name: "common-package",
					Params: Params{{
						Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
				}},
			},
			want: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(tc.want, tc.matrix.HasParams()); d != "" {
				t.Errorf("matrix.HasParams() bool diff %s", diff.PrintWantGot(d))
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
				Params: Params{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: false,
		}, {
			name: "matrixed with include",
			matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: true,
		}, {
			name: "matrixed with params and include",
			matrix: &Matrix{
				Params: Params{{
					Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: IncludeParamsList{{
					Name: "common-package",
					Params: Params{{
						Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
				}},
			},
			want: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(tc.want, tc.matrix.HasInclude()); d != "" {
				t.Errorf("matrix.HasInclude() bool diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestMatrix_GetAllParams(t *testing.T) {
	testCases := []struct {
		name   string
		matrix *Matrix
		want   Params
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   nil,
		},
		{
			name:   "empty matrix",
			matrix: &Matrix{},
			want:   nil,
		},
		{
			name: "matrixed with params",
			matrix: &Matrix{
				Params: Params{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: Params{{Name: "platform", Value: ParamValue{ArrayVal: []string{"linux", "windows"}}}},
		}, {
			name: "matrixed with include",
			matrix: &Matrix{
				Include: IncludeParamsList{{
					Name: "build-1",
					Params: Params{{
						Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: Params{{
				Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
			}, {
				Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
		},
		{
			name: "matrixed with params and include",
			matrix: &Matrix{
				Params: Params{{
					Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: IncludeParamsList{{
					Name: "common-package",
					Params: Params{{
						Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
				}},
			},
			want: Params{{
				Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"},
			}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(tc.want, tc.matrix.GetAllParams()); d != "" {
				t.Errorf("matrix.GetAllParams() bool diff %s", diff.PrintWantGot(d))
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
			Params: Params{{}}},
		want: 0,
	}, {
		name: "combinations count is one from one parameter",
		matrix: &Matrix{
			Params: Params{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
			}}},

		want: 1,
	}, {
		name: "combinations count is one from two parameters",
		matrix: &Matrix{
			Params: Params{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo"}},
			}, {
				Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"bar"}},
			}}},
		want: 1,
	}, {
		name: "combinations count is two from one parameter",
		matrix: &Matrix{
			Params: Params{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}}},
		want: 2,
	}, {
		name: "combinations count is nine",
		matrix: &Matrix{
			Params: Params{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}}},
		want: 9,
	}, {
		name: "combinations count is large",
		matrix: &Matrix{
			Params: Params{{
				Name: "foo", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}, {
				Name: "quz", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"q", "u", "x"}},
			}, {
				Name: "xyzzy", Value: ParamValue{Type: ParamTypeArray, ArrayVal: []string{"x", "y", "z", "z", "y"}},
			}}},
		want: 135,
	}, {
		name: "explicit combinations in the matrix",
		matrix: &Matrix{
			Include: IncludeParamsList{{
				Name: "build-1",
				Params: []Param{{
					Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-1"},
				}, {
					Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile1"},
				}},
			}, {
				Name: "build-2",
				Params: []Param{{
					Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-2"},
				}, {
					Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile2"},
				}},
			}, {
				Name: "build-3",
				Params: []Param{{
					Name: "IMAGE", Value: ParamValue{Type: ParamTypeString, StringVal: "image-3"},
				}, {
					Name: "DOCKERFILE", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/Dockerfile3"},
				}},
			}},
		},
		want: 3,
	}, {
		name: "params and include in matrix with overriding combinations params",
		matrix: &Matrix{
			Params: []Param{{
				Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: IncludeParamsList{{
				Name: "common-package",
				Params: []Param{{
					Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
			}, {
				Name: "s390x-no-race",
				Params: []Param{{
					Name: "GOARCH", Value: ParamValue{Type: ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: ParamValue{Type: ParamTypeString, StringVal: "-cover -v"}}},
			}, {
				Name: "go117-context",
				Params: []Param{{
					Name: "version", Value: ParamValue{Type: ParamTypeString, StringVal: "go1.17"},
				}, {
					Name: "context", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/go117/context"}}},
			}},
		},
		want: 6,
	}, {
		name: "params and include in matrix with overriding combinations params and one new combination",
		matrix: &Matrix{
			Params: []Param{{
				Name: "GOARCH", Value: ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: IncludeParamsList{{
				Name: "common-package",
				Params: []Param{{
					Name: "package", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/common/package/"}}},
			}, {
				Name: "s390x-no-race",
				Params: []Param{{
					Name: "GOARCH", Value: ParamValue{Type: ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: ParamValue{Type: ParamTypeString, StringVal: "-cover -v"}}},
			}, {
				Name: "go117-context",
				Params: []Param{{
					Name: "version", Value: ParamValue{Type: ParamTypeString, StringVal: "go1.17"},
				}, {
					Name: "context", Value: ParamValue{Type: ParamTypeString, StringVal: "path/to/go117/context"}}},
			}, {
				Name: "non-existent-arch",
				Params: []Param{{
					Name: "GOARCH", Value: ParamValue{Type: ParamTypeString, StringVal: "I-do-not-exist"}},
				}},
			}},
		want: 7,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := cmp.Diff(tt.want, tt.matrix.CountCombinations()); d != "" {
				t.Errorf("Matrix.CountCombinations() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
