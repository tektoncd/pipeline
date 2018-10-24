# Pipeline CRDs
Pipeline CRDs is an open source implementation to configure and run CI/CD style pipelines for your kubernetes application.

Pipeline CRDs creates [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) as building blocks to declare pipelines.

A custom resource is an extension of Kubernetes API which can create a custom [Kubernetes Object](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#understanding-kubernetes-objects).
Once a custom resource is installed, users can create and access its objects with kubectl, just as they do for built-in resources like pods, deployments etc.
These resources run on-cluster and are implemeted by [Kubernetes Custom Resource Definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions).


## Building Blocks of Pipeline CRDs
Below diagram lists the main custom resources created by Pipeline CRDs

![Building Blocks](./docs/images/building-blocks.png)

### Task
A Task is a collection of sequential steps you would want to run as part of your continous integration flow.
A task will run inside a container on your cluster. A Task declares,
1. Inputs the task needs.
1. Outputs the task will produce.
1. Sequence of steps to execute. Each step is [a container image](#image-contract).

Here is an example simple Task definition which echoes "hello world". The `hello-world` task does not define any inputs or outputs.

It only has one step named `echo`. The step uses the builder image `busybox` whose entrypoint set to `\bin\sh`.

```shell
apiVersion: pipeline.knative.dev/v1alpha1
kind: Task
metadata:
  name: hello-world
  namespace: default
spec:
  buildSpec:
    steps:
      - name: echo
        image: busybox
        command:
          - echo
        args:
          - "hello world!"
```
Examples of `Task` definitions with inputs and outputs are [here](./examples)

### Step Entrypoint

To get the logs out of a [step](#task), we provide our own executable that wraps
the `command` and `args` values specified in the `step`. This means that every
`Task` must use `command`, and cannot rely on the image's `entrypoint`.

### Pipeline
`Pipeline` describes a graph of [Tasks](#Task) to execute.

Below, is a simple pipeline which runs `hello-world-task` twice one after the other.

In this `echo-hello-twice` pipeline, there are two named tasks; `hello-world-first` and `hello-world-again`.

Both the tasks, refer to [`Task`](#Task) `hello-world` mentioned in `taskRef` config.

```shell
apiVersion: pipeline.knative.dev/v1alpha1
kind: Pipeline
metadata:
  name: echo-hello-twice
  namespace: default
spec:
  tasks:
    - name: hello-world-first
      taskRef:
        name: hello-world
    - name: hello-world-again
      taskRef:
        name: hello-world
```
_TODO: Consider making `taskRef` more [concise](https://github.com/knative/build-pipeline/issues/138)_
_TODO: Consider specifying task dependency_

Examples of pipelines with complex DAGs are [here](./examples/pipelines)

### PipelineResources
`PipelinesResources` in a pipeline are the set of objects that are going to be used as inputs to a [`Task`](#Task) and can be output of [`Task`](#Task) .
For e.g.:
A task's input could be a github source which contains your application code.
A task's output can be your application container image which can be then deployed in a cluster.
Read more on PipelineResources and their types [here](./docs/pipeline-resources.md)

### PipelineParams
_TODO add text here_

### Image Contract

Each container image used as a step in a [`Task`](#task) must comply with a specific
contract.

* [The `entrypoint` of the image will be ignored](#step-entrypoint)

For example, in the following Task the images, `gcr.io/cloud-builders/gcloud`
and `gcr.io/cloud-builders/docker` run as steps:

```yaml
spec:
  buildSpec:
    steps:
    - image: gcr.io/cloud-builders/gcloud
      command: ['gcloud']
      ...
    - image: gcr.io/cloud-builders/docker
      command: ['docker']
      ...
```

You can also provide `args` to the image's `command`:

```yaml
steps:
- image: ubuntu
  command: ['/bin/bash']
  args: ['-c', 'echo hello $FOO']
  env:
  - name: 'FOO'
    value: 'world'
```

### Images Conventions

 * `/workspace`: If an input is provided, the default working directory will be
   `/workspace` and this will be shared across `steps` (note that in
   [#123](https://github.com/knative/build-pipeline/issues/123) we will add supprots for multiple input workspaces)
 * `/builder/home`: This volume is exposed to steps via `$HOME`.
 * Credentials attached to the Build's service account may be exposed as Git or
   Docker credentials as outlined
   [in the auth docs](https://github.com/knative/docs/blob/master/build/auth.md#authentication).