/*
Copyright 2018 The Knative Authors.

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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestBucketGetCopyFromContainerSpec(t *testing.T) {
	names.TestingSeed()
	bucket := ArtifactBucket{
		Location: "gs://fake-bucket",
		Secrets: []SecretParam{{
			FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			SecretName: "secret1",
			SecretKey:  "serviceaccount",
		}},
	}
	want := []corev1.Container{{
		Name:  "artifact-dest-mkdir-workspace-9l9zj",
		Image: "override-with-bash-noop:latest",
		Args:  []string{"-args", "mkdir -p /workspace/destination"},
	}, {
		Name:         "artifact-copy-from-workspace-mz4c7",
		Image:        "override-with-gsutil-image:latest",
		Args:         []string{"-args", "cp -r gs://fake-bucket/src-path/** /workspace/destination"},
		Env:          []corev1.EnvVar{{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/bucketsecret/secret1/serviceaccount"}},
		VolumeMounts: []corev1.VolumeMount{{Name: "volume-bucket-secret1", MountPath: "/var/bucketsecret/secret1"}},
	}}

	got := bucket.GetCopyFromContainerSpec("workspace", "src-path", "/workspace/destination")
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", d)
	}
}

func TestBucketGetCopyToContainerSpec(t *testing.T) {
	names.TestingSeed()
	bucket := ArtifactBucket{
		Location: "gs://fake-bucket",
		Secrets: []SecretParam{{
			FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			SecretName: "secret1",
			SecretKey:  "serviceaccount",
		}},
	}
	want := []corev1.Container{{
		Name:         "artifact-copy-to-workspace-9l9zj",
		Image:        "override-with-gsutil-image:latest",
		Args:         []string{"-args", "cp -r src-path gs://fake-bucket/workspace/destination"},
		Env:          []corev1.EnvVar{{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/bucketsecret/secret1/serviceaccount"}},
		VolumeMounts: []corev1.VolumeMount{{Name: "volume-bucket-secret1", MountPath: "/var/bucketsecret/secret1"}},
	}}

	got := bucket.GetCopyToContainerSpec("workspace", "src-path", "workspace/destination")
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", d)
	}
}

func TestGetSecretsVolumes(t *testing.T) {
	names.TestingSeed()
	bucket := ArtifactBucket{
		Location: "gs://fake-bucket",
		Secrets: []SecretParam{{
			FieldName:  "GOOGLE_APPLICATION_CREDENTIALS",
			SecretName: "secret1",
			SecretKey:  "serviceaccount",
		}},
	}
	want := []corev1.Volume{{
		Name: "bucket-secret-secret1-9l9zj",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "secret1",
			},
		},
	}}
	got := bucket.GetSecretsVolumes()
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff:\n%s", d)
	}
}
