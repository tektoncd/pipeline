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

package test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestSignInterface(t *testing.T) {
	sv, _, err := signature.NewDefaultECDSASignerVerifier()
	if err != nil {
		t.Fatalf("failed to get signerverifier %v", err)
	}

	var mocksigner mockSigner

	tcs := []struct {
		name     string
		signer   signature.SignerVerifier
		target   interface{}
		expected string
		wantErr  bool
	}{{
		name:    "Sign Task",
		signer:  sv,
		target:  GetUnsignedTask("unsigned"),
		wantErr: false,
	}, {
		name:    "Sign String with cosign signer",
		signer:  sv,
		target:  "Hello world",
		wantErr: false,
	}, {
		name:    "Empty TaskSpec",
		signer:  sv,
		target:  nil,
		wantErr: false,
	}, {
		name:    "Empty Signer",
		signer:  nil,
		target:  GetUnsignedTask("unsigned"),
		wantErr: true,
	}, {
		name:     "Sign String with mock signer",
		signer:   mocksigner,
		target:   "Hello world",
		expected: "tY805zV53PtwDarK3VD6dQPx5MbIgctNcg/oSle+MG0=",
		wantErr:  false,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			sig, err := signInterface(tc.signer, tc.target)
			if (err != nil) != tc.wantErr {
				t.Fatalf("SignInterface() get err %v, wantErr %t", err, tc.wantErr)
			}

			if tc.expected != "" {
				signature := base64.StdEncoding.EncodeToString(sig)
				if d := cmp.Diff(signature, tc.expected); d != "" {
					t.Fatalf("Diff:\n%s", diff.PrintWantGot(d))
				}
				return
			}

			if tc.wantErr {
				return
			}

			ts, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}

			h := sha256.New()
			h.Write(ts)

			if err := tc.signer.VerifySignature(bytes.NewReader(sig), bytes.NewReader(h.Sum(nil))); err != nil {
				t.Fatalf("SignInterface() generate wrong signature: %v", err)
			}
		})
	}
}

type mockSigner struct {
	signature.SignerVerifier
}

func (mockSigner) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	return io.ReadAll(message)
}
