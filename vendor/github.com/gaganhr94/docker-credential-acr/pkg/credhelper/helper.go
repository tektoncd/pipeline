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

// Package credhelper provides a Docker credential helper and go-containerregistry
// keychain for Azure Container Registry.
package credhelper

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/gaganhr94/docker-credential-acr/pkg/registry"
	"github.com/gaganhr94/docker-credential-acr/pkg/token"
)

// ACRCredHelper implements the Docker credential helper interface for Azure Container Registry.
// It uses DefaultAzureCredential to automatically discover available Azure authentication.
type ACRCredHelper struct{}

// NewACRCredentialsHelper returns a new Docker credential helper for ACR.
func NewACRCredentialsHelper() credentials.Helper {
	return &ACRCredHelper{}
}

// Get retrieves ACR credentials for the given server URL.
// Returns an error if the server URL is not an ACR endpoint.
func (a *ACRCredHelper) Get(serverURL string) (string, string, error) {
	if !registry.IsACRRegistry(serverURL) {
		return "", "", fmt.Errorf("serverURL %q does not refer to Azure Container Registry", serverURL)
	}

	ctx := context.Background()

	cred, err := token.GetCredential()
	if err != nil {
		return "", "", err
	}

	tenantID := token.GetTenantID()

	refreshToken, err := registry.GetRegistryRefreshToken(ctx, serverURL, tenantID, cred)
	if err != nil {
		return "", "", err
	}

	return "<token>", refreshToken, nil
}

// Add is not supported. ACR credentials are sourced from the environment.
func (a *ACRCredHelper) Add(_ *credentials.Credentials) error {
	return errors.New("add is not supported")
}

// Delete is not supported. ACR credentials are sourced from the environment.
func (a *ACRCredHelper) Delete(_ string) error {
	return errors.New("delete is not supported")
}

// List is not supported.
func (a *ACRCredHelper) List() (map[string]string, error) {
	return nil, errors.New("list is not supported")
}

// --- go-containerregistry integration ---

// acrKeychainHelper implements authn.Helper for use with go-containerregistry's
// authn.NewKeychainFromHelper. Unlike the Docker credential helper, it returns
// ("", "", nil) for non-ACR registries so it can be composed in a MultiKeychain.
type acrKeychainHelper struct{}

// NewKeychainHelper returns an authn.Helper suitable for use with
// authn.NewKeychainFromHelper. It silently returns empty credentials
// for non-ACR registries (allowing fallthrough to other keychains).
func NewKeychainHelper() authn.Helper {
	return &acrKeychainHelper{}
}

// Get returns ACR credentials for the given server URL, or ("", "", nil)
// if the server is not an ACR endpoint (enabling fallthrough in a MultiKeychain).
func (h *acrKeychainHelper) Get(serverURL string) (string, string, error) {
	if !registry.IsACRRegistry(serverURL) {
		return "", "", nil
	}

	ctx := context.Background()

	cred, err := token.GetCredential()
	if err != nil {
		return "", "", err
	}

	tenantID := token.GetTenantID()

	refreshToken, err := registry.GetRegistryRefreshToken(ctx, serverURL, tenantID, cred)
	if err != nil {
		return "", "", err
	}

	return "<token>", refreshToken, nil
}

// Keychain returns a ready-to-use authn.Keychain for ACR. It silently
// skips non-ACR registries and can be composed in a MultiKeychain:
//
//	kc := authn.NewMultiKeychain(
//	    authn.DefaultKeychain,
//	    credhelper.Keychain(),
//	)
func Keychain() authn.Keychain {
	return authn.NewKeychainFromHelper(NewKeychainHelper())
}
