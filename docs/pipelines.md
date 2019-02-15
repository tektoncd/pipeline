# Pipelines

This document defines `Pipelines` and their capabilities.

---

- [Syntax](#syntax)
  - [Declared resources](#declared-resources)
  - [Parameters](#parameters)
  - [Pipeline Tasks](#pipeline-tasks)
    - [From](#from)
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

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Declared resources

In order for a `Pipeline` to interact with the outside world, it will probably
need [`PipelineResources`](#creating-pipelineresources) which will be given to
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

### Parameters

`Pipeline`s can declare input parameters that must be supplied to the `Pipeline`
during a `PipelineRun`. Pipeline parameters can be used to replace template
values in [`PipelineTask` parameters' values](#pipeline-tasks).

Parameters name are limited to alpha-numeric characters, `-` and `_` and can
only start with alpha characters and `_`. For example, `fooIs-Bar_` is a valid
parameter name, `barIsBa$` or `0banana` are not.

#### Usage

The following example shows how `Pipeline`s can be parameterized, and these
parameters can be passed to the `Pipeline` from a `PipelineRun`.

Input parameters in the form of `${params.foo}` are replaced inside of the
[`PipelineTask` parameters' values](#pipeline-tasks) (see also
[templating](tasks.md#templating)).

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
          value: "${params.context}"
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

A `Pipeline` will execute a sequence of [`Tasks`](tasks.md) in the order they
are declared in. At a minimum, this declaration must include a reference to the
`Task`:

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

Sometimes you will have `Tasks` that need to take as input the output of a
previous `Task`, for example, an image built by a previous `Task`.

Express this dependency by adding `from` on `Resources` that your `Tasks` need.

- The (optional) `from` key on an `input source` defines a set of previous
  `PipelineTasks` (i.e. the named instance of a `Task`) in the `Pipeline`
- When the `from` key is specified on an input source, the version of the
  resource that is from the defined list of tasks is used
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
      - name: my-image
        from:
          - build-app
```

The resource `my-image` is expected to be given to the `deploy-app` `Task` from
the `build-app` `Task`. This means that the `PipelineResource` `my-image` must
also be declared as an output of `build-app`.

## Examples

For complete examples, see
[the examples folder](https://github.com/knative/build-pipeline/tree/master/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
