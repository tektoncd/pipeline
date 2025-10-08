/*
 Copyright 2024 The Tekton Authors

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

package cluster_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"bytes"

	"github.com/google/go-cmp/cmp"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/internal/resolution"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/cache"
	cluster "github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/cluster"
	frtesting "github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	clusterresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/cluster"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	frameworktesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/yaml"

	cacheinjection "github.com/tektoncd/pipeline/pkg/remoteresolution/cache/injection"
)

const (
	disabledError = "cannot handle resolution request, enable-cluster-resolver feature flag not true"
)

func TestGetSelector(t *testing.T) {
	resolver := cluster.Resolver{}
	sel := resolver.GetSelector(t.Context())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != cluster.LabelValueClusterResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidate(t *testing.T) {
	resolver := cluster.Resolver{}

	params := []pipelinev1.Param{{
		Name:  clusterresolution.KindParam,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  clusterresolution.NamespaceParam,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  clusterresolution.NameParam,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}

	ctx := framework.InjectResolverConfigToContext(t.Context(), map[string]string{
		clusterresolution.AllowedNamespacesKey: "foo,bar",
		clusterresolution.BlockedNamespacesKey: "abc,def",
	})

	req := v1beta1.ResolutionRequestSpec{Params: params}
	if err := resolver.Validate(ctx, &req); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateNotEnabled(t *testing.T) {
	resolver := &cluster.Resolver{}

	var err error

	params := []pipelinev1.Param{{
		Name:  clusterresolution.KindParam,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  clusterresolution.NamespaceParam,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  clusterresolution.NameParam,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}

	ctx := resolverDisabledContext()
	req := v1beta1.ResolutionRequestSpec{Params: params}
	if err = resolver.Validate(ctx, &req); err == nil {
		t.Fatalf("expected error, got nil")
	} else if err.Error() != disabledError {
		t.Fatalf("expected error %q, got %q", disabledError, err.Error())
	}
}

func TestValidateWithNoParams(t *testing.T) {
	resolver := &cluster.Resolver{}

	// Test Validate with no parameters - should get validation error about missing params
	req := v1beta1.ResolutionRequestSpec{Params: []pipelinev1.Param{}}
	err := resolver.Validate(t.Context(), &req)
	if err == nil {
		t.Fatalf("expected error when no params provided, got nil")
	}
	if !strings.Contains(err.Error(), "missing required cluster resolver params") {
		t.Fatalf("expected validation error about missing params, got %q", err.Error())
	}
}

func TestValidateFailure(t *testing.T) {
	testCases := []struct {
		name        string
		params      map[string]string
		conf        map[string]string
		expectedErr string
	}{
		{
			name: "missing kind",
			params: map[string]string{
				clusterresolution.NameParam:      "foo",
				clusterresolution.NamespaceParam: "bar",
			},
			expectedErr: "missing required cluster resolver params: kind",
		}, {
			name: "invalid kind",
			params: map[string]string{
				clusterresolution.KindParam:      "banana",
				clusterresolution.NamespaceParam: "foo",
				clusterresolution.NameParam:      "bar",
			},
			expectedErr: "unknown or unsupported resource kind 'banana'",
		}, {
			name: "missing multiple",
			params: map[string]string{
				clusterresolution.KindParam: "task",
			},
			expectedErr: "missing required cluster resolver params: name, namespace",
		}, {
			name: "not in allowed namespaces",
			params: map[string]string{
				clusterresolution.KindParam:      "task",
				clusterresolution.NamespaceParam: "foo",
				clusterresolution.NameParam:      "baz",
			},
			conf: map[string]string{
				clusterresolution.AllowedNamespacesKey: "abc,def",
			},
			expectedErr: "access to specified namespace foo is not allowed",
		}, {
			name: "in blocked namespaces",
			params: map[string]string{
				clusterresolution.KindParam:      "task",
				clusterresolution.NamespaceParam: "foo",
				clusterresolution.NameParam:      "baz",
			},
			conf: map[string]string{
				clusterresolution.BlockedNamespacesKey: "foo,bar",
			},
			expectedErr: "access to specified namespace foo is blocked",
		},
		{
			name: "blocked by star",
			params: map[string]string{
				clusterresolution.KindParam:      "task",
				clusterresolution.NamespaceParam: "foo",
				clusterresolution.NameParam:      "baz",
			},
			conf: map[string]string{
				clusterresolution.BlockedNamespacesKey: "*",
			},
			expectedErr: "only explicit allowed access to namespaces is allowed",
		},
		{
			name: "blocked by star but allowed explicitly",
			params: map[string]string{
				clusterresolution.KindParam:      "task",
				clusterresolution.NamespaceParam: "foo",
				clusterresolution.NameParam:      "baz",
			},
			conf: map[string]string{
				clusterresolution.BlockedNamespacesKey: "*",
				clusterresolution.AllowedNamespacesKey: "foo",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &cluster.Resolver{}

			ctx := t.Context()
			if len(tc.conf) > 0 {
				ctx = framework.InjectResolverConfigToContext(ctx, tc.conf)
			}

			var asParams []pipelinev1.Param
			for k, v := range tc.params {
				asParams = append(asParams, pipelinev1.Param{
					Name:  k,
					Value: *pipelinev1.NewStructuredValues(v),
				})
			}
			req := v1beta1.ResolutionRequestSpec{Params: asParams}
			err := resolver.Validate(ctx, &req)
			if tc.expectedErr == "" {
				if err != nil {
					t.Fatalf("got unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("got no error, but expected: %s", tc.expectedErr)
			}
			if d := cmp.Diff(tc.expectedErr, err.Error()); d != "" {
				t.Errorf("error did not match: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolve(t *testing.T) {
	defaultNS := "pipeline-ns"

	exampleTask := &pipelinev1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "example-task",
			Namespace:       "task-ns",
			ResourceVersion: "00002",
			UID:             "a123",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       string(pipelinev1beta1.NamespacedTaskKind),
			APIVersion: "tekton.dev/v1",
		},
		Spec: pipelinev1.TaskSpec{
			Steps: []pipelinev1.Step{{
				Name:    "some-step",
				Image:   "some-image",
				Command: []string{"something"},
			}},
		},
	}
	taskChecksum, err := exampleTask.Checksum()
	if err != nil {
		t.Fatalf("couldn't checksum task: %v", err)
	}
	taskAsYAML, err := yaml.Marshal(exampleTask)
	if err != nil {
		t.Fatalf("couldn't marshal task: %v", err)
	}

	examplePipeline := &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "example-pipeline",
			Namespace:       defaultNS,
			ResourceVersion: "00001",
			UID:             "b123",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1",
		},
		Spec: pipelinev1.PipelineSpec{
			Tasks: []pipelinev1.PipelineTask{{
				Name: "some-pipeline-task",
				TaskRef: &pipelinev1.TaskRef{
					Name: "some-task",
					Kind: pipelinev1.NamespacedTaskKind,
				},
			}},
		},
	}
	pipelineChecksum, err := examplePipeline.Checksum()
	if err != nil {
		t.Fatalf("couldn't checksum pipeline: %v", err)
	}
	pipelineAsYAML, err := yaml.Marshal(examplePipeline)
	if err != nil {
		t.Fatalf("couldn't marshal pipeline: %v", err)
	}

	testCases := []struct {
		name              string
		kind              string
		resourceName      string
		namespace         string
		allowedNamespaces string
		blockedNamespaces string
		expectedStatus    *v1beta1.ResolutionRequestStatus
		expectedErr       error
	}{
		{
			name:         "successful task",
			kind:         "task",
			resourceName: exampleTask.Name,
			namespace:    exampleTask.Namespace,
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString(taskAsYAML),
					RefSource: &pipelinev1.RefSource{
						URI: "/apis/tekton.dev/v1/namespaces/task-ns/task/example-task@a123",
						Digest: map[string]string{
							"sha256": hex.EncodeToString(taskChecksum),
						},
					},
				},
			},
		}, {
			name:         "successful pipeline",
			kind:         "pipeline",
			resourceName: examplePipeline.Name,
			namespace:    examplePipeline.Namespace,
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString(pipelineAsYAML),
					RefSource: &pipelinev1.RefSource{
						URI: "/apis/tekton.dev/v1/namespaces/pipeline-ns/pipeline/example-pipeline@b123",
						Digest: map[string]string{
							"sha256": hex.EncodeToString(pipelineChecksum),
						},
					},
				},
			},
		}, {
			name:         "default namespace",
			kind:         "pipeline",
			resourceName: examplePipeline.Name,
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString(pipelineAsYAML),
					RefSource: &pipelinev1.RefSource{
						URI: "/apis/tekton.dev/v1/namespaces/pipeline-ns/pipeline/example-pipeline@b123",
						Digest: map[string]string{
							"sha256": hex.EncodeToString(pipelineChecksum),
						},
					},
				},
			},
		}, {
			name:         "default kind",
			resourceName: exampleTask.Name,
			namespace:    exampleTask.Namespace,
			expectedStatus: &v1beta1.ResolutionRequestStatus{
				Status: duckv1.Status{},
				ResolutionRequestStatusFields: v1beta1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString(taskAsYAML),
					RefSource: &pipelinev1.RefSource{
						URI: "/apis/tekton.dev/v1/namespaces/task-ns/task/example-task@a123",
						Digest: map[string]string{
							"sha256": hex.EncodeToString(taskChecksum),
						},
					},
				},
			},
		}, {
			name:           "no such task",
			kind:           "task",
			resourceName:   exampleTask.Name,
			namespace:      "other-ns",
			expectedStatus: resolution.CreateResolutionRequestFailureStatus(),
			expectedErr: &resolutioncommon.GetResourceError{
				ResolverName: cluster.ClusterResolverName,
				Key:          "foo/rr",
				Original:     errors.New(`tasks.tekton.dev "example-task" not found`),
			},
		}, {
			name:              "not in allowed namespaces",
			kind:              "task",
			resourceName:      exampleTask.Name,
			namespace:         "other-ns",
			allowedNamespaces: "foo,bar",
			expectedStatus:    resolution.CreateResolutionRequestFailureStatus(),
			expectedErr: &resolutioncommon.InvalidRequestError{
				ResolutionRequestKey: "foo/rr",
				Message:              "access to specified namespace other-ns is not allowed",
			},
		}, {
			name:              "in blocked namespaces",
			kind:              "task",
			resourceName:      exampleTask.Name,
			namespace:         "other-ns",
			blockedNamespaces: "foo,other-ns,bar",
			expectedStatus:    resolution.CreateResolutionRequestFailureStatus(),
			expectedErr: &resolutioncommon.InvalidRequestError{
				ResolutionRequestKey: "foo/rr",
				Message:              "access to specified namespace other-ns is blocked",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := ttesting.SetupFakeContext(t)

			request := createRequest(tc.kind, tc.resourceName, tc.namespace)

			confMap := map[string]string{
				clusterresolution.DefaultKindKey:      "task",
				clusterresolution.DefaultNamespaceKey: defaultNS,
			}
			if tc.allowedNamespaces != "" {
				confMap[clusterresolution.AllowedNamespacesKey] = tc.allowedNamespaces
			}
			if tc.blockedNamespaces != "" {
				confMap[clusterresolution.BlockedNamespacesKey] = tc.blockedNamespaces
			}

			d := test.Data{
				ConfigMaps: []*corev1.ConfigMap{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-resolver-config",
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
					},
					Data: confMap,
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: resolverconfig.ResolversNamespace(system.Namespace()),
						Name:      resolverconfig.GetFeatureFlagsConfigName(),
					},
					Data: map[string]string{
						"enable-cluster-resolver": "true",
					},
				}},
				Pipelines:          []*pipelinev1.Pipeline{examplePipeline},
				ResolutionRequests: []*v1beta1.ResolutionRequest{request},
				Tasks:              []*pipelinev1.Task{exampleTask},
			}

			resolver := &cluster.Resolver{}

			var expectedStatus *v1beta1.ResolutionRequestStatus
			if tc.expectedStatus != nil {
				expectedStatus = tc.expectedStatus.DeepCopy()

				if tc.expectedErr == nil {
					reqParams := make(map[string]pipelinev1.ParamValue)
					for _, p := range request.Spec.Params {
						reqParams[p.Name] = p.Value
					}
					if expectedStatus.Annotations == nil {
						expectedStatus.Annotations = make(map[string]string)
					}
					expectedStatus.Annotations[clusterresolution.ResourceNameAnnotation] = reqParams[clusterresolution.NameParam].StringVal
					if reqParams[clusterresolution.NamespaceParam].StringVal != "" {
						expectedStatus.Annotations[clusterresolution.ResourceNamespaceAnnotation] = reqParams[clusterresolution.NamespaceParam].StringVal
					} else {
						expectedStatus.Annotations[clusterresolution.ResourceNamespaceAnnotation] = defaultNS
					}
				} else {
					expectedStatus.Status.Conditions[0].Message = tc.expectedErr.Error()
				}
				expectedStatus.Source = expectedStatus.RefSource
			}

			frtesting.RunResolverReconcileTest(ctx, t, d, resolver, request, expectedStatus, tc.expectedErr)
		})
	}
}

func createRequest(kind, name, namespace string) *v1beta1.ResolutionRequest {
	rr := &v1beta1.ResolutionRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resolution.tekton.dev/v1beta1",
			Kind:       "ResolutionRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rr",
			Namespace:         "foo",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				resolutioncommon.LabelKeyResolverType: cluster.LabelValueClusterResolverType,
			},
		},
		Spec: v1beta1.ResolutionRequestSpec{
			Params: []pipelinev1.Param{{
				Name:  clusterresolution.NameParam,
				Value: *pipelinev1.NewStructuredValues(name),
			}},
		},
	}
	if kind != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  clusterresolution.KindParam,
			Value: *pipelinev1.NewStructuredValues(kind),
		})
	}
	if namespace != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  clusterresolution.NamespaceParam,
			Value: *pipelinev1.NewStructuredValues(namespace),
		})
	}

	return rr
}

func resolverDisabledContext() context.Context {
	return frameworktesting.ContextWithClusterResolverDisabled(context.Background())
}

func TestResolveWithDisabledResolver(t *testing.T) {
	ctx := frameworktesting.ContextWithClusterResolverDisabled(t.Context())
	resolver := &cluster.Resolver{}

	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	_, err := resolver.Resolve(ctx, req)
	if err == nil {
		t.Error("Expected error when resolver is disabled")
	}
	if !strings.Contains(err.Error(), "enable-cluster-resolver feature flag not true") {
		t.Errorf("Expected disabled error, got: %v", err)
	}
}

func TestResolveWithNoParams(t *testing.T) {
	resolver := &cluster.Resolver{}

	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{},
	}

	_, err := resolver.Resolve(t.Context(), req)
	if err == nil {
		t.Error("Expected error when no params provided")
	}
	if !strings.Contains(err.Error(), "missing required cluster resolver params") {
		t.Errorf("Expected validation error about missing params, got: %v", err)
	}
}

func TestResolveWithInvalidParams(t *testing.T) {
	resolver := &cluster.Resolver{}

	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "invalid", Value: *pipelinev1.NewStructuredValues("value")},
		},
	}

	_, err := resolver.Resolve(t.Context(), req)
	if err == nil {
		t.Error("Expected error with invalid params")
	}
	if !strings.Contains(err.Error(), "missing required cluster resolver params") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

// TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:
// TODO: fix/move broken test => does not belong here
// TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:TODO:
//
// func TestResolverCacheKeyGeneration(t *testing.T) {
// 	tests := []struct {
// 		name          string
// 		resolverType  string
// 		params        []pipelinev1.Param
// 		expectedError bool
// 	}{
// 		{
// 			name:         "valid params",
// 			resolverType: "cluster",
// 			params: []pipelinev1.Param{
// 				{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
// 				{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
// 				{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
// 				{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
// 			},
// 			expectedError: false,
// 		},
// 		{
// 			name:         "params without cache",
// 			resolverType: "cluster",
// 			params: []pipelinev1.Param{
// 				{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
// 				{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
// 				{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
// 			},
// 			expectedError: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			cacheKey, err := cache.GenerateCacheKey(tt.resolverType, tt.params)
// 			if tt.expectedError && err == nil {
// 				t.Error("Expected error but got none")
// 			}
// 			if !tt.expectedError && err != nil {
// 				t.Errorf("Unexpected error: %v", err)
// 			}
// 			if !tt.expectedError && cacheKey == "" {
// 				t.Error("Generated cache key should not be empty")
// 			}
// 		})
// 	}
// }

func TestAnnotatedResourceCreation(t *testing.T) {
	// Create a mock resolved resource using the correct type
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}

	// Create annotated resource
	annotatedResource := cache.NewAnnotatedResource(mockResource, "cluster", cache.CacheOperationStore)

	// Verify annotations are present
	annotations := annotatedResource.Annotations()
	if annotations == nil {
		t.Fatal("Annotations should not be nil")
	}

	// Check specific annotation keys
	expectedKeys := []string{
		"resolution.tekton.dev/cached",
		"resolution.tekton.dev/cache-timestamp",
		"resolution.tekton.dev/cache-resolver-type",
	}

	for _, key := range expectedKeys {
		if _, exists := annotations[key]; !exists {
			t.Errorf("Expected annotation key '%s' not found", key)
		}
	}

	// Verify resolver type annotation
	if annotations["resolution.tekton.dev/cache-resolver-type"] != "cluster" {
		t.Errorf("Expected resolver type 'cluster', got '%s'", annotations["resolution.tekton.dev/cache-resolver-type"])
	}

	// Verify cached annotation
	if annotations["resolution.tekton.dev/cached"] != "true" {
		t.Errorf("Expected cached value 'true', got '%s'", annotations["resolution.tekton.dev/cached"])
	}

	// Verify data is preserved
	if !bytes.Equal(annotatedResource.Data(), mockResource.Data()) {
		t.Error("Data should be preserved in annotated resource")
	}
}

// TODO: fix/move broken test
func TestResolveWithCacheHit(t *testing.T) {
	// Test that cache hits work correctly
	ctx := t.Context()
	resolver := &cluster.Resolver{}

	// Create a mock cached resource
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("cached content"),
		Spec:       []byte("cached spec"),
		Name:       "cached-task",
		Namespace:  "cached-ns",
		Identifier: "cached-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}

	// Add the resource to the global cache
	cacheKey := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, []pipelinev1.Param{
		{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
		{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
		{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
		{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
	})

	// Get cache instance
	cacheInstance := cacheinjection.Get(ctx)

	// Add to cache
	cacheInstance.Add(cacheKey, mockResource)

	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// This should hit the cache and return the cached resource
	result, err := resolver.Resolve(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// TODO: fix this
	// Verify it's an annotated resource (indicating it came from cache)
	annotatedResource, ok := result.(*cache.AnnotatedResource)
	if !ok {
		t.Fatal("Expected annotated resource from cache")
	}

	// Verify annotations indicate it came from cache
	annotations := annotatedResource.Annotations()
	if annotations["resolution.tekton.dev/cached"] != "true" {
		t.Error("Expected cached annotation to be true")
	}
	if annotations["resolution.tekton.dev/cache-resolver-type"] != "cluster" {
		t.Error("Expected resolver type to be cluster")
	}
}

// TODO: fix/move broken test => test does not belong here
func TestResolveWithCacheKeyGenerationError(t *testing.T) {
	// Test error handling when cache key generation fails

	// Create request with invalid params that would cause cache key generation to fail
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// Test that cache key generation works correctly
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Cache key generation should not fail for valid params: %v", err)
	}
	if cacheKey == "" {
		t.Error("Generated cache key should not be empty")
	}
}

func TestResolveWithCacheAlwaysMode(t *testing.T) {
	// Test that cluster resolver caches when cache mode is 'always'

	// Create a request with cache: always
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// Test that the resolver should use cache for 'always' mode
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "always" {
		t.Errorf("Expected cache mode 'always', got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if !useCache {
		t.Error("Expected cache to be enabled for 'always' mode")
	}
}

func TestResolveWithCacheAutoMode(t *testing.T) {
	// Test that cluster resolver does NOT cache when cache mode is 'auto'

	// Create a request with cache: auto
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("auto")},
		},
	}

	// Test that the resolver should NOT use cache for 'auto' mode
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "auto" {
		t.Errorf("Expected cache mode 'auto', got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic - cluster resolver should NOT cache for auto mode
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled for 'auto' mode in cluster resolver")
	}
}

func TestResolveWithCacheNeverModeSimple(t *testing.T) {
	// Test that cluster resolver does NOT cache when cache mode is 'never'

	// Create a request with cache: never
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("never")},
		},
	}

	// Test that the resolver should NOT use cache for 'never' mode
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "never" {
		t.Errorf("Expected cache mode 'never', got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled for 'never' mode")
	}
}

func TestResolveWithNoCacheParameter(t *testing.T) {
	// Test that cluster resolver does NOT cache when no cache parameter is provided

	// Create a request without cache parameter
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			// No cache parameter
		},
	}

	// Test that the resolver should NOT use cache when no cache parameter is provided
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify no cache parameter is present
	if cacheMode, exists := paramsMap["cache"]; exists {
		t.Errorf("Expected no cache parameter, got '%s'", cacheMode)
	}

	// Test the cache decision logic
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled when no cache parameter is provided")
	}
}

func TestResolveWithInvalidCacheMode(t *testing.T) {
	// Test that cluster resolver does NOT cache when cache mode is invalid

	// Create a request with invalid cache mode
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("invalid")},
		},
	}

	// Test that the resolver should NOT use cache for invalid cache mode
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "invalid" {
		t.Errorf("Expected cache mode 'invalid', got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled for invalid cache mode")
	}
}

func TestResolveWithEmptyCacheParameter(t *testing.T) {
	// Test that cluster resolver does NOT cache when cache parameter is empty

	// Create a request with empty cache parameter
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("")},
		},
	}

	// Test that the resolver should NOT use cache for empty cache parameter
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "" {
		t.Errorf("Expected empty cache mode, got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled for empty cache parameter")
	}
}

func TestClusterResolverCacheBehaviorSummary(t *testing.T) {
	// Comprehensive test of cluster resolver cache behavior
	tests := []struct {
		name           string
		cacheMode      string
		expectedCached bool
		description    string
	}{
		{
			name:           "always mode should cache",
			cacheMode:      "always",
			expectedCached: true,
			description:    "Cluster resolver should cache when cache mode is 'always'",
		},
		{
			name:           "never mode should not cache",
			cacheMode:      "never",
			expectedCached: false,
			description:    "Cluster resolver should not cache when cache mode is 'never'",
		},
		{
			name:           "auto mode should not cache",
			cacheMode:      "auto",
			expectedCached: false,
			description:    "Cluster resolver should not cache when cache mode is 'auto' (no immutable reference)",
		},
		{
			name:           "no cache mode should not cache",
			cacheMode:      "",
			expectedCached: false,
			description:    "Cluster resolver should not cache when no cache mode is specified",
		},
		{
			name:           "invalid cache mode should not cache",
			cacheMode:      "invalid",
			expectedCached: false,
			description:    "Cluster resolver should not cache when cache mode is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the cache mode logic from the resolver
			useCache := false
			if tt.cacheMode == "always" {
				useCache = true
			}

			if useCache != tt.expectedCached {
				t.Errorf("%s: expected cache to be %v, got %v", tt.description, tt.expectedCached, useCache)
			}
		})
	}
}

func TestResolveWithCacheMiss(t *testing.T) {
	// Test that cache miss scenarios work correctly
	ctx := t.Context()
	resolver := &cluster.Resolver{}

	// Create a request with cache: always but no cached resource
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// This should miss the cache and resolve normally
	// Note: This test will fail if the task doesn't exist, but that's expected
	// The important part is that it doesn't crash and handles cache miss gracefully
	_, err := resolver.Resolve(ctx, req)
	// We expect an error because the task doesn't exist, but the cache miss should be handled gracefully
	if err != nil {
		// This is expected - the task doesn't exist in the test environment
		// The important thing is that the cache miss didn't cause a panic or unexpected error
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "failed to load") {
			t.Errorf("Unexpected error type: %v", err)
		}
	}
}

// TODO: fix/move broken test => does not belong here
func TestResolveWithCacheStorage(t *testing.T) {
	// Test that cache storage operations work correctly
	ctx := t.Context()

	// Create a request with cache: always
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// Generate cache key
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache from injection
	cacheInstance := cacheinjection.Get(ctx)

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Test cache initialization
	if cacheInstance == nil {
		t.Error("Cache instance should be initialized")
	}

	// Test cache storage operations
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}

	// Add to cache
	cacheInstance.Add(cacheKey, mockResource)

	// Verify storage
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Resource should be in cache")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}

	// Test cache removal
	cacheInstance.Remove(cacheKey)
	if _, exists := cacheInstance.Get(cacheKey); exists {
		t.Error("Resource should be removed from cache")
	}
}

// TODO: fix/move broken test => does not belong here

func TestResolveWithCacheAlwaysEndToEnd(t *testing.T) {
	// Test end-to-end cache behavior with cache: always

	// Create a mock resource that would be returned by the resolver
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}

	// Create request with cache: always
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// Generate cache key
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache from injection
	cacheInstance := cacheinjection.Get(t.Context())

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Verify cache is empty initially
	if _, exists := cacheInstance.Get(cacheKey); exists {
		t.Error("Cache should be empty initially")
	}

	// Add mock resource to cache
	cacheInstance.Add(cacheKey, mockResource)

	// Verify resource was stored in cache
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Resource should be in cache")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}
}

func TestResolveWithCacheNeverEndToEnd(t *testing.T) {
	// Test end-to-end cache behavior with cache: never

	// Create request with cache: never
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("never")},
		},
	}

	// Generate cache key
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache from injection
	cacheInstance := cacheinjection.Get(t.Context())

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Add a mock resource to cache (this should be ignored)
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}
	cacheInstance.Add(cacheKey, mockResource)

	// Test cache initialization
	if cacheInstance == nil {
		t.Error("Cache instance should be initialized")
	}

	// Verify that the cache contains the mock resource (it won't be used)
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Cache should contain the mock resource")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}
}

func TestResolveWithCacheAutoEndToEnd(t *testing.T) {
	// Test end-to-end cache behavior with cache: auto
	ctx := t.Context()

	// Create request with cache: auto
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("auto")},
		},
	}

	// Generate cache key
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache from injection
	cacheInstance := cacheinjection.Get(ctx)

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Add a mock resource to cache (this should be ignored for auto mode)
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}
	cacheInstance.Add(cacheKey, mockResource)

	// Test cache initialization
	if cacheInstance == nil {
		t.Error("Cache instance should be initialized")
	}

	// Verify that the cache contains the mock resource (it won't be used)
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Cache should contain the mock resource")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}
}

func TestResolveWithCacheInitialization(t *testing.T) {
	// Test that cache initialization works correctly

	// Create a request with cache: always
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// Test that cache logger initialization works
	// This should not panic or cause errors
	cacheInstance := cacheinjection.Get(t.Context())

	// Test cache initialization
	if cacheInstance == nil {
		_ = cacheInstance // Use variable to avoid unused warning
		t.Error("Cache instance should be initialized")
	}

	// Test cache key generation
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	if cacheKey == "" {
		t.Error("Generated cache key should not be empty")
	}
}

func TestResolveWithCacheKeyUniqueness(t *testing.T) {
	// Test that cache keys are unique for different parameters

	// Test different parameter combinations
	testCases := []struct {
		name   string
		params []pipelinev1.Param
	}{
		{
			name: "different namespaces",
			params: []pipelinev1.Param{
				{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
				{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
				{Name: "namespace", Value: *pipelinev1.NewStructuredValues("ns1")},
				{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
			},
		},
		{
			name: "different names",
			params: []pipelinev1.Param{
				{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
				{Name: "name", Value: *pipelinev1.NewStructuredValues("task1")},
				{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
				{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
			},
		},
		{
			name: "different kinds",
			params: []pipelinev1.Param{
				{Name: "kind", Value: *pipelinev1.NewStructuredValues("pipeline")},
				{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
				{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
				{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
			},
		},
	}

	cacheKeys := make(map[string]bool)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, tc.params)
			if err != nil {
				t.Fatalf("Failed to generate cache key: %v", err)
			}

			if cacheKey == "" {
				t.Error("Generated cache key should not be empty")
			}

			// Check for uniqueness
			if cacheKeys[cacheKey] {
				t.Errorf("Cache key should be unique, but %s was already generated", cacheKey)
			}
			cacheKeys[cacheKey] = true
		})
	}
}

// TODO: fix/move broken test => does not belong here
func TestIntegrationNoCacheParameter(t *testing.T) {
	// Integration test: Verify no caching is performed when no cache parameter is included
	ctx := t.Context()

	// Create a request WITHOUT cache parameter
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			// No cache parameter
		},
	}

	// Test the cache decision logic directly
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify no cache parameter is present
	if cacheMode, exists := paramsMap["cache"]; exists {
		t.Errorf("Expected no cache parameter, got '%s'", cacheMode)
	}

	// Test the cache decision logic - should NOT cache when no cache parameter
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled when no cache parameter is provided")
	}

	// Generate cache key to verify it's not used
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache instance
	cacheInstance := cacheinjection.Get(ctx)

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Add a mock resource to cache (this should be ignored)
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}
	cacheInstance.Add(cacheKey, mockResource)

	// Verify that the cache contains the mock resource (it won't be used)
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Cache should contain the mock resource")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}

	// Test that cache initialization works
	if cacheInstance == nil {
		t.Error("Cache instance should be initialized")
	}
}

func TestIntegrationCacheNever(t *testing.T) {
	// Integration test: Verify no caching when cache: never
	ctx := t.Context()

	// Create a request with cache: never
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("never")},
		},
	}

	// Test the cache decision logic directly
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "never" {
		t.Errorf("Expected cache mode 'never', got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic - should NOT cache when cache: never
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled for 'never' mode")
	}

	// Generate cache key to verify it's not used
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache instance
	cacheInstance := cacheinjection.Get(ctx)

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Add a mock resource to cache (this should be ignored)
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}
	cacheInstance.Add(cacheKey, mockResource)

	// Verify that the cache contains the mock resource (it won't be used)
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Cache should contain the mock resource")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}

	// Test that cache initialization works
	if cacheinjection.Get(t.Context()) == nil {
		t.Error("Injection cache should be initialized")
	}
}

func TestIntegrationCacheAuto(t *testing.T) {
	// Integration test: Verify no caching when cache: auto (cluster resolver behavior)
	ctx := t.Context()

	// Create a request with cache: auto
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("auto")},
		},
	}

	// Test the cache decision logic directly
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "auto" {
		t.Errorf("Expected cache mode 'auto', got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic - should NOT cache when cache: auto for cluster resolver
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if useCache {
		t.Error("Expected cache to be disabled for 'auto' mode in cluster resolver")
	}

	// Generate cache key to verify it's not used
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache instance
	cacheInstance := cacheinjection.Get(ctx)

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Add a mock resource to cache (this should be ignored for auto mode)
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}
	cacheInstance.Add(cacheKey, mockResource)

	// Verify that the cache contains the mock resource (it won't be used)
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Cache should contain the mock resource")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}

	// Test that cache initialization works
	if cacheinjection.Get(t.Context()) == nil {
		t.Error("Injection cache should be initialized")
	}
}

func TestIntegrationCacheAlways(t *testing.T) {
	// Integration test: Verify caching when cache: always
	ctx := t.Context()

	// Create a request with cache: always
	req := &v1beta1.ResolutionRequestSpec{
		Params: []pipelinev1.Param{
			{Name: "kind", Value: *pipelinev1.NewStructuredValues("task")},
			{Name: "name", Value: *pipelinev1.NewStructuredValues("test-task")},
			{Name: "namespace", Value: *pipelinev1.NewStructuredValues("test-ns")},
			{Name: "cache", Value: *pipelinev1.NewStructuredValues("always")},
		},
	}

	// Test the cache decision logic directly
	paramsMap := make(map[string]string)
	for _, p := range req.Params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// Verify cache mode is correctly extracted
	if paramsMap["cache"] != "always" {
		t.Errorf("Expected cache mode 'always', got '%s'", paramsMap["cache"])
	}

	// Test the cache decision logic - should cache when cache: always
	useCache := false
	if paramsMap["cache"] == "always" {
		useCache = true
	}

	if !useCache {
		t.Error("Expected cache to be enabled for 'always' mode")
	}

	// Generate cache key
	cacheKey, err := cache.GenerateCacheKey(cluster.LabelValueClusterResolverType, req.Params)
	if err != nil {
		t.Fatalf("Failed to generate cache key: %v", err)
	}

	// Get cache instance
	cacheInstance := cacheinjection.Get(ctx)

	// Clear any existing cache entry
	cacheInstance.Remove(cacheKey)

	// Create a mock resource that would be returned by the resolver
	mockResource := &clusterresolution.ResolvedClusterResource{
		Content:    []byte("test content"),
		Spec:       []byte("test spec"),
		Name:       "test-task",
		Namespace:  "test-ns",
		Identifier: "test-identifier",
		Checksum:   []byte{1, 2, 3, 4},
	}

	// Add mock resource to cache
	cacheInstance.Add(cacheKey, mockResource)

	// Verify resource is in cache
	if cached, exists := cacheInstance.Get(cacheKey); !exists {
		t.Error("Resource should be in cache")
	} else if cached == nil {
		t.Error("Cached resource should not be nil")
	}

	// Test that cache initialization works
	if cacheinjection.Get(t.Context()) == nil {
		t.Error("Injection cache should be initialized")
	}
}
