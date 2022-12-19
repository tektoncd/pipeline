# Azure KMS

In order to use Azure KMS ([Key Vault](https://docs.microsoft.com/en-us/azure/key-vault/general/basic-concepts)) with the sigstore project you need to have a few things setup in Azure first.
The key creation will be handled in sigstore, however the Azure Key Vault and the required permission will have to be configured before.

## Azure Prerequisites

- [Resource Group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal#what-is-a-resource-group)
- [Key Vault](https://docs.microsoft.com/en-us/azure/key-vault/general/basic-concepts)
- [Key Vault permissions](https://docs.microsoft.com/en-us/azure/key-vault/general/rbac-guide)
- [Container Registry](https://docs.microsoft.com/en-us/azure/container-registry/container-registry-intro) _(not required, but used in below examples)_

## Permissions (Access Policies)

Different commands require different Key Vault access policies. For more information check the official [Azure Docs](https://azure.microsoft.com/en-us/services/key-vault/).

**cosign generate-key-pair**

Required access policies (keys): `get`, `create`

```shell
cosign generate-key-pair --kms azurekms://[Key Vault Name].vault.azure.net/[Key Name]
```

**cosign sign**

Required access policies (keys): `get`, `sign`

```shell
az acr login --name [Container Registry Name]
cosign sign --key azurekms://[Key Vault Name].vault.azure.net/[Key Name] [Container Registry Name].azurecr.io/[Image Name]
```

**cosign verify**

Required access policy (keys): `verify`

```shell
az acr login --name [Container Registry Name]
cosign verify --key azurekms://[Key Vault Name].vault.azure.net/[Key Name] [Container Registry Name].azurecr.io/[Image Name]
```

## Authentication

There are multiple authentication methods supported for Azure Key Vault and by default they will be evaluated in the following order:

1. Client credentials (FromEnvironment)
1. Client certificate (FromEnvironment)
1. Username password (FromEnvironment)
1. MSI (FromEnvironment)
1. CLI (FromCLI)

You can force either `FromEnvironment` or `FromCLI` by configuring the environment variable `AZURE_AUTH_METHOD` to either `environment` or `cli`.

For backward compatibility, if you configure `AZURE_TENANT_ID`, `AZURE_CLIENT_ID` and `AZURE_CLIENT_SECRET`, `FromEnvironment` will be used.
