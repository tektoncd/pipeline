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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// keyReference is the prefix of secret reference
	keyReference = "k8s://"
)

// FromConfigMap get all verifiers from configmap, k8s is provided to fetch secret from cluster
func FromConfigMap(ctx context.Context, k8s kubernetes.Interface) ([]signature.Verifier, error) {
	cfg := config.FromContextOrDefaults(ctx)
	verifiers := []signature.Verifier{}
	for key := range cfg.TrustedResources.Keys {
		if key == "" {
			continue
		}
		v, err := fromKeyRef(ctx, key, crypto.SHA256, k8s)
		if err != nil {
			return nil, fmt.Errorf("failed to get verifier from keyref: %w", err)
		}
		verifiers = append(verifiers, v)
	}
	if len(verifiers) == 0 {
		return nil, ErrorEmptyPublicKeys
	}
	return verifiers, nil
}

// FromPolicy get all verifiers from VerificationPolicy.
// For each policy, loop the Authorities of the VerificationPolicy to fetch public key
// from either inline Data or from a SecretRef.
func FromPolicy(ctx context.Context, k8s kubernetes.Interface, policy *v1alpha1.VerificationPolicy) ([]signature.Verifier, error) {
	verifiers := []signature.Verifier{}
	for _, a := range policy.Spec.Authorities {
		algorithm, err := matchHashAlgorithm(a.Key.HashAlgorithm)
		if err != nil {
			return nil, fmt.Errorf("authority %q contains an invalid hash algorithm: %w", a.Name, err)
		}
		if a.Key.Data == "" && a.Key.SecretRef == nil {
			return nil, ErrorEmptyKey
		}
		if a.Key.Data != "" {
			v, err := fromData([]byte(a.Key.Data), algorithm)
			if err != nil {
				return nil, fmt.Errorf("failed to get verifier from data: %w", err)
			}
			verifiers = append(verifiers, v)
		} else if a.Key.SecretRef != nil {
			v, err := fromSecret(ctx, fmt.Sprintf("%s%s/%s", keyReference, a.Key.SecretRef.Namespace, a.Key.SecretRef.Name), algorithm, k8s)
			if err != nil {
				return nil, fmt.Errorf("failed to get verifier from secret: %w", err)
			}
			verifiers = append(verifiers, v)
		}
	}
	if len(verifiers) == 0 {
		return verifiers, ErrorEmptyPublicKeys
	}
	return verifiers, nil
}

// fromKeyRef parses the given keyRef, loads the key and returns an appropriate
// verifier using the provided hash algorithm
func fromKeyRef(ctx context.Context, keyRef string, hashAlgorithm crypto.Hash, k8s kubernetes.Interface) (signature.Verifier, error) {
	var raw []byte
	if strings.HasPrefix(keyRef, keyReference) {
		v, err := fromSecret(ctx, keyRef, hashAlgorithm, k8s)
		if err != nil {
			return nil, fmt.Errorf("failed to get verifier from secret: %w", err)
		}
		return v, nil
	}
	raw, err := os.ReadFile(filepath.Clean(keyRef))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrorFailedLoadKeyFile, err)
	}
	v, err := fromData(raw, hashAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("failed to get verifier from data: %w", err)
	}
	return v, nil
}

// fromSecret fetches the public key from SecretRef and returns the verifier
// hashAlgorithm is provided to determine the hash algorithm of the key
func fromSecret(ctx context.Context, secretRef string, hashAlgorithm crypto.Hash, k8s kubernetes.Interface) (signature.Verifier, error) {
	if strings.HasPrefix(secretRef, keyReference) {
		s, err := getKeyPairSecret(ctx, secretRef, k8s)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret: %w", err)
		}
		// only 1 public key should be in the secret
		if len(s.Data) == 0 {
			return nil, fmt.Errorf("secret %q contains no data %w", secretRef, ErrorEmptySecretData)
		}
		if len(s.Data) > 1 {
			return nil, fmt.Errorf("secret %q contains multiple data entries, only one is supported. %w", secretRef, ErrorMultipleSecretData)
		}
		for _, raw := range s.Data {
			v, err := fromData(raw, hashAlgorithm)
			if err != nil {
				return nil, fmt.Errorf("failed to get verifier from secret data: %w", err)
			}
			return v, nil
		}
	}
	return nil, fmt.Errorf("%w: secretRef %v is invalid", ErrorK8sSpecificationInvalid, secretRef)
}

// fromData fetches the public key from raw data and returns the verifier
func fromData(raw []byte, hashAlgorithm crypto.Hash) (signature.Verifier, error) {
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrorDecodeKey, err)
	}
	v, err := signature.LoadVerifier(pubKey, hashAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrorLoadVerifier, err)
	}
	return v, nil
}

// getKeyPairSecret fetches the secret from a k8sRef
// TODO(#5884): use a secret lister to fetch secrets
func getKeyPairSecret(ctx context.Context, k8sRef string, k8s kubernetes.Interface) (*v1.Secret, error) {
	split := strings.Split(strings.TrimPrefix(k8sRef, keyReference), "/")
	if len(split) != 2 {
		return nil, ErrorK8sSpecificationInvalid
	}
	namespace, name := split[0], split[1]

	s, err := k8s.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrorSecretNotFound, err)
	}

	return s, nil
}

// matchHashAlgorithm returns a crypto.Hash code using an algorithm name as input parameter
func matchHashAlgorithm(algorithmName v1alpha1.HashAlgorithm) (crypto.Hash, error) {
	normalizedAlgo := strings.ToLower(string(algorithmName))
	algo, exists := v1alpha1.SupportedSignatureAlgorithms[v1alpha1.HashAlgorithm(normalizedAlgo)]
	if !exists {
		return crypto.SHA256, fmt.Errorf("%w: %s", ErrorAlgorithmInvalid, algorithmName)
	}
	return algo, nil
}
