<!--
---
linkTitle: "Matrix"
weight: 11
---
-->

# Matrix

- [Overview](#overview)
- [Configuring a Matrix](#configuring-a-matrix)
  - [Context Variables](#context-variables)

## Overview

`Matrix` is used to fan out `Tasks` in a `Pipeline`. This doc will explain the details of `matrix` support in
Tekton. 

> :seedling: **`Matrix` is an [alpha](install.md#alpha-features) feature.**
> The `enable-api-fields` feature flag must be set to `"alpha"` to specify `Matrix` in a `PipelineTask`.
>
> :warning: This feature is in a preview mode. 
> It is still in a very early stage of development and is not yet fully functional.

## Configuring a Matrix

A `Matrix` supports the following features:
* [Context Variables](#context-variables)

### Context Variables

Similarly to the `Parameters` in the `Params` field, the `Parameters` in the `Matrix` field will accept 
[context variables](variables.md) that will be substituted, including:

* `PipelineRun` name, namespace and uid
* `Pipeline` name
* `PipelineTask` retries
