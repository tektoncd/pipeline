<!--
---
linkTitle: "Events"
weight: 2
---
-->
# Events in Tekton

Tekton's task controller emits [Kubernetes events](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#event-v1-core)
when `TaskRuns` and `PipelineRuns` execute. This allows you to monitor and react to what's happening during execution by
retrieving those events using the `kubectl describe` command. Tekton can also emit [CloudEvents](https://github.com/cloudevents/spec).

**Note:** `Conditions` [do not yet emit events](https://github.com/tektoncd/pipeline/issues/2461).

## Events in `TaskRuns`

`TaskRuns` emit events for the following `Reasons`:

- `Started`: emitted the first time the `TaskRun` is picked by the
  reconciler from its work queue, so it only happens if webhook validation was
  successful. This event in itself does not indicate that a `Step` is executing;
  the `Step` executes once the following conditions are satisfied:
  - Validation of the `Task` and  its associated resources must succeed, and
  - Checks for associated `Conditions` must succeed, and
  - Scheduling of the associated `Pod` must succeed.
- `Succeeded`: emitted once all steps in the `TaskRun` have executed successfully,
   including post-steps injected by Tekton.
- `Failed`: emitted if the `TaskRun` finishes running unsuccessfully because a `Step` failed,
   or the `TaskRun` timed out or was cancelled. A `TaskRun` also emits `Failed` events
   if it cannot execute at all due to failing validation.

## Events in `PipelineRuns`

`PipelineRuns` emit events for the following `Reasons`:

- `Started`: emitted the first time the `PipelineRun` is picked by the
  reconciler from its work queue, so it only happens if webhook validation was
  successful. This event in itself does not indicate that a `Step` is executing;
  the `Step` executes once validation for the `Pipeline` as well as all associated `Tasks`
  and `Resources` is successful.
- `Running`: emitted when the `PipelineRun` passes validation and
  actually begins execution.
- `Succeeded`: emitted once all `Tasks` reachable via the DAG have
  executed successfully.
- `Failed`: emitted if the `PipelineRun` finishes running unsuccessfully because a `Task` failed or the
  `PipelineRun` timed out or was cancelled. A `PipelineRun` also emits `Failed` events if it cannot
  execute at all due to failing validation.

# Events via `CloudEvents`

When you [configure a sink](install.md#configuring-cloudevents-notifications), Tekton emits
events as described in the table below.

Tekton sends cloud events in a parallel routine to allow for retries without blocking the
reconciler. A routine is started every time the `Succeeded` condition changes - either state,
reason or message. Retries are sent using an exponential back-off strategy. 
Because of retries, events are not guaranteed to be sent to the target sink in the order they happened.

Resource      |Event    |Event Type
:-------------|:-------:|:----------------------------------------------------------
`TaskRun`     | `Started` | `dev.tekton.event.taskrun.started.v1`
`TaskRun`     | `Running` | `dev.tekton.event.taskrun.running.v1`
`TaskRun`     | `Condition Change while Running` | `dev.tekton.event.taskrun.unknown.v1`
`TaskRun`     | `Succeed` | `dev.tekton.event.taskrun.successful.v1`
`TaskRun`     | `Failed`  | `dev.tekton.event.taskrun.failed.v1`
`PipelineRun` | `Started` | `dev.tekton.event.pipelinerun.started.v1`
`PipelineRun` | `Running` | `dev.tekton.event.pipelinerun.running.v1`
`PipelineRun` | `Condition Change while Running` | `dev.tekton.event.pipelinerun.unknown.v1`
`PipelineRun` | `Succeed` | `dev.tekton.event.pipelinerun.successful.v1`
`PipelineRun` | `Failed`  | `dev.tekton.event.pipelinerun.failed.v1`
