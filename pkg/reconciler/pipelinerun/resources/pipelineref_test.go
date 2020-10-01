/*
 Copyright 2020 The Tekton Authors

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

package resources_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPipelineRef(t *testing.T) {
	testcases := []struct {
		name      string
		pipelines []runtime.Object
		ref       *v1beta1.PipelineRef
		expected  runtime.Object
		wantErr   bool
	}{
		{
			name: "local-pipeline",
			pipelines: []runtime.Object{
				tb.Pipeline("simple", tb.PipelineNamespace("default")),
				tb.Pipeline("dummy", tb.PipelineNamespace("default")),
			},
			ref: &v1beta1.PipelineRef{
				Name: "simple",
			},
			expected: tb.Pipeline("simple", tb.PipelineNamespace("default")),
			wantErr:  false,
		},
		{
			name:      "pipeline-not-found",
			pipelines: []runtime.Object{},
			ref: &v1beta1.PipelineRef{
				Name: "simple",
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			tektonclient := fake.NewSimpleClientset(tc.pipelines...)

			lc := &resources.LocalPipelineRefResolver{
				Namespace:    "default",
				Tektonclient: tektonclient,
			}

			task, err := lc.GetPipeline(ctx, tc.ref.Name)
			if tc.wantErr && err == nil {
				t.Fatal("Expected error but found nil instead")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("Received unexpected error ( %#v )", err)
			}

			if d := cmp.Diff(task, tc.expected); tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
