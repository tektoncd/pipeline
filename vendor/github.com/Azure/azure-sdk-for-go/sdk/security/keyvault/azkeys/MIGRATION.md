# Guide to migrate from `keyvault` to `azkeys`

This guide is intended to assist in the migration to the `azkeys` module from the deprecated `keyvault` module. `azkeys` allows users to create and manage [keys][keys] with Azure Key Vault.

## General changes

In the past, Azure Key Vault operations were all contained in a single package. For Go, this was `github.com/Azure/azure-sdk-for-go/services/keyvault/<version>/keyvault`. 

The new SDK divides the Key Vault API into separate modules for keys, secrets, and certificates. This guide focuses on migrating keys operations to use the new `azkeys` module.

There are other changes besides the module name. For example, some type and method names are different, and all new modules authenticate using our [azidentity] module.

## Code examples

The following code example shows the difference between the old and new modules when creating a key. The biggest differences are the client and authentication. In the `keyvault` module, users created a `keyvault.BaseClient` then added an `Authorizer` to the client to authenticate. In the `azkeys` module, users create a credential using the [azidentity] module then use that credential to construct the client.

Another difference is that the Key Vault URL is now passed to the client once during construction, not every time a method is called.

### `keyvault` create key
```go
import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
)

func main() {
    vaultURL := "https://<TODO: your vault name>.vault.azure.net"
    authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		// TODO: handle error
	}

	basicClient := keyvault.New()
	basicClient.Authorizer = authorizer

	fmt.Println("\ncreating a key in keyvault:")
    keyParams := keyvault.KeyCreateParameters{
        Curve: &keyvault.P256,
        Kty:   &keyvault.EC,
    }
	newBundle, err := basicClient.CreateKey(context.TODO(), vaultURL, "<key name>", keyParams)
	if err != nil {
		// TODO: handle error
	}
	fmt.Println("added/updated: " + *newBundle.JSONWebKey.Kid)
}
```

### `azkeys` create key
```go
package main

import (
    "context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

func main() {
	vaultURL := "https://<TODO: your vault name>.vault.azure.net"
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		// TODO: handle error
	}

	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		// TODO: handle error
	}

	keyParams := azkeys.CreateKeyParameters{
		Curve: to.Ptr(azkeys.CurveNameP256K),
		Kty:   to.Ptr(azkeys.KeyTypeEC),
	}
	resp, err := client.CreateKey(context.TODO(), "<key name>", keyParams, nil)
	if err != nil {
		// TODO: handle error
	}
	fmt.Println(*resp.Key.KID)
}
```

[azidentity]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity
[keys]: https://learn.microsoft.com/azure/key-vault/keys/about-keys
