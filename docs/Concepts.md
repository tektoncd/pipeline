# Pipeline CRDs

Pipeline CRDs is an open source implementation to configure and run CI/CD style
pipelines for your Kubernetes application.

Pipeline CRDs creates
[Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
as building blocks to declare pipelines.

A custom resource is an extension of Kubernetes API which can create a custom
[Kubernetes Object](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#understanding-kubernetes-objects).
Once a custom resource is installed, users can create and access its objects
with kubectl, just as they do for built-in resources like pods, deployments etc.
These resources run on-cluster and are implemented by
[Kubernetes Custom Resource Definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions).

High level details of this design:

- [Pipelines](#pipeline) do not know what will trigger them, they can be
  triggered by events or by manually creating [PipelineRuns](#pipelinerun)
- [Tasks](#task) can exist and be invoked completely independently of
  [Pipelines](#pipeline); they are highly cohesive and loosely coupled
- [Tasks](#task) can depend on artifacts, output and parameters created by other
  tasks.
- [PipelineResources](#pipelineresources) are the artifacts used as inputs and
  outputs of Tasks.

## Building Blocks of Pipeline CRDs

Below diagram lists the main custom resources created by Pipeline CRDs:

- [`Task`](#task)
- [`ClusterTask`](#clustertask)
- [`Pipeline`](#pipeline)
- [Runs](#runs)
  - [`PipelineRun`](#pipelinerun)
  - [`TaskRun`](#taskrun)
- [`PipelineResources`](#pipelineresources)

![Building Blocks](./images/building-blocks.png)

### Task

A `Task` is a collection of sequential steps you would want to run as part of
your continuous integration flow. A task will run inside a container on your
cluster.

A `Task` declares:

- [Inputs](#inputs)
- [Outputs](#outputs)
- [Steps](#steps)

#### Inputs

Declare the inputs the `Task` needs. Every `Task` input resource should provide
name and type (like git, image).

#### Outputs

Outputs declare the outputs `Task` will produce.

#### Steps

Steps is a sequence of steps to execute. Each step is
[a container image](./using.md#image-contract).

Here is an example simple `Task` definition which echoes "hello world". The
`hello-world` task does not define any inputs or outputs.

It only has one step named `echo`. The step uses the builder image `busybox`
whose entrypoint set to `/bin/sh`.

```yaml
apiVersion: pipeline.knative.dev/v1alpha1
kind: Task
metadata:
  name: hello-world
  namespace: default
spec:
  steps:
    - name: echo
      image: busybox
      command:
        - echo
      args:
        - "hello world!"
```

Examples of `Task` definitions with inputs and outputs are [here](../examples)

#### Cluster Task

A `ClusterTask` is similar to `Task` but with a cluster-wide scope. Cluster
Tasks are available in all namespaces, typically used to conveniently provide
commonly used tasks to users.

#### Pipeline

A `Pipeline` describes a graph of [Tasks](#Task) to execute.

Below, is a simple pipeline which runs `hello-world-task` twice one after the
other.

In this `echo-hello-twice` pipeline, there are two named tasks;
`hello-world-first` and `hello-world-again`.

Both the tasks, refer to [`Task`](#Task) `hello-world` mentioned in `taskRef`
config.

```yaml
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

Examples of more complex `Pipelines` are [in our examples dir](../examples/).

#### PipelineResources

`PipelinesResources` in a pipeline are the set of objects that are going to be
used as inputs to a [`Task`](#Task) and can be output of [`Task`](#Task) .

For example:

- A Task's input could be a GitHub source which contains your application code.
- A Task's output can be your application container image which can be then
  deployed in a cluster.
- A Task's output can be a jar file to be uploaded to a storage bucket.

Read more on PipelineResources and their types
[here](./using.md#creating-pipelineresources).

`PipelineResources` in a Pipeline are the set of objects that are going to be
used as inputs and outputs of a `Task`.

#### Runs

To invoke a [`Pipeline`](#pipeline) or a [`Task`](#task), you must create a
corresponding `Run` object:

- [TaskRun](#taskrun)
- [PipelineRun](#pipelinerun)

##### TaskRun

Creating a `TaskRun` invokes a [Task](#task), running all of the steps until
completion or failure. Creating a `TaskRun` requires satisfying all of the input
requirements of the `Task`.

`TaskRun` definition includes `inputs`, `outputs` for `Task` referred in spec.

Input and output resources include the PipelineResource's name in the `Task`
spec and a reference to the actual `PipelineResource` that should be used.

`TaskRuns` can be created directly by a user or by a
[PipelineRun](#pipelinerun).

##### PipelineRun

Creating a `PipelineRun` invokes the pipeline, creating [TaskRuns](#taskrun) for
each task in the pipeline.

A `PipelineRun` ties together:

- A [Pipeline](#pipeline)
- The [PipelineResources](#pipelineresources) to use for each [Task](#task)
- Which **serviceAccount** to use (provided to all tasks)
- Where **results** are stored (e.g. in GCS)

A `PipelineRun` could be created:

- By a user manually
- In response to an event (e.g. in response to a GitHub event, possibly
  processed via [Knative eventing](https://github.com/knative/eventing))
