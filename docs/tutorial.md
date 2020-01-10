# Hello World Tutorial

Welcome to the Tekton Pipeline tutorial!

This tutorial will walk you through creating and running some simple
[`Tasks`](tasks.md) & [`Pipelines`](pipelines.md) and running them by creating
[`TaskRuns`](taskruns.md) and [`PipelineRuns`](pipelineruns.md).

- [Creating a hello world `Task`](#task)
- [Creating a hello world `Pipeline`](#pipeline)

Before starting this tutorial, please install the [Tekton CLI](https://github.com/tektoncd/cli).

For more details on using `Pipelines`, see [our usage docs](README.md).

**Note:** [This tutorial can be run on a local workstation](#local-development)

## Task

The main objective of Tekton Pipelines is to run your Task individually or as a
part of a Pipeline. Every `Task` runs as a Pod on your Kubernetes cluster with
each step as its own container.

A [`Task`](tasks.md) defines the work that needs to be executed, for example the
following is a simple `Task` that will echo hello world:

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

The `steps` are a series of commands to be sequentially executed by the `Task`.

A [`TaskRun`](taskruns.md) runs the `Task` you defined. Here is a simple example
of a `TaskRun` you can use to execute your `Task`:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: echo-hello-world-task-run
spec:
  taskRef:
    name: echo-hello-world
```

To apply the yaml files, use the following command:

```bash
kubectl apply -f <name-of-file.yaml>
```

To see the output of the `TaskRun`, use the following command:

```bash
tkn taskrun describe echo-hello-world-task-run
```

You will get an output similar to the following:

```
Name:        echo-hello-world-task-run
Namespace:   default
Task Ref:    echo-hello-world

Status
STARTED         DURATION    STATUS
4 minutes ago   9 seconds   Succeeded

Input Resources
No resources

Output Resources
No resources

Params
No params

Steps
NAME
echo
```

The status of type `Succeeded` shows that the `Task` ran successfully.

To see the actual outcome, use the following command:
```bash
tkn taskrun logs echo-hello-world-task-run
```

You will get an output similar to this:

```
[echo] hello world
```

### Task Inputs and Outputs

In more common scenarios, a `Task` needs multiple steps with input and output
resources to process. For example a `Task` could fetch source code from a GitHub
repository and build a Docker image from it.

[`PipelineResources`](resources.md) are used to define the artifacts that can be
passed in and out of a `Task`. There are a few system defined resource types ready
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
built by the `Task`:

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
args of the `Task` command support variable substitution so that the definition of `Task` is
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
        type: string
        description: The path to the dockerfile to build
        default: /workspace/docker-source/Dockerfile
      - name: pathToContext
        type: string
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
      image: gcr.io/kaniko-project/executor:v0.15.0
      # specifying DOCKER_CONFIG is required to allow kaniko to detect docker credential
      env:
        - name: "DOCKER_CONFIG"
          value: "/tekton/home/.docker/"
      command:
        - /kaniko/executor
      args:
        - --dockerfile=$(inputs.params.pathToDockerFile)
        - --destination=$(outputs.resources.builtImage.url)
        - --context=$(inputs.params.pathToContext)
```

Before you continue with the `TaskRun` you will have to
create a `secret` to push your image to your desired registry.

To do so, use the following command:

```bash
kubectl create secret docker-registry regcred \
                    --docker-server=<your-registry-server> \
                    --docker-username=<your-name> \
                    --docker-password=<your-pword> \
                    --docker-email=<your-email>
```

To be able to use this `secret` the `TaskRun` needs to use a `ServiceAccount`.

The `ServiceAccount` should look similar to this:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tutorial-service
secrets:
  - name: regcred
```

You need to put your new `ServiceAccount` into action, to do so, use the following command:

```bash
kubectl apply -f <name-of-file.yaml>
```

Now you are ready for your first `TaskRun`.

A `TaskRun` binds the inputs and outputs to already defined `PipelineResources`,
sets values to the parameters used for variable substitution in addition to executing the
`Task` steps.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: TaskRun
metadata:
  name: build-docker-image-from-git-source-task-run
spec:
  serviceAccountName: tutorial-service
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
`PipelineResources`, the `Task` and `TaskRun`.

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
tkn taskrun describe build-docker-image-from-git-source-task-run
```

You will get an output similar to the following:

```
Name:        build-docker-image-from-git-source-task-run
Namespace:   default
Task Ref:    build-docker-image-from-git-source

Status
STARTED       DURATION     STATUS
2 hours ago   56 seconds   Succeeded

Input Resources
NAME            RESOURCE REF
docker-source   skaffold-git

Output Resources
NAME         RESOURCE REF
builtImage   skaffold-image-leeroy-web

Params
NAME               VALUE
pathToDockerFile   Dockerfile
pathToContext      /workspace/docker-source/examples/microservices/leeroy-web

Steps
NAME
build-and-push
create-dir-builtimage-wtjh9
git-source-skaffold-git-tck6k
image-digest-exporter-hlbsq
```

The status of type `Succeeded` shows the Task ran successfully and you
can also validate the Docker image is created in the location specified in the
resource definition.

If you run into issues, use the following command to receive the logs:

```bash
tkn taskrun logs build-docker-image-from-git-source-task-run
```

## Pipeline

A [`Pipeline`](pipelines.md) defines a list of `Tasks` to execute in order, while
also indicating if any outputs should be used as inputs of a following `Task` by
using [the `from` field](pipelines.md#from) and also indicating
[the order of executing (using the `runAfter` and `from` fields)](pipelines.md#ordering).
The same variable substitution you used in `Tasks` is also available in a `Pipeline`.

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
        type: string
        description: Path to the manifest to apply
      - name: yamlPathToImage
        type: string
        description:
          The path to the image to replace in the yaml manifest (arg to yq)
  steps:
    - name: replace-image
      image: mikefarah/yq
      command: ["yq"]
      args:
        - "w"
        - "-i"
        - "$(inputs.params.path)"
        - "$(inputs.params.yamlPathToImage)"
        - "$(inputs.resources.image.url)"
    - name: run-kubectl
      image: lachlanevenson/k8s-kubectl
      command: ["kubectl"]
      args:
        - "apply"
        - "-f"
        - "$(inputs.params.path)"
```

With the new `Task` inside of your `Pipeline`,
you need to give your `ServiceAccount` additional permissions to be able to execute the `run-kubectl` step.

First you have to create a new role, which you have to assign to your `ServiceAccount`,
to do so, use the following command:
```bash
kubectl create clusterrole tutorial-role \
               --verb=get,list,watch,create,update,patch,delete \
               --resource=deployments
```

Now you need to assign this new role `tutorial-role` to your `ServiceAccount`,
to do so, use the following command:

```bash
kubectl create clusterrolebinding tutorial-binding \
             --clusterrole=tutorial-role \
             --serviceaccount=default:tutorial-service
```


To run the `Pipeline`, create a [`PipelineRun`](pipelineruns.md) as follows:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineRun
metadata:
  name: tutorial-pipeline-run-1
spec:
  serviceAccountName: tutorial-service
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

While the `Pipeline` is running, you can see what exactly is happening, just use the following command:

```bash
tkn pipelinerun logs tutorial-pipeline-run-1 -f
```

To see the output of the `PipelineRun`, use the following command:

```bash
tkn pipelinerun describe tutorial-pipeline-run-1
```

You will get an output similar to the following:

```bash
Name:           tutorial-pipeline-run-1
Namespace:      default
Pipeline Ref:   tutorial-pipeline

Status
STARTED       DURATION   STATUS
4 hours ago   1 minute   Succeeded

Resources
NAME          RESOURCE REF
source-repo   skaffold-git
web-image     skaffold-image-leeroy-web

Params
No params

Taskruns
NAME                                               TASK NAME            STARTED       DURATION     STATUS
tutorial-pipeline-run-1-deploy-web-jjf2l           deploy-web           4 hours ago   14 seconds   Succeeded
tutorial-pipeline-run-1-build-skaffold-web-7jgjh   build-skaffold-web   4 hours ago   1 minute     Succeeded
```

The status of type `Succeeded` shows the `Pipeline` ran successfully, also
the status of individual Task runs are shown.

## Local development

### Known good configuration

Tekton Pipelines is known to work with:

- [Docker for Desktop](https://www.docker.com/products/docker-desktop). A known good
  configuration specifies six CPUs, 10 GB of memory and 2 GB of swap space. 
- These [prerequisites](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md#requirements).
- Setting `host.docker.local:5000` as an insecure registry with Docker for
  Desktop (set via preferences or configuration, see the
  [Docker insecure registry documentation](https://docs.docker.com/registry/insecure/).
  for details)
- Passing `--insecure` as an argument to Kaniko tasks lets us push to an
  insecure registry.
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
