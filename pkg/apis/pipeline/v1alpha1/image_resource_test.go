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
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestNewImageResource_Invalid(t *testing.T) {
	r := tb.PipelineResource("git-resource", "default", tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGit))

	_, err := v1alpha1.NewImageResource(r)
	if err == nil {
		t.Error("Expected error creating Image resource")
	}
}

func TestNewImageResource_Valid(t *testing.T) {
	want := &v1alpha1.ImageResource{
		Name:   "image-resource",
		Type:   v1alpha1.PipelineResourceTypeImage,
		URL:    "https://test.com/test/test",
		Digest: "test",
	}

	r := tb.PipelineResource(
		"image-resource",
		"default",
		tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeImage,
			tb.PipelineResourceSpecParam("URL", "https://test.com/test/test"),
			tb.PipelineResourceSpecParam("Digest", "test"),
		),
	)

	got, err := v1alpha1.NewImageResource(r)
	if err != nil {
		t.Fatalf("Unexpected error creating Image resource: %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Mismatch of Image resource: %s", diff)
	}
}

func TestImageResource_Replacements(t *testing.T) {
	ir := &v1alpha1.ImageResource{
		Name:   "image-resource",
		Type:   v1alpha1.PipelineResourceTypeImage,
		URL:    "https://test.com/test/test",
		Digest: "test",
	}

	want := map[string]string{
		"name":   "image-resource",
		"type":   string(v1alpha1.PipelineResourceTypeImage),
		"url":    "https://test.com/test/test",
		"digest": "test",
	}

	got := ir.Replacements()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Mismatch of ImageResource Replacements: %s", diff)
	}
}
