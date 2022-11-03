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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestVerificationPolicy_Invalid(t *testing.T) {
	tests := []struct {
		name               string
		verificationPolicy *v1alpha1.VerificationPolicy
		want               *apis.FieldError
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
				Resources: []v1alpha1.ResourcePattern{{"^["}},
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
		want: apis.ErrInvalidValue("^[", "ResourcePattern", fmt.Sprintf("%v: error parsing regexp: missing closing ]: `[`", v1alpha1.InvalidResourcePatternErr)),
	}, {
		name: "missing Authoritities",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{{".*"}},
			},
		},
		want: apis.ErrMissingField("authorities"),
	}, {
		name: "missing Authority key",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{{".*"}},
				Authorities: []v1alpha1.Authority{
					{
						Name: "foo",
						Key:  &v1alpha1.KeyRef{},
					},
				},
			},
		},
		want: apis.ErrMissingOneOf("key[0].data", "key[0].secretref"),
	}, {
		name: "should not have both data and secretref",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{{".*"}},
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
		want: apis.ErrMultipleOneOf("key[0].data", "key[0].secretref"),
	}, {
		name: "invalid hash algorithm",
		verificationPolicy: &v1alpha1.VerificationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vp",
			},
			Spec: v1alpha1.VerificationPolicySpec{
				Resources: []v1alpha1.ResourcePattern{{".*"}},
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
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.verificationPolicy.Validate(context.Background())
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
	}{
		{
			name: "key in data",
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{{".*"}},
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
		}, {
			name: "key in secretref",
			verificationPolicy: &v1alpha1.VerificationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{{".*"}},
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
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.verificationPolicy.Validate(context.Background())
			if err != nil {
				t.Errorf("validating valid VerificationPolicy: %v", err)
			}
		})
	}
}
