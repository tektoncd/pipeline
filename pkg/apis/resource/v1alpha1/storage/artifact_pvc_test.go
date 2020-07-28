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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/storage"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPVCGetCopyFromContainerSpec(t *testing.T) {
	names.TestingSeed()

	pvc := storage.ArtifactPVC{
		Name:       "pipelinerun-pvc",
		ShellImage: "busybox",
	}
	want := []v1beta1.Step{{Container: corev1.Container{
		Name:  "source-copy-workspace-9l9zj",
		Image: "busybox",

		Command: []string{"cp", "-r", "src-path/.", "/workspace/destination"},
	}}}

	got := pvc.GetCopyFromStorageToSteps("workspace", "src-path", "/workspace/destination")
	diff.ErrorWantGot(t, got, want, "Diff:\n%s")
}

func TestPVCGetCopyToContainerSpec(t *testing.T) {
	names.TestingSeed()

	pvc := storage.ArtifactPVC{
		Name:       "pipelinerun-pvc",
		ShellImage: "busybox",
	}
	want := []v1beta1.Step{{Container: corev1.Container{
		Name:         "source-mkdir-workspace-9l9zj",
		Image:        "busybox",
		Command:      []string{"mkdir", "-p", "/workspace/destination"},
		VolumeMounts: []corev1.VolumeMount{{MountPath: "/pvc", Name: "pipelinerun-pvc"}},
	}}, {Container: corev1.Container{
		Name:         "source-copy-workspace-mz4c7",
		Image:        "busybox",
		Command:      []string{"cp", "-r", "src-path/.", "/workspace/destination"},
		VolumeMounts: []corev1.VolumeMount{{MountPath: "/pvc", Name: "pipelinerun-pvc"}},
	}}}

	got := pvc.GetCopyToStorageFromSteps("workspace", "src-path", "/workspace/destination")
	diff.ErrorWantGot(t, got, want, "Diff:\n%s")
}

func TestPVCGetPvcMount(t *testing.T) {
	names.TestingSeed()
	name := "pipelinerun-pvc"
	pvcDir := "/pvc"

	want := corev1.VolumeMount{
		Name:      name,
		MountPath: pvcDir,
	}
	got := storage.GetPvcMount(name)
	diff.ErrorWantGot(t, got, want, "Diff:\n%s")
}

func TestPVCGetMakeStep(t *testing.T) {
	names.TestingSeed()

	want := v1beta1.Step{Container: corev1.Container{
		Name:    "create-dir-workspace-9l9zj",
		Image:   "busybox",
		Command: []string{"mkdir", "-p", "/workspace/destination"},
	}}
	got := storage.CreateDirStep("busybox", "workspace", "/workspace/destination")
	diff.ErrorWantGot(t, got, want, "Diff:\n%s")
}

func TestStorageBasePath(t *testing.T) {
	pvc := storage.ArtifactPVC{
		Name: "pipelinerun-pvc",
	}
	pipelinerun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "pipelineruntest",
		},
	}
	got := pvc.StorageBasePath(pipelinerun)
	diff.ErrorWantGot(t, got, "/pvc", "Diff:\n%s")
}
