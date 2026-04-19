/*
Copyright 2022 The Tekton Authors
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

package v1alpha1_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func withExactMatchFlag(t *testing.T, enabled bool) context.Context {
	t.Helper()

	featureFlags, err := config.NewFeatureFlagsFromMap(map[string]string{
		config.EnableVerificationExactMatchPolicy: strconv.FormatBool(enabled),
	})
	if err != nil {
		t.Fatalf("creating feature flags: %v", err)
	}

	cfg := &config.Config{FeatureFlags: featureFlags}
	return config.ToContext(context.Background(), cfg)
}

func patternResource(pattern string) v1alpha1.ResourcePattern {
	return v1alpha1.ResourcePattern{pattern, ""}
}

func exactMatchResource(url string) v1alpha1.ResourcePattern {
	return v1alpha1.ResourcePattern{"", url}
}

func TestVerificationPolicy_Invalid(t *testing.T) {
	tests := []struct {
		name               string
		verificationPolicy *v1alpha1.VerificationPolicy
		want               *apis.FieldError
		withContext        func(context.Context) context.Context
	}{{
		name: "missing Resources",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data: "inline_key",
						},
					},
				},
			},
		},
		want: apis.ErrMissingField("resources"),
	}, {
		name: "invalid ResourcePattern",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource("^[")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data: "inline_key",
						},
					},
				},
			},
		},
		want: apis.ErrInvalidValue("^[", "ResourcePattern", fmt.Sprintf("%v: error parsing regexp: missing closing ]: `[`", v1alpha1.InvalidResourcePatternErr)).ViaFieldIndex("resources", 0),
	}, {
		name: "missing Authoritities",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
			},
		},
		want: apis.ErrMissingField("authorities"),
	}, {
		name: "wrong mode",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data: "inlinekey",
						},
					},
				},
				Mode: "wrongMode",
			},
		},
		want: apis.ErrInvalidValue(fmt.Sprintf("available values are: %s, %s, but got: %s", v1alpha1.ModeEnforce, v1alpha1.ModeWarn, "wrongMode"), "mode"),
	}, {
		name: "missing Authority key",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key:  &v1alpha1.KeyRef{},
					},
				},
			},
		},
		want: apis.ErrMissingOneOf("data", "kms", "secretref").ViaFieldIndex("key", 0),
	}, {
		name: "should not have both data and secretref",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data: "inlinekey",
							SecretRef: &corev1.SecretReference{
								Name: "name",
							},
						},
					},
				},
			},
		},
		want: apis.ErrMultipleOneOf("data", "kms", "secretref").ViaFieldIndex("key", 0),
	}, {
		name: "should not have both data and KMS",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data: "inlinekey",
							KMS:  "kms://key/path",
						},
					},
				},
			},
		},
		want: apis.ErrMultipleOneOf("data", "kms", "secretref").ViaFieldIndex("key", 0),
	}, {
		name: "should not have both secretref and KMS",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							SecretRef: &corev1.SecretReference{
								Name:      "name",
								Namespace: "namespace",
							},
							KMS: "kms://key/path",
						},
					},
				},
			},
		},
		want: apis.ErrMultipleOneOf("data", "kms", "secretref").ViaFieldIndex("key", 0),
	}, {
		name: "should not have data, secretref and KMS at the same time",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data: "inlinekey",
							SecretRef: &corev1.SecretReference{
								Name:      "name",
								Namespace: "namespace",
							},
							KMS: "kms://key/path",
						},
					},
				},
			},
		},
		want: apis.ErrMultipleOneOf("data", "kms", "secretref").ViaFieldIndex("key", 0),
	}, {
		name: "invalid hash algorithm",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data:          "inlinekey",
							HashAlgorithm: "sha1",
						},
					},
				},
			},
		},
		want: apis.ErrInvalidValue("sha1", "HashAlgorithm").ViaFieldIndex("key", 0),
	}, {
		name: "exactMatch requires feature flag",
		withContext: func(context.Context) context.Context {
			return withExactMatchFlag(t, false)
		},
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{exactMatchResource("https://github.com/tektoncd/catalog.git")},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key: &v1alpha1.KeyRef{
							Data: "inlinekey",
						},
					},
				},
			},
		},
		want: apis.ErrGeneric(
			fmt.Sprintf("feature flag %s should be set to true to use exactMatch", config.EnableVerificationExactMatchPolicy),
			"exactMatch",
		).ViaFieldIndex("resources", 0),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.withContext != nil {
				ctx = tt.withContext(ctx)
			}
			err := tt.verificationPolicy.Validate(ctx)
			if d := cmp.Diff(tt.want.Error(), err.Error()); d != "" {
				t.Error("VerificationPolicy validate error mismatch", diff.PrintWantGot(d))
			}
		})
	}
}

func TestVerificationPolicy_Valid(t *testing.T) {
	tests := []struct {
		name               string
		verificationPolicy *v1alpha1.VerificationPolicy
		withContext        func(context.Context) context.Context
	}{
		{
			name: "key in data",
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
					Authorities: []v1alpha1.Authority{
						{
							Name: "foo",
							Key: &v1alpha1.KeyRef{
								Data:          "inlinekey",
								HashAlgorithm: "sha256",
							},
						},
					},
				},
			},
		},
		{
			name: "exact match resource pattern",
			withContext: func(context.Context) context.Context {
				return withExactMatchFlag(t, true)
			},
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{exactMatchResource("https://github.com/tektoncd/catalog.git")},
					Authorities: []v1alpha1.Authority{
						{
							Name: "foo",
							Key: &v1alpha1.KeyRef{
								Data:          "inlinekey",
								HashAlgorithm: "sha256",
							},
						},
					},
				},
			},
		},
		{
			name: "key in secretref",
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
					Authorities: []v1alpha1.Authority{
						{
							Name: "foo",
							Key: &v1alpha1.KeyRef{
								SecretRef: &corev1.SecretReference{
									Name: "name",
								},
								HashAlgorithm: "sha256",
							},
						},
					},
				},
			},
		}, {
			name: "key in KMS",
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
					Authorities: []v1alpha1.Authority{
						{
							Name: "foo",
							Key: &v1alpha1.KeyRef{
								KMS: "kms://key/path",
							},
						},
					},
				},
			},
		}, {
			name: "enforce mode",
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
					Authorities: []v1alpha1.Authority{
						{
							Name: "foo",
							Key: &v1alpha1.KeyRef{
								KMS: "kms://key/path",
							},
						},
					},
					Mode: v1alpha1.ModeEnforce,
				},
			},
		}, {
			name: "warn mode",
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{patternResource(".*")},
					Authorities: []v1alpha1.Authority{
						{
							Name: "foo",
							Key: &v1alpha1.KeyRef{
								KMS: "kms://key/path",
							},
						},
					},
					Mode: v1alpha1.ModeWarn,
				},
			},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.withContext != nil {
				ctx = tt.withContext(ctx)
			}
			err := tt.verificationPolicy.Validate(ctx)
			if err != nil {
				t.Errorf("validating valid VerificationPolicy: %v", err)
			}
		})
	}
}
