<!--
---
linkTitle: "Workspaces"
weight: 405
---
-->

# Workspaces

- [Overview](#overview)
  - [`Workspaces` in `Tasks` and `TaskRuns`](#workspaces-in-tasks-and-taskruns)
  - [`Workspaces` in `Pipelines` and `PipelineRuns`](#workspaces-in-pipelines-and-pipelineruns)
  - [Optional `Workspaces`](#optional-workspaces)
  - [Isolated `Workspaces`](#isolated-workspaces)
- [Configuring `Workspaces`](#configuring-workspaces)
  - [Using `Workspaces` in `Tasks`](#using-workspaces-in-tasks)
    - [Isolating `Workspaces` to Specific `Steps` or `Sidecars`](#isolating-workspaces-to-specific-steps-or-sidecars)
    - [Setting a default `TaskRun` `Workspace Binding`](#setting-a-default-taskrun-workspace-binding)
    - [Using `Workspace` variables in `Tasks`](#using-workspace-variables-in-tasks)
    - [Mapping `Workspaces` in `Tasks` to `TaskRuns`](#mapping-workspaces-in-tasks-to-taskruns)
    - [Examples of `TaskRun` definition using `Workspaces`](#examples-of-taskrun-definition-using-workspaces)
  - [Using `Workspaces` in `Pipelines`](#using-workspaces-in-pipelines)
    - [Specifying `Workspace` order in a `Pipeline` and Affinity Assistants](#specifying-workspace-order-in-a-pipeline-and-affinity-assistants)
    - [Specifying `Workspaces` in `PipelineRuns`](#specifying-workspaces-in-pipelineruns)
    - [Example `PipelineRun` definition using `Workspaces`](#example-pipelinerun-definition-using-workspaces)
  - [Specifying `VolumeSources` in `Workspaces`](#specifying-volumesources-in-workspaces)
    - [Using `PersistentVolumeClaims` as `VolumeSource`](#using-persistentvolumeclaims-as-volumesource)
    - [Using other types of `VolumeSources`](#using-other-types-of-volumesources)
- [Using Persistent Volumes within a `PipelineRun`](#using-persistent-volumes-within-a-pipelinerun)
- [More examples](#more-examples)

## Overview

`Workspaces` allow `Tasks` to declare parts of the filesystem that need to be provided
at runtime by `TaskRuns`. A `TaskRun` can make these parts of the filesystem available
in many ways: using a read-only `ConfigMap` or `Secret`, an existing `PersistentVolumeClaim`
shared with other Tasks, create a `PersistentVolumeClaim` from a provided `VolumeClaimTemplate`, or simply an `emptyDir` that is discarded when the `TaskRun`
completes.

`Workspaces` are similar to `Volumes` except that they allow a `Task` author 
to defer to users and their `TaskRuns` when deciding which class of storage to use.

`Workspaces` can serve the following purposes:

- Storage of inputs and/or outputs
- Sharing data among `Tasks`
- A mount point for credentials held in `Secrets`
- A mount point for configurations held in `ConfigMaps`
- A mount point for common tools shared by an organization
- A cache of build artifacts that speed up jobs

### `Workspaces` in `Tasks` and `TaskRuns`

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

`Tasks` can also share `Workspaces` with their `Sidecars`, though there's a little more
configuration involved to add the required `volumeMount`. This allows for a
long-running process in a `Sidecar` to share data with the executing `Steps` of a `Task`.

**Note**: If the `enable-api-fields` feature-flag is set to `"alpha"` then workspaces
will automatically be available to `Sidecars` too!

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

### Optional `Workspaces`

Both `Tasks` and `Pipelines` can declare a `Workspace` "optional". When an optional `Workspace`
is declared the `TaskRun` or `PipelineRun` may omit a `Workspace` Binding for that `Workspace`.
The `Task` or `Pipeline` behaviour may change when the Binding is omitted. This feature has
many uses:

- A `Task` may optionally accept credentials to run authenticated commands.
- A `Pipeline` may accept optional configuration that changes the linting or compilation
parameters used.
- An optional build cache may be provided to speed up compile times.

See the section [Using `Workspaces` in `Tasks`](#using-workspaces-in-tasks) for more info on
the `optional` field.

### Isolated `Workspaces`

This is an alpha feature. The `enable-api-fields` feature flag [must be set to `"alpha"`](./install.md)
for Isolated Workspaces to function.

Certain kinds of data are more sensitive than others. To reduce exposure of sensitive data Task
authors can isolate `Workspaces` to only those `Steps` and `Sidecars` that require access to
them. The primary use-case for this is credentials but it can apply to any data that should have
its access strictly limited to only specific container images.

See the section [Isolating `Workspaces` to Specific `Steps` or `Sidecars`](#isolating-workspaces-to-specific-steps-or-sidecars)
for more info on this feature.

## Configuring `Workspaces`

This section describes how to configure one or more `Workspaces` in a `TaskRun`.

### Using `Workspaces` in `Tasks`

To configure one or more `Workspaces` in a `Task`, add a `workspaces` list with each entry using the following fields:

- `name` -  (**required**) A **unique** string identifier that can be used to refer to the workspace
- `description` - An informative string describing the purpose of the `Workspace`
- `readOnly` - A boolean declaring whether the `Task` will write to the `Workspace`. Defaults to `false`.
- `optional` - A boolean indicating whether a TaskRun can omit the `Workspace`. Defaults to `false`.
- `mountPath` - A path to a location on disk where the workspace will be available to `Steps`. If a
  `mountPath` is not provided the workspace will be placed by default at `/workspace/<name>` where `<name>`
  is the workspace's unique name.

Note the following:

- A `Task` definition can include as many `Workspaces` as it needs. It is recommended that `Tasks` use
  **at most** one _writeable_ `Workspace`.
- A `readOnly` `Workspace` will have its volume mounted as read-only. Attempting to write
  to a `readOnly` `Workspace` will result in errors and failed `TaskRuns`.

Below is an example `Task` definition that includes a `Workspace` called `messages` to which the `Task` writes a message:

```yaml
spec:
  steps:
    - name: write-message
      image: ubuntu
      script: |
        #!/usr/bin/env bash
        set -xe
        if [ "$(workspaces.messages.bound)" == "true" ] ; then
          echo hello! > $(workspaces.messages.path)/message
        fi
  workspaces:
    - name: messages
      description: |
        The folder where we write the message to. If no workspace
        is provided then the message will not be written.
      optional: true
      mountPath: /custom/path/relative/to/root
```

#### Sharing `Workspaces` with `Sidecars`

A `Task's` `Sidecars` are also able to access the `Workspaces` the `Task` defines but must have their
`volumeMount` configuration set explicitly. Below is an example `Task` that shares a `Workspace` between
its `Steps` and its `Sidecar`. In the example a `Sidecar` sleeps for a short amount of time and then writes
a `ready` file which the `Step` is waiting for:

```yaml
spec:
  workspaces:
    - name: signals
  steps:
    - image: alpine
      script: |
        while [ ! -f "$(workspaces.signals.path)/ready" ]; do
          echo "Waiting for ready file..."
          sleep 1
        done
        echo "Saw ready file!"
  sidecars:
    - image: alpine
      # Note: must explicitly include volumeMount for the workspace to be accessible in the Sidecar
      volumeMounts:
        - name: $(workspaces.signals.volume)
          mountPath: $(workspaces.signals.path)
      script: |
        sleep 3
        touch "$(workspaces.signals.path)/ready"
```

**Note:** Starting in Pipelines v0.24.0 `Sidecars` automatically get access to `Workspaces`.This is an
alpha feature and requires Pipelines to have [the "alpha" feature gate enabled](./install.md#alpha-features).

If a Sidecar already has a `volumeMount` at the location expected for a `workspace` then that `workspace` is
not bound to the Sidecar. This preserves backwards-compatibility with any existing uses of the `volumeMount`
trick described above.

#### Isolating `Workspaces` to Specific `Steps` or `Sidecars`

This is an alpha feature. The `enable-api-fields` feature flag [must be set to `"alpha"`](./install.md)
for Isolated Workspaces to function.

To limit access to a `Workspace` from a subset of a `Task's` `Steps` or `Sidecars` requires
adding a `workspaces` declaration to those sections. In the following example a `Task` has several
`Steps` but only the one that performs a `git clone` will be able to access the SSH credentials
passed into it:

```yaml
spec:
  workspaces:
  - name: ssh-credentials
    description: An .ssh directory with keys, known_host and config files used to clone the repo.
  steps:
  - name: clone-repo
    workspaces:
    - name: ssh-credentials # This Step receives the sensitive workspace; the others do not.
    image: git
    script: # git clone ...
  - name: build-source
    image: third-party-source-builder:latest # This image doesn't get access to ssh-credentials.
  - name: lint-source
    image: third-party-source-linter:latest # This image doesn't get access to ssh-credentials.
```

It can potentially be useful to mount `Workspaces` to different locations on a per-`Step` or
per-`Sidecar` basis and this is also supported:

```yaml
kind: Task
spec:
  workspaces:
  - name: ws
    mountPath: /workspaces/ws
  steps:
  - name: edit-files-1
    workspaces:
    - name: ws
      mountPath: /foo # overrides mountPath
  - name: edit-files-2
    workspaces:
    - name: ws # no mountPath specified so will use /workspaces/ws
  sidecars:
  - name: watch-files-on-workspace
    workspaces:
    - name: ws
      mountPath: /files # overrides mountPath
```

#### Setting a default `TaskRun` `Workspace Binding`

An organization may want to specify default `Workspace` configuration for `TaskRuns`. This allows users to
use `Tasks` without having to know the specifics of `Workspaces` - they can simply rely on the platform
to use the default configuration when a `Workspace` is missing. To support this Tekton allows a default
`Workspace Binding` to be specified for `TaskRuns`. When the `TaskRun` executes, any `Workspaces` that 
a `Task` requires but which are not provided by the `TaskRun` will be bound with the default configuration.

The configuration for the default `Workspace Binding` is added to the `config-defaults` `ConfigMap`, under
the `default-task-run-workspace-binding` key. For an example, see the [Customizing basic execution
parameters](./install.md#customizing-basic-execution-parameters) section of the install doc.

**Note:** the default configuration is used for any _required_ `Workspace` declared by a `Task`. Optional
`Workspaces` are not populated with the default binding. This is because a `Task's` behaviour will typically
differ slightly when an optional `Workspace` is bound.

#### Using `Workspace` variables in `Tasks`

The following variables make information about `Workspaces` available to `Tasks`:

- `$(workspaces.<name>.path)` - specifies the path to a `Workspace`
   where `<name>` is the name of the `Workspace`. This will be an
   empty string when a Workspace is declared optional and not provided
   by a TaskRun.
- `$(workspaces.<name>.bound)` - either `true` or `false`, specifies
   whether a workspace was bound. Always `true` if the workspace is required.
- `$(workspaces.<name>.claim)` - specifies the name of the `PersistentVolumeClaim` used as a volume source for the `Workspace` 
   where `<name>` is the name of the `Workspace`. If a volume source other than `PersistentVolumeClaim` is used, an empty string is returned.
- `$(workspaces.<name>.volume)`- specifies the name of the `Volume`
   provided for a `Workspace` where `<name>` is the name of the `Workspace`.

#### Mapping `Workspaces` in `Tasks` to `TaskRuns`

A `TaskRun` that executes a `Task` containing a `workspaces` list must bind
those `workspaces` to actual physical `Volumes`. To do so, the `TaskRun` includes
its own `workspaces` list. Each entry in the list contains the following fields:

- `name` - (**required**) The name of the `Workspace` within the `Task` for which the `Volume` is being provided
- `subPath` - An optional subdirectory on the `Volume` to store data for that `Workspace`

The entry must also include one `VolumeSource`. See [Specifying `VolumeSources` in `Workspaces`](#specifying-volumesources-in-workspaces) for more information.

**Caution:**
- The `Workspaces` declared in a `Task` must be available when executing the associated `TaskRun`.
  Otherwise, the `TaskRun` will fail.

#### Examples of `TaskRun` definition using `Workspaces`

The following example illustrate how to specify `Workspaces` in your `TaskRun` definition,
an [`emptyDir`](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir)
is provided for a Task's `workspace` called `myworkspace`:

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  generateName: example-taskrun-
spec:
  taskRef:
    name: example-task
  workspaces:
    - name: myworkspace # this workspace name must be declared in the Task
      emptyDir: {}      # emptyDir volumes can be used for TaskRuns, 
                        # but consider using a PersistentVolumeClaim for PipelineRuns
```
For examples of using other types of volume sources, see [Specifying `VolumeSources` in `Workspaces`](#specifying-volumesources-in-workspaces).
For a more in-depth example, see [`Workspaces` in a `TaskRun`](../examples/v1beta1/taskruns/workspace.yaml).

### Using `Workspaces` in `Pipelines`

While individual `Tasks` declare the `Workspaces` they need to run, the `Pipeline` decides
which `Workspaces` are shared among its `Tasks`. To declare shared `Workspaces` in a `Pipeline`,
you must add the following information to your `Pipeline` definition:

- A list of `Workspaces` that your `PipelineRuns` will be providing. Use the `workspaces` field to
  specify the target `Workspaces` in your `Pipeline` definition as shown below. Each entry in the
  list must have a unique name.
- A mapping of `Workspace` names between the `Pipeline` and the `Task` definitions.

The example below defines a `Pipeline` with a `Workspace` named `pipeline-ws1`. This
`Workspace` is bound in two `Tasks` - first as the `output` workspace declared by the `gen-code`
`Task`, then as the `src` workspace declared by the `commit` `Task`. If the `Workspace`
provided by the `PipelineRun` is a `PersistentVolumeClaim` then these two `Tasks` can share
data within that `Workspace`.

```yaml
spec:
  workspaces:
    - name: pipeline-ws1 # Name of the workspace in the Pipeline
    - name: pipeline-ws2
      optional: true
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

Include a `subPath` in the `Workspace Binding` to mount different parts of the same volume for different Tasks. See [a full example of this kind of Pipeline](../examples/v1beta1/pipelineruns/pipelinerun-using-different-subpaths-of-workspace.yaml) which writes data to two adjacent directories on the same Volume.

The `subPath` specified in a `Pipeline` will be appended to any `subPath` specified as part of the `PipelineRun` workspace declaration. So a `PipelineRun` declaring a `Workspace` with `subPath` of `/foo` for a `Pipeline` who binds it to a `Task` with `subPath` of `/bar` will end up mounting the `Volume`'s `/foo/bar` directory.

#### Specifying `Workspace` order in a `Pipeline` and Affinity Assistants

Sharing a `Workspace` between `Tasks` requires you to define the order in which those `Tasks`
write to or read from that `Workspace`. Use the `runAfter` field in your `Pipeline` definition
to define when a `Task` should be executed. For more information, see the [`runAfter` documentation](pipelines.md#using-the-runafter-parameter).

When a `PersistentVolumeClaim` is used as volume source for a `Workspace` in a `PipelineRun`,
an Affinity Assistant will be created. The Affinity Assistant acts as a placeholder for `TaskRun` pods
sharing the same `Workspace`. All `TaskRun` pods within the `PipelineRun` that share the `Workspace`
will be scheduled to the same Node as the Affinity Assistant pod. This means that Affinity Assistant is incompatible
with e.g. other affinity rules configured for the `TaskRun` pods. If the `PipelineRun` has a custom
[PodTemplate](pipelineruns.md#specifying-a-pod-template) configured, the `NodeSelector` and `Tolerations` fields
will also be set on the Affinity Assistant pod. The Affinity Assistant
is deleted when the `PipelineRun` is completed. The Affinity Assistant can be disabled by setting the
[disable-affinity-assistant](install.md#customizing-basic-execution-parameters) feature gate to `true`.

**Note:** Affinity Assistant use [Inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity)
that require substantial amount of processing which can slow down scheduling in large clusters
significantly. We do not recommend using them in clusters larger than several hundred nodes

**Note:** Pod anti-affinity requires nodes to be consistently labelled, in other words every
node in the cluster must have an appropriate label matching `topologyKey`. If some or all nodes
are missing the specified `topologyKey` label, it can lead to unintended behavior.

**Note:** Any time during the execution of a `pipelineRun`, if the node with a placeholder Affinity Assistant pod and
the `taskRun` pods sharing a `workspace` is `cordoned` or disabled for scheduling anything new (`tainted`), the
`pipelineRun` controller deletes the placeholder pod. The `taskRun` pods on a `cordoned` node continues running
until completion. The deletion of a placeholder pod triggers creating a new placeholder pod on any available node
such that the rest of the `pipelineRun` can continue without any disruption until it finishes.

#### Specifying `Workspaces` in `PipelineRuns`

For a `PipelineRun` to execute a `Pipeline` that includes one or more `Workspaces`, it needs to
bind the `Workspace` names to volumes using its own `workspaces` field. Each entry in
this list must correspond to a `Workspace` declaration in the `Pipeline`. Each entry in the
`workspaces` list must specify the following:

- `name` - (**required**) the name of the `Workspace` specified in the `Pipeline` definition for which a volume is being provided.
- `subPath` - (optional) a directory on the volume that will store that `Workspace's` data. This directory must exist at the
  time the `TaskRun` executes, otherwise the execution will fail.

The entry must also include one `VolumeSource`. See [Using `VolumeSources` with `Workspaces`](#specifying-volumesources-in-workspaces) for more information.

**Note:** If the `Workspaces` specified by a `Pipeline` are not provided at runtime by a `PipelineRun`, that `PipelineRun` will fail.

#### Example `PipelineRun` definition using `Workspaces`

In the example below, a `volumeClaimTemplate` is provided for how a `PersistentVolumeClaim` should be created for a workspace named
`myworkspace` declared in a `Pipeline`. When using `volumeClaimTemplate` a new `PersistentVolumeClaim` is created for 
each `PipelineRun` and it allows the user to specify e.g. size and StorageClass for the volume.

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: example-pipelinerun-
spec:
  pipelineRef:
    name: example-pipeline
  workspaces:
    - name: myworkspace # this workspace name must be declared in the Pipeline
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce # access mode may affect how you can use this volume in parallel tasks
          resources:
            requests:
              storage: 1Gi
```

For examples of using other types of volume sources, see [Specifying `VolumeSources` in `Workspaces`](#specifying-volumesources-in-workspaces).
For a more in-depth example, see the [`Workspaces` in `PipelineRun`](../examples/v1beta1/pipelineruns/workspaces.yaml) YAML sample.

### Specifying `VolumeSources` in `Workspaces`

You can only use a single type of `VolumeSource` per `Workspace` entry. The configuration
options differ for each type. `Workspaces` support the following fields:

#### Using `PersistentVolumeClaims` as `VolumeSource`

`PersistentVolumeClaim` volumes are a good choice for sharing data among `Tasks` within a `Pipeline`.
Beware that the [access mode](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes)
configured for the `PersistentVolumeClaim` effects how you can use the volume for parallel `Tasks` in a `Pipeline`. See
[Specifying `workspace` order in a `Pipeline` and Affinity Assistants](#specifying-workspace-order-in-a-pipeline-and-affinity-assistants) for more information about this.
There are two ways of using `PersistentVolumeClaims` as a `VolumeSource`.

##### `volumeClaimTemplate`

The `volumeClaimTemplate` is a template of a [`PersistentVolumeClaim` volume](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim),
created for each `PipelineRun` or `TaskRun`. When the volume is created from a template in a `PipelineRun` or `TaskRun` 
it will be deleted when the `PipelineRun` or `TaskRun` is deleted.

```yaml
workspaces:
  - name: myworkspace
    volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
```

##### `persistentVolumeClaim`

The `persistentVolumeClaim` field references an *existing* [`persistentVolumeClaim` volume](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim). The example exposes only the subdirectory `my-subdir` from that `PersistentVolumeClaim`

```yaml
workspaces:
  - name: myworkspace
    persistentVolumeClaim:
      claimName: mypvc
    subPath: my-subdir
```

#### Using other types of `VolumeSources`

##### `emptyDir`

The `emptyDir` field references an [`emptyDir` volume](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) which holds
a temporary directory that only lives as long as the `TaskRun` that invokes it. `emptyDir` volumes are **not** suitable for sharing data among `Tasks` within a `Pipeline`.
However, they work well for single `TaskRuns` where the data stored in the `emptyDir` needs to be shared among the `Steps` of the `Task` and discarded after execution.

```yaml
workspaces:
  - name: myworkspace
    emptyDir: {}
```

##### `configMap`

The `configMap` field references a [`configMap` volume](https://kubernetes.io/docs/concepts/storage/volumes/#configmap).
Using a `configMap` as a `Workspace` has the following limitations:

- `configMap` volume sources are always mounted as read-only. `Steps` cannot write to them and will error out if they try.
- The `configMap` you want to use as a `Workspace` must exist prior to submitting the `TaskRun`.
- `configMaps` are [size-limited to 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L5042).

```yaml
workspaces:
  - name: myworkspace
    configmap:
      name: my-configmap
```

##### `secret`

The `secret` field references a [`secret` volume](https://kubernetes.io/docs/concepts/storage/volumes/#secret).
Using a `secret` volume has the following limitations:

- `secret` volume sources are always mounted as read-only. `Steps` cannot write to them and will error out if they try.
- The `secret` you want to use as a `Workspace` must exist prior to submitting the `TaskRun`.
- `secret` are [size-limited to 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L5042).

```yaml
workspaces:
  - name: myworkspace
    secret:
      secretName: my-secret
```

##### `projected`

The `projected` field references a [`projected` volume](https://kubernetes.io/docs/concepts/storage/projected-volumes).
`projected` volume workspaces are a [beta feature](./additional-configs.md#beta-features).
Using a `projected` volume has the following limitations:

- `projected` volume sources are always mounted as read-only. `Steps` cannot write to them and will error out if they try.
- The volumes you want to project as a `Workspace` must exist prior to submitting the `TaskRun`.
- The following volumes can be projected: `configMap`, `secret`, `serviceAccountToken` and `downwardApi`

```yaml
workspaces:
  - name: myworkspace
    projected:
      sources:
        - configMap:
            name: my-configmap
        - secret:
            name: my-secret
```

##### `csi`

The `csi` field references a [`csi` volume](https://kubernetes.io/docs/concepts/storage/volumes/#csi).
`csi` workspaces are a [beta feature](./additional-configs.md#beta-features).
Using a `csi` volume has the following limitations:

- `csi` volume sources require a volume driver to use, which must correspond to the value by the CSI driver as defined in the [CSI spec](https://github.com/container-storage-interface/spec/blob/master/spec.md#getplugininfo).

```yaml
workspaces:
  - name: my-credentials
    csi:
      driver: secrets-store.csi.k8s.io
      readOnly: true
      volumeAttributes:
        secretProviderClass: "vault-database"
```

Example of CSI workspace using Hashicorp Vault:

- Install the required csi driver. eg. [secrets-store-csi-driver](https://github.com/hashicorp/vault-csi-provider#using-yaml)
- Install the `vault` Provider onto the kubernetes cluster. [Reference](https://learn.hashicorp.com/tutorials/vault/kubernetes-raft-deployment-guide)
- Deploy a provider via [example](https://gist.github.com/JeromeJu/cc8e4e758029b6694806604750b8911c)
- Create a SecretProviderClass Provider using the following [yaml](https://github.com/tektoncd/pipeline/blob/main/examples/v1beta1/pipelineruns/no-ci/csi-workspace.yaml#L1-L19)
- Specify the ServiceAccount via vault:

```
vault write auth/kubernetes/role/database \
bound_service_account_names=default \
bound_service_account_namespaces=default \
policies=internal-app \
ttl=20m
```

If you need support for a `VolumeSource` type not listed above, [open an issue](https://github.com/tektoncd/pipeline/issues) or
a [pull request](https://github.com/tektoncd/pipeline/blob/main/CONTRIBUTING.md).

## Using Persistent Volumes within a `PipelineRun`

When using a workspace with a [`PersistentVolumeClaim` as `VolumeSource`](#using-persistentvolumeclaims-as-volumesource),
a Kubernetes [Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) is used within the `PipelineRun`.
There are some details that are good to know when using Persistent Volumes within a `PipelineRun`.

### Storage Class

`PersistentVolumeClaims` specify a [Storage Class](https://kubernetes.io/docs/concepts/storage/storage-classes/) for the underlying Persistent Volume. Storage Classes have specific
characteristics. If a StorageClassName is not specified for your `PersistentVolumeClaim`, the cluster defined _default_
Storage Class is used. For _regional_ clusters - clusters that typically consist of Nodes located in multiple Availability
Zones - it is important to know whether your Storage Class is available to all Nodes. Default Storage Classes are typically
only available to Nodes within *one* Availability Zone. There is usually an option to use a _regional_ Storage Class,
but they have trade-offs, e.g. you need to pay for multiple volumes since they are replicated and your volume may have 
substantially higher latency.

When using a workspace backed by a `PersistentVolumeClaim` (typically only available within a Data Center) and the `TaskRun`
pods can be scheduled to any Availability Zone in a regional cluster, some techniques must be used to avoid deadlock in the `Pipeline`.

Tekton provides an Affinity Assistant that schedules all `TaskRun` Pods sharing a `PersistentVolumeClaim` to the same
Node. This avoids deadlocks that can happen when two Pods requiring the same Volume are scheduled to different Availability Zones.
A volume typically only lives within a single Availability Zone.

### Access Modes

A `PersistentVolumeClaim` specifies an [Access Mode](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes).
Available Access Modes are `ReadWriteOnce`, `ReadWriteMany` and `ReadOnlyMany`. What Access Mode you can use depend on
the storage solution that you are using.

* `ReadWriteOnce` is the most commonly available Access Mode. A volume with this Access Mode can only be mounted on one
  Node at a time. This can be problematic for a `Pipeline` that has parallel `Tasks` that access the volume concurrently.
  The Affinity Assistant helps with this problem by scheduling all `Tasks` that use the same `PersistentVolumeClaim` to
  the same Node.
  
* `ReadOnlyMany` is read-only and is less common in a CI/CD-pipeline. These volumes often need to be "prepared" with data
  in some way before use. Dynamically provided volumes can usually not be used in read-only mode.

* `ReadWriteMany` is the least commonly available Access Mode. If you use this access mode and these volumes are available
  to all Nodes within your cluster, you may want to disable the Affinity Assistant.

## More examples

See the following in-depth examples of configuring `Workspaces`:

- [`Workspaces` in a `TaskRun`](../examples/v1beta1/taskruns/workspace.yaml)
- [`Workspaces` in a `PipelineRun`](../examples/v1beta1/pipelineruns/workspaces.yaml)
- [`Workspaces` from a volumeClaimTemplate in a `PipelineRun`](../examples/v1beta1/pipelineruns/workspace-from-volumeclaimtemplate.yaml)
