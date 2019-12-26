# API compatibility policy

This document proposes a policy regarding making API updates to the CRDs in this
repo. Users should be able to build on the APIs in these projects with a clear
idea of what they can rely on and what should be considered in progress and
therefore likely to change.

For these purposes the CRDs are divided into two groups:

- [`TaskRun`, `Task`, and `ClusterTask`] - "more stable"
- [`PipelineRun`, `Pipeline` and `PipelineResource`] - "less stable"

The use of `alpha`, `beta` and `GA` in this document is meant to correspond
roughly to
[the kubernetes API deprecation policies](https://kubernetes.io/docs/reference/using-api/deprecation-policy/#deprecating-a-flag-or-cli).

## What does compatibility mean here

This policy is about changes to the APIs of the
[CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
aka the spec of the CRD objects themselves.

A backwards incompatible change would be a change that requires a user to update
existing instances of these CRDs in clusters where they are deployed (after
[automatic conversion is available](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definition-versioning/#webhook-conversion)
this process may become less painful).

The current process would look something like:

1. Backup the instances
1. Delete the instances
1. Deploy the new type definitions
1. Update the backups with the new spec
1. Deploy the updated backups

TODO(bobcatfish): This policy really _should_ include the entire API.

The API is considered to consist of:

- The spec if the CRDs
- The order that `PipelineResources` declared within a `Task` are applied in

### Getting to beta

[This document](https://docs.google.com/document/d/1H8I2Rk4kLdQaR4mV0A71Qbk-1FxXFrmvisEAjLKT6H0/edit#)
(visible to members of [the mailing list](https://github.com/tektoncd/community/blob/master/contact.md#mailing-list))
describes our plan to get to beta.

#### Backwards compatible changes first

At this point, any backwards incompatible changes must
be introduced in a backwards compatible manner first, with a deprecation warning
in the release notes, for at least one full release before the backward
incompatible change is made.

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

These changes must be made [in a backwards compatible manner first](#backwards-compatible-changes-first),
and the changes must be approved by [more than half of the project OWNERS](OWNERS)
(i.e. 50% + 1).