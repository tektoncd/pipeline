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
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func compareVolumeResource(t *testing.T, got, want *v1alpha1.VolumeResource) {
	t.Helper()
	if got.Name != want.Name {
		t.Errorf("Expected both to have name %s but got %s", want.Name, got.Name)
	}
	if got.Type != want.Type {
		t.Errorf("Expected both to have type %s but got %s", want.Type, got.Type)
	}
	if got.SubPath != want.SubPath {
		t.Errorf("Expected both to have SubPath %s but got %s", want.SubPath, got.SubPath)
	}
	if got.ParsedSize != want.ParsedSize {
		t.Errorf("Expected both to have ParsedSize %v but got %v", want.ParsedSize, got.ParsedSize)
	}
	if got.BashNoopImage != want.BashNoopImage {
		t.Errorf("Expected both to have BashNoopImage %v but got %v", want.BashNoopImage, got.BashNoopImage)
	}
	if got.StorageClassName != want.StorageClassName {
		t.Errorf("Expected both to have StorageClassName %v but got %v", want.StorageClassName, got.StorageClassName)
	}
}

func TestNewVolumeResource(t *testing.T) {
	size5, err := resource.ParseQuantity("5Gi")
	if err != nil {
		t.Fatalf("Failed to parse size: %v", err)
	}
	size10, err := resource.ParseQuantity("10Gi")
	if err != nil {
		t.Fatalf("Failed to parse size: %v", err)
	}
	for _, c := range []struct {
		desc      string
		resource  *v1alpha1.PipelineResource
		want      *v1alpha1.VolumeResource
		pvcExists bool
	}{{
		desc: "basic volume resource",
		resource: tb.PipelineResource("test-volume-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("name", "test-volume-resource"),
			tb.PipelineResourceSpecParam("type", "volume"),
		)),
		pvcExists: true,
		want: &v1alpha1.VolumeResource{
			Name:          "test-volume-resource",
			Type:          v1alpha1.PipelineResourceTypeStorage,
			ParsedSize:    size5,
			BashNoopImage: "override-with-bash-noop:latest",
		},
	}, {
		desc: "volume resource with size, storage class, and subpath",
		resource: tb.PipelineResource("test-volume-resource", "default", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeStorage,
			tb.PipelineResourceSpecParam("name", "test-volume-resource"),
			tb.PipelineResourceSpecParam("size", "10Gi"),
			tb.PipelineResourceSpecParam("type", "volume"),
			tb.PipelineResourceSpecParam("subpath", "greatfolder"),
			tb.PipelineResourceSpecParam("storageClassName", "myStorageClass"),
		)),
		pvcExists: true,
		want: &v1alpha1.VolumeResource{
			Name:             "test-volume-resource",
			Type:             v1alpha1.PipelineResourceTypeStorage,
			ParsedSize:       size10,
			SubPath:          "greatfolder",
			StorageClassName: "myStorageClass",
			BashNoopImage:    "override-with-bash-noop:latest",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got, err := v1alpha1.NewVolumeResource(images, c.resource)
			if err != nil {
				t.Errorf("Didn't expect error creating volume resource but got %v", err)
			}
			compareVolumeResource(t, got, c.want)
		})
	}
}

func TestNewVolumeResource_Invalid(t *testing.T) {
	resource := tb.PipelineResource("test-volume-resource", "default", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeStorage,
		tb.PipelineResourceSpecParam("name", "test-volume-resource"),
		tb.PipelineResourceSpecParam("size", "about the size of a small dog"),
		tb.PipelineResourceSpecParam("type", "volume"),
	))
	_, err := v1alpha1.NewVolumeResource(images, resource)
	if err == nil {
		t.Errorf("Epxected error when creating resource with invalid size but got none")
	}
}

func TestReplacements(t *testing.T) {
	size5, err := resource.ParseQuantity("5Gi")
	if err != nil {
		t.Fatalf("Failed to parse size: %v", err)
	}
	volume := &v1alpha1.VolumeResource{
		Name:             "test-volume-resource",
		Type:             v1alpha1.PipelineResourceTypeStorage,
		ParsedSize:       size5,
		SubPath:          "greatfolder",
		StorageClassName: "greatstorageclass",
	}
	replacements := volume.Replacements()
	expectedReplacements := map[string]string{
		"name":             "test-volume-resource",
		"type":             "storage",
		"size":             "5Gi",
		"subPath":          "greatfolder",
		"storageClassName": "greatstorageclass",
	}
	if d := cmp.Diff(expectedReplacements, replacements); d != "" {
		t.Errorf("Did not get expected replacements (-want, +got): %v", d)
	}
}

func TestApplyPVC_doesntExist(t *testing.T) {
	ownerReferences := []metav1.OwnerReference{{Name: "SomeTaskRun"}}
	name, namespace, storageClassName := "mypvc", "foospace", "mystorageclass"
	size, err := resource.ParseQuantity("7Gi")
	if err != nil {
		t.Fatalf("Unexpected error parsing size argument: %v", err)
	}
	var pvcToCreate *corev1.PersistentVolumeClaim
	create := func(pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
		pvcToCreate = pvc
		return pvc, nil
	}
	get := func(name string, options metav1.GetOptions) (*corev1.PersistentVolumeClaim, error) {
		return nil, errors.NewNotFound(corev1.Resource("persistentvolumeclaim"), name)
	}
	err = v1alpha1.ApplyPVC(name, namespace, &storageClassName, size, ownerReferences, get, create)
	if err != nil {
		t.Fatalf("Didn't expect error when creating PVC that didn't exist but got %v", err)
	}
	if pvcToCreate == nil {
		t.Fatalf("Expected create to be called with PVC to create but it wasn't")
	}
	if len(pvcToCreate.OwnerReferences) != 1 || pvcToCreate.OwnerReferences[0].Name != "SomeTaskRun" {
		t.Errorf("Expected PVC to be created with passed in owner references but they were %v", pvcToCreate.OwnerReferences)
	}
	if pvcToCreate.Name != name || pvcToCreate.Namespace != namespace {
		t.Errorf("Expected PVC to be called %s/%s but was called %s/%s", namespace, name, pvcToCreate.Namespace, pvcToCreate.Name)
	}
	if pvcToCreate.Spec.StorageClassName != nil {
		if *pvcToCreate.Spec.StorageClassName != storageClassName {
			t.Errorf("Expected PVC to be created with storage class %s but was %s", storageClassName, *pvcToCreate.Spec.StorageClassName)
		}
	} else {
		t.Errorf("Expected PVC storage class to be set but as nil")
	}
}

func TestApplyPVC_exists(t *testing.T) {
	ownerReferences := []metav1.OwnerReference{{Name: "SomeTaskRun"}}
	name, namespace := "mypvc", "foospace"
	size, err := resource.ParseQuantity("7Gi")
	if err != nil {
		t.Fatalf("Unexpected error parsing size argument: %v", err)
	}
	create := func(pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
		return nil, errors.NewAlreadyExists(corev1.Resource("persistentvolumeclaim"), "Didn't expect create to be called")
	}
	existingPVC := &corev1.PersistentVolumeClaim{}
	get := func(name string, options metav1.GetOptions) (*corev1.PersistentVolumeClaim, error) {
		return existingPVC, nil
	}
	err = v1alpha1.ApplyPVC(name, namespace, nil, size, ownerReferences, get, create)
	if err != nil {
		t.Fatalf("Didn't expect error since PVC already exists but got %v", err)
	}
}

func Test_VolumeResource_GetInputTaskModifier(t *testing.T) {
	size, err := resource.ParseQuantity(v1alpha1.DefaultPvcSize)
	if err != nil {
		t.Fatalf("Failed to parse size: %v", err)
	}
	names.TestingSeed()
	testcases := []struct {
		name           string
		volumeResource *v1alpha1.VolumeResource
		wantSteps      []v1alpha1.Step
	}{{
		name: "with path",
		volumeResource: &v1alpha1.VolumeResource{
			Name:          "test-volume-resource",
			Type:          v1alpha1.PipelineResourceTypeVolume,
			ParsedSize:    size,
			SubPath:       "/src-dir",
			BashNoopImage: "override-with-bash-noop:latest",
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-test-volume-resource-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /workspace/foobar"},
		}}, {Container: corev1.Container{
			Name:    "download-copy-test-volume-resource-mz4c7",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /volumeresource-test-volume-resource/src-dir/. /workspace/foobar"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource-test-volume-resource",
			}},
		}}},
	}, {
		name: "without path",
		volumeResource: &v1alpha1.VolumeResource{
			Name:          "test-volume-resource",
			Type:          v1alpha1.PipelineResourceTypeVolume,
			ParsedSize:    size,
			BashNoopImage: "override-with-bash-noop:latest",
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-test-volume-resource-mssqb",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /workspace/foobar"},
		}}, {Container: corev1.Container{
			Name:    "download-copy-test-volume-resource-78c5n",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /volumeresource-test-volume-resource/. /workspace/foobar"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource-test-volume-resource",
			}},
		}}},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expectedVolumes := []corev1.Volume{{
				Name: tc.volumeResource.Name,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: tc.volumeResource.Name},
				},
			}}

			tm, err := tc.volumeResource.GetInputTaskModifier(nil, "/workspace/foobar")
			if err != nil {
				t.Fatalf("Did not expect error when getting steps but got %v", err)
			}

			if d := cmp.Diff(tc.wantSteps, tm.GetStepsToPrepend()); d != "" {
				t.Errorf("Mismatch between steps to prepend (-want, +got): %s", d)
			}
			if len(tm.GetStepsToAppend()) != 0 {
				t.Errorf("Did not expect to append steps but got %v", tm.GetStepsToPrepend())
			}
			if d := cmp.Diff(expectedVolumes, tm.GetVolumes()); d != "" {
				t.Errorf("Mismatch between volumes (-want, +got): %s", d)
			}
		})
	}
}

func Test_VolumeResource_GetOutputTaskModifier(t *testing.T) {
	size, err := resource.ParseQuantity(v1alpha1.DefaultPvcSize)
	if err != nil {
		t.Fatalf("Failed to parse size: %v", err)
	}
	names.TestingSeed()
	testcases := []struct {
		name           string
		volumeResource *v1alpha1.VolumeResource
		wantSteps      []v1alpha1.Step
	}{{
		name: "with path",
		volumeResource: &v1alpha1.VolumeResource{
			Name:          "test-volume-resource",
			Type:          v1alpha1.PipelineResourceTypeVolume,
			ParsedSize:    size,
			SubPath:       "/src-dir",
			BashNoopImage: "override-with-bash-noop:latest",
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-test-volume-resource-9l9zj",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /volumeresource-test-volume-resource/src-dir"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource-test-volume-resource",
			}},
		}}, {Container: corev1.Container{
			Name:    "upload-copy-test-volume-resource-mz4c7",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /workspace/output/foobar/. /volumeresource-test-volume-resource/src-dir"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource-test-volume-resource",
			}},
		}}},
	}, {
		name: "without path",
		volumeResource: &v1alpha1.VolumeResource{
			Name:          "test-volume-resource",
			Type:          v1alpha1.PipelineResourceTypeVolume,
			ParsedSize:    size,
			BashNoopImage: "override-with-bash-noop:latest",
		},
		wantSteps: []v1alpha1.Step{{Container: corev1.Container{
			Name:    "create-dir-test-volume-resource-mssqb",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "mkdir -p /volumeresource-test-volume-resource"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource-test-volume-resource",
			}},
		}}, {Container: corev1.Container{
			Name:    "upload-copy-test-volume-resource-78c5n",
			Image:   "override-with-bash-noop:latest",
			Command: []string{"/ko-app/bash"},
			Args:    []string{"-args", "cp -r /workspace/output/foobar/. /volumeresource-test-volume-resource"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "test-volume-resource",
				MountPath: "/volumeresource-test-volume-resource",
			}},
		}}},
	}}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expectedVolumes := []corev1.Volume{{
				Name: tc.volumeResource.Name,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: tc.volumeResource.Name},
				},
			}}

			tm, err := tc.volumeResource.GetOutputTaskModifier(nil, "/workspace/output/foobar")
			if err != nil {
				t.Fatalf("Did not expect error when getting steps but got %v", err)
			}

			if len(tm.GetStepsToPrepend()) != 0 {
				t.Errorf("Did not expect to prepend steps but got %v", tm.GetStepsToPrepend())
			}
			if d := cmp.Diff(tc.wantSteps, tm.GetStepsToAppend()); d != "" {
				t.Errorf("Mismatch between steps to append (-want, +got): %s", d)
			}
			if d := cmp.Diff(expectedVolumes, tm.GetVolumes()); d != "" {
				t.Errorf("Mismatch between volumes (-want, +got): %s", d)
			}
		})
	}
}
