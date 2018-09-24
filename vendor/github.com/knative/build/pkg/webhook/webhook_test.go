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

package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mattbaird/jsonpatch"
	"go.uber.org/zap"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"

	"github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/builder/nop"
	fakebuildclientset "github.com/knative/build/pkg/client/clientset/versioned/fake"
	"github.com/knative/pkg/logging"
)

const (
	testNamespace         = "test-namespace"
	testBuildName         = "test-build"
	testBuildTemplateName = "test-build-template"
)

var (
	defaultOptions = ControllerOptions{
		ServiceName:      "build-webhook",
		ServiceNamespace: "knative-build",
		Port:             443,
		SecretName:       "build-webhook-certs",
		WebhookName:      "webhook.build.knative.dev",
	}
	testLogger = zap.NewNop().Sugar()
	testCtx    = logging.WithLogger(context.TODO(), testLogger)
)

func testBuild(name string) v1alpha1.Build {
	return v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: v1alpha1.BuildSpec{
			Steps: []corev1.Container{{Image: "hello"}},
		},
	}
}

func testBuildTemplate(name string) v1alpha1.BuildTemplate {
	return v1alpha1.BuildTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: v1alpha1.BuildTemplateSpec{},
	}
}

func testClusterBuildTemplate(name string) v1alpha1.ClusterBuildTemplate {
	return v1alpha1.ClusterBuildTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: v1alpha1.BuildTemplateSpec{},
	}
}

func mustMarshal(t *testing.T, in interface{}) []byte {
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return b
}

func mustUnmarshalPatches(t *testing.T, b []byte) []jsonpatch.JsonPatchOperation {
	var p []jsonpatch.JsonPatchOperation
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("Unmarshalling patches: %v", err)
	}
	return p
}

func TestAdmitBuild(t *testing.T) {
	for _, c := range []struct {
		desc        string
		op          admissionv1beta1.Operation
		kind        string
		wantAllowed bool
		new, old    v1alpha1.Build
		wantPatches []jsonpatch.JsonPatchOperation
	}{{
		desc:        "delete op",
		op:          admissionv1beta1.Delete,
		wantAllowed: true,
	}, {
		desc:        "connect op",
		op:          admissionv1beta1.Connect,
		wantAllowed: true,
	}, {
		desc:        "bad kind",
		op:          admissionv1beta1.Create,
		kind:        "Garbage",
		wantAllowed: false,
	}, {
		desc:        "invalid name",
		op:          admissionv1beta1.Create,
		kind:        "Build",
		new:         testBuild("build.invalid"),
		wantAllowed: false,
	}, {
		desc:        "invalid name too long",
		op:          admissionv1beta1.Create,
		kind:        "Build",
		new:         testBuild(strings.Repeat("a", 64)),
		wantAllowed: false,
	}, {
		desc:        "create valid",
		op:          admissionv1beta1.Create,
		kind:        "Build",
		new:         testBuild("valid-build"),
		wantAllowed: true,
		wantPatches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      "/spec/generation",
			Value:     float64(1),
		}},
	}, {
		desc:        "no-op update",
		op:          admissionv1beta1.Update,
		kind:        "Build",
		old:         testBuild("valid-build"),
		new:         testBuild("valid-build"),
		wantAllowed: true,
		wantPatches: nil,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			client := fakekubeclientset.NewSimpleClientset()
			// Create the default ServiceAccount that the Build uses.
			if _, err := client.CoreV1().ServiceAccounts(c.new.Namespace).Create(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
			}); err != nil {
				t.Fatalf("Failed to create ServiceAccount: %v", err)
			}
			ac := NewAdmissionController(client, fakebuildclientset.NewSimpleClientset(), &nop.Builder{}, defaultOptions, testLogger)
			resp := ac.admit(ctx, &admissionv1beta1.AdmissionRequest{
				Operation: c.op,
				Kind:      metav1.GroupVersionKind{Kind: c.kind},
				OldObject: runtime.RawExtension{Raw: mustMarshal(t, c.old)},
				Object:    runtime.RawExtension{Raw: mustMarshal(t, c.new)},
			})
			if resp.Allowed != c.wantAllowed {
				t.Errorf("allowed got %t, want %t: %+v", resp.Allowed, c.wantAllowed, resp.Result)
			}
			if c.wantPatches != nil {
				gotPatches := mustUnmarshalPatches(t, resp.Patch)
				if diff := cmp.Diff(gotPatches, c.wantPatches); diff != "" {
					t.Errorf("patches differed: %s", diff)
				}
			}
		})
	}
}

func TestValidateBuild(t *testing.T) {
	ctx := context.Background()
	hasDefault := "has-default"
	empty := ""
	for _, c := range []struct {
		desc    string
		build   *v1alpha1.Build
		tmpl    *v1alpha1.BuildTemplate
		ctmpl   *v1alpha1.ClusterBuildTemplate
		sa      *corev1.ServiceAccount
		secrets []*corev1.Secret
		reason  string // if "", expect success.
	}{{
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		reason: "negative build timeout",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Timeout: metav1.Duration{Duration: -48 * time.Hour},
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},	
	}, {
		reason: "maximum timeout",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Timeout: metav1.Duration{Duration: 48 * time.Hour},
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Timeout: metav1.Duration{Duration: 5 * time.Minute},
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		desc: "Multiple unnamed steps",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:oops",
				}},
			},
		},
		reason: "DuplicateStepName",
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Steps: []corev1.Container{{Name: "foo"}},
			},
		},
		reason: "StepMissingImage",
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
				Volumes: []corev1.Volume{{
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}, {
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}},
			},
		},
		reason: "DuplicateVolumeName",
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{
					Arguments: []v1alpha1.ArgumentSpec{{
						Name:  "foo",
						Value: "hello",
					}, {
						Name:  "foo",
						Value: "world",
					}},
				},
			},
		},
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "foo",
				}},
			},
		},
		reason: "DuplicateArgName",
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{
					Name: "foo-bar",
				},
				Volumes: []corev1.Volume{{
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}},
			},
		},
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
				Volumes: []corev1.Volume{{
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}},
			},
		},
		reason: "DuplicateVolumeName",
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
				Template: &v1alpha1.TemplateInstantiationSpec{
					Name: "template",
				},
			},
		},
		reason: "TemplateAndSteps",
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{
					Name: "template",
					Arguments: []v1alpha1.ArgumentSpec{{
						Name:  "foo",
						Value: "hello",
					}},
				},
			},
		},
		tmpl: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "template"},
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
		},
		reason: "UnsatisfiedParameter",
	}, {
		desc: "Arg doesn't match any parameter",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{
					Name: "template",
					Arguments: []v1alpha1.ArgumentSpec{{
						Name:  "bar",
						Value: "hello",
					}},
				},
			},
		},
		tmpl: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "template"},
			Spec:       v1alpha1.BuildTemplateSpec{},
		},
	}, {
		desc: "Unsatisfied parameter has a default",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{
					Name: "template",
					Arguments: []v1alpha1.ArgumentSpec{{
						Name:  "foo",
						Value: "hello",
					}},
				},
			},
		},
		tmpl: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "template"},
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "foo",
				}, {
					Name:    "bar",
					Default: &hasDefault,
				}},
			},
		},
	}, {
		desc: "Unsatisfied parameter has empty default",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{
					Name: "empty-default",
					Kind: "BuildTemplate",
				},
			},
		},
		tmpl: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "empty-default"},
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name:    "foo",
					Default: &empty,
				}},
			},
		},
	}, {
		desc: "build template cluster",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{
					Name: "empty-default",
					Kind: "ClusterBuildTemplate",
				},
			},
		},
		ctmpl: &v1alpha1.ClusterBuildTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "empty-default"},
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name:    "foo",
					Default: &empty,
				}},
			},
		},
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				Template: &v1alpha1.TemplateInstantiationSpec{},
			},
		},
		reason: "MissingTemplateName",
	}, {
		desc: "Acceptable secret annotations",
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				// ServiceAccountName will default to "default"
				Steps: []corev1.Container{{Image: "hello"}},
			},
		},
		sa: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "default"},
			Secrets: []corev1.ObjectReference{
				{Name: "good-sekrit"},
				{Name: "another-good-sekrit"},
				{Name: "one-more-good-sekrit"},
				{Name: "last-one-promise"},
			},
		},
		secrets: []*corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "good-sekrit",
				Annotations: map[string]string{"build.dev/docker-0": "https://index.docker.io/v1/"},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "another-good-sekrit",
				Annotations: map[string]string{"unrelated": "index.docker.io"},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "one-more-good-sekrit",
				Annotations: map[string]string{"build.dev/docker-1": "gcr.io"},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "last-one-promise",
				Annotations: map[string]string{"docker-0": "index.docker.io"},
			},
		}},
	}, {
		build: &v1alpha1.Build{
			Spec: v1alpha1.BuildSpec{
				ServiceAccountName: "serviceaccount",
				Steps:              []corev1.Container{{Image: "hello"}},
			},
		},
		sa: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "serviceaccount"},
			Secrets:    []corev1.ObjectReference{{Name: "bad-sekrit"}},
		},
		secrets: []*corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bad-sekrit",
				Annotations: map[string]string{"build.dev/docker-0": "index.docker.io"},
			},
		}},
		reason: "BadSecretAnnotation",
	}} {
		name := c.desc
		if c.reason != "" {
			name = "invalid-" + c.reason
		}
		t.Run(name, func(t *testing.T) {
			client := fakekubeclientset.NewSimpleClientset()
			buildClient := fakebuildclientset.NewSimpleClientset()
			// Create a BuildTemplate.
			if c.tmpl != nil {
				if _, err := buildClient.BuildV1alpha1().BuildTemplates("").Create(c.tmpl); err != nil {
					t.Fatalf("Failed to create BuildTemplate: %v", err)
				}
			} else if c.ctmpl != nil {
				if _, err := buildClient.BuildV1alpha1().ClusterBuildTemplates().Create(c.ctmpl); err != nil {
					t.Fatalf("Failed to create ClusterBuildTemplate: %v", err)
				}
			}
			// Create ServiceAccount or create the default ServiceAccount.
			if c.sa != nil {
				if _, err := client.CoreV1().ServiceAccounts(c.sa.Namespace).Create(c.sa); err != nil {
					t.Fatalf("Failed to create ServiceAccount: %v", err)
				}
			} else {
				if _, err := client.CoreV1().ServiceAccounts("").Create(&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: "default"},
				}); err != nil {
					t.Fatalf("Failed to create ServiceAccount: %v", err)
				}
			}
			// Create any necessary Secrets.
			for _, s := range c.secrets {
				if _, err := client.CoreV1().Secrets("").Create(s); err != nil {
					t.Fatalf("Failed to create Secret %q: %v", s.Name, err)
				}
			}

			ac := NewAdmissionController(client, buildClient, &nop.Builder{}, defaultOptions, testLogger)
			verr := ac.validateBuild(ctx, nil, nil, c.build)
			if gotErr, wantErr := verr != nil, c.reason != ""; gotErr != wantErr {
				t.Errorf("validateBuild(%s); got %v, want %q", name, verr, c.reason)
			}
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	ctx := context.Background()
	hasDefault := "has-default"
	for _, c := range []struct {
		desc   string
		tmpl   *v1alpha1.BuildTemplate
		reason string // if "", expect success.
	}{{
		desc: "Single named step",
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		desc: "Multiple unnamed steps",
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:oops",
				}},
			},
		},
		reason: "DuplicateStepName",
	}, {
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
				Volumes: []corev1.Volume{{
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}, {
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}},
			},
		},
		reason: "DuplicateVolumeName",
	}, {
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "foo",
				}, {
					Name: "foo",
				}},
			},
		},
		reason: "DuplicateParamName",
	}, {
		tmpl: &v1alpha1.BuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name: "step-name-${FOO${BAR}}",
				}},
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "FOO",
				}, {
					Name:    "BAR",
					Default: &hasDefault,
				}},
			},
		},
		reason: "NestedPlaceholder",
	}} {
		name := c.desc
		if c.reason != "" {
			name = "invalid-" + c.reason
		}
		t.Run(name, func(t *testing.T) {
			ac := NewAdmissionController(fakekubeclientset.NewSimpleClientset(), fakebuildclientset.NewSimpleClientset(), &nop.Builder{}, defaultOptions, testLogger)
			verr := ac.validateBuildTemplate(ctx, nil, nil, c.tmpl)
			if gotErr, wantErr := verr != nil, c.reason != ""; gotErr != wantErr {
				t.Errorf("validateBuildTemplate(%s); got %v, want %q", name, verr, c.reason)
			}
		})
	}
}

func TestValidateClusterBuildTemplate(t *testing.T) {
	ctx := context.Background()
	hasDefault := "has-default"
	for _, c := range []struct {
		desc   string
		tmpl   *v1alpha1.ClusterBuildTemplate
		reason string // if "", expect success.
	}{{
		desc: "Single named step",
		tmpl: &v1alpha1.ClusterBuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		desc: "Multiple unnamed steps",
		tmpl: &v1alpha1.ClusterBuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Image: "gcr.io/foo-bar/baz:latest",
				}},
			},
		},
	}, {
		tmpl: &v1alpha1.ClusterBuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}, {
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:oops",
				}},
			},
		},
		reason: "DuplicateStepName",
	}, {
		tmpl: &v1alpha1.ClusterBuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name:  "foo",
					Image: "gcr.io/foo-bar/baz:latest",
				}},
				Volumes: []corev1.Volume{{
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}, {
					Name: "foo",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}},
			},
		},
		reason: "DuplicateVolumeName",
	}, {
		tmpl: &v1alpha1.ClusterBuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "foo",
				}, {
					Name: "foo",
				}},
			},
		},
		reason: "DuplicateParamName",
	}, {
		tmpl: &v1alpha1.ClusterBuildTemplate{
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Name: "step-name-${FOO${BAR}}",
				}},
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "FOO",
				}, {
					Name:    "BAR",
					Default: &hasDefault,
				}},
			},
		},
		reason: "NestedPlaceholder",
	}} {
		name := c.desc
		if c.reason != "" {
			name = "invalid-" + c.reason
		}
		t.Run(name, func(t *testing.T) {
			ac := NewAdmissionController(fakekubeclientset.NewSimpleClientset(), fakebuildclientset.NewSimpleClientset(), &nop.Builder{}, defaultOptions, testLogger)
			verr := ac.validateClusterBuildTemplate(ctx, nil, nil, c.tmpl)
			if gotErr, wantErr := verr != nil, c.reason != ""; gotErr != wantErr {
				t.Errorf("validateBuildTemplate(%s); got %v, want %q", name, verr, c.reason)
			}
		})
	}
}

func TestAdmitBuildTemplate(t *testing.T) {
	for _, c := range []struct {
		desc        string
		op          admissionv1beta1.Operation
		kind        string
		wantAllowed bool
		new, old    v1alpha1.BuildTemplate
		wantPatches []jsonpatch.JsonPatchOperation
	}{{
		desc:        "delete op",
		op:          admissionv1beta1.Delete,
		wantAllowed: true,
	}, {
		desc:        "connect op",
		op:          admissionv1beta1.Connect,
		wantAllowed: true,
	}, {
		desc:        "bad kind",
		op:          admissionv1beta1.Create,
		kind:        "Garbage",
		wantAllowed: false,
	}, {
		desc:        "invalid name",
		op:          admissionv1beta1.Create,
		kind:        "BuildTemplate",
		new:         testBuildTemplate("build-template.invalid"),
		wantAllowed: false,
	}, {
		desc:        "invalid name too long",
		op:          admissionv1beta1.Create,
		kind:        "BuildTemplate",
		new:         testBuildTemplate(strings.Repeat("a", 64)),
		wantAllowed: false,
	}, {
		desc:        "create valid",
		op:          admissionv1beta1.Create,
		kind:        "BuildTemplate",
		new:         testBuildTemplate("valid-build-template"),
		wantAllowed: true,
		wantPatches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      "/spec/generation",
			Value:     float64(1),
		}},
	}, {
		desc:        "no-op update",
		op:          admissionv1beta1.Update,
		kind:        "BuildTemplate",
		old:         testBuildTemplate("valid-build-template"),
		new:         testBuildTemplate("valid-build-template"),
		wantAllowed: true,
		wantPatches: nil,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			ac := NewAdmissionController(fakekubeclientset.NewSimpleClientset(), fakebuildclientset.NewSimpleClientset(), &nop.Builder{}, defaultOptions, testLogger)
			resp := ac.admit(ctx, &admissionv1beta1.AdmissionRequest{
				Operation: c.op,
				Kind:      metav1.GroupVersionKind{Kind: c.kind},
				OldObject: runtime.RawExtension{Raw: mustMarshal(t, c.old)},
				Object:    runtime.RawExtension{Raw: mustMarshal(t, c.new)},
			})
			if resp.Allowed != c.wantAllowed {
				t.Errorf("allowed got %t, want %t", resp.Allowed, c.wantAllowed)
			}
			if c.wantPatches != nil {
				gotPatches := mustUnmarshalPatches(t, resp.Patch)
				if diff := cmp.Diff(gotPatches, c.wantPatches); diff != "" {
					t.Errorf("patches differed: %s", diff)
				}
			}
		})
	}
}

func TestAdmitClusterBuildTemplate(t *testing.T) {
	for _, c := range []struct {
		desc        string
		op          admissionv1beta1.Operation
		kind        string
		wantAllowed bool
		new, old    v1alpha1.ClusterBuildTemplate
		wantPatches []jsonpatch.JsonPatchOperation
	}{{
		desc:        "delete op",
		op:          admissionv1beta1.Delete,
		wantAllowed: true,
	}, {
		desc:        "connect op",
		op:          admissionv1beta1.Connect,
		wantAllowed: true,
	}, {
		desc:        "bad kind",
		op:          admissionv1beta1.Create,
		kind:        "Garbage",
		wantAllowed: false,
	}, {
		desc:        "invalid name",
		op:          admissionv1beta1.Create,
		kind:        "ClusterBuildTemplate",
		new:         testClusterBuildTemplate("build-template.invalid"),
		wantAllowed: false,
	}, {
		desc:        "invalid name too long",
		op:          admissionv1beta1.Create,
		kind:        "ClusterBuildTemplate",
		new:         testClusterBuildTemplate(strings.Repeat("a", 64)),
		wantAllowed: false,
	}, {
		desc:        "create valid",
		op:          admissionv1beta1.Create,
		kind:        "ClusterBuildTemplate",
		new:         testClusterBuildTemplate("valid-build-template"),
		wantAllowed: true,
		wantPatches: []jsonpatch.JsonPatchOperation{{
			Operation: "add",
			Path:      "/spec/generation",
			Value:     float64(1),
		}},
	}, {
		desc:        "no-op update",
		op:          admissionv1beta1.Update,
		kind:        "ClusterBuildTemplate",
		old:         testClusterBuildTemplate("valid-build-template"),
		new:         testClusterBuildTemplate("valid-build-template"),
		wantAllowed: true,
		wantPatches: nil,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			ac := NewAdmissionController(fakekubeclientset.NewSimpleClientset(), fakebuildclientset.NewSimpleClientset(), &nop.Builder{}, defaultOptions, testLogger)
			resp := ac.admit(ctx, &admissionv1beta1.AdmissionRequest{
				Operation: c.op,
				Kind:      metav1.GroupVersionKind{Kind: c.kind},
				OldObject: runtime.RawExtension{Raw: mustMarshal(t, c.old)},
				Object:    runtime.RawExtension{Raw: mustMarshal(t, c.new)},
			})
			if resp.Allowed != c.wantAllowed {
				t.Errorf("allowed got %t, want %t", resp.Allowed, c.wantAllowed)
			}
			if c.wantPatches != nil {
				gotPatches := mustUnmarshalPatches(t, resp.Patch)
				if diff := cmp.Diff(gotPatches, c.wantPatches); diff != "" {
					t.Errorf("patches differed: %s", diff)
				}
			}
		})
	}
}
