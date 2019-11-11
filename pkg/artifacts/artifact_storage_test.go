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

package artifacts

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/system"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	logtesting "knative.dev/pkg/logging/testing"
)

var (
	images = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "tianon/true",
		GitImage:                 "override-with-git:latest",
		CredsImage:               "override-with-creds:latest",
		KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
		ShellImage:               "busybox",
		GsutilImage:              "google/cloud-sdk",
		BuildGCSFetcherImage:     "gcr.io/cloud-builders/gcs-fetcher:latest",
		PRImage:                  "override-with-pr:latest",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}
	pipelinerun = &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	defaultStorageClass   *string
	customStorageClass    = "custom-storage-class"
	persistentVolumeClaim = GetPersistentVolumeClaim(DefaultPVCSize, defaultStorageClass)
	quantityComparer      = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})

	pipelineWithTasksWithFrom = v1alpha1.Pipeline{
		Spec: v1alpha1.PipelineSpec{
			Resources: []v1alpha1.PipelineDeclaredResource{{
				Name: "input1",
				Type: "git",
			}, {
				Name: "output",
				Type: "git",
			}},
			Tasks: []v1alpha1.PipelineTask{
				{
					Name: "task1",
					TaskRef: &v1alpha1.TaskRef{
						Name: "task",
					},
					Resources: &v1alpha1.PipelineTaskResources{
						Outputs: []v1alpha1.PipelineTaskOutputResource{{
							Name:     "foo",
							Resource: "output",
						}},
					},
				},
				{
					Name: "task2",
					TaskRef: &v1alpha1.TaskRef{
						Name: "task",
					},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name:     "foo",
							Resource: "output",
							From:     []string{"task1"},
						}},
					},
				},
			},
		},
	}
)

func GetPersistentVolumeClaim(size string, storageClassName *string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelineruntest-pvc", Namespace: pipelinerun.Namespace, OwnerReferences: pipelinerun.GetOwnerReference()},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:        corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}},
			StorageClassName: storageClassName,
		},
	}
	return pvc
}

func TestConfigMapNeedsPVC(t *testing.T) {
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
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "gs://fake-bucket",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pvcNeeded: false,
	}, {
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pvcNeeded: true,
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		pvcNeeded: true,
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
		},
		pvcNeeded: true,
	}, {
		desc: "no secret",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey: "gs://fake-bucket",
			},
		},
		pvcNeeded: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			needed, err := ConfigMapNeedsPVC(c.configMap, nil, logger)
			if err != nil {
				t.Fatalf("Somehow had error checking if PVC was needed run: %s", err)
			}
			if needed != c.pvcNeeded {
				t.Fatalf("Expected that ConfigMapNeedsPVC would be %t, but was %t", c.pvcNeeded, needed)
			}
		})
	}

}

func TestInitializeArtifactStorageWithConfigMap(t *testing.T) {
	logger := logtesting.TestLogger(t)
	for _, c := range []struct {
		desc                    string
		configMap               *corev1.ConfigMap
		expectedArtifactStorage ArtifactStorageInterface
		storagetype             string
	}{{
		desc: "pvc configmap size",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetPVCConfigName(),
			},
			Data: map[string]string{
				PVCSizeKey: "10Gi",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: GetPersistentVolumeClaim("10Gi", defaultStorageClass),
			ShellImage:            "busybox",
		},
		storagetype: "pvc",
	}, {
		desc: "pvc configmap storageclass",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetPVCConfigName(),
			},
			Data: map[string]string{
				PVCStorageClassNameKey: customStorageClass,
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: GetPersistentVolumeClaim("5Gi", &customStorageClass),
			ShellImage:            "busybox",
		},
		storagetype: "pvc",
	}, {
		desc: "valid bucket",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "gs://fake-bucket",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactBucket{
			Location: "gs://fake-bucket",
			Secrets: []v1alpha1.SecretParam{{
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
			ShellImage:  "busybox",
			GsutilImage: "google/cloud-sdk",
		},
		storagetype: "bucket",
	}, {
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: persistentVolumeClaim,
			ShellImage:            "busybox",
		},
		storagetype: "pvc",
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: persistentVolumeClaim,
			ShellImage:            "busybox",
		},
		storagetype: "pvc",
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: persistentVolumeClaim,
			ShellImage:            "busybox",
		},
		storagetype: "pvc",
	}, {
		desc: "no secret",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey: "gs://fake-bucket",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactBucket{
			Location:    "gs://fake-bucket",
			ShellImage:  "busybox",
			GsutilImage: "google/cloud-sdk",
		},
		storagetype: "bucket",
	}, {
		desc: "valid bucket with boto config",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "s3://fake-bucket",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
				BucketServiceAccountFieldName:  "BOTO_CONFIG",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactBucket{
			Location:    "s3://fake-bucket",
			ShellImage:  "busybox",
			GsutilImage: "google/cloud-sdk",
			Secrets: []v1alpha1.SecretParam{{
				FieldName:  "BOTO_CONFIG",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
		},
		storagetype: "bucket",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap)
			artifactStorage, err := InitializeArtifactStorage(images, pipelinerun, &pipelineWithTasksWithFrom.Spec, fakekubeclient, logger)
			if err != nil {
				t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
			}
			if artifactStorage == nil {
				t.Fatal("artifactStorage was nil, expected an actual value")
			}
			// If the expected storage type is PVC, make sure we're actually creating that PVC.
			if c.storagetype == "pvc" {
				_, err := fakekubeclient.CoreV1().PersistentVolumeClaims(pipelinerun.Namespace).Get(GetPVCName(pipelinerun), metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Error getting expected PVC %s for PipelineRun %s: %s", GetPVCName(pipelinerun), pipelinerun.Name, err)
				}
			}
			// Make sure we don't get any errors running CleanupArtifactStorage against the resulting storage, whether it's
			// a bucket or a PVC.
			if err := CleanupArtifactStorage(pipelinerun, fakekubeclient, logger); err != nil {
				t.Fatalf("Error cleaning up artifact storage: %s", err)
			}
			if diff := cmp.Diff(artifactStorage.GetType(), c.storagetype); diff != "" {
				t.Fatalf("-want +got: %s", diff)
			}
			if diff := cmp.Diff(artifactStorage, c.expectedArtifactStorage, quantityComparer); diff != "" {
				t.Fatalf("-want +got: %s", diff)
			}
		})
	}
}

func TestInitializeArtifactStorageNoStorageNeeded(t *testing.T) {
	logger := logtesting.TestLogger(t)
	// This Pipeline has Tasks that use both inputs and outputs, but there is
	// no link between the inputs and outputs, so no storage is needed
	pipeline := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{
				{
					Name: "task1",
					TaskRef: &v1alpha1.TaskRef{
						Name: "task",
					},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "resource",
						}},
						Outputs: []v1alpha1.PipelineTaskOutputResource{{
							Name:     "output",
							Resource: "resource",
						}},
					},
				},
				{
					Name: "task2",
					TaskRef: &v1alpha1.TaskRef{
						Name: "task",
					},
					Resources: &v1alpha1.PipelineTaskResources{
						Inputs: []v1alpha1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "resource",
						}},
						Outputs: []v1alpha1.PipelineTaskOutputResource{{
							Name:     "output",
							Resource: "resource",
						}},
					},
				},
			},
		},
	}
	pipelinerun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelinerun",
			Namespace: "namespace",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: &v1alpha1.PipelineRef{
				Name: "pipeline",
			},
		},
	}
	for _, c := range []struct {
		desc      string
		configMap *corev1.ConfigMap
	}{{
		desc: "has pvc configured",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetPVCConfigName(),
			},
			Data: map[string]string{
				PVCSizeKey: "10Gi",
			},
		},
	}, {
		desc: "has bucket configured",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "gs://fake-bucket",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
	}, {
		desc: "no configmap",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap)
			artifactStorage, err := InitializeArtifactStorage(images, pipelinerun, &pipeline.Spec, fakekubeclient, logger)
			if err != nil {
				t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
			}
			if artifactStorage.GetType() != "none" {
				t.Errorf("Expected NoneArtifactStorage when none is needed but got %s", artifactStorage.GetType())
			}
		})
	}
}

func TestCleanupArtifactStorage(t *testing.T) {
	pipelinerun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	logger := logtesting.TestLogger(t)
	for _, c := range []struct {
		desc      string
		configMap *corev1.ConfigMap
	}{{
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap, GetPVCSpec(pipelinerun, persistentVolumeClaim.Spec.Resources.Requests["storage"], defaultStorageClass))
			_, err := fakekubeclient.CoreV1().PersistentVolumeClaims(pipelinerun.Namespace).Get(GetPVCName(pipelinerun), metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Error getting expected PVC %s for PipelineRun %s: %s", GetPVCName(pipelinerun), pipelinerun.Name, err)
			}
			if err := CleanupArtifactStorage(pipelinerun, fakekubeclient, logger); err != nil {
				t.Fatalf("Error cleaning up artifact storage: %s", err)
			}
			_, err = fakekubeclient.CoreV1().PersistentVolumeClaims(pipelinerun.Namespace).Get(GetPVCName(pipelinerun), metav1.GetOptions{})
			if err == nil {
				t.Fatalf("Found PVC %s for PipelineRun %s after it should have been cleaned up", GetPVCName(pipelinerun), pipelinerun.Name)
			} else if !errors.IsNotFound(err) {
				t.Fatalf("Error checking if PVC %s for PipelineRun %s has been cleaned up: %s", GetPVCName(pipelinerun), pipelinerun.Name, err)
			}
		})
	}
}

func TestInitializeArtifactStorageWithoutConfigMap(t *testing.T) {
	pipelinerun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelineruntest",
			Namespace: "foo",
		},
	}
	logger := logtesting.TestLogger(t)
	fakekubeclient := fakek8s.NewSimpleClientset()

	pvc, err := InitializeArtifactStorage(images, pipelinerun, &pipelineWithTasksWithFrom.Spec, fakekubeclient, logger)
	if err != nil {
		t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
	}

	expectedArtifactPVC := &v1alpha1.ArtifactPVC{
		Name:                  "pipelineruntest",
		PersistentVolumeClaim: persistentVolumeClaim,
		ShellImage:            "busybox",
	}

	if diff := cmp.Diff(pvc, expectedArtifactPVC, cmpopts.IgnoreUnexported(resource.Quantity{})); diff != "" {
		t.Fatalf("-want +got: %s", diff)
	}
}

func TestGetArtifactStorageWithConfigMap(t *testing.T) {
	pipelinerun := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	logger := logtesting.TestLogger(t)
	for _, c := range []struct {
		desc                    string
		configMap               *corev1.ConfigMap
		expectedArtifactStorage ArtifactStorageInterface
	}{{
		desc: "valid bucket",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "gs://fake-bucket",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactBucket{
			Location: "gs://fake-bucket",
			Secrets: []v1alpha1.SecretParam{{
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
			ShellImage:  "busybox",
			GsutilImage: "google/cloud-sdk",
		},
	}, {
		desc: "location empty",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketLocationKey:              "",
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:       pipelinerun.Name,
			ShellImage: "busybox",
		},
	}, {
		desc: "missing location",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
			Data: map[string]string{
				BucketServiceAccountSecretName: "secret1",
				BucketServiceAccountSecretKey:  "sakey",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:       pipelinerun.Name,
			ShellImage: "busybox",
		},
	}, {
		desc: "no config map data",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetBucketConfigName(),
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:       pipelinerun.Name,
			ShellImage: "busybox",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap)

			artifactStorage, err := GetArtifactStorage(images, pipelinerun.Name, fakekubeclient, logger)
			if err != nil {
				t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
			}

			if diff := cmp.Diff(artifactStorage, c.expectedArtifactStorage); diff != "" {
				t.Fatalf("-want +got: %s", diff)
			}
		})
	}
}

func TestGetArtifactStorageWithoutConfigMap(t *testing.T) {
	logger := logtesting.TestLogger(t)
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvc, err := GetArtifactStorage(images, "pipelineruntest", fakekubeclient, logger)
	if err != nil {
		t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
	}

	expectedArtifactPVC := &v1alpha1.ArtifactPVC{
		Name:       "pipelineruntest",
		ShellImage: "busybox",
	}

	if diff := cmp.Diff(pvc, expectedArtifactPVC); diff != "" {
		t.Fatalf("-want +got: %s", diff)
	}
}

func TestGetArtifactStorageWithPVCConfigMap(t *testing.T) {
	logger := logtesting.TestLogger(t)
	prName := "pipelineruntest"
	for _, c := range []struct {
		desc                    string
		configMap               *corev1.ConfigMap
		expectedArtifactStorage ArtifactStorageInterface
	}{{
		desc: "valid pvc",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.GetNamespace(),
				Name:      GetPVCConfigName(),
			},
			Data: map[string]string{
				PVCSizeKey: "10Gi",
			},
		},
		expectedArtifactStorage: &v1alpha1.ArtifactPVC{
			Name:       "pipelineruntest",
			ShellImage: "busybox",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(c.configMap)

			artifactStorage, err := GetArtifactStorage(images, prName, fakekubeclient, logger)
			if err != nil {
				t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
			}

			if diff := cmp.Diff(artifactStorage, c.expectedArtifactStorage); diff != "" {
				t.Fatalf("-want +got: %s", diff)
			}
		})
	}
}
