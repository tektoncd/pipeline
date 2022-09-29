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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	test "github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	"go.uber.org/zap/zaptest"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/logging"
)

const (
	namespace = "trusted-resources"
)

func TestVerifyInterface_Task_Success(t *testing.T) {
	// get signerverifer
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

	err = VerifyInterface(signedTask, sv, signature)
	if err != nil {
		t.Fatalf("VerifyInterface() get err %v", err)
	}

}

func TestVerifyInterface_Task_Error(t *testing.T) {
	// get signerverifer
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
		name        string
		task        *v1beta1.Task
		expectedErr error
	}{{
		name:        "Unsigned Task Fail Verification",
		task:        unsignedTask,
		expectedErr: fmt.Errorf("invalid signature when validating ASN.1 encoded signature"),
	}, {
		name:        "Empty task Fail Verification",
		task:        nil,
		expectedErr: fmt.Errorf("invalid signature when validating ASN.1 encoded signature"),
	}, {
		name:        "Tampered task Fail Verification",
		task:        tamperedTask,
		expectedErr: fmt.Errorf("invalid signature when validating ASN.1 encoded signature"),
	},
	}

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

			err := VerifyInterface(tc.task, sv, signature)
			if err == nil {
				t.Fatalf("verifyTaskRun() expects to get err but got nil")
			}
			if (err != nil) && (err.Error() != tc.expectedErr.Error()) {
				t.Fatalf("VerifyInterface() get err %v, wantErr %t", err, tc.expectedErr)
			}
		})
	}

}

func TestVerifyTask_Success(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	ctx = test.SetupTrustedResourceConfig(ctx, keypath, config.EnforceResourceVerificationMode)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	err = VerifyTask(ctx, signedTask, nil)
	if err != nil {
		t.Fatalf("verifyTaskRun() get err %v", err)
	}

}

func TestVerifyTask_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	ctx = test.SetupTrustedResourceConfig(ctx, keypath, config.EnforceResourceVerificationMode)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"

	tcs := []struct {
		name string
		task v1beta1.TaskObject
	}{{
		name: "Tampered Task Fails Verification with tampered content",
		task: tamperedTask,
	}, {
		name: "Unsigned Task Fails Verification without signature",
		task: unsignedTask,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyTask(ctx, tc.task, nil)
			if err == nil {
				t.Fatalf("verifyTaskRun() expects to get err but got nil")
			}
		})
	}

}

func TestVerifyPipeline_Success(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	ctx = test.SetupTrustedResourceConfig(ctx, keypath, config.EnforceResourceVerificationMode)

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	err = VerifyPipeline(ctx, signedPipeline, nil)
	if err != nil {
		t.Fatalf("VerifyPipeline() get err %v", err)
	}

}

func TestVerifyPipeline_Error(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	ctx = test.SetupTrustedResourceConfig(ctx, keypath, config.EnforceResourceVerificationMode)

	unsignedPipeline := test.GetUnsignedPipeline("test-pipeline")

	signedPipeline, err := test.GetSignedPipeline(unsignedPipeline, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Annotations["random"] = "attack"

	tcs := []struct {
		name     string
		pipeline v1beta1.PipelineObject
	}{{
		name:     "Tampered Pipeline Fails Verification with tampered content",
		pipeline: tamperedPipeline,
	}, {
		name:     "Unsigned Pipeline Fails Verification without signature",
		pipeline: unsignedPipeline,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPipeline(ctx, tc.pipeline, nil)
			if err == nil {
				t.Fatalf("VerifyPipeline() expects to get err but got nil")
			}
		})
	}

}

func TestVerifyTask_SecretRef(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath, err := test.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	fileBytes, err := ioutil.ReadFile(filepath.Clean(keypath))
	if err != nil {
		t.Fatal(err)
	}

	secret := &v1.Secret{
		Data: map[string][]byte{"cosign.pub": fileBytes},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "verification-secrets",
			Namespace: "default"}}
	kubeclient := fakek8s.NewSimpleClientset(secret)

	secretref := fmt.Sprintf("%sdefault/verification-secrets", keyReference)

	ctx = test.SetupTrustedResourceConfig(ctx, secretref, config.EnforceResourceVerificationMode)

	unsignedTask := test.GetUnsignedTask("test-task")

	signedTask, err := test.GetSignedTask(unsignedTask, signer, "signed")
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"

	tcs := []struct {
		name    string
		task    v1beta1.TaskObject
		wantErr bool
	}{{
		name:    "Signed Task Passes Verification",
		task:    signedTask,
		wantErr: false,
	}, {
		name:    "Tampered Task Fails Verification with tampered content",
		task:    tamperedTask,
		wantErr: true,
	}, {
		name:    "Unsigned Task Fails Verification without signature",
		task:    unsignedTask,
		wantErr: true,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyTask(ctx, tc.task, kubeclient)
			if (err != nil) != tc.wantErr {
				t.Fatalf("verifyTaskRun() get err %v, wantErr %t", err, tc.wantErr)
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
	},
	}

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
