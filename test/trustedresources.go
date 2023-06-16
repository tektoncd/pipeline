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

package test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/logging"
)

// TODO(#5820): refactor those into an internal pkg
const (
	namespace = "trusted-resources"
	// signatureAnnotation is the key of signature in annotation map
	signatureAnnotation = "tekton.dev/signature"
)

var (
	read = readPasswordFn
)

// GetUnsignedTask returns unsigned task with given name
func GetUnsignedTask(name string) *v1beta1.Task {
	return &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "ubuntu",
				Name:  "echo",
			}},
		},
	}
}

// GetUnsignedPipeline returns unsigned pipeline with given name
func GetUnsignedPipeline(name string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Pipeline"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "task",
				},
			},
		},
	}
}

// SetupTrustedResourceConfig configures the trusted-resources-verification-no-match-policy feature flag with the given mode for testing
func SetupTrustedResourceConfig(ctx context.Context, verificationNoMatchPolicy string) context.Context {
	store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
	featureflags := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "feature-flags",
		},
		Data: map[string]string{
			"trusted-resources-verification-no-match-policy": verificationNoMatchPolicy,
		},
	}
	store.OnConfigChanged(featureflags)

	return store.ToContext(ctx)
}

// SetupVerificationPolicies set verification policies and secrets to store public keys.
// This function helps to setup 4 kinds of VerificationPolicies:
// 1. One public key in inline data
// 2. One public key in secret
// 3. the policy pattern doesn't match any resources
// 4. warn mode policy without keys
// SignerVerifier is returned to sign resources
// The k8s clientset is returned to fetch secret from it.
// VerificationPolicies are returned to fetch public keys
func SetupVerificationPolicies(t *testing.T) (signature.SignerVerifier, *ecdsa.PrivateKey, *fakek8s.Clientset, []*v1alpha1.VerificationPolicy) {
	t.Helper()
	sv, keys, pub, err := GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to generate keys %v", err)
	}
	_, _, pub2, err := GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to generate keys %v", err)
	}

	secret := &corev1.Secret{
		Data: map[string][]byte{"cosign.pub": pub},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "verification-secrets",
			Namespace: namespace}}

	keyInDataVp := getVerificationPolicy(
		"keyInDataVp",
		namespace,
		[]v1alpha1.ResourcePattern{
			{Pattern: "https://github.com/tektoncd/catalog.git"},
		},
		[]v1alpha1.Authority{
			{
				Name: "pubkey",
				Key: &v1alpha1.KeyRef{
					Data:          string(pub),
					HashAlgorithm: "sha256",
				},
			},
		}, v1alpha1.ModeEnforce)

	keyInSecretVp := getVerificationPolicy(
		"keyInSecretVp",
		namespace,
		[]v1alpha1.ResourcePattern{{
			Pattern: "gcr.io/tekton-releases/catalog/upstream/git-clone"},
		},
		[]v1alpha1.Authority{
			{
				Name: "pubkey",
				Key: &v1alpha1.KeyRef{
					SecretRef: &corev1.SecretReference{
						Name:      secret.Name,
						Namespace: secret.Namespace,
					},
					HashAlgorithm: "sha256",
				},
			},
		}, v1alpha1.ModeEnforce)

	wrongKeyandPatternVp := getVerificationPolicy(
		"wrongKeyInDataVp",
		namespace,
		[]v1alpha1.ResourcePattern{
			{Pattern: "this should not match any resources"},
		},
		[]v1alpha1.Authority{
			{
				Name: "pubkey",
				Key: &v1alpha1.KeyRef{
					Data:          string(pub2),
					HashAlgorithm: "sha256",
				},
			},
		}, v1alpha1.ModeEnforce)

	warnModeVP := getVerificationPolicy(
		"warnModeVP",
		namespace,
		[]v1alpha1.ResourcePattern{{
			Pattern: "warnVP"},
		},
		[]v1alpha1.Authority{
			{
				Name: "pubkey",
				Key: &v1alpha1.KeyRef{
					SecretRef: &corev1.SecretReference{
						Name:      secret.Name,
						Namespace: secret.Namespace,
					},
					HashAlgorithm: "sha256",
				},
			},
		}, v1alpha1.ModeWarn)

	k8sclient := fakek8s.NewSimpleClientset(secret)

	return sv, keys, k8sclient, []*v1alpha1.VerificationPolicy{&keyInDataVp, &keyInSecretVp, &wrongKeyandPatternVp, &warnModeVP}
}

// SetupMatchAllVerificationPolicies set verification policies with a Pattern to match all resources
// SignerVerifier is returned to sign resources
// The k8s clientset is returned to fetch secret from it.
// VerificationPolicies are returned to fetch public keys
func SetupMatchAllVerificationPolicies(t *testing.T, namespace string) (signature.SignerVerifier, *fakek8s.Clientset, []*v1alpha1.VerificationPolicy) {
	t.Helper()
	sv, _, pub, err := GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatalf("failed to generate keys %v", err)
	}

	secret := &corev1.Secret{
		Data: map[string][]byte{"cosign.pub": pub},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "verification-secrets",
			Namespace: namespace}}

	matchAllVp := getVerificationPolicy(
		"matchAllVp",
		namespace,
		[]v1alpha1.ResourcePattern{
			{Pattern: ".*"},
		},
		[]v1alpha1.Authority{
			{
				Name: "pubkey",
				Key: &v1alpha1.KeyRef{
					Data:          string(pub),
					HashAlgorithm: "sha256",
				},
			},
		}, v1alpha1.ModeEnforce)

	k8sclient := fakek8s.NewSimpleClientset(secret)

	return sv, k8sclient, []*v1alpha1.VerificationPolicy{&matchAllVp}
}

// GetSignerFromFile generates key files to tmpdir, return signer and pubkey path
func GetSignerFromFile(ctx context.Context, t *testing.T) (signature.Signer, string) {
	t.Helper()
	sv, _, pub, err := GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	pubKey := filepath.Join(tmpDir, "ecdsa.pub")
	if err := os.WriteFile(pubKey, pub, 0600); err != nil {
		t.Fatal(err)
	}

	return sv, pubKey
}

// GetKeysFromFile generates key files to tmpdir, return keys and pubkey path
func GetKeysFromFile(ctx context.Context, t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	_, keys, pub, err := GenerateKeys(elliptic.P256(), crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	pubKey := filepath.Join(tmpDir, "ecdsa.pub")
	if err := os.WriteFile(pubKey, pub, 0600); err != nil {
		t.Fatal(err)
	}

	return keys, pubKey
}

// GenerateKeys creates public key files, return the SignerVerifier
func GenerateKeys(c elliptic.Curve, hashFunc crypto.Hash) (signature.SignerVerifier, *ecdsa.PrivateKey, []byte, error) {
	keys, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	// Now do the public key
	pubBytes, err := cryptoutils.MarshalPublicKeyToPEM(keys.Public())
	if err != nil {
		return nil, nil, nil, err
	}

	sv, err := signature.LoadSignerVerifier(keys, hashFunc)
	if err != nil {
		return nil, nil, nil, err
	}

	return sv, keys, pubBytes, nil
}

// signInterface returns the encoded signature for the given object.
func signInterface(signer signature.Signer, i interface{}) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is nil")
	}
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write(b)

	sig, err := signer.SignMessage(bytes.NewReader(h.Sum(nil)))
	if err != nil {
		return nil, err
	}

	return sig, nil
}

// GetSignedV1beta1Pipeline signed the given pipeline and rename it with given name
func GetSignedV1beta1Pipeline(unsigned *v1beta1.Pipeline, signer signature.Signer, name string) (*v1beta1.Pipeline, error) {
	signedPipeline := unsigned.DeepCopy()
	signedPipeline.Name = name
	if signedPipeline.Annotations == nil {
		signedPipeline.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signedPipeline)
	if err != nil {
		return nil, err
	}
	signedPipeline.Annotations[signatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signedPipeline, nil
}

// GetSignedV1beta1Task signed the given task and rename it with given name
func GetSignedV1beta1Task(unsigned *v1beta1.Task, signer signature.Signer, name string) (*v1beta1.Task, error) {
	signedTask := unsigned.DeepCopy()
	signedTask.Name = name
	if signedTask.Annotations == nil {
		signedTask.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signedTask)
	if err != nil {
		return nil, err
	}
	signedTask.Annotations[signatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signedTask, nil
}

// GetSignedV1Pipeline signed the given pipeline and rename it with given name
func GetSignedV1Pipeline(unsigned *v1.Pipeline, signer signature.Signer, name string) (*v1.Pipeline, error) {
	signedPipeline := unsigned.DeepCopy()
	signedPipeline.Name = name
	if signedPipeline.Annotations == nil {
		signedPipeline.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signedPipeline)
	if err != nil {
		return nil, err
	}
	signedPipeline.Annotations[signatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signedPipeline, nil
}

// GetSignedV1Task signed the given task and rename it with given name
func GetSignedV1Task(unsigned *v1.Task, signer signature.Signer, name string) (*v1.Task, error) {
	signedTask := unsigned.DeepCopy()
	signedTask.Name = name
	if signedTask.Annotations == nil {
		signedTask.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signedTask)
	if err != nil {
		return nil, err
	}
	signedTask.Annotations[signatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signedTask, nil
}

func getPass(confirm bool) ([]byte, error) {
	read := read(confirm)
	return read()
}
func readPasswordFn(confirm bool) func() ([]byte, error) {
	pw, ok := os.LookupEnv("PRIVATE_PASSWORD")
	if ok {
		return func() ([]byte, error) {
			return []byte(pw), nil
		}
	}
	return func() ([]byte, error) {
		return nil, fmt.Errorf("fail to get password")
	}
}

func getVerificationPolicy(name, namespace string, patterns []v1alpha1.ResourcePattern, authorities []v1alpha1.Authority, mode v1alpha1.ModeType) v1alpha1.VerificationPolicy {
	return v1alpha1.VerificationPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VerificationPolicy",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources:   patterns,
			Authorities: authorities,
			Mode:        mode,
		},
	}
}
