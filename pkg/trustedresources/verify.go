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
	"encoding/base64"
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

type Hashable interface {
	Checksum() ([]byte, error)
}

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
	signature, err := extractSignature(resource)
	if err != nil {
		return VerificationResult{VerificationResultType: VerificationError, Err: err}
	}
	return verifyResource(ctx, resource, k8s, signature, matchedPolicies)
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

	// get the checksum of the resource
	checksumBytes, err := getChecksum(resource)
	if err != nil {
		return VerificationResult{VerificationResultType: VerificationError, Err: err}
	}

	// first evaluate all enforce policies. Return VerificationError type of VerificationResult if any policy fails.
	for _, p := range enforcePolicies {
		verifiers, err := verifier.FromPolicy(ctx, k8s, p)
		if err != nil {
			return VerificationResult{VerificationResultType: VerificationError, Err: fmt.Errorf("failed to get verifiers from policy: %w", err)}
		}
		passVerification := doesAnyVerifierPass(ctx, checksumBytes, signature, verifiers)
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
		passVerification := doesAnyVerifierPass(ctx, checksumBytes, signature, verifiers)
		if !passVerification {
			warn := fmt.Errorf("%w: resource %s in namespace %s fails verification", ErrResourceVerificationFailed, resource.GetName(), resource.GetNamespace())
			logger.Warnf(warn.Error())
			return VerificationResult{VerificationResultType: VerificationWarn, Err: warn}
		}
	}

	return VerificationResult{VerificationResultType: VerificationPass}
}

// doesAnyVerifierPass loop over verifiers to verify the checksum and the signature, return true if any verifier pass verification.
func doesAnyVerifierPass(ctx context.Context, checksumBytes []byte, signature []byte, verifiers []signature.Verifier) bool {
	logger := logging.FromContext(ctx)
	passVerification := false
	for _, verifier := range verifiers {
		if err := verifier.VerifySignature(bytes.NewReader(signature), bytes.NewReader(checksumBytes)); err == nil {
			// if one of the verifier passes verification, then this policy passes verification
			passVerification = true
			break
		} else {
			// FixMe: changing %v to %w breaks integration tests.
			warn := fmt.Errorf("%w:%v", ErrResourceVerificationFailed, err.Error())
			logger.Warnf(warn.Error())
		}
	}
	return passVerification
}

// extractSignature extracts the signature if it is present in the metadata.
// Returns a non-nil error if the signature cannot be decoded.
func extractSignature(in metav1.Object) ([]byte, error) {
	// signature should be contained in annotation
	sig, ok := in.GetAnnotations()[SignatureAnnotation]
	if !ok {
		return nil, nil
	}
	// extract signature
	signature, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// getChecksum gets the sha256 checksum of the resource.
// Returns a non-nil error if the checksum cannot be computed or the resource is of unknown type.
func getChecksum(resource metav1.Object) ([]byte, error) {
	h, ok := resource.(Hashable)
	if !ok {
		return nil, fmt.Errorf("%w: resource %T is not a Hashable type", ErrResourceNotSupported, resource)
	}
	checksumBytes, err := h.Checksum()
	if err != nil {
		return nil, err
	}
	return checksumBytes, nil
}
