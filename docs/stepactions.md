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
  args:: ["-lh"]
```
