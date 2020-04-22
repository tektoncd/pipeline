/*
Copyright 2019 The Tekton Authors
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

package builder_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	params "github.com/tektoncd/pipeline/pkg/apis/params/v1beta1"
	"github.com/tektoncd/pipeline/test/builder"
)

func TestGenerateString(t *testing.T) {
	value := builder.ArrayOrString("somestring")
	expectedValue := &params.ArrayOrString{
		Type:      params.ParamTypeString,
		StringVal: "somestring",
	}
	if d := cmp.Diff(expectedValue, value); d != "" {
		t.Fatalf("ArrayOrString diff -want, +got: %v", d)
	}
}

func TestGenerateArray(t *testing.T) {
	value := builder.ArrayOrString("some", "array", "elements")
	expectedValue := &params.ArrayOrString{
		Type:     params.ParamTypeArray,
		ArrayVal: []string{"some", "array", "elements"},
	}
	if d := cmp.Diff(expectedValue, value); d != "" {
		t.Fatalf("ArrayOrString diff -want, +got: %v", d)
	}
}
