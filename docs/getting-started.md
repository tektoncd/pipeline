# Getting Started with Tekton Pipelines

This guide helps you get started with Tekton Pipelines by walking you through a simple `Hello World` tutorial. The tutorial shows you how to:

- Create a `Task`	
- Create a `Pipeline` containing your `Tasks`	
- Use a `TaskRun` to instantiate and execute a `Task` outside of a `Pipeline`	
- Use a `PipelineRun` to instantiate and run a `Pipeline` containing your `Tasks`

## Before you begin

Before you begin this tutorial, make sure you have [installed and configured](install.md)
the latest release of Tekton on your Kubernetes cluster.

### Install Minikube
If you would like to complete this tutorial on your local workstation, you will need Minikube.
Install [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) v1.50 or higher.

### Install Tekton CLI
This getting started guide uses the Tekton CLI, tkn, so make sure to install that on your local machine.
Install [Tekton CLI](https://github.com/tektoncd/cli). 

### Sign in to Docker Hub
You will need an account on [Docker Hub](https://hub.docker.com).

## Creating and running a `Task`

A [`Task`](tasks.md) defines a series of `Steps` that run in a desired order and complete a set amount of build work. Every `Task` runs as a Pod on your Kubernetes cluster with each `Step` as its own container. For example, the following `Task` outputs "Hello World":

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

Apply your YAML files as follows:

```bash
kubectl apply -f <YOUR-TASK.yaml>
kubectl apply -f <YOUR-TASKRUN.yaml>
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
`Task` could fetch source code with a Dockerfile and build a Docker image from it.

Use one or more [`PipelineResources`](resources.md) to define the artifacts you want to pass in
and out of your `Task`. The following are examples of the most commonly needed resources.

The [`git` resource](resources.md#git-resource) specifies a git repository with
a specific revision from which the `Task` will pull the source code.

**Note:**  In the yaml below replace <em>'https://github.com/popcor255/python-flask-docker-hello-world'</em> with a git repository that has Dockerfile.

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: hello-world-git
spec:
  type: git
  params:
    - name: revision
      value: master
    - name: url
      value: https://github.com/popcor255/python-flask-docker-hello-world #configure: change if you want to build something else, perhaps from your own local git repository.
```

The [`image` resource](resources.md#image-resource) specifies the repository to which the image built by the `Task` will be pushed:

**Note:**  In the yaml below replace the <em>'docker.io/<your docker hub username>/hello-world'</em> with your docker hub repository

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: hello-world-image
spec:
  type: image
  params:
    - name: url
      value: docker.io/<your docker hub username>/hello-world #configure: replace with where the image should go: perhaps your local registry or Dockerhub with a secret and configured service account
```

In the following example, you can see a `Task` definition with the `git` input and `image` output
introduced earlier. The arguments of the `Task` command support variable substitution so that
the `Task` definition is constant and the value of parameters can change at runtime.

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: build-and-push-docker-image-from-git
spec:
  params:
    - name: builder_image
      description: The location of the builder image
      default: quay.io/buildah/stable:v1.14.3
  resources:
    inputs:
      - name: source
        type: git
    outputs:
      - name: image
        type: image
  steps:
    - name: build-and-push
      image: $(params.builder_image)
      workingDir: /workspace/source
      command: ["/bin/bash"]
      args:
        - -c
        - |
          set -e
          IMAGE_NAME="$(resources.outputs.image.url)"
          buildah bud --tls-verify="false" --layers -f "./Dockerfile" -t "$IMAGE_NAME:latest" .
          buildah push --tls-verify="false" "$IMAGE_NAME:latest" "docker://$IMAGE_NAME:latest"
      securityContext:
        privileged: true
      volumeMounts:
        - name: varlibcontainers
          mountPath: /var/lib/containers
  volumes:
    - name: varlibcontainers
      emptyDir: {}
```

### Configuring `Task` execution credentials

Before you can execute your `TaskRun`, you must create a `secret` to push your image
to your desired image registry:

**Note:** You can get your Docker access token at [Docker Hub](https://hub.docker.com/settings/security).

```bash
kubectl create secret docker-registry regcred \
                    --docker-server=docker.io \
                    --docker-username=<your-name> \
                    --docker-password=<your-token> \
                    --docker-email=<your-email>
```

You must specify a `ServiceAccount` that uses this `secret` to execute your `TaskRun`:

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
kubectl apply -f <YOUR-SERVICE-ACCOUNT.yaml>
```

### Running your `Task`

You are now ready for your first `TaskRun`!

A `TaskRun` binds the inputs and outputs to already defined `PipelineResources`, sets values
for variable substitution parameters, and executes the `Steps` in the `Task`.

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: build-and-push-docker-image-from-git-task-run
spec:
  serviceAccountName: tutorial-service
  taskRef:
    name: build-and-push-docker-image-from-git
  resources:
    inputs:
      - name: source
        resourceRef:
          name: hello-world-git
    outputs:
      - name: image
        resourceRef:
          name: hello-world-image
```

Save the YAML files that contain your Task, and `PipelineResource` definitions and apply them using the following command:

```bash
kubectl apply -f <YOUR-TASK.yaml>
kubectl apply -f <YOUR-TASKRUN.yaml>
```

To examine the resources you've created so far, use the following command:

```bash
kubectl get tekton-pipelines
```

The output will look similar to the following:

```
NAME                                                               SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
taskrun.tekton.dev/build-and-push-docker-image-from-git-task-run   True        Succeeded   9m2s        7m18s

NAME                                                   AGE
task.tekton.dev/build-and-push-docker-image-from-git   9m11s

NAME                                            AGE
pipelineresource.tekton.dev/hello-world-git     48m
pipelineresource.tekton.dev/hello-world-image   48m
```

To see the result of executing your `TaskRun`, use the following command:

```bash
tkn taskrun describe build-docker-image-from-git-source-task-run
```

The output will look similar to the following:

```
Name:        build-and-push-docker-image-from-git-task-run
Namespace:   default
Task Ref:    build-and-push-docker-image-from-git
Service Account:   tutorial-service

Status
STARTED          DURATION    STATUS
10 minutes ago   1 minute    Succeeded

Input Resources
No resources

Output Resources
No resources

Params
No params

Steps
NAME                               STATUS
build-and-push                     Completed
create-dir-image-lb62b             Completed
git-source-hello-world-git-lzmc2   Completed
image-digest-exporter-j7kjr        Completed
image-digest-exporter-zkgqd        Completed
```

The `Succeeded` status indicates the `Task` has completed with no errors. You
can confirm that the output Docker image has been created in the location specified in the resource definition.

To view detailed information about the execution of your `TaskRun`, view the logs:

```bash
tkn taskrun logs build-and-push-docker-image-from-git-task-run
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
    - name: source
      type: git
    - name: image
      type: image
  tasks:
    - name: build-and-push-to-dockerhub
      taskRef:
        name: build-and-push-docker-image-from-git
      resources:
        inputs:
          - name: source
            resource: source
        outputs:
          - name: image
            resource: image
    - name: deploy-app
      taskRef:
        name: deploy-using-kubectl
      resources:
        inputs:
          - name: image
            resource: image
            from:
              - build-and-push-to-dockerhub
```

The above `Pipeline` is referencing a `Task` called `deploy-using-kubectl` defined as follows:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: deploy-using-kubectl
spec:
  params:
    - name: URL
      type: string
      description: The location of the infra/yaml that will be applied
      default: "https://raw.githubusercontent.com/popcor255/Kubernetes-Objects/master/pods/"
    - name: PATH
      type: string
      description: This is the name of the file
      default: my-pod.yaml
  resources:
    inputs:
      - name: image
        type: image
  steps:
    - name: fetch-yaml
      image: ellerbrock/alpine-bash-curl-ssl
      command: ["/bin/bash"]
      args:
        - -c
        - |
          set -e
          curl $(params.URL)$(params.PATH) > $(params.PATH)
    - name: replace-image
      image: mikefarah/yq
      command: ["yq"]
      args:
        - "w"
        - "-i"
        - "$(params.PATH)"
        - "spec.containers[0].image"
        - "$(resources.inputs.image.url)"
    - name: run-kubectl
      image: lachlanevenson/k8s-kubectl
      command: ["kubectl"]
      args:
        - "apply"
        - "-f"
        - "$(params.PATH)"
```

### Configuring `Pipeline` execution credentials

The `run-kubectl` step in the above example requires additional permissions. You must grant those
permissions to your `ServiceAccount`.

First, create a new role called `tutorial-role`:

```bash
kubectl create clusterrole tutorial-role \
               --verb=* \
               --resource=pods,deployments,deployments.apps
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
    - name: source
      resourceRef:
        name: hello-world-git
    - name: image
      resourceRef:
        name: hello-world-image
```

The `PipelineRun` automatically defines a corresponding `TaskRun` for each `Task` you have defined
in your `Pipeline` collects the results of executing each `TaskRun`. In our example, the
`TaskRun` order is as follows:

1. `tutorial-pipeline-run-1-build-and-push-to-dockerhub-x7pvq` runs `build-and-push-to-dockerhub`,
   since it has no [`from` or `runAfter` clauses](pipelines.md#ordering).
1. `tutorial-pipeline-run-1-deploy-app-mnc8l  ` runs `deploy-app` because
   its [input](tasks.md#inputs) `hell-world-image` comes [`from`](pipelines.md#from)
   `build-and-push-to-dockerhub`. Thus, `build-and-push-to-dockerhub` must run before `deploy-app`.

Save the `Task`, `Pipeline`, and `PipelineRun` definitions above to as YAML files and apply them using the following command:

```bash
kubectl apply -f <YOUR-TASK.yaml>
kubectl apply -f <YOUR-PIPELINE.yaml>
kubectl apply -f <YOUR-PIPELINERUN.yaml>

```
**Note:**  Apply the `deploy-task` or the `PipelineRun` will not execute.

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
Name:              tutorial-pipeline-run-1
Namespace:         default
Pipeline Ref:      tutorial-pipeline
Service Account:   tutorial-service

Status
STARTED         DURATION   STATUS
9 minutes ago   1 minute   Succeeded

Resources
NAME     RESOURCE REF
source   hello-world-git
image    hello-world-image

Params
No params

Taskruns
NAME                                                        TASK NAME                     STARTED         DURATION     STATUS
tutorial-pipeline-run-1-deploy-app-mnc8l                    deploy-app                    7 minutes ago   10 seconds   Succeeded
tutorial-pipeline-run-1-build-and-push-to-dockerhub-x7pvq   build-and-push-to-dockerhub   9 minutes ago   1 minute     Succeeded
```

The `Succeded` status indicates that your `PipelineRun` completed without errors.
You can see the statuses of the individual `TaskRuns`.

## Further reading

To learn more about the Tekton Pipelines entities involved in this tutorial, see the following topics:

- [`Tasks`](tasks.md)
- [`TaskRuns`](taskruns.md)
- [`Pipelines`](pipelines.md)
- [`PipelineResources`](resources.md)
- [`PipelineRuns`](pipelineruns.md)
- [`Logs`](logs.md)

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
