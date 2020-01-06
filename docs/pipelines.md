# Pipelines

This document defines `Pipelines` and their capabilities.

---

- [Syntax](#syntax)
  - [Declared resources](#declared-resources)
  - [Workspaces](#declared-workspaces)
  - [Parameters](#parameters)
  - [Pipeline Tasks](#pipeline-tasks)
    - [From](#from)
    - [RunAfter](#runAfter)
    - [Retries](#retries)
- [Ordering](#ordering)
- [Examples](#examples)

## Syntax

To define a configuration file for a `Pipeline` resource, you can specify the
following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1alpha1`.
  - [`kind`][kubernetes-overview] - Specify the `Pipeline` resource object.
  - [`metadata`][kubernetes-overview] - Specifies data to uniquely identify the
    `Pipeline` resource object, for example a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    your `Pipeline` resource object. In order for a `Pipeline` to do anything,
    the spec must include:
    - [`tasks`](#pipeline-tasks) - Specifies which `Tasks` to run and how to run
      them
- Optional:
  - [`resources`](#declared-resources) - Specifies which
    [`PipelineResources`](resources.md) of which types the `Pipeline` will be
    using in its [Tasks](#pipeline-tasks)
  - `tasks`
    - `resources.inputs` / `resource.outputs`
      - [`from`](#from) - Used when the content of the
        [`PipelineResource`](resources.md) should come from the
        [output](tasks.md#outputs) of a previous [Pipeline Task](#pipeline-tasks)
      - [`runAfter`](#runAfter) - Used when the [Pipeline Task](#pipeline-tasks)
        should be executed after another Pipeline Task, but there is no
        [output linking](#from) required
      - [`retries`](#retries) - Used when the task is wanted to be executed if
        it fails. Could be a network error or a missing dependency. It does not
        apply to cancellations.
      - [`conditions`](#conditions) - Used when a task is to be executed only if the specified
        conditions are evaluated to be true.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Declared resources

In order for a `Pipeline` to interact with the outside world, it will probably
need [`PipelineResources`](resources.md) which will be given to
`Tasks` as inputs and outputs.

Your `Pipeline` must declare the `PipelineResources` it needs in a `resources`
section in the `spec`, giving each a name which will be used to refer to these
`PipelineResources` in the `Tasks`.

For example:

```yaml
spec:
  resources:
    - name: my-repo
      type: git
    - name: my-image
      type: image
```

### Declared Workspaces

`workspaces` are a way of declaring volumes you expect to be made available to your
executing `Pipeline` and its `Task`s. They are similar to [`volumes`](#volumes) but
allow you to enforce at runtime that the volumes have been attached and
[allow you to specify subpaths](taskruns.md#workspaces) in the volumes to attach.

Any `Pipeline` using a `Task` that declares a workspace will need to provide one at
runtime. Doing so requires two additions in a Pipeline:

1. The `Pipeline` will need to declare a list of `workspaces` that `PipelineRun`s will be
expected to provide. This is done with the `workspaces` field in the `Pipeline`'s spec.
Each entry in that list must have a unique name.
2. When a `Pipeline` refers to a `Task` requiring workspaces, one of the named workspaces
from (1) will need to be provided. The workspace name needs to be mapped from the name
given to it by the pipeline to the name expected by the task.

In total this looks as follows:

```yaml
spec:
  workspaces:
    - name: pipeline-ws1 # The name of a workspace provided by PipelineRuns
  tasks:
    - name: use-ws-from-pipeline
      taskRef:
        name: gen-code # gen-code task expects a workspace be provided with name "output"
      workspaces:
        - name: output
          workspace: pipeline-ws1
    - name: use-ws-again
      taskRef:
        name: commit # commit task expects a workspace be provided with name "src"
      workspaces:
        - name: src
          workspace: pipeline-ws1
```

This will tell Tekton to take whatever workspace is provided by the PipelineRun
with name "pipeline-ws1" and wire it into the "output" workspace expected by
the gen-code task. The same workspace will then also be wired into the "src" workspace
expected by the commit task. If the workspace provided by the PipelineRun is a
persitent volume claim then we have successfully shared files between the two tasks!

#### Workspaces Don't Imply Task Ordering (Yet)

One usecase for workspaces in `Pipeline`s is to provide a PVC to multiple `Task`s
and have one or some write to it before the others read from it. This kind of behaviour
relies on the order of the `Task`s - one writes, the next reads, and so on - but this
ordering is not currently enforced by Tekton. This means that `Task`s which write to a
PVC may be run at the same time as `Task`s expecting to read that data. In the worst case
this can result in deadlock behaviour where multiple `Task`'s pods are all attempting
to mount a PVC for writing at the same time.

To avoid this situation `Pipeline` authors can explicitly declare the ordering of `Task`s
sharing a PVC-backed workspace by using the `runAfter` field. See [the section on
`runAfter`](#runAfter) for more information about using this field.

### Parameters

`Pipeline`s can declare input parameters that must be supplied to the `Pipeline`
during a `PipelineRun`. Pipeline parameters can be used to replace template
values in [`PipelineTask` parameters' values](#pipeline-tasks).

Parameter names are limited to alpha-numeric characters, `-` and `_` and can
only start with alpha characters and `_`. For example, `fooIs-Bar_` is a valid
parameter name, `barIsBa$` or `0banana` are not.

Each declared parameter has a `type` field, assumed to be `string` if not provided by the user. The other possible type is `array` — useful, for instance, when a dynamic number of string arguments need to be supplied to a task. When the actual parameter value is supplied, its parsed type is validated against the `type` field.

#### Usage

The following example shows how `Pipeline`s can be parameterized, and these
parameters can be passed to the `Pipeline` from a `PipelineRun`.

Input parameters in the form of `$(params.foo)` are replaced inside of the
[`PipelineTask` parameters' values](#pipeline-tasks) (see also
[variable substitution](tasks.md#variable-substitution)).

The following `Pipeline` declares an input parameter called 'context', and uses
it in the `PipelineTask`'s parameter. The `description` and `default` fields for
a parameter are optional, and if the `default` field is specified and this
`Pipeline` is used by a `PipelineRun` without specifying a value for 'context',
the `default` value will be used.

```yaml
apiVersion: tekton.dev/v1alpha1
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
apiVersion: tekton.dev/v1alpha1
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

### Pipeline Tasks

A `Pipeline` will execute a graph of [`Tasks`](tasks.md) (see
[ordering](#ordering) for how to express this graph). A valid `Pipeline` 
declaration must include a reference to at least one [`Task`](tasks.md). Each
`Task` within a `Pipeline` must have a
[valid](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names)
name and task reference, for example:

```yaml
tasks:
  - name: build-the-image
    taskRef:
      name: build-push
```

[Declared `PipelineResources`](#declared-resources) can be given to `Task`s in
the `Pipeline` as inputs and outputs, for example:

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

[Parameters](tasks.md#parameters) can also be provided:

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

#### from

Sometimes you will have [Pipeline Tasks](#pipeline-tasks) that need to take as
input the output of a previous `Task`, for example, an image built by a previous
`Task`.

Express this dependency by adding `from` on [`PipelineResources`](resources.md)
that your `Tasks` need.

- The (optional) `from` key on an `input source` defines a set of previous
  `PipelineTasks` (i.e. the named instance of a `Task`) in the `Pipeline`
- When the `from` key is specified on an input source, the version of the
  resource that is from the defined list of tasks is used
- `from` can support fan in and fan out
- The `from` clause [expresses ordering](#ordering), i.e. the
  [Pipeline Task](#pipeline-tasks) which provides the `PipelineResource` must run
  _before_ the Pipeline Task which needs that `PipelineResource` as an input
  - The name of the `PipelineResource` must correspond to a `PipelineResource`
    from the `Task` that the referenced `PipelineTask` gives as an output

For example see this `Pipeline` spec:

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

The resource `my-image` is expected to be given to the `deploy-app` `Task` from
the `build-app` `Task`. This means that the `PipelineResource` `my-image` must
also be declared as an output of `build-app`.

This also means that the `build-app` Pipeline Task will run before `deploy-app`,
regardless of the order they appear in the spec.

#### runAfter

Sometimes you will need to have [Pipeline Tasks](#pipeline-tasks) that need to
run in a certain order, but they do not have an explicit
[output](tasks.md#outputs) to [input](tasks.md#inputs) dependency (which is
expressed via [`from`](#from)). In this case you can use `runAfter` to indicate
that a Pipeline Task should be run after one or more previous Pipeline Tasks.

For example see this `Pipeline` spec:

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

In this `Pipeline`, we want to test the code before we build from it, but there
is no output from `test-app`, so `build-app` uses `runAfter` to indicate that
`test-app` should run before it, regardless of the order they appear in the
spec.

#### retries

Sometimes you need a policy for retrying tasks which have problems such as
network errors, missing dependencies or upload problems. Any of those issues must
be reflected as False (corev1.ConditionFalse) within the TaskRun Status
Succeeded Condition. For that reason there is an optional attribute called
`retries` which declares how many times that task should be retried in case of
failure.

By default and in its absence there are no retries; its value is 0.

```yaml
tasks:
  - name: build-the-image
    retries: 1
    taskRef:
      name: build-push
```

In this example, the task "build-the-image" will be executed and if the first
run fails a second one would triggered. But, if that fails no more would
triggered: a max of two executions.


#### conditions

Sometimes you will need to run tasks only when some conditions are true. The `conditions` field 
allows you to list a series of references to [`Conditions`](./conditions.md) that are run before the task
is run. If all of the conditions evaluate to true, the task is run. If any of the conditions are false,
the Task is not run. Its status.ConditionSucceeded is set to False with the reason set to  `ConditionCheckFailed`.
However, unlike regular task failures, condition failures do not automatically fail the entire pipeline 
run -- other tasks that are not dependent on the task (via `from` or `runAfter`) are still run.

```yaml
tasks:
  - name: conditional-task
    taskRef:
      name: build-push
    conditions:
      - conditionRef: my-condition
        params:
          - name: my-param
            value: my-value
        resources:
          - name: workspace
            resource: source-repo
```

In this example, `my-condition` refers to a [Condition](#conditions) custom resource. The `build-push` 
task will only be executed if the condition evaluates to true. 

Resources in conditions can also use the [`from`](#from) field to indicate that they 
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

## Ordering

The [Pipeline Tasks](#pipeline-tasks) in a `Pipeline` can be connected and run
in a graph, specifically a _Directed Acyclic Graph_ or DAG. Each of the Pipeline
Tasks is a node, which can be connected with an edge (i.e. a _Graph_) such that one will run
before another (i.e. _Directed_), and the execution will eventually complete
(i.e. _Acyclic_, it will not get caught in infinite loops).

This is done using:

- [`from`](#from) clauses on the [`PipelineResources`](resources.md) needed by a
  `Task`
- [`runAfter`](#runAfter) clauses on the [Pipeline Tasks](#pipeline-tasks)

For example see this `Pipeline` spec:

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

This will result in the following execution graph:

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

1. The `lint-repo` and `test-app` Pipeline Tasks will begin executing
   simultaneously. (They have no `from` or `runAfter` clauses.)
1. Once `test-app` completes, both `build-app` and `build-frontend` will begin
   executing simultaneously (both `runAfter` `test-app`).
1. When both `build-app` and `build-frontend` have completed, `deploy-all` will
   execute (it requires `PipelineResources` from both Pipeline Tasks).
1. The entire `Pipeline` will be finished executing after `lint-repo` and
   `deploy-all` have completed.

## Examples

For complete examples, see
[the examples folder](https://github.com/tektoncd/pipeline/tree/master/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
