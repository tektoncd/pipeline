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

`v1beta1.CustomRun` is a beta CRD. Adding new fields to `CustomRun`
that all `CustomRun` controllers are required to support is considered a [backwards incompatible change](#backwards-incompatible-changes),
and follows the [beta policy](#beta-crds) for backwards incompatible changes.

`v1beta1.ResolutionRequest` is a beta CRD. Adding new fields to `ResolutionRequest`
that all resolvers are required to support is considered a [backwards incompatible change](#backwards-incompatible-changes),
and follows the [beta policy](#beta-crds) for backwards incompatible changes.

The following CRDs are deprecated. New features will not be added, but they will receive bug fixes.

- `v1beta1.Task`
- `v1beta1.ClusterTask`
- `v1beta1.TaskRun`
- `v1beta1.Pipeline`
- `v1beta1.PipelineRun`

To migrate to the v1 versions of these APIs, see [the migration guide](./docs/migrating-v1beta1-to-v1.md).
For more information on support windows, see the [deprecations table](./docs/deprecations.md).

### Alpha CRDs

- For Alpha CRDs, the `apiVersion` contains the `alpha` (e.g. `v1alpha1`)
  
- Features may be dropped at any time, though you will be given at least one release worth of warning.

### Beta CRDs

- The `apiVersion` field of the CRD contains contain `beta` (for example, `v1beta1`).

- These features will not be dropped, though the details may change.

- Any [backwards incompatible changes](#backwards-incompatible-changes) must be introduced in a backwards compatible manner first, with a deprecation warning in the release notes and migration instructions.

- Users will be given at least 9 months to migrate before a backward incompatible change is made. This means an older beta API version will continue to receive bug fixes in new releases for a period of at least 9 months from the time a newer version is made available.
  - As a special case, the `v1beta1` CRDs will not be removed until 1 year after they are deprecated,
  because no stable API was available when they were created. The 9 month policy will apply to future beta APIs.

- Alpha features may be present within a beta API version. However, they will not be enabled by default and must be enabled by setting `enable-api-fields` to `alpha`.

### GA CRDs

- The `apiVersion` field of the CRD is `vX` where `X` is an integer.

- Stable API versions remain available for all future releases within a [semver major version](https://semver.org/#summary).

- Stable features may be marked as deprecated but may only be removed by incrementing the api version (i.e v1 to v2).

- We will not make any backwards incompatible changes to fields that are stable without incrementing the api version.

- Alpha and Beta features may be present within a stable API version. Alpha features will not be enabled by default and must be enabled by setting `enable-api-fields` to `alpha`. To disable beta features as well, set `enable-api-fields` to `stable`.

## Feature Gates

Stability levels of feature gates are independent from CRD apiVersions. 

[TEP0138](https://github.com/tektoncd/community/blob/main/teps/0138-decouple-api-and-feature-versioning.md) has introduced per-feature flags for new API-driven features and the migration plan for `enable-api-fields`. Please refer to the table below for the API-driven features validation transition:

| Releases               | Global flag `enable-api-fields`    | Per-feature flag                   |
| ---------------------- | --------------------------------   | ---------------------------------- |
| Prior to v0.53.0       | All alpha/beta API-driven features |                                    |
| After v0.53.0        | [Existing alpha/beta API-driven features](https://github.com/tektoncd/community/blob/02418c0d39578a6a42f9d2d30caea8060dd89385/teps/0138-decouple-api-and-feature-versioning.md#sunset-enable-api-fields-after-existing-features-stabilize) prior to v0.53.0 | New alpha/beta API-driven features introduced after v0.53.0 |
| All [alpha/beta API-driven features in v0.53.0](https://github.com/tektoncd/community/blob/02418c0d39578a6a42f9d2d30caea8060dd89385/teps/0138-decouple-api-and-feature-versioning.md#sunset-enable-api-fields-after-existing-features-stabilize) become stable or are removed | Sunset ~~`enable-api-fields`~~ | All alpha/beta API-driven features |


_Note that behavioural(non-API-driven) flags will retain their original usage._

With per-feature flags, cluster operators are able to enable a single new feature with their dedicated feature flags. For instructions on how to add a per-feature flag, please check the [developer feature versioning guide](./docs/developers/feature-versioning.md#per-feature-flag).

Note that the `enable-api-fields` feature flag will continue to validate all features that were[beta](https://github.com/tektoncd/pipeline/blob/release-v0.52.x/docs/additional-configs.md#beta-features) and [alpha](https://github.com/tektoncd/pipeline/blob/release-v0.52.x/docs/additional-configs.md#alpha-features) prior to [v0.53.0](https://github.com/tektoncd/pipeline/tree/release-v0.53.x):

* `stable` - This value indicates that only fields of the highest stability level are enabled; i.e. `alpha` and `beta` fields are not enabled.

* `beta` (default) - This value indicates that only fields which are of `beta` (or greater) stability are enabled, i.e. `alpha` fields are not enabled.

* `alpha` - This value indicates that fields of all stability levels are enabled, specifically `alpha`, `beta` and `GA`(`stable`).

See the current list of [alpha features](https://github.com/tektoncd/pipeline/blob/main/docs/additional-configs.md#alpha-features) and [beta features](https://github.com/tektoncd/pipeline/blob/main/docs/additional-configs.md#beta-features).

| `enable-api-fields` value  | stable features enabled | beta features enabled | alpha features enabled |
|----------------------------|-------------------------|-----------------------|------------------------|
| stable                     |             x           |                       |                        |
| beta                       |             x           |          x            |                        |
| alpha                      |             x           |          x            |           x            |

### Alpha features

- Alpha features are disabled by default and must be enabled by [setting `enable-api-fields` to `alpha`](https://github.com/tektoncd/pipeline/blob/main/docs/additional-configs.md#alpha-features)

- These features may be dropped or backwards incompatible changes made at any time, though one release worth of warning will be provided.

- Alpha features are reviewed for promotion to beta at a regular basis. However, there is no guarantee that they will be promoted to beta.

### Beta features

- Beta features are enabled by default and can be disabled by [setting `enable-api-fields` to `stable`](https://github.com/tektoncd/pipeline/blob/main/docs/additional-configs.md#beta-features).

- These features will not be dropped, though the details may change.

- Beta features may be changed in a backwards incompatible way by following the same process as [Beta CRDs](#beta-crds) 
  i.e. by providing a 9 month support period.

- Beta features are reviewed for promotion to GA/Stable on a regular basis. However, there is no guarantee that they will be promoted to GA/stable.
  
### GA/Stable features

- GA/Stable features are enabled by default.
  - GA/Stable API-driven features are no longer controlled by the `enable-api-fields` flag because they cannot be disabled.

- GA/Stable features are features that have been promoted from beta to the highest level of stability. They cannot be disabled in any CRD version.

- GA/Stable features will not be removed or changed in a backwards incompatible manner without incrementing the API Version.

### Feature Graduation Process
Features are first released as experimental in alpha, refined in beta, and finalized in stable releases.

#### Introducing an `alpha` feature
- When a feature is first introduced to Tekton, it will have the `alpha` stability level and be disabled by default. 
- At this stage, users could choose to experiment with the feature. Feedback will be collected from users and it will help maintainers to determine whether to promote the feature to higher stability level or deprecate it.

#### Promoting a feature to `beta`
- After feedback of the usage of the alpha features, once the needs and motivations are validated, a feature could be promoted to `beta`. This stage is where features are further tested and refined.
- The dedicated feature flag for this feature will change its stability level to `beta`. It will continue to be disabled by default.

#### Graduating a feature to `stable`
- This is the final stage of feature graduation process, where features are considered to be complete and ready to be released for the public.
- Once a feature has graduated to `stable`, it will be turned on by default.

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
would be invalid after the change. Similarly, adding a new field to the ResolutionRequest API that
all resolvers are expected to support is also a backwards incompatible change.

These changes must be approved by [more than half of the project OWNERS](OWNERS)
(i.e. 50% + 1).

## Deprecated Features

Tekton Pipelines [maintains a list of features that have been deprecated](https://github.com/tektoncd/pipeline/tree/main/docs/deprecations.md)
which includes the earliest date each feature will be removed.

## Go libraries compatibility policy

In this context, "tombstoning" refers to retaining a field in client libraries, so that clients can continue to deserialize objects created with an older server version,
while disallowing new usage of the field.

### Covered parts of the codebase

The compatibility policy for the [github.com/tektoncd/pipeline Go package](https://pkg.go.dev/github.com/tektoncd/pipeline) currently only covers
Go structs representing CRDs in `pkg/apis`, the generated client libraries in `pkg/client`, and the [resolver interface](https://github.com/tektoncd/pipeline/blob/main/pkg/resolution/resolver/framework/interface.go).
Backwards compatibility for other parts of the codebase is on a best-effort basis.

Pipelines contributors are working on modularizing `pkg/apis`, so that clients only need to import a minimal set of dependencies.
This work is tracked in [issue #6483](https://github.com/tektoncd/pipeline/issues/6483).
Pipelines contributors also plan to move all internal functionality in the entire [github.com/tektoncd/pipeline Go package](https://pkg.go.dev/github.com/tektoncd/pipeline)
into internal packages. This work is tracked in [issue #5679](https://github.com/tektoncd/pipeline/issues/5679).
Please follow these issues for developments in this area and any related changes to compatibility policies.

### Alpha CRDs

- Go structs representing alpha CRDs may change in breaking ways at any time. If the change impacts the yaml/JSON API, we will provide one release of warning.

- If an alpha CRD stops being served, its client library will not be removed until the last release with support for the CRD has reached its end of life.

- If support for a field is dropped, the field may also be removed from the client libraries; i.e. it is not guaranteed to be tombstoned.

### Beta CRDs

- Go structs representing beta CRDs may change in breaking ways if the yaml/JSON API is unaffected.

- If a beta CRD stops being served, its client library will not be removed until the last release with support for the CRD has reached its end of life.

- If support for an alpha field is dropped, the field will be tombstoned in client libraries and will not be removed until the last release with support for the field.

### Stable CRDs

- Go structs representing stable fields may not change in breaking ways without a semver major version bump, even if the yaml/JSON API is unaffected.

- If a GA CRD stops being served (via a semver major version bump), the client libraries for this CRD will not be removed.

- If support for an alpha field is dropped, the field will be tombstoned in client libraries and will not be removed.

- If an alpha or beta field is renamed, the old field name will be tombstoned in client libraries and will not be removed.

### Resolver Interface

Remote resolution is currently a beta feature. [Issue #6579](https://github.com/tektoncd/pipeline/issues/6579) tracks promoting resolution to stable.
Backwards compatibility for the [resolver interface](https://github.com/tektoncd/pipeline/blob/main/pkg/resolution/resolver/framework/interface.go)
is currently on a best-effort basis. When remote resolution is promoted to stable, backwards incompatible changes may not be made to the resolver interface.

## Notes for Developers

Are you a Tekton contributor looking to make a backwards incompatible change?
Read more on additional considerations at [api-changes.md](./docs/developers/api-changes.md#deprecations).
