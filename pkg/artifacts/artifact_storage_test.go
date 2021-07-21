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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/storage"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/kmeta"
)

var (
	images = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "override-with-nop:latest",
		GitImage:                 "override-with-git:latest",
		KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
		ShellImage:               "busybox",
		GsutilImage:              "gcr.io/google.com/cloudsdktool/cloud-sdk",
		PRImage:                  "override-with-pr:latest",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}
	pipelinerun = &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	defaultStorageClass   *string
	customStorageClass    = "custom-storage-class"
	persistentVolumeClaim = getPersistentVolumeClaim(config.DefaultPVCSize, defaultStorageClass)
	quantityComparer      = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})

	pipelineWithTasksWithFrom = v1beta1.Pipeline{
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{{
				Name: "input1",
				Type: "git",
			}, {
				Name: "output",
				Type: "git",
			}},
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "task1",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name:     "foo",
							Resource: "output",
						}},
					},
				},
				{
					Name: "task2",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
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

func getPersistentVolumeClaim(size string, storageClassName *string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelineruntest-pvc", Namespace: pipelinerun.Namespace, OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(pipelinerun)}},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:        corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}},
			StorageClassName: storageClassName,
		},
	}
	return pvc
}

func TestNeedsPVC(t *testing.T) {
	for _, c := range []struct {
		desc         string
		bucketConfig map[string]string
		pvcNeeded    bool
	}{{
		desc: "valid bucket",
		bucketConfig: map[string]string{
			config.BucketLocationKey:                 "gs://fake-bucket",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		pvcNeeded: false,
	}, {
		desc: "location empty",
		bucketConfig: map[string]string{
			config.BucketLocationKey:                 "",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		pvcNeeded: true,
	}, {
		desc: "missing location",
		bucketConfig: map[string]string{
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		pvcNeeded: true,
	}, {
		desc:         "no config map data",
		bucketConfig: map[string]string{},
		pvcNeeded:    true,
	}, {
		desc: "no secret",
		bucketConfig: map[string]string{
			config.BucketLocationKey: "gs://fake-bucket",
		},
		pvcNeeded: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			artifactBucket, _ := config.NewArtifactBucketFromMap(c.bucketConfig)
			configs := config.Config{
				ArtifactBucket: artifactBucket,
			}
			ctx := config.ToContext(context.Background(), &configs)
			needed := needsPVC(ctx)
			if needed != c.pvcNeeded {
				t.Fatalf("Expected that ConfigMapNeedsPVC would be %t, but was %t", c.pvcNeeded, needed)
			}
		})
	}

}

func TestInitializeArtifactStorage(t *testing.T) {
	for _, c := range []struct {
		desc                    string
		storageConfig           map[string]string
		storagetype             string
		expectedArtifactStorage ArtifactStorageInterface
	}{{
		desc: "pvc configmap size",
		storageConfig: map[string]string{
			config.PVCSizeKey: "10Gi",
		},
		storagetype: "pvc",
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: getPersistentVolumeClaim("10Gi", defaultStorageClass),
			ShellImage:            "busybox",
		},
	}, {
		desc: "pvc configmap storageclass",
		storageConfig: map[string]string{
			config.PVCStorageClassNameKey: customStorageClass,
		},
		storagetype: "pvc",
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: getPersistentVolumeClaim("5Gi", &customStorageClass),
			ShellImage:            "busybox",
		},
	}, {
		desc: "valid bucket",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "gs://fake-bucket",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		storagetype: "bucket",
		expectedArtifactStorage: &storage.ArtifactBucket{
			Location: "gs://fake-bucket",
			Secrets: []resourcev1alpha1.SecretParam{{
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
			ShellImage:  "busybox",
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
	}, {
		desc: "location empty",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		storagetype: "pvc",
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: persistentVolumeClaim,
			ShellImage:            "busybox",
		},
	}, {
		desc: "missing location",
		storageConfig: map[string]string{
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		storagetype: "pvc",
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: persistentVolumeClaim,
			ShellImage:            "busybox",
		},
	}, {
		desc:          "no config map data",
		storageConfig: map[string]string{},
		storagetype:   "pvc",
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:                  "pipelineruntest",
			PersistentVolumeClaim: persistentVolumeClaim,
			ShellImage:            "busybox",
		},
	}, {
		desc: "no secret",
		storageConfig: map[string]string{
			config.BucketLocationKey: "gs://fake-bucket",
		},
		storagetype: "bucket",
		expectedArtifactStorage: &storage.ArtifactBucket{
			Location:    "gs://fake-bucket",
			ShellImage:  "busybox",
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
	}, {
		desc: "valid bucket with boto config",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "s3://fake-bucket",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
			config.BucketServiceAccountFieldNameKey:  "BOTO_CONFIG",
		},
		storagetype: "bucket",
		expectedArtifactStorage: &storage.ArtifactBucket{
			Location:    "s3://fake-bucket",
			ShellImage:  "busybox",
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Secrets: []resourcev1alpha1.SecretParam{{
				FieldName:  "BOTO_CONFIG",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset()
			configs := config.Config{}
			if c.storagetype == "bucket" {
				configs.ArtifactBucket, _ = config.NewArtifactBucketFromMap(c.storageConfig)
			} else {
				configs.ArtifactPVC, _ = config.NewArtifactPVCFromMap(c.storageConfig)
			}
			ctx := config.ToContext(context.Background(), &configs)
			artifactStorage, err := InitializeArtifactStorage(ctx, images, pipelinerun, &pipelineWithTasksWithFrom.Spec, fakekubeclient)
			if err != nil {
				t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
			}
			if artifactStorage == nil {
				t.Fatal("artifactStorage was nil, expected an actual value")
			}
			// If the expected storage type is PVC, make sure we're actually creating that PVC.
			if c.storagetype == "pvc" {
				_, err := fakekubeclient.CoreV1().PersistentVolumeClaims(pipelinerun.Namespace).Get(ctx, GetPVCName(pipelinerun), metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Error getting expected PVC %s for PipelineRun %s: %s", GetPVCName(pipelinerun), pipelinerun.Name, err)
				}
			}
			// Make sure we don't get any errors running CleanupArtifactStorage against the resulting storage, whether it's
			// a bucket or a PVC.
			if err := CleanupArtifactStorage(ctx, pipelinerun, fakekubeclient); err != nil {
				t.Fatalf("Error cleaning up artifact storage: %s", err)
			}
			if d := cmp.Diff(artifactStorage.GetType(), c.storagetype); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
			if d := cmp.Diff(artifactStorage, c.expectedArtifactStorage, quantityComparer); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestInitializeArtifactStorageNoStorageNeeded(t *testing.T) {
	// This Pipeline has Tasks that use both inputs and outputs, but there is
	// no link between the inputs and outputs, so no storage is needed
	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "task1",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "resource",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name:     "output",
							Resource: "resource",
						}},
					},
				},
				{
					Name: "task2",
					TaskRef: &v1beta1.TaskRef{
						Name: "task",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "input1",
							Resource: "resource",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{{
							Name:     "output",
							Resource: "resource",
						}},
					},
				},
			},
		},
	}
	pipelinerun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelinerun",
			Namespace: "namespace",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "pipeline",
			},
		},
	}
	for _, c := range []struct {
		desc          string
		storageConfig map[string]string
		storagetype   string
	}{{
		desc: "has pvc configured",
		storageConfig: map[string]string{
			config.PVCSizeKey: "10Gi",
		},
		storagetype: "pvc",
	}, {
		desc: "has bucket configured",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "gs://fake-bucket",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		storagetype: "bucket",
	}, {
		desc: "no configmap",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		storagetype: "bucket",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset()
			configs := config.Config{}
			if c.storagetype == "bucket" {
				configs.ArtifactBucket, _ = config.NewArtifactBucketFromMap(c.storageConfig)
			} else {
				configs.ArtifactPVC, _ = config.NewArtifactPVCFromMap(c.storageConfig)
			}
			ctx := config.ToContext(context.Background(), &configs)
			artifactStorage, err := InitializeArtifactStorage(ctx, images, pipelinerun, &pipeline.Spec, fakekubeclient)
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
	pipelinerun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	for _, c := range []struct {
		desc          string
		storageConfig map[string]string
	}{{
		desc: "location empty",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
	}, {
		desc: "missing location",
		storageConfig: map[string]string{
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
	}, {
		desc:          "no config map data",
		storageConfig: map[string]string{},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset(getPVCSpec(pipelinerun, persistentVolumeClaim.Spec.Resources.Requests["storage"], defaultStorageClass))
			ab, err := config.NewArtifactBucketFromMap(c.storageConfig)
			if err != nil {
				t.Fatalf("Error getting an ArtifactBucket from data %s, %s", c.storageConfig, err)
			}
			configs := config.Config{
				ArtifactBucket: ab,
			}
			ctx := config.ToContext(context.Background(), &configs)
			_, err = fakekubeclient.CoreV1().PersistentVolumeClaims(pipelinerun.Namespace).Get(ctx, GetPVCName(pipelinerun), metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Error getting expected PVC %s for PipelineRun %s: %s", GetPVCName(pipelinerun), pipelinerun.Name, err)
			}
			if err := CleanupArtifactStorage(ctx, pipelinerun, fakekubeclient); err != nil {
				t.Fatalf("Error cleaning up artifact storage: %s", err)
			}
			_, err = fakekubeclient.CoreV1().PersistentVolumeClaims(pipelinerun.Namespace).Get(ctx, GetPVCName(pipelinerun), metav1.GetOptions{})
			if err == nil {
				t.Fatalf("Found PVC %s for PipelineRun %s after it should have been cleaned up", GetPVCName(pipelinerun), pipelinerun.Name)
			} else if !errors.IsNotFound(err) {
				t.Fatalf("Error checking if PVC %s for PipelineRun %s has been cleaned up: %s", GetPVCName(pipelinerun), pipelinerun.Name, err)
			}
		})
	}
}

func TestInitializeArtifactStorageWithoutConfigMap(t *testing.T) {
	pipelinerun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelineruntest",
			Namespace: "foo",
		},
	}
	fakekubeclient := fakek8s.NewSimpleClientset()

	pvc, err := InitializeArtifactStorage(context.Background(), images, pipelinerun, &pipelineWithTasksWithFrom.Spec, fakekubeclient)
	if err != nil {
		t.Fatalf("Somehow had error initializing artifact storage run out of fake client: %s", err)
	}

	expectedArtifactPVC := &storage.ArtifactPVC{
		Name:                  "pipelineruntest",
		PersistentVolumeClaim: persistentVolumeClaim,
		ShellImage:            "busybox",
	}

	if d := cmp.Diff(pvc, expectedArtifactPVC, cmpopts.IgnoreUnexported(resource.Quantity{})); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}
}

func TestGetArtifactStorageWithConfig(t *testing.T) {
	pipelinerun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	for _, c := range []struct {
		desc                    string
		storageConfig           map[string]string
		expectedArtifactStorage ArtifactStorageInterface
	}{{
		desc: "valid bucket",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "gs://fake-bucket",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		expectedArtifactStorage: &storage.ArtifactBucket{
			Location: "gs://fake-bucket",
			Secrets: []resourcev1alpha1.SecretParam{{
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "sakey",
				SecretName: "secret1",
			}},
			ShellImage:  "busybox",
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
	}, {
		desc: "location empty",
		storageConfig: map[string]string{
			config.BucketLocationKey:                 "",
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:       pipelinerun.Name,
			ShellImage: "busybox",
		},
	}, {
		desc: "missing location",
		storageConfig: map[string]string{
			config.BucketServiceAccountSecretNameKey: "secret1",
			config.BucketServiceAccountSecretKeyKey:  "sakey",
		},
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:       pipelinerun.Name,
			ShellImage: "busybox",
		},
	}, {
		desc:          "no config map data",
		storageConfig: map[string]string{},
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:       pipelinerun.Name,
			ShellImage: "busybox",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset()
			ab, err := config.NewArtifactBucketFromMap(c.storageConfig)
			if err != nil {
				t.Fatalf("Error getting an ArtifactBucket from data %s, %s", c.storageConfig, err)
			}
			configs := config.Config{
				ArtifactBucket: ab,
			}
			ctx := config.ToContext(context.Background(), &configs)
			artifactStorage := GetArtifactStorage(ctx, images, pipelinerun.Name, fakekubeclient)

			if d := cmp.Diff(artifactStorage, c.expectedArtifactStorage); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetArtifactStorageWithoutConfig(t *testing.T) {
	fakekubeclient := fakek8s.NewSimpleClientset()
	pvc := GetArtifactStorage(context.Background(), images, "pipelineruntest", fakekubeclient)

	expectedArtifactPVC := &storage.ArtifactPVC{
		Name:       "pipelineruntest",
		ShellImage: "busybox",
	}

	if d := cmp.Diff(pvc, expectedArtifactPVC); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}
}

func TestGetArtifactStorageWithPVCConfig(t *testing.T) {
	prName := "pipelineruntest"
	for _, c := range []struct {
		desc                    string
		storageConfig           map[string]string
		expectedArtifactStorage ArtifactStorageInterface
	}{{
		desc: "valid pvc",
		storageConfig: map[string]string{
			config.PVCSizeKey: "10Gi",
		},
		expectedArtifactStorage: &storage.ArtifactPVC{
			Name:       "pipelineruntest",
			ShellImage: "busybox",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fakekubeclient := fakek8s.NewSimpleClientset()
			ap, err := config.NewArtifactPVCFromMap(c.storageConfig)
			if err != nil {
				t.Fatalf("Error getting an ArtifactPVC from data %s, %s", c.storageConfig, err)
			}
			configs := config.Config{
				ArtifactPVC: ap,
			}
			ctx := config.ToContext(context.Background(), &configs)
			artifactStorage := GetArtifactStorage(ctx, images, prName, fakekubeclient)

			if d := cmp.Diff(artifactStorage, c.expectedArtifactStorage); d != "" {
				t.Fatalf(diff.PrintWantGot(d))
			}
		})
	}
}
