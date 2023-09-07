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
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	"github.com/go-jose/go-jose/v3"
	"github.com/jellydator/ttlcache/v3"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/sigstore/sigstore/pkg/signature"
	sigkms "github.com/sigstore/sigstore/pkg/signature/kms"
)

func init() {
	sigkms.AddProvider(ReferenceScheme, func(ctx context.Context, keyResourceID string, _ crypto.Hash, opts ...signature.RPCOption) (sigkms.SignerVerifier, error) {
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

	client, err := getKeysClient(vaultURL)
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
	clientOpts := getAzClientOpts()

	switch method {
	case environmentAuthenticationMethod:
		envCred, err := azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{ClientOptions: clientOpts})
		if err == nil {
			return envCred, nil
		}

		o := &azidentity.ManagedIdentityCredentialOptions{ClientOptions: clientOpts}
		if ID, ok := os.LookupEnv(azureClientID); ok {
			o.ID = azidentity.ClientID(ID)
		}
		msiCred, err := azidentity.NewManagedIdentityCredential(o)
		if err == nil {
			return msiCred, nil
		}

		return nil, fmt.Errorf("failed to create default azure credential from env auth method: %w", err)
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

	envCreds, err := azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{ClientOptions: clientOpts})
	if err == nil {
		return envCreds, nil
	}

	cliCreds, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create default Azure credential from env auth method: %w", err)
	}
	return cliCreds, nil
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
