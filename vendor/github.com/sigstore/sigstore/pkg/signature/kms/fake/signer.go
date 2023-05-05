// Copyright 2022 The Sigstore Authors.
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

// Package fake implements fake signer to be used in tests
package fake

import (
	"context"
	"crypto"
	"io"

	"github.com/sigstore/sigstore/pkg/signature"
	sigkms "github.com/sigstore/sigstore/pkg/signature/kms"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// KmsCtxKey is used to look up the private key in the struct.
type KmsCtxKey struct{}

// SignerVerifier creates and verifies digital signatures over a message using an in-memory signer
type SignerVerifier struct {
	signer signature.SignerVerifier
}

// ReferenceScheme is a scheme for fake KMS keys. Do not use in production.
const ReferenceScheme = "fakekms://"

func init() {
	sigkms.AddProvider(ReferenceScheme, func(ctx context.Context, _ string, hf crypto.Hash, _ ...signature.RPCOption) (sigkms.SignerVerifier, error) {
		return LoadSignerVerifier(ctx, hf)
	})
}

// LoadSignerVerifier generates a signer/verifier using the default ECDSA signer or loads
// a signer from a provided private key and hash. The context should contain a mapping from
// a string "priv" to a crypto.PrivateKey (RSA, ECDSA, or ED25519).
func LoadSignerVerifier(ctx context.Context, hf crypto.Hash) (*SignerVerifier, error) {
	val := ctx.Value(KmsCtxKey{})
	if val == nil {
		signer, _, err := signature.NewDefaultECDSASignerVerifier()
		if err != nil {
			return nil, err
		}
		sv := &SignerVerifier{
			signer: signer,
		}
		return sv, nil
	}
	signer, err := signature.LoadSignerVerifier(val.(crypto.PrivateKey), hf)
	if err != nil {
		return nil, err
	}
	sv := &SignerVerifier{
		signer: signer,
	}
	return sv, nil
}

// SignMessage signs the provided message using the in-memory signer.
func (g *SignerVerifier) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	return g.signer.SignMessage(message, opts...)
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer.
func (g *SignerVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return g.signer.PublicKey(opts...)
}

// VerifySignature verifies the signature for the given message. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the SignerVerifier was created.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// All other options are ignored if specified.
func (g *SignerVerifier) VerifySignature(signature, message io.Reader, opts ...signature.VerifyOption) error {
	return g.signer.VerifySignature(signature, message, opts...)
}

// CreateKey returns the signer's public key.
func (g *SignerVerifier) CreateKey(_ context.Context, _ string) (crypto.PublicKey, error) {
	pub, err := g.signer.PublicKey()
	if err != nil {
		return nil, err
	}
	return pub, nil
}

type cryptoSignerWrapper struct {
	ctx      context.Context
	hashFunc crypto.Hash
	sv       *SignerVerifier
	errFunc  func(error)
}

func (c cryptoSignerWrapper) Public() crypto.PublicKey {
	pk, err := c.sv.PublicKey(options.WithContext(c.ctx))
	if err != nil && c.errFunc != nil {
		c.errFunc(err)
	}
	return pk
}

func (c cryptoSignerWrapper) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	hashFunc := c.hashFunc
	if opts != nil {
		hashFunc = opts.HashFunc()
	}
	gcpOptions := []signature.SignOption{
		options.WithContext(c.ctx),
		options.WithDigest(digest),
		options.WithCryptoSignerOpts(hashFunc),
	}

	return c.sv.SignMessage(nil, gcpOptions...)
}

// CryptoSigner returns a crypto.Signer object that uses the underlying SignerVerifier, along with a crypto.SignerOpts object
// that allows the KMS to be used in APIs that only accept the standard golang objects
func (g *SignerVerifier) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	csw := &cryptoSignerWrapper{
		ctx:      ctx,
		sv:       g,
		hashFunc: crypto.SHA256,
		errFunc:  errFunc,
	}

	return csw, crypto.SHA256, nil
}

// SupportedAlgorithms returns a list with the default algorithm
func (g *SignerVerifier) SupportedAlgorithms() (result []string) {
	return []string{"ecdsa-p256-sha256"}
}

// DefaultAlgorithm returns the default algorithm for the signer
func (g *SignerVerifier) DefaultAlgorithm() string {
	return "ecdsa-p256-sha256"
}
