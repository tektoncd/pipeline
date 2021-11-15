<!--
---
linkTitle: "Migrating from Knative Build"
weight: 4100
---
-->
# Migrating from Knative Build

Tekton Pipelines is the technological successor to [Knative Build](https://github.com/knative/build). Tekton
entities are based on Knative Build's entities but provide additional flexibility and reusability. This page
explains how to convert your Knative Build entities to Tekton entities of equivalent functionality.

The table below lists the mapping between the old and new entities:

| **Knative Build Entity** | **Tekton Entity**  |
|--------------------------|--------------------|
| `Build`                  | `TaskRun`          |
| `BuildTemplate`          | `Task`             |
| `ClusterBuildTemplate`   | `ClusterTask`      |

## Conversion guidelines

Follow the guidelines below when converting your Knative Build entities to Tekton entities:

* All `steps` must specify a `name` value.

* `BuildTemplate` [parameters](tasks.md#specifying-parameters) now reside inside the `input.params` field of a `Task`. In a
   related change, parameter placeholders, such as `${FOO}`, now follow the format `$(input.parameters.FOO)`. For more information,
   see [Using variable substitution](tasks.md#using-variable-substitution).

* `Tasks` that ingest inputs from resources must now explicitly specify [`input.resources`](tasks.md#specifying-resources).
  `BuildTemplates` did not explicitly specify input resources and just assumed those resources were always available.

* Input resource data is no longer cloned into the `/workspace` directory. Instead, Tekton clones the data into a subdirectory
  of your choice within the `/workspace` directory. You must specify this subdirectory using the `name` field in your input
  resource definition. For example, if you specify a `git` resource named `foo`, Tekton clones the data into `/workspace/foo`.
  Due to this change, we highly recommend that you set `workingDir: /workspace/foo` in the affected `steps` within your `Task`.
  For more information, see [Controlling where resources are mounted](resources.md#controlling-where-resources-are-mounted).

* `TaskRuns` which specify a `PipelineResource` as the value of the `input.resources` field of the invoked `Task`
  can do so either by referencing an existing `PipelineResource` using the `resourceRef` field or by embedding
  a complete `PipelineResource` definition using the [`resourceSpec`](taskruns.md#specifying-resources) field.

  > :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
  >
  > Consider using replacement features instead. Read more in [documentation](migrating-v1alpha1-to-v1beta1.md#replacing-pipelineresources-with-tasks)
  > and [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

* Containers that execute the `Steps` in your `Tasks` must now abide by Tekton's [container contract](container-contract.md)
  and are now serialized without relying on init containers. Because of this, we highly recommend
  that for each `Step` within your `Task` you specify a `command` value instead of an `entrypoint` and `args` pair.

## Code examples

Study the following code examples to better understand the conversion of entities described earlier in this document.

### Converting a `BuildTemplate` to a `Task`

This example `BuildTemplate` runs `go test`.

```
apiVersion: build.knative.dev/v1alpha1
kind: BuildTemplate
metadata:
  name: go-test
spec:
  parameters:
  - name: TARGET
    description: The Go target to test
    default: ./...

  steps:
  - image: golang
    args: ['go', 'test', '${TARGET}']
```

Below is an equivalent `Task`.

```
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: go-test
spec:
  params:
  - name: TARGET
    description: The Go target to test
    default: ./...

  # The Task must operate on a source, such as a Git repo.
  resources:
    inputs:
    - name: source
      type: git

  steps:
  - name: go-test  # <-- the step must specify a name.
    image: golang
    workingDir: /workspace/source  # <-- set workingdir
    command: ['go', 'test', '$(params.TARGET)']  # <-- specify params.TARGET
```

### Converting a `Build` to a `TaskRun`

This example `Build` instantiates and runs the `BuildTemplate` from the previous example.

```
apiVersion: build.knative.dev/v1alpha1
kind: Build
metadata:
  name: go-test
spec:
  source:
    git:
      url: https://github.com/my-user/my-repo
      revision: master
  template:
    name: go-test
    arguments:
    - name: TARGET
      value: ./path/to/test/...
```

Below is an equivalent `TaskRun`.

```
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: example-run
spec:
  taskRef:
    name: go-test
  params:
  - name: TARGET
    value: ./path/to/test/...
  resources:
    inputs:
    - name: source
      resourceSpec:
        type: git
        params:
        - name: url
          value: https://github.com/my-user/my-repo
```
