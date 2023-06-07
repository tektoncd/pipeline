//
// Copyright 2021 The Sigstore Authors.
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

package azure

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

var azureSupportedHashFuncs = []crypto.Hash{
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
}

//nolint:revive
const (
	AlgorithmES256 = "ES256"
	AlgorithmES384 = "ES384"
	AlgorithmES512 = "ES512"
)

var azureSupportedAlgorithms = []string{
	AlgorithmES256,
	AlgorithmES384,
	AlgorithmES512,
}

// SignerVerifier creates and verifies digital signatures over a message using Azure KMS service
type SignerVerifier struct {
	defaultCtx context.Context
	hashFunc   crypto.Hash
	client     *azureVaultClient
}

// LoadSignerVerifier generates signatures using the specified key in Azure Key Vault and hash algorithm.
//
// It also can verify signatures locally using the public key. hashFunc must not be crypto.Hash(0).
func LoadSignerVerifier(defaultCtx context.Context, referenceStr string, hashFunc crypto.Hash) (*SignerVerifier, error) {
	a := &SignerVerifier{
		defaultCtx: defaultCtx,
	}

	var err error
	a.client, err = newAzureKMS(referenceStr)
	if err != nil {
		return nil, err
	}

	switch hashFunc {
	case 0, crypto.SHA256, crypto.SHA384, crypto.SHA512:
		a.hashFunc = hashFunc
	default:
		return nil, errors.New("hash function not supported by Azure Key Vault")
	}

	return a, nil
}

// SignMessage signs the provided message using Azure Key Vault. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the Signer was created.
//
// SignMessage recognizes the following Options listed in order of preference:
//
// - WithContext()
//
// - WithDigest()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (a *SignerVerifier) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	ctx := context.Background()
	var digest []byte
	var signerOpts crypto.SignerOpts = a.hashFunc

	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	digest, _, err := signature.ComputeDigestForSigning(message, signerOpts.HashFunc(), azureSupportedHashFuncs, opts...)
	if err != nil {
		return nil, err
	}

	rawSig, err := a.client.sign(ctx, digest, a.hashFunc)
	if err != nil {
		return nil, err
	}

	l := len(rawSig)
	r, s := &big.Int{}, &big.Int{}
	r.SetBytes(rawSig[0 : l/2])
	s.SetBytes(rawSig[l/2:])

	// Convert the concatenated r||s byte string to an ASN.1 sequence
	// This logic is borrowed from https://cs.opensource.google/go/go/+/refs/tags/go1.17.3:src/crypto/ecdsa/ecdsa.go;l=121
	var b cryptobyte.Builder
	b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
		b.AddASN1BigInt(r)
		b.AddASN1BigInt(s)
	})

	return b.Bytes()
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
func (a *SignerVerifier) VerifySignature(sig, message io.Reader, opts ...signature.VerifyOption) error {
	ctx := context.Background()
	var digest []byte
	var signerOpts crypto.SignerOpts = a.hashFunc
	for _, opt := range opts {
		opt.ApplyDigest(&digest)
	}

	digest, _, err := signature.ComputeDigestForVerifying(message, signerOpts.HashFunc(), azureSupportedHashFuncs, opts...)
	if err != nil {
		return err
	}

	sigBytes, err := io.ReadAll(sig)
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}

	// Convert the ASN.1 Sequence to a concatenated r||s byte string
	// This logic is borrowed from https://cs.opensource.google/go/go/+/refs/tags/go1.17.3:src/crypto/ecdsa/ecdsa.go;l=339
	var (
		r, s  = &big.Int{}, &big.Int{}
		inner cryptobyte.String
	)
	input := cryptobyte.String(sigBytes)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(r) ||
		!inner.ReadASN1Integer(s) ||
		!inner.Empty() {
		return errors.New("parsing signature")
	}

	rawSigBytes := []byte{}
	rawSigBytes = append(rawSigBytes, r.Bytes()...)
	rawSigBytes = append(rawSigBytes, s.Bytes()...)
	return a.client.verify(ctx, rawSigBytes, digest, a.hashFunc)
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. All options provided in arguments to this method are ignored.
func (a *SignerVerifier) PublicKey(_ ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return a.client.public(context.Background())
}

// CreateKey attempts to create a new key in Vault with the specified algorithm.
func (a *SignerVerifier) CreateKey(ctx context.Context, _ string) (crypto.PublicKey, error) {
	return a.client.createKey(ctx)
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
	azOptions := []signature.SignOption{
		options.WithContext(c.ctx),
		options.WithDigest(digest),
		options.WithCryptoSignerOpts(hashFunc),
	}

	return c.sv.SignMessage(nil, azOptions...)
}

// CryptoSigner returns a crypto.Signer object that uses the underlying SignerVerifier, along with a crypto.SignerOpts object
// that allows the KMS to be used in APIs that only accept the standard golang objects
func (a *SignerVerifier) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	csw := &cryptoSignerWrapper{
		ctx:      ctx,
		sv:       a,
		hashFunc: a.hashFunc,
		errFunc:  errFunc,
	}

	return csw, a.hashFunc, nil
}

// SupportedAlgorithms returns the list of algorithms supported by the Azure KMS service
func (*SignerVerifier) SupportedAlgorithms() []string {
	return azureSupportedAlgorithms
}

// DefaultAlgorithm returns the default algorithm for the Azure KMS service
func (*SignerVerifier) DefaultAlgorithm() string {
	return AlgorithmES256
}
