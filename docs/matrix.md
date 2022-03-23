<!--
---
linkTitle: "Matrix"
weight: 11
---
-->

# Matrix

- [Overview](#overview)
- [Configuring a Matrix](#configuring-a-matrix)
  - [Parameters](#parameters)
  - [Context Variables](#context-variables)
  - [Results](#results)
    - [Specifying Results in a Matrix](#specifying-results-in-a-matrix)
    - [Results from fanned out PipelineTasks](#results-from-fanned-out-pipelinetasks)

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
* [Parameters](#parameters)
* [Context Variables](#context-variables)

### Parameters

The `Matrix` will take `Parameters` of type `"array"` only, which will be supplied to the
`PipelineTask` by substituting `Parameters` of type `"string"` in the underlying `Task`.
The names of the `Parameters` in the `Matrix` must match the names of the `Parameters`
in the underlying `Task` that they will be substituting.

In the example below, the *test* `Task` takes *browser* and *platform* `Parameters` of type
`"string"`. A `Pipeline` used to fan out the `Task` using `Matrix` would have two `Parameters`
of type `"array"`, and it would execute nine `TaskRuns`:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: platform-browser-tests
spec:
  params:
    - name: platforms
      type: array
      default:
        - linux
        - mac
        - windows
    - name: browsers
      type: array
      default:
        - chrome
        - safari
        - firefox    
  tasks:
  - name: fetch-repository
    taskRef:
      name: git-clone
    ...
  - name: test
    matrix:
      - name: platform
        value: $(params.platforms)
      - name: browser
        value: $(params.browsers)
    taskRef:
      name: browser-test
  ...
```

A `Parameter` can be passed to either the `matrix` or `params` field, not both. 

For further details on specifying `Parameters` in the `Pipeline` and passing them to
`PipelineTasks`, see [documentation](pipelines.md#specifying-parameters).

### Context Variables

Similarly to the `Parameters` in the `Params` field, the `Parameters` in the `Matrix` field will accept 
[context variables](variables.md) that will be substituted, including:

* `PipelineRun` name, namespace and uid
* `Pipeline` name
* `PipelineTask` retries

### Results

#### Specifying Results in a Matrix

Consuming `Results` from previous `TaskRuns` or `Runs` in a `Matrix`, which would dynamically generate 
`TaskRuns` or `Runs` from the fanned out `PipelineTask`, is not yet supported. This dynamic fan out of
`PipelineTasks` through consuming `Results` will be supported soon. 

#### Results from fanned out PipelineTasks

Consuming `Results` from fanned out `PipelineTasks` will not be in the supported in the initial iteration
of `Matrix`. Supporting consuming `Results` from fanned out `PipelineTasks` will be revisited after array
and object `Results` are supported. 
