# Azure Container Registry client module for Go

Azure Container Registry allows you to store and manage container images and artifacts in a private registry for all types of container deployments.

Use the client library for Azure Container Registry to:

- List images or artifacts in a registry
- Obtain metadata for images and artifacts, repositories and tags
- Set read/write/delete properties on registry items
- Delete images and artifacts, repositories and tags
- Upload and download images

[Source code](https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/containers/azcontainerregistry) | [Package (pkg.go.dev)](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry) | [REST API documentation](https://learn.microsoft.com/rest/api/containerregistry/) | [Product documentation](https://learn.microsoft.com/azure/container-registry/)

## Getting started

### Install packages

Install `azcontainerregistry` and `azidentity` with `go get`:
```Bash
go get github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity
```
[azidentity](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity) is used for Azure Active Directory authentication as demonstrated below.

### Prerequisites

- An [Azure subscription](https://azure.microsoft.com/free/)
- A supported Go version (the Azure SDK supports the two most recent Go releases)
- A [Container Registry service instance](https://learn.microsoft.com/azure/container-registry/container-registry-intro)

To create a new Container Registry, you can use the [Azure Portal](https://learn.microsoft.com/azure/container-registry/container-registry-get-started-portal),
[Azure PowerShell](https://learn.microsoft.com/azure/container-registry/container-registry-get-started-powershell), or the [Azure CLI](https://learn.microsoft.com/azure/container-registry/container-registry-get-started-azure-cli).
Here's an example using the Azure CLI:

```Powershell
az acr create --name MyContainerRegistry --resource-group MyResourceGroup --location westus --sku Basic
```
### Authentication

This document demonstrates using [azidentity.NewDefaultAzureCredential](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity#NewDefaultAzureCredential) to authenticate.
This credential type works in both local development and production environments.
We recommend using a [managed identity](https://learn.microsoft.com/azure/active-directory/managed-identities-azure-resources/overview) in production.

[Client](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry#Client) and [BlobClient](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry#BlobClient) accepts any [azidentity][https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity] credential.
See the [azidentity](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity) documentation for more information about other credential types.

#### Create a client

Constructing the client requires your Container Registry's endpoint URL, which you can get from the Azure CLI (`loginServer` value returned by `az acr list`) or the Azure Portal (`Login server` value on registry overview page).

```go
import (
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"log"
)

func main() {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("failed to obtain a credential: %v", err)
	}

	client, err := azcontainerregistry.NewClient("<your Container Registry's endpoint URL>", cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
}
```

## Key concepts

A **registry** stores Docker images and [OCI Artifacts](https://opencontainers.org/).
An image or artifact consists of a **manifest** and **layers**.
An image's manifest describes the layers that make up the image, and is uniquely identified by its **digest**.
An image can also be "tagged" to give it a human-readable alias.
An image or artifact can have zero or more **tags** associated with it, and each tag uniquely identifies the image.
A collection of images that share the same name but have different tags, is referred to as a **repository**.

For more information please see [Container Registry Concepts](https://learn.microsoft.com/azure/container-registry/container-registry-concepts).

## Examples

Get started with our [examples](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry#pkg-examples).

## Troubleshooting

For information about troubleshooting, refer to the [troubleshooting guide](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/containers/azcontainerregistry/TROUBLESHOOTING.md).

## Contributing

This project welcomes contributions and suggestions. Most contributions require you to agree to a Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct][https://opensource.microsoft.com/codeofconduct/]. For more information, see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact opencode@microsoft.com with any additional questions or comments.

