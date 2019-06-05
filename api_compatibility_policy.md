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

## `TaskRun`, `Task`, and `ClusterTask`

The CRD types
[`Task`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md),
[`ClusterTask`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md#clustertask),
and
[`TaskRun`](https://github.com/tektoncd/pipeline/blob/master/docs/taskruns.md)
should be considered `alpha`, however these types are more stable than
`Pipeline`, `PipelineRun`, and `PipelineResource`.

### Possibly `beta` in 0.3

The status of these types will be revisited ~2 releases (i.e. 0.3) and see if
they can be promoted to `beta`.

Once these types are promoted to `beta`, any backwards incompatible changes must
be introduced in a backwards compatible manner first, with a deprecation warning
in the release notes, for at least one full release before the backward
incompatible change is made.

There are two reasons for this:

- `Task` and `TaskRun` are considered upgraded versions of [Build](https://github.com/knative/docs/blob/master/docs/build/builds.md#source) and [BuildTemplate](https://github.com/knative/docs/blob/master/docs/build/build-templates.md), meaning that the APIs benefit from a significant amount of user feedback and iteration
- Going forward users should use `TaskRun` and `Task` instead of `Build` and `BuildTemplate`, those users should not expect the API to be changed on them
  without warning

The exception to this is that `PipelineResource` definitions can be embedded in
`TaskRuns`, and since the `PipelineResource` definitions are considered less
stable, changes to the spec of the embedded `PipelineResource` can be introduced
between releases.

## `PipelineRun`, `Pipeline` and `PipelineResource`

The CRD types
[`Pipeline`](https://github.com/tektoncd/pipeline/blob/master/docs/pipelines.md),
[`PipelineRun`](https://github.com/tektoncd/pipeline/blob/master/docs/pipelines.md)
and
[`PipelineResource`](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md#pipelineresources)
should be considered `alpha`, i.e. the API should be considered unstable.
Backwards incompatible changes can be introduced between releases, however they
must include a backwards incompatibility warning in the release notes.

The reason for this is not yet having enough user feedback to commit to the APIs
as they currently exist. Once significant user input has been given into the API
design, we can upgrade these CRDs to `beta`.
