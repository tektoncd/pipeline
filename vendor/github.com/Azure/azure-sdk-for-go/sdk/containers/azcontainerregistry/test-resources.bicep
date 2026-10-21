// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

param baseName string
param location string = resourceGroup().location

resource registry 'Microsoft.ContainerRegistry/registries@2022-02-01-preview' = {
  name: baseName
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    publicNetworkAccess: 'Enabled'
    zoneRedundancy: 'Disabled'
    anonymousPullEnabled: true
  }
}

output LOGIN_SERVER string = registry.properties.loginServer
output REGISTRY_NAME string = registry.name
