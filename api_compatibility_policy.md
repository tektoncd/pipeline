# API compatibility policy

This document proposes a policy regarding making API updates to the CRDs in this
repo. Users should be able to build on the APIs in these projects with a clear
idea of what they can rely on and what should be considered in progress and
therefore likely to change.

The use of `alpha`, `beta` and `GA` in this document is meant to correspond
roughly to
[the kubernetes API deprecation policies](https://kubernetes.io/docs/reference/using-api/deprecation-policy/#deprecating-a-flag-or-cli).

## What is the API?

The API is considered to consist of:

- The structure of the CRDs including the `spec` and `status` sections, as well as:
  - The ordering of the `steps` within the `status`
  - The naming of the `step` containers within the `status`
  - The labels propagated from `PipelineRuns` to `TaskRuns` and `TaskRuns` to `Pods`.
- The structure of the [directories created in executing containers by Tekton](docs/tasks.md#reserved-directories)
- The order that `PipelineResources` declared within a `Task` are applied in
- The interfaces of the images that are built as part of Tekton Pipelines,
  i.e. images in [cmd](https://github.com/tektoncd/pipeline/tree/master/cmd) which are used as part of
  [PipelineResources](docs/resources.md)

This policy is about changes to any of the above facets of the API.

A backwards incompatible change would be a change that requires a user to update
existing instances of these CRDs in clusters where they are deployed and/or cause them
to change their CRD definitions to account for changes in the above.

## Alpha, Beta and GA

Some of our CRDs and features are considered alpha, some are beta, and we are working
toward GA.

The following CRDs are considered beta, though features may be introduced that are
alpha:

* `Task`
* `TaskRun`
* `ClusterTask`
* `Pipeline`
* `PipelineRun`

We are following [the Kubernetes definitions of these stages](https://kubernetes.io/docs/reference/using-api/api-overview/#api-versioning)
and we are following [the Kubernetes deprecation policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/).

* Alpha:
  * These features may be dropped at any time, though you will be given at least
    one release worth of warning.
* Beta:
  * These features will not be dropped, though the details may change.
  * Any [backwards incompatible changes](#backwards-incompatible-changes) must be
    introduced in a backwards compatible manner first, with a deprecation warning
    in the release notes and migration instructions.
  * You will be given at least 9 months to migrate before a backward incompatible
    change is made. This means an older beta API version will continue to be
    supported in new releases for a period of at least 9 months from the time a
    newer version is made available.

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

These changes must be approved by [more than half of the project OWNERS](OWNERS)
(i.e. 50% + 1).

## Deprecated Features

Tekton Pipelines [maintains a list of features that have been deprecated](https://github.com/tektoncd/pipeline/tree/master/docs/deprecations.md)
which includes the earliest date each feature will be removed.
