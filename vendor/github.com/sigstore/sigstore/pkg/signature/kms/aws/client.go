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

// Package aws implement the interface with amazon aws kms service
package aws

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sigstore/sigstore/pkg/signature"
	sigkms "github.com/sigstore/sigstore/pkg/signature/kms"
)

func init() {
	sigkms.AddProvider(ReferenceScheme, func(ctx context.Context, keyResourceID string, _ crypto.Hash, _ ...signature.RPCOption) (sigkms.SignerVerifier, error) {
		return LoadSignerVerifier(ctx, keyResourceID)
	})
}

const (
	cacheKey = "signer"
	// ReferenceScheme schemes for various KMS services are copied from https://github.com/google/go-cloud/tree/master/secrets
	ReferenceScheme = "awskms://"
)

type awsClient struct {
	client   *kms.Client
	endpoint string
	keyID    string
	alias    string
	keyCache *ttlcache.Cache[string, cmk]
}

var (
	errKMSReference = errors.New("kms specification should be in the format awskms://[ENDPOINT]/[ID/ALIAS/ARN] (endpoint optional)")

	// Key ID/ALIAS/ARN conforms to KMS standard documented here: https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html#key-id
	// Key format examples:
	// Key ID: awskms:///1234abcd-12ab-34cd-56ef-1234567890ab
	// Key ID with endpoint: awskms://localhost:4566/1234abcd-12ab-34cd-56ef-1234567890ab
	// Key ARN: awskms:///arn:aws:kms:us-east-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab
	// Key ARN with endpoint: awskms://localhost:4566/arn:aws:kms:us-east-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab
	// Alias name: awskms:///alias/ExampleAlias
	// Alias name with endpoint: awskms://localhost:4566/alias/ExampleAlias
	// Alias ARN: awskms:///arn:aws:kms:us-east-2:111122223333:alias/ExampleAlias
	// Alias ARN with endpoint: awskms://localhost:4566/arn:aws:kms:us-east-2:111122223333:alias/ExampleAlias
	uuidRE      = `m?r?k?-?[A-Fa-f0-9]{8}-?[A-Fa-f0-9]{4}-?[A-Fa-f0-9]{4}-?[A-Fa-f0-9]{4}-?[A-Fa-f0-9]{12}`
	arnRE       = `arn:(?:aws|aws-us-gov):kms:[a-z0-9-]+:\d{12}:`
	hostRE      = `([^/]*)/`
	keyIDRE     = regexp.MustCompile(`^awskms://` + hostRE + `(` + uuidRE + `)$`)
	keyARNRE    = regexp.MustCompile(`^awskms://` + hostRE + `(` + arnRE + `key/` + uuidRE + `)$`)
	aliasNameRE = regexp.MustCompile(`^awskms://` + hostRE + `((alias/.*))$`)
	aliasARNRE  = regexp.MustCompile(`^awskms://` + hostRE + `(` + arnRE + `(alias/.*))$`)
	allREs      = []*regexp.Regexp{keyIDRE, keyARNRE, aliasNameRE, aliasARNRE}
)

// ValidReference returns a non-nil error if the reference string is invalid
func ValidReference(ref string) error {
	for _, re := range allREs {
		if re.MatchString(ref) {
			return nil
		}
	}
	return errKMSReference
}

// ParseReference parses an awskms-scheme URI into its constituent parts.
func ParseReference(resourceID string) (endpoint, keyID, alias string, err error) {
	var v []string
	for _, re := range allREs {
		v = re.FindStringSubmatch(resourceID)
		if len(v) >= 3 {
			endpoint, keyID = v[1], v[2]
			if len(v) == 4 {
				alias = v[3]
			}
			return
		}
	}
	err = fmt.Errorf("invalid awskms format %q", resourceID)
	return
}

func newAWSClient(ctx context.Context, keyResourceID string, opts ...func(*config.LoadOptions) error) (*awsClient, error) {
	if err := ValidReference(keyResourceID); err != nil {
		return nil, err
	}
	a := &awsClient{}
	var err error
	a.endpoint, a.keyID, a.alias, err = ParseReference(keyResourceID)
	if err != nil {
		return nil, err
	}

	if err := a.setupClient(ctx, opts...); err != nil {
		return nil, err
	}

	a.keyCache = ttlcache.New[string, cmk](
		ttlcache.WithDisableTouchOnHit[string, cmk](),
	)

	return a, nil
}

func (a *awsClient) setupClient(ctx context.Context, opts ...func(*config.LoadOptions) error) (err error) {
	if a.endpoint != "" {
		opts = append(opts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL: "https://" + a.endpoint,
				}, nil
			}),
		))
	}
	if os.Getenv("AWS_TLS_INSECURE_SKIP_VERIFY") == "1" {
		opts = append(opts, config.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint: gosec
			},
		}))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	a.client = kms.NewFromConfig(cfg)
	return
}

type cmk struct {
	KeyMetadata *types.KeyMetadata
	PublicKey   crypto.PublicKey
}

func (c *cmk) HashFunc() crypto.Hash {
	switch c.KeyMetadata.SigningAlgorithms[0] {
	case types.SigningAlgorithmSpecRsassaPssSha256, types.SigningAlgorithmSpecRsassaPkcs1V15Sha256, types.SigningAlgorithmSpecEcdsaSha256:
		return crypto.SHA256
	case types.SigningAlgorithmSpecRsassaPssSha384, types.SigningAlgorithmSpecRsassaPkcs1V15Sha384, types.SigningAlgorithmSpecEcdsaSha384:
		return crypto.SHA384
	case types.SigningAlgorithmSpecRsassaPssSha512, types.SigningAlgorithmSpecRsassaPkcs1V15Sha512, types.SigningAlgorithmSpecEcdsaSha512:
		return crypto.SHA512
	default:
		return 0
	}
}

func (c *cmk) Verifier() (signature.Verifier, error) {
	switch c.KeyMetadata.SigningAlgorithms[0] {
	case types.SigningAlgorithmSpecRsassaPssSha256, types.SigningAlgorithmSpecRsassaPssSha384, types.SigningAlgorithmSpecRsassaPssSha512:
		pub, ok := c.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not rsa")
		}
		return signature.LoadRSAPSSVerifier(pub, c.HashFunc(), nil)
	case types.SigningAlgorithmSpecRsassaPkcs1V15Sha256, types.SigningAlgorithmSpecRsassaPkcs1V15Sha384, types.SigningAlgorithmSpecRsassaPkcs1V15Sha512:
		pub, ok := c.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not rsa")
		}
		return signature.LoadRSAPKCS1v15Verifier(pub, c.HashFunc())
	case types.SigningAlgorithmSpecEcdsaSha256, types.SigningAlgorithmSpecEcdsaSha384, types.SigningAlgorithmSpecEcdsaSha512:
		pub, ok := c.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not ecdsa")
		}
		return signature.LoadECDSAVerifier(pub, c.HashFunc())
	default:
		return nil, fmt.Errorf("signing algorithm unsupported")
	}
}

func (a *awsClient) fetchCMK(ctx context.Context) (*cmk, error) {
	var err error
	cmk := &cmk{}
	cmk.PublicKey, err = a.fetchPublicKey(ctx)
	if err != nil {
		return nil, err
	}
	cmk.KeyMetadata, err = a.fetchKeyMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return cmk, nil
}

func (a *awsClient) getHashFunc(ctx context.Context) (crypto.Hash, error) {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return 0, err
	}
	return cmk.HashFunc(), nil
}

func (a *awsClient) getCMK(ctx context.Context) (*cmk, error) {
	var lerr error
	loader := ttlcache.LoaderFunc[string, cmk](
		func(c *ttlcache.Cache[string, cmk], key string) *ttlcache.Item[string, cmk] {
			var k *cmk
			k, lerr = a.fetchCMK(ctx)
			if lerr == nil {
				return c.Set(cacheKey, *k, time.Second*300)
			}
			return nil
		},
	)

	item := a.keyCache.Get(cacheKey, ttlcache.WithLoader[string, cmk](loader))
	if lerr == nil {
		cmk := item.Value()
		return &cmk, nil
	}
	return nil, lerr
}

func (a *awsClient) createKey(ctx context.Context, algorithm string) (crypto.PublicKey, error) {
	if a.alias == "" {
		return nil, errors.New("must use alias key format")
	}

	// look for existing key first
	cmk, err := a.getCMK(ctx)
	if err == nil {
		out := cmk.PublicKey
		return out, nil
	}

	// return error if not *kms.NotFoundException
	var errNotFound *types.NotFoundException
	if !errors.As(err, &errNotFound) {
		return nil, fmt.Errorf("looking up key: %w", err)
	}

	usage := types.KeyUsageTypeSignVerify
	description := "Created by Sigstore"
	key, err := a.client.CreateKey(ctx, &kms.CreateKeyInput{
		CustomerMasterKeySpec: types.CustomerMasterKeySpec(algorithm),
		KeyUsage:              usage,
		Description:           &description,
	})
	if err != nil {
		return nil, fmt.Errorf("creating key: %w", err)
	}

	_, err = a.client.CreateAlias(ctx, &kms.CreateAliasInput{
		AliasName:   &a.alias,
		TargetKeyId: key.KeyMetadata.KeyId,
	})
	if err != nil {
		return nil, fmt.Errorf("creating alias %q: %w", a.alias, err)
	}

	cmk, err = a.getCMK(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving PublicKey from cache: %w", err)
	}

	return cmk.PublicKey, err
}

func (a *awsClient) verify(ctx context.Context, sig, message io.Reader, opts ...signature.VerifyOption) error {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return err
	}
	verifier, err := cmk.Verifier()
	if err != nil {
		return err
	}
	return verifier.VerifySignature(sig, message, opts...)
}

func (a *awsClient) verifyRemotely(ctx context.Context, sig, digest []byte) error {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return err
	}
	alg := cmk.KeyMetadata.SigningAlgorithms[0]
	messageType := types.MessageTypeDigest
	if _, err := a.client.Verify(ctx, &kms.VerifyInput{
		KeyId:            &a.keyID,
		Message:          digest,
		MessageType:      messageType,
		Signature:        sig,
		SigningAlgorithm: alg,
	}); err != nil {
		return fmt.Errorf("unable to verify signature: %w", err)
	}
	return nil
}

func (a *awsClient) sign(ctx context.Context, digest []byte, _ crypto.Hash) ([]byte, error) {
	cmk, err := a.getCMK(ctx)
	if err != nil {
		return nil, err
	}
	alg := cmk.KeyMetadata.SigningAlgorithms[0]

	messageType := types.MessageTypeDigest
	out, err := a.client.Sign(ctx, &kms.SignInput{
		KeyId:            &a.keyID,
		Message:          digest,
		MessageType:      messageType,
		SigningAlgorithm: alg,
	})
	if err != nil {
		return nil, fmt.Errorf("signing with kms: %w", err)
	}
	return out.Signature, nil
}

func (a *awsClient) fetchPublicKey(ctx context.Context) (crypto.PublicKey, error) {
	out, err := a.client.GetPublicKey(ctx, &kms.GetPublicKeyInput{
		KeyId: &a.keyID,
	})
	if err != nil {
		return nil, fmt.Errorf("getting public key: %w", err)
	}
	key, err := x509.ParsePKIXPublicKey(out.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}
	return key, nil
}

func (a *awsClient) fetchKeyMetadata(ctx context.Context) (*types.KeyMetadata, error) {
	out, err := a.client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: &a.keyID,
	})
	if err != nil {
		return nil, fmt.Errorf("getting key metadata: %w", err)
	}
	return out.KeyMetadata, nil
}
