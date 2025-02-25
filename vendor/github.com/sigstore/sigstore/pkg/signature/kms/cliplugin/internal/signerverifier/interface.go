//
// Copyright 2025 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package signerverifier contains interface for to be implemented by KMSs.
package signerverifier

import (
	"context"
	"crypto"

	"github.com/sigstore/sigstore/pkg/signature"
)

// SignerVerifier creates and verifies digital signatures over a message using a KMS service
// The contents must be kept in sync with kms.SignerVerifier, to continue satisfying that interface.
// We don't directly embed kms.SignerVerfifier because then we would have an import cycle.
type SignerVerifier interface {
	signature.SignerVerifier
	CreateKey(ctx context.Context, algorithm string) (crypto.PublicKey, error)
	CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error)
	SupportedAlgorithms() []string
	DefaultAlgorithm() string
}
