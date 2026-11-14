/*
Copyright 2026 The Tekton Authors

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

package spire

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/result"
)

func TestResultValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{{
		name: "empty",
	}, {
		name:  "plain string",
		value: "value",
		want:  "value",
	}, {
		name:  "JSON string",
		value: `"quoted\nvalue"`,
		want:  "quoted\nvalue",
	}, {
		name:  "array",
		value: `["left","right"]`,
		want:  "left,right",
	}, {
		name:  "object keys are sorted",
		value: `{"z":"last","a":"first"}`,
		want:  "a,first,z,last",
	}, {
		name:  "non-string array remains a string",
		value: `[1,2]`,
		want:  `[1,2]`,
	}, {
		name:  "non-string object remains a string",
		value: `{"key":1}`,
		want:  `{"key":1}`,
	}, {
		name:  "malformed JSON remains a string",
		value: `{"key":`,
		want:  `{"key":`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResultValue(result.RunResult{Key: "answer", Value: tc.value})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("ResultValue() = %q, want %q", got, tc.want)
			}
		})
	}
}
