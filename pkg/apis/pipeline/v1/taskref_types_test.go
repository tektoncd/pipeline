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

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func TestTaskRef_IsCustomTask(t *testing.T) {
	tests := []struct {
		name string
		tr   *v1.TaskRef
		want bool
	}{{
		name: "not a custom task - apiVersion and Kind are not set",
		tr: &v1.TaskRef{
			Name: "foo",
		},
		want: false,
	}, {
		// related issue: https://github.com/tektoncd/pipeline/issues/6459
		name: "not a custom task - apiVersion is not set",
		tr: &v1.TaskRef{
			Name: "foo",
			Kind: "Example",
		},
		want: false,
	}, {
		name: "not a custom task - kind is not set",
		tr: &v1.TaskRef{
			Name:       "foo",
			APIVersion: "example/v0",
		},
		want: false,
	}, {
		name: "custom task with name",
		tr: &v1.TaskRef{
			Name:       "foo",
			Kind:       "Example",
			APIVersion: "example/v0",
		},
		want: true,
	}, {
		name: "custom task without name",
		tr: &v1.TaskRef{
			Kind:       "Example",
			APIVersion: "example/v0",
		},
		want: true,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tr.IsCustomTask(); got != tt.want {
				t.Errorf("IsCustomTask() = %v, want %v", got, tt.want)
			}
		})
	}
}
