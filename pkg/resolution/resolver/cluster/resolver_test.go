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

package cluster_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	cluster "github.com/tektoncd/pipeline/pkg/resolution/resolver/cluster"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	frtesting "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework/testing"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/internal"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/yaml"
)

const (
	disabledError = "cannot handle resolution request, enable-cluster-resolver feature flag not true"
)

func TestGetSelector(t *testing.T) {
	resolver := cluster.Resolver{}
	sel := resolver.GetSelector(context.Background())
	if typ, has := sel[resolutioncommon.LabelKeyResolverType]; !has {
		t.Fatalf("unexpected selector: %v", sel)
	} else if typ != cluster.LabelValueClusterResolverType {
		t.Fatalf("unexpected type: %q", typ)
	}
}

func TestValidateParams(t *testing.T) {
	resolver := cluster.Resolver{}

	params := []pipelinev1.Param{{
		Name:  cluster.KindParam,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  cluster.NamespaceParam,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  cluster.NameParam,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}

	ctx := framework.InjectResolverConfigToContext(context.Background(), map[string]string{
		cluster.AllowedNamespacesKey: "foo,bar",
		cluster.BlockedNamespacesKey: "abc,def",
	})

	if err := resolver.ValidateParams(ctx, params); err != nil {
		t.Fatalf("unexpected error validating params: %v", err)
	}
}

func TestValidateParamsNotEnabled(t *testing.T) {
	resolver := cluster.Resolver{}

	var err error

	params := []pipelinev1.Param{{
		Name:  cluster.KindParam,
		Value: *pipelinev1.NewStructuredValues("task"),
	}, {
		Name:  cluster.NamespaceParam,
		Value: *pipelinev1.NewStructuredValues("foo"),
	}, {
		Name:  cluster.NameParam,
		Value: *pipelinev1.NewStructuredValues("baz"),
	}}
	err = resolver.ValidateParams(resolverDisabledContext(), params)
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
	}
}

func TestValidateParamsFailure(t *testing.T) {
	testCases := []struct {
		name        string
		params      map[string]string
		conf        map[string]string
		expectedErr string
	}{
		{
			name: "missing kind",
			params: map[string]string{
				cluster.NameParam:      "foo",
				cluster.NamespaceParam: "bar",
			},
			expectedErr: "missing required cluster resolver params: kind",
		}, {
			name: "invalid kind",
			params: map[string]string{
				cluster.KindParam:      "banana",
				cluster.NamespaceParam: "foo",
				cluster.NameParam:      "bar",
			},
			expectedErr: "unknown or unsupported resource kind 'banana'",
		}, {
			name: "missing multiple",
			params: map[string]string{
				cluster.KindParam: "task",
			},
			expectedErr: "missing required cluster resolver params: name, namespace",
		}, {
			name: "not in allowed namespaces",
			params: map[string]string{
				cluster.KindParam:      "task",
				cluster.NamespaceParam: "foo",
				cluster.NameParam:      "baz",
			},
			conf: map[string]string{
				cluster.AllowedNamespacesKey: "abc,def",
			},
			expectedErr: "access to specified namespace foo is not allowed",
		}, {
			name: "in blocked namespaces",
			params: map[string]string{
				cluster.KindParam:      "task",
				cluster.NamespaceParam: "foo",
				cluster.NameParam:      "baz",
			},
			conf: map[string]string{
				cluster.BlockedNamespacesKey: "foo,bar",
			},
			expectedErr: "access to specified namespace foo is blocked",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &cluster.Resolver{}

			ctx := context.Background()
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
			err := resolver.ValidateParams(ctx, asParams)
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
	taskAsYAML, err := yaml.Marshal(exampleTask)
	if err != nil {
		t.Fatalf("couldn't marshal task: %v", err)
	}
	taskSpec, err := yaml.Marshal(exampleTask.Spec)
	if err != nil {
		t.Fatalf("couldn't marshal task spec: %v", err)
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
	pipelineAsYAML, err := yaml.Marshal(examplePipeline)
	if err != nil {
		t.Fatalf("couldn't marshal pipeline: %v", err)
	}
	pipelineSpec, err := yaml.Marshal(examplePipeline.Spec)
	if err != nil {
		t.Fatalf("couldn't marshal pipeline spec: %v", err)
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
							"sha256": sha256CheckSum(taskSpec),
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
							"sha256": sha256CheckSum(pipelineSpec),
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
							"sha256": sha256CheckSum(pipelineSpec),
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
							"sha256": sha256CheckSum(taskSpec),
						},
					},
				},
			},
		}, {
			name:           "no such task",
			kind:           "task",
			resourceName:   exampleTask.Name,
			namespace:      "other-ns",
			expectedStatus: internal.CreateResolutionRequestFailureStatus(),
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
			expectedStatus:    internal.CreateResolutionRequestFailureStatus(),
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
			expectedStatus:    internal.CreateResolutionRequestFailureStatus(),
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
				cluster.DefaultKindKey:      "task",
				cluster.DefaultNamespaceKey: defaultNS,
			}
			if tc.allowedNamespaces != "" {
				confMap[cluster.AllowedNamespacesKey] = tc.allowedNamespaces
			}
			if tc.blockedNamespaces != "" {
				confMap[cluster.BlockedNamespacesKey] = tc.blockedNamespaces
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
					expectedStatus.Annotations[cluster.ResourceNameAnnotation] = reqParams[cluster.NameParam].StringVal
					if reqParams[cluster.NamespaceParam].StringVal != "" {
						expectedStatus.Annotations[cluster.ResourceNamespaceAnnotation] = reqParams[cluster.NamespaceParam].StringVal
					} else {
						expectedStatus.Annotations[cluster.ResourceNamespaceAnnotation] = defaultNS
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
				Name:  cluster.NameParam,
				Value: *pipelinev1.NewStructuredValues(name),
			}},
		},
	}
	if kind != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  cluster.KindParam,
			Value: *pipelinev1.NewStructuredValues(kind),
		})
	}
	if namespace != "" {
		rr.Spec.Params = append(rr.Spec.Params, pipelinev1.Param{
			Name:  cluster.NamespaceParam,
			Value: *pipelinev1.NewStructuredValues(namespace),
		})
	}

	return rr
}

func resolverDisabledContext() context.Context {
	return frtesting.ContextWithClusterResolverDisabled(context.Background())
}

func sha256CheckSum(input []byte) string {
	h := sha256.New()
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))
}
