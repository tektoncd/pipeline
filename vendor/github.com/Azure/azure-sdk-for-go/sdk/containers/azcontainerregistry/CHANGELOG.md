# Release History

## 0.2.3 (2025-04-15)

### Other Changes
* Default audience of Azure Container Registry of all clouds to https://containerregistry.azure.net

## 0.2.2 (2024-09-19)

### Features Added
* Add `AuthenticationClient` enabling third party libraries to interact with container and artifact registries

### Other Changes
* Updated dependencies.

## 0.2.1 (2024-01-24)

### Features Added
* Add `ConfigMediaType` and `MediaType` properties to `ManifestAttributes`
* Enabled spans for distributed tracing

### Other Changes
* Refine some logics and comments
* Updated to latest version of azcore

## 0.2.0 (2023-06-06)

### Features Added
* Add `DigestValidationReader` to help to do digest validation when read manifest or blob

### Breaking Changes
* Remove `MarshalJSON` for some of the types that are not used in the request.

### Bugs Fixed
* Add state restore for hash calculator when upload fails
* Do not re-calculate digest when retry

### Other Changes
* Change default audience to https://containerregistry.azure.net
* Refine examples of image upload and download

## 0.1.1 (2023-03-07)

### Bugs Fixed
* Fix possible failure when request retry

### Other Changes
* Rewrite auth policy to promote efficiency of auth process

## 0.1.0 (2023-02-07)

* This is the initial release of the `azcontainerregistry` library
