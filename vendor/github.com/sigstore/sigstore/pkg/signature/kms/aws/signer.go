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

package aws

import (
	"context"
	"crypto"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

var awsSupportedAlgorithms = []types.CustomerMasterKeySpec{
	types.CustomerMasterKeySpecRsa2048,
	types.CustomerMasterKeySpecRsa3072,
	types.CustomerMasterKeySpecRsa4096,
	types.CustomerMasterKeySpecEccNistP256,
	types.CustomerMasterKeySpecEccNistP384,
	types.CustomerMasterKeySpecEccNistP521,
}

var awsSupportedHashFuncs = []crypto.Hash{
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
}

// SignerVerifier is a signature.SignerVerifier that uses the AWS Key Management Service
type SignerVerifier struct {
	client *awsClient
}

// LoadSignerVerifier generates signatures using the specified key object in AWS KMS and hash algorithm.
//
// It also can verify signatures locally using the public key. hashFunc must not be crypto.Hash(0).
func LoadSignerVerifier(ctx context.Context, referenceStr string, opts ...func(*config.LoadOptions) error) (*SignerVerifier, error) {
	a := &SignerVerifier{}

	var err error
	a.client, err = newAWSClient(ctx, referenceStr, opts...)
	if err != nil {
		return nil, err
	}

	return a, nil
}

// SignMessage signs the provided message using AWS KMS. If the message is provided,
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
	var digest []byte
	var err error
	ctx := context.Background()

	for _, opt := range opts {
		opt.ApplyContext(&ctx)
		opt.ApplyDigest(&digest)
	}

	var signerOpts crypto.SignerOpts
	signerOpts, err = a.client.getHashFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting fetching default hash function: %w", err)
	}
	for _, opt := range opts {
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	hf := signerOpts.HashFunc()

	if len(digest) == 0 {
		digest, hf, err = signature.ComputeDigestForSigning(message, hf, awsSupportedHashFuncs, opts...)
		if err != nil {
			return nil, err
		}
	}

	return a.client.sign(ctx, digest, hf)
}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. If the caller wishes to specify the context to use to obtain
// the public key, pass option.WithContext(desiredCtx).
//
// All other options are ignored if specified.
func (a *SignerVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	ctx := context.Background()
	for _, opt := range opts {
		opt.ApplyContext(&ctx)
	}

	cmk, err := a.client.getCMK(ctx)
	if err != nil {
		return nil, err
	}
	return cmk.PublicKey, nil
}

// VerifySignature verifies the signature for the given message. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the SignerVerifier was created.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithContext()
//
// - WithDigest()
//
// - WithRemoteVerification()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (a *SignerVerifier) VerifySignature(sig, message io.Reader, opts ...signature.VerifyOption) (err error) {
	ctx := context.Background()
	var digest []byte
	var remoteVerification bool

	for _, opt := range opts {
		opt.ApplyContext(&ctx)
		opt.ApplyDigest(&digest)
		opt.ApplyRemoteVerification(&remoteVerification)
	}

	if !remoteVerification {
		return a.client.verify(ctx, sig, message, opts...)
	}

	var signerOpts crypto.SignerOpts
	signerOpts, err = a.client.getHashFunc(ctx)
	if err != nil {
		return fmt.Errorf("getting hash func: %w", err)
	}
	for _, opt := range opts {
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}
	hf := signerOpts.HashFunc()

	if len(digest) == 0 {
		digest, _, err = signature.ComputeDigestForVerifying(message, hf, awsSupportedHashFuncs, opts...)
		if err != nil {
			return err
		}
	}

	sigBytes, err := io.ReadAll(sig)
	if err != nil {
		return fmt.Errorf("reading signature: %w", err)
	}
	return a.client.verifyRemotely(ctx, sigBytes, digest)
}

// CreateKey attempts to create a new key in Vault with the specified algorithm.
func (a *SignerVerifier) CreateKey(ctx context.Context, algorithm string) (crypto.PublicKey, error) {
	return a.client.createKey(ctx, algorithm)
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
	awsOptions := []signature.SignOption{
		options.WithContext(c.ctx),
		options.WithDigest(digest),
		options.WithCryptoSignerOpts(hashFunc),
	}

	return c.sv.SignMessage(nil, awsOptions...)
}

// CryptoSigner returns a crypto.Signer object that uses the underlying SignerVerifier, along with a crypto.SignerOpts object
// that allows the KMS to be used in APIs that only accept the standard golang objects
func (a *SignerVerifier) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	defaultHf, err := a.client.getHashFunc(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("getting fetching default hash function: %w", err)
	}

	csw := &cryptoSignerWrapper{
		ctx:      ctx,
		sv:       a,
		hashFunc: defaultHf,
		errFunc:  errFunc,
	}

	return csw, defaultHf, nil
}

// SupportedAlgorithms returns the list of algorithms supported by the AWS KMS service
func (*SignerVerifier) SupportedAlgorithms() []string {
	s := make([]string, len(awsSupportedAlgorithms))
	for i := range awsSupportedAlgorithms {
		s[i] = string(awsSupportedAlgorithms[i])
	}
	return s
}

// DefaultAlgorithm returns the default algorithm for the AWS KMS service
func (*SignerVerifier) DefaultAlgorithm() string {
	return string(types.CustomerMasterKeySpecEccNistP256)
}
