# v1.21.1 (2023-05-04)

* No change notes available for this release.

# v1.21.0 (2023-05-01)

* **Feature**: This release makes the NitroEnclave request parameter Recipient and the response field for CiphertextForRecipient available in AWS SDKs. It also adds the regex pattern for CloudHsmClusterId validation.

# v1.20.12 (2023-04-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.11 (2023-04-20)

* No change notes available for this release.

# v1.20.10 (2023-04-10)

* No change notes available for this release.

# v1.20.9 (2023-04-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.8 (2023-03-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.7 (2023-03-10)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.6 (2023-02-28)

* **Documentation**: AWS KMS is deprecating the RSAES_PKCS1_V1_5 wrapping algorithm option in the GetParametersForImport API that is used in the AWS KMS Import Key Material feature. AWS KMS will end support for this wrapping algorithm by October 1, 2023.

# v1.20.5 (2023-02-22)

* **Bug Fix**: Prevent nil pointer dereference when retrieving error codes.

# v1.20.4 (2023-02-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.3 (2023-02-15)

* **Announcement**: When receiving an error response in restJson-based services, an incorrect error type may have been returned based on the content of the response. This has been fixed via PR #2012 tracked in issue #1910.
* **Bug Fix**: Correct error type parsing for restJson services.

# v1.20.2 (2023-02-03)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.20.1 (2023-01-23)

* No change notes available for this release.

# v1.20.0 (2023-01-05)

* **Feature**: Add `ErrorCodeOverride` field to all error structs (aws/smithy-go#401).

# v1.19.4 (2022-12-15)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.19.3 (2022-12-14)

* No change notes available for this release.

# v1.19.2 (2022-12-07)

* **Documentation**: Updated examples and exceptions for External Key Store (XKS).

# v1.19.1 (2022-12-02)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.19.0 (2022-11-29.2)

* **Feature**: AWS KMS introduces the External Key Store (XKS), a new feature for customers who want to protect their data with encryption keys stored in an external key management system under their control.

# v1.18.18 (2022-11-22)

* No change notes available for this release.

# v1.18.17 (2022-11-16)

* No change notes available for this release.

# v1.18.16 (2022-11-10)

* No change notes available for this release.

# v1.18.15 (2022-10-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.14 (2022-10-21)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.13 (2022-10-20)

* No change notes available for this release.

# v1.18.12 (2022-10-13)

* No change notes available for this release.

# v1.18.11 (2022-09-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.10 (2022-09-14)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.9 (2022-09-02)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.8 (2022-08-31)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.7 (2022-08-30)

* No change notes available for this release.

# v1.18.6 (2022-08-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.5 (2022-08-22)

* No change notes available for this release.

# v1.18.4 (2022-08-11)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.3 (2022-08-09)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.2 (2022-08-08)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.1 (2022-08-01)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.18.0 (2022-07-18)

* **Feature**: Added support for the SM2 KeySpec in China Partition Regions

# v1.17.5 (2022-07-05)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.4 (2022-06-29)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.3 (2022-06-07)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.2 (2022-05-17)

* **Documentation**: Add HMAC best practice tip, annual rotation of AWS managed keys.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.1 (2022-04-25)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.17.0 (2022-04-19)

* **Feature**: Adds support for KMS keys and APIs that generate and verify HMAC codes

# v1.16.3 (2022-03-30)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.16.2 (2022-03-24)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.16.1 (2022-03-23)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.16.0 (2022-03-08)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.15.0 (2022-02-24)

* **Feature**: API client updated
* **Feature**: Adds RetryMaxAttempts and RetryMod to API client Options. This allows the API clients' default Retryer to be configured from the shared configuration files or environment variables. Adding a new Retry mode of `Adaptive`. `Adaptive` retry mode is an experimental mode, adding client rate limiting when throttles reponses are received from an API. See [retry.AdaptiveMode](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/retry#AdaptiveMode) for more details, and configuration options.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.14.0 (2022-01-14)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.13.0 (2022-01-07)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.12.0 (2021-12-21)

* **Feature**: API Paginators now support specifying the initial starting token, and support stopping on empty string tokens.
* **Feature**: Updated to latest service endpoints

# v1.11.1 (2021-12-02)

* **Bug Fix**: Fixes a bug that prevented aws.EndpointResolverWithOptions from being used by the service client. ([#1514](https://github.com/aws/aws-sdk-go-v2/pull/1514))
* **Dependency Update**: Updated to the latest SDK module versions

# v1.11.0 (2021-11-19)

* **Feature**: API client updated
* **Dependency Update**: Updated to the latest SDK module versions

# v1.10.0 (2021-11-12)

* **Feature**: Service clients now support custom endpoints that have an initial URI path defined.

# v1.9.0 (2021-11-06)

* **Feature**: The SDK now supports configuration of FIPS and DualStack endpoints using environment variables, shared configuration, or programmatically.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Feature**: Updated service to latest API model.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.8.0 (2021-10-21)

* **Feature**: API client updated
* **Feature**: Updated  to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.7.0 (2021-10-11)

* **Feature**: API client updated
* **Dependency Update**: Updated to the latest SDK module versions

# v1.6.1 (2021-09-17)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.6.0 (2021-09-02)

* **Feature**: API client updated

# v1.5.0 (2021-08-27)

* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.3 (2021-08-19)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.2 (2021-08-04)

* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version.
* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.1 (2021-07-15)

* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.4.0 (2021-06-25)

* **Feature**: API client updated
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.2 (2021-06-04)

* No change notes available for this release.

# v1.3.1 (2021-05-20)

* **Dependency Update**: Updated to the latest SDK module versions

# v1.3.0 (2021-05-14)

* **Feature**: Constant has been added to modules to enable runtime version inspection for reporting.
* **Dependency Update**: Updated to the latest SDK module versions

