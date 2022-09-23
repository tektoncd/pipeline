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
	"testing"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

func TestVerifyInterface_Task(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	// get signerverifer
	sv, _, err := signature.NewDefaultECDSASignerVerifier()
	if err != nil {
		t.Fatalf("failed to get signerverifier %v", err)
	}

	unsignedTask := getUnsignedTask("test-task")

	signedTask, err := getSignedTask(unsignedTask, sv)
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
		name:        "Signed Task Pass Verification",
		task:        signedTask,
		expectedErr: nil,
	}, {
		name:        "Unsigned Task Fail Verification",
		task:        unsignedTask,
		expectedErr: fmt.Errorf("invalid signature when validating ASN.1 encoded signature"),
	}, {
		name:        "task Fail Verification with empty task",
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
				if sig, ok := tc.task.Annotations[signatureAnnotation]; ok {
					delete(tc.task.Annotations, signatureAnnotation)
					signature, err = base64.StdEncoding.DecodeString(sig)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			err := verifyInterface(ctx, tc.task, sv, signature)
			if (err != nil) && (err.Error() != tc.expectedErr.Error()) {
				t.Fatalf("VerifyInterface() get err %v, wantErr %t", err, tc.expectedErr)
			}
		})
	}

}

func getUnsignedTask(name string) *v1beta1.Task {
	return &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "tekton-pipelines",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "ubuntu",
				Name:  "echo",
			}},
		},
	}
}

func getSignedTask(unsigned *v1beta1.Task, signer signature.Signer) (*v1beta1.Task, error) {
	signedTask := unsigned.DeepCopy()
	signedTask.Name = "signed"
	if signedTask.Annotations == nil {
		signedTask.Annotations = map[string]string{}
	}
	signature, err := signInterface(signer, signedTask)
	if err != nil {
		return nil, err
	}
	signedTask.Annotations[signatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signedTask, nil
}
