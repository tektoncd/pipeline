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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	"github.com/go-jose/go-jose/v3"
	"github.com/jellydator/ttlcache/v3"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azkeys"
	"github.com/sigstore/sigstore/pkg/signature"
	sigkms "github.com/sigstore/sigstore/pkg/signature/kms"
)

func init() {
	sigkms.AddProvider(ReferenceScheme, func(ctx context.Context, keyResourceID string, hashFunc crypto.Hash, opts ...signature.RPCOption) (sigkms.SignerVerifier, error) {
		return LoadSignerVerifier(ctx, keyResourceID, hashFunc)
	})
}

type kvClient interface {
	CreateKey(ctx context.Context, name string, parameters azkeys.CreateKeyParameters, options *azkeys.CreateKeyOptions) (azkeys.CreateKeyResponse, error)
	GetKey(ctx context.Context, name, version string, options *azkeys.GetKeyOptions) (azkeys.GetKeyResponse, error)
	Sign(ctx context.Context, name, version string, parameters azkeys.SignParameters, options *azkeys.SignOptions) (azkeys.SignResponse, error)
	Verify(ctx context.Context, name, version string, parameters azkeys.VerifyParameters, options *azkeys.VerifyOptions) (azkeys.VerifyResponse, error)
}

type azureVaultClient struct {
	client    kvClient
	keyCache  *ttlcache.Cache[string, crypto.PublicKey]
	vaultURL  string
	vaultName string
	keyName   string
}

var (
	errAzureReference = errors.New("kms specification should be in the format azurekms://[VAULT_NAME][VAULT_URL]/[KEY_NAME]")

	referenceRegex = regexp.MustCompile(`^azurekms://([^/]+)/([^/]+)?$`)
)

const (
	// ReferenceScheme schemes for various KMS services are copied from https://github.com/google/go-cloud/tree/master/secrets
	ReferenceScheme = "azurekms://"
	cacheKey        = "azure_vault_signer"
)

// ValidReference returns a non-nil error if the reference string is invalid
func ValidReference(ref string) error {
	if !referenceRegex.MatchString(ref) {
		return errAzureReference
	}
	return nil
}

func parseReference(resourceID string) (vaultURL, vaultName, keyName string, err error) {
	v := referenceRegex.FindStringSubmatch(resourceID)
	if len(v) != 3 {
		err = fmt.Errorf("invalid azurekms format %q", resourceID)
		return
	}

	vaultURL = fmt.Sprintf("https://%s/", v[1])
	vaultName, keyName = strings.Split(v[1], ".")[0], v[2]
	return
}

func newAzureKMS(keyResourceID string) (*azureVaultClient, error) {
	if err := ValidReference(keyResourceID); err != nil {
		return nil, err
	}
	vaultURL, vaultName, keyName, err := parseReference(keyResourceID)
	if err != nil {
		return nil, err
	}

	client, err := getKeysClient(vaultURL)
	if err != nil {
		return nil, fmt.Errorf("new azure kms client: %w", err)
	}

	azClient := &azureVaultClient{
		client:    client,
		vaultURL:  vaultURL,
		vaultName: vaultName,
		keyName:   keyName,
		keyCache: ttlcache.New[string, crypto.PublicKey](
			ttlcache.WithDisableTouchOnHit[string, crypto.PublicKey](),
		),
	}

	return azClient, nil
}

type authenticationMethod string

const (
	unknownAuthenticationMethod     = "unknown"
	environmentAuthenticationMethod = "environment"
	cliAuthenticationMethod         = "cli"
)

// getAuthMethod returns the an authenticationMethod to use to get an Azure Authorizer.
// If no environment variables are set, unknownAuthMethod will be used.
// If the environment variable 'AZURE_AUTH_METHOD' is set to either environment or cli, use it.
// If the environment variables 'AZURE_TENANT_ID', 'AZURE_CLIENT_ID' and 'AZURE_CLIENT_SECRET' are set, use environment.
func getAuthenticationMethod() authenticationMethod {
	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	authMethod := os.Getenv("AZURE_AUTH_METHOD")

	if authMethod != "" {
		switch strings.ToLower(authMethod) {
		case "environment":
			return environmentAuthenticationMethod
		case "cli":
			return cliAuthenticationMethod
		}
	}

	if tenantID != "" && clientID != "" && clientSecret != "" {
		return environmentAuthenticationMethod
	}

	return unknownAuthenticationMethod
}

type azureCredential interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// getAzureCredential takes an authenticationMethod and returns an Azure credential or an error.
// If the method is unknown, Environment will be tested and if it returns an error CLI will be tested.
// If the method is specified, the specified method will be used and no other will be tested.
// This means the following default order of methods will be used if nothing else is defined:
// 1. Client credentials (FromEnvironment)
// 2. Client certificate (FromEnvironment)
// 3. Username password (FromEnvironment)
// 4. MSI (FromEnvironment)
// 5. CLI (FromCLI)
func getAzureCredential(method authenticationMethod) (azureCredential, error) {
	switch method {
	case environmentAuthenticationMethod:
		cred, err := azidentity.NewEnvironmentCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default azure credential from env auth method: %w", err)
		}
		return cred, nil
	case cliAuthenticationMethod:
		cred, err := azidentity.NewAzureCLICredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default Azure credential from env auth method: %w", err)
		}
		return cred, nil
	case unknownAuthenticationMethod:
		break
	default:
		return nil, fmt.Errorf("you should never reach this")
	}

	cred, err := azidentity.NewEnvironmentCredential(nil)
	if err == nil {
		return cred, nil
	}

	cred2, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create default Azure credential from env auth method: %w", err)
	}
	return cred2, nil
}

func getKeysClient(vaultURL string) (*azkeys.Client, error) {
	authMethod := getAuthenticationMethod()
	cred, err := getAzureCredential(authMethod)
	if err != nil {
		return nil, err
	}

	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, err
	}

	return client, nil
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
	case azkeys.JSONWebKeyTypeECHSM:
		*key.Kty = azkeys.JSONWebKeyTypeEC
	case azkeys.JSONWebKeyTypeRSAHSM:
		*key.Kty = azkeys.JSONWebKeyTypeRSA
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

	pub, ok := jwk.Key.(*ecdsa.PublicKey)
	if !ok {
		if err != nil {
			return nil, fmt.Errorf("public key was not ECDSA: %#v", pub)
		}
	}

	return pub, nil
}

func (a *azureVaultClient) getKey(ctx context.Context) (azkeys.KeyBundle, error) {
	resp, err := a.client.GetKey(ctx, a.vaultURL, a.keyName, nil)
	if err != nil {
		return azkeys.KeyBundle{}, fmt.Errorf("public key: %w", err)
	}

	return resp.KeyBundle, err
}

func (a *azureVaultClient) public(ctx context.Context) (crypto.PublicKey, error) {
	var lerr error
	loader := ttlcache.LoaderFunc[string, crypto.PublicKey](
		func(c *ttlcache.Cache[string, crypto.PublicKey], key string) *ttlcache.Item[string, crypto.PublicKey] {
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
	_, err := a.getKey(ctx)
	if err == nil {
		return a.public(ctx)
	}

	_, err = a.client.CreateKey(
		ctx,
		a.keyName,
		azkeys.CreateKeyParameters{
			KeyAttributes: &azkeys.KeyAttributes{
				Enabled: to.Ptr(true),
			},
			KeySize: to.Ptr(int32(2048)),
			KeyOps: []*azkeys.JSONWebKeyOperation{
				to.Ptr(azkeys.JSONWebKeyOperationSign),
				to.Ptr(azkeys.JSONWebKeyOperationVerify),
			},
			Kty: to.Ptr(azkeys.JSONWebKeyTypeEC),
			Tags: map[string]*string{
				"use": to.Ptr("sigstore"),
			},
		}, nil)
	if err != nil {
		return nil, err
	}

	return a.public(ctx)
}

func getKeyVaultSignatureAlgo(algo crypto.Hash) (azkeys.JSONWebKeySignatureAlgorithm, error) {
	switch algo {
	case crypto.SHA256:
		return azkeys.JSONWebKeySignatureAlgorithmES256, nil
	case crypto.SHA384:
		return azkeys.JSONWebKeySignatureAlgorithmES384, nil
	case crypto.SHA512:
		return azkeys.JSONWebKeySignatureAlgorithmES512, nil
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algo)
	}
}

func (a *azureVaultClient) sign(ctx context.Context, hash []byte, algo crypto.Hash) ([]byte, error) {
	keyVaultAlgo, err := getKeyVaultSignatureAlgo(algo)
	if err != nil {
		return nil, fmt.Errorf("failed to get KeyVaultSignatureAlgorithm: %w", err)
	}

	encodedHash := make([]byte, base64.RawURLEncoding.EncodedLen(len(hash)))
	base64.StdEncoding.Encode(encodedHash, hash)

	params := azkeys.SignParameters{
		Algorithm: &keyVaultAlgo,
		Value:     encodedHash,
	}

	result, err := a.client.Sign(ctx, a.vaultURL, a.keyName, params, nil)
	if err != nil {
		return nil, fmt.Errorf("signing the payload: %w", err)
	}

	decodedRes := make([]byte, base64.RawURLEncoding.DecodedLen(len(result.Result)))

	n, err := base64.StdEncoding.Decode(decodedRes, result.Result)
	if err != nil {
		return nil, fmt.Errorf("decoding the result: %w", err)
	}

	decodedRes = decodedRes[:n]

	return decodedRes, nil
}

func (a *azureVaultClient) verify(ctx context.Context, signature, hash []byte, algo crypto.Hash) error {
	keyVaultAlgo, err := getKeyVaultSignatureAlgo(algo)
	if err != nil {
		return fmt.Errorf("failed to get KeyVaultSignatureAlgorithm: %w", err)
	}

	encodedHash := make([]byte, base64.RawURLEncoding.EncodedLen(len(hash)))
	base64.StdEncoding.Encode(encodedHash, hash)

	encodedSignature := make([]byte, base64.RawURLEncoding.EncodedLen(len(signature)))
	base64.StdEncoding.Encode(encodedSignature, signature)

	params := azkeys.VerifyParameters{
		Algorithm: &keyVaultAlgo,
		Digest:    encodedHash,
		Signature: encodedSignature,
	}

	result, err := a.client.Verify(ctx, a.vaultURL, a.keyName, params, nil)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if !*result.Value {
		return errors.New("failed vault verification")
	}

	return nil
}
