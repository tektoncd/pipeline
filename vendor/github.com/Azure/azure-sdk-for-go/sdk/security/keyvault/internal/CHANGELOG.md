# Release History

## 1.1.1 (2025-02-13)

### Bugs Fixed
* Fixed data race when using Client from multiple goroutines concurrently (thanks, @strager)

## 1.1.0 (2024-10-21)

### Features Added
* Added CAE support
* `KeyVaultChallengePolicy` always requests tokens from the Vault's tenant, overriding any credential default

## 1.0.1 (2024-04-09)

### Other Changes
* Upgraded dependencies

## 1.0.0 (2023-08-15)

### Features Added
* This is the first stable release of the `internal` library for KeyVault.

### Other Changes
* Upgrade dependencies

## 0.8.0 (2023-03-08)

### Breaking Changes
* Moved to new location

### Other Changes
* Upgrade dependencies

## 0.7.1 (2022-11-14)

### Bugs Fixed
* `KeyVaultChallengePolicy` uses incorrect authentication scope when challenge verification is disabled

## 0.7.0 (2022-09-20)

### Breaking Changes
* Added `*KeyVaultChallengePolicyOptions` parameter to `NewKeyVaultChallengePolicy`

## 0.6.0 (2022-09-12)

### Breaking Changes
* Verify the challenge resource matches the vault domain. See https://aka.ms/azsdk/blog/vault-uri for more information.
* `ParseID()` no longer appends a trailing slash to vault URLs

## 0.5.0 (2022-05-12)

### Breaking Changes
* Removed `ExpiringResource` and its dependencies in favor of shared implementation from `internal/temporal`.

### Other Changes
* Updated to latest versions of `azcore` and `internal`.

## 0.4.0 (2022-04-22)

### Breaking Changes
* Updated `ExpiringResource` and its dependent types to use generics.

### Other Changes
* Remove reference to `TokenRequestOptions.TenantID` as it's been removed and wasn't working anyways.

## 0.3.0 (2022-04-04)

### Features Added
* Adds the `ParseKeyvaultID` function to parse an ID into the Key Vault URL, item name, and item version

### Breaking Changes
* Updates to azcore v0.23.0

## 0.2.1 (2022-01-31)

### Bugs Fixed
* Avoid retries on terminal failures (#16932)

## 0.2.0 (2022-01-12)

### Bugs Fixed
* Fixes a bug with Managed HSMs that prevented correctly authorizing requests.

## 0.1.0 (2021-11-09)
* This is the initial release of the `internal` library for KeyVault
