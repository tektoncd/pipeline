<!--
---
linkTitle: "Events"
weight: 2
---
-->
# Events

Tekton runtime resources, specifically `TaskRuns` and `PipelineRuns`,
emit events when they are executed, so that users can monitor their lifecycle
and react to it. Tekton emits [kubernetes events](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#event-v1-core), that can be retrieve from the resource via
`kubectl describe [resource]`.

No events are emitted for `Conditions` today (https://github.com/tektoncd/pipeline/issues/2461).

## TaskRuns

`TaskRun` events are generated for the following `Reasons`:

- `Started`: this is triggered the first time the `TaskRun` is picked by the
  reconciler from its work queue, so it only happens if web-hook validation was
  successful. Note that this event does not imply that a step started executing,
  as several conditions must be met first:
  - task and bound resource validation must be successful
  - attached conditions must run successfully
  - the `Pod` associated to the `TaskRun` must be successfully scheduled
- `Succeeded`: this is triggered once all steps in the `TaskRun` are executed
  successfully, including post-steps injected by Tekton.
- `Failed`: this is triggered if the `TaskRun` is completed, but not successfully.
  Causes of failure may be: one the steps failed, the `TaskRun` was cancelled or
  the `TaskRun` timed out. `Failed` events are also triggered in case the `TaskRun`
  cannot be executed at all because of validation issues.

## PipelineRuns

`PipelineRun` events are generated for the following `Reasons`:

- `Started`: this is triggered the first time the `PipelineRun` is picked by the
  reconciler from its work queue, so it only happens if web-hook validation was
  successful. Note that this event does not imply that a step started executing,
  as pipeline, task and bound resource validation must be successful first.
- `Running`: this is triggered when the `PipelineRun` passes validation and
  actually starts running.
- `Succeeded`: this is triggered once all `Tasks` reachable via the DAG are
  executed successfully.
- `Failed`: this is triggered if the `PipelineRun` is completed, but not
  successfully. Causes of failure may be: one the `Tasks` failed or the
  `PipelineRun` was cancelled or timed out. `Failed` events are also triggered
  in case the `PipelineRun` cannot be executed at all because of validation issues.
