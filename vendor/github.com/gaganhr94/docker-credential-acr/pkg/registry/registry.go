// Copyright 2026 docker-credential-acr authors
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

package registry

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
)

var acrRE = regexp.MustCompile(`.*\.azurecr\.io|.*\.azurecr\.cn|.*\.azurecr\.de|.*\.azurecr\.us`)

const mcrHostname = "mcr.microsoft.com"

// IsACRRegistry returns true if the given server URL refers to an Azure Container Registry
// or Microsoft Container Registry endpoint.
//
// It recognises the following domains:
//   - *.azurecr.io  (Azure public cloud)
//   - *.azurecr.cn  (Azure China)
//   - *.azurecr.de  (Azure Germany)
//   - *.azurecr.us  (Azure US Government)
//   - mcr.microsoft.com (Microsoft Container Registry)
func IsACRRegistry(serverURL string) bool {
	sURL, err := url.Parse(fmt.Sprintf("https://%s", serverURL))
	if err != nil {
		return false
	}
	host := sURL.Hostname()
	if host == "" {
		return false
	}
	return acrRE.MatchString(host) || host == mcrHostname
}

// ExchangeACRAccessToken exchanges an Azure AD access token for an ACR refresh token.
// The serverURL should be the registry hostname (e.g. "myregistry.azurecr.io").
// The tenantID is the Azure AD tenant ID, and accessToken is a valid AAD bearer token.
func ExchangeACRAccessToken(ctx context.Context, serverURL, tenantID, accessToken string) (string, error) {
	endpoint := fmt.Sprintf("https://%s", serverURL)
	client, err := azcontainerregistry.NewAuthenticationClient(endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("acr: failed to create authentication client: %w", err)
	}

	resp, err := client.ExchangeAADAccessTokenForACRRefreshToken(ctx,
		azcontainerregistry.PostContentSchemaGrantTypeAccessToken,
		serverURL,
		&azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions{
			AccessToken: &accessToken,
			Tenant:      &tenantID,
		},
	)
	if err != nil {
		return "", fmt.Errorf("acr: failed to exchange AAD token for ACR refresh token: %w", err)
	}

	if resp.RefreshToken == nil {
		return "", fmt.Errorf("acr: received nil refresh token from ACR")
	}

	return *resp.RefreshToken, nil
}

// GetRegistryRefreshToken obtains an ACR refresh token for the given server URL
// using the provided Azure credential. It first acquires an AAD access token scoped
// to Azure Resource Manager, then exchanges it for an ACR refresh token.
func GetRegistryRefreshToken(ctx context.Context, serverURL, tenantID string, cred azcore.TokenCredential) (string, error) {
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return "", fmt.Errorf("acr: failed to get AAD token: %w", err)
	}

	return ExchangeACRAccessToken(ctx, serverURL, tenantID, token.Token)
}
