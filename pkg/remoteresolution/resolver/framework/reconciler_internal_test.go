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
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	rrfake "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned/fake"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolvermetrics"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
	clock "k8s.io/utils/clock/testing"
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
		{name: "transient", err: errors.New("etcdserver: leader changed"), want: resolvermetrics.StatusError},
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

func TestResolveRecordsWriteFailureAsError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	oldProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	defer func() {
		otel.SetMeterProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	}()

	recorder, err := resolvermetrics.NewRecorder()
	if err != nil {
		t.Fatal(err)
	}
	rr := &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "request",
			Namespace: "test",
			Labels: map[string]string{
				resolutioncommon.LabelKeyResolverType: resolutionframework.LabelValueFakeResolverType,
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{Params: []pipelinev1.Param{{
			Name:  resolutionframework.FakeParamName,
			Value: *pipelinev1.NewStructuredValues("resource"),
		}}},
	}
	client := rrfake.NewSimpleClientset(rr)
	client.PrependReactor("patch", "resolutionrequests", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("patch failed")
	})
	r := &Reconciler{
		Clock: clock.NewFakePassiveClock(time.Unix(0, 0)),
		resolver: &FakeResolver{ForParam: map[string]*resolutionframework.FakeResolvedResource{
			"resource": {Content: `apiVersion: tekton.dev/v1
kind: Pipeline`},
		}},
		resolutionRequestClientSet: client,
		metrics:                    recorder,
	}

	if err := r.resolve(context.Background(), "test/request", rr); err == nil {
		t.Fatal("resolve() succeeded, want patch error")
	}
	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatal(err)
	}
	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "tekton_pipelines_resolver_resolution_total" {
				continue
			}
			sum := metric.Data.(metricdata.Sum[int64])
			if len(sum.DataPoints) != 1 {
				t.Fatalf("resolution metric has %d data points, want 1", len(sum.DataPoints))
			}
			status, _ := sum.DataPoints[0].Attributes.Value("status")
			if got := status.AsString(); got != resolvermetrics.StatusError {
				t.Fatalf("resolution status = %q, want %q", got, resolvermetrics.StatusError)
			}
			return
		}
	}
	t.Fatal("resolution metric not found")
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
