/*
Copyright 2022 The Tekton Authors
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

package matrix

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func Test_ToMap(t *testing.T) {
	tests := []struct {
		name         string
		combinations Combinations
		want         map[string][]v1beta1.Param
	}{{
		name: "one array in matrix",
		combinations: Combinations{{
			MatrixID: "0",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}},
		}, {
			MatrixID: "1",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}},
		}, {
			MatrixID: "2",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}},
		}},
		want: map[string][]v1beta1.Param{
			"0": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}},
			"1": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}},
			"2": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}},
		},
	}, {
		name: "multiple arrays in matrix",
		combinations: Combinations{{
			MatrixID: "0",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "chrome"},
			}},
		}, {
			MatrixID: "1",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "chrome"},
			}},
		}, {
			MatrixID: "2",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "chrome"},
			}},
		}, {
			MatrixID: "3",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "safari"},
			}},
		}, {
			MatrixID: "4",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "safari"},
			}},
		}, {
			MatrixID: "5",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "safari"},
			}},
		}, {
			MatrixID: "6",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "firefox"},
			}},
		}, {
			MatrixID: "7",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "firefox"},
			}},
		}, {
			MatrixID: "8",
			Params: []v1beta1.Param{{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "firefox"},
			}},
		}},
		want: map[string][]v1beta1.Param{
			"0": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "chrome"},
			}},
			"1": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "chrome"},
			}},
			"2": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "chrome"},
			}},
			"3": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "safari"},
			}},
			"4": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "safari"},
			}},
			"5": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "safari"},
			}},
			"6": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "linux"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "firefox"},
			}},
			"7": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "mac"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "firefox"},
			}},
			"8": {{
				Name:  "platform",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "windows"},
			}, {
				Name:  "browser",
				Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "firefox"},
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := cmp.Diff(tt.want, tt.combinations.ToMap()); d != "" {
				t.Errorf("Map of Combinations of Parameters did not match the expected Map %s", d)
			}
		})
	}
}
