## Introduction
FIPS compliance requires compiling the project with a Go FIPS-compliant compiler (e.g., golang-fips) and using dynamic linking.

This approach works for most binaries in tektoncd/pipeline, except for the entrypoint, which must be statically compiled to ensure it runs in any environment, regardless of library locations or versions. To mark a statically compiled binary as FIPS compliant, we must eliminate cryptographic symbols (crypto/*, golang.org/x/crypto, etc.).

To achieve this, we need compile-time options to disable TLS, SPIRE, and any network-related functionality.

This document provides instructions on compiling the entrypoint command to ensure FIPS compliance.

## Disable SPIRE during Build
To disable SPIRE during the build process, use the following command

```shell
CGO_ENABLED=0 go build -tags disable_spire -o bin/entrypoint ./cmd/entrypoint
```
