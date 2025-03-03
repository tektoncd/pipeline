<!--
---
linkTitle: "Tasks"
weight: 201
---
-->

# Tasks

- [Overview](#overview)
- [Configuring a Task](#configuring-a-task)
  - [Steps](#steps)
  - [Specifying Parameters](#specifying-parameters)
  - [Specifying Workspaces](#specifying-workspaces)
  - [Emitting Results](#emitting-results)
  - [Specifying Volumes](#specifying-volumes)
  - [Specifying Step Template](#specifying-step-template)
  - [Specifying Sidecars](#specifying-sidecars)
  - [Adding Description](#adding-description)
  - [Using Variable Substitution](#using-variable-substitution)
  - [Running Step Containers as a Non Root User](#running-step-containers-as-a-non-root-user)

## Overview

A `Task` is a collection of `Steps` that you define and arrange in a specific order
of execution as part of your continuous integration flow. A `Task` executes as a
Pod on your Kubernetes cluster. A `Task` is available within a specific namespace,
while a cluster resolver can be used to access Tasks across the entire cluster.

**Note:** The cluster resolver is the recommended way to access Tasks across the cluster. ClusterTasks are deprecated.

A `Task` declaration includes the following elements:

- [Parameters](#specifying-parameters)
- [Steps](#steps)
- [Workspaces](#specifying-workspaces)
- [Results](#emitting-results)

## Configuring a Task

A `Task` definition supports the following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version. For example,
    `tekton.dev/v1beta1`.
  - [`kind`][kubernetes-overview] - Identifies this resource object as a `Task` object.
  - [`metadata`][kubernetes-overview] - Specifies metadata that uniquely identifies the
    `Task` resource object. For example, a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    this `Task` resource object.
  - [`steps`](#steps) - Specifies one or more container images to run in the `Task`.
- Optional:
  - [`description`](#adding-description) - An informative description of the `Task`.
  - [`params`](#specifying-parameters) - Specifies execution parameters for the `Task`.
  - [`workspaces`](#specifying-workspaces) - Specifies paths to volumes required by the `Task`.
  - [`results`](#emitting-results) - Specifies the names under which `Tasks` write execution results.
  - [`volumes`](#specifying-volumes) - Specifies one or more volumes that will be available to the `Steps` in the `Task`.
  - [`stepTemplate`](#specifying-step-template) - Specifies a `Container` step definition to use as the basis for all `Steps` in the `Task`.
  - [`sidecars`](#specifying-sidecars) - Specifies `Sidecar` containers to run alongside the `Steps` in the `Task`.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

The non-functional example below demonstrates the use of most of the above-mentioned fields:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: example-task-name
spec:
  params:
    - name: pathToDockerFile
      type: string
      description: The path to the dockerfile to build
      default: /workspace/workspace/Dockerfile
    - name: builtImageUrl
      type: string
      description: location to push the built image to
  steps:
    - name: ubuntu-example
      image: ubuntu
      args: ["ubuntu-build-example", "SECRETS-example.md"]
    - image: gcr.io/example-builders/build-example
      command: ["echo"]
      args: ["$(params.pathToDockerFile)"]
    - name: dockerfile-pushexample
      image: gcr.io/example-builders/push-example
      args: ["push", "$(params.builtImageUrl)"]
      volumeMounts:
        - name: docker-socket-example
          mountPath: /var/run/docker.sock
  volumes:
    - name: example-volume
      emptyDir: {}
```

### Steps

A `Step` is a reference to a container image that executes a specific tool on a
specific input and produces a specific output. To add `Steps` to a `Task` you
define a `steps` field (required) containing a list of desired `Steps`. The order in
which the `Steps` appear in this list is the order in which they will execute.

The following requirements apply to each container image referenced in a `steps` field:

- The container image must abide by the [container contract](./container-contract.md).
- Each container image runs to completion or until the first failure occurs.
- The CPU, memory, and ephemeral storage resource requests set on `Step`s
  will be adjusted to comply with any [`LimitRange`](https://kubernetes.io/docs/concepts/policy/limit-range/)s
  present in the `Namespace`. In addition, Kubernetes determines a pod's effective resource
  requests and limits by summing the requests and limits for all its containers, even
  though Tekton runs `Steps` sequentially.
  For more detail, see [Compute Resources in Tekton](./compute-resources.md).

**Note:** If the image referenced in the `step` field is from a private registry, `TaskRuns` or `PipelineRuns` that consume the task
          must provide the `imagePullSecrets` in a [podTemplate](./podtemplates.md).

Below is an example of setting the resource requests and limits for a step:


{{< tabs >}}
{{% tab header="v1" %}}
```yaml
spec:
  steps:
    - name: step-with-limts
      computeResources:
        requests:
          memory: 1Gi
          cpu: 500m
        limits:
          memory: 2Gi
          cpu: 800m
```
{{% /tab %}}

{{% tab header="v1beta1" %}}
```yaml
spec:
  steps:
    - name: step-with-limts
      resources:
        requests:
          memory: 1Gi
          cpu: 500m
        limits:
          memory: 2Gi
          cpu: 800m
```
{{% /tab %}}
{{< /tabs >}}

#### Specifying Parameters

You can specify parameters, such as compilation flags or artifact names, that you want to supply to the `Task` at execution time.
 `Parameters` are passed to the `Task` from its corresponding `TaskRun`.

#### Parameter name
Parameter name format:
- Must only contain alphanumeric characters, hyphens (`-`), underscores (`_`), and dots (`.`). However, `object` parameter name and its key names can't contain dots (`.`). See the reasons in the third item added in this [PR](https://github.com/tektoncd/community/pull/711).
- Must begin with a letter or an underscore (`_`).

For example, `foo.Is-Bar_` is a valid parameter name for string or array type, but is invalid for object parameter because it contains dots. On the other hand, `barIsBa$` or `0banana` are invalid for all types.

> NOTE:
> 1. Parameter names are **case insensitive**. For example, `APPLE` and `apple` will be treated as equal. If they appear in the same TaskSpec's params, it will be rejected as invalid.
> 2. If a parameter name contains dots (.), it must be referenced by using the [bracket notation](#using-variable-substitution) with either single or double quotes i.e. `$(params['foo.bar'])`, `$(params["foo.bar"])`. See the following example for more information.

#### Parameter type
Each declared parameter has a `type` field, which can be set to `string`, `array` or `object`.

##### `object` type

`object` type is useful in cases where users want to group related parameters. For example, an object parameter called `gitrepo` can contain both the `url` and the `commmit` to group related information:

```yaml
spec:
  params:
    - name: gitrepo
      type: object
      properties:
        url:
          type: string
        commit:
          type: string
```

Refer to the [TaskRun example](../examples/v1/taskruns/object-param-result.yaml) and the [PipelineRun example](../examples/v1/pipelineruns/pipeline-object-param-and-result.yaml) in which `object` parameters are demonstrated.

  > NOTE:
  > - `object` param must specify the `properties` section to define the schema i.e. what keys are available for this object param. See how to define `properties` section in the following example and the [TEP-0075](https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md#defaulting-to-string-types-for-values).
  > - When providing value for an `object` param, one may provide values for just a subset of keys in spec's `default`, and provide values for the rest of keys at runtime ([example](../examples/v1/taskruns/object-param-result.yaml)).
  > - When using object in variable replacement, users can only access its individual key ("child" member) of the object by its name i.e. `$(params.gitrepo.url)`. Using an entire object as a value is only allowed when the value is also an object like [this example](../examples/v1/pipelineruns/pipeline-object-param-and-result.yaml). See more details about using object param from the [TEP-0075](https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md#using-objects-in-variable-replacement).

##### `array` type

`array` type is useful in cases where the number of compilation flags being supplied to a task varies throughout the `Task's` execution.
`array` param can be defined by setting `type` to `array`.  Also, `array` params only supports `string` array i.e.
each array element has to be of type `string`.

```yaml
spec:
  params:
    - name: flags
      type: array
```

##### `string` type

If not specified, the `type` field defaults to `string`. When the actual parameter value is supplied, its parsed type is validated against the `type` field.

The following example illustrates the use of `Parameters` in a `Task`. The `Task` declares 3 input parameters named `gitrepo` (of type `object`), `flags`
(of type `array`) and `someURL` (of type `string`). These parameters are used in the `steps.args` list
  - For `object` parameter, you can only use individual members (aka keys).
  - You can expand parameters of type `array` inside an existing array using the star operator. In this example, `flags` contains the star operator: `$(params.flags[*])`.

**Note:** Input parameter values can be used as variables throughout the `Task` by using [variable substitution](#using-variable-substitution).

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: task-with-parameters
spec:
  params:
    - name: gitrepo
      type: object
      properties:
        url:
          type: string
        commit:
          type: string
    - name: flags
      type: array
    - name: someURL
      type: string
    - name: foo.bar
      description: "the name contains dot character"
      default: "test"
  steps:
    - name: do-the-clone
      image: some-git-image
      args: [
        "-url=$(params.gitrepo.url)",
        "-revision=$(params.gitrepo.commit)"
      ]
    - name: build
      image: my-builder
      args: [
        "build",
        "$(params.flags[*])",
        # It would be equivalent to use $(params["someURL"]) here,
        # which is necessary when the parameter name contains '.'
        # characters (e.g. `$(params["some.other.URL"])`). See the example in step "echo-param"
        'url=$(params.someURL)',
      ]
    - name: echo-param
      image: bash
      args: [
        "echo",
        "$(params['foo.bar'])",
      ]
```

The following `TaskRun` supplies the value for the parameter `gitrepo`, `flags` and `someURL`:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: run-with-parameters
spec:
  taskRef:
    name: task-with-parameters
  params:
    - name: gitrepo
      value:
        url: "abc.com"
        commit: "c12b72"
    - name: flags
      value:
        - "--set"
        - "arg1=foo"
        - "--randomflag"
        - "--someotherflag"
    - name: someURL
      value: "http://google.com"
```

#### Specifying Workspaces

[`Workspaces`](workspaces.md#using-workspaces-in-tasks) allow you to specify
one or more volumes that your `Task` requires during execution. It is recommended that `Tasks` uses **at most**
one writeable `Workspace`. For example:

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

For more information, see [Using `Workspaces` in `Tasks`](workspaces.md#using-workspaces-in-tasks)
and the [`Workspaces` in a `TaskRun`](../examples/v1/taskruns/workspace.yaml) example YAML file.

### Propagated `Workspaces`

Workspaces can be propagated to embedded task specs, not referenced Tasks. For more information, see [Propagated Workspaces](taskruns.md#propagated-workspaces).

### Emitting Results

A Task is able to emit string results that can be viewed by users and passed to other Tasks in a Pipeline. These
results have a wide variety of potential uses. To highlight just a few examples from the Tekton Catalog: the
[`git-clone` Task](https://github.com/tektoncd/catalog/blob/main/task/git-clone/0.1/git-clone.yaml) emits a
cloned commit SHA as a result, the [`generate-build-id` Task](https://github.com/tektoncd/catalog/blob/main/task/generate-build-id/0.1/generate-build-id.yaml)
emits a randomized ID as a result, and the [`kaniko` Task](https://github.com/tektoncd/catalog/tree/main/task/kaniko/0.1)
emits a container image digest as a result. In each case these results convey information for users to see when
looking at their TaskRuns and can also be used in a Pipeline to pass data along from one Task to the next.
`Task` results are best suited for holding small amounts of data, such as commit SHAs, branch names,
ephemeral namespaces, and so on.

To define a `Task's` results, use the `results` field.
In the example below, the `Task` specifies two files in the `results` field:
`current-date-unix-timestamp` and `current-date-human-readable`.

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: print-date
  annotations:
    description: |
      A simple task that prints the date
spec:
  results:
    - name: current-date-unix-timestamp
      description: The current date in unix timestamp format
    - name: current-date-human-readable
      description: The current date in human readable format
  steps:
    - name: print-date-unix-timestamp
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        date +%s | tee $(results.current-date-unix-timestamp.path)
    - name: print-date-human-readable
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        date | tee $(results.current-date-human-readable.path)
```

In this example, [`$(results.name.path)`](https://github.com/tektoncd/pipeline/blob/main/docs/variables.md#variables-available-in-a-task)
is replaced with the path where Tekton will store the Task's results.

When this Task is executed in a TaskRun, the results will appear in the TaskRun's status:


{{< tabs >}}
{{% tab header="v1" %}}
```yaml
apiVersion: tekton.dev/v1
kind: TaskRun
# ...
status:
  # ...
  results:
    - name: current-date-human-readable
      value: |
        Wed Jan 22 19:47:26 UTC 2020
    - name: current-date-unix-timestamp
      value: |
        1579722445
```
{{% /tab %}}

{{% tab header="v1beta1" %}}
```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
# ...
status:
  # ...
  taskResults:
    - name: current-date-human-readable
      value: |
        Wed Jan 22 19:47:26 UTC 2020
    - name: current-date-unix-timestamp
      value: |
        1579722445
```
{{% /tab %}}
{{< /tabs >}}

Tekton does not perform any processing on the contents of results; they are emitted
verbatim from your Task including any leading or trailing whitespace characters. Make sure to write only the
precise string you want returned from your `Task` into the result files that your `Task` creates.

The stored results can be used [at the `Task` level](./pipelines.md#passing-one-tasks-results-into-the-parameters-or-when-expressions-of-another)
or [at the `Pipeline` level](./pipelines.md#emitting-results-from-a-pipeline).

> **Note** Tekton does not enforce Task results unless there is a consumer: when a Task declares a result,
> it may complete successfully even if no result was actually produced. When a Task that declares results is
> used in a Pipeline, and a component of the Pipeline attempts to consume the Task's result, if the result
> was not produced the pipeline will fail. [TEP-0048](https://github.com/tektoncd/community/blob/main/teps/0048-task-results-without-results.md)
> propopses introducing default values for results to help Pipeline authors manage this case.

#### Emitting Object `Results`
Emitting a task result of type `object` is implemented based on the
[TEP-0075](https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md#emitting-object-results).
You can initialize `object` results from a `task` using JSON escaped string. For example, to assign the following data to an object result:

```
{"url":"abc.dev/sampler","digest":"19f02276bf8dbdd62f069b922f10c65262cc34b710eea26ff928129a736be791"}
```

You will need to use escaped JSON to write to pod termination message:

```
{\"url\":\"abc.dev/sampler\",\"digest\":\"19f02276bf8dbdd62f069b922f10c65262cc34b710eea26ff928129a736be791\"}
```

An example of a task definition producing an object result:

```yaml
kind: Task
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
metadata:
  name: write-object
  annotations:
    description: |
      A simple task that writes object
spec:
  results:
    - name: object-results
      type: object
      description: The object results
      properties:
        url:
          type: string
        digest:
          type: string
  steps:
    - name: write-object
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        echo -n "{\"url\":\"abc.dev/sampler\",\"digest\":\"19f02276bf8dbdd62f069b922f10c65262cc34b710eea26ff928129a736be791\"}" | tee $(results.object-results.path)
```

> **Note:**
> -  that the opening and closing braces  are mandatory along with an escaped JSON.
> - object result must specify the `properties` section to define the schema i.e. what keys are available for this object result. Failing to emit keys from the defined object results will result in validation error at runtime.

#### Emitting Array `Results`

Tekton Task also supports defining a result of type `array` and `object` in addition to `string`.
Emitting a task result of type `array` is a `beta` feature implemented based on the
[TEP-0076](https://github.com/tektoncd/community/blob/main/teps/0076-array-result-types.md#emitting-array-results).
You can initialize `array` results from a `task` using JSON escaped string, for example, to assign the following
list of animals to an array result:

```
["cat", "dog", "squirrel"]
```

You will have to initialize the pod termination message as escaped JSON:

```
[\"cat\", \"dog\", \"squirrel\"]
```

An example of a task definition producing an array result with such greetings `["hello", "world"]`:

```yaml
kind: Task
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
metadata:
  name: write-array
  annotations:
    description: |
      A simple task that writes array
spec:
  results:
    - name: array-results
      type: array
      description: The array results
  steps:
    - name: write-array
      image: bash:latest
      script: |
        #!/usr/bin/env bash
        echo -n "[\"hello\",\"world\"]" | tee $(results.array-results.path)
```

**Note** that the opening and closing square brackets are mandatory along with an escaped JSON.

Now, similar to the Go zero-valued slices, an array result is considered as uninitialized (i.e. `nil`) if it's set to an empty
array i.e. `[]`. For example, `echo -n "[]" |  tee $(results.result.path);` is equivalent to `result := []string{}`.
The result initialized in this way will have zero length. And trying to access this array with a star notation i.e.
`$(tasks.write-array-results.results.result[*])` or an element of such array i.e. `$(tasks.write-array-results.results.result[0])`
results in `InvalidTaskResultReference` with `index out of range`.

Depending on your use case, you might have to initialize a result array to the desired length just like using `make()` function in Go.
`make()` function is used to allocate an array and returns a slice of the specified length i.e.
`result := make([]string, 5)` results in `["", "", "", "", ""]`, similarly set the array result to following JSON escaped
expression to allocate an array of size 2:

```
echo -n "[\"\", \"\"]" | tee $(results.array-results.path) # an array of size 2 with empty string
echo -n "[\"first-array-element\", \"\"]" | tee $(results.array-results.path) # an array of size 2 with only first element initialized
echo -n "[\"\", \"second-array-element\"]" | tee $(results.array-results.path) # an array of size 2 with only second element initialized
echo -n "[\"first-array-element\", \"second-array-element\"]" | tee $(results.array-results.path) # an array of size 2 with both elements initialized
```

This is also important to maintain the order of the elements in an array. The order in which the task result was
initialized is the order in which the result is consumed by the dependent tasks. For example, a task is producing
two array results `images` and `configmaps`. The pipeline author can implement deployment by indexing into each array result:

```yaml
    - name: deploy-stage-1
      taskRef:
        name: deploy
      params:
        - name: image
          value: $(tasks.setup.results.images[0])
        - name: configmap
          value: $(tasks.setup.results.configmap[0])
      ...
    - name: deploy-stage-2
      taskRef:
        name: deploy
      params:
        - name: image
          value: $(tasks.setup.results.images[1])
        - name: configmap
          value: $(tasks.setup.results.configmap[1])
```

As a task author, make sure the task array results are initialized accordingly or set to a zero value in case of no
`image` or `configmap` to maintain the order.

**Note**: Tekton uses [termination
messages](https://kubernetes.io/docs/tasks/debug/debug-application/determine-reason-pod-failure/#writing-and-reading-a-termination-message). As
written in
[tektoncd/pipeline#4808](https://github.com/tektoncd/pipeline/issues/4808),
the maximum size of a `Task's` results is limited by the container termination message feature of Kubernetes.
At present, the limit is ["4096 bytes"](https://github.com/kubernetes/kubernetes/blob/96e13de777a9eb57f87889072b68ac40467209ac/pkg/kubelet/container/runtime.go#L632).
This also means that the number of Steps in a Task affects the maximum size of a Result,
as each Step is implemented as a container in the TaskRun's pod.
The more containers we have in our pod, *the smaller the allowed size of each container's
message*, meaning that the **more steps you have in a Task, the smaller the result for each step can be**.
For example, if you have 10 steps, the size of each step's Result will have a maximum of less than 1KB.

If your `Task` writes a large number of small results, you can work around this limitation
by writing each result from a separate `Step` so that each `Step` has its own termination message.
If a termination message is detected as being too large the TaskRun will be placed into a failed state
with the following message: `Termination message is above max allowed size 4096, caused by large task
result`. Since Tekton also uses the termination message for some internal information, so the real
available size will less than 4096 bytes.

As a general rule-of-thumb, if a result needs to be larger than a kilobyte, you should likely use a
[`Workspace`](#specifying-workspaces) to store and pass it between `Tasks` within a `Pipeline`.

#### Larger `Results` using sidecar logs

This is a beta feature which is guarded behind its own feature flag.  The `results-from` feature flag must be set to
[`"sidecar-logs"`](./install.md#enabling-larger-results-using-sidecar-logs) to enable larger results using sidecar logs.

Instead of using termination messages to store results, the taskrun controller injects a sidecar container which monitors
the results of all the steps. The sidecar mounts the volume where results of all the steps are stored. As soon as it
finds a new result, it logs it to std out. The controller has access to the logs of the sidecar container.
**CAUTION**: we need you to enable access to [kubernetes pod/logs](./install.md#enabling-larger-results-using-sidecar-logs).

This feature allows users to store up to 4 KB per result by default. Because we are not limited by the size of the
termination messages, users can have as many results as they require (or until the CRD reaches its limit). If the size
of a result exceeds this limit, then the TaskRun will be placed into a failed state with the following message: `Result
exceeded the maximum allowed limit.`

**Note**: If you require even larger results, you can specify a different upper limit per result by setting
`max-result-size` feature flag to your desired size in bytes ([see instructions](./install.md#enabling-larger-results-using-sidecar-logs)).
**CAUTION**: the larger you make the size, more likely will the CRD reach its max limit enforced by the `etcd` server
leading to bad user experience.

Refer to the detailed instructions listed in [additional config](additional-configs.md#enabling-larger-results-using-sidecar-logs)
to learn how to enable this feature.

### Specifying Volumes

Specifies one or more [`Volumes`](https://kubernetes.io/docs/concepts/storage/volumes/) that the `Steps` in your
`Task` require to execute in addition to volumes that are implicitly created for input and output resources.

For example, you can use `Volumes` to do the following:

- [Mount a Kubernetes `Secret`](auth.md).
- Create an `emptyDir` persistent `Volume` that caches data across multiple `Steps`.
- Mount a [Kubernetes `ConfigMap`](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/)
  as `Volume` source.
- Mount a host's Docker socket to use a `Dockerfile` for building container images.
  **Note:** Building a container image on-cluster using `docker build` is **very
  unsafe** and is mentioned only for the sake of the example. Use [kaniko](https://github.com/GoogleContainerTools/kaniko) instead.

### Specifying Step Template

The `stepTemplate` field specifies a [`Container`](https://kubernetes.io/docs/concepts/containers/)
configuration that will be used as the starting point for all of the `Steps` in your
`Task`. Individual configurations specified within `Steps` supersede the template wherever
overlap occurs.

In the example below, the `Task` specifies a `stepTemplate` field with the environment variable
`FOO` set to `bar`. The first `Step` in the `Task` uses that value for `FOO`, but the second `Step`
overrides the value set in the template with `baz`. Additional, the `Task` specifies a `stepTemplate`
field with the environment variable `TOKEN` set to `public`. The last one `Step` in the `Task` uses
`private` in the referenced secret to override the value set in the template.

```yaml
stepTemplate:
  env:
    - name: "FOO"
      value: "bar"
    - name: "TOKEN"
      value: "public"
steps:
  - image: ubuntu
    command: [echo]
    args: ["FOO is ${FOO}"]
  - image: ubuntu
    command: [echo]
    args: ["FOO is ${FOO}"]
    env:
      - name: "FOO"
        value: "baz"
  - image: ubuntu
    command: [echo]
    args: ["TOKEN is ${TOKEN}"]
    env:
      - name: "TOKEN"
        valueFrom:
          secretKeyRef:
            key: "token"
            name: "test"
---
# The secret 'test' part data is as follows.
data:
  # The decoded value of 'cHJpdmF0ZQo=' is 'private'.
  token: "cHJpdmF0ZQo="
```

### Specifying Sidecars

The `sidecars` field specifies a list of [`Containers`](https://kubernetes.io/docs/concepts/containers/)
to run alongside the `Steps` in your `Task`. You can use `Sidecars` to provide auxiliary functionality, such as
[Docker in Docker](https://hub.docker.com/_/docker) or running a mock API server that your app can hit during testing.
`Sidecars` spin up before your `Task` executes and are deleted after the `Task` execution completes.
For further information, see [`Sidecars` in `TaskRuns`](taskruns.md#specifying-sidecars).

**Note**: Starting in v0.62 you can enable native Kubernetes sidecar support using the `enable-kubernetes-sidecar` feature flag ([see instructions](./additional-configs.md#customizing-the-pipelines-controller-behavior)). If kubernetes does not wait for your sidecar application to be ready, use a `startupProbe` to help kubernetes identify when it is ready.

Refer to the detailed instructions listed in [additional config](additional-configs.md#enabling-larger-results-using-sidecar-logs)
to learn how to enable this feature.

In the example below, a `Step` uses a Docker-in-Docker `Sidecar` to build a Docker image:

```yaml
steps:
  - image: docker
    name: client
    script: |
      #!/usr/bin/env bash
      cat > Dockerfile << EOF
      FROM ubuntu
      RUN apt-get update
      ENTRYPOINT ["echo", "hello"]
      EOF
      docker build -t hello . && docker run hello
      docker images
    volumeMounts:
      - mountPath: /var/run/
        name: dind-socket
sidecars:
  - image: docker:18.05-dind
    name: server
    securityContext:
      privileged: true
    volumeMounts:
      - mountPath: /var/lib/docker
        name: dind-storage
      - mountPath: /var/run/
        name: dind-socket
volumes:
  - name: dind-storage
    emptyDir: {}
  - name: dind-socket
    emptyDir: {}
```

Sidecars, just like `Steps`, can also run scripts:

```yaml
sidecars:
  - image: busybox
    name: hello-sidecar
    script: |
      echo 'Hello from sidecar!'
```
**Note:** Tekton's current `Sidecar` implementation contains a bug.
Tekton uses a container image named `nop` to terminate `Sidecars`.
That image is configured by passing a flag to the Tekton controller.
If the configured `nop` image contains the exact command the `Sidecar`
was executing before receiving a "stop" signal, the `Sidecar` keeps
running, eventually causing the `TaskRun` to time out with an error.
For more information, see [issue 1347](https://github.com/tektoncd/pipeline/issues/1347).

### Adding Description

The `description` field is an optional field that allows you to add an informative description to the `Task`.

### Using Variable Substitution

Tekton provides variables to inject values into the contents of certain fields.
The values you can inject come from a range of sources including other fields
in the Task, context-sensitive information that Tekton provides, and runtime
information received from a TaskRun.

The mechanism of variable substitution is quite simple - string replacement is
performed by the Tekton Controller when a TaskRun is executed.

`Tasks` allow you to substitute variable names for the following entities:

- [Parameters and resources](#using-variable-substitution)
- [`Array` parameters](#using-variable-substitution)
- [`Workspaces`](#using-variable-substitution)
- [`Volume` names and types](#using-variable-substitution)

See the [complete list of variable substitutions for Tasks](./variables.md#variables-available-in-a-task)
and the [list of fields that accept substitutions](./variables.md#fields-that-accept-variable-substitutions).

#### Using Variable Substitution

[`params`](#specifying-parameters) and [`resources`](#specifying-resources) attributes can replace
variable values as follows:

- To reference a parameter in a `Task`, use the following syntax, where `<name>` is the name of the parameter:
  ```shell
  # dot notation
  # Here, the name cannot contain dots (eg. foo.bar is not allowed). If the name contains `dots`, it can only be accessed via the bracket notation.
  $(params.<name> )
  # or bracket notation (wrapping <name> with either single or double quotes):
  # Here, the name can contain dots (eg. foo.bar is allowed).
  $(params['<name>'])
  $(params["<name>"])
  ```
- To access parameter values from resources, see [variable substitution](resources.md#variable-substitution)

## Code examples

Study the following code examples to better understand how to configure your `Tasks`:

- [Building and pushing a Docker image](#building-and-pushing-a-docker-image)
- [Mounting multiple `Volumes`](#mounting-multiple-volumes)
- [Mounting a `ConfigMap` as a `Volume` source](#mounting-a-configmap-as-a-volume-source)
- [Using a `Secret` as an environment source](#using-a-secret-as-an-environment-source)
- [Using a `Sidecar` in a `Task`](#using-a-sidecar-in-a-task)

_Tip: See the collection of Tasks in the
[Tekton community catalog](https://github.com/tektoncd/catalog) for
more examples.

### Building and pushing a Docker image

The following example `Task` builds and pushes a `Dockerfile`-built image.

**Note:** Building a container image using `docker build` on-cluster is **very
unsafe** and is shown here only as a demonstration. Use [kaniko](https://github.com/GoogleContainerTools/kaniko) instead.

```yaml
spec:
  params:
    # This may be overridden, but is a sensible default.
    - name: dockerfileName
      type: string
      description: The name of the Dockerfile
      default: Dockerfile
    - name: image
      type: string
      description: The image to build and push
  workspaces:
  - name: source
  steps:
    - name: dockerfile-build
      image: gcr.io/cloud-builders/docker
      workingDir: "$(workspaces.source.path)"
      args:
        [
          "build",
          "--no-cache",
          "--tag",
          "$(params.image)",
          "--file",
          "$(params.dockerfileName)",
          ".",
        ]
      volumeMounts:
        - name: docker-socket
          mountPath: /var/run/docker.sock

    - name: dockerfile-push
      image: gcr.io/cloud-builders/docker
      args: ["push", "$(params.image)"]
      volumeMounts:
        - name: docker-socket
          mountPath: /var/run/docker.sock

  # As an implementation detail, this Task mounts the host's daemon socket.
  volumes:
    - name: docker-socket
      hostPath:
        path: /var/run/docker.sock
        type: Socket
```

#### Mounting multiple `Volumes`

The example below illustrates mounting multiple `Volumes`:

```yaml
spec:
  steps:
    - image: ubuntu
      script: |
        #!/usr/bin/env bash
        curl https://foo.com > /var/my-volume
      volumeMounts:
        - name: my-volume
          mountPath: /var/my-volume

    - image: ubuntu
      script: |
        #!/usr/bin/env bash
        cat /etc/my-volume
      volumeMounts:
        - name: my-volume
          mountPath: /etc/my-volume

  volumes:
    - name: my-volume
      emptyDir: {}
```

#### Mounting a `ConfigMap` as a `Volume` source

The example below illustrates how to mount a `ConfigMap` to act as a `Volume` source:

```yaml
spec:
  params:
    - name: CFGNAME
      type: string
      description: Name of config map
    - name: volumeName
      type: string
      description: Name of volume
  steps:
    - image: ubuntu
      script: |
        #!/usr/bin/env bash
        cat /var/configmap/test
      volumeMounts:
        - name: "$(params.volumeName)"
          mountPath: /var/configmap

  volumes:
    - name: "$(params.volumeName)"
      configMap:
        name: "$(params.CFGNAME)"
```

#### Using a `Secret` as an environment source

The example below illustrates how to use a `Secret` as an environment source:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: goreleaser
spec:
  params:
    - name: package
      type: string
      description: base package to build in
    - name: github-token-secret
      type: string
      description: name of the secret holding the github-token
      default: github-token
  workspaces:
  - name: source
  steps:
    - name: release
      image: goreleaser/goreleaser
      workingDir: $(workspaces.source.path)/$(params.package)
      command:
        - goreleaser
      args:
        - release
      env:
        - name: GOPATH
          value: /workspace
        - name: GITHUB_TOKEN
          valueFrom:
            secretKeyRef:
              name: $(params.github-token-secret)
              key: bot-token
```

#### Using a `Sidecar` in a `Task`

The example below illustrates how to use a `Sidecar` in your `Task`:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: with-sidecar-task
spec:
  params:
    - name: sidecar-image
      type: string
      description: Image name of the sidecar container
    - name: sidecar-env
      type: string
      description: Environment variable value
  sidecars:
    - name: sidecar
      image: $(params.sidecar-image)
      env:
        - name: SIDECAR_ENV
          value: $(params.sidecar-env)
  steps:
    - name: test
      image: hello-world
```

## Debugging

This section describes techniques for debugging the most common issues in `Tasks`.

### Inspecting the file structure

A common issue when configuring `Tasks` stems from not knowing the location of your data.
For the most part, files ingested and output by your `Task` live in the `/workspace` directory,
but the specifics can vary. To inspect the file structure of your `Task`, add a step that outputs
the name of every file stored in the `/workspace` directory to the build log. For example:

```yaml
- name: build-and-push-1
  image: ubuntu
  command:
    - /bin/bash
  args:
    - -c
    - |
      set -ex
      find /workspace
```

You can also choose to examine the *contents* of every file used by your `Task`:

```yaml
- name: build-and-push-1
  image: ubuntu
  command:
    - /bin/bash
  args:
    - -c
    - |
      set -ex
      find /workspace | xargs cat
```

### Inspecting the `Pod`

To inspect the contents of the `Pod` used by your `Task` at a specific stage in the `Task's` execution,
log into the `Pod` and add a `Step` that pauses the `Task` at the desired stage. For example:

```yaml
- name: pause
  image: docker
  args: ["sleep", "6000"]
```

### Running Step Containers as a Non Root User

All steps that do not require to be run as a root user should make use of TaskRun features to
designate the container for a step runs as a user without root permissions. As a best practice,
running containers as non root should be built into the container image to avoid any possibility
of the container being run as root. However, as a further measure of enforcing this practice,
steps can make use of a `securityContext` to specify how the container should run.

An example of running Task steps as a non root user is shown below:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: show-non-root-steps
spec:
  steps:
    # no securityContext specified so will use
    # securityContext from TaskRun podTemplate
    - name: show-user-1001
      image: ubuntu
      command:
        - ps
      args:
        - "aux"
    # securityContext specified so will run as
    # user 2000 instead of 1001
    - name: show-user-2000
      image: ubuntu
      command:
        - ps
      args:
        - "aux"
      securityContext:
        runAsUser: 2000
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: TaskRun
metadata:
  generateName: show-non-root-steps-run-
spec:
  taskRef:
    name: show-non-root-steps
  podTemplate:
    securityContext:
      runAsNonRoot: true
      runAsUser: 1001
```

In the example above, the step `show-user-2000` specifies via a `securityContext` that the container
for the step should run as user 2000. A `securityContext` must still be specified via a TaskRun `podTemplate`
for this TaskRun to run in a Kubernetes environment that enforces running containers as non root as a requirement.

The `runAsNonRoot` property specified via the `podTemplate` above validates that steps part of this TaskRun are
running as non root users and will fail to start any step container that attempts to run as root. Only specifying
`runAsNonRoot: true` will not actually run containers as non root as the property simply validates that steps are not
running as root. It is the `runAsUser` property that is actually used to set the non root user ID for the container.

If a step defines its own `securityContext`, it will be applied for the step container over the `securityContext`
specified at the pod level via the TaskRun `podTemplate`.

More information about Pod and Container Security Contexts can be found via the [Kubernetes website](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).

The example Task/TaskRun above can be found as a [TaskRun example](../examples/v1/taskruns/run-steps-as-non-root.yaml).

## `Task` Authoring Recommendations

Recommendations for authoring `Tasks` are available in the [Tekton Catalog][recommendations].

[recommendations]: https://github.com/tektoncd/catalog/blob/main/recommendations.md

---

Except as otherwise noted, the contents of this page are licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/).
Code samples are licensed under the [Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
