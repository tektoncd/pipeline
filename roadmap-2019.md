# Tekton Pipelines 2019 Roadmap

This is an incomplete list of work we hope to accomplish in 2019.

Highlights:

- [Version 1.0](#version-10)
- [Workflow features](#workflow) such as conditional execution
- [Triggering](#triggering)
- [Security](#security)
- [SCM](#scm)
- [Library of shared Pipelines and Tasks](#community-library)

_For comparison, Build CRD's 2018 roadmap is
[here](https://github.com/knative/build/blob/master/roadmap-2018.md)._

## Version 1.0

Currently [the Pipeline API is considered `alpha`](api_compatibility_policy.md).
In 2019 we aim to release a 1.0 version, after which we would implement a policy
where backwards incompatible changes must be made across multiple releases.

This would also imply that the project is in a state where it is safe for other
projects to rely on it, for example:

- If users of [`knative/build`](https://github.com/knative/build) want to
  migrate to [TaskRun](docs/taskruns.md)
- If [`knative/serving`](https://github.com/knative/serving) would like to take
  a dependency on this project

## Workflow

Enable more powerful workflows than the simple linear builds we support today.
This includes:

- DAG pipelines, including fan-in and fan-out with concurrent task execution
- Conditional execution (e.g., run a task based on the value of a pipeline
  parameter, run a task in response to the failure of another task)
- Cancelling or pausing a workflow
- Resuming a paused or failed workflow
- Enforcing timeouts on Tasks and on Pipelines
- Sending notifications (e.g., Slack, e-mail, GitHub statuses)
- Reporting build results and logs in a useful and extensible way
- Enabling pluggable tasks that conform to some defined contract (e.g., similar
  to Serving's duck-typing support), allowing tasks in a pipeline to be executed
  off-cluster, etc.

Some of this work is underway, some is being designed, and some are only
identified as gaps. More workflow features will be identified in the future.

## Triggering

Automating a workflow means automating how those workflows are started. We need
to design support for automatically kicking off pipelines based on events, e.g.,
GitHub webhook events. This should either take a hard dependency on Knative
Eventing, or support a duck-typing model to allow the eventing subsystem to be
swappable in the future. This work has not yet been designed at this time, so
there's a great opportunity in the community to pick up this work and make it
shine.

## Security

Security requirements inform much of the overall Tekton pipelines design, but we
still have lots of work to do. Declarative pipelines should make it possible to
automatically vet delivery systems for compliance, secure software supply
chains, auditability and other key features.

## SCM

Software Change Management systems are the start of any CI/CD system. Tekton
supports the git protocol today with simple authentication requirements, but
this is just the beginning. Users expect an SCM system to provide features like:

- Code review management
- Issue tracking
- Pre-submit unit and integration testing
- Linting
- and more!

Tekton should abstract these interactions (and providers!) away so Task authors
can focus on adding value rather than implementing API wrappers.

## Community library

Pipelines are designed with many extension points. Pipeline and Task
configurations can be parameterized and shared, they depend on Resources and
builder images which can also be shared. The success of the project depends on
having a thriving community of shared, reusable configurations. This means
writing and sharing example workflows, documenting best practices for
reusability, and answering questions from users.

This also means more fully specifying the builder contract (along the lines of
Serving's very thorough
[spec](https://github.com/knative/serving/blob/master/docs/spec/spec.md))

In 2019 we'll publish a library of Tasks, Resources and Pipelines for users to
share and reuse.

## Release and Dogfooding

Get Pipelines into a reliable release cadence, with trustworthy integration
tests and performance metrics to guard against regressions. The Pipelines team
itself should dogfood Pipelines for our own CI and CD infrastructure, alongside
or instead of Prow, by having Prow use Pipelines somehow.
