/*
Copyright 2018 The Knative Authors

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

package convert

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/buildtest"
)

var ignorePrivateResourceFields = cmpopts.IgnoreUnexported(resource.Quantity{})

var nopContainer = corev1.Container{
	Name:  "nop",
	Image: *nopImage,
}

func read2CRD(f string) (*v1alpha1.Build, error) {
	var bs v1alpha1.Build
	if err := buildtest.DataAs(f, &bs.Spec); err != nil {
		return nil, err
	}
	return &bs, nil
}

func TestRoundtrip(t *testing.T) {
	inputs := []string{
		"testdata/helloworld.yaml",
		"testdata/two-step.yaml",
		"testdata/env.yaml",
		"testdata/env-valuefrom.yaml",
		"testdata/workingdir.yaml",
		"testdata/workspace.yaml",
		"testdata/resources.yaml",
		"testdata/security-context.yaml",
		"testdata/volumes.yaml",
		"testdata/custom-source.yaml",
		"testdata/nodeselector.yaml",

		"testdata/git-revision.yaml",
		"testdata/git-subpath.yaml",
		"testdata/gcs-archive.yaml",
		"testdata/gcs-manifest.yaml",
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			og, err := read2CRD(in)
			if err != nil {
				t.Fatalf("Unexpected error in read2CRD(%q): %v", in, err)
			}
			cs := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
				Secrets: []corev1.ObjectReference{{
					Name: "multi-creds",
				}},
			}, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-creds",
					Annotations: map[string]string{"build.dev/docker-0": "https://us.gcr.io",
						"build.dev/docker-1": "https://docker.io",
						"build.dev/git-0":    "github.com",
						"build.dev/git-1":    "gitlab.com",
					}},
				Type: "kubernetes.io/basic-auth",
				Data: map[string][]byte{
					"username": []byte("foo"),
					"password": []byte("BestEver"),
				},
			})
			p, err := FromCRD(og, cs)
			if err != nil {
				t.Fatalf("Unable to convert %q from CRD: %v", in, err)
			}

			// Verify that secrets are loaded correctly.
			if p.Spec.ServiceAccountName != "" {
				for _, vol := range p.Spec.Volumes {
					if vol.Name == "secret-volume-multi-creds" {
						if vol.Secret.SecretName != "multi-creds" {
							t.Errorf("Expected multi-creds to be mounted in Pod %v", p.Spec)
						}
					}
				}
				expected := map[string]int{"https://us.gcr.io": 1, "https://docker.io": 1, "github.com": 1, "gitlab.com": 1}
				for _, a := range p.Spec.InitContainers[0].Args {
					expected[a] -= 1
				}
				for k, c := range expected {
					if c > 0 {
						t.Errorf("Expected arg related to %s in args, got %v", k, p.Spec.InitContainers[0].Args)
					}
				}
			}

			// Verify that volumeMounts are mounted at a unique path.
			for i, s := range p.Spec.InitContainers {
				seen := map[string]struct{}{}
				for _, vm := range s.VolumeMounts {
					if _, found := seen[vm.MountPath]; found {
						t.Errorf("Step %d had duplicate volumeMount path %q", i, vm.MountPath)
					}
					seen[vm.MountPath] = struct{}{}
				}
			}

			// Verify that reverse transformation works.
			b, err := ToCRD(p)
			if err != nil {
				t.Fatalf("Unable to convert %q to CRD: %v", in, err)
			}

			if d := cmp.Diff(og, b, ignorePrivateResourceFields); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}

func TestFromCRD(t *testing.T) {
	subPath := "subpath"
	implicitVolumeMountsWithSubPath := []corev1.VolumeMount{}
	for _, vm := range implicitVolumeMounts {
		if vm.Name == "workspace" {
			implicitVolumeMountsWithSubPath = append(implicitVolumeMountsWithSubPath, corev1.VolumeMount{
				Name:      vm.Name,
				MountPath: vm.MountPath,
				SubPath:   subPath,
			})
		} else {
			implicitVolumeMountsWithSubPath = append(implicitVolumeMountsWithSubPath, vm)
		}
	}

	for _, c := range []struct {
		desc    string
		b       v1alpha1.BuildSpec
		want    *corev1.PodSpec
		wantErr error
	}{{
		desc: "simple",
		b: v1alpha1.BuildSpec{
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         initContainerPrefix + credsInit,
				Image:        *credsImage,
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}, {
				Name:         "build-step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{nopContainer},
			Volumes:    implicitVolumes,
		},
	}, {
		desc: "source",
		b: v1alpha1.BuildSpec{
			Source: &v1alpha1.SourceSpec{
				Git: &v1alpha1.GitSourceSpec{
					Url:      "github.com/my/repo",
					Revision: "master",
				},
			},
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         initContainerPrefix + credsInit,
				Image:        *credsImage,
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}, {
				Name:         initContainerPrefix + gitSource,
				Image:        *gitImage,
				Args:         []string{"-url", "github.com/my/repo", "-revision", "master"},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}, {
				Name:         "build-step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{nopContainer},
			Volumes:    implicitVolumes,
		},
	}, {
		desc: "git-source-with-subpath",
		b: v1alpha1.BuildSpec{
			Source: &v1alpha1.SourceSpec{
				Git: &v1alpha1.GitSourceSpec{
					Url:      "github.com/my/repo",
					Revision: "master",
				},
				SubPath: subPath,
			},
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         initContainerPrefix + credsInit,
				Image:        *credsImage,
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts, // without subpath
				WorkingDir:   workspaceDir,
			}, {
				Name:         initContainerPrefix + gitSource,
				Image:        *gitImage,
				Args:         []string{"-url", "github.com/my/repo", "-revision", "master"},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts, // without subpath
				WorkingDir:   workspaceDir,
			}, {
				Name:         "build-step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMountsWithSubPath,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{nopContainer},
			Volumes:    implicitVolumes,
		},
	}, {
		desc: "gcs-source-with-subpath",
		b: v1alpha1.BuildSpec{
			Source: &v1alpha1.SourceSpec{
				GCS: &v1alpha1.GCSSourceSpec{
					Type:     v1alpha1.GCSManifest,
					Location: "gs://foo/bar",
				},
				SubPath: subPath,
			},
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         initContainerPrefix + credsInit,
				Image:        *credsImage,
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts, // without subpath
				WorkingDir:   workspaceDir,
			}, {
				Name:         initContainerPrefix + gcsSource,
				Image:        *gcsFetcherImage,
				Args:         []string{"--type", "Manifest", "--location", "gs://foo/bar"},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts, // without subpath
				WorkingDir:   workspaceDir,
			}, {
				Name:         "build-step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMountsWithSubPath,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{nopContainer},
			Volumes:    implicitVolumes,
		},
	}, {
		desc: "custom-source-with-subpath",
		b: v1alpha1.BuildSpec{
			Source: &v1alpha1.SourceSpec{
				Custom: &corev1.Container{
					Image: "image",
				},
				SubPath: subPath,
			},
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         initContainerPrefix + credsInit,
				Image:        *credsImage,
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts, // without subpath
				WorkingDir:   workspaceDir,
			}, {
				Name:         initContainerPrefix + customSource,
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMountsWithSubPath, // *with* subpath
				WorkingDir:   workspaceDir,
			}, {
				Name:         "build-step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMountsWithSubPath,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{nopContainer},
			Volumes:    implicitVolumes,
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			cs := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
			})
			got, err := FromCRD(&v1alpha1.Build{Spec: c.b}, cs)
			if err != c.wantErr {
				t.Fatalf("FromCRD: %v", err)
			}

			if d := cmp.Diff(&got.Spec, c.want, ignorePrivateResourceFields); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}
