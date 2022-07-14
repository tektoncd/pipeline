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

package v1beta1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestTaskResult_SetDefaults(t *testing.T) {
	tests := []struct {
		name   string
		before *v1beta1.TaskResult
		after  *v1beta1.TaskResult
	}{{
		name:   "empty taskresult",
		before: nil,
		after:  nil,
	}, {
		name: "inferred string type",
		before: &v1beta1.TaskResult{
			Name: "resultname",
		},
		after: &v1beta1.TaskResult{
			Name: "resultname",
			Type: v1beta1.ResultsTypeString,
		},
	}, {
		name: "string type specified not changed",
		before: &v1beta1.TaskResult{
			Name: "resultname",
			Type: v1beta1.ResultsTypeString,
		},
		after: &v1beta1.TaskResult{
			Name: "resultname",
			Type: v1beta1.ResultsTypeString,
		},
	}, {
		name: "array type specified not changed",
		before: &v1beta1.TaskResult{
			Name: "resultname",
			Type: v1beta1.ResultsTypeArray,
		},
		after: &v1beta1.TaskResult{
			Name: "resultname",
			Type: v1beta1.ResultsTypeArray,
		},
	}, {
		name: "inferred object type from properties - PropertySpec type is provided",
		before: &v1beta1.TaskResult{
			Name:       "resultname",
			Properties: map[string]v1beta1.PropertySpec{"key1": {v1beta1.ParamTypeString}},
		},
		after: &v1beta1.TaskResult{
			Name:       "resultname",
			Type:       v1beta1.ResultsTypeObject,
			Properties: map[string]v1beta1.PropertySpec{"key1": {v1beta1.ParamTypeString}},
		},
	}, {
		name: "inferred type from properties - PropertySpec type is not provided",
		before: &v1beta1.TaskResult{
			Name:       "resultname",
			Properties: map[string]v1beta1.PropertySpec{"key1": {}},
		},
		after: &v1beta1.TaskResult{
			Name:       "resultname",
			Type:       v1beta1.ResultsTypeObject,
			Properties: map[string]v1beta1.PropertySpec{"key1": {v1beta1.ParamTypeString}},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tc.before.SetDefaults(ctx)
			if d := cmp.Diff(tc.before, tc.after); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
