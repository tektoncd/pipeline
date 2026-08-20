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

package framework

import (
	"context"
	"errors"
	"fmt"
	"testing"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolvermetrics"
)

type testResolvedResource struct {
	data []byte
}

func (r testResolvedResource) Data() []byte                     { return r.data }
func (r testResolvedResource) Annotations() map[string]string   { return nil }
func (r testResolvedResource) RefSource() *pipelinev1.RefSource { return nil }

func TestResolutionMetricStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: resolvermetrics.StatusTimeout},
		{name: "wrapped deadline", err: fmt.Errorf("resolver: %w", context.DeadlineExceeded), want: resolvermetrics.StatusTimeout},
		{name: "canceled", err: context.Canceled, want: ""},
		{name: "invalid request", err: &resolutioncommon.InvalidRequestError{ResolutionRequestKey: "ns/name", Message: "bad param"}, want: resolvermetrics.StatusInvalidRequest},
		{name: "transient", err: errors.New("etcdserver: leader changed"), want: ""},
		{name: "generic", err: errors.New("boom"), want: resolvermetrics.StatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolutionMetricStatus(tt.err); got != tt.want {
				t.Fatalf("resolutionMetricStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvedResourceKind(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "task", data: `apiVersion: tekton.dev/v1
kind: Task`, want: "Task"},
		{name: "pipeline", data: `apiVersion: tekton.dev/v1
kind: Pipeline`, want: "Pipeline"},
		{name: "step action", data: `apiVersion: tekton.dev/v1beta1
kind: StepAction`, want: "StepAction"},
		{name: "unsupported kind", data: `apiVersion: v1
kind: Pod`, want: resolvermetrics.ResourceKindUnknown},
		{name: "missing kind", data: `apiVersion: tekton.dev/v1`, want: resolvermetrics.ResourceKindUnknown},
		{name: "bad yaml", data: `kind: [`, want: resolvermetrics.ResourceKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := testResolvedResource{data: []byte(tt.data)}
			if got := resolvedResourceKind(resource); got != tt.want {
				t.Fatalf("resolvedResourceKind() = %q, want %q", got, tt.want)
			}
		})
	}
}
