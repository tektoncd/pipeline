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

package verifier

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sigstore/sigstore/pkg/signature"
	fakekms "github.com/sigstore/sigstore/pkg/signature/kms/fake"
	gcpkms "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

const (
	namespace = "trusted-resources"
)

func TestFromPolicy_Success(t *testing.T) {
	ctx := context.Background()
	_, key256, k8sclient, vps := test.SetupVerificationPolicies(t)
	keyInDataVp, keyInSecretVp := vps[0], vps[1]

	keyInKMSVp := &v1alpha1.VerificationPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VerificationPolicy",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keyInKMSVp",
			Namespace: namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{},
			Authorities: []v1alpha1.Authority{
				{
					Name: "kms",
					Key: &v1alpha1.KeyRef{
						KMS:           "fakekms://key",
						HashAlgorithm: "sha256",
					},
				},
			},
		},
	}
	ctx = context.WithValue(context.TODO(), fakekms.KmsCtxKey{}, key256)

	_, key384, pub, err := test.GenerateKeys(elliptic.P384(), crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to generate keys %v", err)
	}

	sha384Vp := &v1alpha1.VerificationPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VerificationPolicy",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "differentAlgo",
			Namespace: namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{},
			Authorities: []v1alpha1.Authority{
				{
					Name: "sha384Key",
					Key: &v1alpha1.KeyRef{
						Data:          string(pub),
						HashAlgorithm: "sha384",
					},
				},
			},
		},
	}

	tcs := []struct {
		name   string
		policy *v1alpha1.VerificationPolicy
		key    *ecdsa.PrivateKey
	}{{
		name:   "key in data",
		policy: keyInDataVp,
		key:    key256,
	}, {
		name:   "key in secret",
		policy: keyInSecretVp,
		key:    key256,
	}, {
		name:   "key in kms",
		policy: keyInKMSVp,
		key:    key256,
	}, {
		name:   "key with sha384",
		policy: sha384Vp,
		key:    key384,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			verifiers, err := FromPolicy(ctx, k8sclient, tc.policy)
			for _, v := range verifiers {
				checkVerifier(t, tc.key, v)
			}
			if err != nil {
				t.Errorf("couldn't construct expected verifier from VerificationPolicy: %v", err)
			}
		})
	}
}

func TestFromPolicy_Error(t *testing.T) {
	tcs := []struct {
		name          string
		policy        *v1alpha1.VerificationPolicy
		expectedError error
	}{{
		name: "hash algorithm is invalid",
		policy: &v1alpha1.VerificationPolicy{
			Spec: v1alpha1.VerificationPolicySpec{
				Authorities: []v1alpha1.Authority{
					{
						Key: &v1alpha1.KeyRef{
							Data:          "inlinekey",
							HashAlgorithm: "sha1",
						},
					},
				},
			},
		},
		expectedError: ErrAlgorithmInvalid,
	}, {
		name: "key is empty",
		policy: &v1alpha1.VerificationPolicy{
			Spec: v1alpha1.VerificationPolicySpec{
				Authorities: []v1alpha1.Authority{
					{
						Key: &v1alpha1.KeyRef{},
					},
				},
			},
		},
		expectedError: ErrEmptyKey,
	}, {
		name: "authority is empty",
		policy: &v1alpha1.VerificationPolicy{
			Spec: v1alpha1.VerificationPolicySpec{
				Authorities: []v1alpha1.Authority{},
			},
		},
		expectedError: ErrEmptyPublicKeys,
	}, {
		name: "from data error",
		policy: &v1alpha1.VerificationPolicy{
			Spec: v1alpha1.VerificationPolicySpec{
				Authorities: []v1alpha1.Authority{
					{
						Key: &v1alpha1.KeyRef{
							Data:          "inlinekey",
							HashAlgorithm: "sha256",
						},
					},
				},
			},
		},
		expectedError: ErrDecodeKey,
	}, {
		name: "from secret error",
		policy: &v1alpha1.VerificationPolicy{
			Spec: v1alpha1.VerificationPolicySpec{
				Authorities: []v1alpha1.Authority{
					{
						Key: &v1alpha1.KeyRef{
							SecretRef: &v1.SecretReference{
								Name:      "wrongSecret",
								Namespace: "wrongNamespace",
							},
						},
					},
				},
			},
		},
		expectedError: ErrSecretNotFound,
	}, {
		name: "from kms error",
		policy: &v1alpha1.VerificationPolicy{
			Spec: v1alpha1.VerificationPolicySpec{
				Authorities: []v1alpha1.Authority{
					{
						Key: &v1alpha1.KeyRef{
							KMS: "gcpkms://wrongurl",
						},
					},
				},
			},
		},
		expectedError: gcpkms.ValidReference("gcpkms://wrongurl"),
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromPolicy(context.Background(), fakek8s.NewSimpleClientset(), tc.policy)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("FromPolicy got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestFromKeyRef_Success(t *testing.T) {
	ctx := context.Background()
	fileKey, keypath := test.GetKeysFromFile(ctx, t)

	_, secretKey, pub, err := test.GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	secretData := &v1.Secret{
		Data: map[string][]byte{
			"data": pub,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: namespace}}

	k8sclient := fakek8s.NewSimpleClientset(secretData)

	tcs := []struct {
		name   string
		keyref string
		key    *ecdsa.PrivateKey
	}{{
		name:   "key in file",
		keyref: keypath,
		key:    fileKey,
	}, {
		name:   "key in secret",
		keyref: fmt.Sprintf("k8s://%s/secret", namespace),
		key:    secretKey,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			verifier, err := fromKeyRef(ctx, tc.keyref, crypto.SHA256, k8sclient)
			checkVerifier(t, tc.key, verifier)
			if err != nil {
				t.Errorf("couldn't construct expected verifier from keyref: %v", err)
			}
		})
	}
}

func TestFromKeyRef_Error(t *testing.T) {
	ctx := context.Background()
	_, keypath := test.GetKeysFromFile(ctx, t)
	tcs := []struct {
		name          string
		keyref        string
		algorithm     crypto.Hash
		expectedError error
	}{{
		name:          "failed to read file",
		keyref:        "wrongPath",
		algorithm:     crypto.SHA256,
		expectedError: ErrFailedLoadKeyFile,
	}, {
		name:          "failed to read from secret",
		keyref:        fmt.Sprintf("k8s://%s/not-exist-secret", namespace),
		algorithm:     crypto.SHA256,
		expectedError: ErrSecretNotFound,
	}, {
		name:          "failed to read from data",
		keyref:        keypath,
		algorithm:     crypto.BLAKE2b_256,
		expectedError: ErrLoadVerifier,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fromKeyRef(ctx, tc.keyref, tc.algorithm, fakek8s.NewSimpleClientset())
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("fromKeyRef got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestFromSecret_Success(t *testing.T) {
	_, keys, pub, err := test.GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	secretData := &v1.Secret{
		Data: map[string][]byte{
			"data": pub,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: namespace}}

	k8sclient := fakek8s.NewSimpleClientset(secretData)

	v, err := fromSecret(context.Background(), fmt.Sprintf("k8s://%s/secret", namespace), crypto.SHA256, k8sclient)
	checkVerifier(t, keys, v)
	if err != nil {
		t.Errorf("couldn't construct expected verifier from secret: %v", err)
	}
}

func TestFromSecret_Error(t *testing.T) {
	secretNoData := &v1.Secret{
		Data: map[string][]byte{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-secret",
			Namespace: namespace}}
	secretMultipleData := &v1.Secret{
		Data: map[string][]byte{
			"data1": []byte("key"),
			"data2": []byte("key"),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiple-data-secret",
			Namespace: namespace}}
	secretInvalidData := &v1.Secret{
		Data: map[string][]byte{
			"data1": []byte("key"),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-data-secret",
			Namespace: namespace}}

	k8sclient := fakek8s.NewSimpleClientset(secretNoData, secretMultipleData, secretInvalidData)

	tcs := []struct {
		name          string
		secretref     string
		expectedError error
	}{{
		name:          "no data in secret",
		secretref:     fmt.Sprintf("k8s://%s/empty-secret", namespace),
		expectedError: ErrEmptySecretData,
	}, {
		name:          "multiple data in secret",
		secretref:     fmt.Sprintf("k8s://%s/multiple-data-secret", namespace),
		expectedError: ErrMultipleSecretData,
	}, {
		name:          "invalid data in secret",
		secretref:     fmt.Sprintf("k8s://%s/invalid-data-secret", namespace),
		expectedError: ErrDecodeKey,
	}, {
		name:          "invalid secretref",
		secretref:     "invalid-secretref",
		expectedError: ErrK8sSpecificationInvalid,
	}, {
		name:          "secretref has k8s prefix but contains invalid data",
		secretref:     "k8s://ns/name/foo",
		expectedError: ErrK8sSpecificationInvalid,
	}, {
		name:          "secret doesn't exist",
		secretref:     fmt.Sprintf("k8s://%s/not-exist-secret", namespace),
		expectedError: ErrSecretNotFound,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fromSecret(context.Background(), tc.secretref, crypto.SHA256, k8sclient)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("FromSecret got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestFromData_Error(t *testing.T) {
	_, _, pub, err := test.GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to generate keys %v", err)
	}
	tcs := []struct {
		name          string
		data          []byte
		algorithm     crypto.Hash
		expectedError error
	}{{
		name:          "data in cannot be decoded",
		data:          []byte("wrong key"),
		algorithm:     crypto.SHA256,
		expectedError: ErrDecodeKey,
	}, {
		name:          "verifier cannot be loaded due to wrong algorithm",
		data:          pub,
		algorithm:     crypto.BLAKE2b_256,
		expectedError: ErrLoadVerifier,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fromData(tc.data, tc.algorithm)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("fromData got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestMatchHashAlgorithm_Success(t *testing.T) {
	tcs := []struct {
		name      string
		algorithm v1alpha1.HashAlgorithm
		want      crypto.Hash
	}{{
		name:      "correct",
		algorithm: "SHA256",
		want:      crypto.SHA256,
	}, {
		name:      "lower case",
		algorithm: "Sha256",
		want:      crypto.SHA256,
	}, {
		name:      "empty should be defaulted to SHA256",
		algorithm: "",
		want:      crypto.SHA256,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := matchHashAlgorithm(tc.algorithm)
			if err != nil {
				t.Errorf("failed to get hash algorithm: %v", err)
			}
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestHashAlgorithm_Error(t *testing.T) {
	_, err := matchHashAlgorithm("SHA1")
	if !errors.Is(err, ErrAlgorithmInvalid) {
		t.Errorf("hashAlgorithm got: %v, want: %v", err, ErrAlgorithmInvalid)
	}
}

// checkVerifier checks if the keys public key is equal to the verifier's public key
func checkVerifier(t *testing.T, keys *ecdsa.PrivateKey, verifier signature.Verifier) {
	t.Helper()
	p, _ := verifier.PublicKey()
	if !keys.PublicKey.Equal(p) {
		t.Errorf("got wrong verifier %v", verifier)
	}
}
