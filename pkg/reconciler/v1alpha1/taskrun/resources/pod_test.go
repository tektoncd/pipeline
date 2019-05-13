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
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakek8s "k8s.io/client-go/kubernetes/fake"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"github.com/tektoncd/pipeline/test/names"
)

var (
	resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	})
	nopContainer = corev1.Container{
		Name:    "nop",
		Image:   *nopImage,
		Command: []string{"/builder/tools/entrypoint"},
		Args:    []string{"-wait_file", "/builder/tools/0", "-post_file", "/builder/tools/1", "-entrypoint", "/ko-app/nop", "--"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      entrypoint.MountName,
			MountPath: entrypoint.MountPoint,
		}},
	}
)

func TestTryGetPod(t *testing.T) {
	err := fmt.Errorf("something went wrong")
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

	randReader = strings.NewReader(strings.Repeat("a", 10000))
	defer func() { randReader = rand.Reader }()

	for _, c := range []struct {
		desc         string
		trs          v1alpha1.TaskRunSpec
		ts           v1alpha1.TaskSpec
		bAnnotations map[string]string
		want         *corev1.PodSpec
		wantErr      error
	}{{
		desc: "simple",
		ts: v1alpha1.TaskSpec{
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
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        *credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "build-step-name",
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
			},
				nopContainer,
			},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "with-service-account",
		ts: v1alpha1.TaskSpec{
			Steps: []corev1.Container{{
				Name:  "name",
				Image: "image",
			}},
		},
		trs: v1alpha1.TaskRunSpec{
			ServiceAccount: "service-account",
		},
		want: &corev1.PodSpec{
			ServiceAccountName: "service-account",
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:    containerPrefix + credsInit + "-mz4c7",
				Image:   *credsImage,
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
				Name:         "build-step-name",
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
			},
				nopContainer,
			},
			Volumes: implicitVolumesWithSecrets,
		},
	}, {
		desc: "very-long-step-name",
		ts: v1alpha1.TaskSpec{
			Steps: []corev1.Container{{
				Name:  "a-very-long-character-step-name-to-trigger-max-len----and-invalid-characters",
				Image: "image",
			}},
		},
		bAnnotations: map[string]string{
			"simple-annotation-key": "simple-annotation-val",
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        *credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "build-step-a-very-long-character-step-name-to-trigger-max-len",
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
			},
				nopContainer,
			},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "step-name-ends-with-non-alphanumeric",
		ts: v1alpha1.TaskSpec{
			Steps: []corev1.Container{{
				Name:  "ends-with-invalid-%%__$$",
				Image: "image",
			}},
		},
		bAnnotations: map[string]string{
			"simple-annotation-key": "simple-annotation-val",
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        *credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "build-step-ends-with-invalid",
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
			},
				nopContainer,
			},
			Volumes: implicitVolumes,
		},
	}, {
		desc: "working-dir-in-workspace-dir",
		ts: v1alpha1.TaskSpec{
			Steps: []corev1.Container{{
				Name:       "name",
				Image:      "image",
				WorkingDir: filepath.Join(workspaceDir, "test"),
			}},
		},
		want: &corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{{
				Name:         containerPrefix + credsInit + "-9l9zj",
				Image:        *credsImage,
				Command:      []string{"/ko-app/creds-init"},
				Args:         []string{},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}, {
				Name:         containerPrefix + workingDirInit + "-mz4c7",
				Image:        *v1alpha1.BashNoopImage,
				Command:      []string{"/ko-app/bash"},
				Args:         []string{"-args", fmt.Sprintf("mkdir -p %s", filepath.Join(workspaceDir, "test"))},
				Env:          implicitEnvVars,
				VolumeMounts: implicitVolumeMounts,
				WorkingDir:   workspaceDir,
			}},
			Containers: []corev1.Container{{
				Name:         "build-step-name",
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
			},
				nopContainer,
			},
			Volumes: implicitVolumes,
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
					Annotations: c.bAnnotations,
				},
				Spec: c.trs,
			}
			cache, _ := entrypoint.NewCache()
			got, err := MakePod(tr, c.ts, cs, cache, logger)
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
