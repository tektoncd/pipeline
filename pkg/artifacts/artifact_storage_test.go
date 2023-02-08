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
		ShellImage:               "busybox",
		GsutilImage:              "gcr.io/google.com/cloudsdktool/cloud-sdk",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}
	pipelinerun = &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	defaultStorageClass   *string
	persistentVolumeClaim = getPersistentVolumeClaim(config.DefaultPVCSize, defaultStorageClass)
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
