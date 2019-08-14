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

  - [`serviceAccount`](#service-account) - Specifies a `ServiceAccount` resource
    object that enables your build to run with the defined authentication
    information.
  - [`inputs`] - Specifies [input parameters](#input-parameters) and
    [input resources](#providing-resources)
  - [`outputs`] - Specifies [output resources](#providing-resources)
  - [`timeout`] - Specifies timeout after which the `TaskRun` will fail. If the value of
    `timeout` is empty, the default timeout will be applied. If the value is set to 0,
    there is no timeout. You can also follow the instruction [here](#Configuring-default-timeout)
    to configure the default timeout.
  - [`podTemplate`](#pod-template) - Specifies a subset of
    [`PodSpec`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#pod-v1-core)
	configuration that will be used as the basis for the `Task` pod.

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
        image: gcr.io/kaniko-project/executor:v0.9.0
        # specifying DOCKER_CONFIG is required to allow kaniko to detect docker credential
        env:
          - name: "DOCKER_CONFIG"
            value: "/builder/home/.docker/"
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

### Configuring Default Timeout

You can configure the default timeout by changing the value of `default-timeout-minutes`
in [`config/config-defaults.yaml`](./../config/config-defaults.yaml). The default timeout
is 60 minutes, if `default-timeout-minutes` is not available. There is no timeout by
default, if `default-timeout-minutes` is set to 0.

### Service Account

Specifies the `name` of a `ServiceAccount` resource object. Use the
`serviceAccount` field to run your `Task` with the privileges of the specified
service account. If no `serviceAccount` field is specified, your `Task` runs
using the
[`default` service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#use-the-default-service-account-to-access-the-api-server)
that is in the
[namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)
of the `TaskRun` resource object.

For examples and more information about specifying service accounts, see the
[`ServiceAccount`](./auth.md) reference topic.

## Pod Template

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
(`my-cache`), that is provided by the TaskRun, using a
PersistenceVolumeClaim. The Pod will also run as a non-root user.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: myTask
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
  name: myTaskRun
  namespace: default
spec:
  taskRef:
    name: myTask
  podTemplate:
    securityContext:
      runAsNonRoot: true
    volumes:
    - name: my-cache
      persistentVolumeClaim:
        claimName: my-volume-claim
```

### Overriding where resources are copied from

When specifying input and output `PipelineResources`, you can optionally specify
`paths` for each resource. `paths` will be used by `TaskRun` as the resource's
new source paths i.e., copy the resource from specified list of paths. `TaskRun`
expects the folder and contents to be already present in specified paths.
`paths` feature could be used to provide extra files or altered version of
existing resource before execution of steps.

Output resource includes name and reference to pipeline resource and optionally
`paths`. `paths` will be used by `TaskRun` as the resource's new destination
paths i.e., copy the resource entirely to specified paths. `TaskRun` will be
responsible for creating required directories and copying contents over. `paths`
feature could be used to inspect the results of taskrun after execution of
steps.

`paths` feature for input and output resource is heavily used to pass same
version of resources across tasks in context of pipelinerun.

In the following example, task and taskrun are defined with input resource,
output resource and step which builds war artifact. After execution of
taskrun(`volume-taskrun`), `custom` volume will have entire resource
`java-git-resource` (including the war artifact) copied to the destination path
`/custom/workspace/`.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: volume-task
  namespace: default
spec:
  inputs:
    resources:
      - name: workspace
        type: git
  steps:
    - name: build-war
      image: objectuser/run-java-jar #https://hub.docker.com/r/objectuser/run-java-jar/
      command: jar
      args: ["-cvf", "projectname.war", "*"]
      volumeMounts:
        - name: custom-volume
          mountPath: /custom
```

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: volume-taskrun
  namespace: default
spec:
  taskRef:
    name: volume-task
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: java-git-resource
  outputs:
    resources:
      - name: workspace
        paths:
          - /custom/workspace/
        resourceRef:
          name: java-git-resource
  volumes:
    - name: custom-volume
      emptyDir: {}
```

## Status

As a TaskRun completes, it's `status` field is filled in with relevant information for
the overall run, as well as each step.

The following example shows a completed TaskRun and it's `status` field:

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

Fields include start and stop times for the `TaskRun` and each `Step`, and exit codes.
For each step we also include the fully-qualified image used, with the digest.

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
---
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
      command:
        - /bin/bash
      args:
        - "cat README.md"
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
        image: gcr.io/kaniko-project/executor:v0.9.0
        # specifying DOCKER_CONFIG is required to allow kaniko to detect docker credential
        env:
          - name: "DOCKER_CONFIG"
            value: "/builder/home/.docker/"
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
  serviceAccount: test-task-robot-git-ssh
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

Where `serviceAccount: test-build-robot-git-ssh` references the following
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
`serviceAccount` field to run your `Task` with the privileges of the specified
service account. If no `serviceAccount` field is specified, your `Task` runs
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

Tekton doesn't provide a mechanism to specify sidecars for Task steps
but it's still possible for sidecars to be added to your Pods:
[Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
provide cluster admins a mechanism to inject sidecar containers as Pods launch.
As a concrete example this is one possible method [used by Istio](https://istio.io/docs/setup/kubernetes/additional-setup/sidecar-injection/#automatic-sidecar-injection)
to inject an envoy proxy in to pods so that they can be included as part of
Istio's service mesh.

Tekton will happily work with sidecars injected into a TaskRun's
pods but the behaviour is a bit nuanced: When TaskRun's steps are complete
any sidecar containers running inside the Pod will be terminated. In
order to terminate the sidecars they will be restarted with a new
"nop" image that quickly exits. The result will be that your TaskRun's
Pod will include the sidecar container with a Retry Count of 1 and
with a different container image than you might be expecting.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
