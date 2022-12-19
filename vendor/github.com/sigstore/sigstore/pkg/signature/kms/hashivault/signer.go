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

package hashivault

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// Taken from https://www.vaultproject.io/api/secret/transit
// nolint:revive
const (
	AlgorithmECDSAP256 = "ecdsa-p256"
	AlgorithmECDSAP384 = "ecdsa-p384"
	AlgorithmECDSAP521 = "ecdsa-p521"
	AlgorithmED25519   = "ed25519"
	AlgorithmRSA2048   = "rsa-2048"
	AlgorithmRSA3072   = "rsa-3072"
	AlgorithmRSA4096   = "rsa-4096"
)

var hvSupportedAlgorithms = []string{
	AlgorithmECDSAP256,
	AlgorithmECDSAP384,
	AlgorithmECDSAP521,
	AlgorithmED25519,
	AlgorithmRSA2048,
	AlgorithmRSA3072,
	AlgorithmRSA4096,
}

var hvSupportedHashFuncs = []crypto.Hash{
	crypto.SHA224,
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
	crypto.Hash(0),
}

// SignerVerifier creates and verifies digital signatures over a message using Hashicorp Vault KMS service
type SignerVerifier struct {
	hashFunc crypto.Hash
	client   *hashivaultClient
}

// LoadSignerVerifier generates signatures using the specified key object in Vault and hash algorithm.
//
// It also can verify signatures (via a remote vall to the Vault instance). hashFunc should be
// set to crypto.Hash(0) if the key referred to by referenceStr is an ED25519 signing key.
func LoadSignerVerifier(referenceStr string, hashFunc crypto.Hash, opts ...signature.RPCOption) (*SignerVerifier, error) {
	h := &SignerVerifier{}
	ctx := context.Background()
	rpcAuth := options.RPCAuth{}
	var keyVersion string
	for _, opt := range opts {
		opt.ApplyRPCAuthOpts(&rpcAuth)
		opt.ApplyContext(&ctx)
		opt.ApplyKeyVersion(&keyVersion)
	}

	var keyVersionUint uint64
	var err error
	if keyVersion != "" {
		keyVersionUint, err = strconv.ParseUint(keyVersion, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing key version: %w", err)
		}
	}

	if rpcAuth.OIDC.Token != "" {
		rpcAuth.Token, err = oidcLogin(ctx, rpcAuth.Address, rpcAuth.OIDC.Path, rpcAuth.OIDC.Role, rpcAuth.OIDC.Token)
		if err != nil {
			return nil, err
		}
	}
	h.client, err = newHashivaultClient(rpcAuth.Address, rpcAuth.Token, rpcAuth.Path, referenceStr, keyVersionUint)
	if err != nil {
		return nil, err
	}

	switch hashFunc {
	case 0, crypto.SHA224, crypto.SHA256, crypto.SHA384, crypto.SHA512:
		h.hashFunc = hashFunc
	default:
		return nil, errors.New("hash function not supported by Hashivault")
	}

	return h, nil
}

// SignMessage signs the provided message using HashiCorp Vault KMS. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the HashivaultSigner was created.
//
// SignMessage recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// All other options are ignored if specified.
func (h SignerVerifier) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	var digest []byte
	var signerOpts crypto.SignerOpts = h.hashFunc

	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	digest, hf, err := signature.ComputeDigestForSigning(message, signerOpts.HashFunc(), hvSupportedHashFuncs, opts...)
	if err != nil {
		return nil, err
	}

	return h.client.sign(digest, hf, opts...)
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. All options provided in arguments to this method are ignored.
func (h SignerVerifier) PublicKey(_ ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return h.client.public()
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
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (h SignerVerifier) VerifySignature(sig, message io.Reader, opts ...signature.VerifyOption) error {
	var digest []byte
	var signerOpts crypto.SignerOpts = h.hashFunc

	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	digest, hf, err := signature.ComputeDigestForVerifying(message, signerOpts.HashFunc(), hvSupportedHashFuncs, opts...)
	if err != nil {
		return err
	}

	sigBytes, err := io.ReadAll(sig)
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}

	return h.client.verify(sigBytes, digest, hf, opts...)
}

// CreateKey attempts to create a new key in Vault with the specified algorithm.
func (h SignerVerifier) CreateKey(_ context.Context, algorithm string) (crypto.PublicKey, error) {
	return h.client.createKey(algorithm)
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
	hvOptions := []signature.SignOption{
		options.WithContext(c.ctx),
		options.WithDigest(digest),
		options.WithCryptoSignerOpts(hashFunc),
	}

	return c.sv.SignMessage(nil, hvOptions...)
}

// CryptoSigner returns a crypto.Signer object that uses the underlying SignerVerifier, along with a crypto.SignerOpts object
// that allows the KMS to be used in APIs that only accept the standard golang objects
func (h *SignerVerifier) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	csw := &cryptoSignerWrapper{
		ctx:      ctx,
		sv:       h,
		hashFunc: h.hashFunc,
		errFunc:  errFunc,
	}

	return csw, h.hashFunc, nil
}

// SupportedAlgorithms returns the list of algorithms supported by the Hashicorp Vault service
func (h *SignerVerifier) SupportedAlgorithms() []string {
	return hvSupportedAlgorithms
}

// DefaultAlgorithm returns the default algorithm for the Hashicorp Vault service
func (h *SignerVerifier) DefaultAlgorithm() string {
	return AlgorithmECDSAP256
}
