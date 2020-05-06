<!--
---
linkTitle: "Pipelines"
weight: 3
---
-->
# Pipelines

- [Overview](#pipelines)
- [Configuring a `Pipeline`](#configuring-a-pipeline)
  - [Specifying `Resources`](#specifying-resources)
  - [Specifying `Workspaces`](#specifying-workspaces)
  - [Specifying `Parameters`](#specifying-parameters)
  - [Adding `Tasks` to the `Pipeline`](#adding-tasks-to-the-pipeline)
    - [Using the `from` parameter](#using-the-from-parameter)
    - [Using the `runAfter` parameter](#using-the-runafter-parameter)
    - [Using the `retries` parameter](#using-the-retries-parameter)
    - [Guard `Task` execution using `Conditions`](#guard-task-execution-using-conditions)
    - [Configuring the failure timeout](#configuring-the-failure-timeout)
    - [Configuring execution results at the `Task` level](#configuring-execution-results-at-the-task-level)
  - [Configuring execution results at the `Pipeline` level](#configuring-execution-results-at-the-pipeline-level)
  - [Configuring the `Task` execution order](#configuring-the-task-execution-order)
  - [Adding a description](#adding-a-description)
  - [Adding `Finally` to the `Pipeline` (Preview)](#adding-finally-to-the-pipeline-preview)
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
  - [`resources`](#specifying-resources) - **alpha only** Specifies
    [`PipelineResources`](resources.md) needed or created by the `Tasks` comprising the `Pipeline`.
  - [`tasks`](#adding-tasks-to-the-pipeline):
      - `resources.inputs` / `resource.outputs`
        - [`from`](#using-the-from-parameter) - Indicates the data for a [`PipelineResource`](resources.md)
          originates from the output of a previous `Task`.
      - [`runAfter`](#using-the-runafter-parameter) - Indicates that a `Task`
        should execute after one or more other `Tasks` without output linking.
      - [`retries`](#using-the-retries-parameter) - Specifies the number of times to retry the
        execution of a `Task` after a failure. Does not apply to execution cancellations.
      - [`conditions`](#guard-task-execution-using-conditions) - Specifies `Conditions` that only allow a `Task`
        to execute if they successfully evaluate.
      - [`timeout`](#configuring-the-failure-timeout) - Specifies the timeout before a `Task` fails. 
  - [`results`](#configuring-execution-results-at-the-pipeline-level) - Specifies the location to which
    the `Pipeline` emits its execution results.
  - [`description`](#adding-a-description) - Holds an informative description of the `Pipeline` object.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

## Specifying `Resources`

A `Pipeline` requires [`PipelineResources`](resources.md) to provide inputs and store outputs
for the `Tasks` that comprise it. You can declare those in the `resources` field in the `spec`
section of the `Pipeline` definition. Each entry requires a unique `name` and a `type`. For example:

```yaml
spec:
  resources:
    - name: my-repo
      type: git
    - name: my-image
      type: image
```

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

For more information, see:
- [Using `Workspaces` in `Pipelines`](workspaces.md#using-workspaces-in-pipelines)
- The [`Workspaces` in a `PipelineRun`](../examples/v1beta1/pipelineruns/workspaces.yaml) code example

## Specifying `Parameters`

You can specify global parameters, such as compilation flags or artifact names, that you want to supply
to the `Pipeline` at execution time. `Parameters` are passed to the `Pipeline` from its corresponding
`PipelineRun` and can replace template values specified within each `Task` in the `Pipeline`.

Parameter names:
- Must only contain alphanumeric characters, hyphens (`-`), and underscores (`_`).
- Must begin with a letter or an underscore (`_`).

For example, `fooIs-Bar_` is a valid parameter name, but `barIsBa$` or `0banana` are not.

Each declared parameter has a `type` field, which can be set to either `array` or `string`.
`array` is useful in cases where the number of compiliation flags being supplied to the `Pipeline`
varies throughout its execution. If no value is specified, the `type` field defaults to `string`.
When the actual parameter value is supplied, its parsed type is validated against the `type` field.
The `description` and `default` fields for a `Parameter` are optional.

The following example illustrates the use of `Parameters` in a `Pipeline`. 

The following `Pipeline` declares an input parameter called `context` and passes its
value to the `Task` to set the value of the `pathToContext` parameter within the `Task`.
If you specify a value for the `default` field and invoke this `Pipeline` in a `PipelineRun`
without specifying a value for `context`, that value will be used.

**Note:** Input parameter values can be used as variables throughout the `Pipeline`
by using [variable substitution](variables.md#variables-available-in-a-pipeline).

```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: pipeline-with-parameters
spec:
  params:
    - name: context
      type: string
      description: Path to context
      default: /some/where/or/other
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-push
      params:
        - name: pathToDockerFile
          value: Dockerfile
        - name: pathToContext
          value: "$(params.context)"
```

The following `PipelineRun` supplies a value for `context`:

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipelinerun-with-parameters
spec:
  pipelineRef:
    name: pipeline-with-parameters
  params:
    - name: "context"
      value: "/workspace/examples/microservices/leeroy-web"
```

## Adding `Tasks` to the `Pipeline`

 Your `Pipeline` definition must reference at least one [`Task`](tasks.md).
Each `Task` within a `Pipeline` must have a [valid](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names)
`name` and a `taskRef`. For example:

```yaml
tasks:
  - name: build-the-image
    taskRef:
      name: build-push
```

You can use [`PipelineResources`](#specifying-resources) as inputs and outputs for `Tasks`
in the `Pipeline`. For example:

```yaml
spec:
  tasks:
    - name: build-the-image
      taskRef:
        name: build-push
      resources:
        inputs:
          - name: workspace
            resource: my-repo
        outputs:
          - name: image
            resource: my-image
```

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

### Using the `from` parameter

If a `Task` in your `Pipeline` needs to use the output of a previous `Task`
as its input, use the optional `from` parameter to specify a list of `Tasks`
that must execute **before** the `Task` that requires their outputs as its 
input. When your target `Task` executes, only the version of the desired 
`PipelineResource` produced by the last `Task` in this list is used. The
`name` of this output `PipelineReource` output must match the `name` of the
input `PipelineResource` specified in the `Task` that ingests it. 

In the example below, the `deploy-app` `Task` ingests the output of the `build-app`
`Task` named `my-image` as its input.  Therefore, the `build-app` `Task` will 
execute before the `deploy-app` `Task` regardless of the order in which those
`Tasks` are declared in the `Pipeline`.

```yaml
- name: build-app
  taskRef:
    name: build-push
  resources:
    outputs:
      - name: image
        resource: my-image
- name: deploy-app
  taskRef:
    name: deploy-kubectl
  resources:
    inputs:
      - name: image
        resource: my-image
        from:
          - build-app
```

### Using the `runAfter` parameter

If you need your `Tasks` to execute in a specific order within the `Pipeline`
but they don't have resource dependencies that require the `from` parameter,
use the `runAfter` parameter to indicate that a `Task` must execute after
one or more other `Tasks`.

In the example below, we want to test the code before we build it. Since there
is no output from the `test-app` `Task`, the `build-app` `Task` uses `runAfter`
to indicate that `test-app` must run before it, regardless of the order in which
they are referenced in the `Pipeline` definition.

```yaml
- name: test-app
  taskRef:
    name: make-test
  resources:
    inputs:
      - name: workspace
        resource: my-repo
- name: build-app
  taskRef:
    name: kaniko-build
  runAfter:
    - test-app
  resources:
    inputs:
      - name: workspace
        resource: my-repo
```

### Using the `retries` parameter

For each `Task` in the `Pipeline`, you can specify the number of times Tekton
should retry its execution when it fails. When a `Task` fails, the corresponding
`TaskRun` sets its `Succeeded` `Condition` to `False`. The `retries` parameter
instructs Tekton to retry executing the `Task` when this happens.

If you expect a `Task` to encounter problems during execution (for example,
you know that there will be issues with network connectivitity or missing
dependencies), set its `retries` parameter to a suitable value greater than 0.
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

### Guard `Task` execution using `Conditions`

To run a `Task` only when certain conditions are met, it is possible to _guard_ task execution using
the `conditions` field. The `conditions` field allows you to list a series of references to
[`Condition`](./conditions.md) resources. The declared `Conditions` are run before the `Task` is run.
If all of the conditions successfully evaluate, the `Task` is run. If any of the conditions fails,
the `Task` is not run and the `TaskRun` status field `ConditionSucceeded` is set to `False` with the
reason set to `ConditionCheckFailed`.

In this example, `is-master-branch` refers to a [Condition](conditions.md) resource. The `deploy`
task will only be executed if the condition successfully evaluates.

```yaml
tasks:
  - name: deploy-if-branch-is-master
    conditions:
      - conditionRef: is-master-branch
        params:
          - name: branch-name
            value: my-value
    taskRef:
      name: deploy
```

Unlike regular task failures, condition failures do not automatically fail the entire `PipelineRun` -- 
other tasks that are **not dependent** on the `Task` (via `from` or `runAfter`) are still run.

In this example, `(task C)` has a `condition` set to _guard_ its execution. If the condition
is **not** successfully evaluated, task `(task D)` will not be run, but all other tasks in the pipeline
that not depend on `(task C)` will be executed and the `PipelineRun` will successfully complete.
 
  ```
         (task B) — (task E)
       / 
   (task A) 
       \
         (guarded task C) — (task D)
  ```

Resources in conditions can also use the [`from`](#using-the-from-parameter) field to indicate that they
expect the output of a previous task as input. As with regular Pipeline Tasks, using `from`
implies ordering --  if task has a condition that takes in an output resource from
another task, the task producing the output resource will run first:

```yaml
tasks:
  - name: first-create-file
    taskRef:
      name: create-file
    resources:
      outputs:
        - name: workspace
          resource: source-repo
  - name: then-check
    conditions:
      - conditionRef: "file-exists"
        resources:
          - name: workspace
            resource: source-repo
            from: [first-create-file]
    taskRef:
      name: echo-hello
```

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
      Timeout: "0h1m30s"
```

### Configuring execution results at the `Task` level

Tasks can emit [`Results`](tasks.md#storing-execution-results) while they execute. You can
use these `Results` values as parameter values in subsequent `Tasks` within your `Pipeline`
through [variable substitution](variables.md#variables-available-in-a-pipeline). Tekton infers the
`Task` order so that the `Task` emitting the referenced `Results` values executes before the
`Task` that consumes them. 

In the example below, the result of the `previous-task-name` `Task` is declared as `bar-result`:

```yaml
params:
  - name: foo
    value: "$(tasks.previous-task-name.results.bar-result)"
```

For an end-to-end example, see [`Task` `Results` in a `PipelineRun`](../examples/v1beta1/pipelineruns/task_results_example.yaml).

## Configuring execution results at the `Pipeline` level

You can configure your `Pipeline` to emit `Results` during its execution as references to
the `Results` emitted by each `Task` within it. 

In the example below, the `Pipeline` specifies a `results` entry with the name `sum` that
references the `Result` emitted by the `second-add` `Task`.

```yaml
  results:
    - name: sum
      description: the sum of all three operands
      value: $(tasks.second-add.results.sum)
```

For an end-to-end example, see [`Results` in a `PipelineRun`](../examples/v1beta1/pipelineruns/pipelinerun-results.yaml).

## Configuring the `Task` execution order

You can connect `Tasks` in a `Pipeline` so that they execute in a Directed Acyclic Graph (DAG).
Each `Task` in the `Pipeline` becomes a node on the graph that can be connected with an edge
so that one will run before another and the execution of the `Pipeline` progresses to completion
without getting stuck in an infinite loop.

This is done using:

- [`from`](#using-the-from-parameter) clauses on the [`PipelineResources`](resources.md) used by each `Task`
- [`runAfter`](#using-the-runafter-parameter) clauses on the corresponding `Tasks`
- By linking the [`results`](#configuring-execution-results-at-the-pipeline-level) of one `Task` to the params of another

For example, the `Pipeline` defined as follows

```yaml
- name: lint-repo
  taskRef:
    name: pylint
  resources:
    inputs:
      - name: workspace
        resource: my-repo
- name: test-app
  taskRef:
    name: make-test
  resources:
    inputs:
      - name: workspace
        resource: my-repo
- name: build-app
  taskRef:
    name: kaniko-build-app
  runAfter:
    - test-app
  resources:
    inputs:
      - name: workspace
        resource: my-repo
    outputs:
      - name: image
        resource: my-app-image
- name: build-frontend
  taskRef:
    name: kaniko-build-frontend
  runAfter:
    - test-app
  resources:
    inputs:
      - name: workspace
        resource: my-repo
    outputs:
      - name: image
        resource: my-frontend-image
- name: deploy-all
  taskRef:
    name: deploy-kubectl
  resources:
    inputs:
      - name: my-app-image
        resource: my-app-image
        from:
          - build-app
      - name: my-frontend-image
        resource: my-frontend-image
        from:
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

1. The `lint-repo` and `test-app` `Tasks` have no `from` or `runAfter` clauses
   and start executing simultaneously.
2. Once `test-app` completes, both `build-app` and `build-frontend` start
   executing simultaneously since they both `runAfter` the `test-app` `Task`.
3. The `deploy-all` `Task` executes once both `build-app` and `build-frontend`
   complete, since it ingests `PipelineResources` from both.
4. The entire `Pipeline` completes execution once both `lint-repo` and `deploy-all`
   complete execution.

## Adding a description

The `description` field is an optional field and can be used to provide description of the `Pipeline`.

## Adding `Finally` to the `Pipeline` (Preview)

_Finally type is available in the `Pipeline` but functionality is in progress. Final tasks are can be specified and
are validated but not executed yet._

You can specify a list of one or more final tasks under `finally` section. Final tasks are guaranteed to be executed
in parallel after all `PipelineTasks` under `tasks` have completed regardless of success or error. Final tasks are very
similar to `PipelineTasks` under `tasks` section and follow the same syntax. Each final task must have a
[valid](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names) `name` and a [taskRef or
taskSpec](taskruns.md#specifying-the-target-task). For example:

```yaml
spec:
  tasks:
    - name: tests
      taskRef:
        Name: integration-test
  finally:
    - name: cleanup-test
      taskRef:
        Name: cleanup
```

_[PR #2661](https://github.com/tektoncd/pipeline/pull/2661) is implementing this new functionality by adding support to enable
final tasks along with workspaces and parameters. `PipelineRun` status is being updated to include execution status of
final tasks i.e. `PipelineRun` status is set to success or failure depending on execution of `PipelineTasks`, this status
remains same when all final tasks finishes successfully but is set to failure if any of the final tasks fail._

## Code examples

For a better understanding of `Pipelines`, study [our code examples](https://github.com/tektoncd/pipeline/tree/master/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
