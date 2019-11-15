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

package resources

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

var (
	resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})
	credsImage = "override-with-creds:latest"
	shellImage = "busybox"
)

func TestTryGetPod(t *testing.T) {
	err := xerrors.New("something went wrong")
	for _, c := range []struct {
		desc    string
		trs     v1alpha1.TaskRunStatus
		gp      GetPod
		wantNil bool
		wantErr error
	}{{
		desc: "no-pod",
		trs:  v1alpha1.TaskRunStatus{},
		gp: func(string, metav1.GetOptions) (*corev1.Pod, error) {
			t.Errorf("Did not expect pod to be fetched")
			return nil, nil
		},
		wantNil: true,
		wantErr: nil,
	}, {
		desc: "non-existent-pod",
		trs: v1alpha1.TaskRunStatus{
			PodName: "no-longer-exist",
		},
		gp: func(name string, opts metav1.GetOptions) (*corev1.Pod, error) {
			return nil, errors.NewNotFound(schema.GroupResource{}, name)
		},
		wantNil: true,
		wantErr: nil,
	}, {
		desc: "existing-pod",
		trs: v1alpha1.TaskRunStatus{
			PodName: "exists",
		},
		gp: func(name string, opts metav1.GetOptions) (*corev1.Pod, error) {
			return &corev1.Pod{}, nil
		},
		wantNil: false,
		wantErr: nil,
	}, {
		desc: "pod-fetch-error",
		trs: v1alpha1.TaskRunStatus{
			PodName: "something-went-wrong",
		},
		gp: func(name string, opts metav1.GetOptions) (*corev1.Pod, error) {
			return nil, err
		},
		wantNil: true,
		wantErr: err,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			pod, err := TryGetPod(c.trs, c.gp)
			if err != c.wantErr {
				t.Fatalf("TryGetPod: %v", err)
			}

			wasNil := pod == nil
			if wasNil != c.wantNil {
				t.Errorf("Pod got %v, want %v", wasNil, c.wantNil)
			}
		})
	}
}

func TestMakePod(t *testing.T) {
	names.TestingSeed()

	implicitVolumeMountsWithSecrets := append(implicitVolumeMounts, corev1.VolumeMount{
		Name:      "secret-volume-multi-creds-9l9zj",
		MountPath: "/var/build-secrets/multi-creds",
	})
	implicitVolumesWithSecrets := append(implicitVolumes, corev1.Volume{
		Name:         "secret-volume-multi-creds-9l9zj",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "multi-creds"}},
	})

	runtimeClassName := "gvisor"

	randReader = strings.NewReader(strings.Repeat("a", 10000))
	defer func() { randReader = rand.Reader }()

	for _, c := range []struct {
		desc        string
		trs         v1alpha1.TaskRunSpec
		ts          v1alpha1.TaskSpec
		annotations map[string]string
		want        *corev1.PodSpec
		wantErr     error
	}{{
		desc: "simple",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "name",
				Image: "image",
			}}},
		},
		annotations: map[string]string{
			"simple-annotation-key": "simple-annotation-val",
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "with-service-account",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "name",
				Image: "image",
			}}},
		},
		trs: v1alpha1.TaskRunSpec{
			ServiceAccountName: "service-account",
		},
		want: &corev1.PodSpec{
			ServiceAccountName: "service-account",
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:    containerPrefix + credsInit + "-mz4c7",
				Image:   credsImage,
				Command: []string{"/ko-app/creds-init"},
				Args: []string{
					"-basic-docker=multi-creds=https://docker.io",
					"-basic-docker=multi-creds=https://us.gcr.io",
					"-basic-git=multi-creds=github.com",
					"-basic-git=multi-creds=gitlab.com",
				},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMountsWithSecrets,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: implicitVolumesWithSecrets,
		},
	}, {
		desc: "with-deprecated-service-account",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "name",
				Image: "image",
			}}},
		},
		trs: v1alpha1.TaskRunSpec{
			DeprecatedServiceAccount: "service-account",
		},
		want: &corev1.PodSpec{
			ServiceAccountName: "service-account",
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:    containerPrefix + credsInit + "-mz4c7",
				Image:   credsImage,
				Command: []string{"/ko-app/creds-init"},
				Args: []string{
					"-basic-docker=multi-creds=https://docker.io",
					"-basic-docker=multi-creds=https://us.gcr.io",
					"-basic-git=multi-creds=github.com",
					"-basic-git=multi-creds=gitlab.com",
				},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMountsWithSecrets,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: implicitVolumesWithSecrets,
		},
	}, {
		desc: "with-pod-template",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "name",
				Image: "image",
			}}},
		},
		trs: v1alpha1.TaskRunSpec{
			PodTemplate: v1alpha1.PodTemplate{
				SecurityContext: &corev1.PodSecurityContext{
					Sysctls: []corev1.Sysctl{
						{Name: "net.ipv4.tcp_syncookies", Value: "1"},
					},
				},
				RuntimeClassName: &runtimeClassName,
			},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: implicitVolumes,
			SecurityContext: &corev1.PodSecurityContext{
				Sysctls: []corev1.Sysctl{
					{Name: "net.ipv4.tcp_syncookies", Value: "1"},
				},
			},
			RuntimeClassName: &runtimeClassName,
		},
	}, {
		desc: "very-long-step-name",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "a-very-very-long-character-step-name-to-trigger-max-len----and-invalid-characters",
				Image: "image",
			}}},
		},
		annotations: map[string]string{
			"simple-annotation-key": "simple-annotation-val",
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-a-very-very-long-character-step-name-to-trigger-max-len",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "step-name-ends-with-non-alphanumeric",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "ends-with-invalid-%%__$$",
				Image: "image",
			}}},
		},
		annotations: map[string]string{
			"simple-annotation-key": "simple-annotation-val",
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-ends-with-invalid",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "working-dir-in-workspace-dir",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:       "name",
				Image:      "image",
				WorkingDir: filepath.Join(workspaceDir, "test"),
			}}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}, {
				Name:         containerPrefix + workingDirInit + "-mz4c7",
				Image:        shellImage,
				Command:      []string{"sh"},
				Args:         []string{"-c", fmt.Sprintf("mkdir -p %s", filepath.Join(workspaceDir, "test"))},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-name",
				Image:        "image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   filepath.Join(workspaceDir, "test"),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "additional-sidecar-container",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{Container: corev1.Container{
				Name:  "primary-name",
				Image: "primary-image",
			}}},
			Sidecars: []corev1.Container{{
				Name:  "sc-name",
				Image: "sidecar-image",
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "step-primary-name",
				Image:        "primary-image",
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}, {
				Name:  "sidecar-sc-name",
				Image: "sidecar-image",
				Resources: corev1.ResourceRequirements{
					Requests: nil,
				},
			}},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "step with script",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Name:    "one",
					Image:   "image",
					Command: []string{"entrypointer"},
					Args:    []string{"wait-file", "out-file", "-entrypoint", "image-entrypoint", "--"},
				},
				Script: "echo hello from step one",
			}, {
				Container: corev1.Container{
					Name:         "two",
					Image:        "image",
					VolumeMounts: []corev1.VolumeMount{{Name: "i-have-a-volume-mount"}},
					Command:      []string{"entrypointer"},
					// args aren't valid, but just in case they end up here we'll replace them.
					Args: []string{"wait-file", "out-file", "-entrypoint", "image-entrypoint", "--", "args", "somehow"},
				},
				Script: `#!/usr/bin/env python
print("Hello from Python")`,
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}, {
				Name:    "place-scripts-mz4c7",
				Image:   images.ShellImage,
				Command: []string{"sh"},
				TTY:     true,
				Args: []string{"-c", `tmpfile="/builder/scripts/script-0-mssqb"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-78c5n'
echo hello from step one
script-heredoc-randomly-generated-78c5n
tmpfile="/builder/scripts/script-1-6nl7g"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-j2tds'
#!/usr/bin/env python
print("Hello from Python")
script-heredoc-randomly-generated-j2tds
`},
				VolumeMounts: []corev1.VolumeMount{scriptsVolumeMount},
			}},
			Containers: []corev1.Container{{
				Name:         "step-one",
				Image:        "image",
				Command:      []string{"entrypointer"},
				Args:         []string{"wait-file", "out-file", "-entrypoint", "/builder/scripts/script-0-mssqb"},
				Env:          implicitEnvVars,
				VolumeMounts: append(implicitVolumeMounts, scriptsVolumeMount),
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}, {
				Name:         "step-two",
				Image:        "image",
				Command:      []string{"entrypointer"},
				Args:         []string{"wait-file", "out-file", "-entrypoint", "/builder/scripts/script-1-6nl7g"},
				Env:          implicitEnvVars,
				VolumeMounts: append([]corev1.VolumeMount{{Name: "i-have-a-volume-mount"}}, append(implicitVolumeMounts, scriptsVolumeMount)...),
				WorkingDir:   workspaceDir,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("0"),
						corev1.ResourceMemory:           resource.MustParse("0"),
						corev1.ResourceEphemeralStorage: resource.MustParse("0"),
					},
				},
			}},
			Volumes: append(implicitVolumes, scriptsVolume),
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
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
					Name:        "taskrun-name",
					Annotations: c.annotations,
				},
				Spec: c.trs,
			}
			got, err := MakePod(images, tr, c.ts, cs)
			if err != c.wantErr {
				t.Fatalf("MakePod: %v", err)
			}

			// Generated name from hexlifying a stream of 'a's.
			wantName := "taskrun-name-pod-616161"
			if got.Name != wantName {
				t.Errorf("Pod name got %q, want %q", got.Name, wantName)
			}

			if d := cmp.Diff(&got.Spec, c.want, resourceQuantityCmp); d != "" {
				t.Errorf("Diff spec:\n%s", d)
			}

			wantAnnotations := map[string]string{ReadyAnnotation: ""}
			if c.annotations != nil {
				for key, val := range c.annotations {
					wantAnnotations[key] = val
				}
			}
			if d := cmp.Diff(got.Annotations, wantAnnotations); d != "" {
				t.Errorf("Diff annotations:\n%s", d)
			}
		})
	}
}

func TestMakeLabels(t *testing.T) {
	taskRunName := "task-run-name"
	for _, c := range []struct {
		desc     string
		trLabels map[string]string
		want     map[string]string
	}{{
		desc: "taskrun labels pass through",
		trLabels: map[string]string{
			"foo":   "bar",
			"hello": "world",
		},
		want: map[string]string{
			taskRunLabelKey:   taskRunName,
			ManagedByLabelKey: ManagedByLabelValue,
			"foo":             "bar",
			"hello":           "world",
		},
	}, {
		desc: "taskrun managed-by overrides; taskrun label key doesn't",
		trLabels: map[string]string{
			"foo":             "bar",
			taskRunLabelKey:   "override-me",
			ManagedByLabelKey: "managed-by-something-else",
		},
		want: map[string]string{
			taskRunLabelKey:   taskRunName,
			ManagedByLabelKey: "managed-by-something-else",
			"foo":             "bar",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := makeLabels(&v1alpha1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:   taskRunName,
					Labels: c.trLabels,
				},
			})
			if d := cmp.Diff(got, c.want); d != "" {
				t.Errorf("Diff labels:\n%s", d)
			}
		})
	}
}

func TestAddReadyAnnotation(t *testing.T) {
	pod := &corev1.Pod{}
	updateFunc := func(p *corev1.Pod) (*corev1.Pod, error) { return p, nil }
	if err := AddReadyAnnotation(pod, updateFunc); err != nil {
		t.Errorf("error received: %v", err)
	}
	if v := pod.ObjectMeta.Annotations[ReadyAnnotation]; v != readyAnnotationValue {
		t.Errorf("Annotation %q=%q missing from Pod", ReadyAnnotation, readyAnnotationValue)
	}
}

func TestAddReadyAnnotationUpdateError(t *testing.T) {
	testerror := xerrors.New("error updating pod")
	pod := &corev1.Pod{}
	updateFunc := func(p *corev1.Pod) (*corev1.Pod, error) { return p, testerror }
	if err := AddReadyAnnotation(pod, updateFunc); err != testerror {
		t.Errorf("expected %v received %v", testerror, err)
	}
}

func TestMakeAnnotations(t *testing.T) {
	for _, c := range []struct {
		desc                     string
		taskRun                  *v1alpha1.TaskRun
		expectedAnnotationSubset map[string]string
	}{{
		desc: "a taskruns annotations are copied to the pod",
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-taskrun",
				Annotations: map[string]string{
					"foo": "bar",
					"baz": "quux",
				},
			},
		},
		expectedAnnotationSubset: map[string]string{
			"foo": "bar",
			"baz": "quux",
		},
	}, {
		desc:    "initial pod annotations contain the ReadyAnnotation to pause steps until sidecars are ready",
		taskRun: &v1alpha1.TaskRun{},
		expectedAnnotationSubset: map[string]string{
			ReadyAnnotation: "",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			annos := makeAnnotations(c.taskRun)
			for k, v := range c.expectedAnnotationSubset {
				receivedValue, ok := annos[k]
				if !ok {
					t.Errorf("expected annotation %q was missing", k)
				}
				if receivedValue != v {
					t.Errorf("expected annotation %q=%q, received %q=%q", k, v, k, receivedValue)
				}
			}
		})
	}
}

func TestMakeWorkingDirScript(t *testing.T) {
	for _, c := range []struct {
		desc        string
		workingDirs map[string]bool
		want        string
	}{{
		desc:        "default",
		workingDirs: map[string]bool{"/workspace": true},
		want:        "",
	}, {
		desc:        "simple",
		workingDirs: map[string]bool{"/workspace/foo": true, "/workspace/bar": true, "/baz": true},
		want:        "mkdir -p /workspace/bar /workspace/foo",
	}, {
		desc:        "empty",
		workingDirs: map[string]bool{"/workspace": true, "": true, "/baz": true, "/workspacedir": true},
		want:        "",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			if script := makeWorkingDirScript(c.workingDirs); script != c.want {
				t.Errorf("Expected `%v`, got `%v`", c.want, script)
			}
		})
	}
}
