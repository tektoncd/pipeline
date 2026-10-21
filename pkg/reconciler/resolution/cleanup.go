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
	"fmt"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/reconciler/apiserver"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// CleanupAndValidate performs the common post-resolve pattern for resource
// kinds covered by trustedresources verification (Task, Pipeline):
//  1. Clears OwnerReferences on the resolved object
//  2. Verifies the resource via trustedresources (always; with no matching
//     policies the result is driven by the trusted-resources no-match-policy)
//  3. Dry-run validates via the API server
//
// It returns the mutated runtime.Object from dry-run validation and the
// VerificationResult. The helper does not preserve ObjectMeta: callers that
// need the original metadata on the mutated object must restore it themselves.
func CleanupAndValidate(
	ctx context.Context,
	namespace string,
	obj runtime.Object,
	k8s kubernetes.Interface,
	tekton clientset.Interface,
	refSource *v1.RefSource,
	verificationPolicies []*v1alpha1.VerificationPolicy,
) (runtime.Object, *trustedresources.VerificationResult, error) {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return nil, nil, fmt.Errorf("object %T does not implement metav1.Object", obj)
	}
	metaObj.SetOwnerReferences(nil)

	result := trustedresources.VerifyResource(ctx, metaObj, k8s, refSource, verificationPolicies)

	mutated, err := apiserver.DryRunValidate(ctx, namespace, obj, tekton)
	if err != nil {
		return nil, nil, err
	}

	return mutated, &result, nil
}

// CleanupAndDryRun performs the same OwnerReferences cleanup and dry-run
// validation as CleanupAndValidate but without trustedresources verification,
// for resource kinds that verification does not cover (e.g. StepAction).
func CleanupAndDryRun(
	ctx context.Context,
	namespace string,
	obj runtime.Object,
	tekton clientset.Interface,
) (runtime.Object, error) {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("object %T does not implement metav1.Object", obj)
	}
	metaObj.SetOwnerReferences(nil)

	return apiserver.DryRunValidate(ctx, namespace, obj, tekton)
}
