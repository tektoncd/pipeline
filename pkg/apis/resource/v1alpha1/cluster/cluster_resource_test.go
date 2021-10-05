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

package cluster_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/cluster"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestNewClusterResource(t *testing.T) {
	for _, c := range []struct {
		desc     string
		resource *resourcev1alpha1.PipelineResource
		want     *cluster.Resource
	}{{
		desc: "basic cluster resource",
		resource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeCluster,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "url",
						Value: "http://10.10.10.10",
					},
					{
						Name:  "cadata",
						Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					},
					{
						Name:  "token",
						Value: "my-token",
					},
				},
			},
		},
		want: &cluster.Resource{
			Name:                  "test-resource",
			Type:                  resourcev1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			CAData:                []byte("my-cluster-cert"),
			Token:                 "my-token",
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "resource with password instead of token",
		resource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeCluster,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "url",
						Value: "http://10.10.10.10",
					},
					{
						Name:  "cadata",
						Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					},
					{
						Name:  "username",
						Value: "user",
					},
					{
						Name:  "password",
						Value: "pass",
					},
				},
			},
		},
		want: &cluster.Resource{
			Name:                  "test-resource",
			Type:                  resourcev1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			CAData:                []byte("my-cluster-cert"),
			Username:              "user",
			Password:              "pass",
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "resource with clientKeyData and clientCertificateData instead of token or password",
		resource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeCluster,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "url",
						Value: "http://10.10.10.10",
					},
					{
						Name:  "cadata",
						Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					},
					{
						Name:  "username",
						Value: "user",
					},
					{
						Name:  "clientKeyData",
						Value: "Y2xpZW50LWtleS1kYXRh",
					},
					{
						Name:  "clientCertificateData",
						Value: "Y2xpZW50LWNlcnRpZmljYXRlLWRhdGE=",
					},
				},
			},
		},
		want: &cluster.Resource{
			Name:                  "test-resource",
			Type:                  resourcev1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			Username:              "user",
			CAData:                []byte("my-cluster-cert"),
			ClientKeyData:         []byte("client-key-data"),
			ClientCertificateData: []byte("client-certificate-data"),
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "set insecure flag to true when there is no cert",
		resource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeCluster,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "url",
						Value: "http://10.10.10.10",
					},
					{
						Name:  "token",
						Value: "my-token",
					},
				},
			},
		},
		want: &cluster.Resource{
			Name:                  "test-resource",
			Type:                  resourcev1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			Token:                 "my-token",
			Insecure:              true,
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "basic cluster resource with namespace",
		resource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeCluster,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "url",
						Value: "http://10.10.10.10",
					},
					{
						Name:  "cadata",
						Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					},
					{
						Name:  "token",
						Value: "my-token",
					},
					{
						Name:  "namespace",
						Value: "my-namespace",
					},
				},
			},
		},
		want: &cluster.Resource{
			Name:                  "test-resource",
			Type:                  resourcev1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			CAData:                []byte("my-cluster-cert"),
			Token:                 "my-token",
			Namespace:             "my-namespace",
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "basic resource with secrets",
		resource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeCluster,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "url",
						Value: "http://10.10.10.10",
					},
				},
				SecretParams: []resourcev1alpha1.SecretParam{
					{
						FieldName:  "cadata",
						SecretName: "secret1",
						SecretKey:  "cadatakey",
					},
					{
						FieldName:  "token",
						SecretName: "secret1",
						SecretKey:  "tokenkey",
					},
				},
			},
		},
		want: &cluster.Resource{
			Name: "test-resource",
			Type: resourcev1alpha1.PipelineResourceTypeCluster,
			URL:  "http://10.10.10.10",
			Secrets: []resourcev1alpha1.SecretParam{{
				FieldName:  "cadata",
				SecretKey:  "cadatakey",
				SecretName: "secret1",
			}, {
				FieldName:  "token",
				SecretKey:  "tokenkey",
				SecretName: "secret1",
			}},
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got, err := cluster.NewResource("test-resource", "override-with-kubeconfig-writer:latest", "override-with-shell-image:latest", c.resource)
			if err != nil {
				t.Errorf("Test: %q; TestNewClusterResource() error = %v", c.desc, err)
			}
			c.want.ShellImage = "override-with-shell-image:latest"
			if d := cmp.Diff(got, c.want); d != "" {
				t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestClusterResource_GetInputTaskModifier(t *testing.T) {
	names.TestingSeed()
	clusterResource := &cluster.Resource{
		Name: "test-cluster-resource",
		Type: resourcev1alpha1.PipelineResourceTypeCluster,
		URL:  "http://10.10.10.10",
		Secrets: []resourcev1alpha1.SecretParam{{
			FieldName:  "cadata",
			SecretKey:  "cadatakey",
			SecretName: "secret1",
		}},
		KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
	}

	ts := v1beta1.TaskSpec{}
	wantSteps := []v1beta1.Step{
		{
			Container: corev1.Container{
				Name:    "kubeconfig-9l9zj",
				Image:   "override-with-kubeconfig-writer:latest",
				Command: []string{"/ko-app/kubeconfigwriter"},
				Args:    []string{"-clusterConfig", `{"name":"test-cluster-resource","type":"cluster","url":"http://10.10.10.10","revision":"","username":"","password":"","namespace":"","token":"","Insecure":false,"cadata":null,"clientKeyData":null,"clientCertificateData":null,"secrets":[{"fieldName":"cadata","secretKey":"cadatakey","secretName":"secret1"}]}`},
				Env: []corev1.EnvVar{
					{
						Name:  "TEKTON_RESOURCE_NAME",
						Value: "test-cluster-resource",
					},
					{
						Name: "CADATA",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "secret1",
								},
								Key: "cadatakey",
							},
						},
					}},
			},
		},
	}

	got, err := clusterResource.GetInputTaskModifier(&ts, "/location")
	if err != nil {
		t.Fatalf("GetDownloadSteps: %v", err)
	}
	if d := cmp.Diff(got.GetStepsToPrepend(), wantSteps); d != "" {
		t.Errorf("Error mismatch between download steps: %s", diff.PrintWantGot(d))
	}
}
