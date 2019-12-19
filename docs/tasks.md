# Tasks

A `Task` (or a [`ClusterTask`](#clustertask)) is a collection of sequential
steps you would want to run as part of your continuous integration flow. A task
will run inside a pod on your cluster.

A `Task` declares:

- [Inputs](#inputs)
- [Outputs](#outputs)
- [Steps](#steps)

A `Task` is available within a namespace, and `ClusterTask` is available across
entire Kubernetes cluster.

---

- [ClusterTasks](#clustertask)
- [Syntax](#syntax)
  - [Steps](#steps)
    - [Step script](#step-script)
  - [Inputs](#inputs)
  - [Outputs](#outputs)
  - [Controlling where resources are mounted](#controlling-where-resources-are-mounted)
  - [Volumes](#volumes)
  - [Workspaces](#workspaces)
  - [Step Template](#step-template)
  - [Variable Substitution](#variable-substitution)
- [Examples](#examples)
- [Debugging Tips](#debugging)

## ClusterTask

Similar to Task, but with a cluster scope.

In case of using a ClusterTask, the `TaskRef` kind should be added. The default
kind is Task which represents a namespaced Task

```yaml
apiVersion: tekton.dev/v1alpha1
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

A `Task` functions exactly like a `ClusterTask`, and as such all references to
`Task` below are also describing `ClusterTask`.

## Syntax

To define a configuration file for a `Task` resource, you can specify the
following fields:

- Required:
  - [`apiVersion`][kubernetes-overview] - Specifies the API version, for example
    `tekton.dev/v1alpha1`.
  - [`kind`][kubernetes-overview] - Specify the `Task` resource object.
  - [`metadata`][kubernetes-overview] - Specifies data to uniquely identify the
    `Task` resource object, for example a `name`.
  - [`spec`][kubernetes-overview] - Specifies the configuration information for
    your `Task` resource object. `Task` steps must be defined through either of
    the following fields:
    - [`steps`](#steps) - Specifies one or more container images that you want
      to run in your `Task`.
- Optional:
  - [`inputs`](#inputs) - Specifies parameters and
    [`PipelineResources`](resources.md) needed by your `Task`
  - [`outputs`](#outputs) - Specifies [`PipelineResources`](resources.md)
    created by your `Task`
  - [`volumes`](#volumes) - Specifies one or more volumes that you want to make
    available to your `Task`'s steps.
  - [`workspaces`](#workspaces) - Specifies paths at which you expect volumes to
    be mounted and available
  - [`stepTemplate`](#step-template) - Specifies a `Container` step
    definition to use as the basis for all steps within your `Task`.
  - [`sidecars`](#sidecars) - Specifies sidecar containers to run alongside
    steps.

[kubernetes-overview]:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields

The following example is a non-working sample where most of the possible
configuration fields are used:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: example-task-name
spec:
  inputs:
    resources:
      - name: workspace
        type: git
    params:
      - name: pathToDockerFile
        type: string
        description: The path to the dockerfile to build
        default: /workspace/workspace/Dockerfile
  outputs:
    resources:
      - name: builtImage
        type: image
  steps:
    - name: ubuntu-example
      image: ubuntu
      args: ["ubuntu-build-example", "SECRETS-example.md"]
    - image: gcr.io/example-builders/build-example
      command: ["echo"]
      args: ["$(inputs.params.pathToDockerFile)"]
    - name: dockerfile-pushexample
      image: gcr.io/example-builders/push-example
      args: ["push", "$(outputs.resources.builtImage.url)"]
      volumeMounts:
        - name: docker-socket-example
          mountPath: /var/run/docker.sock
  volumes:
    - name: example-volume
      emptyDir: {}
```

### Steps

The `steps` field is required. You define one or more `steps` fields to define
the body of a `Task`.

If multiple `steps` are defined, they will be executed in the same order as they
are defined, if the `Task` is invoked by a [`TaskRun`](taskruns.md).
Each `steps` in a `Task` must specify a container image that adheres to the
[container contract](./container-contract.md). For each of the `steps` fields,
or container images that you define:

- The container images are run and evaluated in order, starting from the top of
  the configuration file.
- Each container image runs until completion or until the first failure is
  detected.
- The CPU, memory, and ephemeral storage resource requests will be set to zero
  if the container image does not have the largest resource request out of all
  container images in the Task. This ensures that the Pod that executes the Task
  will only request the resources necessary to execute any single container
  image in the Task, rather than requesting the sum of all of the container
  image's resource requests.

#### Step Script

To simplify executing scripts inside a container, a step can specify a `script`.
If this field is present, the step cannot specify `command`.

When specified, a `script` gets invoked as if it were the contents of a file in
the container. Any `args` are passed to the script file.

Scripts that do not start with a [shebang](https://en.wikipedia.org/wiki/Shebang_(Unix))
line will use the following default preamble:

```bash
#!/bin/sh
set -xe
```
Users can override this by starting their script with a shebang to declare what
tool should be used to interpret the script. That tool must then also be
available within the step's container.

This allows you to execute a Bash script, if the image includes `bash`:

```yaml
steps:
- image: ubuntu  # contains bash
  script: |
    #!/usr/bin/env bash
    echo "Hello from Bash!"
```

...or to execute a Python script, if the image includes `python`:

```yaml
steps:
- image: python  # contains python
  script: |
    #!/usr/bin/env python3
    print("Hello from Python!")
```

...or to execute a Node script, if the image includes `node`:

```yaml
steps:
- image: node  # contains node
  script: |
    #!/usr/bin/env node
    console.log("Hello from Node!")
```

This also simplifies executing script files in the workspace:

```yaml
steps:
- image: ubuntu
  script: |
    #!/usr/bin/env bash
    /workspace/my-script.sh  # provided by an input resource
```

...or in the container image:

```yaml
steps:
- image: my-image  # contains /bin/my-binary
  script: |
    #!/usr/bin/env bash
    /bin/my-binary
```

### Inputs

A `Task` can declare the inputs it needs, which can be either or both of:

- [`parameters`](#parameters)
- [input resources](#input-resources)

#### Parameters

Tasks can declare input parameters that must be supplied to the task during a
TaskRun. Some example use-cases of this include:

- A Task that needs to know what compilation flags to use when building an
  application.
- A Task that needs to know what to name a built artifact.

Parameters name are limited to alpha-numeric characters, `-` and `_` and can
only start with alpha characters and `_`. For example, `fooIs-Bar_` is a valid
parameter name, `barIsBa$` or `0banana` are not.

Each declared parameter has a `type` field, assumed to be `string` if not provided by the user. The other possible type is `array` — useful, for instance, when a dynamic number of compilation flags need to be supplied to a task building an application. When the actual parameter value is supplied, its parsed type is validated against the `type` field.

##### Usage

The following example shows how Tasks can be parameterized, and these parameters
can be passed to the `Task` from a `TaskRun`.

Input parameters in the form of `$(inputs.params.foo)` are replaced inside of
the [`steps`](#steps) (see also [variable substitution](#variable-substitution)).

The following `Task` declares an input parameter called 'flags', and uses it in
the `steps.args` list.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: task-with-parameters
spec:
  inputs:
    params:
      - name: flags
        type: array
      - name: someURL
        type: string
  steps:
    - name: build
      image: my-builder
      args: ["build", "$(inputs.params.flags)", "url=$(inputs.params.someURL)"]
```

The following `TaskRun` supplies a dynamic number of strings within the `flags` parameter:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: run-with-parameters
spec:
  taskRef:
    name: task-with-parameters
  inputs:
    params:
      - name: flags
        value:
          - "--set"
          - "arg1=foo"
          - "--randomflag"
          - "--someotherflag"
      - name: someURL
        value: "http://google.com"
```

#### Input resources

Use input [`PipelineResources`](resources.md) field to provide your `Task` with
data or context that is needed by your `Task`. See the [using resources docs](./resources.md#using-resources).


### Outputs

`Task` definitions can include inputs and outputs
[`PipelineResource`](resources.md) declarations. If specific set of resources
are only declared in output then a copy of resource to be uploaded or shared for
next Task is expected to be present under the path
`/workspace/output/resource_name/`.

```yaml
outputs:
  resources:
    name: storage-gcs
    type: gcs
steps:
  - image: objectuser/run-java-jar #https://hub.docker.com/r/objectuser/run-java-jar/
    command: [jar]
    args:
      ["-cvf", "-o", "/workspace/output/storage-gcs/", "projectname.war", "*"]
    env:
      - name: "FOO"
        value: "world"
```

**note**: if the task is relying on output resource functionality then the
containers in the task `steps` field cannot mount anything in the path
`/workspace/output`.

In the following example Task `tar-artifact` resource is used both as input and
output so input resource is downloaded into directory `customworkspace`(as
specified in [`targetPath`](#targetpath)). Step `untar` extracts tar file into
`tar-scratch-space` directory , `edit-tar` adds a new file and last step
`tar-it-up` creates new tar file and places in `/workspace/customworkspace/`
directory. After execution of the Task steps, (new) tar file in directory
`/workspace/customworkspace` will be uploaded to the bucket defined in
`tar-artifact` resource definition.

```yaml
inputs:
  resources:
    name: tar-artifact
    targetPath: customworkspace
outputs:
  resources:
    name: tar-artifact
steps:
 - name: untar
    image: ubuntu
    command: ["/bin/bash"]
    args: ['-c', 'mkdir -p /workspace/tar-scratch-space/ && tar -xvf /workspace/customworkspace/rules_docker-master.tar -C /workspace/tar-scratch-space/']
 - name: edit-tar
    image: ubuntu
    command: ["/bin/bash"]
    args: ['-c', 'echo crazy > /workspace/tar-scratch-space/rules_docker-master/crazy.txt']
 - name: tar-it-up
   image: ubuntu
   command: ["/bin/bash"]
   args: ['-c', 'cd /workspace/tar-scratch-space/ && tar -cvf /workspace/customworkspace/rules_docker-master.tar rules_docker-master']
```


### Volumes

Specifies one or more
[volumes](https://kubernetes.io/docs/concepts/storage/volumes/) that you want to
make available to your `Task`, including all the [`steps`](#steps). Add volumes
to complement the volumes that are implicitly created for
[input resources](#input-resources) and [output resources](#outputs).

For example, use volumes to accomplish one of the following common tasks:

- [Mount a Kubernetes secret](./auth.md).
- Create an `emptyDir` volume to act as a cache for use across multiple build
  steps. Consider using a persistent volume for inter-build caching.
- Mount
  [Kubernetes configmap](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/)
  as volume source.
- Mount a host's Docker socket to use a `Dockerfile` for container image builds.
  **Note:** Building a container image using `docker build` on-cluster is _very
  unsafe_. Use [kaniko](https://github.com/GoogleContainerTools/kaniko) instead.
  This is used only for the purposes of demonstration.

### Workspaces

`workspaces` are a way of declaring volumes you expect to be made available to your
executing `Task` and the path to make them available at. They are similar to
[`volumes`](#volumes) but allow you to enforce at runtime that the volumes have
been attached and [allow you to specify subpaths](taskruns.md#workspaces) in the volumes
to attach.

The volume will be made available at `/workspace/myworkspace`, or you can override
this with `mountPath`. The value at `mountPath` can be anywhere on your pod's filesystem.
The path will be available via [variable substitution](#variable-substitution) with
`$(workspaces.myworkspace.path)`.

A task can declare that it will not write to the volume by adding `readOnly: true`
to the workspace declaration. This will in turn mark the volumeMount as `readOnly`
on the Task's underlying pod.

The actual volumes must be provided at runtime
[in the `TaskRun`](taskruns.md#workspaces).
In a future iteration ([#1438](https://github.com/tektoncd/pipeline/issues/1438))
it [will be possible to specify these in the `PipelineRun`](pipelineruns.md#workspaces)
as well.

For example:

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

_For a complete example see [workspace.yaml](../examples/taskruns/workspace.yaml)._

### Step Template

Specifies a [`Container`](https://kubernetes.io/docs/concepts/containers/)
configuration that will be used as the basis for all [`steps`](#steps) in your
`Task`. Configuration in an individual step will override or merge with the
step template's configuration.

In the example below, the `Task` specifies a `stepTemplate` with the
environment variable `FOO` set to `bar`. The first step will use that value for
`FOO`, but in the second step, `FOO` is overridden and set to `baz`.

```yaml
stepTemplate:
  env:
    - name: "FOO"
      value: "bar"
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
```

### Sidecars

Specifies a list of
[`Containers`](https://kubernetes.io/docs/concepts/containers/) to run
alongside your Steps. These containers can provide auxiliary functions like
[Docker in Docker](https://hub.docker.com/_/docker) or running a mock API
server for your app to hit during tests.

Sidecars are started before your Task's steps are executed and are torn
down after all steps have completed. For further information about a sidecar's
lifecycle see the [TaskRun doc](./taskruns.md#sidecars).

In the example below, a Docker in Docker sidecar is run so that a step can
use it to build a docker image:

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

Note: There is a known bug with Tekton's existing sidecar implementation.
Tekton uses a specific image, called "nop", to stop sidecars. The "nop" image
is configurable using a flag of the Tekton controller. If the configured "nop"
image contains the command that the sidecar was running before the sidecar
was stopped then the sidecar will actually keep running, causing the TaskRun's
Pod to remain running, and eventually causing the TaskRun to timeout rather
then exit successfully. Issue https://github.com/tektoncd/pipeline/issues/1347
has been created to track this bug.

### Variable Substitution

`Tasks` support string replacement using values from:

* [Inputs and Outputs](#input-and-output-substitution)
  * [Array params](#variable-substitution-with-parameters-of-type-array)
* [`workspaces`](#variable-substitution-with-workspaces)
* [`volumes`](#variable-substitution-with-volumes)

#### Input and Output substitution

[`inputs`](#inputs) and [`outputs`](#outputs) attributes can be used in replacements,
including [`params`](#params) and [`resources`](./resources.md#variable-substitution).

Input parameters can be referenced in the `Task` spec using the variable substitution syntax below,
where `<name>` is the name of the parameter:

```shell
$(inputs.params.<name>)
```

Param values from resources can also be accessed using [variable substitution](./resources.md#variable-substitution)

##### Variable Substitution with Parameters of Type `Array`

Referenced parameters of type `array` will expand to insert the array elements in the reference string's spot.

So, with the following parameter:
```
inputs:
    params:
      - name: array-param
        value:
          - "some"
          - "array"
          - "elements"
```
then `command: ["first", "$(inputs.params.array-param)", "last"]` will become
`command: ["first", "some", "array", "elements", "last"]`


Note that array parameters __*must*__ be referenced in a completely isolated string within a larger string array.
Any other attempt to reference an array is invalid and will throw an error.

For instance, if `build-args` is a declared parameter of type `array`, then this is an invalid step because
the string isn't isolated:
```
 - name: build-step
      image: gcr.io/cloud-builders/some-image
      args: ["build", "additionalArg $(inputs.params.build-args)"]
```

Similarly, referencing `build-args` in a non-array field is also invalid:
```
 - name: build-step
      image: "$(inputs.params.build-args)"
      args: ["build", "args"]
```

A valid reference to the `build-args` parameter is isolated and in an eligible field (`args`, in this case):
```
 - name: build-step
      image: gcr.io/cloud-builders/some-image
      args: ["build", "$(inputs.params.build-args)", "additonalArg"]
```

#### Variable Substitution with Workspaces

Paths to a `Task's` declared [workspaces](#workspaces) can be substituted with:

```
$(workspaces.myworkspace.path)
```

Since the name of the `Volume` is not known until runtime and is randomized, you can also
substitute the volume name with:

```
$(workspaces.myworkspace.volume)
```

#### Variable Substitution within Volumes

Task volume names and different
[types of volumes](https://kubernetes.io/docs/concepts/storage/volumes/#types-of-volumes)
can be parameterized. Current support includes for widely used types of volumes
like configmap, secret and PersistentVolumeClaim. Here is an
[example](#using-kubernetes-configmap-as-volume-source) on how to use this in
Task definitions.

## Examples

Use these code snippets to help you understand how to define your `Tasks`.

- [Example of image building and pushing](#example-task)
- [Mounting extra volumes](#using-an-extra-volume)
- [Mounting configMap as volume
  source](#using-kubernetes-configmap-as-volume-source)
- [Using secret as environment source](#using-secret-as-environment-source)

_Tip: See the collection of simple
[examples](https://github.com/tektoncd/pipeline/tree/master/examples) for
additional code samples._

### Example Task

For example, a `Task` to encapsulate a `Dockerfile` build might look something
like this:

**Note:** Building a container image using `docker build` on-cluster is _very
unsafe_. Use [kaniko](https://github.com/GoogleContainerTools/kaniko) instead.
This is used only for the purposes of demonstration.

```yaml
spec:
  inputs:
    resources:
      - name: workspace
        type: git
    params:
      # These may be overridden, but provide sensible defaults.
      - name: directory
        type: string
        description: The directory containing the build context.
        default: /workspace
      - name: dockerfileName
        type: string
        description: The name of the Dockerfile
        default: Dockerfile
  outputs:
    resources:
      - name: builtImage
        type: image
  steps:
    - name: dockerfile-build
      image: gcr.io/cloud-builders/docker
      workingDir: "$(inputs.params.directory)"
      args:
        [
          "build",
          "--no-cache",
          "--tag",
          "$(outputs.resources.image)",
          "--file",
          "$(inputs.params.dockerfileName)",
          ".",
        ]
      volumeMounts:
        - name: docker-socket
          mountPath: /var/run/docker.sock

    - name: dockerfile-push
      image: gcr.io/cloud-builders/docker
      args: ["push", "$(outputs.resources.image)"]
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

#### Using an extra volume

Mounting multiple volumes:

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

#### Using Kubernetes Configmap as Volume Source

```yaml
spec:
  inputs:
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
        - name: "$(inputs.params.volumeName)"
          mountPath: /var/configmap

  volumes:
    - name: "$(inputs.params.volumeName)"
      configMap:
        name: "$(inputs.params.CFGNAME)"
```

#### Using secret as environment source

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: goreleaser
spec:
  inputs:
    params:
    - name: package
      type: string
      description: base package to build in
    - name: github-token-secret
      type: string
      description: name of the secret holding the github-token
      default: github-token
    resources:
    - name: source
      type: git
      targetPath: src/$(inputs.params.package)
  steps:
  - name: release
    image: goreleaser/goreleaser
    workingDir: /workspace/src/$(inputs.params.package)
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
          name: $(inputs.params.github-token-secret)
          key: bot-token
```

#### Using a sidecar

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: with-sidecar-task
spec:
  inputs:
    params:
    - name: sidecar-image
      type: string
      description: Image name of the sidecar container
    - name: sidecar-env
      type: string
      description: Environment variable value
  sidecars:
  - name: sidecar
    image: $(inputs.params.sidecar-image)
    env:
    - name: SIDECAR_ENV
      value: $(inputs.params.sidecar-env)  
  steps:
  - name: test
    image: hello-world
```

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).

## Debugging

In software, we do things not because they are easy, but because we think they will be.
Lots of things can go wrong when writing a Task.
This section contains some tips on how to debug one.

### Inspecting the Filesystem

One common problem when writing Tasks is not understanding where files are on disk.
For the most part, these all live somewhere under `/workspace`, but the exact layout can
be tricky.
To see where things are before your task runs, you can add a step like this:

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

This step will output the name of every file under /workspace to your build logs.

To see the contents of every file, you can use a similar step:

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

These steps are useful both before and after your Task steps!

### Inspecting the pod

One `task` will map to one `Pod`, to check arbitrary thing in `Pod`, the best way is to login the `pod`, add a step at the position you want to `pause` the task, then checking.

```yaml
- name: pause
  image: docker
  args: ["sleep", "6000"]

```
