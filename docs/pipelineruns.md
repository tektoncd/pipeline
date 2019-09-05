# PipelineRuns

This document defines `PipelineRuns` and their capabilities.

On its own, a [`Pipeline`](pipelines.md) declares what [`Tasks`](tasks.md) to
run, and [the order they run in](pipelines.md#ordering). To execute the `Tasks`
in the `Pipeline`, you must create a `PipelineRun`.

Creation of a `PipelineRun` will trigger the creation of
[`TaskRuns`](taskruns.md) for each `Task` in your pipeline.

---

- [Syntax](#syntax)
  - [Resources](#resources)
  - [Service account](#service-account)
  - [Service accounts](#service-accounts)
  - [Pod Template](#pod-template)
- [Cancelling a PipelineRun](#cancelling-a-pipelinerun)
- [Examples](https://github.com/tektoncd/pipeline/tree/master/examples/pipelineruns)
- [Logs](logs.md)

## Syntax

To define a configuration file for a `PipelineRun` resource, you can specify the
following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1alpha1`.
  - [`kind`][kubernetes-overview] - Specify the `PipelineRun` resource object.
  - [`metadata`][kubernetes-overview] - Specifies data to uniquely identify the
    `PipelineRun` resource object, for example a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    your `PipelineRun` resource object.
    - `pipelineRef` - Specifies the [`Pipeline`](pipelines.md) you want to run.
- Optional:

  - [`resources`](#resources) - Specifies which
    [`PipelineResources`](resources.md) to use for this `PipelineRun`.
  - [`serviceAccount`](#service-account) - Specifies a `ServiceAccount` resource
    object that enables your build to run with the defined authentication
    information.
  - [`serviceAccounts`](#service-accounts) - Specifies a list of `ServiceAccount` 
    and `PipelineTask` pairs that enable you to overwrite `ServiceAccount` for concrete `PipelineTask`.
  - [`timeout`] - Specifies timeout after which the `PipelineRun` will fail. If the value of
    `timeout` is empty, the default timeout will be applied. If the value is set to 0,
    there is no timeout. `PipelineRun` shares the same default timeout as `TaskRun`. You can
    follow the instruction [here](taskruns.md#Configuring-default-timeout) to configure the
    default timeout, the same way as `TaskRun`.
  - [`podTemplate`](#pod-template) - Specifies a subset of
    [`PodSpec`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#pod-v1-core)
	configuration that will be used as the basis for the `Task` pod.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Resources

When running a [`Pipeline`](pipelines.md), you will need to specify the
[`PipelineResources`](resources.md) to use with it. One `Pipeline` may need to
be run with different `PipelineResources` in cases such as:

- When triggering the run of a `Pipeline` against a pull request, the triggering
  system must specify the commitish of a git `PipelineResource` to use
- When invoking a `Pipeline` manually against one's own setup, one will need to
  ensure that one's own GitHub fork (via the git `PipelineResource`), image
  registry (via the image `PipelineResource`) and Kubernetes cluster (via the
  cluster `PipelineResource`).

Specify the `PipelineResources` in the PipelineRun using the `resources` section
in the `PipelineRun` spec, for example:

```yaml
spec:
  resources:
    - name: source-repo
      resourceRef:
        name: skaffold-git
    - name: web-image
      resourceRef:
        name: skaffold-image-leeroy-web
    - name: app-image
      resourceRef:
        name: skaffold-image-leeroy-app
```

Or you can embed the spec of the `Resource` directly in the `PipelineRun`:


```yaml
spec:
  resources:
    - name: source-repo
      resourceSpec:
        type: git
        params:
          - name: revision
            value: v0.32.0
          - name: url
            value: https://github.com/GoogleContainerTools/skaffold
    - name: web-image
      resourceSpec:
        type: image
        params:
          - name: url
            value: gcr.io/christiewilson-catfactory/leeroy-web
    - name: app-image
      resourceSpec:
        type: image
        params:
          - name: url
            value: gcr.io/christiewilson-catfactory/leeroy-app
```

### Service Account

Specifies the `name` of a `ServiceAccount` resource object. Use the
`serviceAccount` field to run your `Pipeline` with the privileges of the
specified service account. If no `serviceAccount` field is specified, your
resulting `TaskRuns` run using the
[`default` service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#use-the-default-service-account-to-access-the-api-server)
that is in the
[namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)
of the `TaskRun` resource object.

For examples and more information about specifying service accounts, see the
[`ServiceAccount`](./auth.md) reference topic.

### Service Accounts

Specifies the list of `ServiceAccount` and `PipelineTask` pairs. Specified 
`PipelineTask` will be run with configured `ServiceAccount`, 
overwriting [`serviceAccount`](#service-account) configuration, for example:

```yaml
spec:
  serviceAccount: sa-1
  serviceAccounts:
    - taskName: build-task
      serviceAccount: sa-for-build
```
If used with this `Pipeline`, `test-task` will use the `ServiceAccount` `sa-1`, while `build-task` will use `sa-for-build`.

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

### Pod Template

Specifies a subset of
[`PodSpec`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#pod-v1-core)
configuration that will be used as the basis for the `Task` pod. This
allows to customize some Pod specific field per `Task` execution, aka
`TaskRun`. The current field supported are:

- `nodeSelector`: a selector which must be true for the pod to fit on
  a node, see [here](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/).
- `tolerations`: allow (but do not require) the pods to schedule onto
  nodes with matching taints.
- `affinity`: allow to constrain which nodes your pod is eligible to
  be scheduled on, based on labels on the node.
- `securityContext`: pod-level security attributes and common
  container settings, like `runAsUser` or `selinux`.
- `volumes`: list of volumes that can be mounted by containers
  belonging to the pod. This lets the user of a Task define which type
  of volume to use for a Task `volumeMount`

In the following example, the Task is defined with a `volumeMount`
(`my-cache`), that is provided by the PipelineRun, using a
PersistenceVolumeClaim. The Pod will also run as a non-root user.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: myTask
spec:
  steps:
    - name: write something
      image: ubuntu
      command: ["bash", "-c"]
      args: ["echo 'foo' > /my-cache/bar"]
      volumeMounts:
        - name: my-cache
          mountPath: /my-cache
---
apiVersion: tekton.dev/v1alpha1
kind: Pipeline
metadata:
  name: myPipeline
spec:
  tasks:
    - name: task1
      taskRef:
        name: myTask
---
apiVersion: tekton.dev/v1alpha1
kind: PipelineRun
metadata:
  name: myPipelineRun
spec:
  pipelineRef:
    name: myPipeline
  podTemplate:
    securityContext:
      runAsNonRoot: true
    volumes:
    - name: my-cache
      persistentVolumeClaim:
        claimName: my-volume-claim
```

## Cancelling a PipelineRun

In order to cancel a running pipeline (`PipelineRun`), you need to update its
spec to mark it as cancelled. Related `TaskRun` instances will be marked as
cancelled and running Pods will be deleted.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineRun
metadata:
  name: go-example-git
spec:
  # […]
  status: "PipelineRunCancelled"
```

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
