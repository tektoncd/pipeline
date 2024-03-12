## Go

```yaml
clear-output-folder: false
export-clients: true
go: true
input-file: https://github.com/Azure/azure-rest-api-specs/blob/7452e1cc7db72fbc6cd9539b390d8b8e5c2a1864/specification/keyvault/data-plane/Microsoft.KeyVault/stable/7.5/keys.json
license-header: MICROSOFT_MIT_NO_VERSION
module: github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys
openapi-type: "data-plane"
output-folder: ../azkeys
override-client-name: Client
security: "AADToken"
security-scopes: "https://vault.azure.net/.default"
use: "@autorest/go@4.0.0-preview.59"
inject-spans: true
version: "^3.0.0"

directive:
  # delete unused models
  - remove-model: KeyExportParameters
  - remove-model: KeyProperties

  # make vault URL a parameter of the client constructor
  - from: swagger-document
    where: $["x-ms-parameterized-host"]
    transform: $.parameters[0]["x-ms-parameter-location"] = "client"

  # rename parameter models to match their methods
  - rename-model:
      from: KeyCreateParameters
      to: CreateKeyParameters
  - rename-model:
      from: KeyExportParameters
      to: ExportKeyParameters
  - rename-model:
      from: KeyImportParameters
      to: ImportKeyParameters
  - rename-model:
      from: KeyReleaseParameters
      to: ReleaseParameters
  - rename-model:
      from: KeyRestoreParameters
      to: RestoreKeyParameters
  - rename-model:
      from: KeySignParameters
      to: SignParameters
  - rename-model:
      from: KeyUpdateParameters
      to: UpdateKeyParameters
  - rename-model:
      from: KeyVerifyParameters
      to: VerifyParameters
  - rename-model:
      from: GetRandomBytesRequest
      to: GetRandomBytesParameters

  # rename paged operations from Get* to List*
  - rename-operation:
      from: GetDeletedKeys
      to: ListDeletedKeyProperties
  - rename-operation:
      from: GetKeys
      to: ListKeyProperties
  - rename-operation:
      from: GetKeyVersions
      to: ListKeyPropertiesVersions

  # rename KeyItem and KeyBundle
  - rename-model:
      from: DeletedKeyBundle
      to: DeletedKey
  - rename-model:
      from: KeyItem
      to: KeyProperties
  - rename-model:
      from: DeletedKeyItem
      to: DeletedKeyProperties
  - rename-model:
      from: DeletedKeyListResult
      to: DeletedKeyPropertiesListResult
  - rename-model:
      from: KeyListResult
      to: KeyPropertiesListResult
  - from: swagger-document
    where: $.definitions.RestoreKeyParameters.properties.value
    transform: $["x-ms-client-name"] = "KeyBackup"

  # Change LifetimeActions to LifetimeAction
  - rename-model:
      from: LifetimeActions
      to: LifetimeAction
  - rename-model:
      from: LifetimeActionsType
      to: LifetimeActionType
  - rename-model:
      from: LifetimeActionsTrigger
      to: LifetimeActionTrigger
  
  # Rename HsmPlatform to HSMPlatform for consistency
  - where-model: KeyAttributes
    rename-property:
      from: hsmPlatform
      to: HSMPlatform

  # Remove MaxResults parameter
  - where: "$.paths..*"
    remove-parameter:
      in: query
      name: maxresults

  # KeyOps updates
  - rename-model:
      from: KeyOperationsParameters
      to: KeyOperationParameters
  - from: models.go
    where: $
    transform: return $.replace(/KeyOps \[\]\*string/, "KeyOps []*KeyOperation");

  # fix capitalization
  - from: swagger-document
    where: $.definitions.ImportKeyParameters.properties.Hsm
    transform: $["x-ms-client-name"] = "HSM"
  - from: swagger-document
    where: $.definitions..properties..iv
    transform: $["x-ms-client-name"] = "IV"
  - from: swagger-document
    where: $.definitions..properties..kid
    transform: $["x-ms-client-name"] = "KID"

  # keyName, keyVersion -> name, version
  - from: swagger-document
    where: $.paths..parameters..[?(@.name=='key-name')]
    transform: $["x-ms-client-name"] = "name"
  - from: swagger-document
    where: $.paths..parameters..[?(@.name=='key-version')]
    transform: $["x-ms-client-name"] = "version"
  
  # KeyEncryptionAlgorithm renames
  - from: swagger-document
    where: $.definitions.ReleaseParameters.properties.enc
    transform: $["x-ms-client-name"] = "algorithm"

  # rename KeyOperationsParameters fields
  - from: swagger-document
    where: $.definitions.KeyOperationParameters.properties.aad
    transform: $["x-ms-client-name"] = "AdditionalAuthenticatedData"
  - from: swagger-document
    where: $.definitions.KeyOperationParameters.properties.tag
    transform: $["x-ms-client-name"] = "AuthenticationTag"

  # remove JSONWeb Prefix
  - from: 
      - models.go
      - constants.go
    where: $
    transform: return $.replace(/JSONWebKeyOperation/g, "KeyOperation");
  - from: 
      - models.go
      - constants.go
    where: $
    transform: return $.replace(/JSONWebKeyCurveName/g, "CurveName");
  - from: 
      - models.go
      - constants.go
    where: $
    transform: return $.replace(/JSONWebKeyEncryptionAlgorithm/g, "EncryptionAlgorithm");
  - from: 
      - models.go
      - constants.go
    where: $
    transform: return $.replace(/JSONWebKeySignatureAlgorithm/g, "SignatureAlgorithm");
  - from: 
      - models.go
      - constants.go
    where: $
    transform: return $.replace(/JSONWebKeyType/g, "KeyType");

  # remove DeletionRecoveryLevel type
  - from: models.go
    where: $
    transform: return $.replace(/RecoveryLevel \*DeletionRecoveryLevel/g, "RecoveryLevel *string");
  - from: constants.go
    where: $
    transform: return $.replace(/(?:\/\/.*\s)+type DeletionRecoveryLevel string/, "");
  - from: constants.go
    where: $
    transform: return $.replace(/(?:\/\/.*\s)+func PossibleDeletionRecovery(?:.+\s)+\}/, "");
  - from: constants.go
    where: $
    transform: return $.replace(/const \(\n\s\/\/ DeletionRecoveryLevel(?:.+\s)+\)/, "");

  # delete SignatureAlgorithmRSNULL
  - from: constants.go
    where: $
    transform: return $.replace(/.*(\bSignatureAlgorithmRSNULL\b).*/g, "");

   # delete KeyOperationExport
  - from: constants.go
    where: $
    transform: return $.replace(/.*(\bKeyOperationExport\b).*/g, "");

  # delete unused error models
  - from: models.go
    where: $
    transform: return $.replace(/(?:\/\/.*\s)+type (?:Error|KeyVaultError).+\{(?:\s.+\s)+\}\s/g, "");
  - from: models_serde.go
    where: $
    transform: return $.replace(/(?:\/\/.*\s)+func \(\w \*?(?:Error|KeyVaultError)\).*\{\s(?:.+\s)+\}\s/g, "");

  # delete the Attributes model defined in common.json (it's used only with allOf)
  - from: models.go
    where: $
    transform: return $.replace(/(?:\/\/.*\s)+type Attributes.+\{(?:\s.+\s)+\}\s/, "");
  - from: models_serde.go
    where: $
    transform: return $.replace(/(?:\/\/.*\s)+func \(a \*?Attributes\).*\{\s(?:.+\s)+\}\s/g, "");

  # delete the version path param check (version == "" is legal for Key Vault but indescribable by OpenAPI)
  - from: client.go
    where: $
    transform: return $.replace(/\sif version == "" \{\s+.+version cannot be empty"\)\s+\}\s/g, "");

  # delete client name prefix from method options and response types
  - from:
      - client.go
      - models.go
      - options.go
      - response_types.go
      - options.go
    where: $
    transform: return $.replace(/Client(\w+)((?:Options|Response))/g, "$1$2");

  # insert a handwritten type for "KID" fields so we can add parsing methods
  - from: models.go
    where: $
    transform: return $.replace(/(KID \*)string(\s+.*)/g, "$1ID$2")

  # edit doc comments to not contain DeletedKeyBundle
  - from: 
      - models.go
      - response_types.go
    where: $
    transform: return $.replace(/DeletedKeyBundle/g, "DeletedKey")

  # remove redundant section of doc comment
  - from:
      - models.go
      - constants.go
    where: $
    transform: return $.replace(/(For valid values, (.*)\.)|(For more information .*\.)|(For more information on possible algorithm\n\/\/ types, see JsonWebKeySignatureAlgorithm.)/g, "");
```
