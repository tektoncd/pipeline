<!--
---
linkTitle: "Runs"
weight: 2
---
-->

# Runs

- [Overview](#overview)
- [Configuring a `Run`](#configuring-a-run)
  - [Specifying the target Custom Task](#specifying-the-target-custom-task)
  - [Specifying Parameters](#specifying-parameters)
  - [Specifying Workspaces, Service Account, and Pod Template](#specifying-workspaces-service-account-and-pod-template)
- [Monitoring execution status](#monitoring-execution-status)
  - [Monitoring `Results`](#monitoring-results)
- [Code examples](#code-examples)
  - [Example `Run` with a referenced custom task](#example-run-with-a-referenced-custom-task)
  - [Example `Run` with an unnamed custom task](#example-run-with-an-unnamed-custom-task)
  - [Example of specifying parameters](#example-of-specifying-parameters)

# Overview

A `Run` allows you to instantiate and execute a [Custom
Task](https://github.com/tektoncd/community/blob/master/teps/0002-custom-tasks.md),
which can be implemented by a custom task controller running on-cluster. Custom
Tasks can implement behavior that doesn't correspond directly to running a
workload in a `Pod` on the cluster. For Pod-based on-cluster workloads, you
should use a [`TaskRun`](taskruns.md).

In order for a `Run` to actually execute, there must be a custom task
controller running on the cluster that is responsible for watching and updating
`Run`s which reference their type. If no such controller is running, `Run`s
will have no `.status` value and no further action will be taken.

`Run`s are an **_experimental alpha feature_** and should be expected to change
in breaking ways or even be removed.

## Configuring a `Run`

A `Run` definition supports the following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1beta1`.
  - [`kind`][kubernetes-overview] - Identifies this resource object as a `Run` object.
  - [`metadata`][kubernetes-overview] - Specifies the metadata that uniquely identifies the
    `Run`, such as a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration for the `Run`.
    - [`ref`](#specifying-the-target-custom-task) - Specifies the type and
      (optionally) name of the custom task type to execute.
- Optional:
  - [`params`](#specifying-parameters) - Specifies the desired execution
    parameters for the custom task.
  - [`serviceAccountName`](#specifying-a-serviceaccount) - Specifies a `ServiceAccount`
    object that provides custom credentials for executing the `Run`.
  - [`workspaces`](#specifying-workspaces) - Specifies the physical volumes to use for the
    [`Workspaces`](workspaces.md) required by a custom task.
  - [`podTemplate`](#specifying-a-pod-template) - Specifies a [`Pod` template](podtemplates.md) to use
    to configure pods created by the custom task.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Specifying the target Custom Task

To specify the custom task type you want to execute in your `Run`, use the
`ref` field as shown below:

```yaml
spec:
  ref:
    apiVersion: example.dev/v1alpha1
    kind: Example
```

This initiates the execution of a `Run` of a custom task of type `Example`, in
the `example.dev` API group,  with the version `v1alpha1`.

You can also specify the `name` and optional `namespace` (default is `default`)
of a custom task resource object previously defined in the cluster.

```yaml
spec:
  ref:
    apiVersion: example.dev/v1alpha1
    kind: Example
    name: my-example
```

If the `ref` specifies a name, the custom task controller should look up the
`Example` resource with that name, and use that object to configure the
execution.

If the `ref` does not specify a name, the custom task controller might support
some default behavior for executing unnamed tasks.

In either case, if the named resource cannot be found, or if unnamed tasks are
not supported, the custom task controller should update the `Run`'s status to
indicate the error.

### Specifying `Parameters`

If a custom task supports [`parameters`](tasks.md#parameters), you can use the
`params` field in the `Run` to specify their values:

```yaml
spec:
  params:
  - name: my-param
    value: chicken
```

If the custom task controller knows how to interpret the parameter value, it
will do so. It might enforce that some parameter values must be specified, or
reject unknown parameter values.

### Specifying Workspaces, Service Account, and Pod Template

A `Run` object can specify workspaces, a service account name, or a pod template.
These are intended to be used with custom tasks that create Pods or other resources that embed a Pod specification.
The custom task can use these specifications to construct the Pod specification.
Not all custom tasks will support these values.
Consult the documentation of the custom task that you are using to determine whether these values apply.

#### Specifying workspaces

If the custom task supports it, you can provide [`Workspaces`](workspaces.md) to share data with the custom task.

```yaml
spec:
  workspaces:
  - name: my-workspace
    emptyDir: {}
```

Consult the documentation of the custom task that you are using to determine whether it supports workspaces and how to name them.

#### Specifying a ServiceAccount

If the custom task supports it, you can execute the `Run` with a specific set of credentials by
specifying a `ServiceAccount` object name in the `serviceAccountName` field in your `Run`
definition. If you do not explicitly specify this, the `Run` executes with the service account
specified in the `configmap-defaults` `ConfigMap`. If this default is not specified, `Runs`
will execute with the [`default` service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#use-the-default-service-account-to-access-the-api-server)
set for the target [`namespace`](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/).

```yaml
spec:
  serviceAccountName: my-account
```

Consult the documentation of the custom task that you are using to determine whether it supports a service account name.

#### Specifying a pod template

If the custom task supports it, you can specify a [`Pod` template](podtemplates.md) configuration that the custom task will
use to configure Pods (or other resources that embed a Pod specification) that it creates.

```yaml
spec:
  podTemplate:
    securityContext:
      runAsUser: 1001
```

Consult the documentation of the custom task that you are using to determine whether it supports a pod template.

## Monitoring execution status

As your `Run` executes, its `status` field accumulates information on the
execution of the `Run`. This information includes start and completion times,
and any output `results` reported by the custom task controller.

The following example shows the `status` field of a `Run` that has executed
successfully:

```yaml
completionTime: "2019-08-12T18:22:57Z"
conditions:
- lastTransitionTime: "2019-08-12T18:22:57Z"
  message: Execution was successful
  reason: Succeeded
  status: "True"
  type: Succeeded
startTime: "2019-08-12T18:22:51Z"
```

The following tables shows how to read the overall status of a `Run`:

`status`|Description
:-------|-----------:
<unset>|The custom task controller has not taken any action on the Run.
Unknown|The custom task controller has started execution and the Run is ongoing.
True|The Run completed successfully.
False|The Run completed unsuccessfully.

In any case, the custom task controller should populate the `reason` and
`message` fields to provide more information about the status of the execution.

### Monitoring `Results`

After the `Run` completes, the custom task controller can report output
values
in the `results` field:

```
results:
- name: my-result
  value: chicken
```

## Code examples

To better understand `Runs`, study the following code examples:

- [Example `Run` with a referenced custom task](#example-run-with-a-referenced-custom-task)
- [Example `Run` with an unnamed custom task](#example-run-with-an-unnamed-custom-task)
- [Example of specifying parameters](#example-of-specifying-parameters)

### Example `Run` with a referenced custom task

In this example, a `Run` named `my-example-run` invokes a custom task of the `v1alpha1`
version of the `Example` kind in the `example.dev` API group, with the name
`my-example-task`.

In this case the custom task controller is expected to look up the `Example`
resource named `my-example-task` and to use that configuration to configure the
execution of the `Run`.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Run
metadata:
  name: my-example-run
spec:
  ref:
    apiVersion: example.dev/v1alpha1
    kind: Example
    name: my-example-task
```

### Example `Run` with an unnamed custom task

In this example, a `Run` named `my-example-run` invokes a custom task of the `v1alpha1`
version of the `Example` kind in the `example.dev` API group, without a specified name.

In this case the custom task controller is expected to provide some default
behavior when the referenced task is unnamed.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Run
metadata:
  name: my-example-run
spec:
  ref:
    apiVersion: example.dev/v1alpha1
    kind: Example
```

### Example of specifying parameters

In this example, a `Run` named `my-example-run` invokes a custom task, and
specifies some parameter values to further configure the execution's behavior.

In this case the custom task controller is expected to validate and interpret
these parameter values and use them to configure the `Run`'s execution.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Run
metadata:
  name: my-example-run
spec:
  ref:
    apiVersion: example.dev/v1alpha1
    kind: Example
    name: my-example-task
  params:
  - name: my-first-param
    value: i'm number one
  - name: my-second-param
    value: close second
```
