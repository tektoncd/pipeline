/*
Copyright 2019 The Tekton Authors

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

package pod

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/system"
	"github.com/tektoncd/pipeline/pkg/version"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	logtesting "knative.dev/pkg/logging/testing"
)

var (
	images = pipeline.Images{
		EntrypointImage: "entrypoint-image",
		CredsImage:      "override-with-creds:latest",
		ShellImage:      "busybox",
	}

	ignoreReleaseAnnotation = func(k string, v string) bool {
		return k == ReleaseAnnotation
	}
	featureInjectedSidecar                   = "running-in-environment-with-injected-sidecars"
	featureFlagDisableHomeEnvKey             = "disable-home-env-overwrite"
	featureFlagDisableWorkingDirKey          = "disable-working-directory-overwrite"
	featureFlagSetReadyAnnotationOnPodCreate = "enable-ready-annotation-on-pod-create"
)

func TestPodBuild(t *testing.T) {
	implicitEnvVars := []corev1.EnvVar{{
		Name:  "HOME",
		Value: homeDir,
	}}
	secretsVolume := corev1.Volume{
		Name:         "tekton-internal-secret-volume-multi-creds-9l9zj",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "multi-creds"}},
	}
	placeToolsInit := corev1.Container{
		Name:         "place-tools",
		Image:        images.EntrypointImage,
		Command:      []string{"/ko-app/entrypoint", "cp", "/ko-app/entrypoint", "/tekton/tools/entrypoint"},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}
	runtimeClassName := "gvisor"
	automountServiceAccountToken := false
	dnsPolicy := corev1.DNSNone
	enableServiceLinks := false
	priorityClassName := "system-cluster-critical"

	for _, c := range []struct {
		desc            string
		trs             v1beta1.TaskRunSpec
		trAnnotation    map[string]string
		ts              v1beta1.TaskSpec
		featureFlags    map[string]string
		want            *corev1.PodSpec
		wantAnnotations map[string]string
	}{{
		desc: "simple",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-name",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "simple with running-in-environment-with-injected-sidecar set to false",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		},
		featureFlags: map[string]string{
			featureInjectedSidecar: "false",
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-name",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
		wantAnnotations: map[string]string{
			readyAnnotation: readyAnnotationValue,
		},
	}, {
		desc: "with service account",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		},
		trs: v1beta1.TaskRunSpec{
			ServiceAccountName: "service-account",
		},
		want: &corev1.PodSpec{
			ServiceAccountName: "service-account",
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers:     []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-name",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-basic-docker=multi-creds=https://docker.io",
					"-basic-docker=multi-creds=https://us.gcr.io",
					"-basic-git=multi-creds=github.com",
					"-basic-git=multi-creds=gitlab.com",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-mz4c7",
					MountPath: "/tekton/creds",
				}}, append(append([]corev1.VolumeMount{}, implicitVolumeMounts...), corev1.VolumeMount{
					Name:      "tekton-internal-secret-volume-multi-creds-9l9zj",
					MountPath: "/tekton/creds-secrets/multi-creds",
				})...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, secretsVolume, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-mz4c7",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "with-pod-template",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		},
		trs: v1beta1.TaskRunSpec{
			PodTemplate: &v1beta1.PodTemplate{
				SecurityContext: &corev1.PodSecurityContext{
					Sysctls: []corev1.Sysctl{
						{Name: "net.ipv4.tcp_syncookies", Value: "1"},
					},
				},
				RuntimeClassName:             &runtimeClassName,
				AutomountServiceAccountToken: &automountServiceAccountToken,
				DNSPolicy:                    &dnsPolicy,
				DNSConfig: &corev1.PodDNSConfig{
					Nameservers: []string{"8.8.8.8"},
					Searches:    []string{"tekton.local"},
				},
				EnableServiceLinks: &enableServiceLinks,
				PriorityClassName:  &priorityClassName,
			},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-name",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{
					toolsMount,
					downwardMount,
					{Name: "tekton-creds-init-home-9l9zj", MountPath: "/tekton/creds"},
				}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
			SecurityContext: &corev1.PodSecurityContext{
				Sysctls: []corev1.Sysctl{
					{Name: "net.ipv4.tcp_syncookies", Value: "1"},
				},
			},
			RuntimeClassName:             &runtimeClassName,
			AutomountServiceAccountToken: &automountServiceAccountToken,
			DNSPolicy:                    dnsPolicy,
			DNSConfig: &corev1.PodDNSConfig{
				Nameservers: []string{"8.8.8.8"},
				Searches:    []string{"tekton.local"},
			},
			EnableServiceLinks: &enableServiceLinks,
			PriorityClassName:  priorityClassName,
		},
	}, {
		desc: "very long step name",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "a-very-very-long-character-step-name-to-trigger-max-len----and-invalid-characters",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-a-very-very-long-character-step-name-to-trigger-max-len", // step name trimmed.
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "step name ends with non alphanumeric",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "ends-with-invalid-%%__$$",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-ends-with-invalid", // invalid suffix removed.
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "workingDir in workspace",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:       "name",
				Image:      "image",
				Command:    []string{"cmd"}, // avoid entrypoint lookup.
				WorkingDir: filepath.Join(pipeline.WorkspaceDir, "test"),
			}}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:         "working-dir-initializer",
					Image:        images.ShellImage,
					Command:      []string{"sh"},
					Args:         []string{"-c", fmt.Sprintf("mkdir -p %s", filepath.Join(pipeline.WorkspaceDir, "test"))},
					WorkingDir:   pipeline.WorkspaceDir,
					VolumeMounts: implicitVolumeMounts,
				},
				placeToolsInit,
			},
			Containers: []corev1.Container{{
				Name:    "step-name",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             filepath.Join(pipeline.WorkspaceDir, "test"),
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "sidecar container",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "primary-name",
				Image:   "primary-image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
			Sidecars: []v1beta1.Sidecar{{
				Container: corev1.Container{
					Name:  "sc-name",
					Image: "sidecar-image",
				},
			}},
		},
		wantAnnotations: map[string]string{},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-primary-name",
				Image:   "primary-image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}, {
				Name:  "sidecar-sc-name",
				Image: "sidecar-image",
				Resources: corev1.ResourceRequirements{
					Requests: nil,
				},
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "sidecar container with script",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "primary-name",
				Image:   "primary-image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
			Sidecars: []v1beta1.Sidecar{{
				Container: corev1.Container{
					Name:  "sc-name",
					Image: "sidecar-image",
				},
				Script: "#!/bin/sh\necho hello from sidecar",
			}},
		},
		wantAnnotations: map[string]string{},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:         "place-scripts",
					Image:        "busybox",
					Command:      []string{"sh"},
					VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
					Args: []string{"-c", `tmpfile="/tekton/scripts/sidecar-script-0-9l9zj"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'sidecar-script-heredoc-randomly-generated-mz4c7'
#!/bin/sh
echo hello from sidecar
sidecar-script-heredoc-randomly-generated-mz4c7
`},
				},
				placeToolsInit,
			},
			Containers: []corev1.Container{{
				Name:    "step-primary-name",
				Image:   "primary-image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-mssqb",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}, {
				Name:  "sidecar-sc-name",
				Image: "sidecar-image",
				Resources: corev1.ResourceRequirements{
					Requests: nil,
				},
				Command:      []string{"/tekton/scripts/sidecar-script-0-9l9zj"},
				VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
			}},
			Volumes: append(implicitVolumes, scriptsVolume, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-mssqb",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "sidecar container with enable-ready-annotation-on-pod-create",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "primary-name",
				Image:   "primary-image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
			Sidecars: []v1beta1.Sidecar{{
				Container: corev1.Container{
					Name:  "sc-name",
					Image: "sidecar-image",
				},
			}},
		},
		featureFlags: map[string]string{
			featureFlagSetReadyAnnotationOnPodCreate: "true",
		},
		wantAnnotations: map[string]string{}, // no ready annotations on pod create since sidecars are present
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-primary-name",
				Image:   "primary-image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}, {
				Name:  "sidecar-sc-name",
				Image: "sidecar-image",
				Resources: corev1.ResourceRequirements{
					Requests: nil,
				},
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "resource request",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
				},
			}}, {Container: corev1.Container{
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("100Gi"),
					},
				},
			}}},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-unnamed-0",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir: pipeline.WorkspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("8"),
						corev1.ResourceMemory:           zeroQty,
						corev1.ResourceEphemeralStorage: zeroQty,
					},
				},
				TerminationMessagePath: "/tekton/termination",
			}, {
				Name:    "step-unnamed-1",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/tools/0",
					"-post_file",
					"/tekton/tools/1",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, {
					Name:      "tekton-creds-init-home-mz4c7",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir: pipeline.WorkspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              zeroQty,
						corev1.ResourceMemory:           resource.MustParse("100Gi"),
						corev1.ResourceEphemeralStorage: zeroQty,
					},
				},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}, corev1.Volume{
				Name:         "tekton-creds-init-home-mz4c7",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "step with script and stepTemplate",
		ts: v1beta1.TaskSpec{
			StepTemplate: &corev1.Container{
				Env:  []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
				Args: []string{"template", "args"},
			},
			Steps: []v1beta1.Step{{
				Container: corev1.Container{
					Name:  "one",
					Image: "image",
				},
				Script: "#!/bin/sh\necho hello from step one",
			}, {
				Container: corev1.Container{
					Name:         "two",
					Image:        "image",
					VolumeMounts: []corev1.VolumeMount{{Name: "i-have-a-volume-mount"}},
				},
				Script: `#!/usr/bin/env python
print("Hello from Python")`,
			}, {
				Container: corev1.Container{
					Name:    "regular-step",
					Image:   "image",
					Command: []string{"regular", "command"},
				},
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:    "place-scripts",
					Image:   images.ShellImage,
					Command: []string{"sh"},
					Args: []string{"-c", `tmpfile="/tekton/scripts/script-0-9l9zj"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-mz4c7'
#!/bin/sh
echo hello from step one
script-heredoc-randomly-generated-mz4c7
tmpfile="/tekton/scripts/script-1-mssqb"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-78c5n'
#!/usr/bin/env python
print("Hello from Python")
script-heredoc-randomly-generated-78c5n
`},
					VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
				},
				{
					Name:         "place-tools",
					Image:        images.EntrypointImage,
					Command:      []string{"/ko-app/entrypoint", "cp", "/ko-app/entrypoint", "/tekton/tools/entrypoint"},
					VolumeMounts: []corev1.VolumeMount{toolsMount},
				}},
			Containers: []corev1.Container{{
				Name:    "step-one",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"/tekton/scripts/script-0-9l9zj",
					"--",
					"template",
					"args",
				},
				Env: append(implicitEnvVars, corev1.EnvVar{Name: "FOO", Value: "bar"}),
				VolumeMounts: append([]corev1.VolumeMount{scriptsVolumeMount, toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-6nl7g",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}, {
				Name:    "step-two",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/tools/0",
					"-post_file",
					"/tekton/tools/1",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"/tekton/scripts/script-1-mssqb",
					"--",
					"template",
					"args",
				},
				Env: append(implicitEnvVars, corev1.EnvVar{Name: "FOO", Value: "bar"}),
				VolumeMounts: append([]corev1.VolumeMount{{Name: "i-have-a-volume-mount"}, scriptsVolumeMount, toolsMount, {
					Name:      "tekton-creds-init-home-j2tds",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}, {
				Name:    "step-regular-step",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/tools/1",
					"-post_file",
					"/tekton/tools/2",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"regular",
					"--",
					"command",
					"template",
					"args",
				},
				Env: append(implicitEnvVars, corev1.EnvVar{Name: "FOO", Value: "bar"}),
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, {
					Name:      "tekton-creds-init-home-vr6ds",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, scriptsVolume, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-6nl7g",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}, corev1.Volume{
				Name:         "tekton-creds-init-home-j2tds",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}, corev1.Volume{
				Name:         "tekton-creds-init-home-vr6ds",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}, {
		desc: "using another scheduler",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{
				{
					Container: corev1.Container{
						Name:    "schedule-me",
						Image:   "image",
						Command: []string{"cmd"}, // avoid entrypoint lookup.
					},
				},
			},
		},
		trs: v1beta1.TaskRunSpec{
			PodTemplate: &v1beta1.PodTemplate{
				SchedulerName: "there-scheduler",
			},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			SchedulerName:  "there-scheduler",
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
			Containers: []corev1.Container{{
				Name:    "step-schedule-me",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
		},
	}, {
		desc: "setting image pull secret",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{
				{
					Container: corev1.Container{
						Name:    "image-pull",
						Image:   "image",
						Command: []string{"cmd"}, // avoid entrypoint lookup.
					},
				},
			},
		},
		trs: v1beta1.TaskRunSpec{
			PodTemplate: &v1beta1.PodTemplate{
				ImagePullSecrets: []corev1.LocalObjectReference{{Name: "imageSecret"}},
			},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
			Containers: []corev1.Container{{
				Name:    "step-image-pull",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "imageSecret"}},
		}}, {
		desc: "using hostNetwork",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{
				{
					Container: corev1.Container{
						Name:    "use-my-hostNetwork",
						Image:   "image",
						Command: []string{"cmd"}, // avoid entrypoint lookup.
					},
				},
			},
		},
		trs: v1beta1.TaskRunSpec{
			PodTemplate: &v1beta1.PodTemplate{
				HostNetwork: true,
			},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			HostNetwork:    true,
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
			Containers: []corev1.Container{{
				Name:    "step-use-my-hostNetwork",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
		},
	}, {
		desc: "with a propagated Affinity Assistant name - expect proper affinity",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{
				{
					Container: corev1.Container{
						Name:    "name",
						Image:   "image",
						Command: []string{"cmd"}, // avoid entrypoint lookup.
					},
				},
			},
		},
		trAnnotation: map[string]string{
			"pipeline.tekton.dev/affinity-assistant": "random-name-123",
		},
		trs: v1beta1.TaskRunSpec{
			PodTemplate: &v1beta1.PodTemplate{},
		},
		want: &corev1.PodSpec{
			Affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/instance":  "random-name-123",
								"app.kubernetes.io/component": "affinity-assistant",
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					}},
				},
			},
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			HostNetwork:    false,
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
			Containers: []corev1.Container{{
				Name:    "step-name",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
		},
	}, {
		desc: "step-with-timeout",
		ts: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			},
				Timeout: &metav1.Duration{Duration: time.Second},
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			Containers: []corev1.Container{{
				Name:    "step-name",
				Image:   "image",
				Command: []string{"/tekton/tools/entrypoint"},
				Args: []string{
					"-wait_file",
					"/tekton/downward/ready",
					"-wait_file_content",
					"-post_file",
					"/tekton/tools/0",
					"-termination_path",
					"/tekton/termination",
					"-timeout",
					"1s",
					"-entrypoint",
					"cmd",
					"--",
				},
				Env: implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount, {
					Name:      "tekton-creds-init-home-9l9zj",
					MountPath: "/tekton/creds",
				}}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume, corev1.Volume{
				Name:         "tekton-creds-init-home-9l9zj",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
			}),
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
					Data:       c.featureFlags,
				},
			)
			kubeclient := fakek8s.NewSimpleClientset(
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"}},
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "service-account", Namespace: "default"},
					Secrets: []corev1.ObjectReference{{
						Name: "multi-creds",
					}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-creds",
						Namespace: "default",
						Annotations: map[string]string{
							"tekton.dev/docker-0": "https://us.gcr.io",
							"tekton.dev/docker-1": "https://docker.io",
							"tekton.dev/git-0":    "github.com",
							"tekton.dev/git-1":    "gitlab.com",
						}},
					Type: "kubernetes.io/basic-auth",
					Data: map[string][]byte{
						"username": []byte("foo"),
						"password": []byte("BestEver"),
					},
				},
			)
			var trAnnotations map[string]string
			if c.trAnnotation == nil {
				trAnnotations = map[string]string{
					ReleaseAnnotation: version.PipelineVersion,
				}
			} else {
				trAnnotations = c.trAnnotation
				trAnnotations[ReleaseAnnotation] = version.PipelineVersion
			}
			tr := &v1beta1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "taskrun-name",
					Namespace:   "default",
					Annotations: trAnnotations,
				},
				Spec: c.trs,
			}

			// No entrypoints should be looked up.
			entrypointCache := fakeCache{}

			builder := Builder{
				Images:          images,
				KubeClient:      kubeclient,
				EntrypointCache: entrypointCache,
				OverrideHomeEnv: true,
			}
			got, err := builder.Build(store.ToContext(context.Background()), tr, c.ts)
			if err != nil {
				t.Fatalf("builder.Build: %v", err)
			}

			if !strings.HasPrefix(got.Name, "taskrun-name-pod-") {
				t.Errorf("Pod name %q should have prefix 'taskrun-name-pod-'", got.Name)
			}

			if d := cmp.Diff(c.want, &got.Spec, resourceQuantityCmp); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}

			if c.wantAnnotations != nil {
				if d := cmp.Diff(c.wantAnnotations, got.ObjectMeta.Annotations, cmpopts.IgnoreMapEntries(ignoreReleaseAnnotation)); d != "" {
					t.Errorf("Annotation Diff(-want, +got):\n%s", d)
				}
			}
		})
	}
}

func TestMakeLabels(t *testing.T) {
	taskRunName := "task-run-name"
	want := map[string]string{
		taskRunLabelKey: taskRunName,
		"foo":           "bar",
		"hello":         "world",
	}
	got := MakeLabels(&v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: taskRunName,
			Labels: map[string]string{
				"foo":   "bar",
				"hello": "world",
			},
		},
	})
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff labels %s", diff.PrintWantGot(d))
	}
}

func TestShouldOverrideHomeEnv(t *testing.T) {
	for _, tc := range []struct {
		description string
		configMap   *corev1.ConfigMap
		expected    bool
	}{{
		description: "Default behaviour: A missing disable-home-env-overwrite flag should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data:       map[string]string{},
		},
		expected: true,
	}, {
		description: "Setting disable-home-env-overwrite to false should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableHomeEnvKey: "false",
			},
		},
		expected: true,
	}, {
		description: "Setting disable-home-env-overwrite to true should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableHomeEnvKey: "true",
			},
		},
		expected: false,
	}} {
		t.Run(tc.description, func(t *testing.T) {
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(tc.configMap)
			if result := ShouldOverrideHomeEnv(store.ToContext(context.Background())); result != tc.expected {
				t.Errorf("Expected %t Received %t", tc.expected, result)
			}
		})
	}
}

func TestShouldOverrideWorkingDir(t *testing.T) {
	for _, tc := range []struct {
		description string
		configMap   *corev1.ConfigMap
		expected    bool
	}{{
		description: "Default behaviour: A missing disable-working-directory-overwrite flag should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data:       map[string]string{},
		},
		expected: true,
	}, {
		description: "Setting disable-working-directory-overwrite to false should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableWorkingDirKey: "false",
			},
		},
		expected: true,
	}, {
		description: "Setting disable-working-directory-overwrite to true should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableWorkingDirKey: "true",
			},
		},
		expected: false,
	}} {
		t.Run(tc.description, func(t *testing.T) {
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(tc.configMap)
			ctx := store.ToContext(context.Background())
			if result := shouldOverrideWorkingDir(ctx); result != tc.expected {
				t.Errorf("Expected %t Received %t", tc.expected, result)
			}
		})
	}
}

func TestShouldAddReadyAnnotationonPodCreate(t *testing.T) {
	sd := v1beta1.Sidecar{
		Container: corev1.Container{
			Name: "a-sidecar",
		},
	}
	tcs := []struct {
		description string
		sidecars    []v1beta1.Sidecar
		configMap   *corev1.ConfigMap
		expected    bool
	}{{
		description: "Default behavior with sidecars present: Ready annotation not set on pod create",
		sidecars:    []v1beta1.Sidecar{sd},
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data:       map[string]string{},
		},
		expected: false,
	}, {
		description: "Default behavior with no sidecars present: Ready annotation not set on pod create",
		sidecars:    []v1beta1.Sidecar{},
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data:       map[string]string{},
		},
		expected: false,
	}, {
		description: "Setting running-in-environment-with-injected-sidecars to true with sidecars present results in false",
		sidecars:    []v1beta1.Sidecar{sd},
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureInjectedSidecar: "true",
			},
		},
		expected: false,
	}, {
		description: "Setting running-in-environment-with-injected-sidecars to true with no sidecars present results in false",
		sidecars:    []v1beta1.Sidecar{},
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureInjectedSidecar: "true",
			},
		},
		expected: false,
	}, {
		description: "Setting running-in-environment-with-injected-sidecars to false with sidecars present results in false",
		sidecars:    []v1beta1.Sidecar{sd},
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureInjectedSidecar: "false",
			},
		},
		expected: false,
	}, {
		description: "Setting running-in-environment-with-injected-sidecars to false with no sidecars present results in true",
		sidecars:    []v1beta1.Sidecar{},
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureInjectedSidecar: "false",
			},
		},
		expected: true,
	}}

	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(tc.configMap)
			if result := shouldAddReadyAnnotationOnPodCreate(store.ToContext(context.Background()), tc.sidecars); result != tc.expected {
				t.Errorf("expected: %t Received: %t", tc.expected, result)
			}
		})
	}
}
