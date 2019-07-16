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
	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestNewVolumeResource(t *testing.T) {
	for _, c := range []struct {
		desc     string
		resource *v1alpha1.PipelineResource
		want     *v1alpha1.VolumeResource
	}{{
		desc: "basic volume resource",
		resource: tb.PipelineResource("test-volume-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeVolume,
			tb.PipelineResourceSpecParam("name", "test-volume-resource"),
		)),
		want: &v1alpha1.VolumeResource{
			Name: "test-volume-resource",
			Type: v1alpha1.PipelineResourceTypeVolume,
			Size: "5Gi",
		},
	}, {
		desc: "volume resource with size",
		resource: tb.PipelineResource("test-volume-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeVolume,
			tb.PipelineResourceSpecParam("name", "test-volume-resource"),
			tb.PipelineResourceSpecParam("size", "10Gi"),
		)),
		want: &v1alpha1.VolumeResource{
			Name: "test-volume-resource",
			Type: v1alpha1.PipelineResourceTypeVolume,
			Size: "10Gi",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got, err := v1alpha1.NewVolumeResource(c.resource)
			if err != nil {
				t.Errorf("Test: %q; TestNewVolumeResource() error = %v", c.desc, err)
			}
			if d := cmp.Diff(got, c.want); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}

func Test_VolumeResource_GetDownloadContainerSpec(t *testing.T) {
	names.TestingSeed()
	testcases := []struct {
		name           string
		volumeResource *v1alpha1.VolumeResource
		wantContainers []corev1.Container
		wantErr        bool
	}{{
		name: "valid volume resource config",
		volumeResource: &v1alpha1.VolumeResource{
			Name:           "test-volume-resource",
			Type:           v1alpha1.PipelineResourceTypeVolume,
			Size:           v1alpha1.DefaultPvcSize,
			DestinationDir: "/workspace",
			SourceDir:      "/src-dir",
		},
		wantContainers: []corev1.Container{{
			Name:    "create-dir-test-volume-resource-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /workspace"},
		}, {
			Name:    "download-copy-test-volume-resource-mz4c7",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /volumeresource/src-dir/. /workspace"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource",
			}},
		}},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			gotContainers, err := tc.volumeResource.GetDownloadContainerSpec()
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}
			if d := cmp.Diff(gotContainers, tc.wantContainers); d != "" {
				t.Errorf("Error mismatch between download containers spec: %s", d)
			}
		})
	}
}

func Test_VolumeResource_GetUploadContainerSpec(t *testing.T) {
	names.TestingSeed()
	testcases := []struct {
		name           string
		volumeResource *v1alpha1.VolumeResource
		wantContainers []corev1.Container
		wantErr        bool
	}{{
		name: "valid volume resource config",
		volumeResource: &v1alpha1.VolumeResource{
			Name:           "test-volume-resource",
			Type:           v1alpha1.PipelineResourceTypeVolume,
			Size:           v1alpha1.DefaultPvcSize,
			DestinationDir: "/workspace",
			SourceDir:      "/src-dir",
		},
		wantContainers: []corev1.Container{{
			Name:    "upload-mkdir-test-volume-resource-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /volumeresource/workspace"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource",
			}},
		}, {
			Name:    "upload-copy-test-volume-resource-mz4c7",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /src-dir/. /volumeresource/workspace"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource",
			}},
		}},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			gotContainers, err := tc.volumeResource.GetUploadContainerSpec()
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error to be %t but got %v:", tc.wantErr, err)
			}
			if d := cmp.Diff(gotContainers, tc.wantContainers); d != "" {
				t.Errorf("Error mismatch between download containers spec: %s", d)
			}
		})
	}
}
