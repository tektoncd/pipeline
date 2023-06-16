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
	"errors"
	"fmt"
	"regexp"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/trustedresources/verifier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

const (
	// SignatureAnnotation is the key of signature in annotation map
	SignatureAnnotation = "tekton.dev/signature"
	// ConditionTrustedResourcesVerified specifies that the resources pass trusted resources verification or not.
	ConditionTrustedResourcesVerified apis.ConditionType = "TrustedResourcesVerified"
)

const (
	VerificationSkip = iota
	VerificationPass
	VerificationWarn
	VerificationError
)

// VerificationResultType indicates different cases of a verification result
type VerificationResultType int

// VerificationResult contains the type and message about the result of verification
type VerificationResult struct {
	// VerificationResultType has 4 types which is corresponding to 4 cases:
	// 0 (VerificationSkip): The verification was skipped. Err is nil in this case.
	// 1 (VerificationPass): The verification passed. Err is nil in this case.
	// 2 (VerificationWarn): A warning is logged. It could be no matching policies and feature flag "no-match-policy" is "warn", or only Warn mode verification policies fail.
	// 3 (VerificationError): The verification failed, it could be the signature doesn't match the public key, no matching policies and "no-match-policy" is set to "fail" or there are errors during verification.
	VerificationResultType VerificationResultType
	// Err contains the error message when there is a warning logged or error returned.
	Err error
}

// VerifyResource verifies the signature and public key against resource (v1beta1 and v1 task and pipeline).
// VerificationResult is returned with different types for different cases:
// 1) Return VerificationResult with VerificationSkip type, when no policies are found and no-match-policy is set to ignore
// 2) Return VerificationResult with VerificationPass type when verification passed;
// 3) Return VerificationResult with VerificationWarn type, when no matching policies and feature flag "no-match-policy" is "warn", or only Warn mode verification policies fail. Err field is filled with the warning;
// 4) Return VerificationResult with VerificationError type when no policies are found and no-match-policy is set to fail, the resource fails to pass matched enforce verification policy, or there are errors during verification. Err is filled with the err.
// refSource contains the source information of the resource.
func VerifyResource(ctx context.Context, resource metav1.Object, k8s kubernetes.Interface, refSource *v1.RefSource, verificationpolicies []*v1alpha1.VerificationPolicy) VerificationResult {
	var refSourceURI string
	if refSource != nil {
		refSourceURI = refSource.URI
	}

	matchedPolicies, err := getMatchedPolicies(resource.GetName(), refSourceURI, verificationpolicies)
	if err != nil {
		if errors.Is(err, ErrNoMatchedPolicies) {
			switch config.GetVerificationNoMatchPolicy(ctx) {
			case config.IgnoreNoMatchPolicy:
				return VerificationResult{VerificationResultType: VerificationSkip}
			case config.WarnNoMatchPolicy:
				logger := logging.FromContext(ctx)
				warning := fmt.Errorf("failed to get matched policies: %w", err)
				logger.Warnf(warning.Error())
				return VerificationResult{VerificationResultType: VerificationWarn, Err: warning}
			}
		}
		return VerificationResult{VerificationResultType: VerificationError, Err: fmt.Errorf("failed to get matched policies: %w", err)}
	}
	objectMeta, signature, err := prepareObjectMeta(resource)
	if err != nil {
		return VerificationResult{VerificationResultType: VerificationError, Err: err}
	}
	switch v := resource.(type) {
	case *v1beta1.Task:
		task := v1beta1.Task{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "Task"},
			ObjectMeta: objectMeta,
			Spec:       v.TaskSpec(),
		}
		return verifyResource(ctx, &task, k8s, signature, matchedPolicies)
	case *v1.Task:
		task := v1.Task{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1",
				Kind:       "Task"},
			ObjectMeta: objectMeta,
			Spec:       v.Spec,
		}
		return verifyResource(ctx, &task, k8s, signature, matchedPolicies)
	case *v1beta1.Pipeline:
		pipeline := v1beta1.Pipeline{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "Pipeline"},
			ObjectMeta: objectMeta,
			Spec:       v.PipelineSpec(),
		}
		return verifyResource(ctx, &pipeline, k8s, signature, matchedPolicies)
	case *v1.Pipeline:
		pipeline := v1.Pipeline{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "tekton.dev/v1",
				Kind:       "Pipeline"},
			ObjectMeta: objectMeta,
			Spec:       v.Spec,
		}
		return verifyResource(ctx, &pipeline, k8s, signature, matchedPolicies)
	default:
		return VerificationResult{VerificationResultType: VerificationError, Err: fmt.Errorf("%w: got resource %v but v1beta1.Task and v1beta1.Pipeline are currently supported", ErrResourceNotSupported, resource)}
	}
}

// VerifyTask is the deprecated, this is to keep backward compatibility
func VerifyTask(ctx context.Context, taskObj *v1beta1.Task, k8s kubernetes.Interface, refSource *v1.RefSource, verificationpolicies []*v1alpha1.VerificationPolicy) VerificationResult {
	return VerifyResource(ctx, taskObj, k8s, refSource, verificationpolicies)
}

// VerifyPipeline is the deprecated, this is to keep backward compatibility
func VerifyPipeline(ctx context.Context, pipelineObj *v1beta1.Pipeline, k8s kubernetes.Interface, refSource *v1.RefSource, verificationpolicies []*v1alpha1.VerificationPolicy) VerificationResult {
	return VerifyResource(ctx, pipelineObj, k8s, refSource, verificationpolicies)
}

// getMatchedPolicies filters out the policies by checking if the resource url (source) is matching any of the `patterns` in the `resources` list.
func getMatchedPolicies(resourceName string, source string, policies []*v1alpha1.VerificationPolicy) ([]*v1alpha1.VerificationPolicy, error) {
	matchedPolicies := []*v1alpha1.VerificationPolicy{}
	for _, p := range policies {
		for _, r := range p.Spec.Resources {
			matching, err := regexp.MatchString(r.Pattern, source)
			if err != nil {
				// FixMe: changing %v to %w breaks integration tests.
				return matchedPolicies, fmt.Errorf("%v: %w", err, ErrRegexMatch) //nolint:errorlint
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
//  1. If multiple policies match, the resource must satisfy all the "enforce" policies to pass verification. The matching "enforce" policies are evaluated using AND logic.
//     Alternatively, if the resource only matches policies in "warn" mode, it will still pass verification and only log a warning if these policies are not satisfied.
//  2. To pass one policy, the resource can pass any public keys in the policy. We use OR logic on public keys of one policy.
//
// TODO(#6683): return all failed policies in error.
func verifyResource(ctx context.Context, resource metav1.Object, k8s kubernetes.Interface, signature []byte, matchedPolicies []*v1alpha1.VerificationPolicy) VerificationResult {
	logger := logging.FromContext(ctx)
	var warnPolicies []*v1alpha1.VerificationPolicy
	var enforcePolicies []*v1alpha1.VerificationPolicy
	for _, p := range matchedPolicies {
		if p.Spec.Mode == v1alpha1.ModeWarn {
			warnPolicies = append(warnPolicies, p)
		} else {
			enforcePolicies = append(enforcePolicies, p)
		}
	}

	// first evaluate all enforce policies. Return VerificationError type of VerificationResult if any policy fails.
	for _, p := range enforcePolicies {
		verifiers, err := verifier.FromPolicy(ctx, k8s, p)
		if err != nil {
			return VerificationResult{VerificationResultType: VerificationError, Err: fmt.Errorf("failed to get verifiers from policy: %w", err)}
		}
		passVerification := doesAnyVerifierPass(resource, signature, verifiers)
		if !passVerification {
			return VerificationResult{VerificationResultType: VerificationError, Err: fmt.Errorf("%w: resource %s in namespace %s fails verification", ErrResourceVerificationFailed, resource.GetName(), resource.GetNamespace())}
		}
	}

	// then evaluate all warn policies. Return VerificationWarn type of VerificationResult if any warn policies fails.
	for _, p := range warnPolicies {
		verifiers, err := verifier.FromPolicy(ctx, k8s, p)
		if err != nil {
			warn := fmt.Errorf("failed to get verifiers for resource %s from namespace %s: %w", resource.GetName(), resource.GetNamespace(), err)
			logger.Warnf(warn.Error())
			return VerificationResult{VerificationResultType: VerificationWarn, Err: warn}
		}
		passVerification := doesAnyVerifierPass(resource, signature, verifiers)
		if !passVerification {
			warn := fmt.Errorf("%w: resource %s in namespace %s fails verification", ErrResourceVerificationFailed, resource.GetName(), resource.GetNamespace())
			logger.Warnf(warn.Error())
			return VerificationResult{VerificationResultType: VerificationWarn, Err: warn}
		}
	}

	return VerificationResult{VerificationResultType: VerificationPass}
}

// doesAnyVerifierPass loop over verifiers to verify the resource, return true if any verifier pass verification.
func doesAnyVerifierPass(resource metav1.Object, signature []byte, verifiers []signature.Verifier) bool {
	passVerification := false
	for _, verifier := range verifiers {
		// if one of the verifier passes verification, then this policy passes verification
		if err := verifyInterface(resource, verifier, signature); err == nil {
			passVerification = true
			break
		}
	}
	return passVerification
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
		// FixMe: changing %v to %w breaks integration tests.
		return fmt.Errorf("%w:%v", ErrResourceVerificationFailed, err.Error())
	}

	return nil
}

// prepareObjectMeta will remove annotations not configured from user side -- "kubectl-client-side-apply" and "kubectl.kubernetes.io/last-applied-configuration"
// (added when an object is created with `kubectl apply`) to avoid verification failure and extract the signature.
// Returns a copy of the input object metadata with the annotations removed and the object's signature,
// if it is present in the metadata.
// Returns a non-nil error if the signature cannot be decoded.
func prepareObjectMeta(in metav1.Object) (metav1.ObjectMeta, []byte, error) {
	out := metav1.ObjectMeta{}

	// exclude the fields populated by system.
	out.Name = in.GetName()
	out.GenerateName = in.GetGenerateName()
	out.Namespace = in.GetNamespace()

	if in.GetLabels() != nil {
		out.Labels = make(map[string]string)
		for k, v := range in.GetLabels() {
			out.Labels[k] = v
		}
	}

	out.Annotations = make(map[string]string)
	for k, v := range in.GetAnnotations() {
		out.Annotations[k] = v
	}

	// exclude the annotations added by other components
	// Task annotations are unlikely to be changed, we need to make sure other components
	// like resolver doesn't modify the annotations, otherwise the verification will fail
	delete(out.Annotations, "kubectl-client-side-apply")
	delete(out.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

	// signature should be contained in annotation
	sig, ok := in.GetAnnotations()[SignatureAnnotation]
	if !ok {
		return out, nil, nil
	}
	// extract signature
	signature, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return out, nil, err
	}
	delete(out.Annotations, SignatureAnnotation)

	return out, signature, nil
}
