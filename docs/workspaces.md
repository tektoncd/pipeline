<!--
---
linkTitle: "Workspaces"
weight: 5
---
-->
# Workspaces

- [Overview](#overview)
  - [`Workspaces` in `Tasks` and `TaskRuns`](#workspaces-in-tasks-and-taskruns)
  - [`Workspaces` in `Pipelines` and `PipelineRuns`](#workspaces-in-pipelines-and-pipelineruns)
- [Configuring `Workspaces`](#configuring-workspaces)
  - [Using `Workspaces` in `Tasks`](#using-workspaces-in-tasks)
    - [Using `Workspace` variables in `TaskRuns`](#using-workspace-variables-in-taskruns)
    - [Mapping `Workspaces` in `Tasks` to `TaskRuns`](#mapping-workspaces-in-tasks-to-taskruns)
    - [Examples of `TaskRun` definitions using `Workspaces`](#examples-of-taskrun-definitions-using-workspaces)
  - [Using `Workspaces` in `Pipelines`](#using-workspaces-in-pipelines)
    - [Specifying `Workspace` order in a `Pipeline`](#specifying-workspace-order-in-a-pipeline)
    - [Specifying `Workspaces` in `PipelineRuns`](#specifying-workspaces-in-pipelineruns)
    - [Example `PipelineRun` definitions using `Workspaces`](#example-pipelinerun-definitions-using-workspaces)
  - [Specifying `VolumeSources` in `Workspaces`](#specifying-volumesources-in-workspaces)
- [More examples](#more-examples)

## Overview

`Workspaces` allow `Tasks` to declare parts of the filesystem that need to be provided
at runtime by `TaskRuns`. A `TaskRun` can make these parts of the filesystem available
in many ways: using a read-only `ConfigMap` or `Secret`, an existing `PersistentVolumeClaim`
shared with other Tasks, create a `PersistentVolumeClaim` from a provided `VolumeClaimTemplate`, or simply an `emptyDir` that is discarded when the `TaskRun`
completes.

`Workspaces` are similar to `Volumes` except that they allow a `Task` author 
to defer to users and their `TaskRuns` when deciding which class of storage to use.

Workspaces can serve the following purposes:

- Storage of inputs and/or outputs
- Sharing data among `Tasks`
- A mount point for credentials held in `Secrets`
- A mount point for configurations held in `ConfigMaps`
- A mount point for common tools shared by an organization
- A cache of build artifacts that speed up jobs

### Workspaces in `Tasks` and `TaskRuns`

`Tasks` specify where a `Workspace` resides on disk for its `Steps`. At
runtime, a `TaskRun` provides the specific details of the `Volume` that is
mounted into that `Workspace`.

This separation of concerns allows for a lot of flexibility. For example, in isolation,
a single `TaskRun` might simply provide an `emptyDir` volume that mounts quickly
and disappears at the end of the run. In a more complex system, however, a `TaskRun`
might use a `PersistentVolumeClaim` which is pre-populated with
data for the `Task` to process. In both scenarios the `Task's`
`Workspace` declaration remains the same and only the runtime
information in the `TaskRun` changes.

### `Workspaces` in `Pipelines` and `PipelineRuns`

A `Pipeline` can use `Workspaces` to show how storage will be shared through
its `Tasks`. For example, `Task` A might clone a source repository onto a `Workspace`
and `Task` B might compile the code that it finds in that `Workspace`. It's
the `Pipeline's` job to ensure that the `Workspace` these two `Tasks` use is the
same, and more importantly, that the order in which they access the `Workspace` is
correct.

`PipelineRuns` perform mostly the same duties as `TaskRuns` - they provide the
specific `Volume` information to use for the `Workspaces` used by each `Pipeline`.
`PipelineRuns` have the added responsibility of ensuring that whatever `Volume` type they
provide can be safely and correctly shared across multiple `Tasks`.

## Configuring `Workspaces`

This section describes how to configure one or more `Workspaces` in a `TaskRun`.

### Using `Workspaces` in `Tasks`

To configure one or more `Workspaces` in a `Task`, add a `workspaces` list with each entry using the following fields:

- `name` -  (**required**) A **unique** string identifier that can be used to refer to the workspace
- `description` - An informative string describing the purpose of the `Workspace`
- `readOnly` - A boolean declaring whether the `Task` will write to the `Workspace`.
- `mountPath` - A path to a location on disk where the workspace will be available to `Steps`. Relative
  paths will be prepended with `/workspace`. If a `mountPath` is not provided the workspace
  will be placed by default at `/workspace/<name>` where `<name>` is the workspace's
  unique name.
  
Note the following:
  
- A `Task` definition can include as many `Workspaces` as it needs. 
- A `readOnly` `Workspace` will have its volume mounted as read-only. Attempting to write
  to a `readOnly` `Workspace` will result in errors and failed `TaskRuns`.
- `mountPath` can be either absolute or relative. Absolute paths start with `/` and relative paths
  start with the name of a directory. For example, a `mountPath` of `"/foobar"` is  absolute and exposes
  the `Workspace` at `/foobar` inside the `Task's` `Steps`, but a `mountPath` of `"foobar"` is relative and
  exposes the `Workspace` at `/workspace/foobar`.
    
Below is an example `Task` definition that includes a `Workspace` called `messages` to which the `Task` writes a message:

```yaml
spec:
  steps:
  - name: write-message
    image: ubuntu
    script: |
      #!/usr/bin/env bash
      set -xe
      echo hello! > $(workspaces.messages.path)/message
  workspaces:
  - name: messages
    description: The folder where we write the message to
    mountPath: /custom/path/relative/to/root
```

#### Using `Workspace` variables in `Tasks`

The following variables make information about `Workspaces` available to `Tasks`:

- `$(workspaces.<name>.path)` - specifies the path to a `Workspace`
   where `<name>` is the name of the `Workspace`.
- `$(workspaces.<name>.volume)`- specifies the name of the `Volume`
   provided for a `Workspace` where `<name>` is the name of the `Workspace`.

#### Mapping `Workspaces` in `Tasks` to `TaskRuns`

A `TaskRun` that executes a `Task` containing a `workspaces` list must bind
those `workspaces` to actual physical `Volumes`. To do so, the `TaskRun` includes
its own `workspaces` list. Each entry in the list contains the following fields:

- `name` - (**required**) The name of the `Workspace` within the `Task` for which the `Volume` is being provided
- `subPath` - An optional subdirectory on the `Volume` to store data for that `Workspace`

The entry must also include one `VolumeSource`. See [Using `VolumeSources` with `Workspaces`](#specifying-volumesources-in-workspaces) for more information.
               
**Caution:**
- The `subPath` *must* exist on the `Volume` before the `TaskRun` executes or the execution will fail.
- The `Workspaces` declared in a `Task` must be available when executing the associated `TaskRun`.
  Otherwise, the `TaskRun` will fail.

#### Examples of `TaskRun` definitions using `Workspaces`

The following examples illustrate how to specify `Workspaces` in your `TaskRun` definition.
For a more in-depth example, see [`Workspaces` in a `TaskRun`](../examples/v1beta1/taskruns/workspace.yaml).

In the example below, an existing `PersistentVolumeClaim` called `mypvc` is used for a Task's `workspace`
called `myworkspace`. It exposes only the subdirectory `my-subdir` from that `PersistentVolumeClaim`:

```yaml
workspaces:
- name: myworkspace
  persistentVolumeClaim:
    claimName: mypvc
  subPath: my-subdir
```

In the example below, an [`emptyDir`](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir)
is provided for a Task's `workspace` called `myworkspace`:

```yaml
workspaces:
- name: myworkspace
  emptyDir: {}
```

In the example below, a `ConfigMap` named `my-configmap` is used for a `Workspace` 
named `myworkspace` declared inside a `Task`:

```yaml
workspaces:
- name: myworkspace
  configmap:
    name: my-configmap
```

In this example, a `Secret` named `my-secret` is used for a `Workspace` 
named `myworkspace` declared inside a `Task`:

```yaml
workspaces:
- name: myworkspace
  secret:
    secretName: my-secret
```

For a more in-depth example, see [workspace.yaml](../examples/v1beta1/taskruns/workspace.yaml).

### Using `Workspaces` in `Pipelines`

While individual `Tasks` declare the `Workspaces` they need to run, the `Pipeline` decides
which `Workspaces` are shared among its `Tasks`. To declare shared `Workspaces` in a `Pipeline`,
you must add the following information to your `Pipeline` definition:

- A list of `Workspaces` that your `PipelineRuns` will be providing. Use the `workspaces` field to
  specify the target `Workspaces` in your `Pipeline` definition as shown below. Each entry in the
  list must have a unique name.
- A mapping of `Workspace` names between the `Pipeline` and the `Task` definitions.

The example below defines a `Pipeline` with a single `Workspace` named `pipeline-ws1`. This
`Workspace` is bound in two `Tasks` - first as the `output` workspace declared by the `gen-code`
`Task`, then as the `src` workspace declared by the `commit` `Task`. If the `Workspace`
provided by the `PipelineRun` is a `PersistentVolumeClaim` then these two `Tasks` can share
data within that `Workspace`.

```yaml
spec:
  workspaces:
    - name: pipeline-ws1 # Name of the workspace in the Pipeline
  tasks:
    - name: use-ws-from-pipeline
      taskRef:
        name: gen-code # gen-code expects a workspace named "output"
      workspaces:
        - name: output
          workspace: pipeline-ws1
    - name: use-ws-again
      taskRef:
        name: commit # commit expects a workspace named "src"
      workspaces:
        - name: src
          workspace: pipeline-ws1
      runAfter:
        - use-ws-from-pipeline # important: use-ws-from-pipeline writes to the workspace first
```

#### Specifying `Workspace` order in a `Pipeline`

Sharing a `Workspace` between `Tasks` requires you to define the order in which those `Tasks`
will be accessing that `Workspace` since different classes of storage have different limits
for concurrent reads and writes. For example, a `PersistentVolumeClaim` might only allow a
single `Task` writing to it at once.

**Warning:** You *must* ensure that this order is correct. Incorrectly ordering can result
in a deadlock where multiple `Task` `Pods` are attempting to mount a `PersistentVolumeClaim`
for writing at the same time, which would cause the `Tasks` to time out.

To define this order, use the `runAfter` field in your `Pipeline` definition. For more
information, see the [`runAfter` documentation](pipelines.md#runAfter).

#### Specifying `Workspaces` in `PipelineRuns`

For a `PipelineRun` to execute a `Pipeline` that includes one or more `Workspaces`, it needs to
bind the `Workspace` names to physical volumes using its own `workspaces` field. Each entry in
this list must correspond to a `Workspace` declaration in the `Pipeline`. Each entry in the
`workspaces` list must specify the following:

- `name` - (**required**) the name of the `Workspace` specified in the `Pipeline` definition for which a volume is being provided.
- `subPath` - (optional) a directory on the volume that will store that `Workspace's` data. This directory must exist at the
  time the `TaskRun` executes, otherwise the execution will fail.

The entry must also include one `VolumeSource`. See [Using `VolumeSources` with `Workspaces`](#specifying-volumesources-in-workspaces) for more information.

**Note:** If the `Workspaces` specified by a `Pipeline` are not provided at runtime by a `PipelineRun`, that `PipelineRun` will fail.

#### Example `PipelineRun` definitions using `Workspaces`

The examples below illustrate how to specify `Workspaces` in your `PipelineRuns`. For a more in-depth example, see the
[`Workspaces` in `PipelineRun`](../examples/v1beta1/pipelineruns/workspaces.yaml) YAML sample.

In the example below, an existing `PersistentVolumeClaim` named `mypvc` is used for a `Workspace`
named `myworkspace` declared in a `Pipeline`. It exposes only the subdirectory `my-subdir` from that `PersistentVolumeClaim`: 

```yaml
workspaces:
- name: myworkspace
  persistentVolumeClaim:
    claimName: mypvc
  subPath: my-subdir
```

In the example below, a `ConfigMap` named `my-configmap` is used for a  `Workspace`
named `myworkspace` declared in a `Pipeline`:

```yaml
workspaces:
- name: myworkspace
  configmap:
    name: my-configmap
```

In the example below, a `Secret` named `my-secret` is used for a `Workspace`
named `myworkspace` declared in a `Pipeline`:

```yaml
workspaces:
- name: myworkspace
  secret:
    secretName: my-secret
```

### Specifying `VolumeSources` in `Workspaces`

You can only use a single type of `VolumeSource` per `Workspace` entry. The configuration
options differ for each type. `Workspaces` support the following fields:

#### `emptyDir`

The `emptyDir` field references an [`emptyDir` volume](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) which holds
a temporary directory that only lives as long as the `TaskRun` that invokes it. `emptyDir` volumes are **not** suitable for sharing data among `Tasks` within a `Pipeline`.
However, they work well for single `TaskRuns` where the data stored in the `emptyDir` needs to be shared among the `Steps` of the `Task` and discarded after execution.

#### `persistentVolumeClaim`

The `persistentVolumeClaim` field references an existing [`persistentVolumeClaim` volume](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim).
`PersistentVolumeClaim` volumes are a good choice for sharing data among `Tasks` within a `Pipeline`.

#### `volumeClaimTemplate`

The `volumeClaimTemplate` is a template of a [`persistentVolumeClaim` volume](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim), created for each `PipelineRun` or `TaskRun`. 
When the volume is created from a template in a `PipelineRun` or `TaskRun` it will be deleted when the `PipelineRun` or `TaskRun` is deleted.
`volumeClaimTemplate` volumes are a good choice for sharing data among `Tasks` within a `Pipeline` when the volume is only used during a `PipelineRun` or `TaskRun`.

#### `configMap`

The `configMap` field references a [`configMap` volume](https://kubernetes.io/docs/concepts/storage/volumes/#configmap).
Using a `configMap` as a `Workspace` has the following limitations:

- `configMap` volume sources are always mounted as read-only. `Steps` cannot write to them and will error out if they try.
- The `configMap` you want to use as a `Workspace` must exist prior to submitting the `TaskRun`.
- `configMaps` are [size-limited to 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L5042).

#### `secret`

The `secret` field references a [`secret` volume](https://kubernetes.io/docs/concepts/storage/volumes/#secret).
Using a `secret` volume has the following limitations:

- `secret` volume sources are always mounted as read-only. `Steps` cannot write to them and will error out if they try.
- The `secret` you want to use as a `Workspace` must exist prior to submitting the `TaskRun`.
- `secret` are [size-limited to 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L5042).

If you need support for a `VolumeSource` type not listed above, [open an issue](https://github.com/tektoncd/pipeline/issues) or
a [pull request](https://github.com/tektoncd/pipeline/blob/master/CONTRIBUTING.md).

## More examples

See the following in-depth examples of configuring `Workspaces`:

- [`Workspaces` in a `TaskRun`](../examples/v1beta1/taskruns/workspace.yaml)
- [`Workspaces` in a `PipelineRun`](../examples/v1beta1/pipelineruns/workspaces.yaml)
