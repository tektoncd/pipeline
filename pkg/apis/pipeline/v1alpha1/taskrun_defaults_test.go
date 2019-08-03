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

func TestTaskRunSpec_SetDefaults(t *testing.T) {
	cases := []struct {
		desc string
		trs  *v1alpha1.TaskRunSpec
		want *v1alpha1.TaskRunSpec
	}{
		{
			desc: "taskref is nil",
			trs: &v1alpha1.TaskRunSpec{
				TaskRef: nil,
				Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
			want: &v1alpha1.TaskRunSpec{
				TaskRef: nil,
				Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
		},
		{
			desc: "taskref kind is empty",
			trs: &v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{},
				Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
			want: &v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Kind: v1alpha1.NamespacedTaskKind},
				Timeout: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
		},
		{
			desc: "timeout is nil",
			trs: &v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Kind: v1alpha1.ClusterTaskKind},
			},
			want: &v1alpha1.TaskRunSpec{
				TaskRef: &v1alpha1.TaskRef{Kind: v1alpha1.ClusterTaskKind},
				Timeout: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			tc.trs.SetDefaults(ctx)

			if diff := cmp.Diff(tc.want, tc.trs); diff != "" {
				t.Errorf("Mismatch of TaskRunSpec: %s", diff)
			}
		})
	}
}
