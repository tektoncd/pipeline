# Hello World Tutorial

Welcome to the Tekton Pipeline tutorial!

This tutorial will walk you through creating and running some simple
[`Task`](tasks.md), [`Pipeline`](pipelines.md) and running them by creating
[`TaskRuns`](taskruns.md) and [`PipelineRuns`](pipelineruns.md).

- [Creating a hello world `Task`](#task)
- [Creating a hello world `Pipeline`](#pipeline)

For more details on using `Pipelines`, see [our usage docs](README.md).

**Note:** [This tutorial can be run on a local workstation](#local-development)

## Task

The main objective of Tekton Pipelines is to run your Task individually or as a
part of a Pipeline. Every task runs as a Pod on your Kubernetes cluster with
each step as its own container.

A [`Task`](tasks.md) defines the work that needs to be executed, for example the
following is a simple task that will echo hello world:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: echo-hello-world
spec:
  steps:
    - name: echo
      image: ubuntu
      command:
        - echo
      args:
        - "hello world"
```

The `steps` are a series of commands to be sequentially executed by the task.

A [`TaskRun`](taskruns.md) runs the `Task` you defined. Here is a simple example
of a `TaskRun` you can use to execute your task:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: echo-hello-world-task-run
spec:
  taskRef:
    name: echo-hello-world
```

To apply the yaml files use the following command:

```bash
kubectl apply -f <name-of-file.yaml>
```

To see the output of the `TaskRun`, use the following command:

```bash
kubectl get taskruns/echo-hello-world-task-run -o yaml
```

You will get an output similar to the following:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  creationTimestamp: 2018-12-11T15:49:13Z
  generation: 1
  name: echo-hello-world-task-run
  namespace: default
  resourceVersion: "6706789"
  selfLink: /apis/tekton.dev/v1alpha1/namespaces/default/taskruns/echo-hello-world-task-run
  uid: 4e96e9c6-fd5c-11e8-9129-42010a8a0fdc
spec:
  generation: 1
  inputs: {}
  outputs: {}
  taskRef:
    name: echo-hello-world
  taskSpec: null
status:
  conditions:
    - lastTransitionTime: 2018-12-11T15:50:09Z
      status: "True"
      type: Succeeded
  podName: echo-hello-world-task-run-pod-85ca51
  startTime: 2018-12-11T15:49:39Z
  steps:
    - terminated:
        containerID: docker://fcfe4a004...6729d6d2ad53faff41
        exitCode: 0
        finishedAt: 2018-12-11T15:50:01Z
        reason: Completed
        startedAt: 2018-12-11T15:50:01Z
    - terminated:
        containerID: docker://fe86fc5f7...eb429697b44ce4a5b
        exitCode: 0
        finishedAt: 2018-12-11T15:50:02Z
        reason: Completed
        startedAt: 2018-12-11T15:50:02Z
```

The status of type `Succeeded = True` shows the task ran successfully.

### Task Inputs and Outputs

In more common scenarios, a Task needs multiple steps with input and output
resources to process. For example a Task could fetch source code from a GitHub
repository and build a Docker image from it.

[`PipelineResources`](resources.md) are used to define the artifacts that can be
passed in and out of a task. There are a few system defined resource types ready
to use, and the following are two examples of the resources commonly needed.

The [`git` resource](resources.md#git-resource) represents a git repository with
a specific revision:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: skaffold-git
spec:
  type: git
  params:
    - name: revision
      value: master
    - name: url
      value: https://github.com/GoogleContainerTools/skaffold #configure: change if you want to build something else, perhaps from your own local git repository.
```

The [`image` resource](resources.md#image-resource) represents the image to be
built by the task:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: skaffold-image-leeroy-web
spec:
  type: image
  params:
    - name: url
      value: gcr.io/<use your project>/leeroy-web #configure: replace with where the image should go: perhaps your local registry or Dockerhub with a secret and configured service account
```

The following is a `Task` with inputs and outputs. The input resource is a
GitHub repository and the output is the image produced from that source. The
args of the task command support templating so that the definition of task is
constant and the value of parameters can change in runtime.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: build-docker-image-from-git-source
spec:
  inputs:
    resources:
      - name: docker-source
        type: git
    params:
      - name: pathToDockerFile
        description: The path to the dockerfile to build
        default: /workspace/docker-source/Dockerfile
      - name: pathToContext
        description:
          The build context used by Kaniko
          (https://github.com/GoogleContainerTools/kaniko#kaniko-build-contexts)
        default: /workspace/docker-source
  outputs:
    resources:
      - name: builtImage
        type: image
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
        - --dockerfile=${inputs.params.pathToDockerFile}
        - --destination=${outputs.resources.builtImage.url}
        - --context=${inputs.params.pathToContext}
```

`TaskRun` binds the inputs and outputs to already defined `PipelineResources`,
sets values to the parameters used for templating in addition to executing the
task steps.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: build-docker-image-from-git-source-task-run
spec:
  taskRef:
    name: build-docker-image-from-git-source
  inputs:
    resources:
      - name: docker-source
        resourceRef:
          name: skaffold-git
    params:
      - name: pathToDockerFile
        value: Dockerfile
      - name: pathToContext
        value: /workspace/docker-source/examples/microservices/leeroy-web #configure: may change according to your source
  outputs:
    resources:
      - name: builtImage
        resourceRef:
          name: skaffold-image-leeroy-web
```

To apply the yaml files use the following command, you need to apply the two
resources, the task and taskrun.

```bash
kubectl apply -f <name-of-file.yaml>
```

To see all the resource created so far as part of Tekton Pipelines, run the
command:

```bash
kubectl get tekton-pipelines
```

You will get an output similar to the following:

```
NAME                                                   AGE
taskruns/build-docker-image-from-git-source-task-run   30s

NAME                                          AGE
pipelineresources/skaffold-git                6m
pipelineresources/skaffold-image-leeroy-web   7m

NAME                                       AGE
tasks/build-docker-image-from-git-source   7m
```

To see the output of the TaskRun, use the following command:

```bash
kubectl get taskruns/build-docker-image-from-git-source-task-run -o yaml
```

You will get an output similar to the following:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  creationTimestamp: 2018-12-11T18:14:29Z
  generation: 1
  name: build-docker-image-from-git-source-task-run
  namespace: default
  resourceVersion: "6733537"
  selfLink: /apis/tekton.dev/v1alpha1/namespaces/default/taskruns/build-docker-image-from-git-source-task-run
  uid: 99d297fd-fd70-11e8-9129-42010a8a0fdc
spec:
  generation: 1
  inputs:
    params:
      - name: pathToDockerFile
        value: Dockerfile
      - name: pathToContext
        value: /workspace/git-source/examples/microservices/leeroy-web #configure: may change depending on your source
    resources:
      - name: git-source
        paths: null
        resourceRef:
          name: skaffold-git
  outputs:
    resources:
      - name: builtImage
        paths: null
        resourceRef:
          name: skaffold-image-leeroy-web
  taskRef:
    name: build-docker-image-from-git-source
  taskSpec: null
status:
  conditions:
    - lastTransitionTime: 2018-12-11T18:15:09Z
      status: "True"
      type: Succeeded
  podName: build-docker-image-from-git-source-task-run-pod-24d414
  startTime: 2018-12-11T18:14:29Z
  steps:
    - terminated:
        containerID: docker://138ce30c722eed....c830c9d9005a0542
        exitCode: 0
        finishedAt: 2018-12-11T18:14:47Z
        reason: Completed
        startedAt: 2018-12-11T18:14:47Z
    - terminated:
        containerID: docker://4a75136c029fb1....4c94b348d4f67744
        exitCode: 0
        finishedAt: 2018-12-11T18:14:48Z
        reason: Completed
        startedAt: 2018-12-11T18:14:48Z
```

The status of type `Succeeded = True` shows the Task ran successfully and you
can also validate the Docker image is created in the location specified in the
resource definition.

## Pipeline

A [`Pipeline`](pipelines.md) defines a list of tasks to execute in order, while
also indicating if any outputs should be used as inputs of a following task by
using [the `from` field](pipelines.md#from) and also indicating
[the order of executing (using the `runAfter` and `from` fields)](pipelines.md#ordering).
The same templating you used in tasks is also available in pipeline.

For example:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Pipeline
metadata:
  name: tutorial-pipeline
spec:
  resources:
    - name: source-repo
      type: git
    - name: web-image
      type: image
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-docker-image-from-git-source
      params:
        - name: pathToDockerFile
          value: Dockerfile
        - name: pathToContext
          value: /workspace/docker-source/examples/microservices/leeroy-web #configure: may change according to your source
      resources:
        inputs:
          - name: docker-source
            resource: source-repo
        outputs:
          - name: builtImage
            resource: web-image
    - name: deploy-web
      taskRef:
        name: deploy-using-kubectl
      resources:
        inputs:
          - name: source
            resource: source-repo
          - name: image
            resource: web-image
            from:
              - build-skaffold-web
      params:
        - name: path
          value: /workspace/source/examples/microservices/leeroy-web/kubernetes/deployment.yaml #configure: may change according to your source
        - name: yqArg
          value: "-d1"
        - name: yamlPathToImage
          value: "spec.template.spec.containers[0].image"
```

The above `Pipeline` is referencing a `Task` called `deploy-using-kubectl` which
can be found here:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: Task
metadata:
  name: deploy-using-kubectl
spec:
  inputs:
    resources:
      - name: source
        type: git
      - name: image
        type: image
    params:
      - name: path
        description: Path to the manifest to apply
      - name: yqArg
        description:
          Okay this is a hack, but I didn't feel right hard-coding `-d1` down
          below
      - name: yamlPathToImage
        description:
          The path to the image to replace in the yaml manifest (arg to yq)
  steps:
    - name: replace-image
      image: mikefarah/yq
      command: ["yq"]
      args:
        - "w"
        - "-i"
        - "${inputs.params.yqArg}"
        - "${inputs.params.path}"
        - "${inputs.params.yamlPathToImage}"
        - "${inputs.resources.image.url}"
    - name: run-kubectl
      image: lachlanevenson/k8s-kubectl
      command: ["kubectl"]
      args:
        - "apply"
        - "-f"
        - "${inputs.params.path}"
```

To run the `Pipeline`, create a [`PipelineRun`](pipelineruns.md) as follows:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineRun
metadata:
  name: tutorial-pipeline-run-1
spec:
  pipelineRef:
    name: tutorial-pipeline
  resources:
    - name: source-repo
      resourceRef:
        name: skaffold-git
    - name: web-image
      resourceRef:
        name: skaffold-image-leeroy-web
```

The `PipelineRun` will create the `TaskRuns` corresponding to each `Task` and
collect the results.

To apply the yaml files use the following command, you will need to apply the
`deploy-task` if you want to run the Pipeline.

```bash
kubectl apply -f <name-of-file.yaml>
```

To see the output of the `PipelineRun`, use the following command:

```bash
kubectl get pipelineruns/tutorial-pipeline-run-1 -o yaml
```

You will get an output similar to the following:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineRun
metadata:
  annotations:
  creationTimestamp: 2018-12-11T20:30:19Z
  generation: 1
  name: tutorial-pipeline-run-1
  namespace: default
  resourceVersion: "6760151"
  selfLink: /apis/tekton.dev/v1alpha1/namespaces/default/pipelineruns/tutorial-pipeline-run-1
  uid: 93acb0ea-fd83-11e8-9129-42010a8a0fdc
spec:
  generation: 1
  pipelineRef:
    name: tutorial-pipeline
  resources:
    - name: source-repo
      paths: null
      resourceRef:
        name: skaffold-git
    - name: web-image
      paths: null
      resourceRef:
        name: skaffold-image-leeroy-web
  serviceAccount: ""
status:
  conditions:
    - lastTransitionTime: 2018-12-11T20:32:41Z
      message: All Tasks have completed executing
      reason: Succeeded
      status: "True"
      type: Succeeded
  taskRuns:
    tutorial-pipeline-run-1-build-skaffold-web:
      conditions:
        - lastTransitionTime: 2018-12-11T20:31:41Z
          status: "True"
          type: Succeeded
      podName: tutorial-pipeline-run-1-build-skaffold-web-pod-21ddf0
      startTime: 2018-12-11T20:30:19Z
      steps:
        - terminated:
            containerID: docker://c699fcba94....f96108ac9f4db22b94e0c
            exitCode: 0
            finishedAt: 2018-12-11T20:30:36Z
            reason: Completed
            startedAt: 2018-12-11T20:30:36Z
        - terminated:
            containerID: docker://f5f752d....824262ad6ce7675
            exitCode: 0
            finishedAt: 2018-12-11T20:31:17Z
            reason: Completed
            startedAt: 2018-12-11T20:30:37Z
    tutorial-pipeline-run-1-deploy-web:
      conditions:
        - lastTransitionTime: 2018-12-11T20:32:41Z
          status: "True"
          type: Succeeded
      podName: tutorial-pipeline-run-1-deploy-web-pod-7a796b
      startTime: 2018-12-11T20:32:11Z
      steps:
        - terminated:
            containerID: docker://eaefb7b6d685....f001f895430f71374
            exitCode: 0
            finishedAt: 2018-12-11T20:32:28Z
            reason: Completed
            startedAt: 2018-12-11T20:32:28Z
        - terminated:
            containerID: docker://4cfc6eba47a7a....dcaef1e9b1eee3661b8a85f
            exitCode: 0
            finishedAt: 2018-12-11T20:32:31Z
            reason: Completed
            startedAt: 2018-12-11T20:32:31Z
        - terminated:
            containerID: docker://01b376b92....dce4ccec9641d77
            exitCode: 0
            finishedAt: 2018-12-11T20:32:35Z
            reason: Completed
            startedAt: 2018-12-11T20:32:34Z
```

The status of type `Succeeded = True` shows the pipeline ran successfully, also
the status of individual Task runs are shown.

## Local development

### Known good configuration

Tekton Pipelines is known to work with:

- [Docker for Desktop](https://www.docker.com/products/docker-desktop): a
  version that uses Kubernetes 1.11 or higher. At the time of this document,
  this requires the _edge_ version of Docker to be installed. A known good
  configuration specifies six CPUs, 10 GB of memory and 2 GB of swap space
- The following
  [prerequisites](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md#requirements)
- Setting `host.docker.local:5000` as an insecure registry with Docker for
  Desktop (set via preferences or configuration, see the
  [Docker insecure registry documentation](https://docs.docker.com/registry/insecure/)
  for details)
- Passing `--insecure` as an argument to Kaniko tasks lets us push to an
  insecure registry
- Running a local (insecure) Docker registry: this can be run with

`docker run -d -p 5000:5000 --name registry-srv -e REGISTRY_STORAGE_DELETE_ENABLED=true registry:2`

- Optionally, a Docker registry viewer so we can check our pushed images are
  present:

`docker run -it -p 8080:8080 --name registry-web --link registry-srv -e REGISTRY_URL=http://registry-srv:5000/v2 -e REGISTRY_NAME=localhost:5000 hyper/docker-registry-web`

### Images

- Any PipelineResource definitions of image type should be updated to use the
  local registry by setting the url to
  `host.docker.internal:5000/myregistry/<image name>` equivalents
- The `KO_DOCKER_REPO` variable should be set to `localhost:5000/myregistry`
  before using `ko`
- You are able to push to `host.docker.internal:5000/myregistry/<image name>`
  but your applications (e.g any deployment definitions) should reference
  `localhost:5000/myregistry/<image name>`

### Logging

- Logs can remain in-memory only as opposed to sent to a service such as
  [Stackdriver](https://cloud.google.com/logging/).
- See [docs on getting logs from Runs](logs.md)

Elasticsearch, Beats and Kibana can be deployed locally as a means to view logs:
an example is provided at
<https://github.com/mgreau/tekton-pipelines-elastic-tutorials>.

## Experimentation

Lines of code you may want to configure have the #configure annotation. This
annotation applies to subjects such as Docker registries, log output locations
and other nuances that may be specific to particular cloud providers or
services.

The `TaskRuns` have been created in the following
[order](pipelines.md#ordering):

1. `tutorial-pipeline-run-1-build-skaffold-web` - This runs the
   [Pipeline Task](pipelines.md#pipeline-tasks) `build-skaffold-web` first,
   because it has no [`from` or `runAfter` clauses](pipelines.md#ordering)
1. `tutorial-pipeline-run-1-deploy-web` - This runs `deploy-web` second, because
   its [input](tasks.md#inputs) `web-image` comes [`from`](pipelines.md#from)
   `build-skaffold-web` (therefore `build-skaffold-web` must run before
   `deploy-web`).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
