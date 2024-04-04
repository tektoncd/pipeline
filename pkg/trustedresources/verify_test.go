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
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/trustedresources/verifier"
	test "github.com/tektoncd/pipeline/test"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

const (
	namespace = "trusted-resources"
)

var (
	unsignedV1beta1Task = &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   "trusted-resources",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "ubuntu",
				Name:  "echo",
			}},
		},
	}
	unsignedV1Task = v1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Task",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "task",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Image: "ubuntu",
				Name:  "echo",
			}},
		},
	}
	unsignedV1beta1Pipeline = &v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Pipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pipeline",
			Namespace:   "trusted-resources",
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
	unsignedV1Pipeline = v1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Pipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pipeline",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1.PipelineSpec{
			Tasks: []v1.PipelineTask{
				{
					Name: "task",
				},
			},
		},
	}
)

func TestVerifyResource_Task_Success(t *testing.T) {
	signer256, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	unsignedTask := unsignedV1beta1Task
	signedTask, err := test.GetSignedV1beta1Task(unsignedTask, signer256, "signed")
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

	warnNoKeyPolicy := &v1alpha1.VerificationPolicy{
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
			Mode: v1alpha1.ModeWarn,
		},
	}

	signedTask384, err := test.GetSignedV1beta1Task(unsignedTask, signer384, "signed384")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	mismatchedSource := "wrong source"
	tcs := []struct {
		name                       string
		task                       *v1beta1.Task
		source                     *v1.RefSource
		signer                     signature.SignerVerifier
		verificationNoMatchPolicy  string
		verificationPolicies       []*v1alpha1.VerificationPolicy
		expectedVerificationResult VerificationResult
	}{{
		name:                       "signed git source task passes verification",
		task:                       signedTask,
		source:                     &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		verificationPolicies:       vps,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationPass},
	}, {
		name:                       "signed bundle source task passes verification",
		task:                       signedTask,
		source:                     &v1.RefSource{URI: "gcr.io/tekton-releases/catalog/upstream/git-clone"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		verificationPolicies:       vps,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationPass},
	}, {
		name:                       "signed task with sha384 key",
		task:                       signedTask384,
		source:                     &v1.RefSource{URI: "gcr.io/tekton-releases/catalog/upstream/sha384"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		verificationPolicies:       []*v1alpha1.VerificationPolicy{sha384Vp},
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationPass},
	}, {
		name:                       "ignore no match policy skips verification when no matching policies",
		task:                       unsignedTask,
		source:                     &v1.RefSource{URI: mismatchedSource},
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationSkip},
	}, {
		name:                       "warn no match policy skips verification when no matching policies",
		task:                       unsignedTask,
		source:                     &v1.RefSource{URI: mismatchedSource},
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationWarn, Err: ErrNoMatchedPolicies},
	}, {
		name:                       "unsigned task matches warn policy doesn't fail verification",
		task:                       unsignedTask,
		source:                     &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		verificationPolicies:       []*v1alpha1.VerificationPolicy{warnPolicy},
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationWarn, Err: ErrResourceVerificationFailed},
	}, {
		name:                       "modified task matches warn policy doesn't fail verification",
		task:                       modifiedTask,
		source:                     &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		verificationPolicies:       []*v1alpha1.VerificationPolicy{warnPolicy},
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationWarn, Err: ErrResourceVerificationFailed},
	}, {
		name:                       "modified task matches warn policy with empty key doesn't fail verification",
		task:                       modifiedTask,
		source:                     &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		verificationPolicies:       []*v1alpha1.VerificationPolicy{warnNoKeyPolicy},
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationWarn, Err: verifier.ErrEmptyPublicKeys},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := test.SetupTrustedResourceConfig(context.Background(), tc.verificationNoMatchPolicy)
			vr := VerifyResource(ctx, tc.task, k8sclient, tc.source, tc.verificationPolicies)
			if tc.expectedVerificationResult.VerificationResultType != vr.VerificationResultType && errors.Is(vr.Err, tc.expectedVerificationResult.Err) {
				t.Errorf("VerificationResult mismatch: want %v, got %v", tc.expectedVerificationResult, vr)
			}
		})
	}
}

func TestVerifyResource_Task_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.FailNoMatchPolicy)
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedTask := unsignedV1beta1Task

	signedTask, err := test.GetSignedV1beta1Task(unsignedTask, sv, "signed")
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
		source             *v1.RefSource
		verificationPolicy []*v1alpha1.VerificationPolicy
		expectedError      error
	}{{
		name:               "unsigned Task fails verification",
		task:               unsignedTask,
		source:             &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
		verificationPolicy: vps,
		expectedError:      ErrResourceVerificationFailed,
	}, {
		name:               "modified Task fails verification",
		task:               tamperedTask,
		source:             &v1.RefSource{URI: matchingSource},
		verificationPolicy: vps,
		expectedError:      ErrResourceVerificationFailed,
	}, {
		name:               "task not matching pattern fails verification",
		task:               signedTask,
		source:             &v1.RefSource{URI: mismatchedSource},
		verificationPolicy: vps,
		expectedError:      ErrNoMatchedPolicies,
	}, {
		name:               "verification fails with empty policy",
		task:               tamperedTask,
		source:             &v1.RefSource{URI: matchingSource},
		verificationPolicy: []*v1alpha1.VerificationPolicy{},
		expectedError:      ErrNoMatchedPolicies,
	}, {
		name:   "Verification fails with regex error",
		task:   signedTask,
		source: &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
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
		source: &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
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
			vr := VerifyResource(ctx, tc.task, k8sclient, tc.source, tc.verificationPolicy)
			if !errors.Is(vr.Err, tc.expectedError) && vr.VerificationResultType == VerificationError {
				t.Errorf("VerifyResource got: %v, want: %v", err, tc.expectedError)
			}
		})
	}
}

func TestVerifyResource_Pipeline_Success(t *testing.T) {
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	unsignedPipeline := unsignedV1beta1Pipeline
	signedPipeline, err := test.GetSignedV1beta1Pipeline(unsignedPipeline, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	mismatchedSource := "wrong source"
	tcs := []struct {
		name                       string
		pipeline                   *v1beta1.Pipeline
		source                     *v1.RefSource
		verificationNoMatchPolicy  string
		expectedVerificationResult VerificationResult
	}{{
		name:                       "signed git source pipeline passes verification",
		pipeline:                   signedPipeline,
		source:                     &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationPass},
	}, {
		name:                       "signed bundle source pipeline passes verification",
		pipeline:                   signedPipeline,
		source:                     &v1.RefSource{URI: "gcr.io/tekton-releases/catalog/upstream/git-clone"},
		verificationNoMatchPolicy:  config.FailNoMatchPolicy,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationPass},
	}, {
		name:                       "ignore no match policy skips verification when no matching policies",
		pipeline:                   unsignedPipeline,
		source:                     &v1.RefSource{URI: mismatchedSource},
		verificationNoMatchPolicy:  config.IgnoreNoMatchPolicy,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationSkip},
	}, {
		name:                       "warn no match policy skips verification when no matching policies",
		pipeline:                   unsignedPipeline,
		source:                     &v1.RefSource{URI: mismatchedSource},
		verificationNoMatchPolicy:  config.WarnNoMatchPolicy,
		expectedVerificationResult: VerificationResult{VerificationResultType: VerificationWarn, Err: ErrNoMatchedPolicies},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := test.SetupTrustedResourceConfig(context.Background(), tc.verificationNoMatchPolicy)
			vr := VerifyResource(ctx, tc.pipeline, k8sclient, tc.source, vps)
			if tc.expectedVerificationResult.VerificationResultType != vr.VerificationResultType && errors.Is(vr.Err, tc.expectedVerificationResult.Err) {
				t.Errorf("VerificationResult mismatch: want %v, got %v", tc.expectedVerificationResult, vr)
			}
		})
	}
}

func TestVerifyResource_Pipeline_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())
	ctx = test.SetupTrustedResourceConfig(ctx, config.FailNoMatchPolicy)
	sv, _, k8sclient, vps := test.SetupVerificationPolicies(t)

	unsignedPipeline := unsignedV1beta1Pipeline

	signedPipeline, err := test.GetSignedV1beta1Pipeline(unsignedPipeline, sv, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}
	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Annotations["random"] = "attack"

	matchingSource := "git+https://github.com/tektoncd/catalog.git"
	mismatchedSource := "wrong source"
	tcs := []struct {
		name               string
		pipeline           *v1beta1.Pipeline
		source             *v1.RefSource
		verificationPolicy []*v1alpha1.VerificationPolicy
		expectedError      error
	}{{
		name:               "Tampered Task Fails Verification with tampered content",
		pipeline:           tamperedPipeline,
		source:             &v1.RefSource{URI: matchingSource},
		verificationPolicy: vps,
		expectedError:      ErrResourceVerificationFailed,
	}, {
		name:               "Task Not Matching Pattern Fails Verification",
		pipeline:           signedPipeline,
		source:             &v1.RefSource{URI: mismatchedSource},
		verificationPolicy: vps,
		expectedError:      ErrNoMatchedPolicies,
	}, {
		name:     "Verification fails with regex error",
		pipeline: signedPipeline,
		source:   &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"},
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
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			vr := VerifyResource(ctx, tc.pipeline, k8sclient, tc.source, tc.verificationPolicy)
			if !errors.Is(vr.Err, tc.expectedError) && vr.VerificationResultType == VerificationError {
				t.Errorf("VerifyResource got: %v, want: %v", vr.Err, tc.expectedError)
			}
		})
	}
}

func TestVerifyResource_V1Task_Success(t *testing.T) {
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	signedTask, err := getSignedV1Task(unsignedV1Task.DeepCopy(), signer, "signed")
	if err != nil {
		t.Error(err)
	}
	vr := VerifyResource(context.Background(), signedTask, k8sclient, &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"}, vps)
	if vr.VerificationResultType != VerificationPass {
		t.Errorf("VerificationResult mismatch: want %v, got %v", VerificationPass, vr.VerificationResultType)
	}
}

func TestVerifyResource_V1Task_Error(t *testing.T) {
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	signedTask, err := getSignedV1Task(unsignedV1Task.DeepCopy(), signer, "signed")
	if err != nil {
		t.Error(err)
	}
	modifiedTask := signedTask.DeepCopy()
	modifiedTask.Annotations["foo"] = "modified"
	vr := VerifyResource(context.Background(), modifiedTask, k8sclient, &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"}, vps)
	if vr.VerificationResultType != VerificationError && !errors.Is(vr.Err, ErrResourceVerificationFailed) {
		t.Errorf("VerificationResult mismatch: want %v, got %v", VerificationResult{VerificationResultType: VerificationError, Err: ErrResourceVerificationFailed}, vr)
	}
}

func TestVerifyResource_V1Pipeline_Success(t *testing.T) {
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	signed, err := getSignedV1Pipeline(unsignedV1Pipeline.DeepCopy(), signer, "signed")
	if err != nil {
		t.Error(err)
	}
	vr := VerifyResource(context.Background(), signed, k8sclient, &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"}, vps)
	if vr.VerificationResultType != VerificationPass {
		t.Errorf("VerificationResult mismatch: want %v, got %v", VerificationPass, vr.VerificationResultType)
	}
}

func TestVerifyResource_V1Pipeline_Error(t *testing.T) {
	signer, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	signed, err := getSignedV1Pipeline(unsignedV1Pipeline.DeepCopy(), signer, "signed")
	if err != nil {
		t.Error(err)
	}
	modifiedTask := signed.DeepCopy()
	modifiedTask.Annotations["foo"] = "modified"
	vr := VerifyResource(context.Background(), modifiedTask, k8sclient, &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"}, vps)
	if vr.VerificationResultType != VerificationError && !errors.Is(vr.Err, ErrResourceVerificationFailed) {
		t.Errorf("VerificationResult mismatch: want %v, got %v", VerificationResult{VerificationResultType: VerificationError, Err: ErrResourceVerificationFailed}, vr)
	}
}

func TestVerifyResource_TypeNotSupported(t *testing.T) {
	resource := v1beta1.ClusterTask{}
	refSource := &v1.RefSource{URI: "git+https://github.com/tektoncd/catalog.git"}
	_, _, k8sclient, vps := test.SetupVerificationPolicies(t)
	vr := VerifyResource(context.Background(), &resource, k8sclient, refSource, vps)
	if !errors.Is(vr.Err, ErrResourceNotSupported) {
		t.Errorf("want:%v got:%v ", ErrResourceNotSupported, vr.Err)
	}
}

func signInterface(signer signature.Signer, i interface{}) ([]byte, error) {
	if signer == nil {
		return nil, errors.New("signer is nil")
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

func getSignedV1Task(unsigned *v1.Task, signer signature.Signer, name string) (*v1.Task, error) {
	signed := unsigned.DeepCopy()
	signed.Name = name
	if signed.Annotations == nil {
		signed.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signed)
	if err != nil {
		return nil, err
	}
	signed.Annotations[SignatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signed, nil
}

func getSignedV1Pipeline(unsigned *v1.Pipeline, signer signature.Signer, name string) (*v1.Pipeline, error) {
	signed := unsigned.DeepCopy()
	signed.Name = name
	if signed.Annotations == nil {
		signed.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signed)
	if err != nil {
		return nil, err
	}
	signed.Annotations[SignatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signed, nil
}
