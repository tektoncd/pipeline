# Tekton Pipeline Remote Resolution Docs

## Getting Started Tutorial

For new users getting started with Tekton Pipeline remote resolution, check out the
[resolution-getting-started.md](./resolution-getting-started.md) tutorial.

## Built-in Resolvers

These resolvers are enabled by setting the appropriate feature flag in the `resolvers-feature-flags` 
ConfigMap in the `tekton-pipelines-resolvers` namespace.

1. [The `bundles` resolver](./bundle-resolver.md), enabled by setting the `enable-bundles-resolver`
   feature flag to `true`.
1. [The `git` resolver](./git-resolver.md), enabled by setting the `enable-git-resolver`
   feature flag to `true`.
1. [The `hub` resolver](./hub-resolver.md), enabled by setting the `enable-hub-resolver`
   feature flag to `true`.
1. [The `cluster` resolver](./cluster-resolver.md), enabled by setting the `enable-cluster-resolver`
   feature flag to `true`.

## Developer Howto: Writing a Resolver From Scratch

For a developer getting started with writing a new Resolver, see
[how-to-write-a-resolver.md](./how-to-write-a-resolver.md) and the
accompanying [resolver-template](./resolver-template).

## Resolver Reference: The interfaces and methods to implement

For a table of the interfaces and methods a resolver must implement
along with those that are optional, see [resolver-reference.md](./resolver-reference.md).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
