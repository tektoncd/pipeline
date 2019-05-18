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
- [Cancelling a TaskRun](#cancelling-a-taskrun)
- [Examples](#examples)

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
  - `timeout` - Specifies timeout after which the `TaskRun` will fail. Defaults
    to ten minutes.
  - [`nodeSelector`] - a selector which must be true for the pod to fit on a
    node. The selector which must match a node's labels for the pod to be
    scheduled on that node. More info:
    <https://kubernetes.io/docs/concepts/configuration/assign-pod-node/>
  - [`tolerations`] - Tolerations are applied to pods, and allow (but do not
    require) the pods to schedule onto nodes with matching taints. More info:
    <https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/>
  - [`affinity`] - the pod's scheduling constraints. More info:
    <https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#node-affinity-beta-feature>

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

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
