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
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/system"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

var (
	images = pipeline.Images{
		EntrypointImage: "entrypoint-image",
		CredsImage:      "override-with-creds:latest",
		ShellImage:      "busybox",
	}
)

func TestMakePod(t *testing.T) {
	names.TestingSeed()

	implicitEnvVars := []corev1.EnvVar{{
		Name:  "HOME",
		Value: homeDir,
	}}
	secretsVolumeMount := corev1.VolumeMount{
		Name:      "tekton-internal-secret-volume-multi-creds-9l9zj",
		MountPath: "/tekton/creds-secrets/multi-creds",
	}
	secretsVolume := corev1.Volume{
		Name:         "tekton-internal-secret-volume-multi-creds-9l9zj",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "multi-creds"}},
	}

	placeToolsInit := corev1.Container{
		Name:         "place-tools",
		Image:        images.EntrypointImage,
		Command:      []string{"cp", "/ko-app/entrypoint", "/tekton/tools/entrypoint"},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}

	runtimeClassName := "gvisor"
	automountServiceAccountToken := false
	dnsPolicy := corev1.DNSNone
	enableServiceLinks := false
	priorityClassName := "system-cluster-critical"

	for _, c := range []struct {
		desc            string
		trs             v1alpha1.TaskRunSpec
		ts              v1alpha1.TaskSpec
		want            *corev1.PodSpec
		wantAnnotations map[string]string
	}{{
		desc: "simple",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		}},
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume),
		},
	}, {
		desc: "with service account",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		}},
		trs: v1alpha1.TaskRunSpec{
			ServiceAccountName: "service-account",
		},
		want: &corev1.PodSpec{
			ServiceAccountName: "service-account",
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:    "credential-initializer",
				Image:   images.CredsImage,
				Command: []string{"/ko-app/creds-init"},
				Args: []string{
					"-basic-docker=multi-creds=https://docker.io",
					"-basic-docker=multi-creds=https://us.gcr.io",
					"-basic-git=multi-creds=github.com",
					"-basic-git=multi-creds=gitlab.com",
				},
				VolumeMounts: append(implicitVolumeMounts, secretsVolumeMount),
				Env:          implicitEnvVars,
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, secretsVolume, toolsVolume, downwardVolume),
		},
	}, {
		desc: "with-pod-template",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "name",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		}},
		trs: v1alpha1.TaskRunSpec{
			PodTemplate: &v1alpha1.PodTemplate{
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume),
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
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "a-very-very-long-character-step-name-to-trigger-max-len----and-invalid-characters",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		}},
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume),
		},
	}, {
		desc: "step name ends with non alphanumeric",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "ends-with-invalid-%%__$$",
				Image:   "image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
		}},
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume),
		},
	}, {
		desc: "workingDir in workspace",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "name",
				Image:      "image",
				Command:    []string{"cmd"}, // avoid entrypoint lookup.
				WorkingDir: filepath.Join(pipeline.WorkspaceDir, "test"),
			}}},
		}},
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:             filepath.Join(pipeline.WorkspaceDir, "test"),
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume),
		},
	}, {
		desc: "sidecar container",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "primary-name",
				Image:   "primary-image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
			Sidecars: []v1alpha1.Sidecar{{
				Container: corev1.Container{
					Name:  "sc-name",
					Image: "sidecar-image",
				},
			}},
		}},
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
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
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume),
		},
	}, {
		desc: "sidecar container with script",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:    "primary-name",
				Image:   "primary-image",
				Command: []string{"cmd"}, // avoid entrypoint lookup.
			}}},
			Sidecars: []v1alpha1.Sidecar{{
				Container: corev1.Container{
					Name:  "sc-name",
					Image: "sidecar-image",
				},
				Script: "#!/bin/sh\necho hello from sidecar",
			}},
		}},
		wantAnnotations: map[string]string{},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:         "place-scripts",
					Image:        "busybox",
					Command:      []string{"sh"},
					TTY:          true,
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
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
			Volumes: append(implicitVolumes, scriptsVolume, toolsVolume, downwardVolume),
		},
	}, {
		desc: "resource request",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
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
		}},
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
				Env:          implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:   pipeline.WorkspaceDir,
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
				Env:          implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{toolsMount}, implicitVolumeMounts...),
				WorkingDir:   pipeline.WorkspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              zeroQty,
						corev1.ResourceMemory:           resource.MustParse("100Gi"),
						corev1.ResourceEphemeralStorage: zeroQty,
					},
				},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, toolsVolume, downwardVolume),
		},
	}, {
		desc: "step with script and stepTemplate",
		ts: v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
			StepTemplate: &corev1.Container{
				Env:  []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
				Args: []string{"template", "args"},
			},
			Steps: []v1alpha1.Step{{
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
		}},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:    "place-scripts",
					Image:   images.ShellImage,
					Command: []string{"sh"},
					TTY:     true,
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
					Command:      []string{"cp", "/ko-app/entrypoint", "/tekton/tools/entrypoint"},
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
				Env:                    append(implicitEnvVars, corev1.EnvVar{Name: "FOO", Value: "bar"}),
				VolumeMounts:           append([]corev1.VolumeMount{scriptsVolumeMount, toolsMount, downwardMount}, implicitVolumeMounts...),
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
				Env:                    append(implicitEnvVars, corev1.EnvVar{Name: "FOO", Value: "bar"}),
				VolumeMounts:           append([]corev1.VolumeMount{{Name: "i-have-a-volume-mount"}, scriptsVolumeMount, toolsMount}, implicitVolumeMounts...),
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
				Env:                    append(implicitEnvVars, corev1.EnvVar{Name: "FOO", Value: "bar"}),
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
			Volumes: append(implicitVolumes, scriptsVolume, toolsVolume, downwardVolume),
		},
	}, {
		desc: "using another scheduler",
		ts: v1alpha1.TaskSpec{
			TaskSpec: v1beta1.TaskSpec{
				Steps: []v1alpha1.Step{
					{
						Container: corev1.Container{
							Name:    "schedule-me",
							Image:   "image",
							Command: []string{"cmd"}, // avoid entrypoint lookup.
						},
					},
				},
			},
		},
		trs: v1alpha1.TaskRunSpec{
			PodTemplate: &v1alpha1.PodTemplate{
				SchedulerName: "there-scheduler",
			},
		},
		want: &corev1.PodSpec{
			RestartPolicy:  corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{placeToolsInit},
			SchedulerName:  "there-scheduler",
			Volumes:        append(implicitVolumes, toolsVolume, downwardVolume),
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
				Env:                    implicitEnvVars,
				VolumeMounts:           append([]corev1.VolumeMount{toolsMount, downwardMount}, implicitVolumeMounts...),
				WorkingDir:             pipeline.WorkspaceDir,
				Resources:              corev1.ResourceRequirements{Requests: allZeroQty()},
				TerminationMessagePath: "/tekton/termination",
			}},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			kubeclient := fakek8s.NewSimpleClientset(
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"}},
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "service-account", Namespace: "default"},
					Secrets: []corev1.ObjectReference{{
						Name: "multi-creds",
					}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "multi-creds",
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
			tr := &v1alpha1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "taskrun-name",
					Annotations: map[string]string{
						ReleaseAnnotation: ReleaseAnnotationValue,
					},
				},
				Spec: c.trs,
			}

			// No entrypoints should be looked up.
			entrypointCache := fakeCache{}

			got, err := MakePod(images, tr, c.ts, kubeclient, entrypointCache)
			if err != nil {
				t.Fatalf("MakePod: %v", err)
			}

			if !strings.HasPrefix(got.Name, "taskrun-name-pod-") {
				t.Errorf("Pod name %q should have prefix 'taskrun-name-pod-'", got.Name)
			}

			if d := cmp.Diff(c.want, &got.Spec, resourceQuantityCmp); d != "" {
				t.Errorf("Diff(-want, +got):\n%s", d)
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
	got := MakeLabels(&v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: taskRunName,
			Labels: map[string]string{
				"foo":   "bar",
				"hello": "world",
			},
		},
	})
	if d := cmp.Diff(got, want); d != "" {
		t.Errorf("Diff labels:\n%s", d)
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
			ObjectMeta: metav1.ObjectMeta{Name: featureFlagConfigMapName, Namespace: system.GetNamespace()},
			Data:       map[string]string{},
		},
		expected: true,
	}, {
		description: "Setting disable-home-env-overwrite to false should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: featureFlagConfigMapName, Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableHomeEnvKey: "false",
			},
		},
		expected: true,
	}, {
		description: "Setting disable-home-env-overwrite to true should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: featureFlagConfigMapName, Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableHomeEnvKey: "true",
			},
		},
		expected: false,
	}} {
		t.Run(tc.description, func(t *testing.T) {
			kubeclient := fakek8s.NewSimpleClientset(
				tc.configMap,
			)
			if result := shouldOverrideHomeEnv(kubeclient); result != tc.expected {
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
			ObjectMeta: metav1.ObjectMeta{Name: featureFlagConfigMapName, Namespace: system.GetNamespace()},
			Data:       map[string]string{},
		},
		expected: true,
	}, {
		description: "Setting disable-working-directory-overwrite to false should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: featureFlagConfigMapName, Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableWorkingDirKey: "false",
			},
		},
		expected: true,
	}, {
		description: "Setting disable-working-directory-overwrite to true should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: featureFlagConfigMapName, Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableWorkingDirKey: "true",
			},
		},
		expected: false,
	}} {
		t.Run(tc.description, func(t *testing.T) {
			kubeclient := fakek8s.NewSimpleClientset(
				tc.configMap,
			)
			if result := shouldOverrideWorkingDir(kubeclient); result != tc.expected {
				t.Errorf("Expected %t Received %t", tc.expected, result)
			}
		})
	}
}
