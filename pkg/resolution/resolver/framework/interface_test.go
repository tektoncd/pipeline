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

package framework_test

import (
	"fmt"
	"strings"
	"testing"

	pipelineapi "github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

func TestValidateResolvedResourceAllowsTektonKinds(t *testing.T) {
	testCases := []struct {
		name       string
		apiVersion string
		kind       string
	}{
		{
			name:       "v1 PipelineRun",
			apiVersion: pipelinev1.SchemeGroupVersion.String(),
			kind:       pipelineapi.PipelineRunControllerName,
		}, {
			name:       "v1 Pipeline",
			apiVersion: pipelinev1.SchemeGroupVersion.String(),
			kind:       pipelineapi.PipelineControllerName,
		}, {
			name:       "v1 TaskRun",
			apiVersion: pipelinev1.SchemeGroupVersion.String(),
			kind:       pipelineapi.TaskRunControllerName,
		}, {
			name:       "v1 Task",
			apiVersion: pipelinev1.SchemeGroupVersion.String(),
			kind:       pipelineapi.TaskControllerName,
		}, {
			name:       "v1beta1 PipelineRun",
			apiVersion: pipelinev1beta1.SchemeGroupVersion.String(),
			kind:       pipelineapi.PipelineRunControllerName,
		}, {
			name:       "v1beta1 Pipeline",
			apiVersion: pipelinev1beta1.SchemeGroupVersion.String(),
			kind:       pipelineapi.PipelineControllerName,
		}, {
			name:       "v1beta1 TaskRun",
			apiVersion: pipelinev1beta1.SchemeGroupVersion.String(),
			kind:       pipelineapi.TaskRunControllerName,
		}, {
			name:       "v1beta1 Task",
			apiVersion: pipelinev1beta1.SchemeGroupVersion.String(),
			kind:       pipelineapi.TaskControllerName,
		}, {
			name:       "v1alpha1 Run",
			apiVersion: pipelinev1alpha1.SchemeGroupVersion.String(),
			kind:       pipelineapi.RunControllerName,
		}, {
			name:       "v1beta1 CustomRun",
			apiVersion: pipelinev1beta1.SchemeGroupVersion.String(),
			kind:       pipelineapi.CustomRunControllerName,
		}, {
			name:       "v1beta1 StepAction",
			apiVersion: pipelinev1beta1.SchemeGroupVersion.String(),
			kind:       pipelinev1beta1.StepActionKind,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &framework.FakeResolvedResource{
				Content: resolvedResourceContent(tc.apiVersion, tc.kind),
			}

			if err := framework.ValidateResolvedResource(resource); err != nil {
				t.Fatalf("ValidateResolvedResource() error = %v", err)
			}
		})
	}
}

func TestValidateResolvedResourceRejectsUnsupportedData(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		wantErrText string
	}{
		{
			name:        "unsupported Tekton kind",
			content:     resolvedResourceContent(pipelinev1.SchemeGroupVersion.String(), "UnsupportedKind"),
			wantErrText: "resolved data is not of a supported type",
		}, {
			name:        "unsupported group and kind",
			content:     resolvedResourceContent("apps/v1", "Deployment"),
			wantErrText: "resolved data is not of a supported type",
		}, {
			name:        "non Kubernetes yaml object",
			content:     "token: top-secret\nurl: https://example.com/resource.yaml\n",
			wantErrText: "resolved data is not of a supported type",
		}, {
			name:        "non Kubernetes json object",
			content:     `{"token":"top-secret","url":"https://example.com/resource.yaml"}`,
			wantErrText: "resolved data is not of a supported type",
		}, {
			name:        "string data",
			content:     "not a kubernetes object",
			wantErrText: "decoding error",
		}, {
			name:        "binary data",
			content:     string([]byte{0xff, 0x00, 0x01}),
			wantErrText: "decoding error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &framework.FakeResolvedResource{Content: tc.content}

			err := framework.ValidateResolvedResource(resource)
			if err == nil {
				t.Fatal("ValidateResolvedResource() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.wantErrText) {
				t.Fatalf("ValidateResolvedResource() error = %v, want error containing %q", err, tc.wantErrText)
			}
		})
	}
}

func resolvedResourceContent(apiVersion, kind string) string {
	return fmt.Sprintf("apiVersion: %s\nkind: %s\n", apiVersion, kind)
}
