# Tekton Pipelines Tutorial

This tutorial uses a simple `Hello World` example to show you how to:
- Create a `Task`
- Create a `Pipeline` containing your `Tasks`
- Use a `TaskRun` to instantiate and execute a `Task` outside of a `Pipeline`
- Use a `PipelineRun` to instantiate and run a `Pipeline` containing your `Tasks`

This tutorial consists of the following sections:

- [Creating and running a `Task`](#creating-and-running-a-task)
- [Creating and running a `Pipeline`](#creating-and-running-a-pipeline)

**Note:** Items requiring configuration are marked with the `#configure` annotation.
This includes Docker registries, log output locations, and other configuration items
specific to a given cloud computing service.

## Before you begin

Before you begin this tutorial, make sure you have [installed and configured](https://github.com/tektoncd/pipeline/blob/master/docs/install.md)
the latest release of Tekton on your Kubernetes cluster, including the
[Tekton CLI](https://github.com/tektoncd/cli).

If you would like to complete this tutorial on your local workstation, see [Running this tutorial locally](#running-this-tutorial-locally). To learn more about the Tekton entities involved in this tutorial, see [Further reading](#further-reading).

## Creating and running a `Task`

A [`Task`](tasks.md) defines a series of `steps` that run in a desired order and complete a set amount of build work. Every `Task` runs as a Pod on your Kubernetes cluster with each `step` as its own container. For example, the following `Task` outputs "Hello World":

```yaml
apiVersion: tekton.dev/v1beta1
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
        - "Hello World"
```

Apply your `Task` YAML file as follows:

```bash
kubectl apply -f <name-of-task-file.yaml>
```

To see details about your created `Task`, use the following command:
```bash
tkn task describe echo-hello-world
```

The output will look similar to the following:

```
Name:        echo-hello-world
Namespace:   default

📨 Input Resources

 No input resources

📡 Output Resources

 No output resources

⚓ Params

 No params

🦶 Steps

 ∙ echo

🗂  Taskruns

 No taskruns
```

To run this `Task`, instantiate it using a [`TaskRun`](taskruns.md):

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: echo-hello-world-task-run
spec:
  taskRef:
    name: echo-hello-world
```

Apply your `TaskRun` YAML file as follows:

```bash
kubectl apply -f <name-of-taskrun-file.yaml>
```

To check whether running your `TaskRun` succeeded, use the following command:

```bash
tkn taskrun describe echo-hello-world-task-run
```

The output will look similar to the following:

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

The `Succeeded` status confirms that the `TaskRun` completed with no errors.

To see more detail about the execution of your `TaskRun`, view its logs as follows:

```bash
tkn taskrun logs echo-hello-world-task-run
```

The output will look similar to the following:

```
[echo] hello world
```

### Specifying `Task` inputs and outputs

In more complex scenarios, a `Task` requires you to define inputs and outputs. For example, a
`Task` could fetch source code from a GitHub repository and build a Docker image from it.

Use one or more [`PipelineResources`](resources.md) to define the artifacts you want to pass in
and out of your `Task`. The following are examples of the most commonly needed resources.

The [`git` resource](resources.md#git-resource) specifies a git repository with
a specific revision from which the `Task` will pull the source code:

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

The [`image` resource](resources.md#image-resource) specifies the repository to which the image built by the `Task` will be pushed:

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

In the following example, you can see a `Task` definition with the `git` input and `image` output
introduced earlier. The arguments of the `Task` command support variable substitution so that
the `Task` definition is constant and the value of parameters can change during runtime.

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: build-docker-image-from-git-source
spec:
  params:
    - name: pathToDockerFile
      type: string
      description: The path to the dockerfile to build
      default: $(resources.inputs.docker-source.path)/Dockerfile
    - name: pathToContext
      type: string
      description: |
        The build context used by Kaniko
        (https://github.com/GoogleContainerTools/kaniko#kaniko-build-contexts)
      default: $(resources.inputs.docker-source.path)
  resources:
    inputs:
      - name: docker-source
        type: git
    outputs:
      - name: builtImage
        type: image
  steps:
    - name: build-and-push
      image: gcr.io/kaniko-project/executor:v0.16.0
      # specifying DOCKER_CONFIG is required to allow kaniko to detect docker credential
      env:
        - name: "DOCKER_CONFIG"
          value: "/tekton/home/.docker/"
      command:
        - /kaniko/executor
      args:
        - --dockerfile=$(params.pathToDockerFile)
        - --destination=$(resources.outputs.builtImage.url)
        - --context=$(params.pathToContext)
```

### Configuring `Task` execution credentials

Before you can execute your `TaskRun`, you must create a `secret` to push your image
to your desired image registry:

```bash
kubectl create secret docker-registry regcred \
                    --docker-server=<your-registry-server> \
                    --docker-username=<your-name> \
                    --docker-password=<your-pword> \
                    --docker-email=<your-email>
```

You must also specify a `ServiceAccount` that uses this `secret` to execute your `TaskRun`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tutorial-service
secrets:
  - name: regcred
```

Save the `ServiceAccount` definition above to a file and apply the YAML file to make the `ServiceAccount` available for your `TaskRun`:

```bash
kubectl apply -f <name-of-file.yaml>
```

### Running your `Task`

You are now ready for your first `TaskRun`!

A `TaskRun` binds the inputs and outputs to already defined `PipelineResources`, sets values
for variable substitution parameters, and executes the `Steps` in the `Task`.

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: build-docker-image-from-git-source-task-run
spec:
  serviceAccountName: tutorial-service
  taskRef:
    name: build-docker-image-from-git-source
  params:
    - name: pathToDockerFile
      value: Dockerfile
    - name: pathToContext
      value: $(resources.inputs.docker-source.path)/examples/microservices/leeroy-web #configure: may change according to your source
  resources:
    inputs:
      - name: docker-source
        resourceRef:
          name: skaffold-git
    outputs:
      - name: builtImage
        resourceRef:
          name: skaffold-image-leeroy-web
```

Save the YAML files that contain your `Task`, `TaskRun`, and `PipelineResource` definitions and apply them using the following command:

```bash
kubectl apply -f <name-of-file.yaml>
```

To examine the resources you've created so far, use the following command:

```bash
kubectl get tekton-pipelines
```

The output will look similar to the following:

```
NAME                                                   AGE
taskruns/build-docker-image-from-git-source-task-run   30s

NAME                                          AGE
pipelineresources/skaffold-git                6m
pipelineresources/skaffold-image-leeroy-web   7m

NAME                                       AGE
tasks/build-docker-image-from-git-source   7m
```

To see the result of executing your `TaskRun`, use the following command:

```bash
tkn taskrun describe build-docker-image-from-git-source-task-run
```

The output will look similar to the following:

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

The `Succeeded` status indicates the `Task` has completed with no errors. You
can also confirm that the output Docker image has been created in the location specified in the resource definition.

To view detailed information about the execution of your `TaskRun`, view the logs:

```bash
tkn taskrun logs build-docker-image-from-git-source-task-run
```

## Creating and running a `Pipeline`

A [`Pipeline`](pipelines.md) defines an ordered series of `Tasks` that you want to execute
along with the corresponding inputs and outputs for each `Task`. You can specify whether the output of one
`Task` is used as an input for the next `Task` using the [`from`](pipelines.md#from) property.
`Pipelines` offer the same variable substitution as `Tasks`.

Below is an example definition of a `Pipeline`:

```yaml
apiVersion: tekton.dev/v1beta1
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

The above `Pipeline` is referencing a `Task` called `deploy-using-kubectl` defined as follows:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: deploy-using-kubectl
spec:
  params:
    - name: path
      type: string
      description: Path to the manifest to apply
    - name: yamlPathToImage
      type: string
      description: |
        The path to the image to replace in the yaml manifest (arg to yq)
  resources:
    inputs:
      - name: source
        type: git
      - name: image
        type: image
  steps:
    - name: replace-image
      image: mikefarah/yq
      command: ["yq"]
      args:
        - "w"
        - "-i"
        - "$(params.path)"
        - "$(params.yamlPathToImage)"
        - "$(resources.inputs.image.url)"
    - name: run-kubectl
      image: lachlanevenson/k8s-kubectl
      command: ["kubectl"]
      args:
        - "apply"
        - "-f"
        - "$(params.path)"
```

### Configuring `Pipeline` execution credentials

The `run-kubectl` step in the above example requires additional permissions. You must grant those
permissions to your `ServiceAccount`.

First, create a new role called `tutorial-role`:

```bash
kubectl create clusterrole tutorial-role \
               --verb=* \
               --resource=deployments,deployments.apps
```

Next, assign this new role to your `ServiceAccount`:

```bash
kubectl create clusterrolebinding tutorial-binding \
             --clusterrole=tutorial-role \
             --serviceaccount=default:tutorial-service
```

To run your `Pipeline`, instantiate it with a [`PipelineRun`](pipelineruns.md) as follows:

```yaml
apiVersion: tekton.dev/v1beta1
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

The `PipelineRun` automatically defines a corresponding `TaskRun` for each `Task` you have defined
in your `Pipeline` collects the results of executing each `TaskRun`. In our example, the
`TaskRun` order is as follows:

1. `tutorial-pipeline-run-1-build-skaffold-web` runs `build-skaffold-web`,
   since it has no [`from` or `runAfter` clauses](pipelines.md#ordering).
1. `tutorial-pipeline-run-1-deploy-web` runs `deploy-web` because
   its [input](tasks.md#inputs) `web-image` comes [`from`](pipelines.md#from)
   `build-skaffold-web`. Thus, `build-skaffold-web` must run before `deploy-web`.

Save the `Task`, `Pipeline`, and `PipelineRun` definitions above to as YAML files and apply them using the following command:

```bash
kubectl apply -f <name-of-file.yaml>
```
**Note:** Also apply the `deploy-task` or the `PipelineRun` will not execute.

You can monitor the execution of your `PipelineRun` in realtime as follows:

```bash
tkn pipelinerun logs tutorial-pipeline-run-1 -f
```

To view detailed information about your `PipelineRun`, use the following command:

```bash
tkn pipelinerun describe tutorial-pipeline-run-1
```

The output will look similar to the following:

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

The `Succeeded` status indicates that your `PipelineRun` completed without errors.
You can also see the statuses of the individual `TaskRuns`.

## Running this tutorial locally

This section provides guidelines for completing this tutorial on your local workstation on:

- [Docker for Desktop](#prerequisites-docker-for-desktop)
- [Minikube](#prerequisites-minikube)

### Prerequisites: Docker for Desktop

Complete these prerequisites to run this tutorial locally using Docker for Desktop:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md#requirements).
- Install [Docker for Desktop](https://www.docker.com/products/docker-desktop) and configure it to use six CPUs,
  10 GB of RAM and 2GB of swap space.
- Set `host.docker.internal:5000` as an insecure registry with Docker for Desktop. See the [Docker insecure registry documentation](https://docs.docker.com/registry/insecure/).
  for details.
- Pass `--insecure` as an argument to your Kaniko tasks so that you can push to an insecure registry.
- Run a local (insecure) Docker registry as follows:

  `docker run -d -p 5000:5000 --name registry-srv -e REGISTRY_STORAGE_DELETE_ENABLED=true registry:2`

- (Optional) Install a Docker registry viewer to verify the images have been pushed:

`docker run -it -p 8080:8080 --name registry-web --link registry-srv -e REGISTRY_URL=http://registry-srv:5000/v2 -e REGISTRY_NAME=localhost:5000 hyper/docker-registry-web`

- Verify that you can push to `host.docker.internal:5000/myregistry/<image_name>`.

### Reconfigure `image` resources

You must reconfigure any `image` resource definitions in your `PipelineResources` as follows:

- Set the URL to `host.docker.internal:5000/myregistry/<image_name>`
- Set the `KO_DOCKER_REPO` variable to `localhost:5000/myregistry` before using `ko`
- Set your applications (such as deployment definitions) to push to
  `localhost:5000/myregistry/<image name>`.

### Reconfigure logging

- You can keep your logs in memory only without sending them to a logging service
  such as [Stackdriver](https://cloud.google.com/logging/).
- You can deploy Elasticsearch, Beats, or Kibana locally to view logs. You can find an
  example configuration at <https://github.com/mgreau/tekton-pipelines-elastic-tutorials>.
- To learn more about obtaining logs, see [Logs](logs.md).

### Prerequisites: Minikube

Complete these prerequisites to run this tutorial locally using Minikube:

- Install the [required tools](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md#requirements).
- Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) and start a session as follows:
```bash
minikube start --memory 6144 --cpus 2
```
- Point your shell to minikube's docker-daemon by running `eval $(minikube -p minikube docker-env)`
- Set up a [registry on minikube](https://github.com/kubernetes/minikube/tree/master/deploy/addons/registry-aliases) by running `minikube addons enable registry` and `minikube addons enable registry-aliases`

### Reconfigure `image` resources

The `registry-aliases` addon will create several aliases for the minikube registry. You'll need to reconfigure your `image` resource definitions to use one of these aliases in your `PipelineResources` (for this tutorial, we use `example.com`; for a full list of aliases, you can run `minikube ssh -- cat /etc/hosts`. You can also configure your own alias by editing minikube's `/etc/hosts` file and the `coredns` configmap in the `kube-system` namespace).

- Set the URL to `example.com/<image_name>`
- When using `ko`, be sure to [use the `-L` flag](https://github.com/google/ko/blob/master/README.md#with-minikube) (i.e. `ko apply -L -f config/`)
- Set your applications (such as deployment definitions) to push to
  `example.com/<image name>`.

If you wish to use a different image URL, you can add the appropriate line to minikube's `/etc/hosts`.

### Reconfigure logging

See the information in the "Docker for Desktop" section

## Further reading

To learn more about the Tekton Pipelines entities involved in this tutorial, see the following topics:

- [`Tasks`](tasks.md)
- [`TaskRuns`](taskruns.md)
- [`Pipelines`](pipelines.md)
- [`PipelineResources`](resources.md)
- [`PipelineRuns`](pipelineruns.md)

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
