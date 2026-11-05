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
	"crypto/ed25519"
	"crypto/mldsa"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// Taken from https://www.vaultproject.io/api/secret/transit
const (
	AlgorithmECDSAP256 = "ecdsa-p256"
	AlgorithmECDSAP384 = "ecdsa-p384"
	AlgorithmECDSAP521 = "ecdsa-p521"
	AlgorithmED25519   = "ed25519"
	AlgorithmRSA2048   = "rsa-2048"
	AlgorithmRSA3072   = "rsa-3072"
	AlgorithmRSA4096   = "rsa-4096"
	AlgorithmMLDSA     = "ml-dsa" // Vault enterprise: one type for all sets, parameter_set field carries the set
	AlgorithmMLDSA44   = "mldsa-44"
	AlgorithmMLDSA65   = "mldsa-65"
	AlgorithmMLDSA87   = "mldsa-87"
	AlgorithmMLDSA44FB = "ml-dsa-44" // Vault enterprise reports ML-DSA version entries as "ml-dsa-<set>"
	AlgorithmMLDSA65FB = "ml-dsa-65"
	AlgorithmMLDSA87FB = "ml-dsa-87"
)

var hvSupportedAlgorithms = []string{
	AlgorithmECDSAP256,
	AlgorithmECDSAP384,
	AlgorithmECDSAP521,
	AlgorithmED25519,
	AlgorithmRSA2048,
	AlgorithmRSA3072,
	AlgorithmRSA4096,
	AlgorithmMLDSA44,
	AlgorithmMLDSA65,
	AlgorithmMLDSA87,
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
// It can also verify signatures (via a remote vall to the Vault instance).
// ML-DSA keys always sign the raw message (Pure ML-DSA) regardless of hashFunc;
// ED25519 keys do too unless hashFunc is crypto.SHA512 (ED25519ph).
//
// ED25519ph follows the transit prehashed protocol, which HashiCorp Vault >= 1.19
// currently refuses (hashicorp/vault#31574); OpenBao and Vault <= 1.18 accept it.
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

// hashingMode computes the correct hash to use for a given key-hash pair.
// ML-DSA keys always use crypto.Hash(0). Pure Ed25519 works the same except
// explicit SHA-512 requests (for Ed25519ph) are respected.
// All other key types prehash the message as requested.
func hashingMode(pub crypto.PublicKey, requested crypto.Hash) crypto.Hash {
	switch pub.(type) {
	case *mldsa.PublicKey:
		return crypto.Hash(0)
	case ed25519.PublicKey:
		if requested == crypto.SHA512 {
			return crypto.SHA512
		}
		return crypto.Hash(0)
	default:
		return requested
	}
}

// digestPolicy resolves the effective hash for pub (see hashingMode) and rejects
// WithDigest values pure-mode keys cannot use: they sign the message itself, so
// a precomputed digest paired with an explicit prehash is an error. Bytes paired
// with crypto.Hash(0) instead carry the message itself — the crypto.Signer
// convention cryptoSignerWrapper forwards, like the other KMS providers; this
// intentionally diverges from the in-process MLDSASigner, which rejects WithDigest
// outright. This one policy drives SignMessage and VerifySignature.
func digestPolicy(pub crypto.PublicKey, requested crypto.Hash, digest []byte) (crypto.Hash, error) {
	hashFunc := hashingMode(pub, requested)
	if len(digest) > 0 && requested != crypto.Hash(0) && hashFunc == crypto.Hash(0) {
		switch pub.(type) {
		case *mldsa.PublicKey:
			return 0, errors.New("WithDigest is not supported for ML-DSA")
		case ed25519.PublicKey:
			return 0, errors.New("WithDigest is not supported for pure Ed25519")
		}
	}
	return hashFunc, nil
}

// SignMessage signs the provided message using HashiCorp Vault KMS. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the HashivaultSigner was created.
//
// ML-DSA and pure ED25519 keys ignore the loaded signature.HashFunc and any
// WithCryptoSignerOpts: their APIs only support raw messages.
//
// SignMessage recognizes the following Options listed in order of preference:
//
// - WithDigest(): for pure modes (ML-DSA, pure ED25519) the bytes carry the message
// when paired with crypto.Hash(0) and are rejected otherwise
//
// All other options are ignored if specified.
func (h SignerVerifier) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	var digest []byte
	var signerOpts crypto.SignerOpts = h.hashFunc

	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	// The key type is only known remotely, so it gates how the message is digested.
	// Resolved before computing the digest: a raw-message key loaded or called with a
	// prehash must still sign the message itself, not the prehash.
	pub, err := h.client.public()
	if err != nil {
		return nil, fmt.Errorf("determining key type: %w", err)
	}
	hashFunc, err := digestPolicy(pub, signerOpts.HashFunc(), digest)
	if err != nil {
		return nil, err
	}

	if hashFunc == crypto.Hash(0) {
		// Override the hash to the identity function. A provided digest already
		// carries the message bytes; only compute when none was given.
		if len(digest) == 0 {
			digest, _, err = signature.ComputeDigestForSigning(message, crypto.Hash(0), []crypto.Hash{crypto.Hash(0)})
		}
	} else {
		digest, hashFunc, err = signature.ComputeDigestForSigning(message, hashFunc, hvSupportedHashFuncs, opts...)
	}
	if err != nil {
		return nil, err
	}

	return h.client.sign(digest, hashFunc, opts...)
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
// - WithDigest(): for pure modes (ML-DSA, pure ED25519) the bytes carry the message
// when paired with crypto.Hash(0) and are rejected otherwise
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

	// The key type is only known remotely; see SignMessage for how it gates the digest.
	pub, err := h.client.public()
	if err != nil {
		return fmt.Errorf("determining key type: %w", err)
	}
	hashFunc, err := digestPolicy(pub, signerOpts.HashFunc(), digest)
	if err != nil {
		return err
	}

	if hashFunc == crypto.Hash(0) {
		// Override hash function with identity function. A provided digest already
		// carries the message bytes; only compute when none was given.
		if len(digest) == 0 {
			digest, _, err = signature.ComputeDigestForVerifying(message, crypto.Hash(0), []crypto.Hash{crypto.Hash(0)})
		}
	} else {
		digest, hashFunc, err = signature.ComputeDigestForVerifying(message, hashFunc, hvSupportedHashFuncs, opts...)
	}
	if err != nil {
		return err
	}

	sigBytes, err := io.ReadAll(sig)
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}

	return h.client.verify(sigBytes, digest, hashFunc, opts...)
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
		// Empty contexts are enforced for compatibility (see pkg/signature/mldsa.go); honoring a
		// non-empty context would produce unverifiable signatures.
		if mldsaOpts, ok := opts.(*mldsa.Options); ok && mldsaOpts.Context != "" {
			return nil, errors.New("non-empty context is not supported; use empty context for consistency across backends")
		}
		// Vault transit cannot carry an Ed25519 signing context; dropping it would
		// silently sign without the domain separation the caller asked for.
		if edOpts, ok := opts.(*ed25519.Options); ok && edOpts.Context != "" {
			return nil, errors.New("Ed25519 signing context is not supported by Vault transit; use an empty context")
		}
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
//
// For keys that sign the raw message (Ed25519, ML-DSA) the returned options carry crypto.Hash(0), so
// callers pass the message itself rather than a digest. Ed25519ph keys carry crypto.SHA512.
//
// The key type is resolved eagerly with a remote key read, so construction fails
// when the transit server is unreachable; in exchange the returned options always
// reflect the stored key type.
func (h *SignerVerifier) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	pub, err := h.client.public()
	if err != nil {
		return nil, nil, fmt.Errorf("determining key type: %w", err)
	}
	hashFunc := hashingMode(pub, h.hashFunc)

	csw := &cryptoSignerWrapper{
		ctx:      ctx,
		sv:       h,
		hashFunc: hashFunc,
		errFunc:  errFunc,
	}

	return csw, hashFunc, nil
}

// SupportedAlgorithms returns the list of algorithms supported by the Hashicorp Vault service
func (h *SignerVerifier) SupportedAlgorithms() []string {
	return hvSupportedAlgorithms
}

// DefaultAlgorithm returns the default algorithm for the Hashicorp Vault service
func (h *SignerVerifier) DefaultAlgorithm() string {
	return AlgorithmECDSAP256
}
