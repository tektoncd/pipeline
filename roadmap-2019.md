# Tekton Pipelines 2019 Roadmap

This is an incomplete list of work we hope to accomplish in 2019.

For comparison, Build CRD's 2018 roadmap is
[here](https://github.com/knative/build/blob/master/roadmap-2018.md).

## Migrate to Pipelines

The Knative Build Working Group has historically focused on `Build` and
associated resources. With the successful experimentation around `Pipelines` and
`Tasks` and `Resources` at the end of 2018, we believe it's time to further
invest in these resources and migrate existing `Build` users to these new more
flexible and powerful resources.

This means ensuring compatibility between the resources, and updating
documentation where necessary to focus on the new resources. We'll probably have
to support both for some amount of time. By mid-to-late 2019 the `Build`
resource should be no more, and `TaskRun` should take its place.

## Don't Break Serving

As always, we should continue to ensure that our main client, Knative Serving,
remains happy with the work we produce. This means fixing bugs, answering
questions and implementing features in a timely manner.

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

## Community

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
