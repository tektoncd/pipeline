/*
Copyright 2019-2020 The Tekton Authors.

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

package image_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"

	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/image"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewImageResource_Invalid(t *testing.T) {
	r := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
		},
	}

	_, err := image.NewResource("test-resource", r)
	if err == nil {
		t.Error("Expected error creating Image resource")
	}
}

func TestNewImageResource_Valid(t *testing.T) {
	want := &image.Resource{
		Name:   "image-resource",
		Type:   v1alpha1.PipelineResourceTypeImage,
		URL:    "https://test.com/test/test",
		Digest: "test",
	}

	r := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-resource",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeImage,
			Params: []v1alpha1.ResourceParam{
				{
					Name:  "URL",
					Value: "https://test.com/test/test",
				},
				{
					Name:  "Digest",
					Value: "test",
				},
			},
		},
	}

	got, err := image.NewResource("image-resource", r)
	if err != nil {
		t.Fatalf("Unexpected error creating Image resource: %s", err)
	}

	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Mismatch of Image resource: %s", diff.PrintWantGot(d))
	}
}

func TestImageResource_Replacements(t *testing.T) {
	ir := &image.Resource{
		Name:   "image-resource",
		Type:   v1alpha1.PipelineResourceTypeImage,
		URL:    "https://test.com/test/test",
		Digest: "test",
	}

	want := map[string]string{
		"name":   "image-resource",
		"type":   v1alpha1.PipelineResourceTypeImage,
		"url":    "https://test.com/test/test",
		"digest": "test",
	}

	got := ir.Replacements()

	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Mismatch of ImageResource Replacements %s", diff.PrintWantGot(d))
	}
}
