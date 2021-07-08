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

package storage_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/storage"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

var images = pipeline.Images{
	EntrypointImage:          "override-with-entrypoint:latest",
	NopImage:                 "override-with-nop:latest",
	GitImage:                 "override-with-git:latest",
	KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
	ShellImage:               "busybox",
	GsutilImage:              "gcr.io/google.com/cloudsdktool/cloud-sdk",
	PRImage:                  "override-with-pr:latest",
	ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
}

func TestInvalidNewStorageResource(t *testing.T) {
	for _, tc := range []struct {
		name             string
		pipelineResource *resourcev1alpha1.PipelineResource
	}{{
		name: "wrong-resource-type",
		pipelineResource: tb.PipelineResource("gcs-resource",
			tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeGit),
		),
	}, {
		name: "unimplemented type",
		pipelineResource: tb.PipelineResource("gcs-resource",
			tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
				tb.PipelineResourceSpecParam("type", "non-existent-type"),
			),
		),
	}, {
		name: "no type",
		pipelineResource: tb.PipelineResource("gcs-resource",
			tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
			),
		),
	}, {
		name: "no location params",
		pipelineResource: tb.PipelineResource("gcs-resource",
			tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("NotLocation", "doesntmatter"),
				tb.PipelineResourceSpecParam("type", "gcs"),
			),
		),
	}, {
		name: "location param with empty value",
		pipelineResource: tb.PipelineResource("gcs-resource",
			tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("Location", ""),
				tb.PipelineResourceSpecParam("type", "gcs"),
			),
		),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := storage.NewResource("test-resource", images, tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating GCS resource")
			}
		})
	}
}

func TestValidNewGCSResource(t *testing.T) {
	pr := tb.PipelineResource("gcs-resource", tb.PipelineResourceSpec(
		resourcev1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
		tb.PipelineResourceSpecParam("type", "gcs"),
		tb.PipelineResourceSpecParam("dir", "anything"),
		tb.PipelineResourceSpecSecretParam("GOOGLE_APPLICATION_CREDENTIALS", "secretName", "secretKey"),
	))
	expectedGCSResource := &storage.GCSResource{
		Name:     "test-resource",
		Location: "gs://fake-bucket",
		Type:     resourcev1alpha1.PipelineResourceTypeStorage,
		TypeDir:  true,
		Secrets: []resourcev1alpha1.SecretParam{{
			SecretName: "secretName",
			SecretKey:  "secretKey",
			FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
		}},
		ShellImage:  "busybox",
		GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
	}

	gcsRes, err := storage.NewGCSResource("test-resource", images, pr)
	if err != nil {
		t.Fatalf("Unexpected error creating GCS resource: %s", err)
	}
	if d := cmp.Diff(expectedGCSResource, gcsRes); d != "" {
		t.Errorf("Mismatch of GCS resource %s", diff.PrintWantGot(d))
	}
}

func TestGCSGetReplacements(t *testing.T) {
	gcsResource := &storage.GCSResource{
		Name:     "gcs-resource",
		Location: "gs://fake-bucket",
		Type:     resourcev1alpha1.PipelineResourceTypeGCS,
	}
	expectedReplacementMap := map[string]string{
		"name":     "gcs-resource",
		"type":     "gcs",
		"location": "gs://fake-bucket",
	}
	if d := cmp.Diff(gcsResource.Replacements(), expectedReplacementMap); d != "" {
		t.Errorf("GCS Replacement map mismatch %s", diff.PrintWantGot(d))
	}
}

func TestGetParams(t *testing.T) {
	pr := tb.PipelineResource("gcs-resource", tb.PipelineResourceSpec(
		resourcev1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "gcs://some-bucket.zip"),
		tb.PipelineResourceSpecParam("type", "gcs"),
		tb.PipelineResourceSpecSecretParam("test-field-name", "test-secret-name", "test-secret-key"),
	))
	gcsResource, err := storage.NewResource("test-resource", images, pr)
	if err != nil {
		t.Fatalf("Error creating storage resource: %s", err.Error())
	}
	expectedSp := []resourcev1alpha1.SecretParam{{
		SecretKey:  "test-secret-key",
		SecretName: "test-secret-name",
		FieldName:  "test-field-name",
	}}
	if d := cmp.Diff(gcsResource.GetSecretParams(), expectedSp); d != "" {
		t.Errorf("Error mismatch on storage secret params %s", diff.PrintWantGot(d))
	}
}

func TestGetInputSteps(t *testing.T) {
	names.TestingSeed()

	for _, tc := range []struct {
		name        string
		gcsResource *storage.GCSResource
		wantSteps   []v1beta1.Step
		wantErr     bool
	}{{
		name: "valid download protected buckets",
		gcsResource: &storage.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			TypeDir:  true,
			Secrets: []resourcev1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "key.json",
			}},
			ShellImage:  "busybox",
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-gcs-valid-9l9zj",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace"},
		}}, {
			Script: `#!/usr/bin/env bash
if [[ "${GOOGLE_APPLICATION_CREDENTIALS}" != "" ]]; then
  echo GOOGLE_APPLICATION_CREDENTIALS is set, activating Service Account...
  gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
fi
gsutil rsync -d -r gs://some-bucket /workspace
`,
			Container: corev1.Container{
				Name:  "fetch-gcs-valid-mz4c7",
				Image: "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Env: []corev1.EnvVar{{
					Name:  "GOOGLE_APPLICATION_CREDENTIALS",
					Value: "/var/secret/secretName/key.json",
				}, {
					Name:  "HOME",
					Value: "/tekton/home",
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "volume-gcs-valid-secretName",
					MountPath: "/var/secret/secretName",
				}},
			},
		}},
	}, {
		name: "duplicate secret mount paths",
		gcsResource: &storage.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			Secrets: []resourcev1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "fieldName",
				SecretKey:  "key.json",
			}, {
				SecretKey:  "key.json",
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}},
			ShellImage:  "busybox",
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "create-dir-gcs-valid-mssqb",
			Image:   "busybox",
			Command: []string{"mkdir", "-p", "/workspace"},
		}}, {
			Script: `#!/usr/bin/env bash
if [[ "${GOOGLE_APPLICATION_CREDENTIALS}" != "" ]]; then
  echo GOOGLE_APPLICATION_CREDENTIALS is set, activating Service Account...
  gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
fi
gsutil cp gs://some-bucket /workspace
`,
			Container: corev1.Container{
				Name:  "fetch-gcs-valid-78c5n",
				Image: "gcr.io/google.com/cloudsdktool/cloud-sdk",
				Env: []corev1.EnvVar{{
					Name:  "GOOGLE_APPLICATION_CREDENTIALS",
					Value: "/var/secret/secretName/key.json",
				}, {
					Name:  "HOME",
					Value: "/tekton/home",
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "volume-gcs-valid-secretName",
					MountPath: "/var/secret/secretName",
				}},
			},
		}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ts := v1beta1.TaskSpec{}
			gotSpec, err := tc.gcsResource.GetInputTaskModifier(&ts, "/workspace")
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}
			if d := cmp.Diff(tc.wantSteps, gotSpec.GetStepsToPrepend()); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetOutputTaskModifier(t *testing.T) {
	names.TestingSeed()

	for _, tc := range []struct {
		name        string
		gcsResource *storage.GCSResource
		wantSteps   []v1beta1.Step
		wantErr     bool
	}{{
		name: "valid upload to protected buckets with directory paths",
		gcsResource: &storage.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			TypeDir:  true,
			Secrets: []resourcev1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "key.json",
			}},
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "upload-gcs-valid-9l9zj",
			Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Command: []string{"gsutil"},
			Args:    []string{"rsync", "-d", "-r", "/workspace/", "gs://some-bucket"},
			Env: []corev1.EnvVar{{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/var/secret/secretName/key.json",
			}, {
				Name:  "HOME",
				Value: "/tekton/home",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "volume-gcs-valid-secretName",
				MountPath: "/var/secret/secretName",
			}},
		}}},
	}, {
		name: "duplicate secret mount paths",
		gcsResource: &storage.GCSResource{
			Name:     "gcs-valid",
			Location: "gs://some-bucket",
			Secrets: []resourcev1alpha1.SecretParam{{
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				SecretKey:  "key.json",
			}, {
				SecretKey:  "key.json",
				SecretName: "secretName",
				FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			}},
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "upload-gcs-valid-mz4c7",
			Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Command: []string{"gsutil"},
			Args:    []string{"cp", "/workspace/*", "gs://some-bucket"},
			Env: []corev1.EnvVar{{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/var/secret/secretName/key.json",
			}, {
				Name:  "HOME",
				Value: "/tekton/home",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "volume-gcs-valid-secretName",
				MountPath: "/var/secret/secretName",
			}},
		}}},
	}, {
		name: "valid upload to protected buckets with single file",
		gcsResource: &storage.GCSResource{
			Name:        "gcs-valid",
			Location:    "gs://some-bucket",
			TypeDir:     false,
			GsutilImage: "gcr.io/google.com/cloudsdktool/cloud-sdk",
		},
		wantSteps: []v1beta1.Step{{Container: corev1.Container{
			Name:    "upload-gcs-valid-mssqb",
			Image:   "gcr.io/google.com/cloudsdktool/cloud-sdk",
			Command: []string{"gsutil"},
			Args:    []string{"cp", "/workspace/*", "gs://some-bucket"},
			Env: []corev1.EnvVar{{
				Name:  "HOME",
				Value: "/tekton/home",
			}},
		}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ts := v1beta1.TaskSpec{}
			got, err := tc.gcsResource.GetOutputTaskModifier(&ts, "/workspace/")
			if (err != nil) != tc.wantErr {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}

			if d := cmp.Diff(tc.wantSteps, got.GetStepsToAppend()); d != "" {
				t.Errorf("Error mismatch between upload containers spec %s", diff.PrintWantGot(d))
			}
		})
	}
}
