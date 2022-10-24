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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/system"
)

const (
	serviceAccountName           = "my-service-account"
	namespace                    = "namespacey-mcnamespace"
	featureFlagRequireKnownHosts = "require-git-ssh-secret-known-hosts"
)

func TestCredsInit(t *testing.T) {
	customHomeEnvVar := corev1.EnvVar{
		Name:  "HOME",
		Value: "/users/home/my-test-user",
	}
	for _, c := range []struct {
		desc             string
		wantArgs         []string
		wantVolumeMounts []corev1.VolumeMount
		objs             []runtime.Object
		envVars          []corev1.EnvVar
		ctx              context.Context
	}{{
		desc: "service account exists with no secrets; nothing to initialize",
		objs: []runtime.Object{
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace}},
		},
		wantArgs:         nil,
		wantVolumeMounts: nil,
		ctx:              context.Background(),
	}, {
		desc: "service account has no annotated secrets; nothing to initialize",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-creds",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "my-creds",
					Namespace:   namespace,
					Annotations: map[string]string{
						// No matching annotations.
					},
				},
			},
		},
		wantArgs:         nil,
		wantVolumeMounts: nil,
		ctx:              context.Background(),
	}, {
		desc: "service account has annotated secret and no HOME env var passed in; initialize creds in /tekton/creds",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-creds",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-creds",
					Namespace: namespace,
					Annotations: map[string]string{
						"tekton.dev/docker-0": "https://us.gcr.io",
						"tekton.dev/docker-1": "https://docker.io",
						"tekton.dev/git-0":    "github.com",
						"tekton.dev/git-1":    "gitlab.com",
					},
				},
				Type: "kubernetes.io/basic-auth",
				Data: map[string][]byte{
					"username": []byte("foo"),
					"password": []byte("BestEver"),
				},
			},
		},
		envVars: []corev1.EnvVar{},
		wantArgs: []string{
			"-basic-docker=my-creds=https://docker.io",
			"-basic-docker=my-creds=https://us.gcr.io",
			"-basic-git=my-creds=github.com",
			"-basic-git=my-creds=gitlab.com",
		},
		wantVolumeMounts: []corev1.VolumeMount{{
			Name:      "tekton-internal-secret-volume-my-creds-9l9zj",
			MountPath: "/tekton/creds-secrets/my-creds",
		}},
		ctx: context.Background(),
	}, {
		desc: "service account has duplicate dockerconfigjson secret and no HOME env var passed in; initialize creds in /tekton/creds",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-docker-creds",
				}, {
					Name: "my-docker-creds",
				}, {
					Name: "my-docker-creds",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-docker-creds",
					Namespace: namespace,
				},
				Type: "kubernetes.io/dockerconfigjson",
				Data: map[string][]byte{
					".dockerconfigjson": []byte("ewogICJhdXRocyI6IHsKICAgICJleGFtcGxlLmNvbSI6IHsKICAgICAgInVzZXJuYW1lIjogImRlbW8iLAogICAgICAicGFzc3dvcmQiOiAidGVzdCIKICB9Cn0KCg=="),
				},
			},
		},
		envVars: []corev1.EnvVar{},
		wantArgs: []string{
			"-docker-config=my-docker-creds",
		},
		wantVolumeMounts: []corev1.VolumeMount{{
			Name:      "tekton-internal-secret-volume-my-docker-creds-9l9zj",
			MountPath: "/tekton/creds-secrets/my-docker-creds",
		}},
		ctx: context.Background(),
	}, {
		desc: "service account with secret and HOME env var passed in",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-creds",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-creds",
					Namespace: namespace,
					Annotations: map[string]string{
						"tekton.dev/docker-0": "https://us.gcr.io",
						"tekton.dev/docker-1": "https://docker.io",
						"tekton.dev/git-0":    "github.com",
						"tekton.dev/git-1":    "gitlab.com",
					},
				},
				Type: "kubernetes.io/basic-auth",
				Data: map[string][]byte{
					"username": []byte("foo"),
					"password": []byte("BestEver"),
				},
			},
		},
		envVars: []corev1.EnvVar{customHomeEnvVar},
		wantArgs: []string{
			"-basic-docker=my-creds=https://docker.io",
			"-basic-docker=my-creds=https://us.gcr.io",
			"-basic-git=my-creds=github.com",
			"-basic-git=my-creds=gitlab.com",
		},
		wantVolumeMounts: []corev1.VolumeMount{{
			Name:      "tekton-internal-secret-volume-my-creds-9l9zj",
			MountPath: "/tekton/creds-secrets/my-creds",
		}},
		ctx: context.Background(),
	}, {
		desc: "disabling legacy credential helper (creds-init) via feature-flag results in no args or volumes",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-creds",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-creds",
					Namespace: namespace,
					Annotations: map[string]string{
						"tekton.dev/docker-0": "https://us.gcr.io",
						"tekton.dev/docker-1": "https://docker.io",
						"tekton.dev/git-0":    "github.com",
						"tekton.dev/git-1":    "gitlab.com",
					},
				},
				Type: "kubernetes.io/basic-auth",
				Data: map[string][]byte{
					"username": []byte("foo"),
					"password": []byte("BestEver"),
				},
			},
		},
		envVars:          []corev1.EnvVar{customHomeEnvVar},
		wantArgs:         nil,
		wantVolumeMounts: nil,
		ctx: config.ToContext(context.Background(), &config.Config{
			FeatureFlags: &config.FeatureFlags{
				DisableCredsInit: true,
			},
		}),
	}, {
		desc: "secret name contains characters that are not allowed in volume mount context",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "foo.bar.com",
				}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo.bar.com",
					Namespace:   namespace,
					Annotations: map[string]string{"tekton.dev/docker-0": "https://docker.io"},
				},
				Type: "kubernetes.io/basic-auth",
				Data: map[string][]byte{
					"username": []byte("foo"),
					"password": []byte("bar"),
				},
			},
		},
		envVars:  []corev1.EnvVar{},
		wantArgs: []string{"-basic-docker=foo.bar.com=https://docker.io"},
		wantVolumeMounts: []corev1.VolumeMount{{
			Name:      "tekton-internal-secret-volume-foo-bar-com-9l9zj",
			MountPath: "/tekton/creds-secrets/foo.bar.com",
		}},
		ctx: context.Background(),
	}, {
		desc: "service account has empty-named secrets",
		objs: []runtime.Object{
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
				Secrets: []corev1.ObjectReference{{
					Name: "my-creds",
				}, {}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-creds",
					Namespace: namespace,
					Annotations: map[string]string{
						"tekton.dev/git-0": "github.com",
					},
				},
				Type: "kubernetes.io/basic-auth",
				Data: map[string][]byte{
					"username": []byte("foo"),
					"password": []byte("BestEver"),
				},
			},
		},
		envVars: []corev1.EnvVar{},
		wantArgs: []string{
			"-basic-git=my-creds=github.com",
		},
		wantVolumeMounts: []corev1.VolumeMount{{
			Name:      "tekton-internal-secret-volume-my-creds-9l9zj",
			MountPath: "/tekton/creds-secrets/my-creds",
		}},
		ctx: context.Background(),
	}} {
		t.Run(c.desc, func(t *testing.T) {
			names.TestingSeed()
			kubeclient := fakek8s.NewSimpleClientset(c.objs...)
			args, volumes, volumeMounts, err := credsInit(c.ctx, serviceAccountName, namespace, kubeclient)
			if err != nil {
				t.Fatalf("credsInit: %v", err)
			}
			if len(args) == 0 && len(volumes) != 0 {
				t.Fatalf("credsInit returned secret volumes but no arguments")
			}
			if d := cmp.Diff(c.wantArgs, args); d != "" {
				t.Fatalf("Diff %s", diff.PrintWantGot(d))
			}
			if d := cmp.Diff(c.wantVolumeMounts, volumeMounts); d != "" {
				t.Fatalf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestCheckGitSSHSecret(t *testing.T) {
	for _, tc := range []struct {
		desc         string
		configMap    *corev1.ConfigMap
		secret       *corev1.Secret
		wantErrorMsg string
	}{{
		desc: "require known_hosts but secret does not include known_hosts",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				featureFlagRequireKnownHosts: "true",
			},
		},
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-creds",
				Namespace: namespace,
				Annotations: map[string]string{
					"tekton.dev/git-0": "github.com",
				},
			},
			Type: "kubernetes.io/ssh-auth",
			Data: map[string][]byte{
				"ssh-privatekey": []byte("Hello World!"),
			},
		},
		wantErrorMsg: "TaskRun validation failed. Git SSH Secret must have \"known_hosts\" included " +
			"when feature flag \"require-git-ssh-secret-known-hosts\" is set to true",
	}, {
		desc: "require known_hosts and secret includes known_hosts",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				featureFlagRequireKnownHosts: "true",
			},
		},
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-creds",
				Namespace: namespace,
				Annotations: map[string]string{
					"tekton.dev/git-0": "github.com",
				},
			},
			Type: "kubernetes.io/ssh-auth",
			Data: map[string][]byte{
				"ssh-privatekey": []byte("Hello World!"),
				"known_hosts":    []byte("Hello World!"),
			},
		},
	}, {
		desc: "not require known_hosts",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				featureFlagRequireKnownHosts: "false",
			},
		},
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-creds",
				Namespace: namespace,
				Annotations: map[string]string{
					"tekton.dev/git-0": "github.com",
				},
			},
			Type: "kubernetes.io/ssh-auth",
			Data: map[string][]byte{
				"ssh-privatekey": []byte("Hello World!"),
			},
		},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(tc.configMap)
			err := checkGitSSHSecret(store.ToContext(context.Background()), tc.secret)

			if wantError := tc.wantErrorMsg != ""; wantError {
				if err == nil {
					t.Errorf("expected error %q, got nil", tc.wantErrorMsg)
				} else if diff := cmp.Diff(tc.wantErrorMsg, err.Error()); diff != "" {
					t.Errorf("unexpected (-want, +got) = %v", diff)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
