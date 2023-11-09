<!--
---
linkTitle: "StepActions"
weight: 201
---
-->

# StepActions

- [Overview](#overview)

## Overview
:warning: This feature is in a preview mode.
It is still in a very early stage of development and is not yet fully functional.

A `StepAction` is the reusable and scriptable unit of work that is performed by a `Step`.

A `Step` is not reusable, the work it performs is reusable and referenceable. `Steps` are in-lined in the `Task` definition and either perform work directly or perform a `StepAction`. A `StepAction` cannot be run stand-alone (unlike a `TaskRun` or a `PipelineRun`). It has to be referenced by a `Step`. Another way to ehink about this is that a `Step` is not composed of `StepActions` (unlike a `Task` being composed of `Steps` and `Sidecars`). Instead, a `Step` is an actionable component, meaning that it has the ability to refer to a `StepAction`. The author of the `StepAction` must be able to compose a `Step` using a `StepAction` and provide all the necessary context (or orchestration) to it.


## Configuring a `StepAction`

A `StepAction` definition supports the following fields:

- Required
  - [`apiVersion`][kubernetes-overview] - Specifies the API version. For example,
    `tekton.dev/v1alpha1`.
  - [`kind`][kubernetes-overview] - Identifies this resource object as a `StepAction` object.
  - [`metadata`][kubernetes-overview] - Specifies metadata that uniquely identifies the
    `StepAction` resource object. For example, a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for this `StepAction` resource object.
  - `image` - Specifies the image to use for the `Step`.
    - The container image must abide by the [container contract](./container-contract.md).
- Optional
  - `command`
    - cannot be used at the same time as using `script`.
  - `args`
  - `script`
    - cannot be used at the same time as using `command`.
  - `env`
  - [`params`](#declaring-params)
  - [`results`](#declaring-results)
  - [`securityContext`](#declaring-securitycontext)

The non-functional example below demonstrates the use of most of the above-mentioned fields:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: example-stepaction-name
spec:
  env:
    - name: HOME
      value: /home
  image: ubuntu
  command: ["ls"]
  args: ["-lh"]
```

### Declaring Parameters

Like with `Tasks`, a `StepAction` must declare all the parameters that it used. The same rules for `Parameter` [name](./tasks.md/#parameter-name), [type](./tasks.md/#parameter-type) (including [object](./tasks.md/#object-type), [array](./tasks.md/#array-type) and [string](./tasks.md/#string-type)) apply as when declaring them in `Tasks`. A `StepAction` can also provide [default value](./tasks.md/#default-value) to a `Parameter`.

 `Parameters` are passed to the `StepAction` from its corresponding `Step` referencing it.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: stepaction-using-params
spec:
  params:
    - name: gitrepo
      type: object
      properties:
        url:
          type: string
        commit:
          type: string
    - name: flags
      type: array
    - name: outputPath
      type: string
      default: "/workspace"
  image: some-git-image
  args: [
    "-url=$(params.gitrepo.url)",
    "-revision=$(params.gitrepo.commit)",
    "-output=$(params.outputPath)",
    "$(params.flags[*])",
  ]
```

### Declaring Results

A `StepAction` also declares the results that it will emit.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: stepaction-declaring-results
spec:
  results:
    - name: current-date-unix-timestamp
      description: The current date in unix timestamp format
    - name: current-date-human-readable
      description: The current date in human readable format
  image: bash:latest
  script: |
    #!/usr/bin/env bash
    date +%s | tee $(results.current-date-unix-timestamp.path)
    date | tee $(results.current-date-human-readable.path)
```

### Declaring SecurityContext

You can declare `securityContext` in a `StepAction`:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: StepAction
metadata:
  name: example-stepaction-name
spec:
  image:  gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init:latest
  securityContext:
      runAsUser: 0
  script: |
    # clone the repo
    ...
```

Note that the `securityContext` from `StepAction` will overwrite the `securityContext` from [`TaskRun`](./taskruns.md/#example-of-running-step-containers-as-a-non-root-user).

## Referencing a StepAction

`StepActions` can be referenced from the `Step` using the `ref` field, as follows:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: step-action-run
spec:
  TaskSpec:
    steps:
      - name: action-runner
        ref:
          name: step-action
```

Upon resolution and execution of the `TaskRun`, the `Status` will look something like:

```yaml
status:
  completionTime: "2023-10-24T20:28:42Z"
  conditions:
  - lastTransitionTime: "2023-10-24T20:28:42Z"
    message: All Steps have completed executing
    reason: Succeeded
    status: "True"
    type: Succeeded
  podName: step-action-run-pod
  provenance:
    featureFlags:
      EnableStepActions: true
      ...
  startTime: "2023-10-24T20:28:32Z"
  steps:
  - container: step-action-runner
    imageID: docker.io/library/alpine@sha256:eece025e432126ce23f223450a0326fbebde39cdf496a85d8c016293fc851978
    name: action-runner
    terminated:
      containerID: containerd://46a836588967202c05b594696077b147a0eb0621976534765478925bb7ce57f6
      exitCode: 0
      finishedAt: "2023-10-24T20:28:42Z"
      reason: Completed
      startedAt: "2023-10-24T20:28:42Z"
  taskSpec:
    steps:
    - computeResources: {}
      image: alpine
      name: action-runner
```

If a `Step` is referencing a `StepAction`, it cannot contain the fields supported by `StepActions`. This includes:
- `image`
- `command`
- `args`
- `script`
- `env`

Using any of the above fields and referencing a `StepAction` in the same `Step` is not allowed and will cause an validation error.

```yaml
# This is not allowed and will result in a validation error.
# Because the image is expected to be provided by the StepAction
# and not inlined.
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: step-action-run
spec:
  TaskSpec:
    steps:
      - name: action-runner
        ref:
          name: step-action
        image: ubuntu
```
Executing the above `TaskRun` will result in an error that looks like:

```
Error from server (BadRequest): error when creating "STDIN": admission webhook "validation.webhook.pipeline.tekton.dev" denied the request: validation failed: image cannot be used with Ref: spec.taskSpec.steps[0].image
```

When a `Step` is referencing a `StepAction`, it can contain the following fields:
- `computeResources`
- `workspaces` (Isolated workspaces)
- `volumeDevices`
- `imagePullPolicy`
- `onError`
- `stdoutConfig`
- `stderrConfig`
- `securityContext`
- `envFrom`
- `timeout`
- `ref`
- `params`

Using any of the above fields and referencing a `StepAction` is allowed and will not cause an error. For example, the `TaskRun` below will execute without any errors:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: step-action-run
spec:
  TaskSpec:
    steps:
      - name: action-runner
        ref:
          name: step-action
        params:
          - name: step-action-param
            value: hello
        computeResources:
          requests:
            memory: 1Gi
            cpu: 500m
        timeout: 1h
        onError: continue
```

### Passing Params to StepAction

A `StepAction` may require [params](#(declaring-parameters)). In this case, a `Task` needs to ensure that the `StepAction` has access to all the required `params`.
When referencing a `StepAction`, a `Step` can also provide it with `params`, just like how a `TaskRun` provides params to the underlying `Task`.

```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: step-action
spec:
  TaskSpec:
    params:
      - name: param-for-step-action
        description: "this is a param that the step action needs."
    steps:
      - name: action-runner
        ref:
          name: step-action
        params:
          - name: step-action-param
            value: $(params.param-for-step-action)
```

**Note:** If a `Step` declares `params` for an `inlined Step`, it will also lead to a validation error. This is because an `inlined Step` gets it's `params` from the `TaskRun`.

### Specifying Remote StepActions

A `ref` field may specify a `StepAction` in a remote location such as git.
Support for specific types of remote will depend on the `Resolvers` your
cluster's operator has installed. For more information including a tutorial, please check [resolution docs](resolution.md). The below example demonstrates referencing a `StepAction` in git:

```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  generateName: step-action-run-
spec:
  TaskSpec:
    steps:
      - name: action-runner
        ref:
          resolver: git
          params:
            - name: url
              value: https://github.com/repo/repo.git
            - name: revision
              value: main
            - name: pathInRepo
              value: remote_step.yaml
```
