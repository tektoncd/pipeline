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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/storage"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestPVCGetCopyFromContainerSpec(t *testing.T) {
	names.TestingSeed()

	pvc := storage.ArtifactPVC{
		Name:       "pipelinerun-pvc",
		ShellImage: "busybox",
	}
	want := []v1beta1.Step{{
		Name:  "source-copy-workspace-9l9zj",
		Image: "busybox",

		Command: []string{"cp", "-r", "src-path/.", "/workspace/destination"},
		Env:     []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "workspace"}},
	}}

	got := pvc.GetCopyFromStorageToSteps("workspace", "src-path", "/workspace/destination")
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
	}
}

func TestPVCGetCopyToContainerSpec(t *testing.T) {
	names.TestingSeed()

	pvc := storage.ArtifactPVC{
		Name:       "pipelinerun-pvc",
		ShellImage: "busybox",
	}
	want := []v1beta1.Step{{
		Name:  "source-mkdir-workspace-9l9zj",
		Image: "busybox",
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: ptr.Int64(0),
		},
		Command:      []string{"mkdir", "-p", "/workspace/destination"},
		VolumeMounts: []corev1.VolumeMount{{MountPath: "/pvc", Name: "pipelinerun-pvc"}},
	}, {
		Name:  "source-copy-workspace-mz4c7",
		Image: "busybox",
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: ptr.Int64(0),
		},
		Command:      []string{"cp", "-r", "src-path/.", "/workspace/destination"},
		VolumeMounts: []corev1.VolumeMount{{MountPath: "/pvc", Name: "pipelinerun-pvc"}},
		Env:          []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "workspace"}},
	}}

	got := pvc.GetCopyToStorageFromSteps("workspace", "src-path", "/workspace/destination")
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
	}
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
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
	}
}

func TestPVCGetMakeStep(t *testing.T) {
	names.TestingSeed()

	want := v1beta1.Step{
		Name:    "create-dir-workspace-9l9zj",
		Image:   "busybox",
		Command: []string{"mkdir", "-p", "/workspace/destination"},
	}
	got := storage.CreateDirStep("busybox", "workspace", "/workspace/destination")
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
	}
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
	if d := cmp.Diff(got, "/pvc"); d != "" {
		t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
	}
}
