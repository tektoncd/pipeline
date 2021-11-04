<!--
---
linkTitle: "Events"
weight: 700
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

## Format of `CloudEvents`

According to the [`CloudEvents` spec](https://github.com/cloudevents/spec/blob/master/spec.md), HTTP headers are included to match the context fields. For example:

```
"Ce-Id": "77f78ae7-ff6d-4e39-9d05-b9a0b7850527",
"Ce-Source": "/apis/tekton.dev/v1beta1/namespaces/default/taskruns/curl-run-6gplk",
"Ce-Specversion": "1.0",
"Ce-Subject": "curl-run-6gplk",
"Ce-Time": "2021-01-29T14:47:58.157819Z",
"Ce-Type": "dev.tekton.event.taskrun.unknown.v1",
```

Other HTTP headers are:
```
"Accept-Encoding": "gzip",
"Connection": "close",
"Content-Length": "3519",
"Content-Type": "application/json",
"User-Agent": "Go-http-client/1.1"
```

The payload is JSON, a map with a single root key `taskRun` or `pipelineRun`, depending on the source
of the event. Inside the root key, the whole `spec` and `status` of the resource is included. For example:

```json
{
  "taskRun": {
    "metadata": {
      "annotations": {
        "pipeline.tekton.dev/release": "v0.20.1",
        "tekton.dev/pipelines.minVersion": "0.12.1",
        "tekton.dev/tags": "search"
      },
      "creationTimestamp": "2021-01-29T14:47:57Z",
      "generateName": "curl-run-",
      "generation": 1,
      "labels": {
        "app.kubernetes.io/managed-by": "tekton-pipelines",
        "app.kubernetes.io/version": "0.1",
        "tekton.dev/task": "curl"
      },
      "managedFields": "(...)",
      "name": "curl-run-6gplk",
      "namespace": "default",
      "resourceVersion": "156770",
      "selfLink": "/apis/tekton.dev/v1beta1/namespaces/default/taskruns/curl-run-6gplk",
      "uid": "4ccb4f01-3ecc-4eb4-87e1-76f04efeee5c"
    },
    "spec": {
      "params": [
        {
          "name": "url",
          "value": "https://api.hub.tekton.dev/resource/96"
        }
      ],
      "resources": {},
      "serviceAccountName": "default",
      "taskRef": {
        "kind": "Task",
        "name": "curl"
      },
      "timeout": "1h0m0s"
    },
    "status": {
      "conditions": [
        {
          "lastTransitionTime": "2021-01-29T14:47:58Z",
          "message": "pod status \"Initialized\":\"False\"; message: \"containers with incomplete status: [place-tools]\"",
          "reason": "Pending",
          "status": "Unknown",
          "type": "Succeeded"
        }
      ],
      "podName": "curl-run-6gplk-pod",
      "startTime": "2021-01-29T14:47:57Z",
      "steps": [
        {
          "container": "step-curl",
          "name": "curl",
          "waiting": {
            "reason": "PodInitializing"
          }
        }
      ],
      "taskSpec": {
        "description": "This task performs curl operation to transfer data from internet.",
        "params": [
          {
            "description": "URL to curl'ed",
            "name": "url",
            "type": "string"
          },
          {
            "default": [],
            "description": "options of url",
            "name": "options",
            "type": "array"
          },
          {
            "default": "docker.io/curlimages/curl:7.72.0@sha256:3c3ff0c379abb1150bb586c7d55848ed4dcde4a6486b6f37d6815aed569332fe",
            "description": "option of curl image",
            "name": "curl-image",
            "type": "string"
          }
        ],
        "steps": [
          {
            "args": [
              "$(params.options[*])",
              "$(params.url)"
            ],
            "command": [
              "curl"
            ],
            "image": "$(params.curl-image)",
            "name": "curl",
            "resources": {}
          }
        ]
      }
    }
  }
}
```
