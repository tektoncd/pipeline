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

package internalversion_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/internalversion"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestMatrix_FanOut(t *testing.T) {
	tests := []struct {
		name   string
		matrix internalversion.Matrix
		want   []internalversion.Params
	}{{
		name: "matrix with no params",
		matrix: internalversion.Matrix{
			Params: internalversion.Params{},
		},
		want: []internalversion.Params{},
	}, {
		name: "single array in matrix",
		matrix: internalversion.Matrix{
			Params: internalversion.Params{{
				Name:  "platform",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"linux", "mac", "windows"}},
			}},
		},
		want: []internalversion.Params{{
			{
				Name:  "platform",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux"},
			},
		}, {
			{
				Name:  "platform",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "mac"},
			},
		}, {
			{
				Name:  "platform",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "windows"},
			},
		}},
	}, {
		name: "multiple arrays in matrix",
		matrix: internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: internalversion.IncludeParamsList{{}},
		},
		want: []internalversion.Params{{
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}},
	}, {
		name: "Fan out explicit combinations, no matrix params",
		matrix: internalversion.Matrix{
			Include: internalversion.IncludeParamsList{{
				Name: "build-1",
				Params: internalversion.Params{{
					Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
				}, {
					Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
			}, {
				Name: "build-2",
				Params: internalversion.Params{{
					Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-2"},
				}, {
					Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile2"}}},
			}, {
				Name: "build-3",
				Params: internalversion.Params{{
					Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-3"},
				}, {
					Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile3"}}},
			}},
		},
		want: []internalversion.Params{{
			{
				Name:  "DOCKERFILE",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"},
			}, {
				Name:  "IMAGE",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
			},
		}, {
			{
				Name:  "DOCKERFILE",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile2"},
			}, {
				Name:  "IMAGE",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-2"},
			},
		}, {
			{
				Name:  "DOCKERFILE",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile3"},
			}, {
				Name:  "IMAGE",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-3"},
			},
		}},
	}, {
		name: "matrix include unknown param name, append to all combinations",
		matrix: internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: internalversion.IncludeParamsList{{
				Name: "common-package",
				Params: internalversion.Params{{
					Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}},
			}},
		},
		want: []internalversion.Params{{
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "package",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "package",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "package",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "package",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "package",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "package",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}},
	}, {
		name: "matrix include param value does not exist, generate a new combination",
		matrix: internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: internalversion.IncludeParamsList{{
				Name: "non-existent-arch",
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "I-do-not-exist"}}},
			}},
		},
		want: []internalversion.Params{{
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "I-do-not-exist"},
			},
		}},
	}, {
		name: "Matrix include filters single parameter and appends missing values",
		matrix: internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: internalversion.IncludeParamsList{{
				Name: "s390x-no-race",
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"}}},
			}},
		},
		want: []internalversion.Params{{
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {

			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "flags",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}, {
			{
				Name:  "GOARCH",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
			}, {
				Name:  "flags",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"},
			}, {
				Name:  "version",
				Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
			},
		}},
	},
		{
			name: "Matrix include filters multiple parameters and append new parameters",
			matrix: internalversion.Matrix{
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}, {
					Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
				},
				Include: internalversion.IncludeParamsList{
					{
						Name: "390x-no-race",
						Params: internalversion.Params{{
							Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"}}, {
							Name: "flags", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"}}, {
							Name: "version", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"}}},
					},
					{
						Name: "amd64-no-race",
						Params: internalversion.Params{{
							Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"}}, {
							Name: "flags", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"}}, {
							Name: "version", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"}}},
					},
				},
			},
			want: []internalversion.Params{{
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "flags",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "flags",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
				},
			}},
		}, {
			name: "Matrix params and include params handles filter, appending, and generating new combinations at once",
			matrix: internalversion.Matrix{
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}, {
					Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
				},
				Include: internalversion.IncludeParamsList{{
					Name: "common-package",
					Params: internalversion.Params{{
						Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}},
				}, {
					Name: "s390x-no-race",
					Params: internalversion.Params{{
						Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
					}, {
						Name: "flags", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"}}},
				}, {
					Name: "go117-context",
					Params: internalversion.Params{{
						Name: "version", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
					}, {
						Name: "context", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/go117/context"}}},
				}, {
					Name: "non-existent-arch",
					Params: internalversion.Params{{
						Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "I-do-not-exist"}},
					},
				}},
			},
			want: []internalversion.Params{{
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "context",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/go117/context"},
				}, {
					Name:  "package",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "context",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/go117/context"},
				}, {
					Name:  "package",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "context",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/go117/context"},
				}, {
					Name:  "flags",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "package",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/amd64"},
				}, {
					Name:  "package",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/ppc64le"},
				}, {
					Name:  "package",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name:  "flags",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"},
				}, {
					Name:  "package",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
				}, {
					Name:  "version",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.18.1"},
				},
			}, {
				{
					Name:  "GOARCH",
					Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "I-do-not-exist"},
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
		matrix *internalversion.Matrix
		want   bool
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   false,
		},
		{
			name:   "empty matrix",
			matrix: &internalversion.Matrix{},
			want:   false,
		},
		{
			name: "matrixed with params",
			matrix: &internalversion.Matrix{
				Params: internalversion.Params{{Name: "platform", Value: internalversion.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: true,
		}, {
			name: "matrixed with include",
			matrix: &internalversion.Matrix{
				Include: internalversion.IncludeParamsList{{
					Name: "build-1",
					Params: internalversion.Params{{
						Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: false,
		}, {
			name: "matrixed with params and include",
			matrix: &internalversion.Matrix{
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: internalversion.IncludeParamsList{{
					Name: "common-package",
					Params: internalversion.Params{{
						Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}},
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
		matrix *internalversion.Matrix
		want   bool
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   false,
		},
		{
			name:   "empty matrix",
			matrix: &internalversion.Matrix{},
			want:   false,
		},
		{
			name: "matrixed with params",
			matrix: &internalversion.Matrix{
				Params: internalversion.Params{{Name: "platform", Value: internalversion.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: false,
		}, {
			name: "matrixed with include",
			matrix: &internalversion.Matrix{
				Include: internalversion.IncludeParamsList{{
					Name: "build-1",
					Params: internalversion.Params{{
						Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: true,
		}, {
			name: "matrixed with params and include",
			matrix: &internalversion.Matrix{
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: internalversion.IncludeParamsList{{
					Name: "common-package",
					Params: internalversion.Params{{
						Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}},
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
		matrix *internalversion.Matrix
		want   internalversion.Params
	}{
		{
			name:   "nil matrix",
			matrix: nil,
			want:   nil,
		},
		{
			name:   "empty matrix",
			matrix: &internalversion.Matrix{},
			want:   nil,
		},
		{
			name: "matrixed with params",
			matrix: &internalversion.Matrix{
				Params: internalversion.Params{{Name: "platform", Value: internalversion.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
			},
			want: internalversion.Params{{Name: "platform", Value: internalversion.ParamValue{ArrayVal: []string{"linux", "windows"}}}},
		}, {
			name: "matrixed with include",
			matrix: &internalversion.Matrix{
				Include: internalversion.IncludeParamsList{{
					Name: "build-1",
					Params: internalversion.Params{{
						Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
					}, {
						Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
				}},
			},
			want: internalversion.Params{{
				Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
			}, {
				Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"}}},
		},
		{
			name: "matrixed with params and include",
			matrix: &internalversion.Matrix{
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
				}},
				Include: internalversion.IncludeParamsList{{
					Name: "common-package",
					Params: internalversion.Params{{
						Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}},
				}},
			},
			want: internalversion.Params{{
				Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"},
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
		matrix *internalversion.Matrix
		want   int
	}{{
		name: "combinations count is zero",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{}}},
		want: 0,
	}, {
		name: "combinations count is one from one parameter",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "foo", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"foo"}},
			}}},

		want: 1,
	}, {
		name: "combinations count is one from two parameters",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "foo", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"foo"}},
			}, {
				Name: "bar", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"bar"}},
			}}},
		want: 1,
	}, {
		name: "combinations count is two from one parameter",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "foo", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"foo", "bar"}},
			}}},
		want: 2,
	}, {
		name: "combinations count is nine",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "foo", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}}},
		want: 9,
	}, {
		name: "combinations count is large",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "foo", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"f", "o", "o"}},
			}, {
				Name: "bar", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"b", "a", "r"}},
			}, {
				Name: "quz", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"q", "u", "x"}},
			}, {
				Name: "xyzzy", Value: internalversion.ParamValue{Type: internalversion.ParamTypeArray, ArrayVal: []string{"x", "y", "z", "z", "y"}},
			}}},
		want: 135,
	}, {
		name: "explicit combinations in the matrix",
		matrix: &internalversion.Matrix{
			Include: internalversion.IncludeParamsList{{
				Name: "build-1",
				Params: internalversion.Params{{
					Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-1"},
				}, {
					Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile1"},
				}},
			}, {
				Name: "build-2",
				Params: internalversion.Params{{
					Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-2"},
				}, {
					Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile2"},
				}},
			}, {
				Name: "build-3",
				Params: internalversion.Params{{
					Name: "IMAGE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "image-3"},
				}, {
					Name: "DOCKERFILE", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/Dockerfile3"},
				}},
			}},
		},
		want: 3,
	}, {
		name: "params and include in matrix with overriding combinations params",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: internalversion.IncludeParamsList{{
				Name: "common-package",
				Params: internalversion.Params{{
					Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}},
			}, {
				Name: "s390x-no-race",
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"}}},
			}, {
				Name: "go117-context",
				Params: internalversion.Params{{
					Name: "version", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				}, {
					Name: "context", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/go117/context"}}},
			}},
		},
		want: 6,
	}, {
		name: "params and include in matrix with overriding combinations params and one new combination",
		matrix: &internalversion.Matrix{
			Params: internalversion.Params{{
				Name: "GOARCH", Value: internalversion.ParamValue{ArrayVal: []string{"linux/amd64", "linux/ppc64le", "linux/s390x"}},
			}, {
				Name: "version", Value: internalversion.ParamValue{ArrayVal: []string{"go1.17", "go1.18.1"}}},
			},
			Include: internalversion.IncludeParamsList{{
				Name: "common-package",
				Params: internalversion.Params{{
					Name: "package", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/common/package/"}}},
			}, {
				Name: "s390x-no-race",
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "linux/s390x"},
				}, {
					Name: "flags", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "-cover -v"}}},
			}, {
				Name: "go117-context",
				Params: internalversion.Params{{
					Name: "version", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "go1.17"},
				}, {
					Name: "context", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "path/to/go117/context"}}},
			}, {
				Name: "non-existent-arch",
				Params: internalversion.Params{{
					Name: "GOARCH", Value: internalversion.ParamValue{Type: internalversion.ParamTypeString, StringVal: "I-do-not-exist"}},
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
