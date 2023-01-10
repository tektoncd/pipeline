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
	"github.com/tektoncd/pipeline/pkg/apis/objectInterface"
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
		expectedError: ErrorResourceVerificationFailed,
	}, {
		name:          "Empty task Fail Verification",
		task:          nil,
		expectedError: ErrorResourceVerificationFailed,
	}, {
		name:          "Tampered task Fail Verification",
		task:          tamperedTask,
		expectedError: ErrorResourceVerificationFailed,
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

func TestVerifyTask_Configmap_Success(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath := test.GetSignerFromFile(ctx, t)

	ctx = test.SetupTrustedResourceKeyConfig(ctx, keypath, config.EnforceResourceVerificationMode)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	err = VerifyTask(ctx, *signedTask, nil, "", []*v1alpha1.VerificationPolicy{})
	if err != nil {
		t.Errorf("VerifyTask() get err %v", err)
	}
}

func TestVerifyTask_Configmap_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath := test.GetSignerFromFile(ctx, t)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"

	tcs := []struct {
		name          string
		task          *v1beta1.Task
		keypath       string
		expectedError error
	}{{
		name:          "modified Task fails verification",
		task:          tamperedTask,
		keypath:       keypath,
		expectedError: ErrorResourceVerificationFailed,
	}, {
		name:          "unsigned Task fails verification",
		task:          unsignedTask,
		keypath:       keypath,
		expectedError: ErrorSignatureMissing,
	}, {
		name:          "fail to load key from configmap",
		task:          signedTask,
		keypath:       "wrongPath",
		expectedError: verifier.ErrorFailedLoadKeyFile,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx = test.SetupTrustedResourceKeyConfig(ctx, tc.keypath, config.EnforceResourceVerificationMode)
			err := VerifyTask(ctx, *tc.task, nil, "", []*v1alpha1.VerificationPolicy{})
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("VerifyTask got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestVerifyTask_VerificationPolicy_Success(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.EnforceResourceVerificationMode)
	signer256, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, signer256, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

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
	vps = append(vps, sha384Vp)

	signedTask384, err := test.GetSignedTask(unsignedTask, signer384, "signed384")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tcs := []struct {
		name   string
		task   *v1beta1.Task
		source string
		signer signature.SignerVerifier
	}{{
		name:   "signed git source task passes verification",
		task:   signedTask,
		source: "git+https://github.com/tektoncd/catalog.git",
	}, {
		name:   "signed bundle source task passes verification",
		task:   signedTask,
		source: "gcr.io/tekton-releases/catalog/upstream/git-clone",
	}, {
		name:   "signed task with sha384 key",
		task:   signedTask384,
		source: "gcr.io/tekton-releases/catalog/upstream/sha384",
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyTask(ctx, *tc.task, k8sclient, tc.source, vps)
			if err != nil {
				t.Fatalf("VerifyTask() get err %v", err)
			}
		})
	}
}

func TestVerifyTask_VerificationPolicy_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.EnforceResourceVerificationMode)
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"

	tcs := []struct {
		name               string
		task               v1beta1.Task
		source             string
		verificationPolicy []*v1alpha1.VerificationPolicy
		expectedError      error
	}{{
		name:               "modified Task fails verification",
		task:               *tamperedTask,
		source:             "git+https://github.com/tektoncd/catalog.git",
		verificationPolicy: vps,
		expectedError:      ErrorResourceVerificationFailed,
	}, {
		name:               "task not matching pattern fails verification",
		task:               *signedTask,
		source:             "wrong source",
		verificationPolicy: vps,
		expectedError:      ErrorNoMatchedPolicies,
	}, {
		name:               "verification fails with empty policy",
		task:               *tamperedTask,
		source:             "git+https://github.com/tektoncd/catalog.git",
		verificationPolicy: []*v1alpha1.VerificationPolicy{},
		expectedError:      ErrorEmptyVerificationConfig,
	}, {
		name:   "Verification fails with regex error",
		task:   *signedTask,
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
		expectedError: ErrorRegexMatch,
	}, {
		name:   "Verification fails with error from policy",
		task:   *signedTask,
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
		expectedError: verifier.ErrorDecodeKey,
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
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.EnforceResourceVerificationMode)
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tcs := []struct {
		name     string
		pipeline objectInterface.PipelineObject
		source   string
	}{{
		name:     "Signed git source Task Passes Verification",
		pipeline: signedPipeline,
		source:   "git+https://github.com/tektoncd/catalog.git",
	}, {
		name:     "Signed bundle source Task Passes Verification",
		pipeline: signedPipeline,
		source:   "gcr.io/tekton-releases/catalog/upstream/git-clone",
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPipeline(ctx, tc.pipeline, k8sclient, tc.source, vps)
			if err != nil {
				t.Fatalf("VerifyPipeline() get err: %v", err)
			}
		})
	}
}

func TestVerifyPipeline_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.EnforceResourceVerificationMode)
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Annotations["random"] = "attack"

	tcs := []struct {
		name     string
		pipeline objectInterface.PipelineObject
		source   string
	}{{
		name:     "Tampered Task Fails Verification with tampered content",
		pipeline: tamperedPipeline,
		source:   "git+https://github.com/tektoncd/catalog.git",
	}, {
		name:     "Task Not Matching Pattern Fails Verification",
		pipeline: signedPipeline,
		source:   "wrong source",
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPipeline(ctx, tc.pipeline, k8sclient, tc.source, vps)
			if err == nil {
				t.Fatalf("VerifyPipeline() expects to get err but got nil")
			}
		})
	}
}

func TestPrepareObjectMeta(t *testing.T) {
	unsigned := test.GetUnsignedTask("test-task").ObjectMeta

	signed := unsigned.DeepCopy()
	signed.Annotations = map[string]string{SignatureAnnotation: "tY805zV53PtwDarK3VD6dQPx5MbIgctNcg/oSle+MG0="}

	signedWithLabels := signed.DeepCopy()
	signedWithLabels.Labels = map[string]string{"label": "foo"}

	signedWithExtraAnnotations := signed.DeepCopy()
	signedWithExtraAnnotations.Annotations["kubectl-client-side-apply"] = "client"
	signedWithExtraAnnotations.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = "config"

	tcs := []struct {
		name       string
		objectmeta *metav1.ObjectMeta
		expected   metav1.ObjectMeta
		wantErr    bool
	}{{
		name:       "Prepare signed objectmeta without labels",
		objectmeta: signed,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		wantErr: false,
	}, {
		name:       "Prepare signed objectmeta with labels",
		objectmeta: signedWithLabels,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Labels:      map[string]string{"label": "foo"},
			Annotations: map[string]string{},
		},
		wantErr: false,
	}, {
		name:       "Prepare signed objectmeta with extra annotations",
		objectmeta: signedWithExtraAnnotations,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		wantErr: false,
	}, {
		name:       "Fail preparation without signature",
		objectmeta: &unsigned,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		wantErr: true,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			task, signature, err := prepareObjectMeta(*tc.objectmeta)
			if (err != nil) != tc.wantErr {
				t.Fatalf("prepareObjectMeta() get err %v, wantErr %t", err, tc.wantErr)
			}
			if d := cmp.Diff(task, tc.expected); d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			if tc.wantErr {
				return
			}
			if signature == nil {
				t.Fatal("signature is not extracted")
			}
		})
	}
}
