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

// Package token provides Azure credential acquisition using the modern Azure Identity SDK.
//
// It uses azidentity.NewDefaultAzureCredential which tries the following authentication
// methods in order:
//
//  1. Environment credentials (AZURE_CLIENT_ID + AZURE_CLIENT_SECRET + AZURE_TENANT_ID,
//     or AZURE_CLIENT_ID + AZURE_CLIENT_CERTIFICATE_PATH + AZURE_TENANT_ID,
//     or AZURE_CLIENT_ID + AZURE_USERNAME + AZURE_PASSWORD + AZURE_TENANT_ID)
//  2. Workload Identity (AZURE_FEDERATED_TOKEN_FILE + AZURE_CLIENT_ID + AZURE_TENANT_ID)
//  3. Managed Identity (system-assigned or user-assigned via AZURE_CLIENT_ID)
//  4. Azure CLI credentials
//  5. Azure Developer CLI credentials
package token

import (
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// GetCredential returns an Azure token credential using DefaultAzureCredential,
// which automatically discovers available authentication methods from the environment.
//
// The returned credential can be used to acquire tokens for any Azure resource.
func GetCredential() (azcore.TokenCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("acr: failed to create Azure credential: %w", err)
	}
	return cred, nil
}

// GetTenantID returns the Azure AD tenant ID from the AZURE_TENANT_ID environment variable.
// Returns an empty string if not set.
func GetTenantID() string {
	return os.Getenv("AZURE_TENANT_ID")
}
