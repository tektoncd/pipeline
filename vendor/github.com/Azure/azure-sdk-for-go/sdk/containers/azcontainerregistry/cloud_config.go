//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package azcontainerregistry

import "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"

const (
	// ServiceName is the cloud service name for Azure Container Registry
	ServiceName cloud.ServiceName = "azcontainerregistry"
)

func init() {
	cloud.AzureChina.Services[ServiceName] = cloud.ServiceConfiguration{
		Audience: defaultAudience,
	}
	cloud.AzureGovernment.Services[ServiceName] = cloud.ServiceConfiguration{
		Audience: defaultAudience,
	}
	cloud.AzurePublic.Services[ServiceName] = cloud.ServiceConfiguration{
		Audience: defaultAudience,
	}
}

var defaultCloud = cloud.Configuration{
	Services: map[cloud.ServiceName]cloud.ServiceConfiguration{ServiceName: {Audience: defaultAudience}},
}
