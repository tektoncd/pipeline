<!--
---
linkTitle: "PipelineResources"
weight: 208
---
-->

# PipelineResources

> :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
>
> Consider using replacement features instead. Read more in [documentation](migrating-v1alpha1-to-v1beta1.md#replacing-pipelineresources-with-tasks)
> and [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).


> **Note**: `PipelineResources` have not been promoted to Beta in tandem with Pipeline's other CRDs. 
> This means that the level of support for `PipelineResources` remains Alpha.
> `PipelineResources` are now [deprecated](deprecations.md#deprecation-table).** 
>
> For Beta-supported alternatives to PipelineResources see
> [the v1alpha1 to v1beta1 migration guide](./migrating-v1alpha1-to-v1beta1.md#pipelineresources-and-catalog-tasks)
> which lists each PipelineResource type and a suggested option for replacing it.
>
> For more information on why PipelineResources are remaining alpha [see the description
> of their problems, along with next steps, below](#why-aren-t-pipelineresources-in-beta).

--------------------------------------------------------------------------------
-   [Why Aren't PipelineResources in Beta?](#why-aren-t-pipelineresources-in-beta)

## Why Aren't PipelineResources in Beta?

The short answer is that they're not ready to be given a Beta level of support by Tekton's developers. The long answer is, well, longer:

- Their behaviour can be opaque.

    They're implemented as a mixture of injected Task Steps, volume configuration and type-specific code in Tekton
    Pipeline's controller. This means errors from `PipelineResources` can manifest in quite a few different ways
    and it's not always obvious whether an error directly relates to `PipelineResource` behaviour. This problem
    is compounded by the fact that, while our docs explain each Resource type's "happy path", there never seems to
    be enough info available to explain error cases sufficiently.

- When they fail they're difficult to debug.

    Several PipelineResources inject their own Steps before a `Task's` Steps. It's extremely difficult to manually
    insert Steps before them to inspect the state of a container before they run.

- There aren't enough of them.

    The six types of existing PipelineResources only cover a tiny subset of the possible systems and side-effects we
    want to support with Tekton Pipelines.

- Adding extensibility to them makes them really similar to `Tasks`:
    - User-definable `Steps`? This is what `Tasks` provide.
    - User-definable params? Tasks already have these.
    - User-definable "resource results"? `Tasks` have `Task` Results.
    - Sharing data between Tasks using PVCs? `workspaces` provide this for `Tasks`.
- They make `Tasks` less reusable.
    - A `Task` has to choose the `type` of `PipelineResource` it will accept.
    - If a `Task` accepts a `git` `PipelineResource` then it's not able to accept a `gcs` `PipelineResource` from a
      `TaskRun` or `PipelineRun` even though both the `git` and `gcs` `PipelineResources` fetch files. They should
      technically be interchangeable: all they do is write files from somewhere remote onto disk. Yet with the existing
      `PipelineResources` implementation they aren't interchangeable.

They also present challenges from a documentation perspective:

- Their purpose is ambiguous and it's difficult to articulate what the CRD is precisely for.
- Four of the types interact with external systems (git, pull-request, gcs, gcs-build).
- Five of them write files to a Task's disk (git, pull-request, gcs, gcs-build, cluster).
- One tells the Pipelines controller to emit CloudEvents to a specific endpoint (cloudEvent).
- One writes config to disk for a `Task` to use (cluster).
- One writes a digest in one `Task` and then reads it back in another `Task` (image).
- Perhaps the one thing you can say about the `PipelineResource` CRD is that it can create
  side-effects for your `Tasks`.

### What's still missing

So what are PipelineResources still good for?  We think we've identified some of the most important things:

1. You can augment `Task`-only workflows with `PipelineResources` that, without them, can only be done with `Pipelines`.
    - For example, let's say you want to checkout a git repo for your Task to test. You have two options. First, you could use a `git` PipelineResource and add it directly to your test `Task`. Second, you could write a `Pipeline` that has a `git-clone` `Task` which checks out the code onto a PersistentVolumeClaim `workspace` and then passes that PVC `workspace` to your test `Task`. For a lot of users the second workflow is totally acceptable but for others it isn't. Some of the most notable reasons we've heard are:
      - Some users simply cannot allocate storage on their platform, meaning `PersistentVolumeClaims` are out of the question.
      - Expanding a single `Task` workflow into a `Pipeline` is labor-intensive and feels unnecessary.
2. Despite being difficult to explain the whole CRD clearly each individual `type` is relatively easy to explain.
    - For example, users can build a pretty good "hunch" for what a `git` `PipelineResource` is without really reading any docs.
3. Configuring CloudEvents to be emitted by the Tekton Pipelines controller.
    - Work is ongoing to get notifications support into the Pipelines controller which should hopefully be able to replace the `cloudEvents` `PipelineResource`.

For each of these there is some amount of ongoing work or discussion. We have deprecated `PipelineResources` and are 
actively working to cover the missing replacement features. Read more about the deprecation in [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

For Beta-supported alternatives to PipelineResources see
[the v1alpha1 to v1beta1 migration guide](./migrating-v1alpha1-to-v1beta1.md#pipelineresources-and-catalog-tasks)
which lists each PipelineResource type and a suggested option for replacing it.

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
