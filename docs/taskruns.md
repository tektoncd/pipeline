<!--
---
linkTitle: "TaskRuns"
weight: 2
---
-->
# TaskRuns

Use the `TaskRun` resource object to create and run on-cluster processes to
completion.

To create a `TaskRun`, you must first create a [`Task`](tasks.md) which
specifies one or more container images that you have implemented to perform and
complete a task.

A `TaskRun` runs until all `steps` have completed or until a failure occurs.

---

- [TaskRuns](#taskruns)
  - [Syntax](#syntax)
    - [Specifying a task](#specifying-a-task)
    - [Parameters](#parameters)
    - [Providing resources](#providing-resources)
    - [Configuring Default Timeout](#configuring-default-timeout)
    - [Service Account](#service-account)
  - [Pod Template](#pod-template)
  - [Workspaces](#workspaces)
  - [Status](#status)
    - [Steps](#steps)
    - [Results](#results)
  - [Cancelling a TaskRun](#cancelling-a-taskrun)
  - [Examples](#examples)
    - [Example TaskRun](#example-taskrun)
    - [Example with embedded specs](#example-with-embedded-specs)
    - [Example Task Reuse](#example-task-reuse)
      - [Using a `ServiceAccount`](#using-a-serviceaccount)
  - [Sidecars](#sidecars)
  - [LimitRanges](#limitranges)

---

## Syntax

To define a configuration file for a `TaskRun` resource, you can specify the
following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1beta1`.
  - [`kind`][kubernetes-overview] - Specify the `TaskRun` resource object.
  - [`metadata`][kubernetes-overview] - Specifies data to uniquely identify the
    `TaskRun` resource object, for example a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    your `TaskRun` resource object.
    - [`taskRef` or `taskSpec`](#specifying-a-task) - Specifies the details of
      the [`Task`](tasks.md) you want to run
- Optional:

  - [`serviceAccountName`](#service-account) - Specifies a `ServiceAccount` resource
    object that enables your build to run with the defined authentication
    information. When a `ServiceAccount` isn't specified, the `default-service-account`
    specified in the configmap `config-defaults` will be applied.
  - [`params`](#parameters) - Specifies parameters values
  - [`resources`](#providing-resources) - Specifies `PipelineResource` values
    - [`inputs`] - Specifies input resources
    - [`outputs`] - Specifies output resources
  - [`timeout`] - Specifies timeout after which the `TaskRun` will fail. If the value of
    `timeout` is empty, the default timeout will be applied. If the value is set to 0,
    there is no timeout. You can also follow the instruction [here](#Configuring-default-timeout)
    to configure the default timeout.
  - [`podTemplate`](#pod-template) - Specifies a [pod template](./podtemplates.md) that will be used as the basis for the `Task` pod.
  - [`workspaces`](#workspaces) - Specify the actual volumes to use for the
    [workspaces](workspaces.md#declaring-workspaces-in-tasks) declared by a `Task`

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

### Specifying a task

Since a `TaskRun` is an invocation of a [`Task`](tasks.md), you must specify
what `Task` to invoke.

You can do this by providing a reference to an existing `Task`:

```yaml
spec:
  taskRef:
    name: read-task
```

Or you can embed the spec of the `Task` directly in the `TaskRun`:

```yaml
spec:
  taskSpec:
    resources:
      inputs:
        - name: workspace
          type: git
    steps:
      - name: build-and-push
        image: gcr.io/kaniko-project/executor:v0.17.1
        # specifying DOCKER_CONFIG is required to allow kaniko to detect docker credential
        env:
          - name: "DOCKER_CONFIG"
            value: "/tekton/home/.docker/"
        command:
          - /kaniko/executor
        args:
          - --destination=gcr.io/my-project/gohelloworld
```

### Parameters

If a `Task` has [`parameters`](tasks.md#parameters), you can specify values for
them using the `params` section:

```yaml
spec:
  params:
    - name: flags
      value: -someflag
```

If a parameter does not have a default value, it must be specified.

### Providing resources

If a `Task` requires [input resources](tasks.md#input-resources) or
[output resources](tasks.md#output-resources), they must be provided to run the
`Task`.

They can be provided via references to existing
[`PipelineResources`](resources.md):

```yaml
spec:
  resources:
    inputs:
      - name: workspace
        resourceRef:
          name: java-git-resource
    outputs:
      - name: image
        resourceRef:
          name: my-app-image
```

Or by embedding the specs of the resources directly:

```yaml
spec:
  resources:
    inputs:
      - name: workspace
        resourceSpec:
          type: git
          params:
            - name: url
              value: https://github.com/pivotal-nader-ziada/gohelloworld
```

The `paths` field can be used to [override the paths to a resource](./resources.md#overriding-where-resources-are-copied-from)

### Configuring Default Timeout

You can configure the default timeout by changing the value of
`default-timeout-minutes` in
[`config/config-defaults.yaml`](./../config/config-defaults.yaml).

The `timeout` format is a `duration` as validated by Go's
[`ParseDuration`](https://golang.org/pkg/time/#ParseDuration), valid format for
examples are :

- `1h30m`
- `1h`
- `1m`
- `60s`

The default timeout is 60 minutes, if `default-timeout-minutes` is not
available. There is no timeout by default, if `default-timeout-minutes` is set
to 0.

### Service Account

Specifies the `name` of a `ServiceAccount` resource object. Use the
`serviceAccountName` field to run your `Task` with the privileges of the specified
service account. If no `serviceAccountName` field is specified, your `Task` runs
using the service account specified in the ConfigMap `configmap-defaults`
which if absent will default to
[`default` service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#use-the-default-service-account-to-access-the-api-server)
that is in the [namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)
of the `TaskRun` resource object.

For examples and more information about specifying service accounts, see the
[`ServiceAccount`](./auth.md) reference topic.

## Pod Template

Specifies a [pod template](./podtemplates.md) configuration that will be used as the basis for the `Task` pod. This
allows to customize some Pod specific field per `Task` execution, aka `TaskRun`.

In the following example, the Task is defined with a `volumeMount`
(`my-cache`), that is provided by the TaskRun, using a
PersistentVolumeClaim. The SchedulerName has also been provided to define which scheduler should be used to
dispatch the Pod. The Pod will also run as a non-root user.

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: mytask
  namespace: default
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
kind: TaskRun
metadata:
  name: mytaskrun
  namespace: default
spec:
  taskRef:
    name: mytask
  podTemplate:
    schedulerName: volcano
    securityContext:
      runAsNonRoot: true
    volumes:
    - name: my-cache
      persistentVolumeClaim:
        claimName: my-volume-claim
```

## Workspaces

For a `TaskRun` to execute a `Task` that declares `workspaces` it needs to map
those `workspaces` to actual physical volumes.

Here are the relevant fields of a `TaskRun` spec when providing a
`PersistentVolumeClaim` as a workspace:

```yaml
workspaces:
- name: myworkspace # must match workspace name in Task
  persistentVolumeClaim:
    claimName: mypvc # this PVC must already exist
  subPath: my-subdir
```

For more examples and complete documentation on configuring `workspaces` in
`TaskRun`s see [workspaces.md](./workspaces.md#providing-workspaces-with-taskruns).

Tekton supports several different kinds of `Volume` in `Workspaces`. For a list of
the different kinds see the section on
[`VolumeSources` for Workspaces](workspaces.md#volumesources-for-workspaces).

_For a complete example see [the Workspaces TaskRun](../examples/v1beta1/taskruns/workspace.yaml)
in the examples directory._

## Status

As a TaskRun completes, its `status` field is filled in with relevant information for
the overall run, as well as each step.

The following example shows a completed TaskRun and its `status` field:

```yaml
completionTime: "2019-08-12T18:22:57Z"
conditions:
- lastTransitionTime: "2019-08-12T18:22:57Z"
  message: All Steps have completed executing
  reason: Succeeded
  status: "True"
  type: Succeeded
podName: status-taskrun-pod-6488ef
startTime: "2019-08-12T18:22:51Z"
steps:
- container: step-hello
  imageID: docker-pullable://busybox@sha256:895ab622e92e18d6b461d671081757af7dbaa3b00e3e28e12505af7817f73649
  name: hello
  terminated:
    containerID: docker://d5a54f5bbb8e7a6fd3bc7761b78410403244cf4c9c5822087fb0209bf59e3621
    exitCode: 0
    finishedAt: "2019-08-12T18:22:56Z"
    reason: Completed
    startedAt: "2019-08-12T18:22:54Z"
  ```

Fields include start and stop times for the `TaskRun` and each `Step` and exit codes.
For each step we also include the fully-qualified image used, with the digest.

If any pods have been [`OOMKilled`](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/)
by Kubernetes, the `Taskrun` will be marked as failed even if the exit code is 0.

### Steps

If multiple `steps` are defined in the `Task` invoked by the `TaskRun`, we will see the
`status.steps` of the `TaskRun` displayed in the same order as they are defined in
`spec.steps` of the `Task`, when the `TaskRun` is accessed by the `get` command, e.g.
`kubectl get taskrun <name> -o yaml`. Replace \<name\> with the name of the `TaskRun`.

### Results

If one or more `results` are defined in the `Task` invoked by the `TaskRun`, we will get a new entry
`Task Results` added to the status.
Here is an example:

```yaml
Status:
  # […]
  Steps:
  # […]
  Task Results:
    Name:   current-date-human-readable
    Value:  Thu Jan 23 16:29:06 UTC 2020

    Name:   current-date-unix-timestamp
    Value:  1579796946

```

Results will be printed verbatim; any new lines or other whitespace returned as part of the result will be included in the output.

## Cancelling a TaskRun

In order to cancel a running task (`TaskRun`), you need to update its spec to
mark it as cancelled. Running Pods will be deleted.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: go-example-git
spec:
  # […]
  status: "TaskRunCancelled"
```

## Examples

- [Example TaskRun](#example-taskrun)
- [Example TaskRun with embedded specs](#example-with-embedded-specs)
- [Example Task reuse](#example-task-reuse)

### Example TaskRun

To run a `Task`, create a new `TaskRun` which defines all inputs, outputs that
the `Task` needs to run. Below is an example where Task `read-task` is run by
creating `read-repo-run`. Task `read-task` has git input resource and TaskRun
`read-repo-run` includes reference to `go-example-git`.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: go-example-git
spec:
  type: git
  params:
    - name: url
      value: https://github.com/pivotal-nader-ziada/gohelloworld
---
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: read-task
spec:
  resources:
    inputs:
      - name: workspace
        type: git
  steps:
    - name: readme
      image: ubuntu
      script: cat workspace/README.md
---
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: read-repo-run
spec:
  taskRef:
    name: read-task
  resources:
    inputs:
      - name: workspace
        resourceRef:
          name: go-example-git
```

### Example with embedded specs

Another way of running a Task is embedding the TaskSpec in the taskRun yaml.
This can be useful for "one-shot" style runs, or debugging. TaskRun resource can
include either Task reference or TaskSpec but not both. Below is an example
where `build-push-task-run-2` includes `TaskSpec` and no reference to Task.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: go-example-git
spec:
  type: git
  params:
    - name: url
      value: https://github.com/pivotal-nader-ziada/gohelloworld
---
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: build-push-task-run-2
spec:
  resources:
    inputs:
      - name: workspace
        resourceRef:
          name: go-example-git
  taskSpec:
    resources:
      inputs:
        - name: workspace
          type: git
    steps:
      - name: build-and-push
        image: gcr.io/kaniko-project/executor:v0.17.1
        # specifying DOCKER_CONFIG is required to allow kaniko to detect docker credential
        env:
          - name: "DOCKER_CONFIG"
            value: "/tekton/home/.docker/"
        command:
          - /kaniko/executor
        args:
          - --destination=gcr.io/my-project/gohelloworld
```

Input and output resources can also be embedded without creating Pipeline
Resources. TaskRun resource can include either a Pipeline Resource reference or
a Pipeline Resource Spec but not both. Below is an example where Git Pipeline
Resource Spec is provided as input for TaskRun `read-repo`.

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: read-repo
spec:
  taskRef:
    name: read-task
  resources:
    inputs:
      - name: workspace
        resourceSpec:
          type: git
          params:
            - name: url
              value: https://github.com/pivotal-nader-ziada/gohelloworld
```

**Note**: TaskRun can embed both TaskSpec and resource spec at the same time.
The `TaskRun` will also serve as a record of the history of the invocations of
the `Task`.

### Example Task Reuse

For the sake of illustrating re-use, here are several example
[`TaskRuns`](taskruns.md) (including referenced
[`PipelineResources`](resources.md)) instantiating the
[`Task` (`dockerfile-build-and-push`) in the `Task` example docs](tasks.md#example-task).

Build `mchmarny/rester-tester`:

```yaml
# The PipelineResource
metadata:
  name: mchmarny-repo
spec:
  type: git
  params:
    - name: url
      value: https://github.com/mchmarny/rester-tester.git
```

```yaml
# The TaskRun
spec:
  taskRef:
    name: dockerfile-build-and-push
  params:
    - name: IMAGE
      value: gcr.io/my-project/rester-tester
  resources:
    inputs:
      - name: workspace
        resourceRef:
          name: mchmarny-repo
```

Build `googlecloudplatform/cloud-builder`'s `wget` builder:

```yaml
# The PipelineResource
metadata:
  name: cloud-builder-repo
spec:
  type: git
  params:
    - name: url
      value: https://github.com/googlecloudplatform/cloud-builders.git
```

```yaml
# The TaskRun
spec:
  taskRef:
    name: dockerfile-build-and-push
  params:
    - name: IMAGE
      value: gcr.io/my-project/wget
    # Optional override to specify the subdirectory containing the Dockerfile
    - name: DIRECTORY
      value: /workspace/wget
  resources:
    inputs:
      - name: workspace
        resourceRef:
          name: cloud-builder-repo
```

Build `googlecloudplatform/cloud-builder`'s `docker` builder with `17.06.1`:

```yaml
# The PipelineResource
metadata:
  name: cloud-builder-repo
spec:
  type: git
  params:
    - name: url
      value: https://github.com/googlecloudplatform/cloud-builders.git
```

```yaml
# The TaskRun
spec:
  taskRef:
    name: dockerfile-build-and-push
  params:
    - name: IMAGE
      value: gcr.io/my-project/docker
    # Optional overrides
    - name: DIRECTORY
      value: /workspace/docker
    - name: DOCKERFILE_NAME
      value: Dockerfile-17.06.1
  resources:
    inputs:
      - name: workspace
        resourceRef:
          name: cloud-builder-repo
```

#### Using a `ServiceAccount`

Specifying a `ServiceAccount` to access a private `git` repository:

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: test-task-with-serviceaccount-git-ssh
spec:
  serviceAccountName: test-task-robot-git-ssh
  resources:
    inputs:
      - name: workspace
        type: git
  steps:
    - name: config
      image: ubuntu
      command: ["/bin/bash"]
      args: ["-c", "cat README.md"]
```

Where `serviceAccountName: test-build-robot-git-ssh` references the following
`ServiceAccount`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-task-robot-git-ssh
secrets:
  - name: test-git-ssh
```

And `name: test-git-ssh`, references the following `Secret`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: test-git-ssh
  annotations:
    tekton.dev/git-0: github.com
type: kubernetes.io/ssh-auth
data:
  # Generated by:
  # cat id_rsa | base64 -w 0
  ssh-privatekey: LS0tLS1CRUdJTiBSU0EgUFJJVk.....[example]
  # Generated by:
  # ssh-keyscan github.com | base64 -w 0
  known_hosts: Z2l0aHViLmNvbSBzc2g.....[example]
```

Specifies the `name` of a `ServiceAccount` resource object. Use the
`serviceAccountName` field to run your `Task` with the privileges of the specified
service account. If no `serviceAccountName` field is specified, your `Task` runs
using the
[`default` service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#use-the-default-service-account-to-access-the-api-server)
that is in the
[namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)
of the `Task` resource object.

For examples and more information about specifying service accounts, see the
[`ServiceAccount`](./auth.md) reference topic.

## Sidecars

A well-established pattern in Kubernetes is that of the "sidecar" - a
container which runs alongside your workloads to provide ancillary support.
Typical examples of the sidecar pattern are logging daemons, services to
update files on a shared volume, and network proxies.

Tekton will happily work with sidecars injected into a TaskRun's
pods but the behavior is a bit nuanced: When TaskRun's steps are complete
any sidecar containers running inside the Pod will be terminated. In
order to terminate the sidecars they will be restarted with a new
"nop" image that quickly exits. The result will be that your TaskRun's
Pod will include the sidecar container with a Retry Count of 1 and
with a different container image than you might be expecting.

Note: There are some known issues with the existing implementation of sidecars:

- The configured "nop" image must not provide the command that the
sidecar is expected to run. If it does provide the command then it will
not exit. This will result in the sidecar running forever and the Task
eventually timing out. [This bug is being tracked in issue 1347](https://github.com/tektoncd/pipeline/issues/1347)
is the issue where this bug is being tracked.

- `kubectl get pods` will show a TaskRun's Pod as "Completed" if a sidecar
exits successfully and "Error" if the sidecar exits with an error, regardless
of how the step containers inside that pod exited. This issue only manifests
with the `get pods` command. The Pod description will instead show a Status of
Failed and the individual container statuses will correctly reflect how and why
they exited.

## LimitRanges

In order to request the minimum amount of resources needed to support the containers
for `steps` that are part of a `TaskRun`, Tekton only requests the maximum values for CPU,
memory, and ephemeral storage from the `steps` that are part of a TaskRun. Only the max
resource request values are needed since `steps` only execute one at a time in a `TaskRun` pod.
All requests that are not the max values are set to zero as a result.

When a [LimitRange](https://kubernetes.io/docs/concepts/policy/limit-range/) is present in a namespace
with a minimum set for container resource requests (i.e. CPU, memory, and ephemeral storage) where `TaskRuns`
are attempting to run, Tekton will search through all LimitRanges present in the namespace and use the minimum
set for container resource requests instead of requesting 0.

An example `TaskRun` with a LimitRange is available [here](../examples/v1beta1/taskruns/no-ci/limitrange.yaml).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
