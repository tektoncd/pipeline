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
- [Cancelling a PipelineRun](#cancelling-a-pipelinerun)
- [Examples](#examples)

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
  - `timeout` - Specifies timeout after which the `PipelineRun` will fail.
  - [`nodeSelector`] - A selector which must be true for the pod to fit on a
    node. The selector which must match a node's labels for the pod to be
    scheduled on that node. More info:
    <https://kubernetes.io/docs/concepts/configuration/assign-pod-node/>
  - [`tolerations`] - Tolerations are applied to pods, and allow (but do not
    require) the pods to schedule onto nodes with matching taints. More info:
    <https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/>
  - [`affinity`] - The pod's scheduling constraints. More info:

    <https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#node-affinity-beta-feature>

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
