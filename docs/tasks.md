<!--
---
linkTitle: "Tasks"
weight: 201
---
-->

# Tasks

- [Overview](#overview)
- [Configuring a `Task`](#configuring-a-task)
  - [`Task` vs. `ClusterTask`](#task-vs-clustertask)
  - [Defining `Steps`](#defining-steps)
    - [Reserved directories](#reserved-directories)
    - [Running scripts within `Steps`](#running-scripts-within-steps)
      - [Windows scripts](#windows-scripts)
    - [Specifying a timeout](#specifying-a-timeout)
    - [Specifying `onError` for a `step`](#specifying-onerror-for-a-step)
    - [Accessing Step's `exitCode` in subsequent `Steps`](#accessing-steps-exitcode-in-subsequent-steps)
    - [Produce a task result with `onError`](#produce-a-task-result-with-onerror)
    - [Breakpoint on failure with `onError`](#breakpoint-on-failure-with-onerror)
    - [Redirecting step output streams with `stdoutConfig` and `stderrConfig`](#redirecting-step-output-streams-with-stdoutConfig-and-stderrConfig)
  - [Specifying `Parameters`](#specifying-parameters)
  - [Specifying `Workspaces`](#specifying-workspaces)
  - [Emitting `Results`](#emitting-results)
    - [Larger `Results` using sidecar logs](#larger-results-using-sidecar-logs)
  - [Specifying `Volumes`](#specifying-volumes)
  - [Specifying a `Step` template](#specifying-a-step-template)
  - [Specifying `Sidecars`](#specifying-sidecars)
  - [Specifying a `DisplayName`](#specifying-a-display-name)
  - [Adding a description](#adding-a-description)
  - [Using variable substitution](#using-variable-substitution)
    - [Substituting parameters and resources](#substituting-parameters-and-resources)
    - [Substituting `Array` parameters](#substituting-array-parameters)
    - [Substituting `Workspace` paths](#substituting-workspace-paths)
    - [Substituting `Volume` names and types](#substituting-volume-names-and-types)
    - [Substituting in `Script` blocks](#substituting-in-script-blocks)
- [Code examples](#code-examples)
  - [Building and pushing a Docker image](#building-and-pushing-a-docker-image)
    - [Mounting multiple `Volumes`](#mounting-multiple-volumes)
    - [Mounting a `ConfigMap` as a `Volume` source](#mounting-a-configmap-as-a-volume-source)
    - [Using a `Secret` as an environment source](#using-a-secret-as-an-environment-source)
    - [Using a `Sidecar` in a `Task`](#using-a-sidecar-in-a-task)
- [Debugging](#debugging)
  - [Inspecting the file structure](#inspecting-the-file-structure)
  - [Inspecting the `Pod`](#inspecting-the-pod)
  - [Running Step Containers as a Non Root User](#running-step-containers-as-a-non-root-user)
- [`Task` Authoring Recommendations](#task-authoring-recommendations)

## Overview

A `Task` is a collection of `Steps` that you
define and arrange in a specific order of execution as part of your continuous integration flow.
A `Task` executes as a Pod on your Kubernetes cluster. A `Task` is available within a specific
namespace, while a `ClusterTask` is available across the entire cluster.

A `Task` declaration includes the following elements:

- [Parameters](#specifying-parameters)
- [Steps](#defining-steps)
- [Workspaces](#specifying-workspaces)
- [Results](#emitting-results)

## Configuring a `Task`

A `Task` definition supports the following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version. For example,
    `tekton.dev/v1beta1`.
  - [`kind`][kubernetes-overview] - Identifies this resource object as a `Task` object.
  - [`metadata`][kubernetes-overview] - Specifies metadata that uniquely identifies the
    `Task` resource object. For example, a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    this `Task` resource object.
  - [`steps`](#defining-steps) - Specifies one or more container images to run in the `Task`.
- Optional:
  - [`description`](#adding-a-description) - An informative description of the `Task`.
  - [`params`](#specifying-parameters) - Specifies execution parameters for the `Task`.
  - [`workspaces`](#specifying-workspaces) - Specifies paths to volumes required by the `Task`.
  - [`results`](#emitting-results) - Specifies the names under which `Tasks` write execution results.
  - [`volumes`](#specifying-volumes) - Specifies one or more volumes that will be available to the `Steps` in the `Task`.
  - [`stepTemplate`](#specifying-a-step-template) - Specifies a `Container` step definition to use as the basis for all `Steps` in the `Task`.
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

### `Task` vs. `ClusterTask`

**Note: ClusterTasks are deprecated.** Please use the [cluster resolver](./cluster-resolver.md) instead.

A `ClusterTask` is a `Task` scoped to the entire cluster instead of a single namespace.
A `ClusterTask` behaves identically to a `Task` and therefore everything in this document
applies to both.

**Note:** When using a `ClusterTask`, you must explicitly set the `kind` sub-field in the `taskRef` field to `ClusterTask`.
          If not specified, the `kind` sub-field defaults to `Task.`

Below is an example of a Pipeline declaration that uses a `ClusterTask`:
**Note**: 
- There is no `v1` API specification for `ClusterTask` but a `v1beta1 clustertask` can still be referenced in a `v1 pipeline`.
- The cluster resolver syntax below can be used to reference any task, not just a clustertask.

{{< tabs >}}
{{% tab header="v1 & v1beta1" %}}
```yaml
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: demo-pipeline
spec:
  tasks:
    - name: build-skaffold-web
      taskRef:
        resolver: cluster
        params:
        - name: kind
          value: task
        - name: name
          value: build-push
        - name: namespace
          value: default
```
{{% /tab %}}

{{% tab header="v1beta1" %}}
```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: demo-pipeline
  namespace: default
spec:
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-push
        kind: ClusterTask
      params: ....
```
{{% /tab %}}
{{< /tabs >}}

### Defining `Steps`

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

#### Reserved directories

There are several directories that all `Tasks` run by Tekton will treat as special

* `/workspace` - This directory is where [resources](#specifying-resources) and [workspaces](#specifying-workspaces)
  are mounted. Paths to these are available to `Task` authors via [variable substitution](variables.md)
* `/tekton` - This directory is used for Tekton specific functionality:
    * `/tekton/results` is where [results](#emitting-results) are written to.
      The path is available to `Task` authors via [`$(results.name.path)`](variables.md)
    * There are other subfolders which are [implementation details of Tekton](developers/README.md#reserved-directories)
      and **users should not rely on their specific behavior as it may change in the future**

#### Running scripts within `Steps`

A step can specify a `script` field, which contains the body of a script. That script is
invoked as if it were stored inside the container image, and any `args` are passed directly
to it.

**Note:** If the `script` field is present, the step cannot also contain a `command` field.

Scripts that do not start with a [shebang](https://en.wikipedia.org/wiki/Shebang_(Unix))
line will have the following default preamble prepended:

```bash
#!/bin/sh
set -e
```

You can override this default preamble by prepending a shebang that specifies the desired parser.
This parser must be present within that `Step's` container image.

The example below executes a Bash script:

```yaml
steps:
  - image: ubuntu # contains bash
    script: |
      #!/usr/bin/env bash
      echo "Hello from Bash!"
```

The example below executes a Python script:

```yaml
steps:
  - image: python # contains python
    script: |
      #!/usr/bin/env python3
      print("Hello from Python!")
```

The example below executes a Node script:

```yaml
steps:
  - image: node # contains node
    script: |
      #!/usr/bin/env node
      console.log("Hello from Node!")
```

You can execute scripts directly in the workspace:

```yaml
steps:
  - image: ubuntu
    script: |
      #!/usr/bin/env bash
      /workspace/my-script.sh  # provided by an input resource
```

You can also execute scripts within the container image:

```yaml
steps:
  - image: my-image # contains /bin/my-binary
    script: |
      #!/usr/bin/env bash
      /bin/my-binary
```

##### Windows scripts

Scripts in tasks that will eventually run on windows nodes need a custom shebang line, so that Tekton knows how to run the script. The format of the shebang line is:

`#!win <interpreter command> <args>`

Unlike linux, we need to specify how to interpret the script file which is generated by Tekton. The example below shows how to execute a powershell script:

```yaml
steps:
  - image: mcr.microsoft.com/windows/servercore:1809
    script: |
      #!win powershell.exe -File
      echo 'Hello from PowerShell'
```

Microsoft provide `powershell` images, which contain Powershell Core (which is slightly different from powershell found in standard windows images). The example below shows how to use these images:
```yaml
steps:
  - image: mcr.microsoft.com/powershell:nanoserver
    script: |
      #!win pwsh.exe -File
      echo 'Hello from PowerShell Core'
```

As can be seen the command is different. The windows shebang can be used for any interpreter, as long as it exists in the image and can interpret commands from a file. The example below executes a Python script:
```yaml
  steps:
  - image: python
    script: |
      #!win python
      print("Hello from Python!")
```
Note that other than the `#!win` shebang the example is identical to the earlier linux example.

Finally, if no interpreter is specified on the `#!win` line then the script will be treated as a windows `.cmd` file which will be excecuted. The example below shows this:
```yaml
  steps:
  - image: mcr.microsoft.com/powershell:lts-nanoserver-1809
    script: |
      #!win
      echo Hello from the default cmd file
```

#### Specifying a timeout

A `Step` can specify a `timeout` field.
If the `Step` execution time exceeds the specified timeout, the `Step` kills
its running process and any subsequent `Steps` in the `TaskRun` will not be
executed. The `TaskRun` is placed into a `Failed` condition.  An accompanying log
describing which `Step` timed out is written as the `Failed` condition's message.

The timeout specification follows the duration format as specified in the [Go time package](https://golang.org/pkg/time/#ParseDuration) (e.g. 1s or 1ms).

The example `Step` below is supposed to sleep for 60 seconds but will be canceled by the specified 5 second timeout.
```yaml
steps:
  - name: sleep-then-timeout
    image: ubuntu
    script: |
      #!/usr/bin/env bash
      echo "I am supposed to sleep for 60 seconds!"
      sleep 60
    timeout: 5s
```

#### Specifying `onError` for a `step`

When a `step` in a `task` results in a failure, the rest of the steps in the `task` are skipped and the `taskRun` is
declared a failure. If you would like to ignore such step errors and continue executing the rest of the steps in
the task, you can specify `onError` for such a `step`.

`onError` can be set to either `continue` or `stopAndFail` as part of the step definition. If `onError` is
set to `continue`, the entrypoint sets the original failed exit code of the [script](#running-scripts-within-steps)
in the container terminated state. A `step` with `onError` set to `continue` does not fail the `taskRun` and continues
executing the rest of the steps in a task.

To ignore a step error, set `onError` to `continue`:

```yaml
steps:
  - image: docker.io/library/golang:latest
    name: ignore-unit-test-failure
    onError: continue
    script: |
      go test .
```

The original failed exit code of the [script](#running-scripts-within-steps) is available in the terminated state of
the container.

```
kubectl get tr taskrun-unit-test-t6qcl -o json | jq .status
{
  "conditions": [
    {
      "message": "All Steps have completed executing",
      "reason": "Succeeded",
      "status": "True",
      "type": "Succeeded"
    }
  ],
  "steps": [
    {
      "container": "step-ignore-unit-test-failure",
      "imageID": "...",
      "name": "ignore-unit-test-failure",
      "terminated": {
        "containerID": "...",
        "exitCode": 1,
        "reason": "Completed",
      }
    },
  ],
```

For an end-to-end example, see [the taskRun ignoring a step error](../examples/v1beta1/taskruns/ignore-step-error.yaml)
and [the pipelineRun ignoring a step error](../examples/v1beta1/pipelineruns/ignore-step-error.yaml).

#### Accessing Step's `exitCode` in subsequent `Steps`

A step can access the exit code of any previous step by reading the file pointed to by the `exitCode` path variable:

```shell
cat $(steps.step-<step-name>.exitCode.path)
```

The `exitCode` of a step without any name can be referenced using:

```shell
cat $(steps.step-unnamed-<step-index>.exitCode.path)
```

#### Produce a task result with `onError`

When a step is set to ignore the step error and if that step is able to initialize a result file before failing,
that result is made available to its consumer task.

```yaml
steps:
  - name: ignore-failure-and-produce-a-result
    onError: continue
    image: busybox
    script: |
      echo -n 123 | tee $(results.result1.path)
      exit 1
```

The task consuming the result using the result reference `$(tasks.task1.results.result1)` in a `pipeline` will be able
to access the result and run with the resolved value.

Now, a step can fail before initializing a result and the `pipeline` can ignore such step failure. But, the  `pipeline`
will fail with `InvalidTaskResultReference` if it has a task consuming that task result. For example, any task
consuming `$(tasks.task1.results.result2)` will cause the pipeline to fail.

```yaml
steps:
  - name: ignore-failure-and-produce-a-result
    onError: continue
    image: busybox
    script: |
      echo -n 123 | tee $(results.result1.path)
      exit 1
      echo -n 456 | tee $(results.result2.path)
```

#### Breakpoint on failure with `onError`

[Debugging](taskruns.md#debugging-a-taskrun) a taskRun is supported to debug a container and comes with a set of
[tools](taskruns.md#debug-environment) to declare the step as a failure or a success. Specifying
[breakpoint](taskruns.md#breakpoint-on-failure) at the `taskRun` level overrides ignoring a step error using `onError`.

#### Redirecting step output streams with `stdoutConfig` and `stderrConfig`

This is an alpha feature. The `enable-api-fields` feature flag [must be set to `"alpha"`](./install.md)
for Redirecting Step Output Streams to function.

This feature defines optional `Step` fields `stdoutConfig` and `stderrConfig` which can be used to redirection the output streams `stdout` and `stderr` respectively:

```yaml
- name: ...
  ...
  stdoutConfig:
    path: ...
  stderrConfig:
    path: ...
```

Once `stdoutConfig.path` or `stderrConfig.path` is specified, the corresponding output stream will be duplicated to both the given file and the standard output stream of the container, so users can still view the output through the Pod log API. If both `stdoutConfig.path` and `stderrConfig.path` are set to the same value, outputs from both streams will be interleaved in the same file, but there will be no ordering guarantee on the data. If multiple `Step`'s `stdoutConfig.path` fields are set to the same value, the file content will be overwritten by the last outputting step.

Variable substitution will be applied to the new fields, so one could specify `$(results.<name>.path)` to the `stdoutConfig.path` or `stderrConfig.path` field to extract the stdout of a step into a Task result.

##### Example Usage

Redirecting stdout of `boskosctl` to `jq` and publish the resulting `project-id` as a Task result:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: boskos-acquire
spec:
  results:
  - name: project-id
  steps:
  - name: boskosctl
    image: gcr.io/k8s-staging-boskos/boskosctl
    args:
    - acquire
    - --server-url=http://boskos.test-pods.svc.cluster.local
    - --owner-name=christie-test-boskos
    - --type=gke-project
    - --state=free
    - --target-state=busy
    stdoutConfig:
      path: /data/boskosctl-stdout
    volumeMounts:
    - name: data
      mountPath: /data
  - name: parse-project-id
    image: imega/jq
    args:
    - -r
    - .name
    - /data/boskosctl-stdout
    stdoutConfig:
      path: $(results.project-id.path)
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
  ```

> NOTE:
>
> - If the intent is to share output between `Step`s via a file, the user must ensure that the paths provided are shared between the `Step`s (e.g via `volumes`).
> - There is currently a limit on the overall size of the `Task` results. If the stdout/stderr of a step is set to the path of a `Task` result and the step prints too many data, the result manifest would become too large. Currently the entrypoint binary will fail if that happens.
> - If the stdout/stderr of a `Step` is set to the path of a `Task` result, e.g. `$(results.empty.path)`, but that result is not defined for the `Task`, the `Step` will run but the output will be captured in a file named `$(results.empty.path)` in the current working directory. Similarly, any stubstition that is not valid, e.g. `$(some.invalid.path)/out.txt`, will be left as-is and will result in a file path `$(some.invalid.path)/out.txt` relative to the current working directory.

### Specifying `Parameters`

You can specify parameters, such as compilation flags or artifact names, that you want to supply to the `Task` at execution time.
 `Parameters` are passed to the `Task` from its corresponding `TaskRun`.

#### Parameter name
Parameter name format:
- Must only contain alphanumeric characters, hyphens (`-`), underscores (`_`), and dots (`.`). However, `object` parameter name and its key names can't contain dots (`.`). See the reasons in the third item added in this [PR](https://github.com/tektoncd/community/pull/711).
- Must begin with a letter or an underscore (`_`).

For example, `foo.Is-Bar_` is a valid parameter name for string or array type, but is invalid for object parameter because it contains dots. On the other hand, `barIsBa$` or `0banana` are invalid for all types.

> NOTE:
> 1. Parameter names are **case insensitive**. For example, `APPLE` and `apple` will be treated as equal. If they appear in the same TaskSpec's params, it will be rejected as invalid.
> 2. If a parameter name contains dots (.), it must be referenced by using the [bracket notation](#substituting-parameters-and-resources) with either single or double quotes i.e. `$(params['foo.bar'])`, `$(params["foo.bar"])`. See the following example for more information.

#### Parameter type
Each declared parameter has a `type` field, which can be set to `string`, `array` (beta feature) or `object` (beta feature).

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

Refer to the [TaskRun example](../examples/v1beta1/taskruns/alpha/object-param-result.yaml) and the [PipelineRun example](../examples/v1beta1/pipelineruns/alpha/pipeline-object-param-and-result.yaml) in which `object` parameters are demonstrated.

  > NOTE:
  > - `object` param is a `beta` feature and gated by the `alpha` or `beta` feature flag.
  > - `object` param must specify the `properties` section to define the schema i.e. what keys are available for this object param. See how to define `properties` section in the following example and the [TEP-0075](https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md#defaulting-to-string-types-for-values).
  > - When providing value for an `object` param, one may provide values for just a subset of keys in spec's `default`, and provide values for the rest of keys at runtime ([example](../examples/v1beta1/taskruns/beta/object-param-result.yaml)).
  > - When using object in variable replacement, users can only access its individual key ("child" member) of the object by its name i.e. `$(params.gitrepo.url)`. Using an entire object as a value is only allowed when the value is also an object like [this example](https://github.com/tektoncd/pipeline/blob/55665765e4de35b3a4fb541549ae8cdef0996641/examples/v1beta1/pipelineruns/beta/pipeline-object-param-and-result.yaml#L64-L65). See more details about using object param from the [TEP-0075](https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md#using-objects-in-variable-replacement).

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

#### Default value
Parameter declarations (within Tasks and Pipelines) can include default values which will be used if the parameter is
not specified, for example to specify defaults for both string params and array params
([full example](../examples/v1beta1/taskruns/array-default.yaml)) :

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: task-with-array-default
spec:
  params:
    - name: flags
      type: array
      default:
        - "--set"
        - "arg1=foo"
        - "--randomflag"
        - "--someotherflag"
```

### Specifying `Workspaces`

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
and the [`Workspaces` in a `TaskRun`](../examples/v1beta1/taskruns/workspace.yaml) example YAML file.

### Propagated `Workspaces`

Workspaces can be propagated to embedded task specs, not referenced Tasks. For more information, see [Propagated Workspaces](taskruns.md#propagated-workspaces).

### Emitting `Results`

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

This is an experimental feature. The `results-from` feature flag must be set to `"sidecar-logs"`](./install.md#enabling-larger-results-using-sidecar-logs).

Instead of using termination messages to store results, the taskrun controller injects a sidecar container which monitors the results of all the steps. The sidecar mounts the volume where results of all the steps are stored. As soon as it finds a new result, it logs it to std out. The controller has access to the logs of the sidecar container (Caution: we need you to enable access to [kubernetes pod/logs](./install.md#enabling-larger-results-using-sidecar-logs).

This feature allows users to store up to 4 KB per result by default. Because we are not limited by the size of the termination messages, users can have as many results as they require (or until the CRD reaches its limit). If the size of a result exceeds this limit, then the TaskRun will be placed into a failed state with the following message: `Result exceeded the maximum allowed limit.`

**Note**: If you require even larger results, you can specify a different upper limit per result by setting `max-result-size` feature flag to your desired size in bytes ([see instructions](./install.md#enabling-larger-results-using-sidecar-logs)). **CAUTION**: the larger you make the size, more likely will the CRD reach its max limit enforced by the `etcd` server leading to bad user experience.

### Specifying `Volumes`

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

### Specifying a `Step` template

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

### Specifying `Sidecars`

The `sidecars` field specifies a list of [`Containers`](https://kubernetes.io/docs/concepts/containers/)
to run alongside the `Steps` in your `Task`. You can use `Sidecars` to provide auxiliary functionality, such as
[Docker in Docker](https://hub.docker.com/_/docker) or running a mock API server that your app can hit during testing.
`Sidecars` spin up before your `Task` executes and are deleted after the `Task` execution completes.
For further information, see [`Sidecars` in `TaskRuns`](taskruns.md#specifying-sidecars).

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

### Specifying a display name

The `displayName` field is an optional field that allows you to add a user-facing name to the task that may be used to populate a UI.

### Adding a description

The `description` field is an optional field that allows you to add an informative description to the `Task`.

### Using variable substitution

Tekton provides variables to inject values into the contents of certain fields.
The values you can inject come from a range of sources including other fields
in the Task, context-sensitive information that Tekton provides, and runtime
information received from a TaskRun.

The mechanism of variable substitution is quite simple - string replacement is
performed by the Tekton Controller when a TaskRun is executed.

`Tasks` allow you to substitute variable names for the following entities:

- [Parameters and resources](#substituting-parameters-and-resources)
- [`Array` parameters](#substituting-array-parameters)
- [`Workspaces`](#substituting-workspace-paths)
- [`Volume` names and types](#substituting-volume-names-and-paths)

See the [complete list of variable substitutions for Tasks](./variables.md#variables-available-in-a-task)
and the [list of fields that accept substitutions](./variables.md#fields-that-accept-variable-substitutions).

#### Substituting parameters and resources

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

#### Substituting `Array` parameters

You can expand referenced parameters of type `array` using the star operator. To do so, add the operator (`[*]`)
to the named parameter to insert the array elements in the spot of the reference string.

For example, given a `params` field with the contents listed below, you can expand
`command: ["first", "$(params.array-param[*])", "last"]` to `command: ["first", "some", "array", "elements", "last"]`:

```yaml
params:
  - name: array-param
    value:
      - "some"
      - "array"
      - "elements"
```

You **must** reference parameters of type `array` in a completely isolated string within a larger `string` array.
Referencing an `array` parameter in any other way will result in an error. For example, if `build-args` is a parameter of
type `array`, then the following example is an invalid `Step` because the string isn't isolated:

```yaml
- name: build-step
  image: gcr.io/cloud-builders/some-image
  args: ["build", "additionalArg $(params.build-args[*])"]
```

Similarly, referencing `build-args` in a non-`array` field is also invalid:

```yaml
- name: build-step
  image: "$(params.build-args[*])"
  args: ["build", "args"]
```

A valid reference to the `build-args` parameter is isolated and in an eligible field (`args`, in this case):

```yaml
- name: build-step
  image: gcr.io/cloud-builders/some-image
  args: ["build", "$(params.build-args[*])", "additionalArg"]
```

`array` param when referenced in `args` section of the `step` can be utilized in the `script` as command line arguments:

```yaml
- name: build-step
  image: gcr.io/cloud-builders/some-image
  args: ["$(params.flags[*])"]
  script: |
    #!/usr/bin/env bash
    echo "The script received $# flags."
    echo "The first command line argument is $1."
```

Indexing into an array to reference an individual array element is supported as an **alpha** feature (`enable-api-fields: alpha`).
Referencing an individual array element in `args`:

```yaml
- name: build-step
  image: gcr.io/cloud-builders/some-image
  args: ["$(params.flags[0])"]
```

Referencing an individual array element in `script`:

```yaml
- name: build-step
  image: gcr.io/cloud-builders/some-image
  script: |
    #!/usr/bin/env bash
    echo "$(params.flags[0])"
```

#### Substituting `Workspace` paths

You can substitute paths to `Workspaces` specified within a `Task` as follows:

```yaml
$(workspaces.myworkspace.path)
```

Since the `Volume` name is randomized and only set when the `Task` executes, you can also
substitute the volume name as follows:

```yaml
$(workspaces.myworkspace.volume)
```

#### Substituting `Volume` names and types

You can substitute `Volume` names and [types](https://kubernetes.io/docs/concepts/storage/volumes/#types-of-volumes)
by parameterizing them. Tekton supports popular `Volume` types such as `ConfigMap`, `Secret`, and `PersistentVolumeClaim`.
See this [example](#mounting-a-configmap-as-a-volume-source) to find out how to perform this type of substitution
in your `Task.`

#### Substituting in `Script` blocks

Variables can contain any string, including snippets of script that can
be injected into a Task's `Script` field. If you are using Tekton's variables
in your Task's `Script` field be aware that the strings you're interpolating
could include executable instructions.

Preventing a substituted variable from executing as code depends on the container
image, language or shell that your Task uses. Here's an example of interpolating
a Tekton variable into a `bash` `Script` block that prevents the variable's string
contents from being executed:

```yaml
# Task.yaml
spec:
  steps:
    - image: an-image-that-runs-bash
      env:
        - name: SCRIPT_CONTENTS
          value: $(params.script)
      script: |
        printf '%s' "${SCRIPT_CONTENTS}" > input-script
```

This works by injecting Tekton's variable as an environment variable into the Step's
container. The `printf` program is then used to write the environment variable's
content to a file.

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

The example Task/TaskRun above can be found as a [TaskRun example](../examples/v1beta1/taskruns/run-steps-as-non-root.yaml).

## `Task` Authoring Recommendations

Recommendations for authoring `Tasks` are available in the [Tekton Catalog][recommendations].

[recommendations]: https://github.com/tektoncd/catalog/blob/main/recommendations.md

---

Except as otherwise noted, the contents of this page are licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/).
Code samples are licensed under the [Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
