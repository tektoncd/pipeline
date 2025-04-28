# Release History

## 1.3.1 (2025-02-13)

### Other Changes
* Upgraded dependencies

## 1.3.0 (2024-11-06)

### Features Added
* Added API Version support. Users can now change the default API Version by setting ClientOptions.APIVersioncd 

## 1.2.0 (2024-10-21)

### Features Added
* Add CAE support
* Client requests tokens from the Vault's tenant, overriding any credential default
  (thanks @francescomari)

## 1.1.0 (2024-02-13)

### Other Changes
* Upgraded to API service version `7.5`
* Upgraded dependencies

## 1.1.0-beta.2 (2023-11-08)

### Features Added
* Added the `HSMPlatform` field to the `KeyAttributes` struct

### Other Changes
* Upgraded service version to `7.5-preview.1`
* Updated to latest version of `azcore`.
* Fixed value of `otel.library.name` in traces.

## 1.1.0-beta.1 (2023-10-11)

### Features Added

* Enabled spans for distributed tracing.

## 1.0.1 (2023-08-23)

### Other Changes
* Upgraded dependencies

## 1.0.0 (2023-07-17)

### Features Added
* first stable release of `azkeys` module

## 0.12.0 (2023-06-08)

### Breaking Changes

* Renamed `GetRandomBytesRequest` to `GetRandomBytesParameters`
* `ListDeletedKey` to `ListDeletedKeyProperties`
* `ListKeys` to `ListKeyProperties`
* `DeletedKeyBundle` to `DeletedKey`
* `KeyBundle` to `KeyVaultKey`
* `RestoreKeyParameters.KeyBundleBackup` to `RestoreKeyParameters.KeyBackup`
* `DeletedKeyItem` to `DeletedKeyProperties`
* `KeyItem` to `KeyProperties`
* `DeletedKeyListResult` to `DeletedKeyPropertiesListResult`
* `KeyListResult` `KeyPropertiesListResult`
* `KeyOperationsParameters` to `KeyOperationParameters`
* Changed `JSONWebKey.KeyOperations` from type []*string to []*KeyOperation
* `ReleaseParameters.Enc` to `ReleaseParameters.Algorithm`
* `KeyOperationParameters.AAD` to `KeyOperationParameters.AdditionalAuthenticatedData`
* `KeyOperationParameters.Tag` to `KeyOperationParameters.AuthenticationTag`
* `JSONWebKeyOperation` to `KeyOperation`
* `JSONWebKeyCurveName` to `KeyCurveName`
* `JSONWebKeyEncryptionAlgorithm` to `EncryptionAlgorithm`
* `JSONWebKeySignatureAlgorithm` to `SignatureAlgorithm`
* `JSONWebKeyType` to `KeyType`
* `LifetimeActions` to `LifetimeAction`
* Removed `DeletionRecoveryLevel` type
* Removed `SignatureAlgorithmRSNULL` constant
* Removed `KeyOperationExport` constant
* Removed `MaxResults` option

### Other Changes
* Updated dependencies

## 0.11.0 (2023-04-13)

### Breaking Changes
* Moved from `sdk/keyvault/azkeys` to `sdk/security/keyvault/azkeys`

## 0.10.0 (2023-04-13)

### Features Added
* Upgraded to api version 7.4

### Breaking Changes
* Renamed `ActionType` to `KeyRotationPolicyAction`

## 0.9.0 (2022-11-08)

### Breaking Changes
* `NewClient` returns an `error`

## 0.8.1 (2022-09-20)

### Features Added
* Added `ClientOptions.DisableChallengeResourceVerification`.
  See https://aka.ms/azsdk/blog/vault-uri for more information.

## 0.8.0 (2022-09-12)

### Breaking Changes
* Verify the challenge resource matches the vault domain.

## 0.7.0 (2022-08-09)

### Breaking Changes
* Changed type of `NewClient` options parameter to `azkeys.ClientOptions`, which embeds
  the former type, `azcore.ClientOptions`

## 0.6.0 (2022-07-07)

### Breaking Changes
* The `Client` API now corresponds more directly to the Key Vault REST API.
  Most method signatures and types have changed. See the
  [module documentation](https://aka.ms/azsdk/go/keyvault-keys/docs)
  for updated code examples and more details.

### Other Changes
* Upgrade to latest `azcore`

## 0.5.1 (2022-05-12)

### Other Changes
* Update to latest `azcore` and `internal` modules.

## 0.5.0 (2022-04-06)

### Features Added
* Added the Name property on `Key`

### Breaking Changes
* Requires go 1.18
* `ListPropertiesOfDeletedKeysPager` has `More() bool` and `NextPage(context.Context) (ListPropertiesOfDeletedKeysPage, error)` for paging over deleted keys.
* `ListPropertiesOfKeyVersionsPager` has `More() bool` and `NextPage(context.Context) (ListPropertiesOfKeyVersionsPage, error)` for paging over deleted keys.
* Removing `RawResponse *http.Response` from `crypto` response types

## 0.4.0 (2022-03-08)

### Features Added
* Adds the `ReleasePolicy` parameter to the `UpdateKeyPropertiesOptions` struct.
* Adds the `Immutable` boolean to the `KeyReleasePolicy` model.
* Added a `ToPtr` method on `KeyType` constant

### Breaking Changes
* Requires go 1.18
* Changed the `Data` to `EncodedPolicy` on the `KeyReleasePolicy` struct.
* Changed the `Updated`, `Created`, and `Expires` properties to `UpdatedOn`, `CreatedOn`, and `ExpiresOn`.
* Renamed `JSONWebKeyOperation` to `Operation`.
* Renamed `JSONWebKeyCurveName` to `CurveName`
* Prefixed all KeyType constants with `KeyType`
* Changed `KeyBundle` to `KeyVaultKey` and `DeletedKeyBundle` to `DeletedKey`
* Renamed `KeyAttributes` to `KeyProperties`
* Renamed `ListKeyVersions` to `ListPropertiesOfKeyVersions`
* Removed `Attributes` struct
* Changed `CreateOCTKey`/`Response`/`Options` to `CreateOctKey`/`Response`/`Options`
* Removed all `RawResponse *http.Response` fields from response structs.

## 0.3.0 (2022-02-08)

### Breaking Changes
* Changed the `Tags` properties from `map[string]*string` to `map[string]string`

### Bugs Fixed
* Fixed a bug in `UpdateKeyProperties` where the `KeyOps` would be deleted if the `UpdateKeyProperties.KeyOps` value was left empty.

## 0.2.0 (2022-01-12)

### Bugs Fixed
* Fixes a bug in `crypto.NewClient` where the key version was required in the path, it is no longer required but is recommended.

### Other Changes
* Updates `azcore` dependency from `v0.20.0` to `v0.21.0`

## 0.1.0 (2021-11-09)
* This is the initial release of the `azkeys` library
