<!--
---
linkTitle: "Pipelines"
weight: 203
---
-->

# Pipelines

- [Pipelines](#pipelines)
  - [Overview](#overview)
  - [Configuring a `Pipeline`](#configuring-a-pipeline)
  - [Specifying `Workspaces`](#specifying-workspaces)
  - [Specifying `Parameters`](#specifying-parameters)
  - [Adding `Tasks` to the `Pipeline`](#adding-tasks-to-the-pipeline)
    - [Specifying Remote Tasks](#specifying-remote-tasks)
    - [Specifying `Parameters` in `PipelineTasks`](#specifying-parameters-in-pipelinetasks)
    - [Specifying `Matrix` in `PipelineTasks`](#specifying-matrix-in-pipelinetasks)
    - [Specifying `Workspaces` in `PipelineTasks`](#specifying-workspaces-in-pipelinetasks)
    - [Tekton Bundles](#tekton-bundles)
    - [Using the `runAfter` field](#using-the-runafter-field)
    - [Using the `retries` field](#using-the-retries-field)
    - [Guard `Task` execution using `when` expressions](#guard-task-execution-using-when-expressions)
      - [Guarding a `Task` and its dependent `Tasks`](#guarding-a-task-and-its-dependent-tasks)
        - [Cascade `when` expressions to the specific dependent `Tasks`](#cascade-when-expressions-to-the-specific-dependent-tasks)
        - [Compose using Pipelines in Pipelines](#compose-using-pipelines-in-pipelines)
      - [Guarding a `Task` only](#guarding-a-task-only)
    - [Configuring the failure timeout](#configuring-the-failure-timeout)
  - [Using variable substitution](#using-variable-substitution)
    - [Using the `retries` and `retry-count` variable substitutions](#using-the-retries-and-retry-count-variable-substitutions)
  - [Using `Results`](#using-results)
    - [Passing one Task's `Results` into the `Parameters` or `when` expressions of another](#passing-one-tasks-results-into-the-parameters-or-when-expressions-of-another)
    - [Emitting `Results` from a `Pipeline`](#emitting-results-from-a-pipeline)
  - [Configuring the `Task` execution order](#configuring-the-task-execution-order)
  - [Adding a description](#adding-a-description)
  - [Adding `Finally` to the `Pipeline`](#adding-finally-to-the-pipeline)
    - [Specifying `Workspaces` in `finally` tasks](#specifying-workspaces-in-finally-tasks)
    - [Specifying `Parameters` in `finally` tasks](#specifying-parameters-in-finally-tasks)
    - [Specifying `matrix` in `finally` tasks](#specifying-matrix-in-finally-tasks)
    - [Consuming `Task` execution results in `finally`](#consuming-task-execution-results-in-finally)
    - [Consuming `Pipeline` result with `finally`](#consuming-pipeline-result-with-finally)
    - [`PipelineRun` Status with `finally`](#pipelinerun-status-with-finally)
    - [Using Execution `Status` of `pipelineTask`](#using-execution-status-of-pipelinetask)
    - [Using Aggregate Execution `Status` of All `Tasks`](#using-aggregate-execution-status-of-all-tasks)
    - [Guard `finally` `Task` execution using `when` expressions](#guard-finally-task-execution-using-when-expressions)
      - [`when` expressions using `Parameters` in `finally` `Tasks`](#when-expressions-using-parameters-in-finally-tasks)
      - [`when` expressions using `Results` in `finally` 'Tasks`](#when-expressions-using-results-in-finally-tasks)
      - [`when` expressions using `Execution Status` of `PipelineTask` in `finally` `tasks`](#when-expressions-using-execution-status-of-pipelinetask-in-finally-tasks)
      - [`when` expressions using `Aggregate Execution Status` of `Tasks` in `finally` `tasks`](#when-expressions-using-aggregate-execution-status-of-tasks-in-finally-tasks)
    - [Known Limitations](#known-limitations)
      - [Cannot configure the `finally` task execution order](#cannot-configure-the-finally-task-execution-order)
  - [Using Custom Tasks](#using-custom-tasks)
    - [Specifying the target Custom Task](#specifying-the-target-custom-task)
    - [Specifying a Custom Task Spec in-line (or embedded)](#specifying-a-custom-task-spec-in-line-or-embedded)
    - [Specifying parameters](#specifying-parameters-1)
    - [Specifying matrix](#specifying-matrix)
    - [Specifying workspaces](#specifying-workspaces-1)
    - [Using `Results`](#using-results-1)
    - [Specifying `Timeout`](#specifying-timeout)
    - [Specifying `Retries`](#specifying-retries)
    - [Known Custom Tasks](#known-custom-tasks)
  - [Code examples](#code-examples)

## Overview

A `Pipeline` is a collection of `Tasks` that you define and arrange in a specific order
of execution as part of your continuous integration flow. Each `Task` in a `Pipeline`
executes as a `Pod` on your Kubernetes cluster. You can configure various execution
conditions to fit your business needs.

## Configuring a `Pipeline`

A `Pipeline` definition supports the following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1beta1`.
  - [`kind`][kubernetes-overview] - Identifies this resource object as a `Pipeline` object.
  - [`metadata`][kubernetes-overview] - Specifies metadata that uniquely identifies the
    `Pipeline` object. For example, a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    this `Pipeline` object. This must include:
      - [`tasks`](#adding-tasks-to-the-pipeline) - Specifies the `Tasks` that comprise the `Pipeline`
        and the details of their execution.
- Optional:
  - [`params`](#specifying-parameters) - Specifies the `Parameters` that the `Pipeline` requires.
  - [`workspaces`](#specifying-workspaces) - Specifies a set of Workspaces that the `Pipeline` requires.
  - [`tasks`](#adding-tasks-to-the-pipeline):
      - [`name`](#adding-tasks-to-the-pipeline) - the name of this `Task` within the context of this `Pipeline`.
      - [`displayName`](#adding-tasks-to-the-pipeline) - a user-facing name of this `Task` within the context of this `Pipeline`.
      - [`description`](#adding-tasks-to-the-pipeline) - a description of this `Task` within the context of this `Pipeline`.
      - [`taskRef`](#adding-tasks-to-the-pipeline) - a reference to a `Task` definition.
      - [`taskSpec`](#adding-tasks-to-the-pipeline) - a specification of a `Task`.
      - [`runAfter`](#using-the-runafter-field) - Indicates that a `Task` should execute after one or more other
        `Tasks` without output linking.
      - [`retries`](#using-the-retries-field) - Specifies the number of times to retry the execution of a `Task` after
        a failure. Does not apply to execution cancellations.
      - [`when`](#guard-finally-task-execution-using-when-expressions) - Specifies `when` expressions that guard
        the execution of a `Task`; allow execution only when all `when` expressions evaluate to true.
      - [`timeout`](#configuring-the-failure-timeout) - Specifies the timeout before a `Task` fails.
      - [`params`](#specifying-parameters-in-pipelinetasks) - Specifies the `Parameters` that a `Task` requires.
      - [`workspaces`](#specifying-workspaces-in-pipelinetasks) - Specifies the `Workspaces` that a `Task` requires.
      - [`matrix`](#specifying-matrix-in-pipelinetasks) - Specifies the `Parameters` used to fan out a `Task` into
        multiple `TaskRuns` or `Runs`.
  - [`results`](#emitting-results-from-a-pipeline) - Specifies the location to which the `Pipeline` emits its execution
    results.
  - [`displayName`](#specifying-a-display-name) - is a user-facing name of the pipeline that may be used to populate a UI.
  - [`description`](#adding-a-description) - Holds an informative description of the `Pipeline` object.
  - [`finally`](#adding-finally-to-the-pipeline) - Specifies one or more `Tasks` to be executed in parallel after
    all other tasks have completed.
    - [`name`](#adding-finally-to-the-pipeline) - the name of this `Task` within the context of this `Pipeline`.
    - [`displayName`](#adding-finally-to-the-pipeline) - a user-facing name of this `Task` within the context of this `Pipeline`.
    - [`description`](#adding-finally-to-the-pipeline) - a description of this `Task` within the context of this `Pipeline`.
    - [`taskRef`](#adding-finally-to-the-pipeline) - a reference to a `Task` definition.
    - [`taskSpec`](#adding-finally-to-the-pipeline) - a specification of a `Task`.
    - [`retries`](#using-the-retries-field) - Specifies the number of times to retry the execution of a `Task` after
      a failure. Does not apply to execution cancellations.
    - [`when`](#guard-finally-task-execution-using-when-expressions) - Specifies `when` expressions that guard
      the execution of a `Task`; allow execution only when all `when` expressions evaluate to true.
    - [`timeout`](#configuring-the-failure-timeout) - Specifies the timeout before a `Task` fails.
    - [`params`](#specifying-parameters-in-finally-tasks) - Specifies the `Parameters` that a `Task` requires.
    - [`workspaces`](#specifying-workspaces-in-finally-tasks) - Specifies the `Workspaces` that a `Task` requires.
    - [`matrix`](#specifying-matrix-in-finally-tasks) - Specifies the `Parameters` used to fan out a `Task` into
      multiple `TaskRuns` or `Runs`.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

## Specifying `Workspaces`

`Workspaces` allow you to specify one or more volumes that each `Task` in the `Pipeline`
requires during execution. You specify one or more `Workspaces` in the `workspaces` field.
For example:

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
      runAfter:
        - use-ws-from-pipeline # important: use-ws-from-pipeline writes to the workspace first
      workspaces:
        - name: src
          workspace: pipeline-ws1
```

For simplicity you can also map the name of the `Workspace` in `PipelineTask` to match with
the `Workspace` from the `Pipeline`.
For example:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: pipeline
spec:
  workspaces:
    - name: source
  tasks:
    - name: gen-code
      taskRef:
        name: gen-code # gen-code expects a Workspace named "source"
      workspaces:
        - name: source # <- mapping workspace name
    - name: commit
      taskRef:
        name: commit # commit expects a Workspace named "source"
      workspaces:
        - name: source # <- mapping workspace name
      runAfter:
        - gen-code
```

For more information, see:
- [Using `Workspaces` in `Pipelines`](workspaces.md#using-workspaces-in-pipelines)
- The [`Workspaces` in a `PipelineRun`](../examples/v1beta1/pipelineruns/workspaces.yaml) code example
- The [variables available in a `PipelineRun`](variables.md#variables-available-in-a-pipeline), including `workspaces.<name>.bound`.
- [Mapping `Workspaces`](https://github.com/tektoncd/community/blob/main/teps/0108-mapping-workspaces.md)

## Specifying `Parameters`

(See also [Specifying Parameters in Tasks](tasks.md#specifying-parameters))

You can specify global parameters, such as compilation flags or artifact names, that you want to supply
to the `Pipeline` at execution time. `Parameters` are passed to the `Pipeline` from its corresponding
`PipelineRun` and can replace template values specified within each `Task` in the `Pipeline`.

Parameter names:
- Must only contain alphanumeric characters, hyphens (`-`), and underscores (`_`).
- Must begin with a letter or an underscore (`_`).

For example, `fooIs-Bar_` is a valid parameter name, but `barIsBa$` or `0banana` are not.

Each declared parameter has a `type` field, which can be set to either `array` or `string`.
`array` is useful in cases where the number of compilation flags being supplied to the `Pipeline`
varies throughout its execution. If no value is specified, the `type` field defaults to `string`.
When the actual parameter value is supplied, its parsed type is validated against the `type` field.
The `description` and `default` fields for a `Parameter` are optional.

The following example illustrates the use of `Parameters` in a `Pipeline`.

The following `Pipeline` declares two input parameters :

- `context` which passes its value (a string) to the `Task` to set the value of the `pathToContext` parameter within the `Task`.
- `flags` which passes its value (an array) to the `Task` to set the value of
  the `flags` parameter within the `Task`. The `flags` parameter within the
`Task` **must** also be an array.
If you specify a value for the `default` field and invoke this `Pipeline` in a `PipelineRun`
without specifying a value for `context`, that value will be used.

**Note:** Input parameter values can be used as variables throughout the `Pipeline`
by using [variable substitution](variables.md#variables-available-in-a-pipeline).

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: pipeline-with-parameters
spec:
  params:
    - name: context
      type: string
      description: Path to context
      default: /some/where/or/other
    - name: flags
      type: array
      description: List of flags
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-push
      params:
        - name: pathToDockerFile
          value: Dockerfile
        - name: pathToContext
          value: "$(params.context)"
        - name: flags
          value: ["$(params.flags[*])"]
```

The following `PipelineRun` supplies a value for `context`:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipelinerun-with-parameters
spec:
  pipelineRef:
    name: pipeline-with-parameters
  params:
    - name: "context"
      value: "/workspace/examples/microservices/leeroy-web"
    - name: "flags"
      value:
        - "foo"
        - "bar"
```

## Adding `Tasks` to the `Pipeline`

 Your `Pipeline` definition must reference at least one [`Task`](tasks.md).
Each `Task` within a `Pipeline` must have a [valid](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names)
`name` and a `taskRef` or a `taskSpec`. For example:

```yaml
tasks:
  - name: build-the-image
    taskRef:
      name: build-push
```

**Note:** Using both `apiVersion` and `kind` will create [CustomRun](customruns.md), don't set `apiVersion` if only referring to [`Task`](tasks.md).

or

```yaml
tasks:
  - name: say-hello
    taskSpec:
      steps:
      - image: ubuntu
        script: echo 'hello there'
```

Note that any `task` specified in `taskSpec` will be the same version as the `Pipeline`.

### Specifying Remote Tasks

**([beta feature](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#beta-features))**

A `taskRef` field may specify a Task in a remote location such as git.
Support for specific types of remote will depend on the Resolvers your
cluster's operator has installed. For more information including a tutorial, please check [resolution docs](resolution.md). The below example demonstrates referencing a Task in git:

```yaml
tasks:
- name: "go-build"
  taskRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: revision
      # value can use params declared at the pipeline level or a static value like main
      value: $(params.gitRevision)
    - name: pathInRepo
      value: task/golang-build/0.3/golang-build.yaml
```

### Specifying `Parameters` in `PipelineTasks`

You can also provide [`Parameters`](tasks.md#specifying-parameters):

```yaml
spec:
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-push
      params:
        - name: pathToDockerFile
          value: Dockerfile
        - name: pathToContext
          value: /workspace/examples/microservices/leeroy-web
```

### Specifying `Matrix` in `PipelineTasks`

> :seedling: **`Matrix` is an [alpha](install.md#alpha-features) feature.**
> The `enable-api-fields` feature flag must be set to `"alpha"` to specify `Matrix` in a `PipelineTask`.
>
> :warning: This feature is in a preview mode.
> It is still in a very early stage of development and is not yet fully functional.

You can also provide [`Parameters`](tasks.md#specifying-parameters) through the `matrix` field:

```yaml
spec:
  tasks:
    - name: browser-test
      taskRef:
        name: browser-test
      matrix:
        params:
        - name: browser
          value:
          - chrome
          - safari
          - firefox
        include:
          - name: build-1
            params:
              - name: browser
                value: chrome
              - name: url
                value: some-url
```

For further information, read [`Matrix`](./matrix.md).

### Specifying `Workspaces` in `PipelineTasks`

You can also provide [`Workspaces`](tasks.md#specifying-workspaces):

```yaml
spec:
  tasks:
    - name: use-workspace
      taskRef:
        name: gen-code # gen-code expects a workspace with name "output"
      workspaces:
        - name: output
          workspace: shared-ws
```

### Tekton Bundles

A `Tekton Bundle` is an OCI artifact that contains Tekton resources like `Tasks` which can be referenced within a `taskRef`.

There is currently a hard limit of 20 objects in a bundle.

You can reference a `Tekton bundle` in a `TaskRef` in both `v1` and `v1beta1` using [remote resolution](./bundle-resolver.md#pipeline-resolution). The example syntax shown below for `v1` uses remote resolution and requires enabling [beta features](./additional-configs.md#beta-features).

In `v1beta1`, you can also reference a `Tekton bundle` using OCI bundle syntax, which has been deprecated in favor of remote resolution. The example shown below for `v1beta1` uses OCI bundle syntax, and requires enabling `enable-tekton-oci-bundles: "true"` feature flag.


{{< tabs >}}
{{% tab "v1 & v1beta1" %}}
```yaml
spec:
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: docker.io/myrepo/mycatalog
    - name: name
      value: echo-task
    - name: kind
      value: Task
```
{{% /tab %}}

{{% tab "v1beta1" %}}
```yaml
spec:
  tasks:
    - name: hello-world
      taskRef:
        name: echo-task
        bundle: docker.com/myrepo/mycatalog
```
{{% /tab %}}
{{< /tabs >}}

Here, the `bundle` field is the full reference url to the artifact. The name is the
`metadata.name` field of the `Task`.

You may also specify a `tag` as you would with a Docker image which will give you a fixed,
repeatable reference to a `Task`.

{{< tabs >}}
{{% tab "v1 & v1beta1" %}}
```yaml
spec:
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: docker.io/myrepo/mycatalog:v1.0.1
    - name: name
      value: echo-task
    - name: kind
      value: Task
```
{{% /tab %}}

{{% tab "v1beta1" %}}
```yaml
spec:
  tasks:
    - name: hello-world
      taskRef:
        name: echo-task
        bundle: docker.com/myrepo/mycatalog:v1.0.1
```
{{% /tab %}}
{{< /tabs >}}

You may also specify a fixed digest instead of a tag.

{{< tabs >}}
{{% tab "v1 & v1beta1" %}}
```yaml
spec:
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: docker.io/myrepo/mycatalog@sha256:abc123
    - name: name
      value: echo-task
    - name: kind
      value: Task
```
{{% /tab %}}

{{% tab "v1beta1" %}}
```yaml
spec:
  tasks:
    - name: hello-world
      taskRef:
        name: echo-task
        bundle: docker.io/myrepo/mycatalog@sha256:abc123
```
{{% /tab %}}
{{< /tabs >}}

Any of the above options will fetch the image using the `ImagePullSecrets` attached to the
`ServiceAccount` specified in the `PipelineRun`.
See the [Service Account](pipelineruns.md#specifying-custom-serviceaccount-credentials) section
for details on how to configure a `ServiceAccount` on a `PipelineRun`. The `PipelineRun` will then
run that `Task` without registering it in the cluster allowing multiple versions of the same named
`Task` to be run at once.

`Tekton Bundles` may be constructed with any toolsets that produce valid OCI image artifacts
so long as the artifact adheres to the [contract](tekton-bundle-contracts.md).

### Using the `runAfter` field

If you need your `Tasks` to execute in a specific order within the `Pipeline`,
use the `runAfter` field to indicate that a `Task` must execute after
one or more other `Tasks`.

In the example below, we want to test the code before we build it. Since there
is no output from the `test-app` `Task`, the `build-app` `Task` uses `runAfter`
to indicate that `test-app` must run before it, regardless of the order in which
they are referenced in the `Pipeline` definition.

```yaml
workspaces:
- name: source
tasks:
- name: test-app
  taskRef:
    name: make-test
  workspaces:
  - name: source
    workspace: source
- name: build-app
  taskRef:
    name: kaniko-build
  runAfter:
    - test-app
  workspaces:
  - name: source
    workspace: source
```

### Using the `retries` field

For each `Task` in the `Pipeline`, you can specify the number of times Tekton
should retry its execution when it fails. When a `Task` fails, the corresponding
`TaskRun` sets its `Succeeded` `Condition` to `False`. The `retries` field
instructs Tekton to retry executing the `Task` when this happens. `retries` are executed
even when other `Task`s in the `Pipeline` have failed, unless the `PipelineRun` has
been [cancelled](./pipelineruns.md#cancelling-a-pipelinerun) or
[gracefully cancelled](./pipelineruns.md#gracefully-cancelling-a-pipelinerun).

If you expect a `Task` to encounter problems during execution (for example,
you know that there will be issues with network connectivity or missing
dependencies), set its `retries` field to a suitable value greater than 0.
If you don't explicitly specify a value, Tekton does not attempt to execute
the failed `Task` again.

In the example below, the execution of the `build-the-image` `Task` will be
retried once after a failure; if the retried execution fails, too, the `Task`
execution fails as a whole.

```yaml
tasks:
  - name: build-the-image
    retries: 1
    taskRef:
      name: build-push
```

### Guard `Task` execution using `when` expressions

To run a `Task` only when certain conditions are met, it is possible to _guard_ task execution using the `when` field. The `when` field allows you to list a series of references to `when` expressions.

The components of `when` expressions are `input`, `operator` and `values`:

| Component  | Description                                                                                                | Syntax                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
|------------|------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `input`    | Input for the `when` expression, defaults to an empty string if not provided.                              | * Static values e.g. `"ubuntu"`<br/> * Variables ([parameters](#specifying-parameters) or [results](#using-results)) e.g. `"$(params.image)"` or `"$(tasks.task1.results.image)"` or `"$(tasks.task1.results.array-results[1])"`                                                                                                                                                                                                                                               |
| `operator` | `operator` represents an `input`'s relationship to a set of `values`, a valid `operator` must be provided. | `in` or `notin`                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `values`   | An array of string values, the `values` array must be provided and has to be non-empty.                    | * An array param e.g. `["$(params.images[*])"]`<br/> * An array result of a task `["$(tasks.task1.results.array-results[*])"]`<br/> * `values` can contain static values e.g. `"ubuntu"`<br/> * `values` can contain variables ([parameters](#specifying-parameters) or [results](#using-results)) or [a Workspaces's `bound` state](#specifying-workspaces) e.g. `["$(params.image)"]` or `["$(tasks.task1.results.image)"]` or `["$(tasks.task1.results.array-results[1])"]` |


The [`Parameters`](#specifying-parameters) are read from the `Pipeline` and [`Results`](#using-results) are read directly from previous [`Tasks`](#adding-tasks-to-the-pipeline). Using [`Results`](#using-results) in a `when` expression in a guarded `Task` introduces a resource dependency on the previous `Task` that produced the `Result`.

The declared `when` expressions are evaluated before the `Task` is run. If all the `when` expressions evaluate to `True`, the `Task` is run. If any of the `when` expressions evaluate to `False`, the `Task` is not run and the `Task` is listed in the [`Skipped Tasks` section of the `PipelineRunStatus`](pipelineruns.md#monitoring-execution-status).

In these examples, `first-create-file` task will only be executed if the `path` parameter is `README.md`, `echo-file-exists` task will only be executed if the `exists` result from `check-file` task is `yes` and `run-lint` task will only be executed if the `lint-config` optional workspace has been provided by a PipelineRun.

```yaml
tasks:
  - name: first-create-file
    when:
      - input: "$(params.path)"
        operator: in
        values: ["README.md"]
    taskRef:
      name: first-create-file
---
tasks:
  - name: echo-file-exists
    when:
      - input: "$(tasks.check-file.results.exists)"
        operator: in
        values: ["yes"]
    taskRef:
      name: echo-file-exists
---
tasks:
  - name: run-lint
    when:
      - input: "$(workspaces.lint-config.bound)"
        operator: in
        values: ["true"]
    taskRef:
      name: lint-source
---
tasks:
  - name: deploy-in-blue
    when:
      - input: "blue"
        operator: in
        values: ["$(params.deployments[*])"]
    taskRef:
      name: deployment
```

For an end-to-end example, see [PipelineRun with `when` expressions](../examples/v1beta1/pipelineruns/pipelinerun-with-when-expressions.yaml).

There are a lot of scenarios where `when` expressions can be really useful. Some of these are:
- Checking if the name of a git branch matches
- Checking if the `Result` of a previous `Task` is as expected
- Checking if a git file has changed in the previous commits
- Checking if an image exists in the registry
- Checking if the name of a CI job matches
- Checking if an optional Workspace has been provided

#### Guarding a `Task` and its dependent `Tasks`

To guard a `Task` and its dependent Tasks:
- cascade the `when` expressions to the specific dependent `Tasks` to be guarded as well
- compose the `Task` and its dependent `Tasks` as a unit to be guarded and executed together using `Pipelines` in `Pipelines`

##### Cascade `when` expressions to the specific dependent `Tasks`

Pick and choose which specific dependent `Tasks` to guard as well, and cascade the `when` expressions to those `Tasks`.

Taking the use case below, a user who wants to guard `manual-approval` and its dependent `Tasks`:

```
                                     tests
                                       |
                                       v
                                 manual-approval
                                 |            |
                                 v        (approver)
                            build-image       |
                                |             v
                                v          slack-msg
                            deploy-image
```

The user can design the `Pipeline` to solve their use case as such:

```yaml
tasks:
#...
- name: manual-approval
  runAfter:
    - tests
  when:
    - input: $(params.git-action)
      operator: in
      values:
        - merge
  taskRef:
    name: manual-approval

- name: build-image
  when:
    - input: $(params.git-action)
      operator: in
      values:
        - merge
  runAfter:
    - manual-approval
  taskRef:
    name: build-image

- name: deploy-image
  when:
    - input: $(params.git-action)
      operator: in
      values:
        - merge
  runAfter:
    - build-image
  taskRef:
    name: deploy-image

- name: slack-msg
  params:
    - name: approver
      value: $(tasks.manual-approval.results.approver)
  taskRef:
    name: slack-msg
```

##### Compose using Pipelines in Pipelines

Compose a set of `Tasks` as a unit of execution using `Pipelines` in `Pipelines`, which allows for guarding a `Task` and
its dependent `Tasks` (as a sub-`Pipeline`) using `when` expressions.

**Note:** `Pipelines` in `Pipelines` is an [experimental feature](https://github.com/tektoncd/experimental/tree/main/pipelines-in-pipelines)

Taking the use case below, a user who wants to guard `manual-approval` and its dependent `Tasks`:

```
                                     tests
                                       |
                                       v
                                 manual-approval
                                 |            |
                                 v        (approver)
                            build-image       |
                                |             v
                                v          slack-msg
                            deploy-image
```

The user can design the `Pipelines` to solve their use case as such:

```yaml
## sub pipeline (approve-build-deploy-slack)
tasks:
  - name: manual-approval
    runAfter:
      - integration-tests
    taskRef:
      name: manual-approval

  - name: build-image
    runAfter:
      - manual-approval
    taskRef:
      name: build-image

  - name: deploy-image
    runAfter:
      - build-image
    taskRef:
      name: deploy-image

  - name: slack-msg
    params:
      - name: approver
        value: $(tasks.manual-approval.results.approver)
    taskRef:
      name: slack-msg

---
## main pipeline
tasks:
#...
- name: approve-build-deploy-slack
  runAfter:
    - tests
  when:
    - input: $(params.git-action)
      operator: in
      values:
        - merge
  taskRef:
    apiVersion: tekton.dev/v1beta1
    kind: Pipeline
    name: approve-build-deploy-slack
```

#### Guarding a `Task` only

When `when` expressions evaluate to `False`, the `Task` will be skipped and:
- The ordering-dependent `Tasks` will be executed
- The resource-dependent `Tasks` (and their dependencies) will be skipped because of missing `Results` from the skipped
  parent `Task`. When we add support for [default `Results`](https://github.com/tektoncd/community/pull/240), then the
  resource-dependent `Tasks` may be executed if the default `Results` from the skipped parent `Task` are specified. In
  addition, if a resource-dependent `Task` needs a file from a guarded parent `Task` in a shared `Workspace`, make sure
  to handle the execution of the child `Task` in case the expected file is missing from the `Workspace` because the
  guarded parent `Task` is skipped.

On the other hand, the rest of the `Pipeline` will continue executing.

```
                                     tests
                                       |
                                       v
                                 manual-approval
                                 |            |
                                 v        (approver)
                            build-image       |
                                |             v
                                v          slack-msg
                            deploy-image
```

Taking the use case above, a user who wants to guard `manual-approval` only can design the `Pipeline` as such:

```yaml
tasks:
#...
- name: manual-approval
  runAfter:
    - tests
  when:
    - input: $(params.git-action)
      operator: in
      values:
        - merge
  taskRef:
    name: manual-approval

- name: build-image
  runAfter:
    - manual-approval
  taskRef:
    name: build-image

- name: deploy-image
  runAfter:
    - build-image
  taskRef:
    name: deploy-image

- name: slack-msg
  params:
    - name: approver
      value: $(tasks.manual-approval.results.approver)
  taskRef:
    name: slack-msg
```

If `manual-approval` is skipped, execution of its dependent `Tasks` (`slack-msg`, `build-image` and `deploy-image`)
would be unblocked regardless:
- `build-image` and `deploy-image` should be executed successfully
- `slack-msg` will be skipped because it is missing the `approver` `Result` from `manual-approval`
  - dependents of `slack-msg` would have been skipped too if it had any of them
  - if `manual-approval` specifies a default `approver` `Result`, such as "None", then `slack-msg` would be executed
    ([supporting default `Results` is in progress](https://github.com/tektoncd/community/pull/240))

### Configuring the failure timeout

You can use the `Timeout` field in the `Task` spec within the `Pipeline` to set the timeout
of the `TaskRun` that executes that `Task` within the `PipelineRun` that executes your `Pipeline.`
The `Timeout` value is a `duration` conforming to Go's [`ParseDuration`](https://golang.org/pkg/time/#ParseDuration)
format. For example, valid values are `1h30m`, `1h`, `1m`, and `60s`.

**Note:** If you do not specify a `Timeout` value, Tekton instead honors the timeout for the [`PipelineRun`](pipelineruns.md#configuring-a-pipelinerun).

In the example below, the `build-the-image` `Task` is configured to time out after 90 seconds:

```yaml
spec:
  tasks:
    - name: build-the-image
      taskRef:
        name: build-push
      timeout: "0h1m30s"
```

## Using variable substitution

Tekton provides variables to inject values into the contents of certain fields.
The values you can inject come from a range of sources including other fields
in the Pipeline, context-sensitive information that Tekton provides, and runtime
information received from a PipelineRun.

The mechanism of variable substitution is quite simple - string replacement is
performed by the Tekton Controller when a PipelineRun is executed.

See the [complete list of variable substitutions for Pipelines](./variables.md#variables-available-in-a-pipeline)
and the [list of fields that accept substitutions](./variables.md#fields-that-accept-variable-substitutions).

For an end-to-end example, see [using context variables](../examples/v1beta1/pipelineruns/using_context_variables.yaml).

### Using the `retries` and `retry-count` variable substitutions

Tekton supports variable substitution for the [`retries`](#using-the-retries-field)
parameter of `PipelineTask`. Variables like `context.pipelineTask.retries` and
`context.task.retry-count` can be added to the parameters of a `PipelineTask`.
`context.pipelineTask.retries` will be replaced by `retries` of the `PipelineTask`, while
`context.task.retry-count` will be replaced by current retry number of the `PipelineTask`.

```yaml
params:
- name: pipelineTask-retries
  value: "$(context.pipelineTask.retries)"
taskSpec:
  params:
  - name: pipelineTask-retries
  steps:
  - image: ubuntu
    name: print-if-retries-exhausted
    script: |
      if [ "$(context.task.retry-count)" == "$(params.pipelineTask-retries)" ]
      then
        echo "This is the last retry."
      fi
      exit 1
```

**Note:** Every `PipelineTask` can only access its own `retries` and `retry-count`. These
values aren't accessible for other `PipelineTask`s.

## Using `Results`

Tasks can emit [`Results`](tasks.md#emitting-results) when they execute. A Pipeline can use these
`Results` for two different purposes:

1. A Pipeline can pass the `Result` of a `Task` into the `Parameters` or `when` expressions of another.
2. A Pipeline can itself emit `Results` and include data from the `Results` of its Tasks.

### Passing one Task's `Results` into the `Parameters` or `when` expressions of another

Sharing `Results` between `Tasks` in a `Pipeline` happens via
[variable substitution](variables.md#variables-available-in-a-pipeline) - one `Task` emits
a `Result` and another receives it as a `Parameter` with a variable such as
`$(tasks.<task-name>.results.<result-name>)`. Pipeline support two new types of
results and parameters: array `[]string` and object `map[string]string`.
Array and Object result is a beta feature and can be enabled by setting `enable-api-fields` to `alpha` or `beta`.

| Result Type | Parameter Type | Specification                                    | `enable-api-fields` |
|-------------|----------------|--------------------------------------------------|---------------------|
| string      | string         | `$(tasks.<task-name>.results.<result-name>)`     | stable              |
| array       | array          | `$(tasks.<task-name>.results.<result-name>[*])`  | alpha or beta       |
| array       | string         | `$(tasks.<task-name>.results.<result-name>[i])`  | alpha or beta       |
| object      | object         | `$(tasks.<task-name>.results.<result-name>[*])`  | alpha or beta              |
| object      | string         | `$(tasks.<task-name>.results.<result-name>.key)` | alpha or beta              |

**Note:** Whole Array and Object `Results` (using star notation) cannot be referred in `script`.

**Note:** `Matrix` does not support `object` and `array` results.

When one `Task` receives the `Results` of another, there is a dependency created between those
two `Tasks`. In order for the receiving `Task` to get data from another `Task's` `Result`,
the `Task` producing the `Result` must run first. Tekton enforces this `Task` ordering
by ensuring that the `Task` emitting the `Result` executes before any `Task` that uses it.

In the snippet below, a param is provided its value from the `commit` `Result` emitted by the
`checkout-source` `Task`. Tekton will make sure that the `checkout-source` `Task` runs
before this one.

```yaml
params:
  - name: foo
    value: "$(tasks.checkout-source.results.commit)"
  - name: array-params
    value: "$(tasks.checkout-source.results.array-results[*])"
  - name: array-indexing-params
    value: "$(tasks.checkout-source.results.array-results[1])"
  - name: object-params
    value: "$(tasks.checkout-source.results.object-results[*])"
  - name: object-element-params
    value: "$(tasks.checkout-source.results.object-results.objectkey)"
```

**Note:** If `checkout-source` exits successfully without initializing `commit` `Result`,
the receiving `Task` fails and causes the `Pipeline` to fail with `InvalidTaskResultReference`:

```
unable to find result referenced by param 'foo' in 'task';: Could not find result with name 'commit' for task run 'checkout-source'
```

In the snippet below, a `when` expression is provided its value from the `exists` `Result` emitted by the
`check-file` `Task`. Tekton will make sure that the `check-file` `Task` runs before this one.

```yaml
when:
  - input: "$(tasks.check-file.results.exists)"
    operator: in
    values: ["yes"]
```

For an end-to-end example, see [`Task` `Results` in a `PipelineRun`](../examples/v1beta1/pipelineruns/task_results_example.yaml).

Note that `when` expressions are whitespace-sensitive.  In particular, when producing results intended for inputs to `when`
expressions that may include newlines at their close (e.g. `cat`, `jq`), you may wish to truncate them.

```yaml
taskSpec:
  params:
  - name: jsonQuery-check
  steps:
  - image: ubuntu
    name: store-name-in-results
    script: |
      curl -s https://my-json-server.typicode.com/typicode/demo/profile | jq -r .name | tr -d '\n' | tee $(results.name.path)
```

### Emitting `Results` from a `Pipeline`

A `Pipeline` can emit `Results` of its own for a variety of reasons - an external
system may need to read them when the `Pipeline` is complete, they might summarise
the most important `Results` from the `Pipeline's` `Tasks`, or they might simply
be used to expose non-critical messages generated during the execution of the `Pipeline`.

A `Pipeline's` `Results` can be composed of one or many `Task` `Results` emitted during
the course of the `Pipeline's` execution. A `Pipeline` `Result` can refer to its `Tasks'`
`Results` using a variable of the form `$(tasks.<task-name>.results.<result-name>)`.

After a `Pipeline` has executed the `PipelineRun` will be populated with the `Results`
emitted by the `Pipeline`. These will be written to the `PipelineRun's`
`status.pipelineResults` field.

In the example below, the `Pipeline` specifies a `results` entry with the name `sum` that
references the `outputValue` `Result` emitted by the `calculate-sum` `Task`.

```yaml
results:
  - name: sum
    description: the sum of all three operands
    value: $(tasks.calculate-sum.results.outputValue)
```

For an end-to-end example, see [`Results` in a `PipelineRun`](../examples/v1beta1/pipelineruns/pipelinerun-results.yaml).

Object result and array result are beta features,
see [`Array and Object Results` in a `PipelineRun`](../examples/v1beta1/pipelineruns/beta/pipeline-emitting-results.yaml).

```yaml
    results:
      - name: array-results
        type: array
        description: whole array
        value: $(tasks.task1.results.array-results[*])
      - name: array-indexing-results
        type: string
        description: array element
        value: $(tasks.task1.results.array-results[1])
      - name: object-results
        type: object
        description: whole object
        value: $(tasks.task2.results.object-results[*])
      - name: object-element
        type: string
        description: object element
        value: $(tasks.task2.results.object-results.foo)
```


A `Pipeline Result` is not emitted if any of the following are true:
- A `PipelineTask` referenced by the `Pipeline Result` failed. The `PipelineRun` will also
have failed.
- A `PipelineTask` referenced by the `Pipeline Result` was skipped.
- A `PipelineTask` referenced by the `Pipeline Result` didn't emit the referenced `Task Result`. This
should be considered a bug in the `Task` and [may fail a `PipelineTask` in future](https://github.com/tektoncd/pipeline/issues/3497).
- The `Pipeline Result` uses a variable that doesn't point to an actual `PipelineTask`. This will
result in an `InvalidTaskResultReference` validation error during `PipelineRun` execution.
- The `Pipeline Result` uses a variable that doesn't point to an actual result in a `PipelineTask`.
This will cause an `InvalidTaskResultReference` validation error during `PipelineRun` execution.

**Note:** Since a `Pipeline Result` can contain references to multiple `Task Results`, if any of those
`Task Result` references are invalid the entire `Pipeline Result` is not emitted.
**Note:** If a `PipelineTask` referenced by the `Pipeline Result` was skipped, the `Pipeline Result` will not be emitted and the `PipelineRun` will not fail due to a missing result.

## Configuring the `Task` execution order

You can connect `Tasks` in a `Pipeline` so that they execute in a Directed Acyclic Graph (DAG).
Each `Task` in the `Pipeline` becomes a node on the graph that can be connected with an edge
so that one will run before another and the execution of the `Pipeline` progresses to completion
without getting stuck in an infinite loop.

This is done using:
- _resource dependencies_:
  - [`results`](#emitting-results-from-a-pipeline) of one `Task` being passed into `params` or `when` expressions of
    another

- _ordering dependencies_:
  - [`runAfter`](#using-the-runafter-field) clauses on the corresponding `Tasks`

For example, the `Pipeline` defined as follows

```yaml
tasks:
- name: lint-repo
  taskRef:
    name: pylint
- name: test-app
  taskRef:
    name: make-test
- name: build-app
  taskRef:
    name: kaniko-build-app
  runAfter:
    - test-app
- name: build-frontend
  taskRef:
    name: kaniko-build-frontend
  runAfter:
    - test-app
- name: deploy-all
  taskRef:
    name: deploy-kubectl
  runAfter:
    - build-app
    - build-frontend
```

executes according to the following graph:

```none
        |            |
        v            v
     test-app    lint-repo
    /        \
   v          v
build-app  build-frontend
   \          /
    v        v
    deploy-all
```

In particular:

1. The `lint-repo` and `test-app` `Tasks` have no `runAfter` clauses
   and start executing simultaneously.
2. Once `test-app` completes, both `build-app` and `build-frontend` start
   executing simultaneously since they both `runAfter` the `test-app` `Task`.
3. The `deploy-all` `Task` executes once both `build-app` and `build-frontend`
   complete, since it is supposed to `runAfter` them both.
4. The entire `Pipeline` completes execution once both `lint-repo` and `deploy-all`
   complete execution.

## Specifying a display name

The `displayName` field is an optional field that allows you to add a user-facing name of the `Pipeline` that can be used to populate a UI. For example:

```yaml
spec:
  displayName: "Code Scan"
  tasks:
    - name: scan
      taskRef:
        name: sonar-scan
```

## Adding a description

The `description` field is an optional field and can be used to provide description of the `Pipeline`.

## Adding `Finally` to the `Pipeline`

You can specify a list of one or more final tasks under `finally` section. `finally` tasks are guaranteed to be executed
in parallel after all `PipelineTasks` under `tasks` have completed regardless of success or error. `finally` tasks are very
similar to `PipelineTasks` under `tasks` section and follow the same syntax. Each `finally` task must have a
[valid](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names) `name` and a [taskRef or
taskSpec](taskruns.md#specifying-the-target-task). For example:

```yaml
spec:
  tasks:
    - name: tests
      taskRef:
        name: integration-test
  finally:
    - name: cleanup-test
      taskRef:
        name: cleanup
```

### Specifying `Workspaces` in `finally` tasks

`finally` tasks can specify [workspaces](workspaces.md) which `PipelineTasks` might have utilized
e.g. a mount point for credentials held in Secrets. To support that requirement, you can specify one or more
`Workspaces` in the `workspaces` field for the `finally` tasks similar to `tasks`.

```yaml
spec:
  workspaces:
    - name: shared-workspace
  tasks:
    - name: clone-app-source
      taskRef:
        name: clone-app-repo-to-workspace
      workspaces:
        - name: shared-workspace
          workspace: shared-workspace
  finally:
    - name: cleanup-workspace
      taskRef:
        name: cleanup-workspace
      workspaces:
        - name: shared-workspace
          workspace: shared-workspace
```

### Specifying `Parameters` in `finally` tasks

Similar to `tasks`, you can specify [`Parameters`](tasks.md#specifying-parameters) in `finally` tasks:

```yaml
spec:
  tasks:
    - name: tests
      taskRef:
        name: integration-test
  finally:
    - name: report-results
      taskRef:
        name: report-results
      params:
        - name: url
          value: "someURL"
```

### Specifying `matrix` in `finally` tasks

> :seedling: **`Matrix` is an [alpha](install.md#alpha-features) feature.**
> The `enable-api-fields` feature flag must be set to `"alpha"` to specify `Matrix` in a `PipelineTask`.
>
> :warning: This feature is in a preview mode.
> It is still in a very early stage of development and is not yet fully functional.

Similar to `tasks`, you can also provide [`Parameters`](tasks.md#specifying-parameters) through `matrix`
in `finally` tasks:

```yaml
spec:
  tasks:
    - name: tests
      taskRef:
        name: integration-test
  finally:
    - name: report-results
      taskRef:
        name: report-results
      params:
        - name: url
          value: "someURL"
      matrix:
        params:
        - name: slack-channel
          value:
          - "foo"
          - "bar"
        include:
          - name: build-1
            params:
              - name: slack-channel
                value: "foo"
              - name: flags
                value: "-v"
```

For further information, read [`Matrix`](./matrix.md).

### Consuming `Task` execution results in `finally`

`finally` tasks can be configured to consume `Results` of `PipelineTask` from the `tasks` section:

```yaml
spec:
  tasks:
    - name: clone-app-repo
      taskRef:
        name: git-clone
  finally:
    - name: discover-git-commit
      params:
        - name: commit
          value: $(tasks.clone-app-repo.results.commit)
```
**Note:** The scheduling of such `finally` task does not change, it will still be executed in parallel with other
`finally` tasks after all non-`finally` tasks are done.

The controller resolves task results before executing the `finally` task `discover-git-commit`. If the task
`clone-app-repo` failed before initializing `commit` or skipped with [when expression](#guard-task-execution-using-when-expressions)
resulting in uninitialized task result `commit`, the `finally` Task `discover-git-commit` will be included in the list of
`skippedTasks` and continues executing rest of the `finally` tasks. The pipeline exits with `completion` instead of
`success` if a `finally` task is added to the list of `skippedTasks`.

### Consuming `Pipeline` result with `finally`

`finally` tasks can emit `Results` and these results emitted from the `finally` tasks can be configured in the
[Pipeline Results](#emitting-results-from-a-pipeline). References of `Results` from `finally` will follow the same naming conventions as referencing `Results` from `tasks`: ```$(finally.<finally-pipelinetask-name>.result.<result-name>)```.

```yaml
results:
  - name: comment-count-validate
    value: $(finally.check-count.results.comment-count-validate)
finally:
  - name: check-count
    taskRef:
      name: example-task-name
```

In this example, `pipelineResults` in `status` will show the name-value pair for the result `comment-count-validate` which is produced in the `Task` `example-task-name`.


### `PipelineRun` Status with `finally`

With `finally`, `PipelineRun` status is calculated based on `PipelineTasks` under `tasks` section and `finally` tasks.

Without `finally`:

| `PipelineTasks` under `tasks`                                                                           | `PipelineRun` status | Reason      |
|---------------------------------------------------------------------------------------------------------|----------------------|-------------|
| all `PipelineTasks` successful                                                                          | `true`               | `Succeeded` |
| one or more `PipelineTasks` [skipped](#guard-task-execution-using-when-expressions) and rest successful | `true`               | `Completed` |
| single failure of `PipelineTask`                                                                        | `false`              | `failed`    |

With `finally`:

| `PipelineTasks` under `tasks`                                                                          | `finally` tasks                        | `PipelineRun` status | Reason      |
|--------------------------------------------------------------------------------------------------------|----------------------------------------|----------------------|-------------|
| all `PipelineTask` successful                                                                          | all `finally` tasks successful         | `true`               | `Succeeded` |
| all `PipelineTask` successful                                                                          | one or more failure of `finally` tasks | `false`              | `Failed`    |
| one or more `PipelineTask` [skipped](#guard-task-execution-using-when-expressions) and rest successful | all `finally` tasks successful         | `true`               | `Completed` |
| one or more `PipelineTask` [skipped](#guard-task-execution-using-when-expressions) and rest successful | one or more failure of `finally` tasks | `false`              | `Failed`    |
| single failure of `PipelineTask`                                                                       | all `finally` tasks successful         | `false`              | `failed`    |
| single failure of `PipelineTask`                                                                       | one or more failure of `finally` tasks | `false`              | `failed`    |

Overall, `PipelineRun` state transitioning is explained below for respective scenarios:

* All `PipelineTask` and `finally` tasks are successful: `Started` -> `Running` -> `Succeeded`
* At least one `PipelineTask` skipped and rest successful:  `Started` -> `Running` -> `Completed`
* One `PipelineTask` failed / one or more `finally` tasks failed: `Started` -> `Running` -> `Failed`

Please refer to the [table](pipelineruns.md#monitoring-execution-status) under Monitoring Execution Status to learn about
what kind of events are triggered based on the `Pipelinerun` status.


### Using Execution `Status` of `pipelineTask`

A `pipeline` can check the status of a specific `pipelineTask` from the `tasks` section in `finally` through the task
parameters:

```yaml
finally:
  - name: finaltask
    params:
      - name: task1Status
        value: "$(tasks.task1.status)"
    taskSpec:
      params:
        - name: task1Status
      steps:
        - image: ubuntu
          name: print-task-status
          script: |
            if [ $(params.task1Status) == "Failed" ]
            then
              echo "Task1 has failed, continue processing the failure"
            fi
```

This kind of variable can have any one of the values from the following table:

| Status      | Description                                                                                      |
|-------------|--------------------------------------------------------------------------------------------------|
| `Succeeded` | `taskRun` for the `pipelineTask` completed successfully                                          |
| `Failed`    | `taskRun` for the `pipelineTask` completed with a failure or cancelled by the user               |
| `None`      | the `pipelineTask` has been skipped or no execution information available for the `pipelineTask` |

For an end-to-end example, see [`status` in a `PipelineRun`](../examples/v1beta1/pipelineruns/pipelinerun-task-execution-status.yaml).

### Using Aggregate Execution `Status` of All `Tasks`

A `pipeline` can check an aggregate status of all the `tasks` section in `finally` through the task parameters:

```yaml
finally:
  - name: finaltask
    params:
      - name: aggregateTasksStatus
        value: "$(tasks.status)"
    taskSpec:
      params:
        - name: aggregateTasksStatus
      steps:
        - image: ubuntu
          name: check-task-status
          script: |
            if [ $(params.aggregateTasksStatus) == "Failed" ]
            then
              echo "Looks like one or more tasks returned failure, continue processing the failure"
            fi
```

This kind of variable can have any one of the values from the following table:

| Status      | Description                                                                                                                       |
|-------------|-----------------------------------------------------------------------------------------------------------------------------------|
| `Succeeded` | all `tasks` have succeeded                                                                                                        |
| `Failed`    | one ore more `tasks` failed                                                                                                       |
| `Completed` | all `tasks` completed successfully including one or more skipped tasks                                                            |
| `None`      | no aggregate execution status available (i.e. none of the above), one or more `tasks` could be pending/running/cancelled/timedout |

For an end-to-end example, see [`$(tasks.status)` usage in a `Pipeline`](../examples/v1beta1/pipelineruns/pipelinerun-task-execution-status.yaml).

### Guard `finally` `Task` execution using `when` expressions

Similar to `Tasks`, `finally` `Tasks` can be guarded using [`when` expressions](#guard-task-execution-using-when-expressions)
that operate on static inputs or variables. Like in `Tasks`, `when` expressions in `finally` `Tasks` can operate on
`Parameters` and `Results`. Unlike in `Tasks`, `when` expressions in `finally` `tasks` can also operate on the [`Execution
Status`](#using-execution-status-of-pipelinetask) of `Tasks`.

#### `when` expressions using `Parameters` in `finally` `Tasks`

`when` expressions in `finally` `Tasks` can utilize `Parameters` as demonstrated using [`golang-build`](https://github.com/tektoncd/catalog/tree/main/task/golang-build/0.1)
and [`send-to-channel-slack`](https://github.com/tektoncd/catalog/tree/main/task/send-to-channel-slack/0.1) Catalog
`Tasks`:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pipelinerun-
spec:
  pipelineSpec:
    params:
      - name: enable-notifications
        type: string
        description: a boolean indicating whether the notifications should be sent
    tasks:
      - name: golang-build
        taskRef:
          name: golang-build
      # […]
    finally:
      - name: notify-build-failure # executed only when build task fails and notifications are enabled
        when:
          - input: $(tasks.golang-build.status)
            operator: in
            values: ["Failed"]
          - input: $(params.enable-notifications)
            operator: in
            values: ["true"]
        taskRef:
          name: send-to-slack-channel
      # […]
  params:
    - name: enable-notifications
      value: true
```

#### `when` expressions using `Results` in `finally` 'Tasks`

`when` expressions in `finally` `tasks` can utilize `Results`, as demonstrated using [`git-clone`](https://github.com/tektoncd/catalog/tree/main/task/git-clone/0.2)
and [`github-add-comment`](https://github.com/tektoncd/catalog/tree/main/task/github-add-comment/0.2) Catalog `Tasks`:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pipelinerun-
spec:
  pipelineSpec:
    tasks:
      - name: git-clone
        taskRef:
          name: git-clone
      - name: go-build
      # […]
    finally:
      - name: notify-commit-sha # executed only when commit sha is not the expected sha
        when:
          - input: $(tasks.git-clone.results.commit)
            operator: notin
            values: [$(params.expected-sha)]
        taskRef:
          name: github-add-comment
      # […]
  params:
    - name: expected-sha
      value: 54dd3984affab47f3018852e61a1a6f9946ecfa
```

If the `when` expressions in a `finally` `task` use `Results` from a skipped or failed non-finally `Tasks`, then the
`finally` `task` would also be skipped and be included in the list of `Skipped Tasks` in the `Status`, [similarly to when using
`Results` in other parts of the `finally` `task`](#consuming-task-execution-results-in-finally).

#### `when` expressions using `Execution Status` of `PipelineTask` in `finally` `tasks`

`when` expressions in `finally` `tasks` can utilize [`Execution Status` of `PipelineTasks`](#using-execution-status-of-pipelinetask),
as demonstrated using [`golang-build`](https://github.com/tektoncd/catalog/tree/main/task/golang-build/0.1) and
[`send-to-channel-slack`](https://github.com/tektoncd/catalog/tree/main/task/send-to-channel-slack/0.1) Catalog `Tasks`:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pipelinerun-
spec:
  pipelineSpec:
    tasks:
      - name: golang-build
        taskRef:
          name: golang-build
      # […]
    finally:
      - name: notify-build-failure # executed only when build task fails
        when:
          - input: $(tasks.golang-build.status)
            operator: in
            values: ["Failed"]
        taskRef:
          name: send-to-slack-channel
      # […]
```

For an end-to-end example, see [PipelineRun with `when` expressions](../examples/v1beta1/pipelineruns/pipelinerun-with-when-expressions.yaml).

#### `when` expressions using `Aggregate Execution Status` of `Tasks` in `finally` `tasks`

`when` expressions in `finally` `tasks` can utilize
[`Aggregate Execution Status` of `Tasks`](#using-aggregate-execution-status-of-all-tasks) as demonstrated:

```yaml
finally:
  - name: notify-any-failure # executed only when one or more tasks fail
    when:
      - input: $(tasks.status)
        operator: in
        values: ["Failed"]
    taskRef:
      name: notify-failure
```

For an end-to-end example, see [PipelineRun with `when` expressions](../examples/v1beta1/pipelineruns/pipelinerun-with-when-expressions.yaml).

### Known Limitations

#### Cannot configure the `finally` task execution order

It's not possible to configure or modify the execution order of the `finally` tasks. Unlike `Tasks` in a `Pipeline`,
all `finally` tasks run simultaneously and start executing once all `PipelineTasks` under `tasks` have settled which means
no `runAfter` can be specified in `finally` tasks.

## Using Custom Tasks

Custom Tasks have been promoted from `v1alpha1` to `v1beta1`. Starting from `v0.43.0` to `v0.46.0`, Pipeline Controller is able to create either `v1alpha1` or `v1beta1` Custom Task gated by a feature flag `custom-task-version`, defaulting to `v1beta1`. You can set `custom-task-version` to `v1alpha1` or `v1beta1` to control which version to create.

Starting from `v0.47.0`, feature flag `custom-task-version` is removed and only `v1beta1` Custom Task will be supported. See the [migration doc](migrating-v1alpha1.Run-to-v1beta1.CustomRun.md) for details.

[Custom Tasks](https://github.com/tektoncd/community/blob/main/teps/0002-custom-tasks.md)
can implement behavior that doesn't correspond directly to running a workload in a `Pod` on the cluster.
For example, a custom task might execute some operation outside of the cluster and wait for its execution to complete.

A `PipelineRun` starts a custom task by creating a [`CustomRun`](https://github.com/tektoncd/pipeline/blob/main/docs/customruns.md) instead of a `TaskRun`.
In order for a custom task to execute, there must be a custom task controller running on the cluster
that is responsible for watching and updating `CustomRun`s which reference their type.

### Specifying the target Custom Task

To specify the custom task type you want to execute, the `taskRef` field
must include the custom task's `apiVersion` and `kind` as shown below.
Using `apiVersion` will always create a `CustomRun`. If `apiVersion` is set, `kind` is required as well.

```yaml
spec:
  tasks:
    - name: run-custom-task
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
```

This creates a `Run/CustomRun` of a custom task of type `Example` in the `example.dev` API group with the version `v1alpha1`.

Validation error will be returned if `apiVersion` or `kind` is missing.

You can also specify the `name` of a custom task resource object previously defined in the cluster.

```yaml
spec:
  tasks:
    - name: run-custom-task
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
        name: myexample
```

If the `taskRef` specifies a name, the custom task controller should look up the
`Example` resource with that name and use that object to configure the execution.

If the `taskRef` does not specify a name, the custom task controller might support
some default behavior for executing unnamed tasks.

### Specifying a Custom Task Spec in-line (or embedded)

**For `v1alpha1.Run`**
```yaml
spec:
  tasks:
    - name: run-custom-task
      taskSpec:
        apiVersion: example.dev/v1alpha1
        kind: Example
        spec:
          field1: value1
          field2: value2
```

**For `v1beta1.CustomRun`**
```yaml
spec:
  tasks:
    - name: run-custom-task
      taskSpec:
        apiVersion: example.dev/v1alpha1
        kind: Example
        customSpec:
          field1: value1
          field2: value2
```

If the custom task controller supports the in-line or embedded task spec, this will create a `Run/CustomRun` of a custom task of
type `Example` in the `example.dev` API group with the version `v1alpha1`.

If the `taskSpec` is not supported, the custom task controller should produce proper validation errors.

Please take a look at the
developer guide for custom controllers supporting `taskSpec`:
- [guidance for `Run`](runs.md#developer-guide-for-custom-controllers-supporting-spec)
- [guidance for `CustomRun`](customruns.md#developer-guide-for-custom-controllers-supporting-customspec)

`taskSpec` support for `pipelineRun` was designed and discussed in
[TEP-0061](https://github.com/tektoncd/community/blob/main/teps/0061-allow-custom-task-to-be-embedded-in-pipeline.md)

### Specifying parameters

If a custom task supports [`parameters`](tasks.md#specifying-parameters), you can use the
`params` field to specify their values:

```yaml
spec:
  tasks:
    - name: run-custom-task
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
        name: myexample
      params:
        - name: foo
          value: bah
```

## Context Variables

The `Parameters` in the `Params` field will accept
[context variables](variables.md) that will be substituted, including:

* `PipelineRun` name, namespace and uid
* `Pipeline` name
* `PipelineTask` retries

```yaml
spec:
  tasks:
    - name: run-custom-task
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
        name: myexample
      params:
        - name: foo
          value: $(context.pipeline.name)
```

### Specifying matrix

> :seedling: **`Matrix` is an [alpha](install.md#alpha-features) feature.**
> The `enable-api-fields` feature flag must be set to `"alpha"` to specify `Matrix` in a `PipelineTask`.
>
> :warning: This feature is in a preview mode.
> It is still in a very early stage of development and is not yet fully functional.

If a custom task supports [`parameters`](tasks.md#specifying-parameters), you can use the
`matrix` field to specify their values, if you want to fan:

```yaml
spec:
  tasks:
    - name: run-custom-task
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
        name: myexample
      params:
        - name: foo
          value: bah
      matrix:
        params:
        - name: bar
          value:
            - qux
            - thud
        include:
          - name: build-1
            params:
            - name: common-package
              value: path-to-common-pkg
```

For further information, read [`Matrix`](./matrix.md).

### Specifying workspaces

If the custom task supports it, you can provide [`Workspaces`](workspaces.md#using-workspaces-in-tasks) to share data with the custom task.

```yaml
spec:
  tasks:
    - name: run-custom-task
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
        name: myexample
      workspaces:
        - name: my-workspace
```

Consult the documentation of the custom task that you are using to determine whether it supports workspaces and how to name them.

### Using `Results`

If the custom task produces results, you can reference them in a Pipeline using the normal syntax,
`$(tasks.<task-name>.results.<result-name>)`.

### Specifying `Timeout`

#### `v1alpha1.Run`
If the custom task supports it as [we recommended](runs.md#developer-guide-for-custom-controllers-supporting-timeout), you can provide `timeout` to specify the maximum running time of a `CustomRun` (including all retry attempts or other operations).

#### `v1beta1.CustomRun`
If the custom task supports it as [we recommended](customruns.md#developer-guide-for-custom-controllers-supporting-timeout), you can provide `timeout` to specify the maximum running time of one `CustomRun` execution.

```yaml
spec:
  tasks:
    - name: run-custom-task
      timeout: 2s
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
        name: myexample
```

Consult the documentation of the custom task that you are using to determine whether it supports `Timeout`.

### Specifying `Retries`
If the custom task supports it, you can provide `retries` to specify how many times you want to retry the custom task.

```yaml
spec:
  tasks:
    - name: run-custom-task
      retries: 2
      taskRef:
        apiVersion: example.dev/v1alpha1
        kind: Example
        name: myexample
```

Consult the documentation of the custom task that you are using to determine whether it supports `Retries`.

### Known Custom Tasks

We try to list as many known Custom Tasks as possible here so that users can easily find what they want. Please feel free to share the Custom Task you implemented in this table.

#### v1beta1.CustomRun

| Custom Task                      | Description                                                                                                                      |
|:---------------------------------|:---------------------------------------------------------------------------------------------------------------------------------|
| [Wait Task Beta][wait-task-beta] | Waits a given amount of time before succeeding, specified by an input parameter named duration. Support `timeout` and `retries`. |
| [Approvals][approvals-beta]| Pauses the execution of `PipelineRuns` and waits for manual approvals. Version 0.6.0 and up. |

#### v1alpha1.Run

| Custom Task                                      | Description                                                                                                |
|:-------------------------------------------------|:-----------------------------------------------------------------------------------------------------------|
| [Pipeline Loops][pipeline-loops]                 | Runs a `Pipeline` in a loop with varying `Parameter` values.                                               |
| [Common Expression Language][cel]                | Provides Common Expression Language support in Tekton Pipelines.                                           |
| [Wait][wait]                                     | Waits a given amount of time, specified by a `Parameter` named "duration", before succeeding.              |
| [Approvals][approvals-alpha]                     | Pauses the execution of `PipelineRuns` and waits for manual approvals. Version up to (and including) 0.5.0 |
| [Pipelines in Pipelines][pipelines-in-pipelines] | Defines and executes a `Pipeline` in a `Pipeline`.                                                         |
| [Task Group][task-group]                         | Groups `Tasks` together as a `Task`.                                                                       |
| [Pipeline in a Pod][pipeline-in-pod]             | Runs `Pipeline` in a `Pod`.                                                                                |

[pipeline-loops]: https://github.com/tektoncd/experimental/tree/f60e1cd8ce22ed745e335f6f547bb9a44580dc7c/pipeline-loops
[task-loops]: https://github.com/tektoncd/experimental/tree/f60e1cd8ce22ed745e335f6f547bb9a44580dc7c/task-loops
[cel]: https://github.com/tektoncd/experimental/tree/f60e1cd8ce22ed745e335f6f547bb9a44580dc7c/cel
[wait]: https://github.com/tektoncd/experimental/tree/f60e1cd8ce22ed745e335f6f547bb9a44580dc7c/wait-task
[approvals-alpha]: https://github.com/automatiko-io/automatiko-approval-task/tree/v0.5.0
[approvals-beta]: https://github.com/automatiko-io/automatiko-approval-task/tree/v0.6.1
[task-group]: https://github.com/openshift-pipelines/tekton-task-group/tree/39823f26be8f59504f242a45b9f2e791d4b36e1c
[pipelines-in-pipelines]: https://github.com/tektoncd/experimental/tree/f60e1cd8ce22ed745e335f6f547bb9a44580dc7c/pipelines-in-pipelines
[pipeline-in-pod]: https://github.com/tektoncd/experimental/tree/f60e1cd8ce22ed745e335f6f547bb9a44580dc7c/pipeline-in-pod
[wait-task-beta]: https://github.com/tektoncd/pipeline/tree/a127323da31bcb933a04a6a1b5dbb6e0411e3dc1/test/custom-task-ctrls/wait-task-beta

## Code examples

For a better understanding of `Pipelines`, study [our code examples](https://github.com/tektoncd/pipeline/tree/main/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
