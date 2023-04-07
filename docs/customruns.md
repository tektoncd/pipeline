<!--
---
linkTitle: "CustomRuns"
weight: 206
---
-->

# CustomRuns

- [Overview](#overview)
- [Configuring a `CustomRun`](#configuring-a-customrun)
  - [Specifying the target Custom Task](#specifying-the-target-custom-task)
  - [Cancellation](#cancellation)
  - [Specifying `Timeout`](#specifying-timeout)
  - [Specifying `Retries`](#specifying-retries)
  - [Specifying Parameters](#specifying-parameters)
  - [Specifying Workspaces](#specifying-workspaces)
  - [Specifying Service Account](#specifying-a-serviceaccount)
- [Monitoring execution status](#monitoring-execution-status)
  - [Status Reporting](#status-reporting)
  - [Monitoring `Results`](#monitoring-results)
- [Code examples](#code-examples)
  - [Example `CustomRun` with a referenced custom task](#example-customrun-with-a-referenced-custom-task)
  - [Example `CustomRun` with an unnamed custom task](#example-customrun-with-an-unnamed-custom-task)
  - [Example of specifying parameters](#example-of-specifying-parameters)

# Overview

*`v1beta1.CustomRun` has replaced `v1alpha1.Run` for executing `Custom Tasks`. Please refer to the [migration doc](migrating-v1alpha1.Run-to-v1beta1.CustomRun.md) for details
on updating `v1alpha1.Run` to `v1beta1.CustomRun` before upgrading to a release that does not support `v1alpha1.Run`.*

A `CustomRun` allows you to instantiate and execute a [Custom
Task](https://github.com/tektoncd/community/blob/main/teps/0002-custom-tasks.md),
which can be implemented by a custom task controller running on-cluster. Custom
Tasks can implement behavior that's independent of how Tekton TaskRun is implemented.

In order for a `CustomRun` to actually execute, there must be a custom task
controller running on the cluster that is responsible for watching and updating
`CustomRun`s which reference their type. If no such controller is running, `CustomRun`s
will have no `.status` value and no further action will be taken.

## Configuring a `CustomRun`

A `CustomRun` definition supports the following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version. Currently,only
    `tekton.dev/v1beta1` is supported.
  - [`kind`][kubernetes-overview] - Identifies this resource object as a `CustomRun` object.
  - [`metadata`][kubernetes-overview] - Specifies the metadata that uniquely identifies the
    `CustomRun`, such as a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration for the `CustomRun`.
    - [`customRef`](#1-specifying-the-target-custom-task-with-customref) - Specifies the type and
      (optionally) name of the custom task type to execute.
    - [`customSpec`](#2-specifying-the-target-custom-task-by-embedding-its-spec) - Embed the custom task resource spec
      directly in a `CustomRun`.
- Optional:
  - [`timeout`](#specifying-timeout) - specifies the maximum duration of a single execution
    of a `CustomRun`.
  - [`retries`](#specifying-retries) - specifies the number of retries to execute upon `CustomRun`failure.
  - [`params`](#specifying-parameters) - Specifies the desired execution
    parameters for the custom task.
  - [`serviceAccountName`](#specifying-a-serviceaccount) - Specifies a `ServiceAccount`
    object for executing the `CustomRun`.
  - [`workspaces`](#specifying-workspaces) - Specifies the physical volumes to use for the
    [`Workspaces`](workspaces.md) required by a custom task.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Specifying the target Custom Task

A custom task resource's `CustomSpec` may be directly embedded in the `CustomRun` or it may
be referred to by a `CustomRef`. But, not both at the same time.

1. [Specifying the target Custom Task with customRef](#specifying-the-target-custom-task-with-customref)
   Referring a custom task (i.e. `CustomRef` ) promotes reuse of custom task definitions.

2. [Specifying the target Custom Task by embedding its spec](#specifying-the-target-custom-task-by-embedding-its-customspec)
   Embedding a custom task (i.e. `CustomSpec` ) helps in avoiding name collisions with other users within the same namespace.
   Additionally, in a pipeline with multiple embedded custom tasks, the details of entire pipeline can be fetched in a
   single API request.

#### Specifying the target Custom Task with customRef

To specify the custom task type you want to execute in your `CustomRun`, use the
`customRef` field as shown below:

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: my-example-run
spec:
  customRef:
    apiVersion: example.dev/v1beta1
    kind: MyCustomKind
  params:
  - name: duration
    value: 10s
```

When this `CustomRun` is created, the Custom Task controller responsible for
reconciling objects of kind "MyCustomKind" in the "example.dev/v1beta1" api group
will execute it based on the input params.

You can also specify the `name` and optional `namespace` (default is `default`)
of a custom task resource object previously defined in the cluster.

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: my-example-run
spec:
  customRef:
    apiVersion: example.dev/v1beta1
    kind: Example
    name: an-existing-example-task
```

If the `customRef` specifies a name, the custom task controller should look up the
`Example` resource with that name, and use that object to configure the
execution.

If the `customRef` does not specify a name, the custom task controller might support
some default behavior for executing unnamed tasks.

In either case, if the named resource cannot be found, or if unnamed tasks are
not supported, the custom task controller should update the `CustomRun`'s status to
indicate the error.

####  Specifying the target Custom Task by embedding its customSpec

To specify the custom task spec, it can be embedded directly into a
`CustomRun`'s spec as shown below:

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: embedded-run
spec:
  customSpec:
    apiVersion: example.dev/v1beta1
    kind: Example
    spec:
      field1: value1
      field2: value2
```

When this `CustomRun` is created, the custom task controller responsible for
reconciling objects of kind `Example` in the `example.dev` api group will
execute it.

#### Developer guide for custom controllers supporting `customSpec`.

1. A custom controller may or may not support a `Spec`. In cases where it is
   not supported the custom controller should respond with proper validation
   error.

2. Validation of the fields of the custom task is delegated to the custom
   task controller. It is recommended to implement validations as asynchronous
   (i.e. at reconcile time), rather than part of the webhook. Using a webhook
   for validation is problematic because, it is not possible to filter custom
   task resource objects before validation step, as a result each custom task
   resource has to undergo validation by all the installed custom task
   controllers.

3. A custom task may have an empty spec, but cannot have an empty
   `ApiVersion` and `Kind`. Custom task controllers should handle
   an empty spec, either with a default behaviour, in a case no default
   behaviour is supported then, appropriate validation error should be
   updated to the `CustomRun`'s status.

### Cancellation

The custom task is responsible for implementing `cancellation` to support pipelineRun level `timeouts` and `cancellation`. If the Custom Task implementor does not support cancellation via `.spec.status`, `Pipeline` **can not** timeout within the specified interval/duration and **can not** be cancelled as expected upon request.

Pipeline Controller sets the `spec.Status` and `spec.StatusMessage` to signal `CustomRuns` about the `Cancellation`, while `CustomRun` controller updates its `status.conditions` as following once noticed the change on `spec.Status`.

```yaml
status
  conditions:
  - type: Succeeded
    status: False
    reason: CustomRunCancelled
```

### Specifying `Timeout`

A custom task specification can be created with `Timeout` as follows:

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  generateName: simpleexample
spec:
  timeout: 10s # set timeouts here.
  params:
    - name: searching
      value: the purpose of my existence
  customRef:
    apiVersion: custom.tekton.dev/v1alpha1
    kind: Example
    name: exampleName
```

Supporting timeouts is optional but recommended.

#### Developer guide for custom controllers supporting `Timeout`

1. Tekton controllers will never directly update the status of the `CustomRun`, 
   it is the responsibility of the custom task controller to support timeout.
   If timeout is not supported, it's the responsibility of the custom task
   controller to reject `CustomRun`s that specify a timeout value.
2. When `CustomRun.Spec.Status` is updated to `RunCancelled`, the custom task controller
   MUST cancel the `CustomRun`. Otherwise, pipeline-level `Timeout` and
   `Cancellation` won't work for the Custom Task.
3. A Custom Task controller can watch for this status update
   (i.e. `CustomRun.Spec.Status == RunCancelled`) and or `CustomRun.HasTimedOut()`
   and take any corresponding actions (i.e. a clean up e.g., cancel a cloud build,
   stop the waiting timer, tear down the approval listener). 
4. Once resources or timers are cleaned up, while it is **REQUIRED** to set a
   `conditions` on the `CustomRun`'s `status` of `Succeeded/False` with an optional
   `Reason` of `CustomRunTimedOut`.
5. `Timeout` is specified for each `retry attempt` instead of all `retries`.

### Specifying `Retries`

A custom task specification can be created with `Retries` as follows:

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  generateName: simpleexample
spec:
  retries: 3 # set retries
  params:
    - name: searching
      value: the purpose of my existence
  customRef:
    apiVersion: custom.tekton.dev/v1alpha1
    kind: Example
    name: exampleName
```

Supporting retries is optional but recommended.

#### Developer guide for custom controllers supporting `retries`

1. Tekton controller only depends on `ConditionSucceeded` to determine the 
   termination status of a `CustomRun`, therefore Custom task implementors
   MUST NOT set `ConditionSucceeded` to `False` until all retries are exhausted.
2. Those custom tasks who do not wish to support retry, can simply ignore it.
3. It is recommended, that custom task should update the field `RetriesStatus`
   of a `CustomRun` on each retry performed by the custom task.
4. Tekton controller does not validate that number of entries in `RetriesStatus`
   is same as specified value of retries count.

### Specifying `Parameters`

If a custom task supports [`parameters`](tasks.md#parameters), you can use the
`params` field in the `CustomRun` to specify their values:

```yaml
spec:
  params:
    - name: my-param
      value: chicken
```

If the custom task controller knows how to interpret the parameter value, it
will do so. It might enforce that some parameter values must be specified, or
reject unknown parameter values.

### Specifying workspaces

If the custom task supports it, you can provide [`Workspaces`](workspaces.md) to share data with the custom task .

```yaml
spec:
  workspaces:
    - name: my-workspace
      emptyDir: {}
```

Consult the documentation of the custom task that you are using to determine whether it supports workspaces and how to name them.

### Specifying a ServiceAccount

If the custom task supports it, you can execute the `CustomRun` with a specific set of credentials by
specifying a `ServiceAccount` object name in the `serviceAccountName` field in your `CustomRun`
definition. If you do not explicitly specify this, the `CustomRun` executes with the service account
specified in the `configmap-defaults` `ConfigMap`. If this default is not specified, `CustomRuns`
will execute with the [`default` service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#use-the-default-service-account-to-access-the-api-server)
set for the target [`namespace`](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/).

```yaml
spec:
  serviceAccountName: my-account
```

Consult the documentation of the custom task that you are using to determine whether it supports a service account name.

## Monitoring execution status

As your `CustomRun` executes, its `status` field accumulates information on the
execution of the `CustomRun`. This information includes the current state of the
`CustomRun`, start and completion times, and any output `results` reported by
the custom task controller.

### Status Reporting

When the `CustomRun<Example>` is validated and created, the Custom Task controller will be notified and is expected to begin doing some operation. When the operation begins, the controller **MUST** update the `CustomRun`'s `.status.conditions` to report that it's ongoing:

```yaml
status
  conditions:
  - type: Succeeded
    status: Unknown
```

When the operation completes, if it was successful, the condition **MUST** report `status: True`, and optionally a brief `reason` and human-readable `message`:

```yaml
status
  conditions:
  - type: Succeeded
    status: True
    reason: ExampleComplete # optional
    message: Yay, good times # optional
```

If the operation was _unsuccessful_, the condition **MUST** report `status: False`, and optionally a `reason` and human-readable `message`:

```yaml
status
  conditions:
  - type: Succeeded
    status: False
    reason: ExampleFailed # optional
    message: Oh no bad times # optional
```

If the `CustomRun` was _cancelled_, the condition **MUST** report `status: False`, `reason: CustomRunCancelled`, and optionally a human-readable `message`:

```yaml
status
  conditions:
  - type: Succeeded
    status: False
    reason: CustomRunCancelled
    message: Oh it's cancelled # optional
```

The following tables shows the overall status of a `CustomRun`:

`status`|Description
:-------|:-----------
<unset>|The custom task controller has not taken any action on the `CustomRun`.
Unknown|The custom task controller has started execution and the `CustomRun` is ongoing.
True|The `CustomRun` completed successfully.
False|The `CustomRun` completed unsuccessfully, and all retries were exhausted.

The `CustomRun` type's `.status` will also allow controllers to report other fields, such as `startTime`, `completionTime`, `results` (see below), and arbitrary context-dependent fields the Custom Task author wants to report. A fully-specified `CustomRun` status might look like:

```
status
  conditions:
  - type: Succeeded
    status: True
    reason: ExampleComplete
    message: Yay, good times
  completionTime: "2020-06-18T11:55:01Z"
  startTime: "2020-06-18T11:55:01Z"
  results:
  - name: first-name
    value: Bob
  - name: last-name
    value: Smith
  arbitraryField: hello world
  arbitraryStructuredField:
    listOfThings: ["a", "b", "c"]
```

### Monitoring `Results`

After the `CustomRun` completes, the custom task controller can report output
values in the `results` field:

```
results:
- name: my-result
  value: chicken
```

## Code examples

To better understand `CustomRuns`, study the following code examples:

- [Example `CustomRun` with a referenced custom task](#example-customrun-with-a-referenced-custom-task)
- [Example `CustomRun` with an unnamed custom task](#example-customrun-with-an-unnamed-custom-task)
- [Example of specifying parameters](#example-of-specifying-parameters)

### Example `CustomRun` with a referenced custom task

In this example, a `CustomRun` named `my-example-run` invokes a custom task of the `v1alpha1`
version of the `Example` kind in the `example.dev` API group, with the name
`my-example-task`.

In this case the custom task controller is expected to look up the `Example`
resource named `my-example-task` and to use that configuration to configure the
execution of the `CustomRun`.

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: my-example-run
spec:
  customRef:
    apiVersion: example.dev/v1alpha1
    kind: Example
    name: my-example-task
```

### Example `CustomRun` with an unnamed custom task

In this example, a `CustomRun` named `my-example-run` invokes a custom task of the `v1alpha1`
version of the `Example` kind in the `example.dev` API group, without a specified name.

In this case the custom task controller is expected to provide some default
behavior when the referenced task is unnamed.

```yaml
apiVersion: tekton.dev/v1beta1
kind: CustomRun
metadata:
  name: my-example-run
spec:
  customRef:
    apiVersion: example.dev/v1alpha1
    kind: Example
```

### Example of specifying parameters

In this example, a `CustomRun` named `my-example-run` invokes a custom task, and
specifies some parameter values to further configure the execution's behavior.

In this case the custom task controller is expected to validate and interpret
these parameter values and use them to configure the `CustomRun`'s execution.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: CustomRun
metadata:
  name: my-example-run
spec:
  customRef:
    apiVersion: example.dev/v1alpha1
    kind: Example
    name: my-example-task
  params:
    - name: my-first-param
      value: i'm number one
    - name: my-second-param
      value: close second
```
