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
        computeResources:
          requests:
            memory: 1Gi
            cpu: 500m
        timeout: 1h
        onError: continue
```
