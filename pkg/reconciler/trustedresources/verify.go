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
	"crypto/sha256"
	"encoding/json"

	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	signatureAnnotation = "tekton.dev/signature"
)

// verifyInterface get the checksum of json marshalled object and verify it.
func verifyInterface(
	ctx context.Context,
	obj interface{},
	verifier signature.Verifier,
	signature []byte,
) error {
	ts, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(ts)

	if err := verifier.VerifySignature(bytes.NewReader(signature), bytes.NewReader(h.Sum(nil))); err != nil {
		return err
	}

	return nil
}
