//
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

// Package azure implement the interface with microsoft azure kms service
package azure

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"

	"github.com/go-jose/go-jose/v4"
	"github.com/jellydator/ttlcache/v3"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/sigstore/sigstore/pkg/signature"
	sigkms "github.com/sigstore/sigstore/pkg/signature/kms"
)

func init() {
	sigkms.AddProvider(ReferenceScheme, func(ctx context.Context, keyResourceID string, _ crypto.Hash, _ ...signature.RPCOption) (sigkms.SignerVerifier, error) {
		return LoadSignerVerifier(ctx, keyResourceID)
	})
}

type kvClient interface {
	CreateKey(ctx context.Context, name string, parameters azkeys.CreateKeyParameters, options *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error)
	GetKey(ctx context.Context, name, version string, options *azkeys.GetKeyOptions) (azkeys.GetKeyResponse, error)
	Sign(ctx context.Context, name, version string, parameters azkeys.SignParameters, options *azkeys.SignOptions) (azkeys.SignResponse, error)
	Verify(ctx context.Context, name, version string, parameters azkeys.VerifyParameters, options *azkeys.VerifyOptions) (azkeys.VerifyResponse, error)
}

type azureVaultClient struct {
	client     kvClient
	keyCache   *ttlcache.Cache[string, crypto.PublicKey]
	vaultURL   string
	keyName    string
	keyVersion string
}

var (
	errAzureReference = errors.New("kms specification should be in the format azurekms://[VAULT_NAME][VAULT_URL]/[KEY_NAME]/[VERSION (optional)]")

	referenceRegex = regexp.MustCompile(`^azurekms://([^/]+)/([^/]+)(/[a-z0-9]*)?$`)
)

const (
	// ReferenceScheme schemes for various KMS services are copied from https://github.com/google/go-cloud/tree/master/secrets
	ReferenceScheme = "azurekms://"
	cacheKey        = "azure_vault_signer"
	azureClientID   = "AZURE_CLIENT_ID"
)

// ValidReference returns a non-nil error if the reference string is invalid
func ValidReference(ref string) error {
	if !referenceRegex.MatchString(ref) {
		return errAzureReference
	}
	return nil
}

// The key version can be optionally provided
// If provided, all key operations will specify this version.
// If not provided, the key operations will use the latest key version by default.
func parseReference(resourceID string) (vaultURL, keyName, keyVersion string, err error) {
	if isIDValid := referenceRegex.MatchString(resourceID); !isIDValid {
		err = fmt.Errorf("invalid azurekms format %q", resourceID)
		return
	}

	fullRef := strings.Split(resourceID, "azurekms://")[1]
	splitRef := strings.Split(fullRef, "/")
	vaultURL = fmt.Sprintf("https://%s/", splitRef[0])
	keyName = splitRef[1]

	if len(splitRef) == 3 {
		keyVersion = splitRef[2]
	}

	return
}

func newAzureKMS(keyResourceID string) (*azureVaultClient, error) {
	if err := ValidReference(keyResourceID); err != nil {
		return nil, err
	}
	vaultURL, keyName, keyVersion, err := parseReference(keyResourceID)
	if err != nil {
		return nil, err
	}

	opts := getAzClientOpts()
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{ClientOptions: opts})
	if err != nil {
		return nil, err
	}

	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("new azure kms client: %w", err)
	}

	azClient := &azureVaultClient{
		client:     client,
		vaultURL:   vaultURL,
		keyName:    keyName,
		keyVersion: keyVersion,
		keyCache: ttlcache.New[string, crypto.PublicKey](
			ttlcache.WithDisableTouchOnHit[string, crypto.PublicKey](),
		),
	}

	return azClient, nil
}

func getAzClientOpts() azcore.ClientOptions {
	envName := os.Getenv("AZURE_ENVIRONMENT")
	switch envName {
	case "AZUREUSGOVERNMENT", "AZUREUSGOVERNMENTCLOUD":
		return azcore.ClientOptions{Cloud: cloud.AzureGovernment}
	case "AZURECHINACLOUD":
		return azcore.ClientOptions{Cloud: cloud.AzureChina}
	case "AZURECLOUD", "AZUREPUBLICCLOUD":
		return azcore.ClientOptions{Cloud: cloud.AzurePublic}
	default:
		return azcore.ClientOptions{Cloud: cloud.AzurePublic}
	}
}

func (a *azureVaultClient) fetchPublicKey(ctx context.Context) (crypto.PublicKey, error) {
	keyBundle, err := a.getKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("public key: %w", err)
	}

	key := keyBundle.Key
	keyType := key.Kty

	// Azure Key Vault allows keys to be stored in either default Key Vault storage
	// or in managed HSMs. If the key is stored in a HSM, the key type is suffixed
	// with "-HSM". Since this suffix is specific to Azure Key Vault, it needs
	// be stripped from the key type before attempting to represent the key
	// with a go-jose/JSONWebKey struct.
	switch *keyType {
	case azkeys.KeyTypeECHSM:
		*key.Kty = azkeys.KeyTypeEC
	case azkeys.KeyTypeRSAHSM:
		*key.Kty = azkeys.KeyTypeRSA
	}

	jwkJSON, err := json.Marshal(*key)
	if err != nil {
		return nil, fmt.Errorf("encoding the jsonWebKey: %w", err)
	}

	jwk := jose.JSONWebKey{}
	err = jwk.UnmarshalJSON(jwkJSON)
	if err != nil {
		return nil, fmt.Errorf("decoding the jsonWebKey: %w", err)
	}

	return jwk.Key, nil
}

func (a *azureVaultClient) getKey(ctx context.Context) (azkeys.KeyBundle, error) {
	resp, err := a.client.GetKey(ctx, a.keyName, a.keyVersion, nil)
	if err != nil {
		return azkeys.KeyBundle{}, fmt.Errorf("public key: %w", err)
	}

	return resp.KeyBundle, err
}

func (a *azureVaultClient) public(ctx context.Context) (crypto.PublicKey, error) {
	var lerr error
	loader := ttlcache.LoaderFunc[string, crypto.PublicKey](
		func(c *ttlcache.Cache[string, crypto.PublicKey], _ string) *ttlcache.Item[string, crypto.PublicKey] {
			ttl := 300 * time.Second
			var pubKey crypto.PublicKey
			pubKey, lerr = a.fetchPublicKey(ctx)
			if lerr == nil {
				return c.Set(cacheKey, pubKey, ttl)
			}
			return nil
		},
	)
	item := a.keyCache.Get(cacheKey, ttlcache.WithLoader[string, crypto.PublicKey](loader))
	if lerr != nil {
		return nil, lerr
	}
	return item.Value(), nil
}

func (a *azureVaultClient) createKey(ctx context.Context) (crypto.PublicKey, error) {
	// check if the key already exists by attempting to fetch it
	_, err := a.getKey(ctx)
	// if the error is nil, this means the key already exists
	// and we can return the public key
	if err == nil {
		return a.public(ctx)
	}

	// If the returned error is not nil, set the error to the
	// custom azcore.ResponseError error implementation
	// this custom error allows us to check the status code
	// returned by the GetKey operation. If the operation
	// returned a 404, we know that the key does not exist
	// and we can create it.
	var respErr *azcore.ResponseError
	if ok := errors.As(err, &respErr); !ok {
		return nil, fmt.Errorf("unexpected error returned by get key operation: %w", err)
	}

	// if a non-404 status code is returned, return the error
	// since this is an unexpected error response
	if respErr.StatusCode != http.StatusNotFound {
		return nil, fmt.Errorf("unexpected status code returned by get key operation: %w", err)
	}

	// if a 404 was returned, then we can create the key
	_, err = a.client.CreateKey(
		ctx,
		a.keyName,
		azkeys.CreateKeyParameters{
			KeyAttributes: &azkeys.KeyAttributes{
				Enabled: to.Ptr(true),
			},
			KeySize: to.Ptr(int32(2048)),
			KeyOps: []*azkeys.KeyOperation{
				to.Ptr(azkeys.KeyOperationSign),
				to.Ptr(azkeys.KeyOperationVerify),
			},
			Kty: to.Ptr(azkeys.KeyTypeEC),
			Tags: map[string]*string{
				"use": to.Ptr("sigstore"),
			},
		}, nil)
	if err != nil {
		return nil, err
	}

	return a.public(ctx)
}

func (a *azureVaultClient) getKeyVaultHashFunc(ctx context.Context) (crypto.Hash, azkeys.SignatureAlgorithm, error) {
	publicKey, err := a.public(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get public key: %w", err)
	}
	switch keyImpl := publicKey.(type) {
	case *ecdsa.PublicKey:
		switch keyImpl.Curve {
		case elliptic.P256():
			return crypto.SHA256, azkeys.SignatureAlgorithmES256, nil
		case elliptic.P384():
			return crypto.SHA384, azkeys.SignatureAlgorithmES384, nil
		case elliptic.P521():
			return crypto.SHA512, azkeys.SignatureAlgorithmES512, nil
		default:
			return 0, "", fmt.Errorf("unsupported key size: %s", keyImpl.Params().Name)
		}
	case *rsa.PublicKey:
		switch keyImpl.Size() {
		case 256:
			return crypto.SHA256, azkeys.SignatureAlgorithmRS256, nil
		case 384:
			return crypto.SHA384, azkeys.SignatureAlgorithmRS384, nil
		case 512:
			return crypto.SHA512, azkeys.SignatureAlgorithmRS512, nil
		default:
			return 0, "", fmt.Errorf("unsupported key size: %d", keyImpl.Size())
		}
	default:
		return 0, "", fmt.Errorf("unsupported public key type: %T", publicKey)
	}
}

func (a *azureVaultClient) sign(ctx context.Context, hash []byte) ([]byte, error) {
	_, keyVaultAlgo, err := a.getKeyVaultHashFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get KeyVaultSignatureAlgorithm: %w", err)
	}

	params := azkeys.SignParameters{
		Algorithm: &keyVaultAlgo,
		Value:     hash,
	}

	result, err := a.client.Sign(ctx, a.keyName, a.keyVersion, params, nil)
	if err != nil {
		return nil, fmt.Errorf("signing the payload: %w", err)
	}

	return result.Result, nil
}

func (a *azureVaultClient) verify(ctx context.Context, signature, hash []byte) error {
	_, keyVaultAlgo, err := a.getKeyVaultHashFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to get KeyVaultSignatureAlgorithm: %w", err)
	}

	params := azkeys.VerifyParameters{
		Algorithm: &keyVaultAlgo,
		Digest:    hash,
		Signature: signature,
	}

	result, err := a.client.Verify(ctx, a.keyName, a.keyVersion, params, nil)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if !*result.Value {
		return errors.New("failed vault verification")
	}

	return nil
}
