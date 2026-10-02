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

package v1beta1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

// These tests ensure the immutable controller-managed label check is also
// enforced through the still-served v1beta1 API, so updates cannot bypass it.

// withCaller attaches admission UserInfo to ctx for the given caller identity:
// a ServiceAccount in the Tekton system namespace (the controller) when
// asController is true, otherwise an ordinary user outside that namespace.
func withCaller(ctx context.Context, asController bool) context.Context {
	ui := &authenticationv1.UserInfo{Username: "system:serviceaccount:default:some-user"}
	if asController {
		ui = &authenticationv1.UserInfo{
			Username: "system:serviceaccount:" + system.Namespace() + ":tekton-pipelines-controller",
			Groups:   []string{"system:serviceaccounts:" + system.Namespace()},
		}
	}
	return apis.WithUserInfo(ctx, ui)
}

func TestPipelineRun_ValidateImmutableLabels(t *testing.T) {
	tests := []struct {
		name          string
		baselineList  map[string]string
		updatedLabels map[string]string
		asController  bool
		noUserInfo    bool
		expectedError apis.FieldError
	}{{
		name:          "controller-managed label first stamped from empty is allowed",
		baselineList:  map[string]string{},
		updatedLabels: map[string]string{pipeline.PipelineLabelKey: "my-pipeline"},
		expectedError: apis.FieldError{},
	}, {
		name:          "controller may change a controller-managed label",
		baselineList:  map[string]string{pipeline.PipelineLabelKey: "parent-pipeline"},
		updatedLabels: map[string]string{pipeline.PipelineLabelKey: "child-own-name"},
		asController:  true,
		expectedError: apis.FieldError{},
	}, {
		name:          "user mutating a controller-managed label is rejected",
		baselineList:  map[string]string{pipeline.PipelineLabelKey: "my-pipeline"},
		updatedLabels: map[string]string{pipeline.PipelineLabelKey: "user-edited"},
		expectedError: apis.FieldError{
			Message: `invalid value: label "tekton.dev/pipeline" is immutable once set`,
			Paths:   []string{"metadata.labels.[tekton.dev/pipeline]"},
		},
	}, {
		name:          "empty controller-managed label present is immutable",
		baselineList:  map[string]string{pipeline.PipelineLabelKey: ""},
		updatedLabels: map[string]string{pipeline.PipelineLabelKey: "user-edited"},
		expectedError: apis.FieldError{
			Message: `invalid value: label "tekton.dev/pipeline" is immutable once set`,
			Paths:   []string{"metadata.labels.[tekton.dev/pipeline]"},
		},
	}, {
		name:          "no user info falls back to enforcing immutability",
		baselineList:  map[string]string{pipeline.PipelineLabelKey: "my-pipeline"},
		updatedLabels: map[string]string{pipeline.PipelineLabelKey: "user-edited"},
		noUserInfo:    true,
		expectedError: apis.FieldError{
			Message: `invalid value: label "tekton.dev/pipeline" is immutable once set`,
			Paths:   []string{"metadata.labels.[tekton.dev/pipeline]"},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pr", Labels: tt.baselineList},
				Spec:       v1beta1.PipelineRunSpec{PipelineRef: &v1beta1.PipelineRef{Name: "foo"}},
			}
			updated := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pr", Labels: tt.updatedLabels},
				Spec:       v1beta1.PipelineRunSpec{PipelineRef: &v1beta1.PipelineRef{Name: "foo"}},
			}
			ctx := apis.WithinUpdate(t.Context(), baseline)
			if !tt.noUserInfo {
				ctx = withCaller(ctx, tt.asController)
			}
			err := updated.Validate(ctx)
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("PipelineRun.Validate() label immutability errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskRun_ValidateImmutableLabels(t *testing.T) {
	tests := []struct {
		name          string
		baselineList  map[string]string
		updatedLabels map[string]string
		asController  bool
		noUserInfo    bool
		expectedError apis.FieldError
	}{{
		name:          "controller-managed label first stamped from empty is allowed",
		baselineList:  map[string]string{},
		updatedLabels: map[string]string{pipeline.PipelineRunLabelKey: "my-pipelinerun"},
		expectedError: apis.FieldError{},
	}, {
		name:          "controller may change a controller-managed label",
		baselineList:  map[string]string{pipeline.PipelineLabelKey: "parent-pipeline"},
		updatedLabels: map[string]string{pipeline.PipelineLabelKey: "child-own-name"},
		asController:  true,
		expectedError: apis.FieldError{},
	}, {
		name:          "user mutating a controller-managed label is rejected",
		baselineList:  map[string]string{pipeline.PipelineRunLabelKey: "my-pipelinerun"},
		updatedLabels: map[string]string{pipeline.PipelineRunLabelKey: "user-edited"},
		expectedError: apis.FieldError{
			Message: `invalid value: label "tekton.dev/pipelineRun" is immutable once set`,
			Paths:   []string{"metadata.labels.[tekton.dev/pipelineRun]"},
		},
	}, {
		name:          "empty controller-managed label present is immutable",
		baselineList:  map[string]string{pipeline.PipelineRunLabelKey: ""},
		updatedLabels: map[string]string{pipeline.PipelineRunLabelKey: "user-edited"},
		expectedError: apis.FieldError{
			Message: `invalid value: label "tekton.dev/pipelineRun" is immutable once set`,
			Paths:   []string{"metadata.labels.[tekton.dev/pipelineRun]"},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "tr", Labels: tt.baselineList},
				Spec:       v1beta1.TaskRunSpec{TaskRef: &v1beta1.TaskRef{Name: "foo"}},
			}
			updated := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{Name: "tr", Labels: tt.updatedLabels},
				Spec:       v1beta1.TaskRunSpec{TaskRef: &v1beta1.TaskRef{Name: "foo"}},
			}
			ctx := apis.WithinUpdate(t.Context(), baseline)
			if !tt.noUserInfo {
				ctx = withCaller(ctx, tt.asController)
			}
			err := updated.Validate(ctx)
			if d := cmp.Diff(tt.expectedError.Error(), err.Error(), cmpopts.IgnoreUnexported(apis.FieldError{})); d != "" {
				t.Errorf("TaskRun.Validate() label immutability errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
