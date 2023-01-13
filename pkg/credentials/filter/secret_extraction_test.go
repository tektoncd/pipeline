package filter_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/credentials/filter"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
)

func TestApplySecretLocationsToPodWorks(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "container1",
				},
				{
					Name: "container2",
					Env: []corev1.EnvVar{
						{
							Name: "ENV_VAR",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "somesecret",
									},
									Key: "somekey",
								},
							},
						},
					},
				},
			},
		},
	}

	err := filter.ApplySecretLocationsToPod(pod)
	if err != nil {
		t.Fatal(err)
	}

	annotation1 := pod.ObjectMeta.Annotations[filter.SecretLocationsAnnotationPrefix+"container1"]
	expectedAnnotation1 := "{\"environmentVariables\":[],\"files\":[]}"
	if d := cmp.Diff(expectedAnnotation1, annotation1); d != "" {
		t.Errorf("Expected annotation1, diff: %s", diff.PrintWantGot(d))
	}

	annotation2 := pod.ObjectMeta.Annotations[filter.SecretLocationsAnnotationPrefix+"container2"]
	expectedAnnotation2 := "{\"environmentVariables\":[\"ENV_VAR\"],\"files\":[]}"
	if d := cmp.Diff(expectedAnnotation2, annotation2); d != "" {
		t.Errorf("Expected annotation2, diff: %s", diff.PrintWantGot(d))
	}
}

func TestSecretExtractionFromContainerWorks(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Env: []corev1.EnvVar{
						{
							Name: "SECRET_ENV",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "somesecret",
									},
									Key: "somekey",
								},
							}},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "secret-volume-with-items",
							MountPath: "/mount/at",
						},
						{
							Name:      "secret-volume-without-items",
							MountPath: "/mount/at2",
						},
						{
							Name:      "secret-volume-csi",
							MountPath: "/mount/at3",
						},
						{
							Name:      "secret-volume-projected",
							MountPath: "/mount/at4",
						},
						{
							Name:      "secret-volume-projected-items",
							MountPath: "/mount/at5",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "secret-volume-with-items",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "somesecret2",
							Items: []corev1.KeyToPath{
								{Key: "key1", Path: "some/path/to/file"},
								{Key: "key2"},
							},
						},
					},
				},
				{
					Name: "secret-volume-without-items",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "somesecret2",
						},
					},
				},
				{
					Name: "secret-volume-csi",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver: "secrets-store.csi.k8s.io",
						},
					},
				},
				{
					Name: "secret-volume-not-mounted",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "somesecret3",
						},
					},
				},
				{
					Name: "secret-volume-projected",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "somesecret4"},
									},
								},
							},
						},
					},
				},
				{
					Name: "secret-volume-projected-items",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "somesecret5"},
										Items: []corev1.KeyToPath{
											{Key: "key3", Path: "some/projected/path/to/file"},
											{Key: "key4"},
										},
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "somesecret6"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	secretLocations, err := filter.ExtractSecretLocationsFromPodContainer(pod, &pod.Spec.Containers[0])
	if err != nil {
		t.Error(err)
	}

	if secretLocations.EnvironmentVariables[0] != "SECRET_ENV" {
		t.Errorf("Expected env var SECRET_ENV as only env secret location")
	}

	if secretLocations.Files[0] != "/mount/at/some/path/to/file" {
		t.Errorf("Expected file /mount/at/some/path/to/file as first file secret location")
	}

	if secretLocations.Files[1] != "/mount/at/key2" {
		t.Errorf("Expected file /mount/at/key2 as second file secret location")
	}

	if secretLocations.Files[2] != "/mount/at2" {
		t.Errorf("Expected folder /mount/at2 as third file secret location")
	}

	if secretLocations.Files[3] != "/mount/at3" {
		t.Errorf("Expected folder /mount/at3 as 4th file secret location")
	}

	if secretLocations.Files[4] != "/mount/at4" {
		t.Errorf("Expected folder /mount/at4 as 5th file secret location")
	}

	if secretLocations.Files[5] != "/mount/at5/some/projected/path/to/file" {
		t.Errorf("Expected folder /mount/at5/some/projected/path/to/file as 6th file secret location")
	}

	if secretLocations.Files[6] != "/mount/at5/key4" {
		t.Errorf("Expected folder /mount/at5/key4 as 7th file secret location")
	}

	if secretLocations.Files[7] != "/mount/at5" {
		t.Errorf("Expected folder /mount/at5 as 8th file secret location")
	}
}
