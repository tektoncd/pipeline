# Workspaces

- [Workspaces](#workspaces)
  - [Summary](#summary)
    - [Use Cases](#use-cases)
    - [Workspaces in Tasks and TaskRuns](#workspaces-in-tasks-and-taskruns)
    - [Workspaces in Pipelines and PipelineRuns](#workspaces-in-pipelines-and-pipelineruns)
  - [Configuring Workspaces](#configuring-workspaces)
    - [Declaring Workspaces in Tasks](#declaring-workspaces-in-tasks)
      - [Workspace Variables](#workspace-variables)
        - [Path](#path)
        - [Volume Name](#volume-name)
      - [Example Task With Workspace](#example-task-with-workspace)
    - [Providing Workspaces with TaskRuns](#providing-workspaces-with-taskruns)
      - [Example TaskRun Specs With Workspaces](#example-taskrun-specs-with-workspaces)
    - [Declaring Workspaces in Pipelines](#declaring-workspaces-in-pipelines)
      - [Ordering Workspaces in a Pipeline](#ordering-workspaces-in-a-pipeline)
    - [Workspaces in PipelineRuns](#workspaces-in-pipelineruns)
      - [Example PipelineRun Specs With Workspaces](#example-pipelinerun-specs-with-workspaces)
    - [VolumeSources for Workspaces](#volumesources-for-workspaces)
  - [Examples](#examples)

## Summary

Workspaces allow Tasks to declare parts of the filesystem that need to be provided
at runtime by TaskRuns. A TaskRun can supply these parts of the filesystem
however it needs to - from a read-only ConfigMap or Secret, a PersistentVolumeClaim
shared with other Tasks, or simply an EmptyDir that is discarded at the end of
the TaskRun.

Workspaces are similar in intent to Volumes but they afford a Task author the opportunity
to defer to users and their TaskRuns when deciding which class of storage to use.

### Use Cases

Workspaces can serve many purposes:

- a place to get code from or fetch code onto
- a mount point for credentials held in Secrets
- a mount point for configuration held in ConfigMaps
- storage for data that a Task will output
- a way to share data from one Task to another
- a location to mount common tools shared by an organization
- a cache of build artifacts that speed up jobs

### Workspaces in Tasks and TaskRuns

Tasks declare where a Workspace will appear on disk for its Steps. At
runtime a TaskRun provides the specific details of the Volume that will
be mounted into that Workspace.

This separation of concerns allows for a lot of flexibility - in isolation
a single TaskRun might simply provide an `emptyDir` volume that mounts quickly
and disappears at the end of the run. In a more complex system a TaskRun
might use a `PersistentVolumeClaim` which is pre-populated with
data for the Task to process. In both of these situations the Task's
Workspace declaration has remained the same, it is only the runtime
information that has changed in the TaskRun.

### Workspaces in Pipelines and PipelineRuns

A Pipeline can use Workspaces to show how storage will be shared through
its Tasks: Task A might clone a source repository onto a given Workspace
and Task B might compile the code that it finds in a given Workspace. It's
the Pipeline's job to ensure that the Workspace these two Tasks use is the
same and, importantly, that the order they access the Workspace in is
correct.

PipelineRuns perform much the same duties as TaskRuns - they provide the
concrete Volume information to use for a Pipeline's Workspaces. PipelineRuns
have the added responsibility of ensuring that whatever Volume type they
provide can be safely and correctly shared across multiple Tasks.

## Configuring Workspaces

### Declaring Workspaces in Tasks

A Task declares a `workspaces` list as part of its spec. A Task can declare as
many workspaces as it needs. Every workspace must have a unique name. Each entry
in `workspaces` can contain the following fields:

- `name` - **Required** A string identifier that can be used to refer to the workspace
- `description` - A human-readable string describing what the workspace is for
- `readOnly` - A boolean declaring whether the `Task` will write to the `workspace`.
- `mountPath` - A location on disk where the workspace will appear in Steps. Relative
  paths will be prepended with `/workspace`. If a `mountPath` is not provided the workspace
  will be placed by default at `/workspace/<name>` where `<name>` is the workspace's
  unique name.

> Note: A `readOnly` workspace will have its volume mounted as read-only. Attempting to write
> to a `readOnly` workspace is likely to result in errors and failed `TaskRun`s.

> Note: `mountPath` can be either absolute or relative. Absolute paths start with `/` and
> relative paths start with the name of a directory.
> Example:
> - A `mountPath` of `"/foobar"` is  absolute and exposes the workspace at `/foobar` inside
> the Task's Steps.
> - A `mountPath` of `"foobar"` is  relative and exposes the workspace at `/workspace/foobar`.

#### Workspace Variables

##### Path

The path to a workspace is available as a variable to Tasks with
`$(workspaces.<name>.path)` where `<name>` is the workspace's name.

##### Volume Name

The name of the volume provided for a workspace is available as a variable
to Tasks with `$(workspaces.<name>.volume)` where `<name>` is the workspace's
name.

#### Example Task With Workspace

Here, a Task declares a `messages` workspace that it writes a message on to.

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

### Providing Workspaces with TaskRuns

For a `TaskRun` to execute a `Task` that declares `workspaces` it needs to bind
those `workspaces` to actual physical volumes. To do so the `TaskRun` includes
its own `workspaces` list. Each entry in the list can contain the following fields:

- `name` - **Required** The `Task`'s workspace name that a Volume is being provided for
- `subPath` - An optional subdirectory on the volume to use as the `Workspace`

> The `subPath` *must* exist on the volume before the `TaskRun` starts or it will fail
> with an error.

The entry must also include one `VolumeSource`. See the section elsewhere in this doc
on [configuring a `VolumeSource`](#volumesources-for-workspaces) in your `TaskRun`s.

> If a `Task`'s declared `Workspace`s are not provided at runtime by a `TaskRun`
> then that `TaskRun` will fail with an error.

#### Example TaskRun Specs With Workspaces

The examples below show the relevant fields of TaskRun specs when working with
Workspaces. For a more complete end-to-end example, see [the Workspaces TaskRun](../examples/v1beta1/taskruns/workspace.yaml)
in the examples directory.

Here an existing PersistentVolumeClaim called `mypvc` is used for a Task's `workspace`
called `myworkspace`, exposing only the subdirectory `my-subdir` from that PVC:

```yaml
workspaces:
- name: myworkspace
  persistentVolumeClaim:
    claimName: mypvc
  subPath: my-subdir
```

In this example an [`emptyDir`](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir)
is provided for a Task's `workspace` called `myworkspace`:

```yaml
workspaces:
- name: myworkspace
  emptyDir: {}
```

Here a `ConfigMap` named `my-configmap` is used for a `Task`'s `Workspace`
named `myworkspace`:

```yaml
workspaces:
- name: myworkspace
  configmap:
    name: my-configmap
```

In this example a `Secret` named `my-secret` is used for a `Task`'s `Workspace`
named `myworkspace`:

```yaml
workspaces:
- name: myworkspace
  secret:
    secretName: my-secret
```

_For a complete end-to-end example see [workspace.yaml](../examples/v1beta1/taskruns/workspace.yaml)._

### Declaring Workspaces in Pipelines

Individual `Task`s declare the `Workspace`s they need to run. It's the
role of a `Pipeline` to decide which `Workspace`s should be shared amongst
its `Task`s. Doing so requires two additions in a `Pipeline`:

1. The `Pipeline` will need to declare a list of `workspaces` that
`PipelineRun`s will be expected to provide. This is done with the `workspaces`
field in the `Pipeline`'s spec. Each entry in that list must have a unique
name.
2. When a `Pipeline` gives a `Workspace` to a `Task` it needs to map
from the name given to it by the `Pipeline` to the name expected by
the `Task`.

Here's what the relevant portions of a Pipeline spec look like with `workspaces`:

```yaml
spec:
  workspaces:
    - name: pipeline-ws1 # The name of the workspace in the Pipeline
  tasks:
    - name: use-ws-from-pipeline
      taskRef:
        name: gen-code # gen-code expects a workspace with name "output"
      workspaces:
        - name: output
          workspace: pipeline-ws1
    - name: use-ws-again
      taskRef:
        name: commit # commit expects a workspace with name "src"
      workspaces:
        - name: src
          workspace: pipeline-ws1
      runAfter:
        - use-ws-from-pipeline # important: use-ws-from-pipeline writes to the workspace first
```

This spec describes a Pipeline with a single workspace, `pipeline-ws1`, that
will be bound in two `Task`s, first as the `output` workspace declared by the `gen-code`
`Task`, then as the `src` workspace declared by the `commit` `Task`. If the `Workspace`
provided by the `PipelineRun` is a `PersistentVolumeClaim` then these two Tasks will share
the files in that `Workspace`.

#### Ordering Workspaces in a Pipeline

Wiring a `Workspace` between `Task`s requires you to set the order that each
`Task` will access that `Workspace` in. This is important because different classes
of storage have different rules attached to them about how many readers and writes
can be using them in tandem. One class of `PersistentVolumeClaim` might, for example,
only be able to operate correctly with a single Task writing to it at a time.

> Warning: it's very important to ensure that the order of Tasks is correct for the
> storage type you use. In the worst case incorrectly configured ordering can result
> in deadlock behavior where multiple `Task` `Pod`s are all attempting to mount a
> PVC for writing at the same time. Some or all of the `TaskRun`s would eventually
> time out.

`Pipeline` authors should explicitly declare the ordering of `Task`s sharing a
PVC-backed workspace by using a `Pipeline`'s `runAfter` field. See
[the section on `runAfter`](pipelines.md#runAfter) for more information about
using this field.

### Workspaces in PipelineRuns

For a `PipelineRun` to execute
[a Pipeline that declares `workspaces`](pipelines.md#workspaces), it needs to
bind the `workspace` names to actual physical volumes. This is managed through
a `PipelineRun`'s `workspaces` list. Each entry in this list must correspond
to a declaration in the `Pipeline`. Here are the fields that are supported in
a `PipelineRun`'s `workspaces` entries:

- `name` - **Required** The `Pipeline`'s workspace name that a Volume is being provided for
- `subPath` - An optional subdirectory on the volume to use as the `Workspace`

> The `subPath` *must* exist on the volume before the `TaskRun` starts or it will fail
> with an error.

The entry must also include one `VolumeSource`. See the section elsewhere in this doc
on [configuring a `VolumeSource`](#volumesources-for-workspaces) in your `PipelineRun`s.

> If a `Pipeline`'s declared `Workspace`s are not provided at runtime by a `PipelineRun`
> then that `PipelineRun` will fail with an error.

#### Example PipelineRun Specs With Workspaces

The examples below show the relevant fields of `PipelineRun` specs when working with
`Workspaces`. For a more complete end-to-end example, see [the Workspaces PipelineRun](../examples/v1beta1/pipelineruns/workspaces.yaml)
in the examples directory.

Here an existing PersistentVolumeClaim called `mypvc` is used for a Pipeline's `workspace`
called `myworkspace`, exposing only the subdirectory `my-subdir` from that PVC:

```yaml
workspaces:
- name: myworkspace
  persistentVolumeClaim:
    claimName: mypvc
  subPath: my-subdir
```

Here a `ConfigMap` named `my-configmap` is used for a `Pipeline`'s `Workspace`
named `myworkspace`:

```yaml
workspaces:
- name: myworkspace
  configmap:
    name: my-configmap
```

In this example a `Secret` named `my-secret` is used for a `Pipeline`'s `Workspace`
named `myworkspace`:

```yaml
workspaces:
- name: myworkspace
  secret:
    secretName: my-secret
```

### VolumeSources for Workspaces

Only one type of `VolumeSource` can be used per Workspace entry and the configuration
options differ for each type. These are the field names for the supported types as well
as links to the Kubernetes documentation for configuration options of each:

- `emptyDir` - a temporary directory that only lives as long as the TaskRun.
  - **Note: `EmptyDir` volumes are not suitable for sharing data between `Task`s in a `Pipeline`.**
  - [Kubernetes docs for `emptyDir` volumes](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir)
- `persistentVolumeClaim` - a reference to a `PersistentVolumeClaim`
  - `PersistentVolumeClaim`s are a good choice for sharing data between `Task`s in a `Pipeline`.
  - [Kubernetes docs for `persistentVolumeClaim` volumes](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim)
- `configMap` - a reference to a `ConfigMap`
  - Using a `ConfigMap` as a `Workspace` has the following caveats:
    1. `ConfigMap` volume sources are always mounted as read-only. `Step`s
    cannot write content to them and may error out if a write is attempted.
    2. The `ConfigMap` you want to use as a `Workspace` must exist prior to
    the `TaskRun` being submitted.
    3. `ConfigMap`s have a [size limit of 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L5042).
  - [Kubernetes docs for `configMap` volumes](https://kubernetes.io/docs/concepts/storage/volumes/#configmap)
- `secret` - a reference to a `Secret`
  - Using a `Secret` as a `Workspace` has the following caveats:
    1. `Secret` volume sources are always mounted as read-only. `Step`s
    cannot write content to them and may error out if a write is attempted.
    2. The `Secret` you want to use as a `Workspace` must exist prior to
    the `TaskRun` being submitted.
    3. `Secret`s have a [size limit of 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L5042).
  - [Kubernetes docs for `secret` volumes](https://kubernetes.io/docs/concepts/storage/volumes/#secret)

> If you need support for a `VolumeSource` not listed here
> [please open an issue](https://github.com/tektoncd/pipeline/issues) or feel free to
> [contribute a PR](https://github.com/tektoncd/pipeline/blob/master/CONTRIBUTING.md).

## Examples

- [TaskRun example](../examples/v1beta1/taskruns/workspace.yaml)
- [PipelineRun example](../examples/v1beta1/pipelineruns/workspaces.yaml)
