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

package resolution_test

import (
	"errors"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/pkg/reconciler/resolution"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

func taskWithOwnerRefs() *v1.Task {
	return &v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "my-task",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{Name: "some-owner"}},
		},
	}
}

// TestCleanupAndValidate_NilPoliciesFailNoMatchPolicy guards against
// verification being silently skipped when the lister returns a nil policy
// slice: with no matching policies and no-match-policy "fail", the result
// must be VerificationError, not a bypass.
func TestCleanupAndValidate_NilPoliciesFailNoMatchPolicy(t *testing.T) {
	ctx := test.SetupTrustedResourceConfig(t.Context(), config.FailNoMatchPolicy)
	task := taskWithOwnerRefs()

	mutated, vr, err := resolution.CleanupAndValidate(ctx, "default", task, fakek8s.NewSimpleClientset(), fake.NewSimpleClientset(), nil, nil)
	if err != nil {
		t.Fatalf("CleanupAndValidate: %v", err)
	}
	if mutated == nil {
		t.Fatal("expected mutated object, got nil")
	}
	if vr == nil {
		t.Fatal("expected a VerificationResult, got nil")
	}
	if vr.VerificationResultType != trustedresources.VerificationError {
		t.Errorf("VerificationResultType = %v, want VerificationError", vr.VerificationResultType)
	}
	if !errors.Is(vr.Err, trustedresources.ErrNoMatchedPolicies) {
		t.Errorf("Err = %v, want ErrNoMatchedPolicies", vr.Err)
	}
	if len(task.GetOwnerReferences()) != 0 {
		t.Errorf("OwnerReferences not cleared: %v", task.GetOwnerReferences())
	}
}

// TestCleanupAndValidate_NilPoliciesIgnoreNoMatchPolicy verifies the skip
// path still works when no-match-policy is "ignore".
func TestCleanupAndValidate_NilPoliciesIgnoreNoMatchPolicy(t *testing.T) {
	ctx := test.SetupTrustedResourceConfig(t.Context(), config.IgnoreNoMatchPolicy)
	task := taskWithOwnerRefs()

	_, vr, err := resolution.CleanupAndValidate(ctx, "default", task, fakek8s.NewSimpleClientset(), fake.NewSimpleClientset(), nil, nil)
	if err != nil {
		t.Fatalf("CleanupAndValidate: %v", err)
	}
	if vr == nil {
		t.Fatal("expected a VerificationResult, got nil")
	}
	if vr.VerificationResultType != trustedresources.VerificationSkip {
		t.Errorf("VerificationResultType = %v, want VerificationSkip", vr.VerificationResultType)
	}
}

func TestCleanupAndDryRun(t *testing.T) {
	sa := &v1beta1.StepAction{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "my-stepaction",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{Name: "some-owner"}},
		},
	}

	mutated, err := resolution.CleanupAndDryRun(t.Context(), "default", sa, fake.NewSimpleClientset())
	if err != nil {
		t.Fatalf("CleanupAndDryRun: %v", err)
	}
	if mutated == nil {
		t.Fatal("expected mutated object, got nil")
	}
	if len(sa.GetOwnerReferences()) != 0 {
		t.Errorf("OwnerReferences not cleared: %v", sa.GetOwnerReferences())
	}
}
