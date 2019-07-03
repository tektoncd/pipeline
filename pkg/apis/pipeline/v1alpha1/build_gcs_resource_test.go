/*
Copyright 2019 The Tekton Authors.

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
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Invalid_BuildGCSResource(t *testing.T) {
	testcases := []struct {
		name             string
		pipelineResource *v1alpha1.PipelineResource
	}{{
		name: "no location params",
		pipelineResource: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "buildgcs-resource-with-no-location-param",
			},
			Spec: v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeStorage,
				Params: []v1alpha1.Param{{
					Name:  "NotLocation",
					Value: "doesntmatter",
				}, {
					Name:  "type",
					Value: "build-gcs",
				}},
			},
		},
	}, {
		name: "location param with empty value",
		pipelineResource: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gcs-resource-with-empty-location-param",
			},
			Spec: v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeStorage,
				Params: []v1alpha1.Param{{
					Name:  "Location",
					Value: "",
				}, {
					Name:  "type",
					Value: "build-gcs",
				}},
			},
		},
	}, {
		name: "no artifactType params",
		pipelineResource: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "buildgcs-resource-with-no-artifactType-param",
			},
			Spec: v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeStorage,
				Params: []v1alpha1.Param{{
					Name:  "Location",
					Value: "gs://test",
				}, {
					Name:  "type",
					Value: "build-gcs",
				}},
			},
		},
	}, {
		name: "artifactType param with empty value",
		pipelineResource: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gcs-resource-with-empty-location-param",
			},
			Spec: v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeStorage,
				Params: []v1alpha1.Param{{
					Name:  "Location",
					Value: "gs://test",
				}, {
					Name:  "type",
					Value: "build-gcs",
				}, {
					Name:  "ArtifactType",
					Value: "",
				}},
			},
		},
	}, {
		name: "artifactType param with invalid value",
		pipelineResource: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gcs-resource-with-empty-location-param",
			},
			Spec: v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeStorage,
				Params: []v1alpha1.Param{{
					Name:  "Location",
					Value: "gs://test",
				}, {
					Name:  "type",
					Value: "build-gcs",
				}, {
					Name:  "ArtifactType",
					Value: "invalid-type",
				}},
			},
		},
	}, {
		name: "artifactType param with secrets value",
		pipelineResource: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gcs-resource-with-secrets",
			},
			Spec: v1alpha1.PipelineResourceSpec{
				Type: v1alpha1.PipelineResourceTypeStorage,
				Params: []v1alpha1.Param{{
					Name:  "Location",
					Value: "gs://test",
				}, {
					Name:  "type",
					Value: "build-gcs",
				}, {
					Name:  "ArtifactType",
					Value: "invalid-type",
				}},
				SecretParams: []v1alpha1.SecretParam{{
					SecretKey:  "secretKey",
					SecretName: "secretName",
					FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
				}},
			},
		},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := v1alpha1.NewStorageResource(tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating BuildGCS resource")
			}
		})
	}
}

func Test_Valid_NewBuildGCSResource(t *testing.T) {
	pr := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "build-gcs-resource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeStorage,
			Params: []v1alpha1.Param{{
				Name:  "Location",
				Value: "gs://fake-bucket",
			}, {
				Name:  "type",
				Value: "build-gcs",
			}, {
				Name:  "ArtifactType",
				Value: "Manifest",
			}},
		},
	}
	expectedGCSResource := &v1alpha1.BuildGCSResource{
		Name:         "build-gcs-resource",
		Location:     "gs://fake-bucket",
		Type:         v1alpha1.PipelineResourceTypeStorage,
		ArtifactType: "Manifest",
	}

	r, err := v1alpha1.NewBuildGCSResource(pr)
	if err != nil {
		t.Fatalf("Unexpected error creating BuildGCS resource: %s", err)
	}
	if d := cmp.Diff(expectedGCSResource, r); d != "" {
		t.Errorf("Mismatch of BuildGCS resource: %s", d)
	}
}

func Test_BuildGCSGetReplacements(t *testing.T) {
	r := &v1alpha1.BuildGCSResource{
		Name:     "gcs-resource",
		Location: "gs://fake-bucket",
		Type:     v1alpha1.PipelineResourceTypeBuildGCS,
	}
	expectedReplacementMap := map[string]string{
		"name":     "gcs-resource",
		"type":     "build-gcs",
		"location": "gs://fake-bucket",
		"path":     "",
	}
	if d := cmp.Diff(r.Replacements(), expectedReplacementMap); d != "" {
		t.Errorf("BuildGCS Replacement map mismatch: %s", d)
	}
}

func Test_BuildGCSGetDownloadContainerSpec(t *testing.T) {
	testcases := []struct {
		name           string
		resource       *v1alpha1.BuildGCSResource
		wantContainers []corev1.Container
		wantErr        bool
	}{{
		name: "valid download protected buckets",
		resource: &v1alpha1.BuildGCSResource{
			Name:           "gcs-valid",
			Location:       "gs://some-bucket",
			DestinationDir: "/workspace",
			ArtifactType:   "Archive",
		},
		wantContainers: []corev1.Container{{
			Name:    "create-dir-gcs-valid-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /workspace"},
		}, {
			Name:  "storage-fetch-gcs-valid-mz4c7",
			Image: "gcr.io/cloud-builders/gcs-fetcher:latest",
			Args: []string{"--type", "Archive", "--location", "gs://some-bucket",
				"--dest_dir", "/workspace"},
		}},
	}, {
		name: "invalid no destination directory set",
		resource: &v1alpha1.BuildGCSResource{
			Name:         "gcs-invalid",
			Location:     "gs://some-bucket",
			ArtifactType: "Archive",
		},
		wantErr: true,
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			names.TestingSeed()
			gotContainers, err := tc.resource.GetDownloadContainerSpec()
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}
			if d := cmp.Diff(gotContainers, tc.wantContainers); d != "" {
				t.Errorf("Error mismatch between download containers spec: %s", d)
			}
		})
	}
}

func Test_BuildGCSGetUploadContainerSpec(t *testing.T) {
	testcases := []struct {
		name           string
		resource       *v1alpha1.BuildGCSResource
		wantContainers []corev1.Container
		wantErr        bool
	}{{
		name: "valid upload to protected buckets with directory paths",
		resource: &v1alpha1.BuildGCSResource{
			Name:           "gcs-valid",
			Location:       "gs://some-bucket/manifest.json",
			DestinationDir: "/workspace",
			ArtifactType:   "Manifest",
		},
		wantContainers: []corev1.Container{{
			Name:  "storage-upload-gcs-valid-9l9zj",
			Image: "gcr.io/cloud-builders/gcs-uploader:latest",
			Args:  []string{"--location", "gs://some-bucket/manifest.json", "--dir", "/workspace"},
		}},
	}, {
		name: "invalid upload to protected buckets with single file",
		resource: &v1alpha1.BuildGCSResource{
			Name:           "gcs-valid",
			ArtifactType:   "Archive",
			Location:       "gs://some-bucket",
			DestinationDir: "/workspace/results.tar",
		},
		wantErr: true,
	}, {
		name: "invalid upload with no source directory path",
		resource: &v1alpha1.BuildGCSResource{
			Name:     "gcs-invalid",
			Location: "gs://some-bucket/manifest.json",
		},
		wantErr: true,
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			gotContainers, err := tc.resource.GetUploadContainerSpec()
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}

			if d := cmp.Diff(gotContainers, tc.wantContainers); d != "" {
				t.Errorf("Error mismatch between upload containers spec: %s", d)
			}
		})
	}
}
