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

package trustedresources

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// SignatureAnnotation is the key of signature in annotation map
	SignatureAnnotation = "tekton.dev/signature"
	// keyReference is the prefix of secret reference
	keyReference = "k8s://"
)

// VerifyInterface get the checksum of json marshalled object and verify it.
func VerifyInterface(obj interface{}, verifier signature.Verifier, signature []byte) error {
	ts, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(ts)

	if err := verifier.VerifySignature(bytes.NewReader(signature), bytes.NewReader(h.Sum(nil))); err != nil {
		return err
	}

	return nil
}

// VerifyTask verifies the signature and public key against task
func VerifyTask(ctx context.Context, taskObj v1beta1.TaskObject, k8s kubernetes.Interface) error {
	tm, signature, err := prepareObjectMeta(taskObj.TaskMetadata())
	if err != nil {
		return err
	}
	task := v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task"},
		ObjectMeta: tm,
		Spec:       taskObj.TaskSpec(),
	}
	verifiers, err := getVerifiers(ctx, k8s)
	if err != nil {
		return err
	}
	for _, verifier := range verifiers {
		if err := VerifyInterface(task, verifier, signature); err == nil {
			return nil
		}
	}
	return fmt.Errorf("Task %s in namespace %s fails verification", task.Name, task.Namespace)
}

// VerifyPipeline verifies the signature and public key against pipeline
func VerifyPipeline(ctx context.Context, pipelineObj v1beta1.PipelineObject, k8s kubernetes.Interface) error {
	pm, signature, err := prepareObjectMeta(pipelineObj.PipelineMetadata())
	if err != nil {
		return err
	}
	pipeline := v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Pipeline"},
		ObjectMeta: pm,
		Spec:       pipelineObj.PipelineSpec(),
	}
	verifiers, err := getVerifiers(ctx, k8s)
	if err != nil {
		return err
	}

	for _, verifier := range verifiers {
		if err := VerifyInterface(pipeline, verifier, signature); err == nil {
			return nil
		}
	}

	return fmt.Errorf("Pipeline %s in namespace %s fails verification", pipeline.Name, pipeline.Namespace)
}

// prepareObjectMeta will remove annotations not configured from user side -- "kubectl-client-side-apply" and "kubectl.kubernetes.io/last-applied-configuration"
// to avoid verification failure and extract the signature.
func prepareObjectMeta(in metav1.ObjectMeta) (metav1.ObjectMeta, []byte, error) {
	out := metav1.ObjectMeta{}

	// exclude the fields populated by system.
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace

	if in.Labels != nil {
		out.Labels = make(map[string]string)
		for k, v := range in.Labels {
			out.Labels[k] = v
		}
	}

	out.Annotations = make(map[string]string)
	for k, v := range in.Annotations {
		out.Annotations[k] = v
	}

	// exclude the annotations added by other components
	// Task annotations are unlikely to be changed, we need to make sure other components
	// like resolver doesn't modify the annotations, otherwise the verification will fail
	delete(out.Annotations, "kubectl-client-side-apply")
	delete(out.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

	// signature should be contained in annotation
	sig, ok := in.Annotations[SignatureAnnotation]
	if !ok {
		return out, nil, fmt.Errorf("signature is missing")
	}
	// extract signature
	signature, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return out, nil, err
	}
	delete(out.Annotations, SignatureAnnotation)

	return out, signature, nil
}

// getVerifiers get all verifiers from configmap
func getVerifiers(ctx context.Context, k8s kubernetes.Interface) ([]signature.Verifier, error) {
	cfg := config.FromContextOrDefaults(ctx)
	verifiers := []signature.Verifier{}
	// TODO(#5527): consider using k8s://namespace/name instead of mounting files.
	for key := range cfg.TrustedResources.Keys {
		v, err := verifierForKeyRef(ctx, key, crypto.SHA256, k8s)
		if err == nil {
			verifiers = append(verifiers, v...)
		}
	}
	if len(verifiers) == 0 {
		return verifiers, fmt.Errorf("no public keys are founded for verification")
	}

	return verifiers, nil
}

// verifierForKeyRef parses the given keyRef, loads the key and returns an appropriate
// verifier using the provided hash algorithm
// TODO(#5527): consider wrap verifiers to resolver so the same verifiers are used for the same reconcile event
func verifierForKeyRef(ctx context.Context, keyRef string, hashAlgorithm crypto.Hash, k8s kubernetes.Interface) (verifiers []signature.Verifier, err error) {
	var raw []byte
	verifiers = []signature.Verifier{}
	// if the ref is secret then we fetch the keys from the secrets
	if strings.HasPrefix(keyRef, keyReference) {
		s, err := getKeyPairSecret(ctx, keyRef, k8s)
		if err != nil {
			return nil, err
		}
		for _, raw := range s.Data {
			pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
			if err != nil {
				return nil, fmt.Errorf("pem to public key: %w", err)
			}
			v, _ := signature.LoadVerifier(pubKey, hashAlgorithm)
			verifiers = append(verifiers, v)
		}
		if len(verifiers) == 0 {
			return verifiers, fmt.Errorf("no public keys are founded for verification")
		}
		return verifiers, nil
	}
	// read the key from mounted file
	raw, err = os.ReadFile(filepath.Clean(keyRef))
	if err != nil {
		return nil, err
	}

	// PEM encoded file.
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("pem to public key: %w", err)
	}
	v, _ := signature.LoadVerifier(pubKey, hashAlgorithm)
	verifiers = append(verifiers, v)

	return verifiers, nil
}

func getKeyPairSecret(ctx context.Context, k8sRef string, k8s kubernetes.Interface) (*v1.Secret, error) {
	namespace, name, err := parseRef(k8sRef)
	if err != nil {
		return nil, err
	}

	var s *v1.Secret
	if s, err = k8s.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		return nil, errors.Wrap(err, "checking if secret exists")
	}

	return s, nil
}

// the reference should be formatted as <namespace>/<secret name>
func parseRef(k8sRef string) (string, string, error) {
	s := strings.Split(strings.TrimPrefix(k8sRef, keyReference), "/")
	if len(s) != 2 {
		return "", "", errors.New("kubernetes specification should be in the format k8s://<namespace>/<secret>")
	}
	return s[0], s[1], nil
}
