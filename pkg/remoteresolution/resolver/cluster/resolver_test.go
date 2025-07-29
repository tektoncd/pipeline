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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/internal/resolution"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
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
	resolver := cluster.Resolver{}

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
	req := v1beta1.ResolutionRequestSpec{Params: params}
	err = resolver.Validate(resolverDisabledContext(), &req)
	if err == nil {
		t.Fatalf("expected disabled err")
	}
	if d := cmp.Diff(disabledError, err.Error()); d != "" {
		t.Errorf("unexpected error: %s", diff.PrintWantGot(d))
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
