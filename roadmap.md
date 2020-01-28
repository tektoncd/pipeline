# Tekton Mission and Roadmap

This doc describes Tekton's mission and the 2020 roadmap primarily
for Tekton Pipelines but some sneak peeks at other projects as well.

- [Mission and Vision](#mission-and-vision)
- [Roadmap](#roadmap)
  - [Beta and GA releases](#beta-and-ga)
  - ["Feature Complete" Tekton](#feature-complete-tekton)
    - [Task Interfaces and PipelineResources](#task-interfaces-and-pipelineresources)
  - [Triggers](#triggers)
  - [SCM support](#scm-support)
  - [Catalog](#catalog)
  - [Results](#results)
  - [Release and dogfooding](#release-and-dogfooding)

## Mission and Vision

Tekton's mission:

  Be the industry-standard, cloud-native CI/CD platform and ecosystem.

The vision for this is:

* Tekton API conformance across as many CI/CD platforms as possible
* A rich catalog of high quality, reusable `Tasks` which work with Tekton conformant systems

What this vision looks like differs across different users:

* **Engineers building CI/CD systems**: These users will be motivated to use Tekton
  and integrate it into the CI/CD systems they are using because building on top
  of Tekton means they don't have to re-invent the wheel and out of the box they get
  scalable, serverless cloud native execution
* **Engineers who need CI/CD**: (aka all software engineers!) These users will benefit
  from the rich high quality catalog of reusable components:

  * Quickly build and interact with sophisticated `Pipelines`
  * Be able to port `Pipelines` to any Tekton conformant system
  * Be able to use multiple Tekton conformant systems instead of being locked into one
    or being forced to build glue between multiple completely different systems
  * Use an ecosystem of tools that know how to interact with Tekton components, e.g.
    IDE integrations, linting, CLIs, security and policy systems

## Roadmap

### Beta and GA

In 2019 we got to the point where we had severl projects built on top of Tekton
that were craving additional API stability, so we started a push toward having
a beta release of Tekton. We have
[created a plan](https://docs.google.com/document/d/1H8I2Rk4kLdQaR4mV0A71Qbk-1FxXFrmvisEAjLKT6H0/edit)
which defines what Beta means for Tekton Pipelines, and
[through our beta working group](https://github.com/tektoncd/community/blob/master/working-groups.md#beta-release)
we are working towared a beta release in early 2020.

The initial beta release
[does not include all Tekton Pipeline components](https://docs.google.com/document/d/1H8I2Rk4kLdQaR4mV0A71Qbk-1FxXFrmvisEAjLKT6H0/edit#heading=h.t0sc4hdrr5yq). After our initial
beta release, we will then work toward:

1. Beta for the remaining components
1. Beta for [Tekton Triggers](#triggers)
1. GA for both Tekton Pipelines and Tekton Triggers

As the project matures, we also require:

1. A website that provides a good landing page for users (tekton.dev)
1. Solid, high quality on boarding and documentation

### "Feature Complete" Tekton

Today Tekton can do basically anything you want it to, but there are a few features
still missing or not quite where we want them. Once we have these features, folks
should be able to use Tekton for all of their CI/CD use cases.

Deciding what to support and what to expect users to write into their own `Tasks`
is a constant balancing act, and we want to be careful and deliberate with what we
include.

Features we don't have or aren't yet 100% satisfied with for "feature complete"
Tekton:

- [Failure strategies](https://github.com/tektoncd/pipeline/issues/1684)
- [Conditional execution](https://github.com/tektoncd/pipeline/blob/master/docs/conditions.md)
  is nearly where we want it, though feedback has indicated this doens't always
  work the way users expect it to and could use a bit of additional polish.
- [Pause and resume](https://github.com/tektoncd/pipeline/issues/233)
- [Partial execution](https://github.com/tektoncd/pipeline/issues/50)
- [Notifications](https://github.com/tektoncd/pipeline/issues/1740)
- [`Task` Versioning](https://github.com/tektoncd/pipeline/issues/1839) - Also required
  for [Catalog](#catalog) success
- [Alternative Task Implementations](https://github.com/tektoncd/pipeline/issues/215)
- [Local Execution](https://github.com/tektoncd/pipeline/issues/235)
- Testing and debugging frameworks
- Emiting events throughout `Pipeline` execution
- [Config as code](https://github.com/tektoncd/pipeline/issues/859) - This has some cross
  over with [a related Triggers issue](https://github.com/tektoncd/triggers/issues/189)
- [Performant Tekton](https://github.com/tektoncd/pipeline/issues/540) - We should set
  performance requirements and measure against them
- [Adding support for other architectures](https://github.com/tektoncd/pipeline/issues/856)
- [Rich type support for params](https://github.com/tektoncd/pipeline/issues/1393)

#### Task Interfaces and PipelineResources

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

### Triggers

In 2019 we created a simple system for creating instances of Tekton resources, triggered
by json payloads sent to HTTP endpoints (Tekton Triggers).

In 2020 we would like to add missing features and then push for
[Triggers to be Beta and later GA](#beta-and-ga):

* [Improved support for many EventListeners](https://github.com/tektoncd/triggers/issues/370)
* [Pluggable core interceptors](https://github.com/tektoncd/triggers/issues/271)
* [Support for expressions in TriggerBindings](https://github.com/tektoncd/triggers/issues/271)
* [Using TriggerTemplates outside the context of an event](https://github.com/tektoncd/triggers/issues/200)
* [Routing to multiple interceptors](https://github.com/tektoncd/triggers/issues/205)
* [Dynamic TriggerTemplate parameters](https://github.com/tektoncd/triggers/issues/87)
* [Performant Triggers](https://github.com/tektoncd/triggers/issues/406)
* Support for poll-based triggering (e.g. when a repo changes state)
* Support for additional expression languages
* Increased traceability (e.g. why did my interceptor reject the event?)

### SCM support

SCM support in 2019 was handled by
[the PullRequest Resource](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md#pull-request-resource).
However [we are revisting PipelineResources](#task-interfaces-and-pipelineresources),
and likely this `PipelineResource` will become one or more `Tasks` in
[the catalog](https://github.com/tektoncd/catalog) instead. Once that happens,
we will need to make decisions about how to release and maintain the
supporting code.

* [GitHub App support](https://github.com/tektoncd/triggers/issues/189)
* Support for more SCM providers

### Catalog

The catalog is a key piece of [Tekton's mission](#mission-and-vision). In 2019
we got a solid start on a catalog of reusable `Tasks` at
https://github.com/tektoncd/catalog, and in 2020 we want to keep this momentum
going and make the catalog even more useful by adding:

* [Component versioning](https://github.com/tektoncd/pipeline/issues/1839)
* Clear indications of which versions of Tekton Pipelines (and Triggers) a component
  is compatible with
* A clear story around ownership of components
* [Support for custom catalogs](https://docs.google.com/document/d/1O8VHZ-7tNuuRjPNjPfdo8bD--WDrkcz-lbtJ3P8Wugs/edit#)
* Support for more components, e.g. `Pipelines` and `TriggerTemplates` in
  addition to `Tasks`
* Increased confidence in component quality through:
  * Clear testing requriements
  * Support for testing components that depend on external services
* Increased documentation and examples
* Well defined Tekton API conformance

### Results

One of the benefits of [defining specifications around CI/CD](#mission-and-vision)
is that we can start to create tools around inspecting what is moving through our
Pipelienes. In 2019 we started some design work around a result storage system for
Tekton, and we want to make progress on this in 2020:

* Design reporting to results store: https://github.com/tektoncd/pipeline/issues/454
* Snapshoting data for history + repeatability: https://github.com/tektoncd/pipeline/issues/279

### Release and Dogfooding

In 2020 we should keep the momentum going we started in 2019 and switch as much
of our CI infrastructure as possible to being purely Tekton, including running our
linting, unit tests, integration tests, etc.