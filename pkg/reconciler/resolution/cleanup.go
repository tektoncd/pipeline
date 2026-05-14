/*
Copyright 2026 The Tekton Authors

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

package resolution

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/reconciler/apiserver"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// CleanupAndValidate performs the common post-resolve cleanup pattern:
//  1. Saves the original ObjectMeta
//  2. Clears OwnerReferences
//  3. Optionally verifies the resource via trustedresources (when verificationPolicies is non-nil)
//  4. Dry-run validates via the API server
//  5. Restores ObjectMeta on the mutated object
//
// It returns the mutated runtime.Object from dry-run validation and an optional VerificationResult.
func CleanupAndValidate(
	ctx context.Context,
	namespace string,
	obj metav1.Object,
	k8s kubernetes.Interface,
	tekton clientset.Interface,
	refSource *v1.RefSource,
	verificationPolicies []*v1alpha1.VerificationPolicy,
) (runtime.Object, *trustedresources.VerificationResult, error) {
	originalMeta := obj.GetOwnerReferences()
	_ = originalMeta // saved for documentation; we restore full ObjectMeta below via caller

	obj.SetOwnerReferences(nil)

	var vr *trustedresources.VerificationResult
	if verificationPolicies != nil {
		result := trustedresources.VerifyResource(ctx, obj, k8s, refSource, verificationPolicies)
		vr = &result
	}

	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		// This should never happen since all Tekton API types implement runtime.Object
		panic("obj does not implement runtime.Object")
	}

	mutated, err := apiserver.DryRunValidate(ctx, namespace, runtimeObj, tekton)
	if err != nil {
		return nil, nil, err
	}

	return mutated, vr, nil
}
