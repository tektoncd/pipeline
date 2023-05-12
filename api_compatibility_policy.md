# API compatibility policy

This document proposes a policy regarding making updates to the Tekton API surface in this
repo. Users should be able to build on the APIs in these projects with a clear
idea of what they can rely on and what should be considered in progress and
therefore likely to change.

## What is the API?

The API is considered to consist of:

- The structure of the CRDs including the `spec` and `status` sections, as well as:
  - The ordering of the `steps` within the `status`
  - The naming of the `step` containers within the `status`
  - The labels propagated from `PipelineRuns` to `TaskRuns` and `TaskRuns` to `Pods`.
- The structure of the [directories created in executing containers by Tekton](docs/tasks.md#reserved-directories)
- The interfaces of the images that are built as part of Tekton Pipelines,
  i.e. images in [cmd](https://github.com/tektoncd/pipeline/tree/main/cmd)

This policy is about changes to any of the above facets of the API.

A backwards incompatible change would be a change that requires a user to update
existing instances of these CRDs in clusters where they are deployed and/or cause them
to change their CRD definitions to account for changes in the above.

## CRD API Versions

The `apiVersion` field in a Tekton CRD determines whether the overall API (and its default behaviors) is considered to be in `alpha`, `beta`, or `GA`. The use of `alpha`, `beta` and `GA` (aka `stable`) follows the corresponding Kubernetes [API stages definition](https://kubernetes.io/docs/reference/using-api/#api-versioning).

Within a stable CRD, certain opt-in features or API fields gated may be considered `alpha` or `beta`. Similarly, within a beta CRD, certain opt-in features may be considered `alpha`. See the section on Feature Gates for details.

The following CRDs are considered stable, though features may be introduced that are
alpha or beta:

- `v1.Task`
- `v1.TaskRun`
- `v1.Pipeline`
- `v1.PipelineRun`

`v1beta1.CustomRun` is considered a beta CRD. Adding new fields to `CustomRun`
that all `CustomRun` controllers are required to support is considered a [backwards incompatible change](#backwards-incompatible-changes),
and follows the [beta policy](#beta-crds) for backwards incompatible changes.

`v1beta1.ClusterTask` is a deprecated beta CRD. New features will not be added to `ClusterTask`.

### Alpha CRDs

- For Alpha CRDs, the `apiVersion` contains the `alpha` (e.g. `v1alpha1`)
  
- Features may be dropped at any time, though you will be given at least one release worth of warning.

### Beta CRDs

- The `apiVersion` field of the CRD contains contain `beta` (for example, `v1beta1`).

- These features will not be dropped, though the details may change.

- Any [backwards incompatible changes](#backwards-incompatible-changes) must be introduced in a backwards compatible manner first, with a   deprecation warning in the release notes and migration instructions.

- Users will be given at least 9 months to migrate before a backward incompatible change is made. This means an older beta API version will continue to be supported in new releases for a period of at least 9 months from the time a newer version is made available.


### GA CRDs

- The `apiVersion` field of the CRD is `vX` where `X` is an integer.

- Stable API versions remain available for all future releases within a [semver major version](https://semver.org/#summary).

- Stable features may be marked as deprecated but may only be removed by incrementing the api version (i.e v1 to v2).

- We will not make any backwards incompatible changes to fields that are stable without incrementing the api version.

- Alpha and Beta features may be present within a stable API version. However, they will not be enabled by default and must be enabled by setting `enable-api-fields` to `alpha` or `beta`.

## Feature Gates

CRD API versions gate the overall stability of the CRD and its default behaviors. Within a particular CRD version, certain opt-in features may be at a lower stability level as described in [TEP-33](https://github.com/tektoncd/community/blob/main/teps/0033-tekton-feature-gates.md). These fields may be disabled by default and can be enabled by setting the right `enable-api-fields` feature-flag as described in TEP-33:

* `stable` (default) - This value indicates that only fields of the highest stability level are enabled; For `beta` CRDs, this means only     beta stability fields are enabled, i.e. `alpha` fields are not enabled. For  `GA` CRDs, this means only `GA` fields are enabled by defaultd, i.e. `beta` and `alpha` fields would not be enabled.
* `beta` - This value indicates that only fields which are of `beta` (or greater) stability are enabled, i.e. `alpha` fields are not enabled. 
* `alpha` - This value indicates that fields of all stability levels are enabled, specifically `alpha`, `beta` and `GA`.


| Feature Versions -> | v1 | beta | alpha |
|---------------------|----|------|-------|
| stable              | x  |      |       |
| beta                | x  | x    |       |
| alpha               | x  | x    | x     |


See the current list of [alpha features](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#alpha-features) and [beta features](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#beta-features).


### Alpha features

- Alpha feature in beta or GA CRDs are disabled by default and must be enabled by [setting `enable-api-fields` to `alpha`](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#alpha-features)

- These features may be dropped or backwards incompatible changes made at any time though will be given at least one release worth of warning

- Alpha features are reviewed for promotion to beta at a regular basis. However, there is no guarantee that they will be promoted to beta.

### Beta features

- Beta features in GA CRDs are disabled by default and must be enabled by [setting `enable-api-fields` to `beta`](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#beta-features). In beta API versions, beta features are enabled by default.

- Beta features may be deprecated or changed in a backwards incompatible way by following the same process as [Beta CRDs](#beta-crds) 
  i.e. by providing a 9 month support period.

- Beta features are reviewed for promotion to GA/Stable on a regular basis. However, there is no guarantee that they will be promoted to GA/stable.

- For beta API versions, beta is the highest level of stability possible for any feature.
  
### GA/Stable features

- GA/Stable features are present in a [GA CRD](#ga-crds) only.

- GA/Stable features are enabled by default

- GA/Stable features will not be removed or changed in a backwards incompatible manner without incrementing the API Version.

## Approving API changes

API changes must be approved by [OWNERS](OWNERS). The policy is slightly different
for [additive changes](#additive-changes) vs.
[backwards incompatible changes](#backwards-incompatible-changes).

### Additive changes

Additive changes are changes that add to the API and do not cause problems for users
of previous versions of the API.

These changes must be approved by at least 2 [OWNERS](OWNERS).

### Backwards incompatible changes

Backwards incompatible changes change the API, e.g. by removing fields from a CRD
spec. These changes will mean that folks using a previous version of the API will need
to adjust their usage in order to use the new version.
Adding a new field to the CustomRun API that all CustomRun controllers are expected to support
is also a backwards incompatible change, as CustomRun controllers that were valid before the change
would be invalid after the change.

These changes must be approved by [more than half of the project OWNERS](OWNERS)
(i.e. 50% + 1).

## Deprecated Features

Tekton Pipelines [maintains a list of features that have been deprecated](https://github.com/tektoncd/pipeline/tree/main/docs/deprecations.md)
which includes the earliest date each feature will be removed.

## Go libraries compatibility policy

Tekton Pipelines may introduce breaking changes to its Go client libraries, as long as these changes
do not impact users' yaml/json CRD definitions. For example, a change that renames a field of a CRD
would need to follow the policy on [backwards incompatible changes](#backwards-incompatible-changes),
but a change that renames the Go struct type for that field is allowed.

## Notes for Developers

Are you a Tekton contributor looking to make a backwards incompatible change?
Read more on additional considerations at [api-changes.md](./docs/developers/api-changes.md#deprecations).
