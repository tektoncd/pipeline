# TaskRuns

Use the `TaskRun` resource object to create and run on-cluster processes to
completion.

To create a `TaskRun`, you must first create a [`Task`](tasks.md) which
specifies one or more container images that you have implemented to perform and
complete a task.

A `TaskRun` runs until all `steps` have completed or until a failure occurs.

---

- [Syntax](#syntax)
  - [Specifying a `Task`](#specifying-a-task)
  - [Input parameters](#input-parameters)
  - [Providing resources](#providing-resources)
  - [Overriding where resources are copied from](#overriding-where-resources-are-copied-from)
  - [Service Account](#service-account)
  - [Pod Template](#pod-template)
  - [Workspaces](#workspaces)
- [Status](#status)
  - [Steps](#steps)
- [Cancelling a TaskRun](#cancelling-a-taskrun)
- [Examples](#examples)
- [Sidecars](#sidecars)
- [Logs](logs.md)

---

## Syntax

To define a configuration file for a `TaskRun` resource, you can specify the
following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1alpha1`.
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
    specified in the configmap - config-defaults will be applied.
  - [`inputs`] - Specifies [input parameters](#input-parameters) and
    [input resources](#providing-resources)
  - [`outputs`] - Specifies [output resources](#providing-resources)
  - [`timeout`] - Specifies timeout after which the `TaskRun` will fail. If the value of
    `timeout` is empty, the default timeout will be applied. If the value is set to 0,
    there is no timeout. You can also follow the instruction [here](#Configuring-default-timeout)
    to configure the default timeout.
  - [`podTemplate`](#pod-template) - Specifies a [pod template](./podtemplates.md) that will be used as the basis for the `Task` pod.
  - [`workspaces`](#workspaces) - Specify the actual volumes to use for the
    [workspaces](tasks.md#workspaces) declared by a `Task`

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
    inputs:
      resources:
        - name: workspace
          type: git
    steps:
      - name: build-and-push
        image: gcr.io/kaniko-project/executor:v0.15.0
        # specifying DOCKER_CONFIG is required to allow kaniko to detect docker credential
        env:
          - name: "DOCKER_CONFIG"
            value: "/tekton/home/.docker/"
        command:
          - /kaniko/executor
        args:
          - --destination=gcr.io/my-project/gohelloworld
```

### Input parameters

If a `Task` has [`parameters`](tasks.md#parameters), you can specify values for
them using the `input` section:

```yaml
spec:
  inputs:
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
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: java-git-resource
```

Or by embedding the specs of the resources directly:

```yaml
spec:
  inputs:
    resources:
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
PersistenceVolumeClaim. The Pod will also run as a non-root user.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: mytask
  namespace: default
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
kind: TaskRun
metadata:
  name: mytaskRun
  namespace: default
spec:
  taskRef:
    name: mytask
  podTemplate:
    securityContext:
      runAsNonRoot: true
    volumes:
    - name: my-cache
      persistentVolumeClaim:
        claimName: my-volume-claim
```

## Workspaces

For a `TaskRun` to execute [a `Task` that declares `workspaces`](tasks.md#workspaces),
at runtime you need to map the `workspaces` to actual physical volumes with
`workspaces`. Values in `workspaces` are
[`Volumes`](https://kubernetes.io/docs/tasks/configure-pod-container/configure-volume-storage/), however currently we only support a subset of `VolumeSources`:

* [`emptyDir`](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir)
* [`persistentVolumeClaim`](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim)
* [`configMap`](https://kubernetes.io/docs/concepts/storage/volumes/#configmap)
* [`secret`](https://kubernetes.io/docs/concepts/storage/volumes/#secret)

_If you need support for a `VolumeSource` not listed here
[please open an issue](https://github.com/tektoncd/pipeline/issues) or feel free to
[contribute a PR](https://github.com/tektoncd/pipeline/blob/master/CONTRIBUTING.md)._

If the declared `workspaces` are not provided at runtime, the `TaskRun` will fail
with an error.

For example to provide an existing PVC called `mypvc` for a `workspace` called
`myworkspace` declared by the `Task`, using the `my-subdir` folder which already exists
on the PVC (there will be an error if it does not exist):

```yaml
workspaces:
- name: myworkspace
  persistentVolumeClaim:
    claimName: mypvc
  subPath: my-subdir
```

Or to use [`emptyDir`](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) for the same `workspace`:

```yaml
workspaces:
- name: myworkspace
  emptyDir: {}
```

A ConfigMap can also be used as a workspace with the following caveats:

1. ConfigMap volume sources are always mounted as read-only inside a task's
containers - tasks cannot write content to them and a step may error out
and fail the task if a write is attempted.
2. The ConfigMap you want to use as a workspace must already exist prior
to the TaskRun being submitted.
3. ConfigMaps have a [size limit of 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L5042)

To use a [`configMap`](https://kubernetes.io/docs/concepts/storage/volumes/#configmap)
as a `workspace`:

```yaml
workspaces:
- name: myworkspace
  configmap:
    name: my-configmap
```

A Secret can also be used as a workspace with the following caveats:

1. Secret volume sources are always mounted as read-only inside a task's
containers - tasks cannot write content to them and a step may error out
and fail the task if a write is attempted.
2. The Secret you want to use as a workspace must already exist prior
to the TaskRun being submitted.
3. Secrets have a [size limit of 1MB](https://github.com/kubernetes/kubernetes/blob/f16bfb069a22241a5501f6fe530f5d4e2a82cf0e/pkg/apis/core/validation/validation.go#L4933)

To use a [`secret`](https://kubernetes.io/docs/concepts/storage/volumes/#secret)
as a `workspace`:

```yaml
workspaces:
- name: myworkspace
  secret:
    secretName: my-secret
```

_For a complete example see [workspace.yaml](../examples/taskruns/workspace.yaml)._

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
by Kubernetes, the `Taskrun` will be marked as failed even if the exitcode is 0.

### Steps

If multiple `steps` are defined in the `Task` invoked by the `TaskRun`, we will see the
`status.steps` of the `TaskRun` displayed in the same order as they are defined in
`spec.steps` of the `Task`, when the `TaskRun` is accessed by the `get` command, e.g.
`kubectl get taskrun <name> -o yaml`. Replace \<name\> with the name of the `TaskRun`.

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
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: read-task
spec:
  inputs:
    resources:
      - name: workspace
        type: git
  steps:
    - name: readme
      image: ubuntu
      script: cat workspace/README.md
---
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: read-repo-run
spec:
  taskRef:
    name: read-task
  inputs:
    resources:
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
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: build-push-task-run-2
spec:
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: go-example-git
  taskSpec:
    inputs:
      resources:
        - name: workspace
          type: git
    steps:
      - name: build-and-push
        image: gcr.io/kaniko-project/executor:v0.15.0
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
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: read-repo
spec:
  taskRef:
    name: read-task
  inputs:
    resources:
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
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: mchmarny-repo
    params:
      - name: IMAGE
        value: gcr.io/my-project/rester-tester
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
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: cloud-builder-repo
    params:
      - name: IMAGE
        value: gcr.io/my-project/wget
      # Optional override to specify the subdirectory containing the Dockerfile
      - name: DIRECTORY
        value: /workspace/wget
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
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: cloud-builder-repo
    params:
      - name: IMAGE
        value: gcr.io/my-project/docker
      # Optional overrides
      - name: DIRECTORY
        value: /workspace/docker
      - name: DOCKERFILE_NAME
        value: Dockerfile-17.06.1
```

#### Using a `ServiceAccount`

Specifying a `ServiceAccount` to access a private `git` repository:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: test-task-with-serviceaccount-git-ssh
spec:
  serviceAccountName: test-task-robot-git-ssh
  inputs:
    resources:
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
pods but the behaviour is a bit nuanced: When TaskRun's steps are complete
any sidecar containers running inside the Pod will be terminated. In
order to terminate the sidecars they will be restarted with a new
"nop" image that quickly exits. The result will be that your TaskRun's
Pod will include the sidecar container with a Retry Count of 1 and
with a different container image than you might be expecting.

Note: There are some known issues with the existing implementation of sidecars:

- The configured "nop" image must not provide the command that the
sidecar is expected to run. If it does provide the command then it will
not exit. This will result in the sidecar running forever and the Task
eventually timing out. https://github.com/tektoncd/pipeline/issues/1347
is the issue where this bug is being tracked.

- `kubectl get pods` will show a TaskRun's Pod as "Completed" if a sidecar
exits successfully and "Error" if the sidecar exits with an error, regardless
of how the step containers inside that pod exited. This issue only manifests
with the `get pods` command. The Pod description will instead show a Status of
Failed and the individual container statuses will correctly reflect how and why
they exited.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
