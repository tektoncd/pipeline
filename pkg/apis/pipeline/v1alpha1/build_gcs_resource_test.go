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

func Test_Invalid_BuildGCSResource(t *testing.T) {
	for _, tc := range []struct {
		name             string
		pipelineResource *v1alpha1.PipelineResource
	}{{
		name: "no location params",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-no-location-param", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("NotLocation", "doesntmatter"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
		)),
	}, {
		name: "location param with empty value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-empty-location-param", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", ""),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
		)),
	}, {
		name: "no artifactType params",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-no-artifactType-param", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
		)),
	}, {
		name: "artifactType param with empty value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-empty-artifactType-param", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
			tb.PipelineResourceSpecParam("ArtifactType", ""),
		)),
	}, {
		name: "artifactType param with invalid value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-invalid-artifactType-param", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
			tb.PipelineResourceSpecParam("ArtifactType", "invalid-type"),
		)),
	}, {
		name: "artifactType param with secrets value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-invalid-artifactType-param-and-secrets", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
			tb.PipelineResourceSpecParam("ArtifactType", "invalid-type"),
			tb.PipelineResourceSpecSecretParam("secretKey", "secretName", "GOOGLE_APPLICATION_CREDENTIALS"),
		)),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := v1alpha1.NewStorageResource(tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating BuildGCS resource")
			}
		})
	}
}

func Test_Valid_NewBuildGCSResource(t *testing.T) {
	pr := tb.PipelineResource("build-gcs-resource", "default", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
		tb.PipelineResourceSpecParam("type", "build-gcs"),
		tb.PipelineResourceSpecParam("ArtifactType", "Manifest"),
	))
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
	}
	if d := cmp.Diff(r.Replacements(), expectedReplacementMap); d != "" {
		t.Errorf("BuildGCS Replacement map mismatch: %s", d)
	}
}

func Test_BuildGCSGetDownloadSteps(t *testing.T) {
	resource := &v1alpha1.BuildGCSResource{
		Name:         "gcs-valid",
		Location:     "gs://some-bucket",
		ArtifactType: "Archive",
	}
	wantSteps := []v1alpha1.Step{{Container: corev1.Container{
		Name:    "create-dir-gcs-valid-9l9zj",
		Image:   "override-with-bash-noop:latest",
		Command: []string{"/ko-app/bash"},
		Args:    []string{"-args", "mkdir -p /workspace"},
	}}, {Container: corev1.Container{
		Name:  "storage-fetch-gcs-valid-mz4c7",
		Image: "gcr.io/cloud-builders/gcs-fetcher:latest",
		Args: []string{"--type", "Archive", "--location", "gs://some-bucket",
			"--dest_dir", "/workspace"},
	}}}
	names.TestingSeed()
	got, err := resource.GetDownloadSteps("/workspace")
	if err != nil {
		t.Fatalf("GetDownloadSteps: %v", err)
	}
	if d := cmp.Diff(got, wantSteps); d != "" {
		t.Errorf("Error mismatch between download steps: %s", d)
	}
}

func Test_BuildGCSGetUploadSteps(t *testing.T) {
	for _, tc := range []struct {
		name      string
		resource  *v1alpha1.BuildGCSResource
		wantSteps []v1alpha1.Step
		wantErr   bool
	}{{
		name: "valid upload to protected buckets with directory paths",
		resource: &v1alpha1.BuildGCSResource{
			Name:         "gcs-valid",
			Location:     "gs://some-bucket/manifest.json",
			ArtifactType: "Manifest",
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:  "storage-upload-gcs-valid-mssqb",
			Image: "gcr.io/cloud-builders/gcs-uploader:latest",
			Args:  []string{"--location", "gs://some-bucket/manifest.json", "--dir", "/workspace"},
		}}},
	}, {
		name: "invalid upload to protected buckets with single file",
		resource: &v1alpha1.BuildGCSResource{
			Name:         "gcs-valid",
			ArtifactType: "Archive",
			Location:     "gs://some-bucket",
		},
		wantErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.resource.GetUploadSteps("/workspace")
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("GetUploadSteps: %v", err)
			}

			if d := cmp.Diff(got, tc.wantSteps); d != "" {
				t.Errorf("Error mismatch between upload steps: %s", d)
			}
		})
	}
}
