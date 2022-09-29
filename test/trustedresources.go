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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

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

// SetupTrustedResourceConfig config the keys and feature flag for testing
func SetupTrustedResourceConfig(ctx context.Context, keypath string, resourceVerificationMode string) context.Context {
	store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      config.TrustedTaskConfig,
		},
		Data: map[string]string{
			config.PublicKeys: keypath,
		},
	}
	store.OnConfigChanged(cm)

	featureflags := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "feature-flags",
		},
		Data: map[string]string{
			"resource-verification-mode": resourceVerificationMode,
		},
	}
	store.OnConfigChanged(featureflags)

	return store.ToContext(ctx)
}

// GetSignerFromFile generates key files to tmpdir, return signer and pubkey path
func GetSignerFromFile(ctx context.Context, t *testing.T) (signature.Signer, string, error) {
	t.Helper()

	tmpDir := t.TempDir()
	publicKeyFile := "ecdsa.pub"
	sv, err := GenerateKeyFile(tmpDir, publicKeyFile)
	if err != nil {
		t.Fatal(err)
	}

	return sv, filepath.Join(tmpDir, publicKeyFile), nil
}

// GenerateKeyFile creates public key files, return the SignerVerifier
func GenerateKeyFile(dir string, pubkeyfile string) (signature.SignerVerifier, error) {
	keys, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Now do the public key
	pubBytes, err := cryptoutils.MarshalPublicKeyToPEM(keys.Public())
	if err != nil {
		return nil, err
	}

	pubKey := filepath.Join(dir, pubkeyfile)
	if err := os.WriteFile(pubKey, pubBytes, 0600); err != nil {
		return nil, err
	}

	sv, err := signature.LoadSignerVerifier(keys, crypto.SHA256)
	if err != nil {
		return nil, err
	}

	return sv, nil
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

// GetSignedPipeline signed the given pipeline and rename it with given name
func GetSignedPipeline(unsigned *v1beta1.Pipeline, signer signature.Signer, name string) (*v1beta1.Pipeline, error) {
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

// GetSignedTask signed the given task and rename it with given name
func GetSignedTask(unsigned *v1beta1.Task, signer signature.Signer, name string) (*v1beta1.Task, error) {
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
