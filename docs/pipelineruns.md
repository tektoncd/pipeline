<!--
---
linkTitle: "PipelineRuns"
weight: 204
---
-->

# PipelineRuns

<!-- toc -->
- [PipelineRuns](#pipelineruns)
  - [Overview](#overview)
  - [Configuring a <code>PipelineRun</code>](#configuring-a-pipelinerun)
    - [Specifying the target <code>Pipeline</code>](#specifying-the-target-pipeline)
      - [Tekton Bundles](#tekton-bundles)
      - [Remote Pipelines](#remote-pipelines)
    - [Specifying Task-level `ComputeResources`](#specifying-task-level-computeresources)
    - [Specifying <code>Parameters</code>](#specifying-parameters)
      - [Propagated Parameters](#propagated-parameters)
        - [Scope and Precedence](#scope-and-precedence)
        - [Default Values](#default-values)
        - [Object Parameters](#object-parameters)
    - [Specifying custom <code>ServiceAccount</code> credentials](#specifying-custom-serviceaccount-credentials)
    - [Mapping <code>ServiceAccount</code> credentials to <code>Tasks</code>](#mapping-serviceaccount-credentials-to-tasks)
    - [Specifying a <code>Pod</code> template](#specifying-a-pod-template)
    - [Specifying taskRunSpecs](#specifying-taskrunspecs)
      - [Parameter Substitution in taskRunSpecs](#parameter-substitution-in-taskrunspecs)
      - [Matrix Support with taskRunSpecs](#matrix-support-with-taskrunspecs)
    - [Specifying <code>Workspaces</code>](#specifying-workspaces)
      - [Propagated Workspaces](#propagated-workspaces)
        - [Referenced TaskRuns within Embedded PipelineRuns](#referenced-taskruns-within-embedded-pipelineruns)
    - [Specifying <code>LimitRange</code> values](#specifying-limitrange-values)
    - [Configuring a failure timeout](#configuring-a-failure-timeout)
  - [<code>PipelineRun</code> status](#pipelinerun-status)
    - [The <code>status</code> field](#the-status-field)
    - [Monitoring execution status](#monitoring-execution-status)
    - [Marking off user errors](#marking-off-user-errors)
  - [Cancelling a <code>PipelineRun</code>](#cancelling-a-pipelinerun)
  - [Gracefully cancelling a <code>PipelineRun</code>](#gracefully-cancelling-a-pipelinerun)
  - [Gracefully stopping a <code>PipelineRun</code>](#gracefully-stopping-a-pipelinerun)
  - [Pending <code>PipelineRuns</code>](#pending-pipelineruns)
<!-- /toc -->


## Overview

A `PipelineRun` allows you to instantiate and execute a [`Pipeline`](pipelines.md) on-cluster.
A `Pipeline` specifies one or more `Tasks` in the desired order of execution. A `PipelineRun`
executes the `Tasks` in the `Pipeline` in the order they are specified until all `Tasks` have
executed successfully or a failure occurs.

**Note:** A `PipelineRun` automatically creates corresponding `TaskRuns` for every
`Task` in your `Pipeline`.

The `Status` field tracks the current state of a `PipelineRun`, and can be used to monitor
progress.
This field contains the status of every `TaskRun`, as well as the full `PipelineSpec` used
to instantiate this `PipelineRun`, for full auditability.

## Configuring a `PipelineRun`

A `PipelineRun` definition supports the following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version. For example
    `tekton.dev/v1beta1`.
  - [`kind`][kubernetes-overview] - Indicates that this resource object is a `PipelineRun` object.
  - [`metadata`][kubernetes-overview] - Specifies the metadata that uniquely identifies the
    `PipelineRun` object. For example, a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    this `PipelineRun` object.
    - [`pipelineRef` or `pipelineSpec`](#specifying-the-target-pipeline) - Specifies the target [`Pipeline`](pipelines.md).
- Optional:
  - [`params`](#specifying-parameters) - Specifies the desired execution parameters for the `Pipeline`.
  - [`serviceAccountName`](#specifying-custom-serviceaccount-credentials) - Specifies a `ServiceAccount`
    object that supplies specific execution credentials for the `Pipeline`.
  - [`status`](#cancelling-a-pipelinerun) - Specifies options for cancelling a `PipelineRun`.
  - [`taskRunSpecs`](#specifying-taskrunspecs) - Specifies a list of `PipelineTaskRunSpec` which allows for setting `ServiceAccountName`, [`Pod` template](./podtemplates.md), and `Metadata` for each task. This overrides the `Pod` template set for the entire `Pipeline`.
  - [`timeout`](#configuring-a-failure-timeout) - Specifies the timeout before the `PipelineRun` fails. `timeout` is deprecated and will eventually be removed, so consider using `timeouts` instead.
  - [`timeouts`](#configuring-a-failure-timeout) - Specifies the timeout before the `PipelineRun` fails. `timeouts` allows more granular timeout configuration, at the pipeline, tasks, and finally levels
  - [`podTemplate`](#specifying-a-pod-template) - Specifies a [`Pod` template](./podtemplates.md) to use as the basis for the configuration of the `Pod` that executes each `Task`.
  - [`workspaces`](#specifying-workspaces) - Specifies a set of workspace bindings which must match the names of workspaces declared in the pipeline being used.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Specifying the target `Pipeline`

You must specify the target `Pipeline` that you want the `PipelineRun` to execute, either by referencing
an existing `Pipeline` definition, or embedding a `Pipeline` definition directly in the `PipelineRun`.

To specify the target `Pipeline` by reference, use the `pipelineRef` field:

```yaml
spec:
  pipelineRef:
    name: mypipeline
```
To embed a `Pipeline` definition in the `PipelineRun`, use the `pipelineSpec` field:

```yaml
spec:
  pipelineSpec:
    tasks:
      - name: task1
        taskRef:
          name: mytask
```

The `Pipeline` in the [`pipelineSpec` example](../examples/v1/pipelineruns/pipelinerun-with-pipelinespec.yaml)
example displays morning and evening greetings. Once you create and execute it, you can check the logs for its `Pods`:

```bash
kubectl logs $(kubectl get pods -o name | grep pipelinerun-echo-greetings-echo-good-morning)
Good Morning, Bob!

kubectl logs $(kubectl get pods -o name | grep pipelinerun-echo-greetings-echo-good-night)
Good Night, Bob!
```

You can also embed a `Task` definition the embedded `Pipeline` definition:

```yaml
spec:
  pipelineSpec:
    tasks:
      - name: task1
        taskSpec:
          steps: ...
```

In the [`taskSpec` in `pipelineSpec` example](../examples/v1/pipelineruns/pipelinerun-with-pipelinespec-and-taskspec.yaml)
it's `Tasks` all the way down!

You can also specify labels and annotations with `taskSpec` which are propagated to each `taskRun` and then to the
respective pods. These labels can be used to identify and filter pods for further actions (such as collecting pod metrics,
and cleaning up completed pod with certain labels, etc) even being part of one single Pipeline.

```yaml
spec:
  pipelineSpec:
    tasks:
      - name: task1
        taskSpec:
          metadata:
            labels:
              pipeline-sdk-type: kfp
        # ...
      - name: task2
        taskSpec:
          metadata:
            labels:
              pipeline-sdk-type: tfx
        # ...
```

#### Tekton Bundles

A `Tekton Bundle` is an OCI artifact that contains Tekton resources like `Tasks` which can be referenced within a `taskRef`.

You can reference a `Tekton bundle` in a `TaskRef` in both `v1` and `v1beta1` using [remote resolution](./bundle-resolver.md#pipeline-resolution). The example syntax shown below for `v1` uses remote resolution and requires enabling [beta features](./additional-configs.md#beta-features).

```yaml
spec:
  pipelineRef:
    resolver: bundles
    params:
    - name: bundle
      value: docker.io/myrepo/mycatalog:v1.0
    - name: name
      value: mypipeline
    - name: kind
      value: Pipeline
```

The syntax and caveats are similar to using `Tekton Bundles` for  `Task` references
in [Pipelines](pipelines.md#tekton-bundles) or [TaskRuns](taskruns.md#tekton-bundles).

`Tekton Bundles` may be constructed with any toolsets that produce valid OCI image artifacts
so long as the artifact adheres to the [contract](tekton-bundle-contracts.md).

#### Remote Pipelines

**([beta feature](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#beta-features))**

A `pipelineRef` field may specify a Pipeline in a remote location such as git.
Support for specific types of remote will depend on the Resolvers your
cluster's operator has installed. For more information including a tutorial, please check [resolution docs](resolution.md). The below example demonstrates
referencing a Pipeline in git:

```yaml
spec:
  pipelineRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: revision
      value: abc123
    - name: pathInRepo
      value: /pipeline/buildpacks/0.1/buildpacks.yaml
```

### Specifying Task-level `ComputeResources`

**([alpha only](https://github.com/tektoncd/pipeline/blob/main/docs/additional-configs.md#alpha-features))**

Task-level compute resources can be configured in `PipelineRun.TaskRunSpecs.ComputeResources` or `TaskRun.ComputeResources`.

e.g.

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: pipeline
spec:
  tasks:
    - name: task
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipelinerun
spec:
  pipelineRef:
    name: pipeline
  taskRunSpecs:
    - pipelineTaskName: task
      computeResources:
        requests:
          cpu: 2
```

Further details and examples could be found in [Compute Resources in Tekton](https://github.com/tektoncd/pipeline/blob/main/docs/compute-resources.md).

### Specifying `Parameters`

(See also [Specifying Parameters in Tasks](tasks.md#specifying-parameters))

You can specify `Parameters` that you want to pass to the `Pipeline` during execution,
including different values of the same parameter for different `Tasks` in the `Pipeline`.

**Note:** You must specify all the `Parameters` that the `Pipeline` expects. Parameters
that have default values specified in Pipeline are not required to be provided by PipelineRun.

For example:

```yaml
spec:
  params:
    - name: pl-param-x
      value: "100"
    - name: pl-param-y
      value: "500"
```
You can pass in extra `Parameters` if needed depending on your use cases. An example use
case is when your CI system autogenerates `PipelineRuns` and it has `Parameters` it wants to
provide to all `PipelineRuns`. Because you can pass in extra `Parameters`, you don't have to
go through the complexity of checking each `Pipeline` and providing only the required params.

#### Parameter Enums

> :seedling: **`enum` is an [alpha](additional-configs.md#alpha-features) feature.** The `enable-param-enum` feature flag must be set to `"true"` to enable this feature.

If a `Parameter` is guarded by `Enum` in the `Pipeline`, you can only provide `Parameter` values in the `PipelineRun` that are predefined in the `Param.Enum` in the `Pipeline`. The `PipelineRun` will fail with reason `InvalidParamValue` otherwise.

Tekton will also the validate the `param` values passed to any referenced `Tasks` (via `taskRef`) if `Enum` is specified for the `Task`. The `PipelineRun` will fail with reason `InvalidParamValue` if `Enum` validation is failed for any of the `PipelineTask`.

You can also specify `Enum` in an embedded `Pipeline` in a `PipelineRun`. The same `Param` validation will be executed in this scenario.

See more details in [Param.Enum](./pipelines.md#param-enum).

#### Propagated Parameters

When using an inlined spec, parameters from the parent `PipelineRun` will be
propagated to any inlined specs without needing to be explicitly defined. This
allows authors to simplify specs by automatically propagating top-level
parameters down to other inlined resources.

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pr-echo-
spec:
  params:
    - name: HELLO
      value: "Hello World!"
    - name: BYE
      value: "Bye World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: |
                #!/usr/bin/env bash
                echo "$(params.HELLO)"
      - name: echo-bye
        taskSpec:
          steps:
            - name: echo
              image: ubuntu
              script: |
                #!/usr/bin/env bash
                echo "$(params.BYE)"
```

On executing the pipeline run, the parameters will be interpolated during resolution.
The specifications are not mutated before storage and so it remains the same.
The status is updated.

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pr-echo-szzs9
  ...
spec:
  params:
  - name: HELLO
    value: Hello World!
  - name: BYE
    value: Bye World!
  pipelineSpec:
    tasks:
    - name: echo-hello
      taskSpec:
        steps:
        - image: ubuntu
          name: echo
          script: |
            #!/usr/bin/env bash
            echo "$(params.HELLO)"
    - name: echo-bye
      taskSpec:
        steps:
        - image: ubuntu
          name: echo
          script: |
            #!/usr/bin/env bash
            echo "$(params.BYE)"
status:
  conditions:
  - lastTransitionTime: "2022-04-07T12:34:58Z"
    message: 'Tasks Completed: 2 (Failed: 0, Canceled 0), Skipped: 0'
    reason: Succeeded
    status: "True"
    type: Succeeded
  pipelineSpec:
    ...
  childReferences:
  - name: pr-echo-szzs9-echo-hello
    pipelineTaskName: echo-hello
    kind: TaskRun
  - name: pr-echo-szzs9-echo-bye
    pipelineTaskName: echo-bye
    kind: TaskRun
```

##### Scope and Precedence

When Parameters names conflict, the inner scope would take precedence as shown in this example:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pr-echo-
spec:
  params:
  - name: HELLO
    value: "Hello World!"
  - name: BYE
    value: "Bye World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        params:
        - name: HELLO
          value: "Sasa World!"
        taskSpec:
          params:
            - name: HELLO
              type: string
          steps:
            - name: echo
              image: ubuntu
              script: |
                #!/usr/bin/env bash
                echo "$(params.HELLO)"
    ...
```

resolves to

```yaml
# Successful execution of the above PipelineRun
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pr-echo-szzs9
  ...
spec:
  ...
status:
  conditions:
    - lastTransitionTime: "2022-04-07T12:34:58Z"
      message: 'Tasks Completed: 2 (Failed: 0, Canceled 0), Skipped: 0'
      reason: Succeeded
      status: "True"
      type: Succeeded
  ...
  childReferences:
  - name: pr-echo-szzs9-echo-hello
    pipelineTaskName: echo-hello
    kind: TaskRun
  ...
```

##### Default Values

When `Parameter` specifications have default values, the `Parameter` value provided at runtime would take precedence to give users control, as shown in this example:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pr-echo-
spec:
  params:
  - name: HELLO
    value: "Hello World!"
  - name: BYE
    value: "Bye World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskSpec:
          params:
          - name: HELLO
            type: string
            default: "Sasa World!"
          steps:
            - name: echo
              image: ubuntu
              script: |
                #!/usr/bin/env bash
                echo "$(params.HELLO)"
    ...
```

resolves to

```yaml
# Successful execution of the above PipelineRun
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pr-echo-szzs9
  ...
spec:
  ...
status:
  conditions:
    - lastTransitionTime: "2022-04-07T12:34:58Z"
      message: 'Tasks Completed: 2 (Failed: 0, Canceled 0), Skipped: 0'
      reason: Succeeded
      status: "True"
      type: Succeeded
  ...
  childReferences:
  - name: pr-echo-szzs9-echo-hello
    pipelineTaskName: echo-hello
    kind: TaskRun
  ...
```

##### Referenced Resources

When a PipelineRun definition has referenced specifications but does not explicitly pass Parameters, the PipelineRun will be created but the execution will fail because of missing Parameters.

```yaml
# Invalid PipelineRun attempting to propagate Parameters to referenced Tasks
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pr-echo-
spec:
  params:
  - name: HELLO
    value: "Hello World!"
  - name: BYE
    value: "Bye World!"
  pipelineSpec:
    tasks:
      - name: echo-hello
        taskRef:
          name: echo-hello
      - name: echo-bye
        taskRef:
          name: echo-bye
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: echo-hello
spec:
  steps:
    - name: echo
      image: ubuntu
      script: |
        #!/usr/bin/env bash
        echo "$(params.HELLO)"
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: echo-bye
spec:
  steps:
    - name: echo
      image: ubuntu
      script: |
        #!/usr/bin/env bash
        echo "$(params.BYE)"
```

Fails as follows:

```yaml
# Failed execution of the above PipelineRun
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pr-echo-24lmf
  ...
spec:
  params:
  - name: HELLO
    value: Hello World!
  - name: BYE
    value: Bye World!
  pipelineSpec:
    tasks:
    - name: echo-hello
      taskRef:
        kind: Task
        name: echo-hello
    - name: echo-bye
      taskRef:
        kind: Task
        name: echo-bye
status:
  conditions:
  - lastTransitionTime: "2022-04-07T20:24:51Z"
    message: 'invalid input params for task echo-hello: missing values for
              these params which have no default values: [HELLO]'
    reason: PipelineValidationFailed
    status: "False"
    type: Succeeded
  ...
```

##### Object Parameters

When using an inlined spec, object parameters from the parent `PipelineRun` will also be
propagated to any inlined specs without needing to be explicitly defined. This
allows authors to simplify specs by automatically propagating top-level
parameters down to other inlined resources.
When propagating object parameters, scope and precedence also holds as shown below.

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: pipelinerun-object-param-result
spec:
  params:
    - name: gitrepo
      value:
        url: abc.com
        commit: sha123
  pipelineSpec:
    tasks:
      - name: task1
        params:
          - name: gitrepo
            value:
              branch: main
              url: xyz.com
        taskSpec:
          steps:
            - name: write-result
              image: bash
              args: [
                "echo",
                "--url=$(params.gitrepo.url)",
                "--commit=$(params.gitrepo.commit)",
                "--branch=$(params.gitrepo.branch)",
              ]
```

resolves to

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipelinerun-object-param-resultpxp59
  ...
spec:
  params:
  - name: gitrepo
    value:
      commit: sha123
      url: abc.com
  pipelineSpec:
    tasks:
    - name: task1
      params:
      - name: gitrepo
        value:
          branch: main
          url: xyz.com
      taskSpec:
        metadata: {}
        spec: null
        steps:
        - args:
          - echo
          - --url=$(params.gitrepo.url)
          - --commit=$(params.gitrepo.commit)
          - --branch=$(params.gitrepo.branch)
          image: bash
          name: write-result
status:
  completionTime: "2022-09-08T17:22:01Z"
  conditions:
  - lastTransitionTime: "2022-09-08T17:22:01Z"
    message: 'Tasks Completed: 1 (Failed: 0, Cancelled 0), Skipped: 0'
    reason: Succeeded
    status: "True"
    type: Succeeded
  pipelineSpec:
    tasks:
    - name: task1
      params:
      - name: gitrepo
        value:
          branch: main
          url: xyz.com
      taskSpec:
        metadata: {}
        spec: null
        steps:
        - args:
          - echo
          - --url=xyz.com
          - --commit=sha123
          - --branch=main
          image: bash
          name: write-result
  startTime: "2022-09-08T17:21:57Z"
  childReferences:
  - name: pipelinerun-object-param-resultpxp59-task1
    pipelineTaskName: task1
    kind: TaskRun
          ...
	taskSpec:
          steps:
          - args:
            - echo
            - --url=xyz.com
            - --commit=sha123
            - --branch=main
            image: bash
            name: write-result
```

### Specifying custom `ServiceAccount` credentials

You can execute the `Pipeline` in your `PipelineRun` with a specific set of credentials by
specifying a `ServiceAccount` object name in the `serviceAccountName` field in your `PipelineRun`
definition. If you do not explicitly specify this, the `TaskRuns` created by your `PipelineRun`
will execute with the credentials specified in the `configmap-defaults` `ConfigMap`. If this
default is not specified, the `TaskRuns` will execute with the [`default` service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#use-the-default-service-account-to-access-the-api-server)
set for the target [`namespace`](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/).

For more information, see [`ServiceAccount`](auth.md).

[`Custom tasks`](pipelines.md#using-custom-tasks) may or may not use a service account name.
Consult the documentation of the custom task that you are using to determine whether it supports a service account name.

### Mapping `ServiceAccount` credentials to `Tasks`

If you require more granularity in specifying execution credentials, use the `taskRunSpecs[].taskServiceAccountName` field to
map a specific `serviceAccountName` value to a specific `Task` in the `Pipeline`. This overrides the global
`serviceAccountName` you may have set for the `Pipeline` as described in the previous section.

For example, if you specify these mappings:

{{< tabs >}}
{{% tab "v1" %}}
```yaml
spec:
  taskRunTemplate:
    serviceAccountName: sa-1
  taskRunSpecs:
    - pipelineTaskName: build-task
      serviceAccountName: sa-for-build
```
{{% /tab %}}

{{% tab "v1beta1" %}}
```yaml
spec:
  serviceAccountName: sa-1
  taskRunSpecs:
    - pipelineTaskName: build-task
      taskServiceAccountName: sa-for-build
```
{{% /tab %}}
{{< /tabs >}}

for this `Pipeline`:

```yaml
kind: Pipeline
spec:
  tasks:
    - name: build-task
      taskRef:
        name: build-push
    - name: test-task
      taskRef:
        name: test
```

then `test-task` will execute using the `sa-1` account while `build-task` will execute with `sa-for-build`.

#### Propagated Results

When using an embedded spec, `Results` from the parent `PipelineRun` will be
propagated to any inlined specs without needing to be explicitly defined. This
allows authors to simplify specs by automatically propagating top-level
results down to other inlined resources.
**`Result` substitutions will only be made for `name`, `commands`, `args`, `env` and `script` fields of `steps`, `sidecars`.**

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: uid-pipeline-run
spec:
  pipelineSpec:
    tasks:
    - name: add-uid
      taskSpec:
        results:
          - name: uid
            type: string
        steps:
          - name: add-uid
            image: busybox
            command: ["/bin/sh", "-c"]
            args:
              - echo "1001" | tee $(results.uid.path)
    - name: show-uid
      # params:
      #   - name: uid
      #     value: $(tasks.add-uid.results.uid)
      taskSpec:
        steps:
          - name: show-uid
            image: busybox
            command: ["/bin/sh", "-c"]
            args:
              - echo $(tasks.add-uid.results.uid)
              # - echo $(params.uid)
```

On executing the `PipelineRun`, the `Results` will be interpolated during resolution.

```yaml
name:         uid-pipeline-run-show-uid
apiVersion:  tekton.dev/v1
kind:         TaskRun
metadata:
  ...
spec:
  taskSpec:
    steps:
      args:
        echo 1001
      command:
        - /bin/sh
        - -c
      image:  busybox
      name:   show-uid
status:
  completionTime:  2023-09-11T07:34:28Z
  conditions:
    lastTransitionTime:  2023-09-11T07:34:28Z
    message:               All Steps have completed executing
    reason:                Succeeded
    status:                True
    type:                  Succeeded
  podName:                uid-pipeline-run-show-uid-pod
  steps:
    container:  step-show-uid
    name:       show-uid
  taskSpec:
    steps:
      args:
        echo 1001
      command:
        /bin/sh
        -c
      computeResources:
      image:  busybox
      name:   show-uid
```

### Specifying a `Pod` template

You can specify a [`Pod` template](podtemplates.md) configuration that will serve as the configuration starting
point for the `Pod` in which the container images specified in your `Tasks` will execute. This allows you to
customize the `Pod` configuration specifically for each `TaskRun`.

In the following example, the `Task` defines a `volumeMount` object named `my-cache`. The `PipelineRun`
provisions this object for the `Task` using a `persistentVolumeClaim` and executes it as user 1001.

{{< tabs >}}
{{% tab "v1" %}}
```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: mytask
spec:
  steps:
    - name: writesomething
      image: ubuntu
      command: ["bash", "-c"]
      args: ["echo 'foo' > /my-cache/bar"]
      volumeMounts:
        - name: my-cache
          mountPath: /my-cache
---
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: mypipeline
spec:
  tasks:
    - name: task1
      taskRef:
        name: mytask
---
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: mypipelinerun
spec:
  pipelineRef:
    name: mypipeline
  taskRunTemplate:
    podTemplate:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
      volumes:
        - name: my-cache
          persistentVolumeClaim:
            claimName: my-volume-claim
```
{{% /tab %}}

{{% tab "v1beta1" %}}
```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: mytask
spec:
  steps:
    - name: writesomething
      image: ubuntu
      command: ["bash", "-c"]
      args: ["echo 'foo' > /my-cache/bar"]
      volumeMounts:
        - name: my-cache
          mountPath: /my-cache
---
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: mypipeline
spec:
  tasks:
    - name: task1
      taskRef:
        name: mytask
---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: mypipelinerun
spec:
  pipelineRef:
    name: mypipeline
  podTemplate:
    securityContext:
      runAsNonRoot: true
      runAsUser: 1001
    volumes:
      - name: my-cache
        persistentVolumeClaim:
          claimName: my-volume-claim
```
{{% /tab %}}
{{< /tabs >}}

[`Custom tasks`](pipelines.md#using-custom-tasks) may or may not use a pod template.
Consult the documentation of the custom task that you are using to determine whether it supports a pod template.

### Specifying taskRunSpecs

Specifies a list of `PipelineTaskRunSpec` which contains `TaskServiceAccountName`, `TaskPodTemplate`
and `PipelineTaskName`. Mapping the specs to the corresponding `Task` based upon the `TaskName` a PipelineTask
will run with the configured  `TaskServiceAccountName` and `TaskPodTemplate` overwriting the pipeline
wide `ServiceAccountName`  and [`podTemplate`](./podtemplates.md) configuration,
for example:

{{< tabs >}}
{{% tab "v1" %}}
```yaml
spec:
  podTemplate:
    securityContext:
      runAsUser: 1000
      runAsGroup: 2000
      fsGroup: 3000
  taskRunSpecs:
    - pipelineTaskName: build-task
      serviceAccountName: sa-for-build
      podTemplate:
        nodeSelector:
          disktype: ssd
      timeout: "1h30m"
```
{{% /tab %}}

{{% tab "v1beta1" %}}
```yaml
spec:
  podTemplate:
    securityContext:
      runAsUser: 1000
      runAsGroup: 2000
      fsGroup: 3000
  taskRunSpecs:
    - pipelineTaskName: build-task
      taskServiceAccountName: sa-for-build
      taskPodTemplate:
        nodeSelector:
          disktype: ssd
      timeout: "1h30m"
```
{{% /tab %}}
{{< /tabs >}}

If used with this `Pipeline`,  `build-task` will use the task specific `PodTemplate` (where `nodeSelector` has `disktype` equal to `ssd`)
along with `securityContext` from the `pipelineRun.spec.podTemplate`. The task will also have a specific timeout of 1 hour and 30 minutes. This overrides any existing timeout already defined by the pipelineTask as well, though the specified `pipelineRun.spec.timeouts.tasks` will still take precedence.

For more details on timeout overrides, precedence rules, validation, and practical examples, see [Overriding Individual Task Timeouts](#overriding-individual-task-timeouts) in the failure timeout section.

`PipelineTaskRunSpec` may also contain `StepSpecs` and `SidecarSpecs`; see
[Overriding `Task` `Steps` and `Sidecars`](./taskruns.md#overriding-task-steps-and-sidecars) for more information.

The optional annotations and labels can be added under a `Metadata` field as for a specific running context.

e.g.

Rendering needed secrets with Vault:

```yaml
spec:
  pipelineRef:
    name: pipeline-name
  taskRunSpecs:
    - pipelineTaskName: task-name
      metadata:
        annotations:
          vault.hashicorp.com/agent-inject-secret-foo: "/path/to/foo"
          vault.hashicorp.com/role: role-name
```

Updating labels applied in a runtime context:

```yaml
spec:
  pipelineRef:
    name: pipeline-name
  taskRunSpecs:
    - pipelineTaskName: task-name
      metadata:
        labels:
          app: cloudevent
```

If a metadata key is present in different levels, the value that will be used in the `PipelineRun` is determined using this precedence order: `PipelineRun.spec.taskRunSpec.metadata` > `PipelineRun.metadata` > `Pipeline.spec.tasks.taskSpec.metadata`.

#### Parameter Substitution in taskRunSpecs

The `taskRunSpecs` supports parameter substitution in the `podTemplate` fields. This allows you to dynamically configure pod templates based on pipeline parameters, including those from Matrix tasks.

For example, you can use parameter substitution to configure node selectors based on architecture parameters:

```yaml
spec:
  taskRunSpecs:
    - pipelineTaskName: build-task
      podTemplate:
        nodeSelector:
          kubernetes.io/arch: $(params.arch)
        tolerations:
          - key: "environment"
            operator: "Equal"
            value: "$(params.env)"
            effect: "NoSchedule"
```

#### Matrix Support with taskRunSpecs

When using [`Matrix`](matrix.md) to fan out `PipelineTasks`, the `taskRunSpecs` can reference matrix parameters for dynamic pod template configuration. Each matrix combination will create a separate `TaskRun` with the appropriate parameter values substituted in the pod template.

Here's an example showing how to use `taskRunSpecs` with matrix parameters:

```yaml
spec:
  taskRunSpecs:
    - pipelineTaskName: build-and-push-manifest
      podTemplate:
        nodeSelector:
          kubernetes.io/arch: $(params.arch)
  pipelineSpec:
    tasks:
      - name: build-and-push-manifest
        matrix:
          params:
            - name: arch
              value: ["amd64", "arm64"]
        taskSpec:
          params:
            - name: arch
          steps:
            - name: build-and-push
              image: ubuntu
              script: |
                echo "building on $(params.arch)"
```

In this example, the matrix will create two `TaskRuns` - one for `amd64` and one for `arm64`. Each will have its pod scheduled on the appropriate node architecture using the nodeSelector with the substituted parameter value.

For a complete example, see [`pipelinerun-with-taskrunspecs-matrix-param-substitution.yaml`](../examples/v1/pipelineruns/pipelinerun-with-taskrunspecs-matrix-param-substitution.yaml).

### Specifying `Workspaces`

If your `Pipeline` specifies one or more `Workspaces`, you must map those `Workspaces` to
the corresponding physical volumes in your `PipelineRun` definition. For example, you
can map a `PersistentVolumeClaim` volume to a `Workspace` as follows:

```yaml
workspaces:
  - name: myworkspace # must match workspace name in Task
    persistentVolumeClaim:
      claimName: mypvc # this PVC must already exist
    subPath: my-subdir
```

`workspaces[].subPath` can be an absolute value or can reference `pipelineRun` context variables, such as,
`$(context.pipelineRun.name)` or `$(context.pipelineRun.uid)`.

You can pass in extra `Workspaces` if needed depending on your use cases. An example use
case is when your CI system autogenerates `PipelineRuns` and it has `Workspaces` it wants to
provide to all `PipelineRuns`. Because you can pass in extra `Workspaces`, you don't have to
go through the complexity of checking each `Pipeline` and providing only the required `Workspaces`:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: pipeline
spec:
  tasks:
    - name: task
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipelinerun
spec:
  pipelineRef:
    name: pipeline
  workspaces:
    - name: unusedworkspace
      persistentVolumeClaim:
        claimName: mypvc
```

For more information, see the following topics:
- For information on mapping `Workspaces` to `Volumes`, see [Specifying `Workspaces` in `PipelineRuns`](workspaces.md#specifying-workspaces-in-pipelineruns).
- For a list of supported `Volume` types, see [Specifying `VolumeSources` in `Workspaces`](workspaces.md#specifying-volumesources-in-workspaces).
- For an end-to-end example, see [`Workspaces` in a `PipelineRun`](../examples/v1/pipelineruns/workspaces.yaml).

[`Custom tasks`](pipelines.md#using-custom-tasks) may or may not use workspaces.
Consult the documentation of the custom task that you are using to determine whether it supports workspaces.

#### Propagated Workspaces

When using an embedded spec, workspaces from the parent `PipelineRun` will be
propagated to any inlined specs without needing to be explicitly defined. This
allows authors to simplify specs by automatically propagating top-level
workspaces down to other inlined resources.
**Workspace substutions will only be made for `commands`, `args` and `script` fields of `steps`, `stepTemplates`, and `sidecars`.**

```yaml
# Inline specifications of a PipelineRun
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: recipe-time-
spec:
  workspaces:
    - name: shared-data
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 16Mi
          volumeMode: Filesystem
  pipelineSpec:
    #workspaces:
    #  - name: shared-data
    tasks:
    - name: fetch-secure-data
      # workspaces:
      #   - name: shared-data
      taskSpec:
        # workspaces:
        #   - name: shared-data
        steps:
        - name: fetch-and-write-secure
          image: ubuntu
          script: |
            echo hi >> $(workspaces.shared-data.path)/recipe.txt
    - name: print-the-recipe
      # workspaces:
      #   - name: shared-data
      runAfter:
        - fetch-secure-data
      taskSpec:
        # workspaces:
        #   - name: shared-data
        steps:
        - name: print-secrets
          image: ubuntu
          script: cat $(workspaces.shared-data.path)/recipe.txt
```

On executing the pipeline run, the workspaces will be interpolated during resolution.

```yaml
# Successful execution of the above PipelineRun
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: recipe-time-
  ...
spec:
  pipelineSpec:
  ...
status:
  completionTime: "2022-06-02T18:17:02Z"
  conditions:
  - lastTransitionTime: "2022-06-02T18:17:02Z"
    message: 'Tasks Completed: 2 (Failed: 0, Canceled 0), Skipped: 0'
    reason: Succeeded
    status: "True"
    type: Succeeded
  pipelineSpec:
    ...
  childReferences:
  - name: recipe-time-lslt9-fetch-secure-data
    pipelineTaskName: fetch-secure-data
    kind: TaskRun
  - name: recipe-time-lslt9-print-the-recipe
    pipelineTaskName: print-the-recipe
    kind: TaskRun
```

##### Workspace Referenced Resources

`Workspaces` cannot be propagated to referenced specifications. For example, the following Pipeline will fail when executed because the workspaces defined in the PipelineRun cannot be propagated to the referenced Pipeline.

```yaml
# PipelineRun attempting to propagate Workspaces to referenced Tasks
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: shared-task-storage
spec:
  resources:
    requests:
      storage: 16Mi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteOnce
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: fetch-and-print-recipe
spec:
  tasks:
  - name: fetch-the-recipe
    taskRef:
      name: fetch-secure-data
  - name: print-the-recipe
    taskRef:
      name: print-data
    runAfter:
      - fetch-the-recipe
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: recipe-time-
spec:
  pipelineRef:
    name: fetch-and-print-recipe
  workspaces:
  - name: shared-data
    persistentVolumeClaim:
      claimName: shared-task-storage
```

Upon execution, this will cause failures:

```yaml
# Failed execution of the above PipelineRun

apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: recipe-time-
  ...
spec:
  pipelineRef:
    name: fetch-and-print-recipe
  workspaces:
  - name: shared-data
    persistentVolumeClaim:
      claimName: shared-task-storage
status:
  completionTime: "2022-06-02T19:02:58Z"
  conditions:
  - lastTransitionTime: "2022-06-02T19:02:58Z"
    message: 'Tasks Completed: 1 (Failed: 1, Canceled 0), Skipped: 1'
    reason: Failed
    status: "False"
    type: Succeeded
  pipelineSpec:
    ...
  childReferences:
  - name: recipe-time-v5scg-fetch-the-recipe
    pipelineTaskName: fetch-the-recipe
    kind: TaskRun
```

#### Referenced TaskRuns within Embedded PipelineRuns
As mentioned in the [Workspace Referenced Resources](#workspace-referenced-resources), workspaces can only be propagated from PipelineRuns to embedded Pipeline specs, not Pipeline references. Similarly, workspaces can only be propagated from a Pipeline to embedded Task specs, not referenced Tasks. For example:

```yaml
# PipelineRun attempting to propagate Workspaces to referenced Tasks
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: Task
metadata:
  name: fetch-secure-data
spec:
  workspaces: # If Referenced, Workspaces need to be explicitly declared
  - name: shared-data
  steps:
  - name: fetch-and-write
    image: ubuntu
    script: |
      echo $(workspaces.shared-data.path)
---
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: recipe-time-
spec:
  workspaces:
  - name: shared-data
    persistentVolumeClaim:
      claimName: shared-task-storage
  pipelineSpec:
    # workspaces: # Since this is embedded specs, Workspaces don’t need to be declared
    #    ...
    tasks:
    - name: fetch-the-recipe
      workspaces: # If referencing resources, Workspaces need to be explicitly declared
      - name: shared-data
      taskRef: # Referencing a resource
        name: fetch-secure-data
    - name: print-the-recipe
      # workspaces: # Since this is embedded specs, Workspaces don’t need to be declared
      #    ...
      taskSpec:
        # workspaces: # Since this is embedded specs, Workspaces don’t need to be declared
        #    ...
        steps:
        - name: print-secrets
          image: ubuntu
          script: cat $(workspaces.shared-data.path)/recipe.txt
      runAfter:
        - fetch-the-recipe
```

The above pipelinerun successfully resolves to:

```yaml
# Successful execution of the above PipelineRun
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: recipe-time-
  ...
spec:
  pipelineSpec:
    ...
  workspaces:
  - name: shared-data
    persistentVolumeClaim:
      claimName: shared-task-storage
status:
  completionTime: "2022-06-09T18:42:14Z"
  conditions:
  - lastTransitionTime: "2022-06-09T18:42:14Z"
    message: 'Tasks Completed: 2 (Failed: 0, Cancelled 0), Skipped: 0'
    reason: Succeeded
    status: "True"
    type: Succeeded
  pipelineSpec:
    ...
  childReferences:
  - name: recipe-time-pj6l7-fetch-the-recipe
    pipelineTaskName: fetch-the-recipe
    kind: TaskRun
  - name: recipe-time-pj6l7-print-the-recipe
    pipelineTaskName: print-the-recipe
    kind: TaskRun
```

### Specifying `LimitRange` values

In order to only consume the bare minimum amount of resources needed to execute one `Step` at a
time from the invoked `Task`, Tekton will request the compute values for CPU, memory, and ephemeral
storage for each `Step` based on the [`LimitRange`](https://kubernetes.io/docs/concepts/policy/limit-range/)
object(s), if present. Any `Request` or `Limit` specified by the user (on `Task` for example) will be left unchanged.

For more information, see the [`LimitRange` support in Pipeline](./compute-resources.md#limitrange-support).

### Configuring a failure timeout

You can use the `timeouts` field to set the `PipelineRun's` desired timeout value in minutes.
There are three sub-fields:
- `pipeline`: specifies the timeout for the entire PipelineRun. Defaults to to the global configurable default timeout of 60 minutes.
When `timeouts.pipeline` has elapsed, any running child TaskRuns will be canceled, regardless of whether they are normal Tasks
or `finally` Tasks, and the PipelineRun will fail.
- `tasks`: specifies the timeout for the cumulative time taken by non-`finally` Tasks specified in `pipeline.spec.tasks`.
To specify a timeout for an individual Task, use `pipeline.spec.tasks[].timeout`.
When `timeouts.tasks` has elapsed, any running child TaskRuns will be canceled, finally Tasks will run if `timeouts.finally` is specified,
and the PipelineRun will fail.
- `finally`: the timeout for the cumulative time taken by `finally` Tasks specified in `pipeline.spec.finally`.
(Since all `finally` Tasks run in parallel, this is functionally equivalent to the timeout for any `finally` Task.)
When `timeouts.finally` has elapsed, any running `finally` TaskRuns will be canceled,
and the PipelineRun will fail.

For example:

```yaml
timeouts:
  pipeline: "0h0m60s"
  tasks: "0h0m40s"
  finally: "0h0m20s"
```

All three sub-fields are optional, and will be automatically processed according to the following constraint:
* `timeouts.pipeline >= timeouts.tasks + timeouts.finally`

Each `timeout` field is a `duration` conforming to Go's
[`ParseDuration`](https://golang.org/pkg/time/#ParseDuration) format. For example, valid
values are `1h30m`, `1h`, `1m`, and `60s`.

If any of the sub-fields are set to "0", there is no timeout for that section of the PipelineRun,
meaning that it will run until it completes successfully or encounters an error.
To set `timeouts.tasks` or `timeouts.finally` to "0", you must also set `timeouts.pipeline` to "0".

The global default timeout is set to 60 minutes when you first install Tekton. You can set
a different global default timeout value using the `default-timeout-minutes` field in
[`config/config-defaults.yaml`](./../config/config-defaults.yaml).

#### Overriding Individual Task Timeouts

You can use `taskRunSpecs` to override individual task timeouts at runtime without modifying the Pipeline definition.

**Timeout Precedence (highest to lowest):**
1. `taskRunSpecs[].timeout` - runtime override per task
2. `pipeline.spec.tasks[].timeout` - Pipeline spec timeout
3. `timeouts.tasks` or `timeouts.pipeline` - PipelineRun constraints  
4. Global default timeout

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
spec:
  timeouts:
    pipeline: "10m"    # 3. PipelineRun constraint
  pipelineSpec:
    tasks:
    - name: task-a
      timeout: "8m"     # 2. Pipeline spec timeout
      taskSpec: { ... }
    - name: task-b  
      taskSpec: { ... } # 4. Uses global default (60m)
  taskRunSpecs:
  - pipelineTaskName: task-a
    timeout: "5m"       # 1. Highest priority - overrides 8m Pipeline timeout
```

**Note:** `taskRunSpecs` timeouts cannot exceed pipeline-level constraints and will fail validation if they do.

Example timeouts usages are as follows:

Combination 1: Set the timeout for the entire `pipeline` and reserve a portion of it for `tasks`.

```yaml
kind: PipelineRun
spec:
  timeouts:
    pipeline: "0h4m0s"
    tasks: "0h1m0s"
```

Combination 2: Set the timeout for the entire `pipeline` and reserve a portion of it for `finally`.

```yaml
kind: PipelineRun
spec:
  timeouts:
    pipeline: "0h4m0s"
    finally: "0h3m0s"
```

Combination 3: Set only a `tasks` timeout, with no timeout for the entire `pipeline`.

```yaml
kind: PipelineRun
spec:
  timeouts:
    pipeline: "0"  # No timeout
    tasks: "0h3m0s"
```

Combination : Set only a `finally` timeout, with no timeout for the entire `pipeline`.

```yaml
kind: PipelineRun
spec:
  timeouts:
    pipeline: "0"  # No timeout
    finally: "0h3m0s"
```

You can also use the *Deprecated* `timeout` field to set the `PipelineRun's` desired timeout value in minutes.
If you do not specify this value in the `PipelineRun`, the global default timeout value applies.
If you set the timeout to 0, the `PipelineRun` fails immediately upon encountering an error.

> :warning: ** `timeout` is deprecated and will be removed in future versions. Consider using `timeouts` instead.

> :note: An internal detail of the `PipelineRun` and `TaskRun` reconcilers in the Tekton controller is that it will requeue a `PipelineRun` or `TaskRun` for re-evaluation, versus waiting for the next update, under certain conditions.  The wait time for that re-queueing is the elapsed time subtracted from the timeout; however, if the timeout is set to '0', that calculation produces a negative number, and the new reconciliation event will fire immediately, which can impact overall performance, which is counter to the intent of wait time calculation.  So instead, the reconcilers will use the configured global timeout as the wait time when the associated timeout has been set to '0'.

## `PipelineRun` status

### The `status` field

Your `PipelineRun`'s `status` field can contain the following fields:
- Required:
  <!-- wokeignore:rule=master -->
  - `status` - Most relevant, `status.conditions`, which contains the latest observations of the `PipelineRun`'s state. [See here](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) for information on typical status properties.
  - `startTime` - The time at which the `PipelineRun` began executing, in [RFC3339](https://tools.ietf.org/html/rfc3339) format.
  - `completionTime` - The time at which the `PipelineRun` finished executing, in [RFC3339](https://tools.ietf.org/html/rfc3339) format.
  - [`pipelineSpec`](pipelines.md#configuring-a-pipeline) - The exact `PipelineSpec` used when starting the `PipelineRun`.
- Optional:
  - [`pipelineResults`](pipelines.md#emitting-results-from-a-pipeline) - Results emitted by this `PipelineRun`.
  - `skippedTasks` - A list of `Task`s which were skipped when running this `PipelineRun` due to [when expressions](pipelines.md#guard-task-execution-using-when-expressions), including the when expressions applying to the skipped task.
  - `childReferences` - A list of references to each `TaskRun` or `Run` in this `PipelineRun`, which can be used to look up the status of the underlying `TaskRun` or `Run`. Each entry contains the following:
    - [`kind`][kubernetes-overview] - Generally either `TaskRun` or `Run`.
    - [`apiVersion`][kubernetes-overview] - The API version for the underlying `TaskRun` or `Run`.
    - [`whenExpressions`](pipelines.md#guard-task-execution-using-when-expressions) - The list of when expressions guarding the execution of this task.
  - `provenance` - Metadata about the runtime configuration and the resources used in the PipelineRun. The data in the `provenance` field will be recorded into the build provenance by the provenance generator i.e. (Tekton Chains). Currently, there are 2 subfields:
    - `refSource`: the source from where a remote pipeline definition was fetched.
    - `featureFlags`: the configuration data of the `feature-flags` configmap.
  - `finallyStartTime`- The time at which the PipelineRun's `finally` Tasks, if any, began
  executing, in [RFC3339](https://tools.ietf.org/html/rfc3339) format.

### Monitoring execution status

As your `PipelineRun` executes, its `status` field accumulates information on the execution of each `TaskRun`
as well as the `PipelineRun` as a whole. This information includes the name of the pipeline `Task` associated
to a `TaskRun`, the complete [status of the `TaskRun`](taskruns.md#monitoring-execution-status) and details
about `whenExpressions` that may be associated to a `TaskRun`.

The following example shows an extract from the `status` field of a `PipelineRun` that has executed successfully:

```yaml
completionTime: "2020-05-04T02:19:14Z"
conditions:
  - lastTransitionTime: "2020-05-04T02:19:14Z"
    message: "Tasks Completed: 4, Skipped: 0"
    reason: Succeeded
    status: "True"
    type: Succeeded
startTime: "2020-05-04T02:00:11Z"
childReferences:
- name: triggers-release-nightly-frwmw-build
  pipelineTaskName: build
  kind: TaskRun
  ```

The following tables shows how to read the overall status of a `PipelineRun`.
Completion time is set once a `PipelineRun` reaches status `True` or `False`:

`status` | `reason`           | `completionTime` is set |                                                                           Description
:--------|:-------------------|:-----------------------:|-------------------------------------------------------------------------------------:
Unknown  | Started            |           No            |                          The `PipelineRun` has just been picked up by the controller.
Unknown  | Running            |           No            |                  The `PipelineRun` has been validate and started to perform its work.
Unknown  | Cancelled          |           No            | The user requested the PipelineRun to be cancelled. Cancellation has not be done yet.
True     | Succeeded          |           Yes           |                                             The `PipelineRun` completed successfully.
True     | Completed          |           Yes           |             The `PipelineRun` completed successfully, one or more Tasks were skipped.
False    | Failed             |           Yes           |                        The `PipelineRun` failed because one of the `TaskRuns` failed.
False    | \[Error message\]  |           Yes           |                 The `PipelineRun` failed with a permanent error (usually validation).
False    | Cancelled          |           Yes           |                                         The `PipelineRun` was cancelled successfully.
False    | PipelineRunTimeout |           Yes           |                                                          The `PipelineRun` timed out.
False    | CreateRunFailed    |           Yes           |                                        The `PipelineRun` create run resources failed.

When a `PipelineRun` changes status, [events](events.md#pipelineruns) are triggered accordingly.

When a `PipelineRun` has `Tasks` that were `skipped`, the `reason` for skipping the task will be listed in the `Skipped Tasks` section of the `status` of the `PipelineRun`.

When a `PipelineRun` has `Tasks` with [`when` expressions](pipelines.md#guard-task-execution-using-when-expressions):
- If the `when` expressions evaluate to `true`, the `Task` is executed then the `TaskRun` and its resolved `when` expressions will be listed in the `Task Runs` section of the `status` of the `PipelineRun`.
- If the `when` expressions evaluate to `false`, the `Task` is skipped then its name and its resolved `when` expressions will be listed in the `Skipped Tasks` section of the `status` of the `PipelineRun`.

```yaml
Conditions:
  Last Transition Time:  2020-08-27T15:07:34Z
  Message:               Tasks Completed: 1 (Failed: 0, Cancelled 0), Skipped: 1
  Reason:                Completed
  Status:                True
  Type:                  Succeeded
Skipped Tasks:
  Name:       skip-this-task
  Reason:     When Expressions evaluated to false
  When Expressions:
    Input:     foo
    Operator:  in
    Values:
      bar
    Input:     foo
    Operator:  notin
    Values:
      foo
ChildReferences:
- Name: pipelinerun-to-skip-task-run-this-task
  Pipeline Task Name:  run-this-task
  Kind: TaskRun
```

The name of the `TaskRuns` and `Runs` owned by a `PipelineRun`  are univocally associated to the owning resource.
If a `PipelineRun` resource is deleted and created with the same name, the child `TaskRuns` will be created with the
same name as before. The base format of the name is `<pipelinerun-name>-<pipelinetask-name>`. If the `PipelineTask`
has a `Matrix`, the name will have an int suffix with format `<pipelinerun-name>-<pipelinetask-name>-<combination-id>`.
The name may vary according the logic of [`kmeta.ChildName`](https://pkg.go.dev/github.com/knative/pkg/kmeta#ChildName).

Some examples:

| `PipelineRun` Name                                       | `PipelineTask` Name                                          | `TaskRun` Names                                                                        |
|----------------------------------------------------------|--------------------------------------------------------------|----------------------------------------------------------------------------------------|
| pipeline-run                                             | task1                                                        | pipeline-run-task1                                                                     |
| pipeline-run                                             | task2-0123456789-0123456789-0123456789-0123456789-0123456789 | pipeline-runee4a397d6eab67777d4e6f9991cd19e6-task2-0123456789-0                        |
| pipeline-run-0123456789-0123456789-0123456789-0123456789 | task3                                                        | pipeline-run-0123456789-0123456789-0123456789-0123456789-task3                         |
| pipeline-run-0123456789-0123456789-0123456789-0123456789 | task2-0123456789-0123456789-0123456789-0123456789-0123456789 | pipeline-run-0123456789-012345607ad8c7aac5873cdfabe472a68996b5c                        |
| pipeline-run                                             | task4 (with 2x2 `Matrix`)                                    | pipeline-run-task1-0, pipeline-run-task1-2, pipeline-run-task1-3, pipeline-run-task1-4 |

### Marking off user errors

A user error in Tekton is any mistake made by user, such as a syntax error when specifying pipelines, tasks. User errors can occur in various stages of the Tekton pipeline, from authoring the pipeline configuration to executing the pipelines. They are currently explicitly labeled in the Run's conditions message, for example:

```yaml
# Failed PipelineRun with message labeled "[User error]"
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  ...
spec:
  ...
status:
  ...
  conditions:
  - lastTransitionTime: "2022-06-02T19:02:58Z"
    message: '[User error] PipelineRun default parameters is missing some parameters required by
      Pipeline pipelinerun-with-params''s parameters: pipelineRun missing parameters:
      [pl-param-x]'
    reason: 'ParameterMissing'
    status: "False"
    type: Succeeded
```

```console
~/pipeline$ tkn pr list
NAME                      STARTED         DURATION   STATUS
pipelinerun-with-params   5 seconds ago   0s         Failed(ParameterMissing)
```

## Cancelling a `PipelineRun`

To cancel a `PipelineRun` that's currently executing, update its definition
to mark it as "Cancelled". When you do so, the spawned `TaskRuns` are also marked
as cancelled, all associated `Pods` are deleted, and their `Retries` are not executed.
Pending `finally` tasks are not scheduled.

For example:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: go-example-git
spec:
  # […]
  status: "Cancelled"
```

## Gracefully cancelling a `PipelineRun`

To gracefully cancel a `PipelineRun` that's currently executing, update its definition
to mark it as "CancelledRunFinally". When you do so, the spawned `TaskRuns` are also marked
as cancelled, all associated `Pods` are deleted, and their `Retries` are not executed.
`finally` tasks are scheduled normally.

For example:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: go-example-git
spec:
  # […]
  status: "CancelledRunFinally"
```


## Gracefully stopping a `PipelineRun`

To gracefully stop a `PipelineRun` that's currently executing, update its definition
to mark it as "StoppedRunFinally". When you do so, the spawned `TaskRuns` are completed normally,
including executing their `retries`, but no new non-`finally` task is scheduled. `finally` tasks are executed afterwards.
For example:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: go-example-git
spec:
  # […]
  status: "StoppedRunFinally"
```

## Pending `PipelineRuns`

A `PipelineRun` can be created as a "pending" `PipelineRun` meaning that it will not actually be started until the pending status is cleared.

Note that a `PipelineRun` can only be marked "pending" before it has started, this setting is invalid after the `PipelineRun` has been started.

To mark a `PipelineRun` as pending, set `.spec.status` to `PipelineRunPending` when creating it:

```yaml
apiVersion: tekton.dev/v1 # or tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: go-example-git
spec:
  # […]
  status: "PipelineRunPending"
```

To start the PipelineRun, clear the `.spec.status` field. Alternatively, update the value to `Cancelled` to cancel it.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
