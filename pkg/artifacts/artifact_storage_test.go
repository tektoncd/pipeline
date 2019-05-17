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

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/google/go-cmp/cmp"
	logtesting "github.com/knative/pkg/logging/testing"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/system"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

func TestNeedsPVC(t *testing.T) {
	logger := logtesting.TestLogger(t)
	for _, c := range []struct {
		desc      string
		configMap *corev1.ConfigMap
		pvcNeeded bool
	}{{
		desc: "valid bucket",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "gs://fake-bucket",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pvcNeeded: false,
	}, {
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pvcNeeded: true,
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pvcNeeded: true,
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
		},
		pvcNeeded: true,
	}, {
		desc: "no secret",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey: "gs://fake-bucket",
			},
		},
		pvcNeeded: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			needed, err := NeedsPVC(c.configMap, nil, logger)
			if err != nil {
				t.Fatalf("Somehow had error checking if PVC was needed run: %s", err)
			}
			if needed != c.pvcNeeded {
				t.Fatalf("Expected that NeedsPVC would be %t, but was %t", c.pvcNeeded, needed)
			}
		})
	}
}

func TestInitializeArtifactStorageWithConfigMap(t *testing.T) {
	logger := logtesting.TestLogger(t)
	for _, c := range []struct {
		desc                    string
		configMap               *corev1.ConfigMap
		pipelinerun             *v1alpha1.PipelineRun
		expectedArtifactStorage ArtifactStorageInterface
		storagetype             string
	}{{
		desc: "valid bucket",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "gs://fake-bucket",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactBucket{
			Location: "gs://fake-bucket",
			Secrets: []v1alpha1.SecretParam{{
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
		},
		storagetype: "bucket",
	}, {
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name: "pipelineruntest",
		},
		storagetype: "pvc",
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name: "pipelineruntest",
		},
		storagetype: "pvc",
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name: "pipelineruntest",
		},
		storagetype: "pvc",
	}, {
		desc: "no secret",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey: "gs://fake-bucket",
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactBucket{
			Location: "gs://fake-bucket",
		},
		storagetype: "bucket",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap)
			bucket, err := InitializeArtifactStorage(c.pipelinerun, fakekubeclient, logger)
			if err != nil {
				t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
			}
			// If the expected storage type is PVC, make sure we're actually creating that PVC.
			if c.storagetype == "pvc" {
				_, err := fakekubeclient.CoreV1().PersistentVolumeClaims(c.pipelinerun.Namespace).Get(GetPVCName(c.pipelinerun), metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Error getting expected PVC %s for PipelineRun %s: %s", GetPVCName(c.pipelinerun), c.pipelinerun.Name, err)
				}
			}
			// Make sure we don't get any errors running CleanupArtifactStorage against the resulting storage, whether it's
			// a bucket or a PVC.
			if err := CleanupArtifactStorage(c.pipelinerun, fakekubeclient, logger); err != nil {
				t.Fatalf("Error cleaning up artifact storage: %s", err)
			}
			if diff := cmp.Diff(bucket.GetType(), c.storagetype); diff != "" {
				t.Fatalf("want %v, but got %v", c.storagetype, bucket.GetType())
			}
			if diff := cmp.Diff(bucket, c.expectedArtifactStorage); diff != "" {
				t.Fatalf("want %v, but got %v", c.expectedArtifactStorage, bucket)
			}
		})
	}
}

func TestCleanupArtifactStorage(t *testing.T) {
	logger := logtesting.TestLogger(t)
	for _, c := range []struct {
		desc        string
		configMap   *corev1.ConfigMap
		pipelinerun *v1alpha1.PipelineRun
	}{{
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
		},
		pipelinerun: &v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "pipelineruntest",
			},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap, GetPVCSpec(c.pipelinerun))
			_, err := fakekubeclient.CoreV1().PersistentVolumeClaims(c.pipelinerun.Namespace).Get(GetPVCName(c.pipelinerun), metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Error getting expected PVC %s for PipelineRun %s: %s", GetPVCName(c.pipelinerun), c.pipelinerun.Name, err)
			}
			if err := CleanupArtifactStorage(c.pipelinerun, fakekubeclient, logger); err != nil {
				t.Fatalf("Error cleaning up artifact storage: %s", err)
			}
			_, err = fakekubeclient.CoreV1().PersistentVolumeClaims(c.pipelinerun.Namespace).Get(GetPVCName(c.pipelinerun), metav1.GetOptions{})
			if err == nil {
				t.Fatalf("Found PVC %s for PipelineRun %s after it should have been cleaned up", GetPVCName(c.pipelinerun), c.pipelinerun.Name)
			} else if !errors.IsNotFound(err) {
				t.Fatalf("Error checking if PVC %s for PipelineRun %s has been cleaned up: %s", GetPVCName(c.pipelinerun), c.pipelinerun.Name, err)
			}
		})
	}
}

func TestInitializeArtifactStorageWithoutConfigMap(t *testing.T) {
	logger := logtesting.TestLogger(t)
	fakekubeclient := fakek8s.NewSimpleClientset()
	pipelinerun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}

	pvc, err := InitializeArtifactStorage(pipelinerun, fakekubeclient, logger)
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

func TestGetArtifactStorageWithConfigMap(t *testing.T) {
	logger := logtesting.TestLogger(t)
	prName := "pipelineruntest"
	for _, c := range []struct {
		desc                    string
		configMap               *corev1.ConfigMap
		expectedArtifactStorage ArtifactStorageInterface
	}{{
		desc: "valid bucket",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "gs://fake-bucket",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactBucket{
			Location: "gs://fake-bucket",
			Secrets: []v1alpha1.SecretParam{{
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
		},
	}, {
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketLocationKey:              "",
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{Name: prName},
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
			Data: map[string]string{
				v1alpha1.BucketServiceAccountSecretName: "secret1",
				v1alpha1.BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{Name: prName},
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      v1alpha1.BucketConfigName,
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{Name: prName},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap)

			bucket, err := GetArtifactStorage(prName, fakekubeclient, logger)
			if err != nil {
				t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
			}

			if diff := cmp.Diff(bucket, c.expectedArtifactStorage); diff != "" {
				t.Fatalf("want %v, but got %v", c.expectedArtifactStorage, bucket)
			}
		})
	}
}

func TestGetArtifactStorageWithoutConfigMap(t *testing.T) {
	logger := logtesting.TestLogger(t)
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvc, err := GetArtifactStorage("pipelineruntest", fakekubeclient, logger)
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
