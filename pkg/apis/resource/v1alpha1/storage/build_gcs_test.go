/*
Copyright 2019-2020 The Tekton Authors

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
	CredsImage:               "override-with-creds:latest",
	KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
	ShellImage:               "busybox",
	GsutilImage:              "gcr.io/google.com/cloudsdktool/cloud-sdk",
	BuildGCSFetcherImage:     "gcr.io/cloud-builders/gcs-fetcher:latest",
	PRImage:                  "override-with-pr:latest",
	ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
}

func TestBuildGCSResource_Invalid(t *testing.T) {
	for _, tc := range []struct {
		name             string
		pipelineResource *resourcev1alpha1.PipelineResource
	}{{
		name: "no location params",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-no-location-param", tb.PipelineResourceSpec(
			resourcev1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("NotLocation", "doesntmatter"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
		)),
	}, {
		name: "location param with empty value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-empty-location-param", tb.PipelineResourceSpec(
			resourcev1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", ""),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
		)),
	}, {
		name: "no artifactType params",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-no-artifactType-param", tb.PipelineResourceSpec(
			resourcev1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
		)),
	}, {
		name: "artifactType param with empty value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-empty-artifactType-param", tb.PipelineResourceSpec(
			resourcev1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
			tb.PipelineResourceSpecParam("ArtifactType", ""),
		)),
	}, {
		name: "artifactType param with invalid value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-invalid-artifactType-param", tb.PipelineResourceSpec(
			resourcev1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
			tb.PipelineResourceSpecParam("ArtifactType", "invalid-type"),
		)),
	}, {
		name: "artifactType param with secrets value",
		pipelineResource: tb.PipelineResource("buildgcs-resource-with-invalid-artifactType-param-and-secrets", tb.PipelineResourceSpec(
			resourcev1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("Location", "gs://test"),
			tb.PipelineResourceSpecParam("type", "build-gcs"),
			tb.PipelineResourceSpecParam("ArtifactType", "invalid-type"),
			tb.PipelineResourceSpecSecretParam("secretKey", "secretName", "GOOGLE_APPLICATION_CREDENTIALS"),
		)),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := storage.NewResource("test-resource", images, tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating BuildGCS resource")
			}
		})
	}
}

func TestNewBuildGCSResource_Valid(t *testing.T) {
	pr := tb.PipelineResource("build-gcs-resource", tb.PipelineResourceSpec(
		resourcev1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
		tb.PipelineResourceSpecParam("type", "build-gcs"),
		tb.PipelineResourceSpecParam("ArtifactType", "Manifest"),
	))
	expectedGCSResource := &storage.BuildGCSResource{
		Name:                 "test-resource",
		Location:             "gs://fake-bucket",
		Type:                 resourcev1alpha1.PipelineResourceTypeStorage,
		ArtifactType:         "Manifest",
		ShellImage:           "busybox",
		BuildGCSFetcherImage: "gcr.io/cloud-builders/gcs-fetcher:latest",
	}

	r, err := storage.NewBuildGCSResource("test-resource", images, pr)
	if err != nil {
		t.Fatalf("Unexpected error creating BuildGCS resource: %s", err)
	}
	if d := cmp.Diff(expectedGCSResource, r); d != "" {
		t.Errorf("Mismatch of BuildGCS resource: %s", diff.PrintWantGot(d))
	}
}

func TestBuildGCS_GetReplacements(t *testing.T) {
	r := &storage.BuildGCSResource{
		Name:     "gcs-resource",
		Location: "gs://fake-bucket",
		Type:     resourcev1alpha1.PipelineResourceTypeBuildGCS,
	}
	expectedReplacementMap := map[string]string{
		"name":     "gcs-resource",
		"type":     "build-gcs",
		"location": "gs://fake-bucket",
	}
	if d := cmp.Diff(r.Replacements(), expectedReplacementMap); d != "" {
		t.Errorf("BuildGCS Replacement map mismatch: %s", diff.PrintWantGot(d))
	}
}

func TestBuildGCS_GetInputSteps(t *testing.T) {
	for _, at := range []storage.GCSArtifactType{
		storage.GCSArchive,
		storage.GCSZipArchive,
		storage.GCSTarGzArchive,
		storage.GCSManifest,
	} {
		t.Run(string(at), func(t *testing.T) {
			resource := &storage.BuildGCSResource{
				Name:                 "gcs-valid",
				Location:             "gs://some-bucket",
				ArtifactType:         at,
				ShellImage:           "busybox",
				BuildGCSFetcherImage: "gcr.io/cloud-builders/gcs-fetcher:latest",
			}
			wantSteps := []v1beta1.Step{{Container: corev1.Container{
				Name:    "create-dir-gcs-valid-9l9zj",
				Image:   "busybox",
				Command: []string{"mkdir", "-p", "/workspace"},
			}}, {Container: corev1.Container{
				Name:    "storage-fetch-gcs-valid-mz4c7",
				Image:   "gcr.io/cloud-builders/gcs-fetcher:latest",
				Args:    []string{"--type", string(at), "--location", "gs://some-bucket", "--dest_dir", "/workspace"},
				Command: []string{"/ko-app/gcs-fetcher"},
			}}}
			names.TestingSeed()

			ts := v1beta1.TaskSpec{}
			got, err := resource.GetInputTaskModifier(&ts, "/workspace")
			if err != nil {
				t.Fatalf("GetDownloadSteps: %v", err)
			}
			if d := cmp.Diff(got.GetStepsToPrepend(), wantSteps); d != "" {
				t.Errorf("Error mismatch between download steps: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestBuildGCS_InvalidArtifactType(t *testing.T) {
	pr := tb.PipelineResource("build-gcs-resource", tb.PipelineResourceSpec(
		resourcev1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("Location", "gs://fake-bucket"),
		tb.PipelineResourceSpecParam("type", "build-gcs"),
		tb.PipelineResourceSpecParam("ArtifactType", "InVaLiD"),
	))
	if _, err := storage.NewBuildGCSResource("test-resource", images, pr); err == nil {
		t.Error("NewBuildGCSResource: expected error")
	}
}
