//
// Copyright 2026 The Sigstore Authors.
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

package signature

import (
	"bytes"
	"crypto"
	"crypto/mldsa"
	"errors"
	"fmt"
	"io"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

var mldsaSupportedHashFuncs = []crypto.Hash{
	crypto.Hash(0),
}

// MLDSASigner is a signature.Signer that uses the ML-DSA post-quantum signature scheme.
//
// WARNING: This is experimental and may change.
type MLDSASigner struct {
	priv *mldsa.PrivateKey
}

// validateMLDSAPrivateKey checks that the ML-DSA private key is properly initialized.
// Calling priv.PublicKey() does not panic for an uninitialized &mldsa.PrivateKey{} in current
// Go versions (the panic occurs when probing the resulting public key), but this recover is
// retained as defense-in-depth against future runtime changes.
func validateMLDSAPrivateKey(priv *mldsa.PrivateKey) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("key is invalid: %v", r)
		}
	}()
	if _, valErr := cryptoutils.ValidateMLDSAPublicKey(priv.PublicKey()); valErr != nil {
		return valErr
	}
	return nil
}

// LoadMLDSASigner calculates signatures using the specified private key.
func LoadMLDSASigner(priv *mldsa.PrivateKey) (*MLDSASigner, error) {
	if priv == nil {
		return nil, errors.New("invalid ML-DSA private key specified")
	}
	if err := validateMLDSAPrivateKey(priv); err != nil {
		return nil, fmt.Errorf("invalid ML-DSA private key specified: %w", err)
	}

	return &MLDSASigner{
		priv: priv,
	}, nil
}

// SignMessage signs the provided message using Pure ML-DSA with an empty context.
//
// Passing the WithDigest option with a digest is not supported as ML-DSA handles
// its own internal message processing. Other options are ignored.
func (m MLDSASigner) SignMessage(message io.Reader, opts ...SignOption) ([]byte, error) {
	var digest []byte
	for _, opt := range opts {
		opt.ApplyDigest(&digest)
	}
	if len(digest) > 0 {
		return nil, errors.New("WithDigest is not supported for ML-DSA")
	}
	messageBytes, _, err := ComputeDigestForSigning(message, crypto.Hash(0), mldsaSupportedHashFuncs)
	if err != nil {
		return nil, err
	}

	return m.priv.Sign(nil, messageBytes, nil)
}

// Public returns the public key that can be used to verify signatures created by
// this signer.
func (m MLDSASigner) Public() crypto.PublicKey {
	if m.priv == nil {
		return nil
	}

	return m.priv.Public()
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (m MLDSASigner) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return m.Public(), nil
}

// Sign computes the signature for the specified message using Pure ML-DSA with an empty context
// for consistency across software, KMS, and hardware backends. Callers requiring domain separation
// can enforce it at the payload level.
//
// The rand argument is ignored because ML-DSA internally generates randomness.
// If opts is non-nil, only opts with an empty context and HashFunc() == crypto.Hash(0) are supported;
// pre-hashed μ (crypto.MLDSAMu) and non-empty context strings are not permitted.
func (m MLDSASigner) Sign(_ io.Reader, message []byte, opts crypto.SignerOpts) ([]byte, error) {
	if message == nil {
		return nil, errors.New("message must not be nil")
	}
	if opts != nil {
		if opts.HashFunc() != crypto.Hash(0) {
			return nil, fmt.Errorf("unsupported hash function: %v", opts.HashFunc())
		}
		if mldsaOpts, ok := opts.(*mldsa.Options); ok && mldsaOpts.Context != "" {
			return nil, errors.New("non-empty context is not supported; use empty context for consistency across backends")
		}
	}
	return m.SignMessage(bytes.NewReader(message))
}

// MLDSAVerifier is a signature.Verifier that uses the ML-DSA post-quantum signature system.
//
// WARNING: This is experimental and may change.
type MLDSAVerifier struct {
	publicKey *mldsa.PublicKey
}

// LoadMLDSAVerifier returns a Verifier that verifies signatures using the specified ML-DSA public key.
func LoadMLDSAVerifier(pub *mldsa.PublicKey) (*MLDSAVerifier, error) {
	if pub == nil {
		return nil, errors.New("invalid ML-DSA public key specified")
	}
	if _, err := cryptoutils.ValidateMLDSAPublicKey(pub); err != nil {
		return nil, fmt.Errorf("invalid ML-DSA public key specified: %w", err)
	}

	return &MLDSAVerifier{
		publicKey: pub,
	}, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (m *MLDSAVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return m.publicKey, nil
}

// VerifySignature verifies the signature for the given message using Pure ML-DSA with an empty context.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// Passing the WithDigest option with a digest is explicitly rejected as ML-DSA does not support
// pre-hashed message digests. Other options are ignored.
func (m *MLDSAVerifier) VerifySignature(signature, message io.Reader, opts ...VerifyOption) error {
	if signature == nil {
		return errors.New("nil signature passed to VerifySignature")
	}
	var digest []byte
	for _, opt := range opts {
		opt.ApplyDigest(&digest)
	}
	if len(digest) > 0 {
		return errors.New("WithDigest is not supported for ML-DSA")
	}
	messageBytes, _, err := ComputeDigestForVerifying(message, crypto.Hash(0), mldsaSupportedHashFuncs)
	if err != nil {
		return err
	}

	sigBytes, err := io.ReadAll(signature)
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}

	return mldsa.Verify(m.publicKey, messageBytes, sigBytes, nil)
}

// MLDSASignerVerifier is a signature.SignerVerifier that uses the ML-DSA post-quantum signature system
type MLDSASignerVerifier struct {
	*MLDSASigner
	*MLDSAVerifier
}

// LoadMLDSASignerVerifier creates a combined signer and verifier. This is
// a convenience object that simply wraps an instance of MLDSASigner and MLDSAVerifier.
func LoadMLDSASignerVerifier(priv *mldsa.PrivateKey) (*MLDSASignerVerifier, error) {
	signer, err := LoadMLDSASigner(priv)
	if err != nil {
		return nil, fmt.Errorf("initializing signer: %w", err)
	}
	verifier, err := LoadMLDSAVerifier(priv.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("initializing verifier: %w", err)
	}

	return &MLDSASignerVerifier{
		MLDSASigner:   signer,
		MLDSAVerifier: verifier,
	}, nil
}

// NewDefaultMLDSASignerVerifier creates a combined signer and verifier using ML-DSA.
// This creates a new ML-DSA key using the recommended default MLDSA44 parameter set.
func NewDefaultMLDSASignerVerifier() (*MLDSASignerVerifier, *mldsa.PrivateKey, error) {
	return NewMLDSASignerVerifier(mldsa.MLDSA44())
}

// NewMLDSASignerVerifier creates a combined signer and verifier using ML-DSA.
// This creates a new ML-DSA key using the specified parameter set.
func NewMLDSASignerVerifier(params mldsa.Parameters) (*MLDSASignerVerifier, *mldsa.PrivateKey, error) {
	priv, err := mldsa.GenerateKey(params)
	if err != nil {
		return nil, nil, err
	}

	sv, err := LoadMLDSASignerVerifier(priv)
	if err != nil {
		return nil, nil, err
	}

	return sv, priv, nil
}

// PublicKey returns the public key that is used to verify signatures by
// this verifier. As this value is held in memory, all options provided in arguments
// to this method are ignored.
func (m MLDSASignerVerifier) PublicKey(_ ...PublicKeyOption) (crypto.PublicKey, error) {
	return m.publicKey, nil
}
