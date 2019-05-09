# Tekton Pipelines

Tekton Pipelines is an open source implementation to configure and run CI/CD
style pipelines for your Kubernetes application.

Pipeline creates
[Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
as building blocks to declare pipelines.

A custom resource is an extension of Kubernetes API which can create a custom
[Kubernetes Object](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#understanding-kubernetes-objects).
Once a custom resource is installed, users can create and access its objects
with kubectl, just as they do for built-in resources like pods, deployments etc.
These resources run on-cluster and are implemented by
[Kubernetes Custom Resource Definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions).

High level details of this design:

- [Pipelines](pipelines.md) do not know what will trigger them, they can be
  triggered by events or by manually creating [PipelineRuns](pipelineruns.md)
- [Tasks](tasks.md) can exist and be invoked completely independently of
  [Pipelines](pipelines.md); they are highly cohesive and loosely coupled
- [Tasks](tasks.md) can depend on artifacts and parameters created by other
  tasks.
- [Tasks](tasks.md) can be invoked via [TaskRuns](taskruns.md)
- [PipelineResources](resources.md) are the artifacts used as inputs and outputs
  of Tasks.

## Usage

- [How do I create a new Pipeline?](pipelines.md)
- [How do I make a Task?](tasks.md)
- [How do I make Resources?](resources.md)
- [How do I control auth?](auth.md)
- [How do I run a Pipeline?](pipelineruns.md)
- [How do I run a Task on its own?](taskruns.md)

## Learn more

See the following reference topics for information about each of the build
components:

- [`Task`](tasks.md)
- [`TaskRun`](taskruns.md)
- [`Pipeline`](pipelines.md)
- [`PipelineRun`](pipelineruns.md)
- [`PipelineResource`](resources.md)

Additional reference topics not related to a specific component:

- [Labels](labels.md)

## Try it out

- Follow along with [the tutorial](tutorial.md)
- Look at
  [the examples](https://github.com/tektoncd/pipeline/tree/master/examples)

## Related info

If you are interested in contributing to the Tekton Pipeline project, see the
[Tekton Pipeline contribution guide](https://github.com/tektoncd/pipeline/blob/master/CONTRIBUTING.md).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
