# Build CRD 2018 Roadmap

This is an incomplete list of work we hope to accomplish in 2018.

## Themes

  * Support for build **detection** from source
  * Cultivate a **community** of shareable BuildTemplate configurations, and
    builder images that enable them
  * More complex **workflows** and flow control of builds and/or steps
  * Enable automatic **triggered** builds in response to events, in some common
    understandable way
  * **Improve** the featureset of the Build CRD by standardizing support for
    things like build caching using on-cluster resources
  * **Integrate** Build CRD into more services, and make it more powerful by
    integrating Build CRD with other services.

In more detail:

## Detection

Users shouldn't have to know explicitly which version of a build template
they're using; they have source, they want to build and a container image.
In the middle of that, before building, is **detection**.

How this works in real life is a hard problem, that a lot of smart people have
spent a lot of their smarts working on it. How that fits into the Build
resource, or whether it's a higher-level concept "above" Builds remains to be
seen.

In any case, these are our criteria for detection:

  * Detection heuristics must be open sourced
  * Users must have visibility into what detection decision was made and why
  * Detection should be an understandable, repeatable process

## Community

BuildTemplates are designed to be shared. We should cultivate a repo of common
templates, and the builder images they rely on. These can serve as working
examples of how to produce a good build template, and to encourage users to
contribute their own.

Templates should, as much as possible, take advantage of some of the conceptual
advances we've made with FTL-like layer caching techniques, [remote image
rebasing](https://github.com/google/image-rebase), source detection, and in
general be best-in-class in terms of fast incremental builds.

[GCB](cloud.google.com/container-builder/docs) already maintains a set of
[officially-supported
builders](https://github.com/GoogleCloudPlatform/cloud-builders), and some
[contributed by community
members](https://github.com/GoogleCloudPlatform/cloud-builders-community). We'd
be happy to contribute any of these for which there is demand. CloudFoundry has
done some very exciting work to start supporting buildpacks. Other interested
parties should have a central place to contribute and advertise their support,
and that repository should be discoverable and useful to end users.

## Workflow

Today, a Build expresses a list of step containers to run in order, to
completion. We believe this covers many common build use cases, but some users
will inevitably want to layer on more complex build workflows, and we should
try to enable that.

Features we've identified that users might want to include in their build
workflows:

  * Parallel execution (fan-out/fan-in)
  * Conditional step execution ("only execute step X if Git branch ==
    'master'")
  * Retry steps ("run step X up to three times until it succeeds")
  * Runtime-conditional step execution ("only execute step Y if step X failed";
    e.g., recover/cleanup a broken rollout)

This may expand to cover generating a build from some other format, which
introduces the possibility of preprocessors to translate that other format into
a Kubernetes-native Build{Template} CRD resource.

## Triggering

Many users want to build automatically when they push to a repo, or in response
to some outside event, or on a schedule. How to get notified of repo changes,
and knowing what to build when we get that notification, are open questions. It
probably makes sense to share concepts and infrastructure with knative' ongoing
[eventing](https://github.com/knative/eventing) work, rather than build and
maintain our own.

This work could also be informed by expertise gained by the Prow project, and
the expertise of the community, many of whom have built build triggering
services in the past.

## Improvements

The Build CRD is powerful, but far from perfect.

In particular, settling on a standard for expressing a secure and reliable
persistent caching solution for build inputs and artifacts, supported by build
templates and builder images, could greatly improve build speeds and reduce
cost to users.

## Integrations

The Build CRD provides a useful model for expressing a build which produces a
container image, and a lot of the adjacent technologies (FTL, buildpacks,
dockerless rebasing, etc.) can be useful to other projects as well.
[Skaffold](https://github.com/GoogleCloudPlatform/skaffold), for instance,
could benefit from adopting the Build CRD build model.

There are also gains to be made from integrating Build CRD with other services,
such as [Grafeas](https://grafeas.io), which we can use to record metadata
about containers produced during a build.

We should explore these integrations, and others as they become relevant.
