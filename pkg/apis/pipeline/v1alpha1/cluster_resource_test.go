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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestNewClusterResource(t *testing.T) {
	for _, c := range []struct {
		desc     string
		resource *v1alpha1.PipelineResource
		want     *v1alpha1.ClusterResource
	}{{
		desc: "basic cluster resource",
		resource: tb.PipelineResource("test-cluster-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeCluster,
			tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
			tb.PipelineResourceSpecParam("cadata", "bXktY2x1c3Rlci1jZXJ0Cg"),
			tb.PipelineResourceSpecParam("token", "my-token"),
		)),
		want: &v1alpha1.ClusterResource{
			Name:                  "test-cluster-resource",
			Type:                  v1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			CAData:                []byte("my-cluster-cert"),
			Token:                 "my-token",
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "resource with password instead of token",
		resource: tb.PipelineResource("test-cluster-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeCluster,
			tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
			tb.PipelineResourceSpecParam("cadata", "bXktY2x1c3Rlci1jZXJ0Cg"),
			tb.PipelineResourceSpecParam("username", "user"),
			tb.PipelineResourceSpecParam("password", "pass"),
		)),
		want: &v1alpha1.ClusterResource{
			Name:                  "test-cluster-resource",
			Type:                  v1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			CAData:                []byte("my-cluster-cert"),
			Username:              "user",
			Password:              "pass",
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "set insecure flag to true when there is no cert",
		resource: tb.PipelineResource("test-cluster-resource", "foo", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeCluster,
			tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
			tb.PipelineResourceSpecParam("token", "my-token"),
		)),
		want: &v1alpha1.ClusterResource{
			Name:                  "test-cluster-resource",
			Type:                  v1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			Token:                 "my-token",
			Insecure:              true,
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "basic cluster resource with namespace",
		resource: tb.PipelineResource("test-cluster-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeCluster,
			tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
			tb.PipelineResourceSpecParam("cadata", "bXktY2x1c3Rlci1jZXJ0Cg"),
			tb.PipelineResourceSpecParam("token", "my-token"),
			tb.PipelineResourceSpecParam("namespace", "my-namespace"),
		)),
		want: &v1alpha1.ClusterResource{
			Name:                  "test-cluster-resource",
			Type:                  v1alpha1.PipelineResourceTypeCluster,
			URL:                   "http://10.10.10.10",
			CAData:                []byte("my-cluster-cert"),
			Token:                 "my-token",
			Namespace:             "my-namespace",
			KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
		},
	}, {
		desc: "basic resource with secrets",
		resource: tb.PipelineResource("test-cluster-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeCluster,
			tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
			tb.PipelineResourceSpecSecretParam("cadata", "secret1", "cadatakey"),
			tb.PipelineResourceSpecSecretParam("token", "secret1", "tokenkey"),
		)),
		want: &v1alpha1.ClusterResource{
			Name: "test-cluster-resource",
			Type: v1alpha1.PipelineResourceTypeCluster,
			URL:  "http://10.10.10.10",
			Secrets: []v1alpha1.SecretParam{{
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
			got, err := v1alpha1.NewClusterResource("override-with-kubeconfig-writer:latest", c.resource)
			if err != nil {
				t.Errorf("Test: %q; TestNewClusterResource() error = %v", c.desc, err)
			}
			if d := cmp.Diff(got, c.want); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}

func TestClusterResource_GetInputTaskModifier(t *testing.T) {
	names.TestingSeed()
	clusterResource := &v1alpha1.ClusterResource{
		Name: "test-cluster-resource",
		Type: v1alpha1.PipelineResourceTypeCluster,
		URL:  "http://10.10.10.10",
		Secrets: []v1alpha1.SecretParam{{
			FieldName:  "cadata",
			SecretKey:  "cadatakey",
			SecretName: "secret1",
		}},
		KubeconfigWriterImage: "override-with-kubeconfig-writer:latest",
	}

	ts := v1alpha1.TaskSpec{}
	wantSteps := []v1alpha1.Step{{Container: corev1.Container{
		Name:    "kubeconfig-9l9zj",
		Image:   "override-with-kubeconfig-writer:latest",
		Command: []string{"/ko-app/kubeconfigwriter"},
		Args:    []string{"-clusterConfig", `{"name":"test-cluster-resource","type":"cluster","url":"http://10.10.10.10","revision":"","username":"","password":"","namespace":"","token":"","Insecure":false,"cadata":null,"secrets":[{"fieldName":"cadata","secretKey":"cadatakey","secretName":"secret1"}]}`},
		Env: []corev1.EnvVar{{
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
	}}}

	got, err := clusterResource.GetInputTaskModifier(&ts, "")
	if err != nil {
		t.Fatalf("GetDownloadSteps: %v", err)
	}
	if d := cmp.Diff(got.GetStepsToPrepend(), wantSteps); d != "" {
		t.Errorf("Error mismatch between download steps: %s", d)
	}
}
