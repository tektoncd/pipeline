# Hello World Task

The main objective of the Pipeline CRD is to run your task individually or as a
part of a pipeline. Every task runs as a Pod on your Kubernetes cluster with
each step as a its own container.

## Tasks

A `Task` defines the work that needs to be executed, for example the following
is a simple task that will echo hello world,

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
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

A `TaskRun` runs the `task` you defined. Here is a simple example of a taskRun
you can use to exeute your task

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  name: echo-hello-world-task-run
spec:
  taskRef:
    name: echo-hello-world
  trigger:
    type: manual
```

To apply the yaml files use the following command

```bash
kubectl apply -f <name-of-file.yaml>
```

To see the output of the taskRun, use the following command

```bash
kubectl get taskruns/echo-hello-world-task-run -o yaml
```

you will get an output similar to the following:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"pipeline.knative.dev/v1alpha1","kind":"TaskRun","metadata":{"annotations":{},"name":"echo-hello-world-task-run","namespace":"default"},"spec":{"taskRef":{"name":"echo-hello-world"},"trigger":{"type":"manual"}}}
  creationTimestamp: 2018-12-11T15:49:13Z
  generation: 1
  name: echo-hello-world-task-run
  namespace: default
  resourceVersion: "6706789"
  selfLink: /apis/pipeline.knative.dev/v1alpha1/namespaces/default/taskruns/echo-hello-world-task-run
  uid: 4e96e9c6-fd5c-11e8-9129-42010a8a0fdc
spec:
  generation: 1
  inputs: {}
  outputs: {}
  taskRef:
    name: echo-hello-world
  taskSpec: null
  trigger:
    type: manual
status:
  conditions:
    - lastTransitionTime: 2018-12-11T15:50:09Z
      status: "True"
      type: Succeeded
  podName: echo-hello-world-task-run-pod-85ca51
  startTime: 2018-12-11T15:49:39Z
  steps:
    - logsURL: ""
      terminated:
        containerID: docker://fcfe4a004...6729d6d2ad53faff41
        exitCode: 0
        finishedAt: 2018-12-11T15:50:01Z
        reason: Completed
        startedAt: 2018-12-11T15:50:01Z
    - logsURL: ""
      terminated:
        containerID: docker://fe86fc5f7...eb429697b44ce4a5b
        exitCode: 0
        finishedAt: 2018-12-11T15:50:02Z
        reason: Completed
        startedAt: 2018-12-11T15:50:02Z
```

The status of type `Succeeded = True` shows the task ran successfully

# Task Inputs and Outputs

In more common scenarios, your task needs multiple steps with input and output
resources to process, for example a task could fetch source code from a github
repository and build a docker image from it.

`PipelinesResources` are used to define the artifacts that can be passed in and
out of a task. There are a few system defined resource types ready to use, and
the following are two examples of the resources commonly needed

`git` resource represents a git repository with a specific revision

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: skaffold-git
spec:
  type: git
  params:
    - name: revision
      value: master
    - name: url
      value: https://github.com/GoogleContainerTools/skaffold
```

and the `image` resource represents the docker image to be built by the task

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: skaffold-image-leeroy-web
spec:
  type: image
  params:
    - name: url
      value: gcr.io/<use your project>/leeroy-web
```

The following is a task with inputs and outputs. The input resource is a github
repository and the output is the docker image produced from that source. The
args of the task command support templating so that the definition of task is
constant and the value of parameters can change in runtime.

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Task
metadata:
  name: build-docker-image-from-git-source
spec:
  inputs:
    resources:
      - name: workspace
        type: git
    params:
      - name: pathToDockerFile
        description: The path to the dockerfile to build
        default: /workspace/Dockerfile
      - name: pathToContext
        description:
          The build context used by Kaniko
          (https://github.com/GoogleContainerTools/kaniko#kaniko-build-contexts)
        default: /workspace
  outputs:
    resources:
      - name: builtImage
        type: image
  steps:
    - name: build-and-push
      image: gcr.io/kaniko-project/executor
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
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  name: build-docker-image-from-git-source-task-run
spec:
  taskRef:
    name: build-docker-image-from-git-source
  trigger:
    type: manual
  inputs:
    resources:
      - name: workspace
        resourceRef:
          name: skaffold-git
    params:
      - name: pathToDockerFile
        value: Dockerfile
      - name: pathToContext
        value: /workspace/examples/microservices/leeroy-web
  outputs:
    resources:
      - name: builtImage
        resourceRef:
          name: skaffold-image-leeroy-web
```

To apply the yaml files use the following command, you will need to apply the
two resources, the task and taskrun.

```bash
kubectl apply -f <name-of-file.yaml>
```

To see all the resource created so far as part of the pipeline-crd, run the
command

```bash
kubectl get build-pipeline
```

and you will get an output similar to the following:

```
NAME                                                   AGE
taskruns/build-docker-image-from-git-source-task-run   30s

NAME                                          AGE
pipelineresources/skaffold-git                6m
pipelineresources/skaffold-image-leeroy-web   7m

NAME                                       AGE
tasks/build-docker-image-from-git-source   7m
```

To see the output of the taskRun, use the following command

```bash
kubectl get taskruns/echo-hello-world-task-run -o yaml
```

you will get an output similar to the following:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"pipeline.knative.dev/v1alpha1","kind":"TaskRun","metadata":{"annotations":{},"name":"build-docker-image-from-git-source-task-run","namespace":"default"},"spec":{"inputs":{"params":[{"name":"pathToDockerFile","value":"Dockerfile"},{"name":"pathToContext","value":"/workspace/examples/microservices/leeroy-web"}],"resources":[{"name":"workspace","resourceRef":{"name":"skaffold-git"}}]},"outputs":{"resources":[{"name":"builtImage","resourceRef":{"name":"skaffold-image-leeroy-web"}}]},"results":{"type":"gcs","url":"gcs://somebucket/results/logs"},"taskRef":{"name":"build-docker-image-from-git-source"},"trigger":{"type":"manual"}}}
  creationTimestamp: 2018-12-11T18:14:29Z
  generation: 1
  name: build-docker-image-from-git-source-task-run
  namespace: default
  resourceVersion: "6733537"
  selfLink: /apis/pipeline.knative.dev/v1alpha1/namespaces/default/taskruns/build-docker-image-from-git-source-task-run
  uid: 99d297fd-fd70-11e8-9129-42010a8a0fdc
spec:
  generation: 1
  inputs:
    params:
      - name: pathToDockerFile
        value: Dockerfile
      - name: pathToContext
        value: /workspace/examples/microservices/leeroy-web
    resources:
      - name: workspace
        paths: null
        resourceRef:
          name: skaffold-git
  outputs:
    resources:
      - name: builtImage
        paths: null
        resourceRef:
          name: skaffold-image-leeroy-web
  results:
    type: gcs
    url: gcs://somebucket/results/logs
  taskRef:
    name: build-docker-image-from-git-source
  taskSpec: null
  trigger:
    type: manual
status:
  conditions:
    - lastTransitionTime: 2018-12-11T18:15:09Z
      status: "True"
      type: Succeeded
  podName: build-docker-image-from-git-source-task-run-pod-24d414
  startTime: 2018-12-11T18:14:29Z
  steps:
    - logsURL: ""
      terminated:
        containerID: docker://138ce30c722eed....c830c9d9005a0542
        exitCode: 0
        finishedAt: 2018-12-11T18:14:47Z
        reason: Completed
        startedAt: 2018-12-11T18:14:47Z
    - logsURL: ""
      terminated:
        containerID: docker://4a75136c029fb1....4c94b348d4f67744
        exitCode: 0
        finishedAt: 2018-12-11T18:14:48Z
        reason: Completed
        startedAt: 2018-12-11T18:14:48Z
```

The status of type `Succeeded = True` shows the task ran successfully and you
can also validate the docker image is created in the location specified in the
resource definition.

# Pipeline

Pipeline defines a graph of tasks to execute in a specific order, while also
indicating if any outputs should be used as inputs of a following task by using
the `providedBy` field. The same templating you used in tasks is also available
in pipeline

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Pipeline
metadata:
  name: tutorial-pipeline
spec:
  tasks:
    - name: build-skaffold-web
      taskRef:
        name: build-docker-image-from-git-source
      params:
        - name: pathToDockerFile
          value: Dockerfile
        - name: pathToContext
          value: /workspace/examples/microservices/leeroy-web
    - name: deploy-web
      taskRef:
        name: demo-deploy-kubectl
      resources:
        - name: image
          providedBy:
            - build-skaffold-web
      params:
        - name: path
          value: /workspace/examples/microservices/leeroy-web/kubernetes/deployment.yaml
        - name: yqArg
          value: "-d1"
        - name: yamlPathToImage
          value: "spec.template.spec.containers[0].image"
```

This pipeline is referencing a task to `deploy-using-kubectl` which can be found
here

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Task
metadata:
  name: deploy-using-kubectl
spec:
  inputs:
    resources:
      - name: workspace
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

To run the pipeline, create a pipelineRun as follows,

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineRun
metadata:
  name: tutorial-pipeline-run-1
spec:
  pipelineRef:
    name: tutorial-pipeline
  trigger:
    type: manual
  resources:
    - name: build-skaffold-web
      inputs:
        - name: workspace
          resourceRef:
            name: skaffold-git
      outputs:
        - name: builtImage
          resourceRef:
            name: skaffold-image-leeroy-web
    - name: deploy-web
      inputs:
        - name: workspace
          resourceRef:
            name: skaffold-git
        - name: image
          resourceRef:
            name: skaffold-image-leeroy-web
```

The pipelineRun will create the taskRuns corresponding to each task and collect
the results

To apply the yaml files use the following command, you will need to apply the
`deploy-task` if you want to run the pipeline.

```bash
kubectl apply -f <name-of-file.yaml>
```

To see the output of the pipelineRun, use the following command

```bash
kubectl get pipelineruns/tutorial-pipeline-run-1 -o yaml
```

you will get an output similar to the following:

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineRun
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"pipeline.knative.dev/v1alpha1","kind":"PipelineRun","metadata":{"annotations":{},"name":"tutorial-pipeline-run-1","namespace":"default"},"spec":{"pipelineRef":{"name":"tutorial-pipeline"},"resources":[{"inputs":[{"name":"workspace","resourceRef":{"name":"skaffold-git"}}],"name":"build-skaffold-web","outputs":[{"name":"builtImage","resourceRef":{"name":"skaffold-image-leeroy-web"}}]},{"inputs":[{"name":"workspace","resourceRef":{"name":"skaffold-git"}},{"name":"image","resourceRef":{"name":"skaffold-image-leeroy-web"}}],"name":"deploy-web"}],"trigger":{"type":"manual"}}}
  creationTimestamp: 2018-12-11T20:30:19Z
  generation: 1
  name: tutorial-pipeline-run-1
  namespace: default
  resourceVersion: "6760151"
  selfLink: /apis/pipeline.knative.dev/v1alpha1/namespaces/default/pipelineruns/tutorial-pipeline-run-1
  uid: 93acb0ea-fd83-11e8-9129-42010a8a0fdc
spec:
  generation: 1
  pipelineRef:
    name: tutorial-pipeline
  resources:
    - inputs:
        - name: workspace
          paths: null
          resourceRef:
            name: skaffold-git
      name: build-skaffold-web
      outputs:
        - name: builtImage
          paths: null
          resourceRef:
            name: skaffold-image-leeroy-web
    - inputs:
        - name: workspace
          paths: null
          resourceRef:
            name: skaffold-git
        - name: image
          paths: null
          resourceRef:
            name: skaffold-image-leeroy-web
      name: deploy-web
      outputs: null
  serviceAccount: ""
  trigger:
    type: manual
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
        - logsURL: ""
          terminated:
            containerID: docker://c699fcba94....f96108ac9f4db22b94e0c
            exitCode: 0
            finishedAt: 2018-12-11T20:30:36Z
            reason: Completed
            startedAt: 2018-12-11T20:30:36Z
        - logsURL: ""
          terminated:
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
        - logsURL: ""
          terminated:
            containerID: docker://eaefb7b6d685....f001f895430f71374
            exitCode: 0
            finishedAt: 2018-12-11T20:32:28Z
            reason: Completed
            startedAt: 2018-12-11T20:32:28Z
        - logsURL: ""
          terminated:
            containerID: docker://4cfc6eba47a7a....dcaef1e9b1eee3661b8a85f
            exitCode: 0
            finishedAt: 2018-12-11T20:32:31Z
            reason: Completed
            startedAt: 2018-12-11T20:32:31Z
        - logsURL: ""
          terminated:
            containerID: docker://01b376b92....dce4ccec9641d77
            exitCode: 0
            finishedAt: 2018-12-11T20:32:35Z
            reason: Completed
            startedAt: 2018-12-11T20:32:34Z
```

The status of type `Succeeded = True` shows the pipeline ran successfully, also
the status of individual task runs are shown.
