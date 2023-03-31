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
	"context"
	"crypto"
	"crypto/elliptic"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/trustedresources/verifier"
	test "github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

const (
	namespace = "trusted-resources"
)

func TestVerifyInterface_Task_Success(t *testing.T) {
	sv, _, err := signature.NewDefaultECDSASignerVerifier()
	if err != nil {
		t.Fatalf("failed to get signerverifier %v", err)
	}

	unsignedTask := test.GetUnsignedTask("test-task")
	signedTask, err := test.GetSignedTask(unsignedTask, sv, "signed")
	if err != nil {
		t.Fatalf("Failed to get signed task %v", err)
	}

	signature := []byte{}

	if sig, ok := signedTask.Annotations[SignatureAnnotation]; ok {
		delete(signedTask.Annotations, SignatureAnnotation)
		signature, err = base64.StdEncoding.DecodeString(sig)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = verifyInterface(signedTask, sv, signature)
	if err != nil {
		t.Fatalf("VerifyInterface() get err %v", err)
	}
}

func TestVerifyInterface_Task_Error(t *testing.T) {
	sv, _, err := signature.NewDefaultECDSASignerVerifier()
	if err != nil {
		t.Fatalf("failed to get signerverifier %v", err)
	}

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, sv, "signed")
	if err != nil {
		t.Fatalf("Failed to get signed task %v", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Name = "tampered"

	tcs := []struct {
		name          string
		task          *v1beta1.Task
		expectedError error
	}{{
		name:          "Unsigned Task Fail Verification",
		task:          unsignedTask,
		expectedError: ErrResourceVerificationFailed,
	}, {
		name:          "Empty task Fail Verification",
		task:          nil,
		expectedError: ErrResourceVerificationFailed,
	}, {
		name:          "Tampered task Fail Verification",
		task:          tamperedTask,
		expectedError: ErrResourceVerificationFailed,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			signature := []byte{}

			if tc.task != nil {
				if sig, ok := tc.task.Annotations[SignatureAnnotation]; ok {
					delete(tc.task.Annotations, SignatureAnnotation)
					signature, err = base64.StdEncoding.DecodeString(sig)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			err := verifyInterface(tc.task, sv, signature)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("verifyInterface got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestVerifyTask_Success(t *testing.T) {
	signer256, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	unsignedTask := test.GetUnsignedTask("test-task")
	signedTask, err := test.GetSignedTask(unsignedTask, signer256, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	modifiedTask := signedTask.DeepCopy()
	modifiedTask.Name = "modified"

	signer384, _, pub, err := test.GenerateKeys(elliptic.P384(), crypto.SHA384)
	if err != nil {
		t.Fatalf("failed to generate keys %v", err)
	}

	sha384Vp := &v1alpha1.VerificationPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VerificationPolicy",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "differentAlgo",
			Namespace: namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{
				{Pattern: "gcr.io/tekton-releases/catalog/upstream/sha384"},
			},
			Authorities: []v1alpha1.Authority{
				{
					Name: "sha384Key",
					Key: &v1alpha1.KeyRef{
						Data:          string(pub),
						HashAlgorithm: "sha384",
					},
				},
			},
		},
	}

	warnPolicy := &v1alpha1.VerificationPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VerificationPolicy",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "warnPolicy",
			Namespace: namespace,
		},
		Spec: v1alpha1.VerificationPolicySpec{
			Resources: []v1alpha1.ResourcePattern{
				{Pattern: "https://github.com/tektoncd/catalog.git"},
			},
			Authorities: []v1alpha1.Authority{
				{
					Name: "key",
					Key: &v1alpha1.KeyRef{
						Data:          string(pub),
						HashAlgorithm: "sha384",
					},
				},
			},
			Mode: v1alpha1.ModeWarn,
		},
	}

	signedTask384, err := test.GetSignedTask(unsignedTask, signer384, "signed384")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	mismatchedSource := "wrong source"
	tcs := []struct {
		name                      string
		task                      *v1beta1.Task
		source                    string
		signer                    signature.SignerVerifier
		verificationNoMatchPolicy string
		verificationPolicies      []*v1alpha1.VerificationPolicy
	}{{
		name:                      "signed git source task passes verification",
		task:                      signedTask,
		source:                    "git+https://github.com/tektoncd/catalog.git",
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		verificationPolicies:      vps,
	}, {
		name:                      "signed bundle source task passes verification",
		task:                      signedTask,
		source:                    "gcr.io/tekton-releases/catalog/upstream/git-clone",
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		verificationPolicies:      vps,
	}, {
		name:                      "signed task with sha384 key",
		task:                      signedTask384,
		source:                    "gcr.io/tekton-releases/catalog/upstream/sha384",
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		verificationPolicies:      []*v1alpha1.VerificationPolicy{sha384Vp},
	}, {
		name:                      "ignore no match policy skips verification when no matching policies",
		task:                      unsignedTask,
		source:                    mismatchedSource,
		verificationNoMatchPolicy: config.IgnoreNoMatchPolicy,
	}, {
		name:                      "warn no match policy skips verification when no matching policies",
		task:                      unsignedTask,
		source:                    mismatchedSource,
		verificationNoMatchPolicy: config.WarnNoMatchPolicy,
	}, {
		name:                      "unsigned task matches warn policy doesn't fail verification",
		task:                      unsignedTask,
		source:                    "git+https://github.com/tektoncd/catalog.git",
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		verificationPolicies:      []*v1alpha1.VerificationPolicy{warnPolicy},
	}, {
		name:                      "modified task matches warn policy doesn't fail verification",
		task:                      modifiedTask,
		source:                    "git+https://github.com/tektoncd/catalog.git",
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
		verificationPolicies:      []*v1alpha1.VerificationPolicy{warnPolicy},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := test.SetupTrustedResourceConfig(context.Background(), tc.verificationNoMatchPolicy)
			err := VerifyTask(ctx, tc.task, k8sclient, tc.source, tc.verificationPolicies)
			if err != nil {
				t.Fatalf("VerifyTask() get err %v", err)
			}
		})
	}
}

func TestVerifyTask_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.FailNoMatchPolicy)
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"

	matchingSource := "git+https://github.com/tektoncd/catalog.git"
	mismatchedSource := "wrong source"
	tcs := []struct {
		name               string
		task               *v1beta1.Task
		source             string
		verificationPolicy []*v1alpha1.VerificationPolicy
		expectedError      error
	}{{
		name:               "unsigned Task fails verification",
		task:               unsignedTask,
		source:             "git+https://github.com/tektoncd/catalog.git",
		verificationPolicy: vps,
		expectedError:      ErrResourceVerificationFailed,
	}, {
		name:               "modified Task fails verification",
		task:               tamperedTask,
		source:             matchingSource,
		verificationPolicy: vps,
		expectedError:      ErrResourceVerificationFailed,
	}, {
		name:               "task not matching pattern fails verification",
		task:               signedTask,
		source:             mismatchedSource,
		verificationPolicy: vps,
		expectedError:      ErrNoMatchedPolicies,
	}, {
		name:               "verification fails with empty policy",
		task:               tamperedTask,
		source:             matchingSource,
		verificationPolicy: []*v1alpha1.VerificationPolicy{},
		expectedError:      ErrNoMatchedPolicies,
	}, {
		name:   "Verification fails with regex error",
		task:   signedTask,
		source: "git+https://github.com/tektoncd/catalog.git",
		verificationPolicy: []*v1alpha1.VerificationPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{{
						Pattern: "^[",
					}},
				},
			},
		},
		expectedError: ErrRegexMatch,
	}, {
		name:   "Verification fails with error from policy",
		task:   signedTask,
		source: "git+https://github.com/tektoncd/catalog.git",
		verificationPolicy: []*v1alpha1.VerificationPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{{
						Pattern: ".*",
					}},
					Authorities: []v1alpha1.Authority{
						{
							Name: "foo",
							Key: &v1alpha1.KeyRef{
								Data: "inline_key",
							},
						},
					},
				},
			},
		},
		expectedError: verifier.ErrDecodeKey,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyTask(ctx, tc.task, k8sclient, tc.source, tc.verificationPolicy)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("VerifyTask got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestVerifyPipeline_Success(t *testing.T) {
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")
	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	mismatchedSource := "wrong source"
	tcs := []struct {
		name                      string
		pipeline                  v1beta1.PipelineObject
		source                    string
		verificationNoMatchPolicy string
	}{{
		name:                      "signed git source pipeline passes verification",
		pipeline:                  signedPipeline,
		source:                    "git+https://github.com/tektoncd/catalog.git",
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
	}, {
		name:                      "signed bundle source pipeline passes verification",
		pipeline:                  signedPipeline,
		source:                    "gcr.io/tekton-releases/catalog/upstream/git-clone",
		verificationNoMatchPolicy: config.FailNoMatchPolicy,
	}, {
		name:                      "ignore no match policy skips verification when no matching policies",
		pipeline:                  unsignedPipeline,
		source:                    mismatchedSource,
		verificationNoMatchPolicy: config.IgnoreNoMatchPolicy,
	}, {
		name:                      "warn no match policy skips verification when no matching policies",
		pipeline:                  unsignedPipeline,
		source:                    mismatchedSource,
		verificationNoMatchPolicy: config.WarnNoMatchPolicy,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := test.SetupTrustedResourceConfig(context.Background(), tc.verificationNoMatchPolicy)
			err := VerifyPipeline(ctx, tc.pipeline, k8sclient, tc.source, vps)
			if err != nil {
				t.Fatalf("VerifyPipeline() get err: %v", err)
			}
		})
	}
}

func TestVerifyPipeline_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.FailNoMatchPolicy)
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Annotations["random"] = "attack"

	matchingSource := "git+https://github.com/tektoncd/catalog.git"
	mismatchedSource := "wrong source"
	tcs := []struct {
		name               string
		pipeline           v1beta1.PipelineObject
		source             string
		verificationPolicy []*v1alpha1.VerificationPolicy
	}{{
		name:               "Tampered Task Fails Verification with tampered content",
		pipeline:           tamperedPipeline,
		source:             matchingSource,
		verificationPolicy: vps,
	}, {
		name:               "Task Not Matching Pattern Fails Verification",
		pipeline:           signedPipeline,
		source:             mismatchedSource,
		verificationPolicy: vps,
	}, {
		name:     "Verification fails with regex error",
		pipeline: signedPipeline,
		source:   "git+https://github.com/tektoncd/catalog.git",
		verificationPolicy: []*v1alpha1.VerificationPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vp",
				},
				Spec: v1alpha1.VerificationPolicySpec{
					Resources: []v1alpha1.ResourcePattern{{
						Pattern: "^[",
					}},
				},
			},
		},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPipeline(ctx, tc.pipeline, k8sclient, tc.source, tc.verificationPolicy)
			if err == nil {
				t.Fatalf("VerifyPipeline() expects to get err but got nil")
			}
		})
	}
}

func TestPrepareObjectMeta(t *testing.T) {
	unsigned := test.GetUnsignedTask("test-task").ObjectMeta

	signed := unsigned.DeepCopy()
	sig := "tY805zV53PtwDarK3VD6dQPx5MbIgctNcg/oSle+MG0="
	signed.Annotations = map[string]string{SignatureAnnotation: sig}

	signedWithLabels := signed.DeepCopy()
	signedWithLabels.Labels = map[string]string{"label": "foo"}

	signedWithExtraAnnotations := signed.DeepCopy()
	signedWithExtraAnnotations.Annotations["kubectl-client-side-apply"] = "client"
	signedWithExtraAnnotations.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = "config"

	tcs := []struct {
		name              string
		objectmeta        *metav1.ObjectMeta
		expected          metav1.ObjectMeta
		expectedSignature string
	}{{
		name:       "Prepare signed objectmeta without labels",
		objectmeta: signed,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		expectedSignature: sig,
	}, {
		name:       "Prepare signed objectmeta with labels",
		objectmeta: signedWithLabels,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Labels:      map[string]string{"label": "foo"},
			Annotations: map[string]string{},
		},
		expectedSignature: sig,
	}, {
		name:       "Prepare signed objectmeta with extra annotations",
		objectmeta: signedWithExtraAnnotations,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		expectedSignature: sig,
	}, {
		name:       "resource without signature shouldn't fail",
		objectmeta: &unsigned,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		expectedSignature: "",
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			task, signature, err := prepareObjectMeta(*tc.objectmeta)
			if err != nil {
				t.Fatalf("got unexpected err: %v", err)
			}
			if d := cmp.Diff(task, tc.expected); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			got := base64.StdEncoding.EncodeToString(signature)
			if d := cmp.Diff(got, tc.expectedSignature); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
