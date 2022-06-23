<!--
---
linkTitle: "Matrix"
weight: 11
---
-->

# Matrix

- [Overview](#overview)
- [Configuring a Matrix](#configuring-a-matrix)
  - [Concurrency Control](#concurrency-control)
  - [Parameters](#parameters)
  - [Context Variables](#context-variables)
  - [Results](#results)
    - [Specifying Results in a Matrix](#specifying-results-in-a-matrix)
    - [Results from fanned out PipelineTasks](#results-from-fanned-out-pipelinetasks)
- [Fan Out](#fan-out)
  - [`PipelineTasks` with `Tasks`](#pipelinetasks-with-tasks)

## Overview

`Matrix` is used to fan out `Tasks` in a `Pipeline`. This doc will explain the details of `matrix` support in
Tekton. 

Documentation for specifying `Matrix` in a `Pipeline`:
- [Specifying `Matrix` in `Tasks`](pipelines.md#specifying-matrix-in-pipelinetasks)
- [Specifying `Matrix` in `Finally Tasks`](pipelines.md#specifying-matrix-in-finally-tasks)
- [Specifying `Matrix` in `Custom Tasks`](pipelines.md#specifying-matrix)

> :seedling: **`Matrix` is an [alpha](install.md#alpha-features) feature.**
> The `enable-api-fields` feature flag must be set to `"alpha"` to specify `Matrix` in a `PipelineTask`.
> The `embedded-status` feature flag must be set to `"minimal"` to specify `Matrix` in a `PipelineTask`.

## Configuring a Matrix

A `Matrix` supports the following features:
* [Concurrency Control](#concurrency-control)
* [Parameters](#parameters)
* [Context Variables](#context-variables)
* [Results](#results) 

### Concurrency Control

The default maximum count of `TaskRuns` or `Runs` from a given `Matrix` is **256**. To customize the maximum count of
`TaskRuns` or `Runs` generated from a given `Matrix`, configure the `default-max-matrix-combinations-count` in 
[config defaults](/config/config-defaults.yaml). When a `Matrix` in `PipelineTask` would generate more than the maximum
`TaskRuns` or `Runs`, the `Pipeline` validation would fail.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-defaults
data:
  default-service-account: "tekton"
  default-timeout-minutes: "20"
  default-max-matrix-combinations-count: "1024"
  ...
```

For more information, see [installation customizations](/docs/install.md#customizing-basic-execution-parameters).

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

## Fan Out

### `PipelineTasks` with `Tasks`

When a `PipelineTask` has a `Task` and a `Matrix`, the `Task` will be executed in parallel `TaskRuns` with
substitutions from combinations of `Parameters`.

In the example below, nine `TaskRuns` are created with combinations of platforms ("linux", "mac", "windows")
and browsers ("chrome", "safari", "firefox").

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: platform-browsers
  annotations:
    description: |
      A task that does something cool with platforms and browsers
spec:
  params:
    - name: platform
    - name: browser
  steps:
    - name: echo
      image: alpine
      script: |
        echo "$(params.platform) and $(params.browser)"
---
# run platform-browsers task with:
#   platforms: linux, mac, windows
#   browsers: chrome, safari, firefox
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: matrixed-pr-
spec:
  serviceAccountName: 'default'
  pipelineSpec:
    tasks:
      - name: platforms-and-browsers
        matrix:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
        taskRef:
          name: platform-browsers
```

When the above `PipelineRun` is executed, these are the `TaskRuns` that are created:

```shell
$ tkn taskruns list

NAME                                         STARTED        DURATION     STATUS
matrixed-pr-6lvzk-platforms-and-browsers-8   11 seconds ago   7 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-6   12 seconds ago   7 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-7   12 seconds ago   9 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-4   12 seconds ago   7 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-5   12 seconds ago   6 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-3   13 seconds ago   7 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-1   13 seconds ago   8 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-2   13 seconds ago   8 seconds    Succeeded
matrixed-pr-6lvzk-platforms-and-browsers-0   13 seconds ago   8 seconds    Succeeded
```

When the above `Pipeline` is executed, its status is populated with `ChildReferences` of the above `TaskRuns`. The
`PipelineRun` status tracks the status of all the fanned out `TaskRuns`. This is the `PipelineRun` after completing
successfully:

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: matrixed-pr-
  labels:
    tekton.dev/pipeline: matrixed-pr-6lvzk
  name: matrixed-pr-6lvzk
  namespace: default
spec:
  pipelineSpec:
    tasks:
    - matrix:
      - name: platform
        value:
        - linux
        - mac
        - windows
      - name: browser
        value:
        - chrome
        - safari
        - firefox
      name: platforms-and-browsers
      taskRef:
        kind: Task
        name: platform-browsers
  serviceAccountName: default
  timeout: 1h0m0s
status:
  pipelineSpec:
    tasks:
      - matrix:
          - name: platform
            value:
              - linux
              - mac
              - windows
          - name: browser
            value:
              - chrome
              - safari
              - firefox
        name: platforms-and-browsers
        taskRef:
          kind: Task
          name: platform-browsers
  startTime: "2022-06-23T23:01:11Z"
  completionTime: "2022-06-23T23:01:20Z"
  conditions:
    - lastTransitionTime: "2022-06-23T23:01:20Z"
      message: 'Tasks Completed: 1 (Failed: 0, Cancelled 0), Skipped: 0'
      reason: Succeeded
      status: "True"
      type: Succeeded
  childReferences:
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-4
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-6
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-2
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-1
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-7
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-0
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-8
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-3
    pipelineTaskName: platforms-and-browsers
  - apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    name: matrixed-pr-6lvzk-platforms-and-browsers-5
    pipelineTaskName: platforms-and-browsers
```

To execute this example yourself, run [`PipelineRun` with `Matrix`][pr-with-matrix].

[pr-with-matrix]: ../examples/v1beta1/pipelineruns/alpha/pipelinerun-with-matrix.yaml