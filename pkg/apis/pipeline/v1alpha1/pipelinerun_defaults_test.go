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

package v1alpha1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineRunSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc string
		prs  *v1alpha1.PipelineRunSpec
		want *v1alpha1.PipelineRunSpec
	}{
		{
			desc: "timeout is nil",
			prs:  &v1alpha1.PipelineRunSpec{},
			want: &v1alpha1.PipelineRunSpec{
				Timeout: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
		{
			desc: "timeout is not nil",
			prs: &v1alpha1.PipelineRunSpec{
				Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
			want: &v1alpha1.PipelineRunSpec{
				Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			tc.prs.SetDefaults(ctx)

			if diff := cmp.Diff(tc.want, tc.prs); diff != "" {
				t.Errorf("Mismatch of PipelineRunSpec: %s", diff)
			}
		})
	}
}
