/*
Copyright 2018 The Knative Authors.

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

package artifacts

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/system"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

func TestInitializeArtifactStorage_WithConfigMap(t *testing.T) {
	fakekubeclient := fakek8s.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace,
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "gs://fake-bucket",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
	)
	pipelinerun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}

	bucket, err := InitializeArtifactStorage(pipelinerun, fakekubeclient)
	if err != nil {
		t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
	}

	expectedArtifactBucket := &v1alpha1.ArtifactBucket{
		Location: "gs://fake-bucket",
		Secrets: []v1alpha1.SecretParam{{
			FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			SecretKey:  "sakey",
			SecretName: "secret1",
		}},
	}

	if diff := cmp.Diff(bucket, expectedArtifactBucket); diff != "" {
		t.Fatalf("want %v, but got %v", expectedArtifactBucket, bucket)
	}
}

func TestInitializeArtifactStorage_WithoutConfigMap(t *testing.T) {
	fakekubeclient := fakek8s.NewSimpleClientset()
	pipelinerun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}

	pvc, err := InitializeArtifactStorage(pipelinerun, fakekubeclient)
	if err != nil {
		t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
	}

	expectedArtifactPVC := &v1alpha1.ArtifactPVC{
		Name: "pipelineruntest",
	}

	if diff := cmp.Diff(pvc, expectedArtifactPVC); diff != "" {
		t.Fatalf("want %v, but got %v", expectedArtifactPVC, pvc)
	}
}

func TestGetArtifactStorage_WithConfigMap(t *testing.T) {
	fakekubeclient := fakek8s.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace,
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "gs://fake-bucket",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
	)

	bucket, err := GetArtifactStorage("pipelineruntest", fakekubeclient)
	if err != nil {
		t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
	}

	expectedArtifactBucket := &v1alpha1.ArtifactBucket{
		Location: "gs://fake-bucket",
		Secrets: []v1alpha1.SecretParam{{
			FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			SecretKey:  "sakey",
			SecretName: "secret1",
		}},
	}

	if diff := cmp.Diff(bucket, expectedArtifactBucket); diff != "" {
		t.Fatalf("want %v, but got %v", expectedArtifactBucket, bucket)
	}
}

func TestGetArtifactStorage_WithoutConfigMap(t *testing.T) {
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvc, err := GetArtifactStorage("pipelineruntest", fakekubeclient)
	if err != nil {
		t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
	}

	expectedArtifactPVC := &v1alpha1.ArtifactPVC{
		Name: "pipelineruntest",
	}

	if diff := cmp.Diff(pvc, expectedArtifactPVC); diff != "" {
		t.Fatalf("want %v, but got %v", expectedArtifactPVC, pvc)
	}
}
