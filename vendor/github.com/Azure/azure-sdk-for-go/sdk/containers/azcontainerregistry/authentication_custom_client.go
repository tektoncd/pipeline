//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package azcontainerregistry

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// AuthenticationClientOptions contains the optional parameters for the NewAuthenticationClient method.
type AuthenticationClientOptions struct {
	azcore.ClientOptions
}

// NewAuthenticationClient creates a new instance of AuthenticationClient with the specified values.
//   - endpoint - Registry login URL
//   - options - Client options, pass nil to accept the default values.
func NewAuthenticationClient(endpoint string, options *AuthenticationClientOptions) (*AuthenticationClient, error) {
	if options == nil {
		options = &AuthenticationClientOptions{}
	}

	azcoreClient, err := azcore.NewClient(moduleName, moduleVersion, runtime.PipelineOptions{}, &options.ClientOptions)
	if err != nil {
		return nil, err
	}

	client := &AuthenticationClient{
		internal: azcoreClient,
		endpoint: endpoint,
	}
	return client, nil
}
