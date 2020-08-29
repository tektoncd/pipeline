# Tekton Pipelines 2020 Roadmap

This is an incomplete list of work we hope to accomplish in 2020.

Highlights:

- [Tekton mission and vision](https://github.com/tektoncd/community/blob/master/roadmap.md#mission-and-vision)
- ["Feature complete"](#feature-complete)
- [Beta for all components](#beta-for-all-components)
- [Task interfaces and PipelineResources](#task-interfaces-and-pipelineresources)
- [SCM support](#scm-support)

## "Feature complete"

Today Tekton can do basically anything you want it to, but there are a few features
still missing or not quite where we want them. Once we have these features, folks
should be able to use Tekton for all of their CI/CD use cases.

Deciding what to support and what to expect users to write into their own `Tasks`
is a constant balancing act, and we want to be careful and deliberate with what we
include.

Features we don't have or aren't yet 100% satisfied with for "feature complete"
Tekton (also discoverable via [the area/roadmap label](https://github.com/tektoncd/pipeline/labels/area%2Froadmap)):

- [Failure strategies](https://github.com/tektoncd/pipeline/issues/1684)
- [Conditional execution](https://github.com/tektoncd/pipeline/blob/master/docs/conditions.md)
  - we will continue to iterate on this functionality as we add failure strategies
- [Pause and resume](https://github.com/tektoncd/pipeline/issues/233)
- [Partial execution](https://github.com/tektoncd/pipeline/issues/50)
- [Notifications](https://github.com/tektoncd/pipeline/issues/1740)
- [`Task` Versioning](https://github.com/tektoncd/pipeline/issues/1839) - Also required
  for [Catalog](#catalog) success
- [Alternative Task Implementations](https://github.com/tektoncd/pipeline/issues/215)
- [Local Execution](https://github.com/tektoncd/pipeline/issues/235)
- [Testing](https://github.com/tektoncd/pipeline/issues/1289) and [debugging](https://github.com/tektoncd/pipeline/issues/2069) frameworks
- [Emitting events throughout `Pipeline`
  execution](https://github.com/tektoncd/pipeline/issues/2082)
- [Config as code](https://github.com/tektoncd/pipeline/issues/859) - This has some cross
  over with [a related Triggers issue](https://github.com/tektoncd/triggers/issues/189)
- [Performant Tekton](https://github.com/tektoncd/pipeline/issues/540) - We should set
  performance requirements and measure against them
- [Adding support for other architectures](https://github.com/tektoncd/pipeline/issues/856)
- [Rich type support for params](https://github.com/tektoncd/pipeline/issues/1393)
- Looping syntax (in [Tasks](https://github.com/tektoncd/pipeline/issues/2112) and/or [Pipelines](https://github.com/tektoncd/pipeline/issues/2050))- either implement or decide it is outside of scope (i.e. better suited for a DSL)
- [Supplying credentials for authenticated actions in Tasks](https://github.com/tektoncd/pipeline/issues/2343) - documented guidelines
  and implemented support for best practices when working with credentials.
- [Provide a pipeline concurrency limit](https://github.com/tektoncd/pipeline/issues/1305)

## Beta for all components

In early 2020 we will have our first beta release, however
[it will be only for a subset of Pipeline's resources](https://docs.google.com/document/d/1H8I2Rk4kLdQaR4mV0A71Qbk-1FxXFrmvisEAjLKT6H0/edit#heading=h.t0sc4hdrr5yq).
After the initial beta release, we would like to get the rest of the resources to beta
as well.

## Task Interfaces and PipelineResources

In alpha Tekton Pipelines, [`PipelineResources`](https://github.com/tektoncd/pipeline/issues/1673)
were the interface between `Tasks` in a `Pipeline`: they were used for sharing typed
data and variables between `Tasks`. This is being revisited and the feature has been
deconstructed into:

* `workspaces` - A way for a `Task` to declare data it needs and provides without
  needing to know about the underlying mechanism used
* `Task results` (aka "output params") - A way for a `Task` to provide values as outputs
  which can be provided to downstream `Tasks`

After adding these features we will once again reconsider the design of `PipelineResources`
and decide what additional features, if any, we need.

We will also need to decide how to continue to release and maintain the code that
is used to build the images used by the existing `PipelineResources`:
https://github.com/tektoncd/pipeline/tree/master/cmd.

## SCM support

SCM support in 2019 was handled by
[the PullRequest Resource](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md#pull-request-resource).
However [we are revisiting PipelineResources](#task-interfaces-and-pipelineresources),
and likely this `PipelineResource` will become one or more `Tasks` in
[the catalog](https://github.com/tektoncd/catalog) instead. Once that happens,
we will need to make decisions about how to release and maintain the
supporting code.
