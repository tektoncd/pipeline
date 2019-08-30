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

func Test_Invalid_NewStorageResource(t *testing.T) {
	for _, tc := range []struct {
		name             string
		pipelineResource *v1alpha1.PipelineResource
	}{{
		name: "wrong-resource-type",
		pipelineResource: tb.PipelineResource("gcs-resource", "default",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGit),
		),
	}, {
		name: "unimplemented type",
		pipelineResource: tb.PipelineResource("gcs-resource", "default",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
				tb.PipelineResourceSpecParam("type", "non-existent-type"),
			),
		),
	}, {
		name: "no type",
		pipelineResource: tb.PipelineResource("gcs-resource", "default",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
			),
		),
	}, {
		name: "no location params",
		pipelineResource: tb.PipelineResource("gcs-resource", "default",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("NotLocation", "doesntmatter"),
				tb.PipelineResourceSpecParam("type", "gcs"),
			),
		),
	}, {
		name: "gcs location param with empty value",
		pipelineResource: tb.PipelineResource("gcs-resource", "default",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("Location", ""),
				tb.PipelineResourceSpecParam("type", "gcs"),
			),
		),
	}, {
		name: "s3 location param with empty value",
		pipelineResource: tb.PipelineResource("s3-resource", "default",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("Location", ""),
				tb.PipelineResourceSpecParam("type", "s3"),
			),
		),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := v1alpha1.NewStorageResource(tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating GCS resource")
			}
		})
	}
}

func Test_Valid_NewGCSResource(t *testing.T) {
	pr := tb.PipelineResource("gcs-resource", "default", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
		tb.PipelineResourceSpecParam("type", "gcs"),
		tb.PipelineResourceSpecParam("dir", "anything"),
		tb.PipelineResourceSpecSecretParam("GOOGLE_APPLICATION_CREDENTIALS", "secretName", "secretKey"),
	))
	expectedGCSResource := &v1alpha1.GCSResource{
		Name:     "gcs-resource",
		Location: "gs://fake-bucket",
		Type:     v1alpha1.PipelineResourceTypeStorage,
		TypeDir:  true,
		Secrets: []v1alpha1.SecretParam{{
			SecretName: "secretName",
			SecretKey:  "secretKey",
			FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
		}},
	}

	gcsRes, err := v1alpha1.NewGCSResource(pr)
	if err != nil {
		t.Fatalf("Unexpected error creating GCS resource: %s", err)
	}
	if d := cmp.Diff(expectedGCSResource, gcsRes); d != "" {
		t.Errorf("Mismatch of GCS resource: %s", d)
	}
}

func Test_Valid_NewS3Resource(t *testing.T) {
	pr := tb.PipelineResource("s3-resource", "default", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "./source/my-bucket/something.gz"),
		tb.PipelineResourceSpecParam("Bucket", "s3://somewhere"),
		tb.PipelineResourceSpecParam("ObjectName", "artfact.gz"),
		tb.PipelineResourceSpecSecretParam("AWS_SECRET_ACCESS_KEY", "secretName", "secretKey"),
	))
	expectedS3Resource := &v1alpha1.S3Resource{
		Type:       v1alpha1.PipelineResourceTypeStorage,
		Name:       "s3-resource",
		Location:   "./source/my-bucket/something.gz",
		BucketName: "s3://somewhere",
		ObjectName: "artfact.gz",
		Secrets: []v1alpha1.SecretParam{{
			SecretName: "secretName",
			SecretKey:  "secretKey",
			FieldName:  "AWS_SECRET_ACCESS_KEY",
		}},
	}

	s3Res, err := v1alpha1.NewS3Resource(pr)
	if err != nil {
		t.Fatalf("Unexpected error creating S3 resource: %s", err)
	}
	if d := cmp.Diff(expectedS3Resource, s3Res); d != "" {
		t.Errorf("Mismatch of S3 resource: %s", d)
	}
}

func Test_GCSGetReplacements(t *testing.T) {
	gcsResource := &v1alpha1.GCSResource{
		Name:     "gcs-resource",
		Location: "gs://fake-bucket",
		Type:     v1alpha1.PipelineResourceTypeGCS,
	}
	expectedReplacementMap := map[string]string{
		"name":     "gcs-resource",
		"type":     "gcs",
		"location": "gs://fake-bucket",
	}
	if d := cmp.Diff(gcsResource.Replacements(), expectedReplacementMap); d != "" {
		t.Errorf("GCS Replacement map mismatch: %s", d)
	}
}

func Test_GetParams(t *testing.T) {
	pr := tb.PipelineResource("gcs-resource", "default", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "gcs://some-bucket.zip"),
		tb.PipelineResourceSpecParam("type", "gcs"),
		tb.PipelineResourceSpecSecretParam("test-field-name", "test-secret-name", "test-secret-key"),
	))
	gcsResource, err := v1alpha1.NewStorageResource(pr)
	if err != nil {
		t.Fatalf("Error creating storage resource: %s", err.Error())
	}
	expectedSp := []v1alpha1.SecretParam{{
		SecretKey:  "test-secret-key",
		SecretName: "test-secret-name",
		FieldName:  "test-field-name",
	}}
	if d := cmp.Diff(gcsResource.GetSecretParams(), expectedSp); d != "" {
		t.Errorf("Error mismatch on storage secret params: %s", d)
	}
}

func Test_GetDownloadSteps(t *testing.T) {
	names.TestingSeed()

	for _, tc := range []struct {
		name        string
		gcsResource *v1alpha1.GCSResource
		wantSteps   []v1alpha1.Step
		wantErr     bool
	}{{
		name: "valid download protected buckets",
		gcsResource: &v1alpha1.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			TypeDir:  true,
			Secrets: []v1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "key.json",
			}},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-gcs-valid-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /workspace"},
		}}, {Container: corev1.Container{
			Name:    "fetch-gcs-valid-mz4c7",
			Image:   "override-with-gsutil-image:latest",
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "rsync -d -r gs://some-bucket /workspace"},
			Env: []corev1.EnvVar{{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/var/secret/secretName/key.json",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "volume-gcs-valid-secretName",
				MountPath: "/var/secret/secretName",
			}},
		}}},
	}, {
		name: "duplicate secret mount paths",
		gcsResource: &v1alpha1.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			Secrets: []v1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "fieldName",
				SecretKey:  "key.json",
			}, {
				SecretKey:  "key.json",
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-gcs-valid-mssqb",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /workspace"},
		}}, {Container: corev1.Container{
			Name:    "fetch-gcs-valid-78c5n",
			Image:   "override-with-gsutil-image:latest",
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "cp gs://some-bucket /workspace"},
			Env: []corev1.EnvVar{{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/var/secret/secretName/key.json",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "volume-gcs-valid-secretName",
				MountPath: "/var/secret/secretName",
			}},
		}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			gotContainers, err := tc.gcsResource.GetDownloadSteps("/workspace")
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}
			if d := cmp.Diff(gotContainers, tc.wantSteps); d != "" {
				t.Errorf("Error mismatch between download containers spec: %s", d)
			}
		})
	}
}

func Test_GetUploadSteps(t *testing.T) {
	names.TestingSeed()

	for _, tc := range []struct {
		name        string
		gcsResource *v1alpha1.GCSResource
		wantSteps   []v1alpha1.Step
		wantErr     bool
	}{{
		name: "valid upload to protected buckets with directory paths",
		gcsResource: &v1alpha1.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			TypeDir:  true,
			Secrets: []v1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "key.json",
			}},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "upload-gcs-valid-9l9zj",
			Image:   "override-with-gsutil-image:latest",
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "rsync -d -r /workspace/ gs://some-bucket"},
			Env:     []corev1.EnvVar{{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/secretName/key.json"}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "volume-gcs-valid-secretName",
				MountPath: "/var/secret/secretName",
			}},
		}}},
	}, {
		name: "duplicate secret mount paths",
		gcsResource: &v1alpha1.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			Secrets: []v1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "key.json",
			}, {
				SecretKey:  "key.json",
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}},
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "upload-gcs-valid-mz4c7",
			Image:   "override-with-gsutil-image:latest",
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "cp /workspace/* gs://some-bucket"},
			Env: []corev1.EnvVar{
				{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secret/secretName/key.json"},
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "volume-gcs-valid-secretName",
				MountPath: "/var/secret/secretName",
			}},
		}}},
	}, {
		name: "valid upload to protected buckets with single file",
		gcsResource: &v1alpha1.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			TypeDir:  false,
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "upload-gcs-valid-mssqb",
			Image:   "override-with-gsutil-image:latest",
			Command: []string{"/ko-app/gsutil"},
			Args:    []string{"-args", "cp /workspace/* gs://some-bucket"},
		}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			gotContainers, err := tc.gcsResource.GetUploadSteps("/workspace/")
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}

			if d := cmp.Diff(gotContainers, tc.wantSteps); d != "" {
				t.Errorf("Error mismatch between upload containers spec: %s", d)
			}
		})
	}
}
