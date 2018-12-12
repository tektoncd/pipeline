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

package resources

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/system"
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

var (
	ignorePrivateResourceFields = cmpopts.IgnoreUnexported(resource.Quantity{})
	ignoreVolatileTime          = cmp.Comparer(func(_, _ apis.VolatileTime) bool { return true })
	ignoreVolatileTimePtr       = cmp.Comparer(func(_, _ *apis.VolatileTime) bool { return true })
	nopContainer                = corev1.Container{
		Name:  "nop",
		Image: *nopImage,
	}
)

func TestMakePod(t *testing.T) {
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

	implicitVolumeMountsWithSecrets := append(implicitVolumeMounts, corev1.VolumeMount{
		Name:      "secret-volume-multi-creds",
		MountPath: "/var/build-secrets/multi-creds",
	})
	implicitVolumesWithSecrets := append(implicitVolumes, corev1.Volume{
		Name:         "secret-volume-multi-creds",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "multi-creds"}},
	})

	randReader = strings.NewReader(strings.Repeat("a", 10000))
	defer func() { randReader = rand.Reader }()

	for _, c := range []struct {
		desc         string
		b            v1alpha1.BuildSpec
		bAnnotations map[string]string
		want         *corev1.PodSpec
		wantErr      error
	}{{
		desc: "simple",
		b: v1alpha1.BuildSpec{
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		bAnnotations: map[string]string{
			"simple-annotation-key": "simple-annotation-val",
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
				Name:         initContainerPrefix + gitSource + "-0",
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
		desc: "sources",
		b: v1alpha1.BuildSpec{
			Sources: []v1alpha1.SourceSpec{{
				Git: &v1alpha1.GitSourceSpec{
					Url:      "github.com/my/repo",
					Revision: "master",
				},
				Name: "repo1",
			}, {
				Git: &v1alpha1.GitSourceSpec{
					Url:      "github.com/my/repo",
					Revision: "master",
				},
				Name: "repo2",
			}},
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
				Name:         initContainerPrefix + gitSource + "-" + "repo1",
				Image:        *gitImage,
				Args:         []string{"-url", "github.com/my/repo", "-revision", "master"},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}, {
				Name:         initContainerPrefix + gitSource + "-" + "repo2",
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
				Name:         initContainerPrefix + gitSource + "-0",
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
		desc: "git-sources-with-subpath",
		b: v1alpha1.BuildSpec{
			Sources: []v1alpha1.SourceSpec{{
				Name: "myrepo",
				Git: &v1alpha1.GitSourceSpec{
					Url:      "github.com/my/repo",
					Revision: "master",
				},
				SubPath: subPath,
			}, {
				Name: "ownrepo",
				Git: &v1alpha1.GitSourceSpec{
					Url:      "github.com/own/repo",
					Revision: "master",
				},
				SubPath: subPath,
			}},
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
				Name:         initContainerPrefix + gitSource + "-" + "myrepo",
				Image:        *gitImage,
				Args:         []string{"-url", "github.com/my/repo", "-revision", "master"},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts, // without subpath
				WorkingDir:   workspaceDir,
			}, {
				Name:         initContainerPrefix + gitSource + "-" + "ownrepo",
				Image:        *gitImage,
				Args:         []string{"-url", "github.com/own/repo", "-revision", "master"},
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
				Name:         initContainerPrefix + gcsSource + "-0",
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
		desc: "gcs-source-with-targetPath",
		b: v1alpha1.BuildSpec{
			Source: &v1alpha1.SourceSpec{
				GCS: &v1alpha1.GCSSourceSpec{
					Type:     v1alpha1.GCSManifest,
					Location: "gs://foo/bar",
				},
				TargetPath: "path/foo",
			},
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
				Name:         initContainerPrefix + gcsSource + "-0",
				Image:        *gcsFetcherImage,
				Args:         []string{"--type", "Manifest", "--location", "gs://foo/bar", "--dest_dir", "/workspace/path/foo"},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts, // without subpath
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
	}, {
		desc: "with-service-account",
		b: v1alpha1.BuildSpec{
			ServiceAccountName: "service-account",
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		want: &corev1.PodSpec{
			ServiceAccountName: "service-account",
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:  initContainerPrefix + credsInit,
				Image: *credsImage,
				Args: []string{
					"-basic-docker=multi-creds=https://docker.io",
					"-basic-docker=multi-creds=https://us.gcr.io",
					"-basic-git=multi-creds=github.com",
					"-basic-git=multi-creds=gitlab.com",
				},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMountsWithSecrets,
				WorkingDir:   workspaceDir,
			}, {
				Name:         "build-step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{nopContainer},
			Volumes:    implicitVolumesWithSecrets,
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			cs := fakek8s.NewSimpleClientset(
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "service-account"},
					Secrets: []corev1.ObjectReference{{
						Name: "multi-creds",
					}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "multi-creds",
						Annotations: map[string]string{
							"build.knative.dev/docker-0": "https://us.gcr.io",
							"build.knative.dev/docker-1": "https://docker.io",
							"build.knative.dev/git-0":    "github.com",
							"build.knative.dev/git-1":    "gitlab.com",
						}},
					Type: "kubernetes.io/basic-auth",
					Data: map[string][]byte{
						"username": []byte("foo"),
						"password": []byte("BestEver"),
					},
				},
			)
			b := &v1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "build-name",
					Annotations: c.bAnnotations,
				},
				Spec: c.b,
			}
			got, err := MakePod(b, cs)
			if err != c.wantErr {
				t.Fatalf("MakePod: %v", err)
			}

			// Generated name from hexlifying a stream of 'a's.
			wantName := "build-name-pod-616161"
			if got.Name != wantName {
				t.Errorf("Pod name got %q, want %q", got.Name, wantName)
			}

			if d := cmp.Diff(&got.Spec, c.want, ignorePrivateResourceFields); d != "" {
				t.Errorf("Diff spec:\n%s", d)
			}

			wantAnnotations := map[string]string{"sidecar.istio.io/inject": "false"}
			if c.bAnnotations != nil {
				for key, val := range c.bAnnotations {
					wantAnnotations[key] = val
				}
			}
			if d := cmp.Diff(got.Annotations, wantAnnotations); d != "" {
				t.Errorf("Diff annotations:\n%s", d)
			}
		})
	}
}

func TestBuildStatusFromPod(t *testing.T) {
	for _, c := range []struct {
		desc      string
		podStatus corev1.PodStatus
		buildSpec v1alpha1.BuildSpec
		want      v1alpha1.BuildStatus
	}{{
		desc:      "empty",
		podStatus: corev1.PodStatus{},
		buildSpec: v1alpha1.BuildSpec{},
		want:      v1alpha1.BuildStatus{},
	}, {
		desc: "ignore-creds-init",
		podStatus: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init; ignored
			}, {
				Name: "state-name",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		buildSpec: v1alpha1.BuildSpec{
			// no sources.
		},
		want: v1alpha1.BuildStatus{
			StepsCompleted: []string{"state-name"},
			StepStates: []corev1.ContainerState{{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 123,
				},
			}},
		},
	}, {
		desc: "ignore-creds-init-and-source",
		podStatus: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init; ignored.
			}, {
				// git-init; ignored.
			}, {
				Name: "state-name",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		buildSpec: v1alpha1.BuildSpec{
			Source: &v1alpha1.SourceSpec{
				Git: &v1alpha1.GitSourceSpec{
					Url:      "example.com",
					Revision: "master",
				},
			},
		},
		want: v1alpha1.BuildStatus{
			StepsCompleted: []string{"state-name"},
			StepStates: []corev1.ContainerState{{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 123,
				},
			}},
		},
	}, {
		desc: "ignore-creds-init-and-multiple-sources",
		podStatus: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init; ignored.
			}, {
				// first git-init; ignored.
			}, {
				// second git-init; ignored.
			}, {
				Name: "state-name",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		buildSpec: v1alpha1.BuildSpec{
			Sources: []v1alpha1.SourceSpec{{
				Git: &v1alpha1.GitSourceSpec{
					Url:      "example.com",
					Revision: "master",
				},
			}, {
				Git: &v1alpha1.GitSourceSpec{
					Url:      "yahoo.com",
					Revision: "so cool",
				},
			}},
		},
		want: v1alpha1.BuildStatus{
			StepsCompleted: []string{"state-name"},
			StepStates: []corev1.ContainerState{{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 123,
				},
			}},
		},
	}, {
		desc:      "success",
		podStatus: corev1.PodStatus{Phase: corev1.PodSucceeded},
		want: v1alpha1.BuildStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   v1alpha1.BuildSucceeded,
				Status: corev1.ConditionTrue,
			}},
		},
	}, {
		desc:      "running",
		podStatus: corev1.PodStatus{Phase: corev1.PodRunning},
		want: v1alpha1.BuildStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   v1alpha1.BuildSucceeded,
				Status: corev1.ConditionUnknown,
				Reason: "Building",
			}},
		},
	}, {
		desc: "failure-terminated",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodFailed,
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init status; ignored
			}, {
				Name:    "status-name",
				ImageID: "image-id",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 123,
					},
				},
			}},
		},
		want: v1alpha1.BuildStatus{
			StepsCompleted: []string{"status-name"},
			StepStates: []corev1.ContainerState{{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 123,
				},
			}},
			Conditions: []duckv1alpha1.Condition{{
				Type:    v1alpha1.BuildSucceeded,
				Status:  corev1.ConditionFalse,
				Message: `build step "status-name" exited with code 123 (image: "image-id"); for logs run: kubectl -n knative-build logs pod -c status-name`,
			}},
		},
	}, {
		desc: "failure-message",
		podStatus: corev1.PodStatus{
			Phase:   corev1.PodFailed,
			Message: "boom",
		},
		want: v1alpha1.BuildStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:    v1alpha1.BuildSucceeded,
				Status:  corev1.ConditionFalse,
				Message: "boom",
			}},
		},
	}, {
		desc:      "failure-unspecified",
		podStatus: corev1.PodStatus{Phase: corev1.PodFailed},
		want: v1alpha1.BuildStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:    v1alpha1.BuildSucceeded,
				Status:  corev1.ConditionFalse,
				Message: "build failed for unspecified reasons.",
			}},
		},
	}, {
		desc: "pending-waiting-message",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{{
				// creds-init status; ignored
			}, {
				Name: "status-name",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Message: "i'm pending",
					},
				},
			}},
		},
		want: v1alpha1.BuildStatus{
			StepStates: []corev1.ContainerState{{
				Waiting: &corev1.ContainerStateWaiting{
					Message: "i'm pending",
				},
			}},
			Conditions: []duckv1alpha1.Condition{{
				Type:    v1alpha1.BuildSucceeded,
				Status:  corev1.ConditionUnknown,
				Reason:  "Pending",
				Message: `build step "status-name" is pending with reason "i'm pending"`,
			}},
		},
	}, {
		desc: "pending-pod-condition",
		podStatus: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Status:  corev1.ConditionUnknown,
				Type:    "the type",
				Message: "the message",
			}},
		},
		want: v1alpha1.BuildStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:    v1alpha1.BuildSucceeded,
				Status:  corev1.ConditionUnknown,
				Reason:  "Pending",
				Message: `pod status "the type":"Unknown"; message: "the message"`,
			}},
		},
	}, {
		desc: "pending-message",
		podStatus: corev1.PodStatus{
			Phase:   corev1.PodPending,
			Message: "pod status message",
		},
		want: v1alpha1.BuildStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:    v1alpha1.BuildSucceeded,
				Status:  corev1.ConditionUnknown,
				Reason:  "Pending",
				Message: "pod status message",
			}},
		},
	}, {
		desc:      "pending-no-message",
		podStatus: corev1.PodStatus{Phase: corev1.PodPending},
		want: v1alpha1.BuildStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:    v1alpha1.BuildSucceeded,
				Status:  corev1.ConditionUnknown,
				Reason:  "Pending",
				Message: "Pending",
			}},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			now := metav1.Now()
			p := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod",
					Namespace:         system.Namespace,
					CreationTimestamp: now,
				},
				Status: c.podStatus,
			}
			got := BuildStatusFromPod(p, c.buildSpec)

			// Common traits, set for test case brevity.
			c.want.Cluster = &v1alpha1.ClusterSpec{
				PodName:   "pod",
				Namespace: system.Namespace,
			}
			c.want.Builder = v1alpha1.ClusterBuildProvider
			c.want.StartTime = &now

			if d := cmp.Diff(got, c.want, ignoreVolatileTime); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}
