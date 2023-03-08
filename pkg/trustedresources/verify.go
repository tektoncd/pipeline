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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/trustedresources/verifier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// SignatureAnnotation is the key of signature in annotation map
	SignatureAnnotation = "tekton.dev/signature"
)

// VerifyTask verifies the signature and public key against task.
// source is from ConfigSource.URI, which will be used to match policy patterns. k8s is used to fetch secret from cluster
func VerifyTask(ctx context.Context, taskObj v1beta1.TaskObject, k8s kubernetes.Interface, source string, policies []*v1alpha1.VerificationPolicy) error {
	matchedPolicies, err := matchedPolicies(taskObj.TaskMetadata().Name, source, policies)
	if err != nil {
		return err
	}
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

	return verifyResource(ctx, &task, k8s, signature, source, matchedPolicies)
}

// VerifyPipeline verifies the signature and public key against pipeline.
// source is from ConfigSource.URI, which will be used to match policy patterns, k8s is used to fetch secret from cluster
func VerifyPipeline(ctx context.Context, pipelineObj v1beta1.PipelineObject, k8s kubernetes.Interface, source string, policies []*v1alpha1.VerificationPolicy) error {
	matchedPolicies, err := matchedPolicies(pipelineObj.PipelineMetadata().Name, source, policies)
	if err != nil {
		return err
	}
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

	return verifyResource(ctx, &pipeline, k8s, signature, source, matchedPolicies)
}

// matchedPolicies filters out the policies by checking if the resource url (source) is matching any of the `patterns` in the `resources` list.
func matchedPolicies(resourceName string, source string, policies []*v1alpha1.VerificationPolicy) ([]*v1alpha1.VerificationPolicy, error) {
	if len(policies) == 0 {
		return nil, ErrEmptyVerificationConfig
	}
	matchedPolicies := []*v1alpha1.VerificationPolicy{}
	for _, p := range policies {
		for _, r := range p.Spec.Resources {
			matching, err := regexp.MatchString(r.Pattern, source)
			if err != nil {
				return matchedPolicies, fmt.Errorf("%v: %w", err, ErrRegexMatch)
			}
			if matching {
				matchedPolicies = append(matchedPolicies, p)
				break
			}
		}
	}
	if len(matchedPolicies) == 0 {
		return matchedPolicies, fmt.Errorf("%w: no matching policies are found for resource: %s against source: %s", ErrNoMatchedPolicies, resourceName, source)
	}
	return matchedPolicies, nil
}

// verifyResource verifies resource which implements metav1.Object by provided signature and public keys from verification policies.
// For matched policies, `verifyResource“ will adopt the following rules to do verification:
// 1. If multiple policies are matched, the resource needs to pass all of them to pass verification. We use AND logic on matched policies.
// 2. To pass one policy, the resource can pass any public keys in the policy. We use OR logic on public keys of one policy.
func verifyResource(ctx context.Context, resource metav1.Object, k8s kubernetes.Interface, signature []byte, source string, matchedPolicies []*v1alpha1.VerificationPolicy) error {
	for _, p := range matchedPolicies {
		passVerification := false
		verifiers, err := verifier.FromPolicy(ctx, k8s, p)
		if err != nil {
			return fmt.Errorf("failed to get verifiers from policy: %w", err)
		}
		for _, verifier := range verifiers {
			// if one of the verifier passes verification, then this policy passes verification
			if err := verifyInterface(resource, verifier, signature); err == nil {
				passVerification = true
				break
			}
		}
		// if this policy fails the verification, should return error directly. No need to check other policies
		if !passVerification {
			return fmt.Errorf("%w: resource %s in namespace %s fails verification", ErrResourceVerificationFailed, resource.GetName(), resource.GetNamespace())
		}
	}
	return nil
}

// verifyInterface get the checksum of json marshalled object and verify it.
func verifyInterface(obj interface{}, verifier signature.Verifier, signature []byte) error {
	ts, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal the object: %w", err)
	}

	h := sha256.New()
	h.Write(ts)

	if err := verifier.VerifySignature(bytes.NewReader(signature), bytes.NewReader(h.Sum(nil))); err != nil {
		return fmt.Errorf("%w:%v", ErrResourceVerificationFailed, err.Error())
	}

	return nil
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
		return out, nil, ErrSignatureMissing
	}
	// extract signature
	signature, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return out, nil, err
	}
	delete(out.Annotations, SignatureAnnotation)

	return out, signature, nil
}
