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

package v1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestMatrix_FanOut(t *testing.T) {
	tests := []struct {
		name   string
		matrix v1.Matrix
		want   []v1.Params
	}{{
		name: "matrix with no params",
		matrix: v1.Matrix{
			Params: v1.Params{},
		},
		want: []v1.Params{},
	}, {
		name: "single array in matrix",
		matrix: v1.Matrix{
			Params: v1.Params{{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}},
		},
		want: []v1.Params{{
			{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux"},
			},
		}, {
			{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "mac"},
			},
		}, {
			{
				Name:  "platform",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "windows"},
			},
		}},
	}, {
		name: "multiple arrays in matrix",
		matrix: v1.Matrix{
			Params: v1.Params{{
				Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: v1.IncludeParamsList{{}},
		},
		want: []v1.Params{{
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}},
	}, {
		name: "Fan out explicit combinations, no matrix params",
		matrix: v1.Matrix{
			Include: v1.IncludeParamsList{{
				Name: "build-1",
				Params: v1.Params{{
					Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-1"},
				}, {
					Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
			}, {
				Name: "build-2",
				Params: v1.Params{{
					Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-2"},
				}, {
					Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile2"}}},
			}, {
				Name: "build-3",
				Params: v1.Params{{
					Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-3"},
				}, {
					Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile3"}}},
			}},
		},
		want: []v1.Params{{
			{
				Name:  "DOCKERFILE",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile1"},
			}, {
				Name:  "IMAGE",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-1"},
			},
		}, {
			{
				Name:  "DOCKERFILE",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile2"},
			}, {
				Name:  "IMAGE",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-2"},
			},
		}, {
			{
				Name:  "DOCKERFILE",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile3"},
			}, {
				Name:  "IMAGE",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-3"},
			},
		}},
	}, {
		name: "matrix include unknown param name, append to all combinations",
		matrix: v1.Matrix{
			Params: v1.Params{{
				Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: v1.IncludeParamsList{{
				Name: "common-package",
				Params: v1.Params{{
					Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}},
			}},
		},
		want: []v1.Params{{
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "package",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}},
	}, {
		name: "matrix include param value does not exist, generate a new combination",
		matrix: v1.Matrix{
			Params: v1.Params{{
				Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: v1.IncludeParamsList{{
				Name: "non-existent-arch",
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist"}}},
			}},
		},
		want: []v1.Params{{
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist"},
			},
		}},
	}, {
		name: "Matrix include filters single parameter and appends missing values",
		matrix: v1.Matrix{
			Params: v1.Params{{
				Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: v1.IncludeParamsList{{
				Name: "s390x-no-race",
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"}}},
			}},
		},
		want: []v1.Params{{
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {

			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "flags",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "flags",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"},
			}, {
				Name:  "version",
				Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
			},
		}},
	},
		{
			name: "Matrix include filters multiple parameters and append new parameters",
			matrix: v1.Matrix{
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}, {
					Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
				},
				Include: v1.IncludeParamsList{
					{
						Name: "390x-no-race",
						Params: v1.Params{{
							Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"}}, {
							Name: "flags", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"}}, {
							Name: "version", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"}}},
					},
					{
						Name: "amd64-no-race",
						Params: v1.Params{{
							Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"}}, {
							Name: "flags", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"}}, {
							Name: "version", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"}}},
					},
				},
			},
			want: []v1.Params{{
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "flags",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "flags",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
				},
			}},
		}, {
			name: "Matrix params and include params handles filter, appending, and generating new combinations at once",
			matrix: v1.Matrix{
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}, {
					Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
				},
				Include: v1.IncludeParamsList{{
					Name: "common-package",
					Params: v1.Params{{
						Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}},
				}, {
					Name: "s390x-no-race",
					Params: v1.Params{{
						Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
					}, {
						Name: "flags", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"}}},
				}, {
					Name: "go117-context",
					Params: v1.Params{{
						Name: "version", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
					}, {
						Name: "context", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"}}},
				}, {
					Name: "non-existent-arch",
					Params: v1.Params{{
						Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist"}},
					},
				}},
			},
			want: []v1.Params{{
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "context",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"},
				}, {
					Name:  "package",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "context",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"},
				}, {
					Name:  "package",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "context",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"},
				}, {
					Name:  "flags",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "package",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "package",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "package",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "flags",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "package",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist"},
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
		matrix *v1.Matrix
		want   bool
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   false,
		},
		{
			name:   "empty matrix",
			matrix: &v1.Matrix{},
			want:   false,
		},
		{
			name: "matrixed with params",
			matrix: &v1.Matrix{
				Params: v1.Params{{Name: "platform", Value: v1.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: true,
		}, {
			name: "matrixed with include",
			matrix: &v1.Matrix{
				Include: v1.IncludeParamsList{{
					Name: "build-1",
					Params: v1.Params{{
						Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: false,
		}, {
			name: "matrixed with params and include",
			matrix: &v1.Matrix{
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: v1.IncludeParamsList{{
					Name: "common-package",
					Params: v1.Params{{
						Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}},
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
		matrix *v1.Matrix
		want   bool
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   false,
		},
		{
			name:   "empty matrix",
			matrix: &v1.Matrix{},
			want:   false,
		},
		{
			name: "matrixed with params",
			matrix: &v1.Matrix{
				Params: v1.Params{{Name: "platform", Value: v1.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: false,
		}, {
			name: "matrixed with include",
			matrix: &v1.Matrix{
				Include: v1.IncludeParamsList{{
					Name: "build-1",
					Params: v1.Params{{
						Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: true,
		}, {
			name: "matrixed with params and include",
			matrix: &v1.Matrix{
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: v1.IncludeParamsList{{
					Name: "common-package",
					Params: v1.Params{{
						Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}},
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
		matrix *v1.Matrix
		want   v1.Params
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   nil,
		},
		{
			name:   "empty matrix",
			matrix: &v1.Matrix{},
			want:   nil,
		},
		{
			name: "matrixed with params",
			matrix: &v1.Matrix{
				Params: v1.Params{{Name: "platform", Value: v1.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: v1.Params{{Name: "platform", Value: v1.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
		}, {
			name: "matrixed with include",
			matrix: &v1.Matrix{
				Include: v1.IncludeParamsList{{
					Name: "build-1",
					Params: v1.Params{{
						Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: v1.Params{{
				Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-1"},
			}, {
				Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
		},
		{
			name: "matrixed with params and include",
			matrix: &v1.Matrix{
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: v1.IncludeParamsList{{
					Name: "common-package",
					Params: v1.Params{{
						Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}},
				}},
			},
			want: v1.Params{{
				Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"},
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
		matrix *v1.Matrix
		want   int
	}{{
		name: "combinations count is zero",
		matrix: &v1.Matrix{
			Params: v1.Params{{}}},
		want: 0,
	}, {
		name: "combinations count is one from one parameter",
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "foo", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"foo"}},
			}}},

		want: 1,
	}, {
		name: "combinations count is one from two parameters",
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "foo", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"foo"}},
			}, {
				Name: "bar", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"bar"}},
			}}},
		want: 1,
	}, {
		name: "combinations count is two from one parameter",
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "foo", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}}},
		want: 2,
	}, {
		name: "combinations count is nine",
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "foo", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}}},
		want: 9,
	}, {
		name: "combinations count is large",
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "foo", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}, {
				Name: "quz", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"q", "u", "x"}},
			}, {
				Name: "xyzzy", Value: v1.ParamValue{Type: v1.ParamTypeArray, ArrayVal: []string{"x", "y", "z", "z", "y"}},
			}}},
		want: 135,
	}, {
		name: "explicit combinations in the matrix",
		matrix: &v1.Matrix{
			Include: v1.IncludeParamsList{{
				Name: "build-1",
				Params: v1.Params{{
					Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-1"},
				}, {
					Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile1"},
				}},
			}, {
				Name: "build-2",
				Params: v1.Params{{
					Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-2"},
				}, {
					Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile2"},
				}},
			}, {
				Name: "build-3",
				Params: v1.Params{{
					Name: "IMAGE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "image-3"},
				}, {
					Name: "DOCKERFILE", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/Dockerfile3"},
				}},
			}},
		},
		want: 3,
	}, {
		name: "params and include in matrix with overriding combinations params",
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: v1.IncludeParamsList{{
				Name: "common-package",
				Params: v1.Params{{
					Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}},
			}, {
				Name: "s390x-no-race",
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"}}},
			}, {
				Name: "go117-context",
				Params: v1.Params{{
					Name: "version", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				}, {
					Name: "context", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"}}},
			}},
		},
		want: 6,
	}, {
		name: "params and include in matrix with overriding combinations params and one new combination",
		matrix: &v1.Matrix{
			Params: v1.Params{{
				Name: "GOARCH", Value: v1.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: v1.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: v1.IncludeParamsList{{
				Name: "common-package",
				Params: v1.Params{{
					Name: "package", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/common/package/"}}},
			}, {
				Name: "s390x-no-race",
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "-cover -v"}}},
			}, {
				Name: "go117-context",
				Params: v1.Params{{
					Name: "version", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "go1.17"},
				}, {
					Name: "context", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "path/to/go117/context"}}},
			}, {
				Name: "non-existent-arch",
				Params: v1.Params{{
					Name: "GOARCH", Value: v1.ParamValue{Type: v1.ParamTypeString, StringVal: "I-do-not-exist"}},
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
