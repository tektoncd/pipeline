<!--
---
title: "Feature Flags"
linkTitle: "Feature Flags"
weight: 109
description: >
  Tekton Pipelines feature flags
---
-->

# Feature Flags in Tekton Pipelines

This document gives an overview of feature flags in Tekton Pipelines.

See [feature stability levels](./../api_compatibility_policy.md#feature-gates) for an explanation of the stability level for a feature.

## Table of Contents
- [Feature Stabilities](#feature-stability-level)
  - [Stable Features](#stable-features)
  - [Beta Features](#beta-features)
  - [Alpha Features](#alpha-features)


## Feature stability levels

### Stable Features

| Feature                                                            | Proposal                                                                                          | Alpha Release                                                        | Beta Release                                                  | Stable Release |  Individual Flag |
|:-------------------------------------------------------------------|:--------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------|:-------------------------------------------------------------|:---------------|:-----------------|
| [Propagated `Parameters`](./taskruns.md#propagated-parameters) | [TEP-0107](https://github.com/tektoncd/community/blob/main/teps/0107-propagating-parameters.md) | [v0.36.0](https://github.com/tektoncd/pipeline/releases/tag/v0.36.0) | [v0.45.0](https://github.com/tektoncd/pipeline/releases/tag/v0.45.0) | [v0.47.0](https://github.com/tektoncd/pipeline/releases/tag/v0.47.0) | |
| [Propagated `Workspaces`](./pipelineruns.md#propagated-workspaces) | [TEP-0111](https://github.com/tektoncd/community/blob/main/teps/0111-propagating-workspaces.md) | [v0.40.0](https://github.com/tektoncd/pipeline/releases/tag/v0.40.0) | [v0.45.0](https://github.com/tektoncd/pipeline/releases/tag/v0.45.0) | |
| [CSI workspaces](workspaces.md#csi)| [issue#4446](https://github.com/tektoncd/pipeline/issues/4446)| [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0)| [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0) | [v0.50.0](https://github.com/tektoncd/pipeline/releases/tag/v0.50.0) | |
| [Projected volume workspaces](workspaces.md#projected)| [issue#5075](https://github.com/tektoncd/pipeline/issues/5075)| [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0)| [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0) | [v0.50.0](https://github.com/tektoncd/pipeline/releases/tag/v0.50.0) | |



### Beta Features

Beta features are fields of stable CRDs that follow our "beta" [compatibility policy](../api_compatibility_policy.md).
To enable these features, set the `enable-api-fields` feature flag to `"beta"` in
the `feature-flags` ConfigMap alongside your Tekton Pipelines deployment via
`kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"enable-api-fields":"beta"}}'`.

For beta versions of Tekton CRDs, setting `enable-api-fields` to "beta" is the same as setting it to "stable".

Features currently in "beta" are:

| Feature                                                            | Proposal                                                                                        | Alpha Release                                                        | Beta Release                                                         | Individual Flag | `enable-api-fields=beta` required for `v1beta1` |
|:-------------------------------------------------------------------|:------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------|:---------------------------------------------------------------------|:----------------|:---|
| [Array Results and Array Indexing](pipelineruns.md#specifying-parameters)             | [TEP-0076](https://github.com/tektoncd/community/blob/main/teps/0076-array-result-types.md)     | [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0) | [v0.45.0](https://github.com/tektoncd/pipeline/releases/tag/v0.45.0) |                 | No |
| [Object Parameters and Results](pipelineruns.md#specifying-parameters) | [TEP-0075](https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md) | | [v0.46.0](https://github.com/tektoncd/pipeline/releases/tag/v0.46.0) | | No |
| [Remote Tasks](./taskruns.md#remote-tasks) and [Remote Pipelines](./pipelineruns.md#remote-pipelines) | [TEP-0060](https://github.com/tektoncd/community/blob/main/teps/0060-remote-resolution.md) | | [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0) | | No |
| [`Provenance` field in Status](pipeline-api.md#provenance)| [issue#5550](https://github.com/tektoncd/pipeline/issues/5550)| [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0)| [v0.48.0](https://github.com/tektoncd/pipeline/releases/tag/v0.48.0) | `enable-provenance-in-status`| No |
| [Isolated `Step` & `Sidecar` `Workspaces`](./workspaces.md#isolated-workspaces)                     | [TEP-0029](https://github.com/tektoncd/community/blob/main/teps/0029-step-workspaces.md)                                   | [v0.24.0](https://github.com/tektoncd/pipeline/releases/tag/v0.24.0) | [v0.50.0](https://github.com/tektoncd/pipeline/releases/tag/v0.50.0) | | Yes |

### Alpha Features

Alpha features in the following table are still in development and their syntax is subject to change.
- To enable the features ***without*** an individual flag:
  set the `enable-api-fields` feature flag to `"alpha"` in the `feature-flags` ConfigMap alongside your Tekton Pipelines deployment via `kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"enable-api-fields":"alpha"}}'`.
- To enable the features ***with*** an individual flag:
  set the individual flag accordingly in the `feature-flag` ConfigMap alongside your Tekton Pipelines deployment. Example: `kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"<FLAG-NAME>":"<FLAG-VALUE>"}}'`.


Features currently in "alpha" are:

| Feature                                                                                             | Proposal                                                                                                                   | Release                                                              | Individual Flag               |
|:----------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------|:------------------------------|
| [Bundles ](./pipelineruns.md#tekton-bundles)                                                        | [TEP-0005](https://github.com/tektoncd/community/blob/main/teps/0005-tekton-oci-bundles.md)                                | [v0.18.0](https://github.com/tektoncd/pipeline/releases/tag/v0.18.0) | `enable-tekton-oci-bundles`   |
| [Hermetic Execution Mode](./hermetic.md)                                                            | [TEP-0025](https://github.com/tektoncd/community/blob/main/teps/0025-hermekton.md)                                         | [v0.25.0](https://github.com/tektoncd/pipeline/releases/tag/v0.25.0) |                               |
| [Windows Scripts](./tasks.md#windows-scripts)                                                       | [TEP-0057](https://github.com/tektoncd/community/blob/main/teps/0057-windows-support.md)                                   | [v0.28.0](https://github.com/tektoncd/pipeline/releases/tag/v0.28.0) |                               |
| [Debug](./debug.md)                                                                                 | [TEP-0042](https://github.com/tektoncd/community/blob/main/teps/0042-taskrun-breakpoint-on-failure.md)                     | [v0.26.0](https://github.com/tektoncd/pipeline/releases/tag/v0.26.0) |                               |
| [Step and Sidecar Overrides](./taskruns.md#overriding-task-steps-and-sidecars)                      | [TEP-0094](https://github.com/tektoncd/community/blob/main/teps/0094-specifying-resource-requirements-at-runtime.md)       | [v0.34.0](https://github.com/tektoncd/pipeline/releases/tag/v0.34.0) |                               |
| [Matrix](./matrix.md)                                                                               | [TEP-0090](https://github.com/tektoncd/community/blob/main/teps/0090-matrix.md)                                            | [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0) |                               |
| [Task-level Resource Requirements](compute-resources.md#task-level-compute-resources-configuration) | [TEP-0104](https://github.com/tektoncd/community/blob/main/teps/0104-tasklevel-resource-requirements.md)                   | [v0.39.0](https://github.com/tektoncd/pipeline/releases/tag/v0.39.0) |                               |
| [Trusted Resources](./trusted-resources.md)                                                         | [TEP-0091](https://github.com/tektoncd/community/blob/main/teps/0091-trusted-resources.md)                                 | N/A                                                                  | `trusted-resources-verification-no-match-policy`  |
| [Larger Results via Sidecar Logs](#enabling-larger-results-using-sidecar-logs)                      | [TEP-0127](https://github.com/tektoncd/community/blob/main/teps/0127-larger-results-via-sidecar-logs.md)                   | [v0.43.0](https://github.com/tektoncd/pipeline/releases/tag/v0.43.0) | `results-from`                |
| [Configure Default Resolver](./resolution.md#configuring-built-in-resolvers)                        | [TEP-0133](https://github.com/tektoncd/community/blob/main/teps/0133-configure-default-resolver.md)                        | N/A                                 |                                |
| [Coschedule](./affinityassistants.md)                        | [TEP-0135](https://github.com/tektoncd/community/blob/main/teps/0135-coscheduling-pipelinerun-pods.md)                        | N/A                                 |`coschedule`                                |
